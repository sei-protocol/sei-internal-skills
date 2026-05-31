#!/usr/bin/env bash
# post-or-update.sh — idempotent comment posting.
#
# Logic:
#   - Find existing bot comment by marker prefix
#   - Extract previous findings-hash from marker
#   - If empty comment + no prior comment       → no-op
#   - If empty comment + prior comment exists   → DELETE prior (clean PR after fixes)
#   - If new comment + no prior                 → CREATE
#   - If new comment + prior + hash matches     → no-op (no churn)
#   - If new comment + prior + hash differs     → PATCH in place

set -euo pipefail

PR_NUMBER="${PR_NUMBER:?PR_NUMBER required}"
STATE_DIR="${STATE_DIR:?STATE_DIR required}"
GH_REPO="${GH_REPO:?GH_REPO required (owner/repo)}"

log() { printf '[%s] post-or-update: %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "${STATE_DIR}/audit.log"; }

COMMENT_FILE="${STATE_DIR}/comment.md"
NEW_BODY=$([ -s "${COMMENT_FILE}" ] && cat "${COMMENT_FILE}" || echo "")
NEW_HASH=$(echo "${NEW_BODY}" | grep -oE 'findings-hash=[a-f0-9]+' | head -1 | cut -d= -f2 || echo "")

# Find prior bot comment by marker prefix
PRIOR=$(gh api "repos/${GH_REPO}/issues/${PR_NUMBER}/comments" \
  --jq '.[] | select(.body | startswith("<!-- tide-pr-quality |")) | {id: .id, body: .body}' \
  | head -1 || echo "")

PRIOR_ID=$(echo "${PRIOR}" | jq -r '.id // empty')
PRIOR_HASH=$(echo "${PRIOR}" | jq -r '.body // empty' | grep -oE 'findings-hash=[a-f0-9]+' | head -1 | cut -d= -f2 || echo "")

if [[ -z "${NEW_BODY}" ]]; then
  if [[ -n "${PRIOR_ID}" ]]; then
    log "no current findings; deleting prior comment ${PRIOR_ID}"
    gh api -X DELETE "repos/${GH_REPO}/issues/comments/${PRIOR_ID}" >/dev/null
  else
    log "no current findings, no prior comment; silence"
  fi
  exit 0
fi

if [[ -z "${PRIOR_ID}" ]]; then
  log "no prior comment; creating new"
  gh pr comment "${PR_NUMBER}" --body-file "${COMMENT_FILE}" >/dev/null
  exit 0
fi

if [[ "${NEW_HASH}" == "${PRIOR_HASH}" ]]; then
  log "findings-hash unchanged (${NEW_HASH}); skipping update (no churn)"
  exit 0
fi

log "findings-hash changed (${PRIOR_HASH} → ${NEW_HASH}); PATCHing prior comment ${PRIOR_ID}"
gh api -X PATCH "repos/${GH_REPO}/issues/comments/${PRIOR_ID}" \
  --field body="${NEW_BODY}" >/dev/null
