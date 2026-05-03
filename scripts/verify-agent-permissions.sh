#!/usr/bin/env bash
# verify-agent-permissions.sh — fail if .claude/settings.json contains mutating
# patterns or has drifted from scripts/agent-permissions.json.
#
# Usage:
#   verify-agent-permissions.sh [--canonical-only] [--target <repo-root>]
#
# --canonical-only:  only check the canonical file against the deny-list (used
#                    by update-agent-permissions.sh as a pre-flight).
# --target:          repo root to verify (default: this repo).
#
# Exit codes:
#   0  no violations
#   1  deny-list match or drift from canonical
#   2  invalid usage / missing dependency

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET="$REPO_ROOT"
CANONICAL_ONLY=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --canonical-only) CANONICAL_ONLY=true; shift ;;
    --target) TARGET="$2"; shift 2 ;;
    -h|--help)
      grep '^#' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

CANONICAL="$SCRIPT_DIR/agent-permissions.json"
SETTINGS="$TARGET/.claude/settings.json"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

# Deny-list — substring match against each allow pattern. Any match → violation.
DENY_SUBSTRINGS=(
  'gh issue create'
  'gh issue close'
  'gh issue delete'
  'gh issue edit'
  'gh pr create'
  'gh pr merge'
  'gh pr close'
  'gh pr edit'
  'gh api -X POST'
  'gh api -X PUT'
  'gh api -X DELETE'
  'gh api -X PATCH'
  'gh api --method POST'
  'gh api --method PUT'
  'gh api --method DELETE'
  'gh api --method PATCH'
)

# AWS write verbs — any of these following `aws ` in a pattern → violation.
AWS_WRITE_VERBS=(
  'delete-' 'put-' 'create-' 'update-' 'terminate-'
  'attach-' 'detach-' 'modify-' 'reboot-' 'reset-' 'remove-'
  'register-' 'deregister-' 'associate-' 'disassociate-'
  'authorize-' 'revoke-' 'tag-' 'untag-' 'enable-' 'disable-'
  'start-' 'stop-' 'cancel-' 'restore-' 'send-' 'publish-' 'invoke-'
)

# kubectl read-only subcommands; anything else → violation.
KUBECTL_READ_SUBCMDS='get|describe|logs|top|explain|version|api-resources|api-versions'

# flux read-only subcommands; anything else → violation.
FLUX_READ_SUBCMDS='get|describe|version|check'

violations=()

check_pattern() {
  local pat="$1"
  local source="$2"

  for sub in "${DENY_SUBSTRINGS[@]}"; do
    if [[ "$pat" == *"$sub"* ]]; then
      violations+=("$source: pattern '$pat' contains deny-list substring '$sub'")
      return
    fi
  done

  if [[ "$pat" =~ Bash\(aws[[:space:]] ]]; then
    for verb in "${AWS_WRITE_VERBS[@]}"; do
      if [[ "$pat" == *"$verb"* ]]; then
        violations+=("$source: AWS pattern '$pat' contains write verb '$verb'")
        return
      fi
    done
  fi

  if [[ "$pat" =~ Bash\(kubectl[[:space:]]+([a-z-]+) ]]; then
    local sub="${BASH_REMATCH[1]}"
    if ! [[ "$sub" =~ ^($KUBECTL_READ_SUBCMDS)$ ]]; then
      violations+=("$source: kubectl pattern '$pat' uses non-read subcommand '$sub'")
      return
    fi
  fi

  if [[ "$pat" =~ Bash\(flux[[:space:]]+([a-z-]+) ]]; then
    local sub="${BASH_REMATCH[1]}"
    if ! [[ "$sub" =~ ^($FLUX_READ_SUBCMDS)$ ]]; then
      violations+=("$source: flux pattern '$pat' uses non-read subcommand '$sub'")
      return
    fi
  fi
}

check_file() {
  local file="$1"
  local label="$2"
  if [[ ! -f "$file" ]]; then
    echo "Missing: $file" >&2
    exit 1
  fi
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    check_pattern "$entry" "$label"
  done < <(jq -r '.permissions.allow // [] | .[]' "$file")
}

# Always check canonical
check_file "$CANONICAL" "canonical"

if ! $CANONICAL_ONLY; then
  if [[ -f "$SETTINGS" ]]; then
    check_file "$SETTINGS" ".claude/settings.json"

    # Drift check: settings.json's permissions.allow must equal canonical's exactly.
    SETTINGS_ALLOW=$(jq -S '.permissions.allow' "$SETTINGS")
    CANONICAL_ALLOW=$(jq -S '.permissions.allow' "$CANONICAL")
    if [[ "$SETTINGS_ALLOW" != "$CANONICAL_ALLOW" ]]; then
      violations+=(".claude/settings.json: permissions.allow has drifted from canonical (run 'make update-agent-permissions' or move local additions to .claude/settings.local.json)")
    fi
  else
    echo "Note: $SETTINGS not present; checking canonical only."
  fi
fi

if (( ${#violations[@]} > 0 )); then
  echo "✗ verify-agent-permissions: ${#violations[@]} violation(s)" >&2
  printf '  - %s\n' "${violations[@]}" >&2
  exit 1
fi

echo "✓ verify-agent-permissions: no violations"
