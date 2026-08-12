#!/usr/bin/env bash
# sync-output-styles.sh — copy sei-internal-skills output styles to a target .claude/output-styles/ directory.
#
# Sibling of sync-skills.sh and sync-agents.sh. sei-internal-skills is the canonical home;
# this pushes outward to user-scope (~/.claude/output-styles/).
#
# WHAT AN OUTPUT STYLE IS: a response-format directive that Claude Code loads into
# the system prompt for every turn in the session. It governs how the assistant
# writes — not what it knows (skills) and not which persona handles a task
# (agents). One file, one style, flat .md — no directories, no categories.
#
# THIS SCRIPT SHIPS THE FILE. IT DOES NOT ACTIVATE IT.
# Activation means setting "outputStyle" in a user's settings.json, which changes
# assistant behavior in every session and every repo, silently. That is the user's
# call, not the installer's — and writing it would clobber anyone who already
# picked a different style. So: copy the file, print the opt-in, stop.
#
# Daily flow:
#   make update                       # from the repo: pull + sync everything + verify
#   ./scripts/sync-output-styles.sh   # equivalent to: --target ~
#
# Usage:
#   sync-output-styles.sh [--target <path>] [--dry-run] [--force]
#
# --target:   target directory (the script appends .claude/output-styles/). Default: $HOME.
# --dry-run:  print what would be copied without copying
# --force:    overwrite existing target styles without prompting
#
# If a source file differs from its target counterpart, the style is reported as a
# conflict and skipped unless --force is set. Target-only files are never deleted,
# so a user's own hand-written styles survive a sync.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STYLES_DIR="$(cd "$SCRIPT_DIR/../.claude/output-styles" && pwd)"

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

# Expand ~ if present
TARGET="${TARGET/#\~/$HOME}"
TARGET_STYLES="${TARGET%/}/.claude/output-styles"

# --- Build style list -------------------------------------------------------

declare -a STYLES_TO_SYNC=()
while IFS= read -r f; do
  [[ -n "$f" ]] && STYLES_TO_SYNC+=("$(basename "$f")")
done < <(find "$STYLES_DIR" -maxdepth 1 -type f -name '*.md' | sort)

# --- Report plan ------------------------------------------------------------

echo "Source: $STYLES_DIR"
echo "Target: $TARGET_STYLES"
echo "Output styles to sync (${#STYLES_TO_SYNC[@]}):"
if [[ ${#STYLES_TO_SYNC[@]} -eq 0 ]]; then
  echo "  (none)"
else
  printf '  - %s\n' "${STYLES_TO_SYNC[@]}"
fi

if $DRY_RUN; then
  echo ""; echo "(dry-run — no files copied)"; exit 0
fi
[[ ${#STYLES_TO_SYNC[@]} -eq 0 ]] && exit 0

# --- Execute ----------------------------------------------------------------

mkdir -p "$TARGET_STYLES"
COPIED=0; IN_SYNC=0; CONFLICTS=0

for style in "${STYLES_TO_SYNC[@]}"; do
  src="$STYLES_DIR/$style"; dst="$TARGET_STYLES/$style"
  if [[ -f "$dst" ]]; then
    if cmp -s "$src" "$dst"; then IN_SYNC=$((IN_SYNC+1)); continue; fi
    if ! $FORCE; then
      echo "  ! conflict (target differs, use --force to overwrite): $dst" >&2
      CONFLICTS=$((CONFLICTS+1)); continue
    fi
  fi
  cp "$src" "$dst"
  echo "  ✓ $style"; COPIED=$((COPIED+1))
done

echo ""
echo "Copied: $COPIED   In sync: $IN_SYNC   Conflicts: $CONFLICTS"
if [[ $CONFLICTS -gt 0 ]]; then
  echo "Re-run with --force to overwrite conflicting styles." >&2; exit 1
fi

# --- Opt-in notice ----------------------------------------------------------
#
# Deliberately a message, not a settings write. See the header.
#
# Stay quiet once the user has picked ANY style — they have made the choice, and
# `make update` runs often enough that repeating the hint would just be noise.
# grep over JSON is crude, but both failure modes are harmless here: a false
# positive drops a hint, a false negative repeats one.

SETTINGS="${TARGET%/}/.claude/settings.json"
if grep -q '"outputStyle"' "$SETTINGS" 2>/dev/null; then
  exit 0
fi

cat <<'OPTIN'

Output styles are installed but NOT active. Each one is opt-in.
To turn on ASD-STE100 (concise, direct, complete responses):

  /config  →  Output Style  →  ASD-STE100

Or set it directly for every session:

  "outputStyle": "ASD-STE100"      in ~/.claude/settings.json

Set it per-repo instead by putting the same key in that repo's .claude/settings.json.
OPTIN
