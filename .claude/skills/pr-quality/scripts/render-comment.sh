#!/usr/bin/env bash
# render-comment.sh — produce the PR comment markdown per format-spec.
#
# Reads: state/run-<PR>-<SHA>/aggregated.json, HEAD_SHA env
# Writes: state/run-<PR>-<SHA>/comment.md
#
# If post_cap_count == 0, writes empty file (post-or-update.sh will treat this as "no findings").

set -euo pipefail

STATE_DIR="${STATE_DIR:?STATE_DIR required}"
HEAD_SHA="${HEAD_SHA:?HEAD_SHA required}"

log() { printf '[%s] render-comment: %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "${STATE_DIR}/audit.log"; }

AGG="${STATE_DIR}/aggregated.json"
COUNT=$(jq -r '.post_cap_count' "${AGG}")

if [[ "${COUNT}" -eq 0 ]]; then
  : > "${STATE_DIR}/comment.md"
  log "zero findings; empty comment file written"
  exit 0
fi

# Compute findings-hash for idempotency
FINDINGS_HASH=$(jq -c '.findings' "${AGG}" | sha256sum | cut -d' ' -f1)
SUPPRESSED=$(jq -r '.suppressed_count' "${AGG}")
SUPPRESSED_RULES=$(jq -r '.suppressed_rules | join(", ")' "${AGG}")

{
  echo "<!-- tide-pr-quality | sha=${HEAD_SHA} | findings-hash=${FINDINGS_HASH} -->"
  echo "### PR Quality — ${COUNT} finding(s)"
  echo
  jq -r '.findings[] | "- `\(.span)` — \(.explanation)\n  Rule: [`\(.citation)`](.claude/memory/feedback_\(.citation).md) — see memory entry."' "${AGG}"
  if [[ "${SUPPRESSED}" -gt 0 ]]; then
    echo
    echo "<details><summary>+${SUPPRESSED} additional lower-severity findings suppressed (${SUPPRESSED_RULES})</summary>"
    echo
    echo "Full set in CI artifact \`pr-quality-artifacts\`."
    echo "</details>"
  fi
  echo
  echo "---"
  echo
  echo "Suggestive only; humans decide. Opt out via label \`skip-pr-quality\`."
} > "${STATE_DIR}/comment.md"

log "comment.md rendered ($(wc -c < "${STATE_DIR}/comment.md") bytes, ${SUPPRESSED} suppressed)"
