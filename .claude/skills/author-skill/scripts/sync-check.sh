#!/usr/bin/env bash
# sync-check.sh — Check whether the new skill should be added to scripts/sync-skills.sh.
#
# Usage:
#   sync-check.sh --name <skill-name> --category <portable|sei|none> [--dry-run]
#
# Behavior:
#   - For category=none: report and exit 0.
#   - For category=portable: add to PORTABLE=( ... ) in scripts/sync-skills.sh.
#   - For category=sei: add to SEI=( ... ) in scripts/sync-skills.sh.
#   - On --dry-run, show the proposed diff but do not write.
#   - Refuses if the skill is already in the list (exit 3).

set -euo pipefail

NAME=""
CATEGORY=""
DRY_RUN=0

die() { printf 'sync-check.sh: %s\n' "$1" >&2; exit "${2:-1}"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)     NAME="$2"; shift 2 ;;
    --category) CATEGORY="$2"; shift 2 ;;
    --dry-run)  DRY_RUN=1; shift ;;
    *) die "unknown flag: $1" 1 ;;
  esac
done

[[ -n "$NAME" ]]     || die "--name is required" 1
[[ -n "$CATEGORY" ]] || die "--category is required (portable|sei|none)" 1

if [[ "$CATEGORY" == "none" ]]; then
  printf 'sync-check: skill %s marked as non-portable; no sync list entry added.\n' "$NAME"
  exit 0
fi

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not in a git repo" 1
SYNC="$REPO_ROOT/scripts/sync-skills.sh"
[[ -f "$SYNC" ]] || die "sync-skills.sh not found at $SYNC" 1

case "$CATEGORY" in
  portable) ARRAY_NAME="PORTABLE" ;;
  sei)      ARRAY_NAME="SEI"      ;;
  *) die "--category must be portable, sei, or none" 1 ;;
esac

# Already listed?
if awk -v arr="$ARRAY_NAME" -v skill="$NAME" '
  BEGIN { inarr = 0 }
  $0 ~ "^" arr "=\\(" { inarr = 1; next }
  inarr && /^\)/      { inarr = 0 }
  inarr && $1 == skill { found = 1 }
  END { exit !found }
' "$SYNC"; then
  die "skill '$NAME' already in $ARRAY_NAME=( ... ) in $SYNC" 3
fi

# Build a tempfile with the skill added before the closing paren of the array
TMP="$(mktemp)"
awk -v arr="$ARRAY_NAME" -v skill="$NAME" '
  BEGIN { inarr = 0; inserted = 0 }
  $0 ~ "^" arr "=\\(" { inarr = 1; print; next }
  inarr && /^\)/ {
    if (!inserted) {
      print "  " skill
      inserted = 1
    }
    inarr = 0
    print
    next
  }
  { print }
' "$SYNC" > "$TMP"

if [[ "$DRY_RUN" == "1" ]]; then
  printf 'DRY RUN — proposed diff to %s (%s array):\n\n' "$SYNC" "$ARRAY_NAME"
  diff -u "$SYNC" "$TMP" || true
  rm -f "$TMP"
  exit 0
fi

# Stream contents into the existing file so its mode (and inode) are preserved.
# mv from a mktemp tempfile would clobber the destination's permissions on macOS/Linux.
cat "$TMP" > "$SYNC"
rm -f "$TMP"
printf 'Added %s to %s=( ... ) in %s\n' "$NAME" "$ARRAY_NAME" "$SYNC"
