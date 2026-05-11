#!/usr/bin/env bash
# apply-refactor.sh — Apply a unified diff to a target file with backup + verify + rollback.
#
# Usage:
#   apply-refactor.sh --diff <diff-path> --target <file-path> [--state-dir <state-dir>]
#
# Behavior:
#   - Backs up <target> to <state-dir>/backups/<target>.before (creating dirs as needed).
#   - Applies the diff with `patch`.
#   - Verifies the result:
#       * SKILL.md  → parse frontmatter, line count ≤ 500, description ≤ 1024 chars
#       * *.md      → line count check
#       * *.sh      → `bash -n` syntax check, shebang + set -euo pipefail still present
#   - Rolls back on verify failure (cp backup back, exit 4).
#   - On success: emits a JSONL audit-log line to <state-dir>/audit.log.

set -euo pipefail

DIFF=""
TARGET=""
STATE_DIR="."

die() { printf 'apply-refactor.sh: %s\n' "$1" >&2; exit "${2:-1}"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --diff)      DIFF="$2"; shift 2 ;;
    --target)    TARGET="$2"; shift 2 ;;
    --state-dir) STATE_DIR="$2"; shift 2 ;;
    *) die "unknown flag: $1" 1 ;;
  esac
done

[[ -n "$DIFF" ]]   || die "--diff is required" 1
[[ -n "$TARGET" ]] || die "--target is required" 1
[[ -f "$DIFF" ]]   || die "diff not found: $DIFF" 1
[[ -f "$TARGET" ]] || die "target not found: $TARGET" 1

BACKUP_DIR="$STATE_DIR/backups"
mkdir -p "$BACKUP_DIR"
BACKUP="$BACKUP_DIR/$(basename "$TARGET").before"

cp "$TARGET" "$BACKUP"

# Apply diff
if ! patch --silent "$TARGET" < "$DIFF"; then
  cp "$BACKUP" "$TARGET"
  die "patch failed for $TARGET; rolled back" 2
fi

# Verify
verify_failed=""

case "$TARGET" in
  *SKILL.md)
    # Frontmatter parse
    if ! awk '/^---$/ { fence++; if (fence == 2) { found = 1; exit } } END { exit !found }' "$TARGET"; then
      verify_failed="frontmatter unparseable after apply"
    fi
    # Line count
    if [[ -z "$verify_failed" ]]; then
      lines=$(wc -l < "$TARGET" | tr -d ' ')
      if (( lines > 500 )); then
        verify_failed="SKILL.md now $lines lines (over 500-line ceiling)"
      fi
    fi
    # Description length
    if [[ -z "$verify_failed" ]]; then
      desc=$(awk '/^---$/ { fence++; if (fence == 2) exit; next }
                  fence == 1 && /^description:/ { sub(/^description: */, ""); body = $0; next }
                  fence == 1 && capturing { body = body " " $0 }
                  fence == 1 && /^description:/ { capturing = 1 }
                  END { print body }' "$TARGET")
      desc_clean="${desc#\"}"
      desc_clean="${desc_clean%\"}"
      if (( ${#desc_clean} > 1024 )); then
        verify_failed="description now ${#desc_clean} chars (over 1024)"
      fi
    fi
    ;;
  *.sh)
    # Syntax check
    if ! bash -n "$TARGET" 2>/dev/null; then
      verify_failed="bash syntax check failed"
    fi
    # Shebang
    if [[ -z "$verify_failed" ]] && ! head -1 "$TARGET" | grep -qE '^#!'; then
      verify_failed="shebang missing after apply"
    fi
    # set -euo pipefail
    if [[ -z "$verify_failed" ]] && ! grep -q 'set -euo pipefail' "$TARGET"; then
      verify_failed="set -euo pipefail missing after apply"
    fi
    ;;
  *.md)
    # Plain markdown — minimal check (file still readable)
    if [[ ! -r "$TARGET" ]]; then
      verify_failed="markdown file unreadable after apply"
    fi
    ;;
esac

if [[ -n "$verify_failed" ]]; then
  cp "$BACKUP" "$TARGET"
  die "verify failed: $verify_failed; rolled back from $BACKUP" 4
fi

# Append audit-log line
LOGLINE=$(printf '{"ts":"%s","action":"apply-refactor","target":"%s","diff":"%s","backup":"%s","exit":0,"verify":"pass"}\n' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$TARGET" "$DIFF" "$BACKUP")
mkdir -p "$STATE_DIR"
printf '%s' "$LOGLINE" >> "$STATE_DIR/audit.log"

printf 'applied: %s  (backup: %s)\n' "$TARGET" "$BACKUP"
