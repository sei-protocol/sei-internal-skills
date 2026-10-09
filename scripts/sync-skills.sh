#!/usr/bin/env bash
# sync-skills.sh — copy sei-internal-skills skill directories to a target .claude/skills/ directory.
#
# Sibling of sync-agents.sh. sei-internal-skills is the canonical home; this pushes outward to
# user scope (~/.claude/skills/) and to other repos so they stay current.
#
# A skill is a directory under .claude/skills/ that holds a SKILL.md. Every such
# directory syncs. A directory without a SKILL.md is not a skill, and the script
# ignores it.
#
# Daily flow:
#   make update                     # from the sei-internal-skills repo: pull, sync everything, verify
#
# Usage:
#   sync-skills.sh [--target <path>] [--dry-run] [--force] [--verify] [--inject-doctrine]
#
# --target:      target directory (the script appends .claude/skills/). Default: $HOME.
# --dry-run:     print what would be copied without copying
# --force:       overwrite existing target skills without prompting
# --verify:      run ONLY the catalog guard and exit non-zero on any problem. No copying. For CI.
#                The guard checks that each skill's SKILL.md frontmatter has a `name:` equal
#                to its directory. scripts/skill-package-checks.sh (D1, D2) checks the description.
# --inject-doctrine: also inject the sei-internal-skills operating-doctrine managed block into
#                <target>/AGENTS.md (+ a CLAUDE.md pointer). Off by default;
#                intended for a consuming package, not user scope ($HOME).
#
# Skills are directories (SKILL.md + references/ + ...), not single files. Sync
# uses cp -R, so target-only files stay (user customizations and runtime
# artifacts such as state/ in the target tree are not deleted). If any source
# file is missing from the target copy or differs from it, the script reports
# the skill as a conflict, skips the whole skill and exits 1, unless --force
# is set.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_DIR="$(cd "$SCRIPT_DIR/../.claude/skills" && pwd)"

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

# every skill directory (one that holds a SKILL.md), one per line, sorted
list_skill_dirs() {
  for d in "$SKILLS_DIR"/*/; do
    if [ -f "${d}SKILL.md" ]; then basename "$d"; fi
  done | sort
}

# --- catalog guard ----------------------------------------------------------
# Fail closed if a skill's SKILL.md has no `name:` or names another directory.
run_catalog_guard() {
  local errs=0 n=0 dir name
  while IFS= read -r dir; do
    n=$((n+1))
    name="$(frontmatter_field "$SKILLS_DIR/$dir/SKILL.md" name)"
    if [ -z "$name" ]; then
      echo "  ✗ $dir: no 'name:' in SKILL.md frontmatter" >&2
      errs=$((errs+1))
    elif [ "$name" != "$dir" ]; then
      echo "  ✗ $dir: SKILL.md name '$name' does not match its directory" >&2
      errs=$((errs+1))
    fi
  done < <(list_skill_dirs)
  if [ "$errs" -gt 0 ]; then
    echo "skill catalog: $errs problem(s)" >&2
    return 1
  fi
  echo "skill catalog ✓ ($n skills)"
  return 0
}

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
DRY_RUN=false
FORCE=false
VERIFY=false
INJECT_DOCTRINE=false

# Print the header comment block, and nothing after it.
usage() {
  awk 'NR == 1 { next } !/^#/ { exit } { sub(/^# ?/, ""); print }' "$0"
}

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

# --verify: catalog guard only, no target needed.
if $VERIFY; then
  run_catalog_guard
  exit $?
fi

# Expand ~ if present
TARGET="${TARGET/#\~/$HOME}"
TARGET_SKILLS="${TARGET%/}/.claude/skills"

# Doctrine injection writes a managed block to the package root. Refuse $HOME.
if $INJECT_DOCTRINE && [[ "${TARGET%/}" == "${HOME%/}" ]]; then
  echo "Error: --inject-doctrine refuses \$HOME ($HOME) — target a package directory." >&2
  exit 2
fi

# Run the catalog guard first so an inconsistent catalog fails loudly here too,
# not just in CI.
if ! run_catalog_guard >/dev/null 2>&1; then
  run_catalog_guard >&2 || true
  echo "Refusing to sync an inconsistent catalog (see above)." >&2
  exit 1
fi

# --- Build the skill list: every skill directory ----------------------------

declare -a SKILLS_TO_SYNC=()
while IFS= read -r name; do SKILLS_TO_SYNC+=("$name"); done < <(list_skill_dirs)

# --- Report plan ------------------------------------------------------------

echo "Source: $SKILLS_DIR"
echo "Target: $TARGET_SKILLS"
echo "Skills to sync (${#SKILLS_TO_SYNC[@]}):"
if [[ ${#SKILLS_TO_SYNC[@]} -eq 0 ]]; then
  echo "  (none — no directory under $SKILLS_DIR holds a SKILL.md)"
else
  printf '  - %s\n' "${SKILLS_TO_SYNC[@]}"
fi

if $INJECT_DOCTRINE; then
  doctrine_mode="write"; $DRY_RUN && doctrine_mode="dry-run"
  inject_doctrine "$TARGET" "$SCRIPT_DIR/sei-internal-skills-doctrine.md" "$doctrine_mode"
fi

if $DRY_RUN; then
  echo ""; echo "(dry-run — no files copied)"; exit 0
fi
[[ ${#SKILLS_TO_SYNC[@]} -eq 0 ]] && exit 0

# --- Execute ----------------------------------------------------------------

mkdir -p "$TARGET_SKILLS"
COPIED=0; IN_SYNC=0; MISSING=0; CONFLICTS=0

# Return 0 if every file in source is present and identical in target (target
# may have additional files; those are preserved by the cp -R sync).
source_subset_of_target() {
  local src_dir="$1" dst_dir="$2" src_file rel dst_file
  [[ -d "$dst_dir" ]] || return 1
  while IFS= read -r -d '' src_file; do
    rel="${src_file#"$src_dir"/}"
    dst_file="$dst_dir/$rel"
    if [[ ! -f "$dst_file" ]] || ! cmp -s "$src_file" "$dst_file"; then return 1; fi
  done < <(find "$src_dir" -type f -print0)
  return 0
}

for skill in "${SKILLS_TO_SYNC[@]}"; do
  src="$SKILLS_DIR/$skill"; dst="$TARGET_SKILLS/$skill"
  if [[ ! -d "$src" ]]; then
    echo "  ! source missing, skipping: $src" >&2; MISSING=$((MISSING+1)); continue
  fi
  if [[ -d "$dst" ]]; then
    if source_subset_of_target "$src" "$dst"; then IN_SYNC=$((IN_SYNC+1)); continue; fi
    if ! $FORCE; then
      echo "  ! conflict (target differs, use --force to overwrite): $dst" >&2
      CONFLICTS=$((CONFLICTS+1)); continue
    fi
  fi
  mkdir -p "$dst"; cp -R "$src/." "$dst/"
  echo "  ✓ $skill"; COPIED=$((COPIED+1))
done

echo ""
echo "Copied: $COPIED   In sync: $IN_SYNC   Source missing: $MISSING   Conflicts: $CONFLICTS"
if [[ $CONFLICTS -gt 0 ]]; then
  echo "Re-run with --force to overwrite conflicting skills." >&2; exit 1
fi
