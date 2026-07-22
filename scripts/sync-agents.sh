#!/usr/bin/env bash
# sync-agents.sh — copy sei-internal-skills agent definitions to a target .claude/agents/ directory.
#
# SINGLE SOURCE OF TRUTH: each agent's own `category:` frontmatter. Membership in
# a sync alias (portable / sei) is DERIVED from that category via the small
# domain->alias map below — there is no hand-maintained per-agent list to drift.
# Adding an agent = drop a .md with a `category:`; it syncs automatically. The
# coverage guard (--verify) fails closed if any agent's category maps to no alias.
#
# Usage:
#   sync-agents.sh [--target <path>] [--categories portable,sei,all,<domain>] \
#                  [--dry-run] [--force] [--verify] [--inject-doctrine]
#
# --target:      target directory (the script appends .claude/agents/). Default: $HOME.
# --categories:  comma-separated aliases or domains. Default: portable.
#                aliases: portable, sei, all
#                domains: any value an agent declares in `category:`
# --dry-run:     print what would be copied without copying
# --force:       overwrite existing target files without prompting
# --verify:      run ONLY the coverage guard and exit non-zero on any gap. For CI.
# --inject-doctrine: also inject the sei-internal-skills operating-doctrine managed block into
#                <target>/AGENTS.md (+ a CLAUDE.md pointer). Off by default;
#                intended for a consuming package, not user-scope ($HOME).
#
# To re-categorize an agent, edit its `category:` frontmatter — not this script.
# To change which alias a domain belongs to, edit the domain->alias map below.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$(cd "$SCRIPT_DIR/../.claude/agents" && pwd)"

# shellcheck source=lib/inject-doctrine.sh
. "$SCRIPT_DIR/lib/inject-doctrine.sh"

# --- Domain -> alias map (the ONLY hand-maintained categorization) ----------
#
# Every domain an agent declares in `category:` must appear in exactly one list.
# `all` = PORTABLE+SEI. There are no sei-internal-skills-local agents.
PORTABLE_DOMAINS="platform-infra observability security blockchain code-quality writing-quality product-management release-operations data-architecture"
SEI_DOMAINS="project-management recruiting"

# Cross-cutting exception: agents that are Sei-scoped despite a portable domain.
# sei-network-specialist is platform-infra by domain but ships only via `sei`
# (Sei-specific node networking). Kept as an explicit, guard-checked name list so
# the one exception is visible rather than buried.
SEI_NAME_OVERRIDES="sei-network-specialist"

# --- small helpers ----------------------------------------------------------

in_list() { case " $2 " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

agent_category() {  # agent_category <name> — declared `category:` (empty if none)
  # `grep || true` so a no-match does NOT fail the pipeline and abort the caller
  # under `set -e`/`pipefail` (the guard must print its diagnostic, not crash).
  # `tr -d '\r'` strips a CRLF terminator GNU sed's [[:space:]] would leave.
  { grep -m1 '^category:' "$AGENTS_DIR/$1.md" 2>/dev/null || true; } \
    | tr -d '\r' \
    | sed 's/^category:[[:space:]]*//; s/[[:space:]]*$//; s/^["'"'"']//; s/["'"'"']$//'
}

# alias_for_agent <name> <category> — echoes portable|sei|UNKNOWN
alias_for_agent() {
  if in_list "$1" "$SEI_NAME_OVERRIDES"; then echo sei; return; fi
  if   in_list "$2" "$PORTABLE_DOMAINS"; then echo portable
  elif in_list "$2" "$SEI_DOMAINS";      then echo sei
  else echo UNKNOWN; fi
}

list_agent_names() {  # every .md basename (sans extension), sorted
  for f in "$AGENTS_DIR"/*.md; do basename "$f" .md; done | sort
}

# --- coverage guard ---------------------------------------------------------
run_coverage_guard() {
  local errs=0 name cat al
  while IFS= read -r name; do
    cat="$(agent_category "$name")"
    if [ -z "$cat" ]; then
      echo "  ✗ $name: no 'category:' in frontmatter" >&2
      errs=$((errs+1)); continue
    fi
    al="$(alias_for_agent "$name" "$cat")"
    if [ "$al" = "UNKNOWN" ]; then
      echo "  ✗ $name: category '$cat' maps to no sync alias — add '$cat' to PORTABLE_DOMAINS or SEI_DOMAINS in sync-agents.sh" >&2
      errs=$((errs+1))
    fi
  done < <(list_agent_names)
  if [ "$errs" -gt 0 ]; then
    echo "agent catalog coverage: $errs problem(s) — every agent's category must map to an alias." >&2
    return 1
  fi
  echo "agent catalog coverage ✓ (every agent's category resolves to portable/sei)"
  return 0
}

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
CATEGORIES="portable"
DRY_RUN=false
FORCE=false
VERIFY=false
INJECT_DOCTRINE=false

usage() { grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)     TARGET="$2"; shift 2 ;;
    --categories) CATEGORIES="$2"; shift 2 ;;
    --dry-run)    DRY_RUN=true; shift ;;
    --force)      FORCE=true; shift ;;
    --verify)     VERIFY=true; shift ;;
    --inject-doctrine) INJECT_DOCTRINE=true; shift ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

if $VERIFY; then run_coverage_guard; exit $?; fi

TARGET="${TARGET/#\~/$HOME}"
TARGET_AGENTS="${TARGET%/}/.claude/agents"

if $INJECT_DOCTRINE && [[ "${TARGET%/}" == "${HOME%/}" ]]; then
  echo "Error: --inject-doctrine refuses \$HOME ($HOME) — target a package directory." >&2
  exit 2
fi

# Coverage guard first so a miscategorized agent fails loudly here, not just CI.
if ! run_coverage_guard >/dev/null 2>&1; then
  run_coverage_guard >&2 || true
  echo "Refusing to sync with an incomplete catalog (see above). Fix the category mapping first." >&2
  exit 1
fi

# --- Build agent list from requested categories -----------------------------

want_agent() {  # want_agent <name> <category> <requested-token>
  local name="$1" cat="$2" tok="$3" al
  al="$(alias_for_agent "$name" "$cat")"
  case "$tok" in
    all)      [ "$al" = "portable" ] || [ "$al" = "sei" ] ;;
    portable) [ "$al" = "portable" ] ;;
    sei)      [ "$al" = "sei" ] ;;
    *)        [ "$cat" = "$tok" ] ;;   # literal domain request
  esac
}

declare -a AGENTS_TO_SYNC=()
while IFS= read -r name; do
  cat="$(agent_category "$name")"
  IFS=',' read -ra TOKENS <<< "$CATEGORIES"
  for tok in "${TOKENS[@]}"; do
    if want_agent "$name" "$cat" "$tok"; then AGENTS_TO_SYNC+=("$name"); break; fi
  done
done < <(list_agent_names)

# Deduplicate while preserving order (bash 3.2 compatible)
if [[ ${#AGENTS_TO_SYNC[@]} -gt 0 ]]; then
  UNIQUE_LIST=$(printf '%s\n' "${AGENTS_TO_SYNC[@]}" | awk '!seen[$0]++')
  AGENTS_TO_SYNC=()
  while IFS= read -r a; do [[ -n "$a" ]] && AGENTS_TO_SYNC+=("$a"); done <<< "$UNIQUE_LIST"
fi

# --- Report plan ------------------------------------------------------------

echo "Source: $AGENTS_DIR"
echo "Target: $TARGET_AGENTS"
echo "Categories: $CATEGORIES"
echo "Agents to sync (${#AGENTS_TO_SYNC[@]}):"
if [[ ${#AGENTS_TO_SYNC[@]} -eq 0 ]]; then
  echo "  (none — selected categories are empty)"
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
