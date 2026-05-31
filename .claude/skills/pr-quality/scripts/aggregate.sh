#!/usr/bin/env bash
# aggregate.sh — dedup + severity-rank + 5-cap.
#
# Reads: state/run-<PR>-<SHA>/judges/*.json
# Writes: state/run-<PR>-<SHA>/aggregated.json
#   { findings: [...], total_count, post_cap_count, suppressed_count, suppressed_rules: [...] }

set -euo pipefail

STATE_DIR="${STATE_DIR:?STATE_DIR required}"
JUDGES_DIR="${STATE_DIR}/judges"

log() { printf '[%s] aggregate: %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "${STATE_DIR}/audit.log"; }

# Collect all violations from all judges, normalize shape
ALL=$(jq -s '[.[] | select(.verdict == "violation") | .findings // [.]] | add // []' "${JUDGES_DIR}"/*.json)

# Dedup by (file, line, rule_id); mechanical wins on tie
DEDUPED=$(echo "${ALL}" | jq '
  group_by(.span + "|" + .citation)
  | map(sort_by(if .mechanism == "mechanical" then 0 else 1 end) | .[0])
')

# Severity rank: warn-mechanical, warn-llm, nudge
RANKED=$(echo "${DEDUPED}" | jq '
  sort_by(
    (if .severity == "warn" then 0 else 1 end),
    (if .mechanism == "mechanical" then 0 else 1 end),
    .span
  )
')

TOTAL=$(echo "${RANKED}" | jq 'length')
CAP=5
POST_CAP=$([[ "${TOTAL}" -gt "${CAP}" ]] && echo "${CAP}" || echo "${TOTAL}")
SUPPRESSED=$([[ "${TOTAL}" -gt "${CAP}" ]] && echo "$((TOTAL - CAP))" || echo "0")
SUPPRESSED_RULES=$(echo "${RANKED}" | jq --argjson cap "${CAP}" '[.[$cap:] | .[].citation] | unique')
TOP=$(echo "${RANKED}" | jq --argjson cap "${CAP}" '.[:$cap]')

jq -n \
  --argjson findings "${TOP}" \
  --argjson total "${TOTAL}" \
  --argjson post_cap "${POST_CAP}" \
  --argjson suppressed "${SUPPRESSED}" \
  --argjson suppressed_rules "${SUPPRESSED_RULES}" \
  '{findings: $findings, total_count: $total, post_cap_count: $post_cap, suppressed_count: $suppressed, suppressed_rules: $suppressed_rules}' \
  > "${STATE_DIR}/aggregated.json"

log "aggregated: ${POST_CAP}/${TOTAL} surfaced, ${SUPPRESSED} suppressed"
