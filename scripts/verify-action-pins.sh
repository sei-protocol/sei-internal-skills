#!/usr/bin/env bash
# verify-action-pins.sh — fail if a workflow `uses:` a tag or branch instead of a
# commit sha. A tag is mutable, so the same workflow can run different code
# tomorrow; four workflows here assume an AWS role and push to ECR.
#
# Usage:
#   verify-action-pins.sh [--target <repo-root>]
#
# Checks `uses:` as a YAML key in .github/workflows/*.yml. A local `./` action
# and a `docker://` image carry no sha and are exempt. A `uses:` inside a `run:`
# block would read as a violation; none exists, and a false positive here is a
# review prompt, not a wrong pin.
#
# Exit codes:
#   0  every ref is a 40-hex commit sha
#   1  at least one tag or branch ref
#   2  invalid usage

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="$(cd "$SCRIPT_DIR/.." && pwd)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target) TARGET="$2"; shift 2 ;;
    -h|--help)
      grep '^#' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

WORKFLOWS="$TARGET/.github/workflows"
if [[ ! -d "$WORKFLOWS" ]]; then
  echo "Missing: $WORKFLOWS" >&2
  exit 2
fi

# A `uses:` key, optionally the first item of a list. A leading `#` fails `^[[:space:]]*`,
# so the documented example in writing-contract.yml is not a ref.
USES_KEY='^[[:space:]]*(-[[:space:]]+)?uses:[[:space:]]*'

violations=()
checked=0

while IFS= read -r line; do
  file="${line%%:*}"; rest="${line#*:}"
  lineno="${rest%%:*}"; text="${rest#*:}"
  ref="$(sed -E "s|$USES_KEY||" <<<"$text" | awk '{print $1}')"
  case "$ref" in
    ./*|docker://*) continue ;;
  esac
  checked=$((checked + 1))
  if ! [[ "${ref##*@}" =~ ^[0-9a-f]{40}$ ]]; then
    violations+=("$(basename "$file"):$lineno: '$ref' is not pinned to a commit sha")
  fi
done < <(grep -rnE "$USES_KEY" "$WORKFLOWS" --include='*.yml' || true)

if (( ${#violations[@]} > 0 )); then
  echo "✗ verify-action-pins: ${#violations[@]} unpinned ref(s) of $checked" >&2
  printf '  - %s\n' "${violations[@]}" >&2
  echo "  Resolve the tag with: gh api repos/<owner>/<repo>/git/ref/tags/<tag>" >&2
  echo "  then write: uses: <owner>/<repo>@<sha> # <tag>" >&2
  exit 1
fi

echo "✓ verify-action-pins: $checked refs, every one a commit sha"
