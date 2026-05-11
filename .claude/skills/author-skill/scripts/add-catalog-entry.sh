#!/usr/bin/env bash
# add-catalog-entry.sh — Append a catalog entry to .claude/skills/README.md.
#
# Usage:
#   add-catalog-entry.sh --name <skill-name> --section <Workflow|Workstream Bootstrap|Hardening|Release Operations|Engineer Self-Service|Future Slots> --tagline "<one-line description>" [--dry-run]
#
# Behavior:
#   - Locates the README.md catalog.
#   - Identifies the target section by its H3 heading.
#   - Inserts a new bullet at the end of the section's bullet list.
#   - On --dry-run, prints the proposed insertion + the section's surrounding context but does not write.
#   - Refuses if the section heading is not found (exit 2).
#   - Refuses if the skill is already listed in the catalog (exit 3).

set -euo pipefail

NAME=""
SECTION=""
TAGLINE=""
DRY_RUN=0

die() { printf 'add-catalog-entry.sh: %s\n' "$1" >&2; exit "${2:-1}"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)     NAME="$2"; shift 2 ;;
    --section)  SECTION="$2"; shift 2 ;;
    --tagline)  TAGLINE="$2"; shift 2 ;;
    --dry-run)  DRY_RUN=1; shift ;;
    *) die "unknown flag: $1" 1 ;;
  esac
done

[[ -n "$NAME" ]]    || die "--name is required" 1
[[ -n "$SECTION" ]] || die "--section is required" 1
[[ -n "$TAGLINE" ]] || die "--tagline is required" 1

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not in a git repo" 1
README="$REPO_ROOT/.claude/skills/README.md"
[[ -f "$README" ]] || die "catalog README not found: $README" 1

# Already listed?
if grep -E "^\- \*\*\`$NAME/\`\*\*" "$README" >/dev/null 2>&1; then
  die "skill '$NAME' is already listed in $README" 3
fi

# Build the entry
ENTRY="- **\`$NAME/\`** — $TAGLINE"

# Locate the section heading and find the right insertion point:
#   - section starts at "### <SECTION>"
#   - section ends at the next "### " or end of file
#   - insert at the last bulleted line within the section
SECTION_LINE=$(grep -n "^### $SECTION\$" "$README" | head -1 | cut -d: -f1 || true)
[[ -n "$SECTION_LINE" ]] || die "section '### $SECTION' not found in $README" 2

# Find next "### " heading after SECTION_LINE
NEXT_HEADING_LINE=$(awk -v start="$SECTION_LINE" 'NR > start && /^### / { print NR; exit }' "$README")
if [[ -z "$NEXT_HEADING_LINE" ]]; then
  NEXT_HEADING_LINE=$(( $(wc -l < "$README") + 1 ))
fi

# Find the last bulleted line in [SECTION_LINE+1, NEXT_HEADING_LINE-1]
LAST_BULLET_LINE=$(awk -v start="$SECTION_LINE" -v end="$NEXT_HEADING_LINE" '
  NR > start && NR < end && /^- / { last = NR }
  END { print last }
' "$README")

if [[ -z "$LAST_BULLET_LINE" || "$LAST_BULLET_LINE" == "0" ]]; then
  # No existing bullets in the section — insert right after the heading + blank line
  INSERT_AFTER=$(( SECTION_LINE + 1 ))
else
  INSERT_AFTER="$LAST_BULLET_LINE"
fi

if [[ "$DRY_RUN" == "1" ]]; then
  printf 'DRY RUN — would insert after line %s in %s:\n\n%s\n\n' "$INSERT_AFTER" "$README" "$ENTRY"
  printf 'Section context:\n'
  sed -n "${SECTION_LINE},${INSERT_AFTER}p" "$README"
  exit 0
fi

# Insert. Use awk to avoid in-place sed portability issues across macOS/Linux.
TMP="$(mktemp)"
awk -v ins_after="$INSERT_AFTER" -v entry="$ENTRY" '
  { print }
  NR == ins_after { print entry }
' "$README" > "$TMP"
mv "$TMP" "$README"

printf 'Added catalog entry for %s under "%s" in %s\n' "$NAME" "$SECTION" "$README"
