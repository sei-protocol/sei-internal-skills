#!/usr/bin/env bash
# fetch-context.sh — collect shared context once for all judges.
#
# Writes: state/run-<PR>-<SHA>/context.json with:
#   { pr_number, head_sha, diff, body, commits, changed_files: [{path, scope_tags}], memory: {...} }

set -euo pipefail

PR_NUMBER="${PR_NUMBER:?PR_NUMBER required}"
HEAD_SHA="${HEAD_SHA:?HEAD_SHA required}"
STATE_DIR="${STATE_DIR:?STATE_DIR required}"
REPO_ROOT="${REPO_ROOT:-$(git rev-parse --show-toplevel)}"

log() { printf '[%s] fetch-context: %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "${STATE_DIR}/audit.log"; }

log "fetching PR diff, body, commits"
DIFF=$(gh pr diff "${PR_NUMBER}")
BODY=$(gh pr view "${PR_NUMBER}" --json body --jq .body)
COMMITS=$(gh pr view "${PR_NUMBER}" --json commits --jq '[.commits[] | {sha: .oid, message: .messageHeadline}]')
CHANGED_FILES=$(gh pr diff "${PR_NUMBER}" --name-only)

# Tag each changed file with scope tags for judge filtering
TAGGED_FILES="[]"
while IFS= read -r file; do
  [[ -z "${file}" ]] && continue
  TAGS=()
  case "${file}" in
    *.yaml|*.yml) TAGS+=("yaml") ;;
  esac
  case "${file}" in
    *.go) TAGS+=("go") ;;
    *.py) TAGS+=("py") ;;
    *.ts) TAGS+=("ts") ;;
  esac
  case "${file}" in
    CLAUDE.md|AGENTS.md|README.md|docs/*) TAGS+=("durable-doc") ;;
  esac
  case "${file}" in
    .claude/skills/*.md|.claude/skills/*/*.md) TAGS+=("skill-md") ;;
  esac
  case "${file}" in
    clusters/harbor/*) TAGS+=("harbor") ;;
  esac
  TAGS_JSON=$(printf '%s\n' "${TAGS[@]}" | jq -R . | jq -s .)
  TAGGED_FILES=$(echo "${TAGGED_FILES}" | jq --arg path "${file}" --argjson tags "${TAGS_JSON}" '. + [{path: $path, scope_tags: $tags}]')
done <<< "${CHANGED_FILES}"

# Memory snapshot (read feedback_* entries for the 5 active rules)
MEMORY_DIR="${HOME}/.claude/projects/-Users-brandon-tide-workspace-Tide/memory"
MEMORY="{}"
for entry in feedback_no_cpu_limits feedback_harbor_ecr_convention feedback_narration_comments feedback_temporary_migration_notes feedback_authoritative_voice; do
  if [[ -f "${MEMORY_DIR}/${entry}.md" ]]; then
    CONTENT=$(cat "${MEMORY_DIR}/${entry}.md")
    MEMORY=$(echo "${MEMORY}" | jq --arg k "${entry}" --arg v "${CONTENT}" '. + {($k): $v}')
  fi
done

jq -n \
  --arg pr_number "${PR_NUMBER}" \
  --arg head_sha "${HEAD_SHA}" \
  --arg diff "${DIFF}" \
  --arg body "${BODY}" \
  --argjson commits "${COMMITS}" \
  --argjson changed_files "${TAGGED_FILES}" \
  --argjson memory "${MEMORY}" \
  '{pr_number: $pr_number, head_sha: $head_sha, diff: $diff, body: $body, commits: $commits, changed_files: $changed_files, memory: $memory}' \
  > "${STATE_DIR}/context.json"

log "context.json written ($(wc -c < "${STATE_DIR}/context.json") bytes)"
