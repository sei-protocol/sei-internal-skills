#!/usr/bin/env bash
# update-agent-permissions.sh — install canonical read-only allow-list into .claude/settings.json.
#
# Usage:
#   update-agent-permissions.sh [--target <repo-root>]
#
# Env:
#   DRY_RUN=1   Print the proposed result and exit without writing.
#
# Behavior:
#   - Reads scripts/agent-permissions.json (canonical) and writes into <repo>/.claude/settings.json.
#   - If .claude/settings.json already has top-level keys other than `permissions`, those are preserved.
#   - The full `permissions.allow` list is replaced with the canonical set.
#   - Local additions in the existing allow list that aren't in canonical produce a warning that
#     suggests moving them to .claude/settings.local.json (gitignored).
#   - Refuses to run if any pattern in canonical matches the deny-list (defensive — verify-agent-permissions
#     is the canonical enforcement; this is a belt-and-suspenders check before writing).
#   - Idempotent: re-running produces no diff if already current.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET="$REPO_ROOT"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target) TARGET="$2"; shift 2 ;;
    -h|--help)
      grep '^#' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

CANONICAL="$SCRIPT_DIR/agent-permissions.json"
SETTINGS="$TARGET/.claude/settings.json"
DRY_RUN="${DRY_RUN:-0}"

if [[ ! -f "$CANONICAL" ]]; then
  echo "Canonical file missing: $CANONICAL" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required (brew install jq)" >&2
  exit 1
fi

# Pre-flight: deny-list check on canonical itself
if ! "$SCRIPT_DIR/verify-agent-permissions.sh" --canonical-only >/dev/null 2>&1; then
  echo "Canonical file fails deny-list check; refusing to write." >&2
  echo "Run 'scripts/verify-agent-permissions.sh --canonical-only' for details." >&2
  exit 1
fi

mkdir -p "$TARGET/.claude"

if [[ -f "$SETTINGS" ]]; then
  EXISTING_ALLOW=$(jq -r '.permissions.allow // [] | .[]' "$SETTINGS" 2>/dev/null | sort -u)
  CANONICAL_ALLOW=$(jq -r '.permissions.allow | .[]' "$CANONICAL" | sort -u)
  LOCAL_ADDITIONS=$(comm -23 <(echo "$EXISTING_ALLOW") <(echo "$CANONICAL_ALLOW") || true)
  if [[ -n "${LOCAL_ADDITIONS//[[:space:]]/}" ]]; then
    echo "Warning: existing .claude/settings.json has entries not in canonical:" >&2
    echo "$LOCAL_ADDITIONS" | sed 's/^/  - /' >&2
    echo "Consider moving these to .claude/settings.local.json (gitignored)." >&2
  fi

  # Merge canonical permissions into existing settings (preserve other top-level keys)
  RESULT=$(jq -s '.[0] * .[1]' "$SETTINGS" "$CANONICAL")
else
  RESULT=$(cat "$CANONICAL")
fi

if [[ -f "$SETTINGS" ]] && diff -q <(echo "$RESULT") "$SETTINGS" >/dev/null 2>&1; then
  echo "✓ $SETTINGS is already up to date"
  exit 0
fi

if [[ "$DRY_RUN" == "1" ]]; then
  echo "--- proposed $SETTINGS ---"
  echo "$RESULT"
  exit 0
fi

echo "$RESULT" > "$SETTINGS"
echo "✓ wrote $SETTINGS"
