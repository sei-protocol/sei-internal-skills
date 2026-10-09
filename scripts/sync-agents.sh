#!/usr/bin/env bash
# sync-agents.sh — copy sei-internal-skills agent definitions to a target .claude/agents/ directory.
#
# Sibling of sync-skills.sh. Every .claude/agents/*.md syncs.
#
# Usage:
#   sync-agents.sh [--target <path>] [--dry-run] [--force] [--verify] [--inject-doctrine]
#
# --target:      target directory (the script appends .claude/agents/). Default: $HOME.
# --dry-run:     print what would be copied without copying
# --force:       overwrite existing target files without prompting
# --verify:      run ONLY the catalog guard and exit non-zero on any problem. For CI.
#                The guard checks that each agent's frontmatter has a `name:` equal to
#                its file name and a non-empty `description:`. No other checker covers agents.
# --inject-doctrine: also inject the sei-internal-skills operating-doctrine managed block into
#                <target>/AGENTS.md (+ a CLAUDE.md pointer). Off by default;
#                intended for a consuming package, not user scope ($HOME).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$(cd "$SCRIPT_DIR/../.claude/agents" && pwd)"

# shellcheck source=lib/inject-doctrine.sh
. "$SCRIPT_DIR/lib/inject-doctrine.sh"

# --- small helpers ----------------------------------------------------------

# frontmatter_field <file> <key> — the value of <key> in the file's leading
# `---` frontmatter block, with surrounding quotes and whitespace stripped.
# Empty if the file has no frontmatter or the key is absent. A CRLF terminator
# is stripped.
frontmatter_field() {
  awk -v key="$2" '
    { sub(/\r$/, "") }
    NR == 1 { if ($0 == "---") { fm = 1; next } else { exit } }
    $0 == "---" { exit }
    index($0, key ":") == 1 { print substr($0, length(key) + 2); exit }
  ' "$1" 2>/dev/null \
    | sed 's/^[[:space:]]*//; s/[[:space:]]*$//; s/^["'"'"']//; s/["'"'"']$//'
}

list_agent_names() {  # every .md basename (sans extension), sorted
  for f in "$AGENTS_DIR"/*.md; do
    if [ -f "$f" ]; then basename "$f" .md; fi
  done | sort
}

# --- catalog guard ----------------------------------------------------------
# Fail closed if an agent has no `name:`, names another file, or has no description.
run_catalog_guard() {
  local errs=0 n=0 file name desc
  while IFS= read -r file; do
    n=$((n+1))
    name="$(frontmatter_field "$AGENTS_DIR/$file.md" name)"
    desc="$(frontmatter_field "$AGENTS_DIR/$file.md" description)"
    if [ -z "$name" ]; then
      echo "  ✗ $file.md: no 'name:' in frontmatter" >&2
      errs=$((errs+1))
    elif [ "$name" != "$file" ]; then
      echo "  ✗ $file.md: name '$name' does not match its file name" >&2
      errs=$((errs+1))
    fi
    if [ -z "$desc" ]; then
      echo "  ✗ $file.md: no 'description:' in frontmatter" >&2
      errs=$((errs+1))
    fi
  done < <(list_agent_names)
  if [ "$errs" -gt 0 ]; then
    echo "agent catalog: $errs problem(s)" >&2
    return 1
  fi
  echo "agent catalog ✓ ($n agents)"
  return 0
}

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
DRY_RUN=false
FORCE=false
VERIFY=false
INJECT_DOCTRINE=false

usage() { grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)     TARGET="$2"; shift 2 ;;
    --dry-run)    DRY_RUN=true; shift ;;
    --force)      FORCE=true; shift ;;
    --verify)     VERIFY=true; shift ;;
    --inject-doctrine) INJECT_DOCTRINE=true; shift ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if $VERIFY; then run_catalog_guard; exit $?; fi

TARGET="${TARGET/#\~/$HOME}"
TARGET_AGENTS="${TARGET%/}/.claude/agents"

if $INJECT_DOCTRINE && [[ "${TARGET%/}" == "${HOME%/}" ]]; then
  echo "Error: --inject-doctrine refuses \$HOME ($HOME) — target a package directory." >&2
  exit 2
fi

# Catalog guard first so an inconsistent catalog fails loudly here, not just in CI.
if ! run_catalog_guard >/dev/null 2>&1; then
  run_catalog_guard >&2 || true
  echo "Refusing to sync an inconsistent catalog (see above)." >&2
  exit 1
fi

# --- Build the agent list: every agent file ---------------------------------

declare -a AGENTS_TO_SYNC=()
while IFS= read -r name; do AGENTS_TO_SYNC+=("$name"); done < <(list_agent_names)

# --- Report plan ------------------------------------------------------------

echo "Source: $AGENTS_DIR"
echo "Target: $TARGET_AGENTS"
echo "Agents to sync (${#AGENTS_TO_SYNC[@]}):"
if [[ ${#AGENTS_TO_SYNC[@]} -eq 0 ]]; then
  echo "  (none — $AGENTS_DIR holds no .md file)"
else
  printf '  - %s\n' "${AGENTS_TO_SYNC[@]}"
fi

if $INJECT_DOCTRINE; then
  doctrine_mode="write"; $DRY_RUN && doctrine_mode="dry-run"
  inject_doctrine "$TARGET" "$SCRIPT_DIR/sei-internal-skills-doctrine.md" "$doctrine_mode"
fi

if $DRY_RUN; then echo ""; echo "(dry-run — no files copied)"; exit 0; fi
[[ ${#AGENTS_TO_SYNC[@]} -eq 0 ]] && exit 0

# --- Execute ----------------------------------------------------------------

mkdir -p "$TARGET_AGENTS"
COPIED=0; SKIPPED=0; CONFLICTS=0

for agent in "${AGENTS_TO_SYNC[@]}"; do
  src="$AGENTS_DIR/$agent.md"; dst="$TARGET_AGENTS/$agent.md"
  if [[ ! -f "$src" ]]; then
    echo "  ! source missing, skipping: $src" >&2; SKIPPED=$((SKIPPED+1)); continue
  fi
  if [[ -f "$dst" ]] && cmp -s "$src" "$dst"; then SKIPPED=$((SKIPPED+1)); continue; fi
  if [[ -f "$dst" ]] && ! $FORCE; then
    echo "  ! conflict (differs, use --force to overwrite): $dst" >&2
    CONFLICTS=$((CONFLICTS+1)); continue
  fi
  cp "$src" "$dst"
  echo "  ✓ $agent"; COPIED=$((COPIED+1))
done

echo ""
echo "Copied: $COPIED   Skipped (identical/missing): $SKIPPED   Conflicts: $CONFLICTS"
if [[ $CONFLICTS -gt 0 ]]; then
  echo "Re-run with --force to overwrite conflicting files." >&2; exit 1
fi
