#!/usr/bin/env bash
# check-pr-size.sh — halt on empty/oversized/self-edit PRs.
#
# Exits 0 with log if:
#   - diff is empty
#   - diff only touches .github/workflows/pr-quality.yml
#   - diff > 5000 lines
# Else continues.

set -euo pipefail

PR_NUMBER="${PR_NUMBER:?PR_NUMBER required}"
STATE_DIR="${STATE_DIR:?STATE_DIR required}"

log() { printf '[%s] check-pr-size: %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "${STATE_DIR}/audit.log"; }

CHANGED_FILES=$(gh pr diff "${PR_NUMBER}" --name-only 2>/dev/null || true)
DIFF_LINES=$(gh pr diff "${PR_NUMBER}" 2>/dev/null | wc -l | tr -d ' ')

if [[ -z "${CHANGED_FILES}" || "${DIFF_LINES}" -eq 0 ]]; then
  log "empty diff; exiting cleanly"
  echo "PR_QUALITY_SKIP=1" >> "${GITHUB_ENV:-/dev/null}"
  exit 0
fi

# Self-edit check: only file changed is the workflow itself
if [[ "$(printf '%s\n' "${CHANGED_FILES}" | wc -l | tr -d ' ')" -eq 1 ]] && \
   [[ "${CHANGED_FILES}" == ".github/workflows/pr-quality.yml" ]]; then
  log "PR only touches the bot's own workflow; bot does not review itself; exiting cleanly"
  echo "PR_QUALITY_SKIP=1" >> "${GITHUB_ENV:-/dev/null}"
  exit 0
fi

if [[ "${DIFF_LINES}" -gt 5000 ]]; then
  log "PR exceeds 5000 changed lines (${DIFF_LINES}); judge precision degrades; deferring to human review"
  echo "PR_QUALITY_SKIP=1" >> "${GITHUB_ENV:-/dev/null}"
  exit 0
fi

log "PR size OK: ${DIFF_LINES} lines, $(printf '%s\n' "${CHANGED_FILES}" | wc -l | tr -d ' ') files"
