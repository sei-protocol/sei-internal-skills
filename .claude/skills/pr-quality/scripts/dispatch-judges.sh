#!/usr/bin/env bash
# dispatch-judges.sh — parallel dispatch of the v1 judge set.
#
# Reads: state/run-<PR>-<SHA>/context.json
# Writes: state/run-<PR>-<SHA>/judges/<rule_id>.json (one per rule)
#
# Caps parallelism at 5. Each judge is invoked via claude-code-action's
# subagent dispatch with the per-rule prompt template loaded from
# .claude/skills/pr-quality/references/judges/<rule_id>.md.
#
# Mechanical rules (no_cpu_limits, harbor_ecr_convention): single deterministic pass.
# LLM-judged rules: n=3 samples at temp=0.3, 2/3 self-consistency.
# Skill dispatch (brevity): subagent loads .claude/skills/brevity/SKILL.md and applies.

set -euo pipefail

STATE_DIR="${STATE_DIR:?STATE_DIR required}"
CONTEXT="${STATE_DIR}/context.json"
JUDGES_DIR="${STATE_DIR}/judges"
mkdir -p "${JUDGES_DIR}"

log() { printf '[%s] dispatch-judges: %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "${STATE_DIR}/audit.log"; }

# Rule registry: the only judges that fire. Adding here is a runtime-bypass; new rules go through PR review of references/rule-registry.md.
RULES=(
  "no_cpu_limits:mechanical"
  "harbor_ecr_convention:mechanical"
  "narration_comments:llm:3"
  "temporary_migration_notes:llm:3"
  "authoritative_voice:llm:3"
  "brevity_dispatch:skill"
)

# Per-rule scope filter — if no changed files match the rule's scope, skip dispatch entirely.
rule_applies() {
  local rule="$1"
  local changed_tags
  changed_tags=$(jq -r '[.changed_files[].scope_tags[]] | unique[]' "${CONTEXT}")
  case "${rule}" in
    no_cpu_limits) grep -qx "yaml" <<< "${changed_tags}" ;;
    harbor_ecr_convention) grep -qx "harbor" <<< "${changed_tags}" ;;
    narration_comments) grep -qxE "go|py|ts" <<< "${changed_tags}" ;;
    temporary_migration_notes) grep -qx "durable-doc" <<< "${changed_tags}" ;;
    authoritative_voice) grep -qx "skill-md" <<< "${changed_tags}" ;;
    brevity_dispatch) return 0 ;;  # PR body always present
    *) return 1 ;;
  esac
}

# Track cost; halt if over $1.00. (Implementation detail of the claude-code-action runtime; this script logs.)
COST_CAP_USD="1.00"

# Background dispatch with parallelism cap of 5
PIDS=()
for rule_spec in "${RULES[@]}"; do
  IFS=':' read -r rule kind n <<< "${rule_spec}"
  if ! rule_applies "${rule}"; then
    log "rule ${rule}: scope does not apply; skipping"
    echo '{"verdict": "no_violation", "skipped": "scope_filter"}' > "${JUDGES_DIR}/${rule}.json"
    continue
  fi
  log "dispatching judge: ${rule} (${kind}${n:+, n=${n}})"
  (
    case "${kind}" in
      mechanical) bash "$(dirname "$0")/judge-mechanical.sh" "${rule}" ;;
      llm)        bash "$(dirname "$0")/judge-llm.sh" "${rule}" "${n}" ;;
      skill)      bash "$(dirname "$0")/judge-skill-dispatch.sh" "${rule}" ;;
    esac
  ) > "${JUDGES_DIR}/${rule}.json" 2>>"${STATE_DIR}/audit.log" &
  PIDS+=($!)
  # Cap parallelism at 5
  while [[ ${#PIDS[@]} -ge 5 ]]; do
    wait -n
    NEW_PIDS=()
    for pid in "${PIDS[@]}"; do
      if kill -0 "${pid}" 2>/dev/null; then NEW_PIDS+=("${pid}"); fi
    done
    PIDS=("${NEW_PIDS[@]}")
  done
done
wait

log "all judges complete: $(ls "${JUDGES_DIR}"/*.json 2>/dev/null | wc -l) outputs"
