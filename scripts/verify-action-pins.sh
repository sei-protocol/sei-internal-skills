#!/usr/bin/env bash
# verify-action-pins.sh — fail if a `uses:` ref under .github/ names a tag or a
# branch instead of a commit sha. A tag is mutable, so the same workflow can run
# different code tomorrow, and four workflows here assume an AWS role and push to ECR.
#
# Usage:
#   verify-action-pins.sh [--target <repo-root>]
#
# Reads every *.yml and *.yaml under .github/, so a composite action counts too.
# A local `./` action is exempt. A `docker://` image needs an @sha256: digest,
# which is the same rule under another name.
#
# Exit codes:
#   0  every ref is a commit sha, an image digest, or a local action
#   1  at least one mutable ref
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

ROOT="$TARGET/.github"
if [[ ! -d "$ROOT" ]]; then
  echo "Missing: $ROOT" >&2
  exit 2
fi

# A `uses:` key, quoted or bare, optionally the first item of a list. A leading `#`
# fails `^[[:space:]]*`, so the documented example in writing-contract.yml is not a
# ref. What a line scanner cannot see: a flow-style mapping (`{uses: x@v1}`), and a
# `uses:` at the start of a line inside a `run:` block, which reads as a violation.
USES_KEY='^[[:space:]]*(-[[:space:]]+)?"?uses"?:[[:space:]]*'

violations=()
checked=0

while IFS= read -r line; do
  file="${line%%:*}"; rest="${line#*:}"
  lineno="${rest%%:*}"; text="${rest#*:}"
  ref="$(sed -E "s|$USES_KEY||" <<<"$text" | awk '{print $1}')"
  ref="${ref%\"}"; ref="${ref#\"}"; ref="${ref%\'}"; ref="${ref#\'}"
  where="${file#"$ROOT"/}:$lineno"

  case "$ref" in
    ./*) continue ;;
    docker://*)
      checked=$((checked + 1))
      if ! [[ "$ref" =~ ^docker://[^@]+@sha256:[0-9a-f]{64}$ ]]; then
        violations+=("$where: '$ref' is not pinned to an image digest")
      fi
      continue ;;
  esac

  checked=$((checked + 1))
  if ! [[ "${ref##*@}" =~ ^[0-9a-f]{40}$ ]]; then
    violations+=("$where: '$ref' is not pinned to a commit sha")
  fi
done < <(grep -rnE "$USES_KEY" "$ROOT" --include='*.yml' --include='*.yaml' || true)

if (( ${#violations[@]} > 0 )); then
  echo "✗ verify-action-pins: ${#violations[@]} mutable ref(s) of $checked" >&2
  printf '  - %s\n' "${violations[@]}" >&2
  echo "  Resolve the tag with: gh api repos/<owner>/<repo>/git/ref/tags/<tag>" >&2
  echo "  then write: uses: <owner>/<repo>@<sha> # <tag>" >&2
  exit 1
fi

echo "✓ verify-action-pins: $checked refs, every one pinned"
