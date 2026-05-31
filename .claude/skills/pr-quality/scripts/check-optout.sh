#!/usr/bin/env bash
# check-optout.sh — halt if PR has the skip-pr-quality label.
#
# Reads: gh pr view --json labels
# Exits: 0 (continue) or 0 with log (skip)

set -euo pipefail

PR_NUMBER="${PR_NUMBER:?PR_NUMBER required}"
STATE_DIR="${STATE_DIR:?STATE_DIR required}"

log() { printf '[%s] check-optout: %s\n' "$(date -u +%FT%TZ)" "$*" | tee -a "${STATE_DIR}/audit.log"; }

LABELS=$(gh pr view "${PR_NUMBER}" --json labels --jq '.labels[].name' 2>/dev/null || true)

if printf '%s\n' "${LABELS}" | grep -qx "skip-pr-quality"; then
  log "skip-pr-quality label present; exiting cleanly"
  echo "PR_QUALITY_SKIP=1" >> "${GITHUB_ENV:-/dev/null}"
  exit 0
fi

log "no opt-out label; continuing"
