#!/usr/bin/env bash
# judge-skill-dispatch.sh — composes with another skill by dispatching a subagent.
# Usage: judge-skill-dispatch.sh <dispatch_target>
#
# v1 only target: brevity_dispatch (loads .claude/skills/brevity/SKILL.md, applies to PR body + comment additions).
#
# Stdout: JSON only.
# Stderr: human-readable logs.

set -euo pipefail

TARGET="${1:?dispatch_target required}"
STATE_DIR="${STATE_DIR:?STATE_DIR required}"
CONTEXT="${STATE_DIR}/context.json"

log() { printf '[%s] judge-skill-%s: %s\n' "$(date -u +%FT%TZ)" "${TARGET}" "$*" >&2; }

case "${TARGET}" in
  brevity_dispatch)
    SKILL_PATH=".claude/skills/brevity/SKILL.md"
    if [[ ! -f "${SKILL_PATH}" ]]; then
      log "brevity skill missing at ${SKILL_PATH}"
      echo '{"verdict": "no_violation", "findings": [], "error": "missing_target_skill"}'
      exit 1
    fi
    BODY=$(jq -r '.body' "${CONTEXT}")
    BODY_WORDS=$(echo "${BODY}" | wc -w | tr -d ' ')
    # Quick heuristic: if PR body > 250 words, flag for brevity review.
    # The LLM-dispatched subagent does the real judgment with /brevity loaded.
    if [[ "${BODY_WORDS}" -lt 50 ]]; then
      log "PR body ${BODY_WORDS} words; below brevity floor; no finding"
      echo '{"verdict": "no_violation", "findings": []}'
      exit 0
    fi
    # Emit dispatch marker for the runtime to invoke the brevity skill on the body.
    jq -n \
      --arg target "${TARGET}" \
      --arg skill_path "${SKILL_PATH}" \
      --arg body "${BODY}" \
      --argjson body_words "${BODY_WORDS}" \
      '{
        _skill_dispatch: {
          target: $target,
          skill_path: $skill_path,
          input_kind: "pr_body",
          input: $body,
          input_word_count: $body_words,
          rule_id_on_violation: "brevity_dispatch",
          severity_on_violation: "nudge",
          mechanism: "skill-dispatch"
        }
      }'
    ;;

  *)
    log "unknown dispatch target: ${TARGET}"
    echo '{"verdict": "no_violation", "findings": [], "error": "unknown_target"}'
    exit 1
    ;;
esac
