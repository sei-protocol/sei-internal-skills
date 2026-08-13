#!/usr/bin/env bash
# sync-experimental.sh — copy experimental/ skills and agents into a target .claude/ directory.
#
# OPT-IN ONLY. `make update`, `make sync-all`, `make bootstrap`, and the over-the-wire
# installer all ignore experimental/ entirely. Nothing here reaches a teammate's
# environment unless they run this script by name.
#
# WHY THE FOLDER EXISTS: sei-internal-skills ships a focused core — the skills and agents
# an engineering team uses on ordinary work. experimental/ is the parking lot for
# everything else: resources that are still being shaped, serve a narrow audience,
# or were exploratory. They keep their history and stay runnable; they just do not
# ride along with the default install.
#
# The exclusion is structural, not a flag. sync-skills.sh and sync-agents.sh read
# .claude/skills/ and .claude/agents/ and nothing else, so a resource is excluded by
# living in experimental/ — there is no category to set and none to drift.
#
# Promote: git mv experimental/skills/<name> .claude/skills/<name>  (then `make verify-catalog`
#          — the coverage guard now applies, so its category: must map to an alias).
# Park:    git mv .claude/skills/<name> experimental/skills/<name>
#
# Usage:
#   sync-experimental.sh [--target <path>] [--dry-run] [--force]
#
# --target:   target directory (the script appends .claude/). Default: $HOME.
# --dry-run:  print what would be copied without copying
# --force:    overwrite existing target resources without prompting
#
# Conflict semantics match sync-skills.sh: a locally-modified target is reported and
# skipped unless --force, and target-only files are never deleted.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXP_DIR="$(cd "$SCRIPT_DIR/../experimental" && pwd)"

# --- Argument parsing -------------------------------------------------------

TARGET="$HOME"
DRY_RUN=false
FORCE=false

usage() {
  grep '^#' "$0" | sed 's/^# \{0,1\}//' | grep -v '^!'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)  TARGET="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    --force)   FORCE=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

TARGET="${TARGET/#\~/$HOME}"
TARGET_SKILLS="${TARGET%/}/.claude/skills"
TARGET_AGENTS="${TARGET%/}/.claude/agents"

# --- Build lists ------------------------------------------------------------

declare -a EXP_SKILLS=()
if [[ -d "$EXP_DIR/skills" ]]; then
  while IFS= read -r d; do
    [[ -n "$d" ]] && EXP_SKILLS+=("$(basename "$d")")
  done < <(find "$EXP_DIR/skills" -mindepth 1 -maxdepth 1 -type d | sort)
fi

declare -a EXP_AGENTS=()
if [[ -d "$EXP_DIR/agents" ]]; then
  while IFS= read -r f; do
    [[ -n "$f" ]] && EXP_AGENTS+=("$(basename "$f")")
  done < <(find "$EXP_DIR/agents" -maxdepth 1 -type f -name '*.md' | sort)
fi

# --- Report plan ------------------------------------------------------------

echo "Source: $EXP_DIR"
echo "Target: ${TARGET%/}/.claude"
echo ""
echo "Experimental skills (${#EXP_SKILLS[@]}):"
if [[ ${#EXP_SKILLS[@]} -eq 0 ]]; then echo "  (none)"; else printf '  - %s\n' "${EXP_SKILLS[@]}"; fi
echo "Experimental agents (${#EXP_AGENTS[@]}):"
if [[ ${#EXP_AGENTS[@]} -eq 0 ]]; then echo "  (none)"; else printf '  - %s\n' "${EXP_AGENTS[@]}"; fi

if $DRY_RUN; then
  echo ""; echo "(dry-run — no files copied)"; exit 0
fi
if [[ ${#EXP_SKILLS[@]} -eq 0 && ${#EXP_AGENTS[@]} -eq 0 ]]; then exit 0; fi

# --- Execute ----------------------------------------------------------------

COPIED=0; IN_SYNC=0; CONFLICTS=0

# Return 0 if every file in source is present and identical in target. Target may
# hold extra files; the cp -R sync preserves them.
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

echo ""
if [[ ${#EXP_SKILLS[@]} -gt 0 ]]; then
  mkdir -p "$TARGET_SKILLS"
  for skill in "${EXP_SKILLS[@]}"; do
    src="$EXP_DIR/skills/$skill"; dst="$TARGET_SKILLS/$skill"
    if [[ -d "$dst" ]]; then
      if source_subset_of_target "$src" "$dst"; then IN_SYNC=$((IN_SYNC+1)); continue; fi
      if ! $FORCE; then
        echo "  ! conflict (target differs, use --force to overwrite): $dst" >&2
        CONFLICTS=$((CONFLICTS+1)); continue
      fi
    fi
    mkdir -p "$dst"; cp -R "$src/." "$dst/"
    echo "  ✓ skill/$skill"; COPIED=$((COPIED+1))
  done
fi

if [[ ${#EXP_AGENTS[@]} -gt 0 ]]; then
  mkdir -p "$TARGET_AGENTS"
  for agent in "${EXP_AGENTS[@]}"; do
    src="$EXP_DIR/agents/$agent"; dst="$TARGET_AGENTS/$agent"
    if [[ -f "$dst" ]]; then
      if cmp -s "$src" "$dst"; then IN_SYNC=$((IN_SYNC+1)); continue; fi
      if ! $FORCE; then
        echo "  ! conflict (target differs, use --force to overwrite): $dst" >&2
        CONFLICTS=$((CONFLICTS+1)); continue
      fi
    fi
    cp "$src" "$dst"
    echo "  ✓ agent/$agent"; COPIED=$((COPIED+1))
  done
fi

echo ""
echo "Copied: $COPIED   In sync: $IN_SYNC   Conflicts: $CONFLICTS"
if [[ $CONFLICTS -gt 0 ]]; then
  echo "Re-run with --force to overwrite conflicting resources." >&2; exit 1
fi
echo ""
echo "These are experimental. They are not part of the shipped core and may change or be removed."
