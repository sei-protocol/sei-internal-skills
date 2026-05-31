#!/usr/bin/env bash
# judge-mechanical.sh — runs the per-rule mechanical predicate.
# Usage: judge-mechanical.sh <rule_id>
#
# Stdout: JSON only — { "verdict": "violation"|"no_violation", "findings": [...] }
# Stderr: human-readable logs

set -euo pipefail

RULE="${1:?rule_id required}"
STATE_DIR="${STATE_DIR:?STATE_DIR required}"
CONTEXT="${STATE_DIR}/context.json"

log() { printf '[%s] judge-%s: %s\n' "$(date -u +%FT%TZ)" "${RULE}" "$*" >&2; }

# Walk a YAML file for `cpu:` lines inside a `limits:` block at deeper indent.
# Stateful awk tracking indent + parent block. One deterministic code path.
# Prints every match (no early return); multi-container files surface all violations.
# Per PE cross-review: rejected a PyYAML-based path that mis-cited line numbers
# on multi-container files and silently dropped findings after the first hit.
scan_cpu_limits() {
  awk '
    function indent(s) { match(s, /^ */); return RLENGTH }
    {
      line = $0
      # Strip trailing comment
      sub(/[ \t]+#.*$/, "", line)
      if (line ~ /^[ \t]*limits:[ \t]*$/) {
        limits_indent = indent($0)
        in_limits = 1
        next
      }
      if (in_limits && indent($0) <= limits_indent && length(line) > 0) {
        in_limits = 0
      }
      if (in_limits && match(line, /^[ \t]+cpu:[ \t]*[^[:space:]]/)) {
        print FILENAME ":" NR
      }
    }
  ' "$1"
}

case "${RULE}" in
  no_cpu_limits)
    FINDINGS="[]"
    CHANGED_YAML=$(jq -r '.changed_files[] | select(.scope_tags | contains(["yaml"])) | .path' "${CONTEXT}")
    while IFS= read -r file; do
      [[ -z "${file}" ]] && continue
      [[ ! -f "${file}" ]] && continue
      # Skip Kustomization files — they don't declare container resources
      if grep -qx 'kind: Kustomization' "${file}" 2>/dev/null; then
        continue
      fi
      MATCHES=$(scan_cpu_limits "${file}")
      while IFS= read -r match; do
        [[ -z "${match}" ]] && continue
        FINDINGS=$(echo "${FINDINGS}" | jq --arg span "${match}" --arg cite "no_cpu_limits" \
          '. + [{verdict: "violation", span: $span, citation: $cite, confidence: "high", severity: "warn", mechanism: "mechanical", explanation: "CPU limit set; remove. Set requests only — throttling is an anti-pattern."}]')
      done <<< "${MATCHES}"
    done <<< "${CHANGED_YAML}"
    COUNT=$(echo "${FINDINGS}" | jq 'length')
    log "${COUNT} finding(s)"
    if [[ "${COUNT}" -eq 0 ]]; then
      echo '{"verdict": "no_violation", "findings": []}'
    else
      jq -n --argjson f "${FINDINGS}" '{verdict: "violation", findings: $f}'
    fi
    ;;

  harbor_ecr_convention)
    FINDINGS="[]"
    DIFF=$(jq -r '.diff' "${CONTEXT}")
    # Grep added lines (+) under clusters/harbor/** for ghcr.io.
    # State machine over diff: track current `+++ b/<path>` then matched `+ ...ghcr.io...` lines.
    HARBOR_MATCHES=$(echo "${DIFF}" | awk '
      /^\+\+\+ b\// { sub(/^\+\+\+ b\//, "", $0); current_file = $0; line_num = 0; next }
      /^@@/ { match($0, /\+[0-9]+/); line_num = substr($0, RSTART+1, RLENGTH-1) + 0 - 1; next }
      /^\+/ && !/^\+\+\+/ {
        line_num++
        if (current_file ~ /^clusters\/harbor\// && /ghcr\.io/) print current_file ":" line_num
        next
      }
      /^[- ]/ { line_num++ }
    ')
    while IFS= read -r match; do
      [[ -z "${match}" ]] && continue
      FINDINGS=$(echo "${FINDINGS}" | jq --arg span "${match}" --arg cite "harbor_ecr_convention" \
        '. + [{verdict: "violation", span: $span, citation: $cite, confidence: "high", severity: "warn", mechanism: "mechanical", explanation: "Harbor workload images go to AWS ECR; ghcr.io reference detected."}]')
    done <<< "${HARBOR_MATCHES}"
    COUNT=$(echo "${FINDINGS}" | jq 'length')
    log "${COUNT} finding(s)"
    if [[ "${COUNT}" -eq 0 ]]; then
      echo '{"verdict": "no_violation", "findings": []}'
    else
      jq -n --argjson f "${FINDINGS}" '{verdict: "violation", findings: $f}'
    fi
    ;;

  *)
    log "unknown mechanical rule: ${RULE}"
    echo '{"verdict": "no_violation", "findings": [], "error": "unknown_rule"}'
    exit 1
    ;;
esac
