#!/bin/bash
# Validate GRAFANA_TOKEN against the live endpoint.
# Exits 0 on success, 1 with setup instructions on failure.
set -euo pipefail

BASE_URL="${GRAFANA_BASE_URL:-https://grafana.prod.platform.sei.io}"

if [ -z "${GRAFANA_TOKEN:-}" ]; then
  echo "ERROR: GRAFANA_TOKEN is not set." >&2
  echo "" >&2
  echo "Setup:" >&2
  echo "  1. Open ${BASE_URL} in your browser (OIDC login)" >&2
  echo "  2. Administration → Service Accounts → Add service account" >&2
  echo "  3. Role: Viewer → Create" >&2
  echo "  4. Add service account token → no expiry → Generate → copy" >&2
  echo "  5. export GRAFANA_TOKEN=<token>" >&2
  exit 1
fi

HTTP_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
  "${BASE_URL}/api/org" \
  -H "Authorization: Bearer ${GRAFANA_TOKEN}" 2>/dev/null || echo "000")

if [ "${HTTP_STATUS}" = "200" ]; then
  ORG=$(curl -sf "${BASE_URL}/api/org" -H "Authorization: Bearer ${GRAFANA_TOKEN}" \
    | python3 -c "import json,sys; print(json.load(sys.stdin).get('name','unknown'))" 2>/dev/null)
  echo "Grafana OK — org: ${ORG}"
  exit 0
elif [ "${HTTP_STATUS}" = "401" ]; then
  echo "ERROR: GRAFANA_TOKEN is invalid or expired (HTTP 401)." >&2
  echo "Generate a new token at: ${BASE_URL}/org/serviceaccounts" >&2
  exit 1
else
  echo "ERROR: Grafana unreachable (HTTP ${HTTP_STATUS}). Check VPN / endpoint." >&2
  exit 1
fi
