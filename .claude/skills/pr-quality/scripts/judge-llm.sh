#!/usr/bin/env bash
# judge-llm.sh — runs an LLM-judged rule with n=3 self-consistency.
# Usage: judge-llm.sh <rule_id> <n_samples>
#
# Stdout: JSON only — { "verdict": "violation"|"no_violation", "findings": [...] }
# Stderr: human-readable logs
#
# This script is the contract surface. Actual LLM dispatch is done by the
# claude-code-action runtime, which loads .claude/skills/pr-quality/references/judges/<rule_id>.md
# as the system prompt + few-shot examples and samples n times at temp=0.3.
#
# Each sample returns a per-judge JSON; the runner aggregates 2/3 agreement and emits
# a single finding per (file, line) tuple if the sample voted "violation" at 2/3 or 3/3.
#
# Stdout JSON shape (final, after aggregation):
#   { "verdict": "violation"|"no_violation", "findings": [{span, citation, confidence, severity, mechanism, explanation}] }

set -euo pipefail

RULE="${1:?rule_id required}"
N="${2:?n_samples required}"
STATE_DIR="${STATE_DIR:?STATE_DIR required}"
CONTEXT="${STATE_DIR}/context.json"
JUDGE_PROMPT="$(dirname "$0")/../references/judges/${RULE}.md"

log() { printf '[%s] judge-%s: %s\n' "$(date -u +%FT%TZ)" "${RULE}" "$*" >&2; }

if [[ ! -f "${JUDGE_PROMPT}" ]]; then
  log "judge prompt missing: ${JUDGE_PROMPT}"
  echo '{"verdict": "no_violation", "findings": [], "error": "missing_judge_prompt"}'
  exit 1
fi

# The actual LLM invocation is handled by claude-code-action. In CI the runtime
# expects this script to print structured JSON synthesized from n=${N} samples.
# In v1 we delegate the sampling to the action's built-in dispatch using a marker
# protocol: stdout JSON includes a `_llm_dispatch` block that the runtime expands
# into n samples and replaces with aggregated findings.
#
# For local dev / dry-run, set PR_QUALITY_LOCAL=1 to get a no-op no_violation.

if [[ "${PR_QUALITY_LOCAL:-0}" == "1" ]]; then
  log "PR_QUALITY_LOCAL=1; returning no_violation (no LLM dispatch in local mode)"
  echo '{"verdict": "no_violation", "findings": []}'
  exit 0
fi

# Emit the dispatch marker that the runtime expands.
# Use --rawfile (streams from disk) rather than --arg (passed through argv) so
# large PR diffs don't blow ARG_MAX (~128KB on Linux; PRs >1MB are rare but real).
DIFF_FILE="${STATE_DIR}/diff.txt"
MEMORY_FILE="${STATE_DIR}/memory-${RULE}.txt"

jq -r '.diff' "${CONTEXT}" > "${DIFF_FILE}"
jq -r --arg k "feedback_${RULE}" '.memory[$k] // ""' "${CONTEXT}" > "${MEMORY_FILE}"

jq -n \
  --arg rule "${RULE}" \
  --argjson n "${N}" \
  --rawfile prompt "${JUDGE_PROMPT}" \
  --rawfile diff "${DIFF_FILE}" \
  --rawfile memory "${MEMORY_FILE}" \
  '{
    _llm_dispatch: {
      rule: $rule,
      n: $n,
      temperature: 0.3,
      consistency_threshold: 2,
      prompt: $prompt,
      memory: $memory,
      diff: $diff
    }
  }'
