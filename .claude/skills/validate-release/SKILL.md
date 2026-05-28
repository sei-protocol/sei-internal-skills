---
name: validate-release
description: "Collects chaos suite run results from S3 and Thanos, derives per-scenario metrics and Grafana panel PNGs for each chain's observation window, and pushes a structured release validation report to Notion. Trigger on 'validate release', 'generate chaos report', 'summarize chaos suite', '/validate-release [SUITE_ID]'. Two SUITE_ID formats exist: meta-workflow runs use <sha7>-<YYYYMMDD> (e.g. 3936ac9-20260528); scheduled scenario CronJobs use <YYYYMMDD>-<HHMMSS> (e.g. 20260528-080004). If omitted, lists both types from S3 and asks the user to choose. NOT for single-scenario debugging, ad-hoc Grafana panel export, or production chain analysis — use Grafana UI directly for those."
---

# validate-release

Collect chaos suite run data from S3 (workflow metadata) and Thanos (metrics), render Grafana panels for each scenario's observation window, and push a structured validation report to Notion.

## Preconditions

Check before doing any work. If any are missing, surface the gap and stop.

- `AWS_PROFILE=sei` — credentials with read on `s3://harbor-validation-results` and write on `s3://harbor-validation-results/chaos-suite-reports/`
- `kubectl` authenticated against **prod** cluster (for port-forwarding Thanos and Grafana)
- Notion MCP connected — run `/mcp` and select **claude.ai Notion**. No NOTION_TOKEN env var needed; the MCP handles auth.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `S3_BUCKET` | `harbor-validation-results` | S3 bucket for workflow artifacts and report PNGs |
| `S3_PREFIX` | `nightly` | Path prefix: `nightly/<scenario>/<SUITE_ID>/` |
| `THANOS_PORT` | `9090` | Local port for Thanos port-forward |
| `GRAFANA_PORT` | `3000` | Local port for Grafana port-forward |
| `GRAFANA_DASHBOARD_UID` | `nightly` | Dashboard UID (from `grafana-dashboards-nightly.yaml`) |

## Procedure

### 1. Resolve SUITE_ID

**Two run types exist** — always show both to the user and ask which to report on:

```bash
# Meta-workflow runs: format {sha7}-{YYYYMMDD}, created by chaos-suite CronJob
aws s3 ls s3://${S3_BUCKET}/${S3_PREFIX}/chaos-cpu-stress/ --profile sei \
  | grep -E '[0-9a-f]{7}-[0-9]{8}' | sort -r | head -5

# Scheduled scenario runs: format {YYYYMMDD}-{HHMMSS}, created by individual scenario CronJobs
aws s3 ls s3://${S3_BUCKET}/${S3_PREFIX}/chaos-cpu-stress/ --profile sei \
  | grep -E '[0-9]{8}-[0-9]{6}' | sort -r | head -5
```

Show both lists. If the user doesn't specify, **prefer the `{YYYYMMDD}-{HHMMSS}` format** — that is the scheduled nightly run. Ask for confirmation before proceeding.

### 2. Collect S3 workflow metadata

For each of the 13 chaos scenarios, download the three artifact files:

```bash
SCENARIOS="chaos-bandwidth-limit chaos-byzantine-simulation chaos-container-kill \
  chaos-cpu-stress chaos-disk-io-latency chaos-dns-chaos chaos-memory-stress \
  chaos-network-latency chaos-network-partition chaos-packet-loss \
  chaos-pod-failure chaos-rpc-chaos chaos-time-skew"

for s in $SCENARIOS; do
  aws s3 cp s3://${S3_BUCKET}/${S3_PREFIX}/${s}/${SUITE_ID}/ \
    /tmp/chaos-suite-${SUITE_ID}/${s}/ --recursive --profile sei 2>/dev/null \
  || echo "MISSING: ${s}"
done
```

**What S3 actually contains per scenario** (chaos scenarios only):
- `workflow-vars.yaml` — CHAIN_ID, EXIT_REASON, RPC endpoints, RUN_ID
- `workflow.yaml` — full Workflow CR JSON (creationTimestamp, seid image, task specs)
- `workflownodes.yaml` — WorkflowNodeList (empty after GC; non-empty if run was recent)

**No `report.json` exists for chaos scenarios.** Metrics come from Thanos (step 4).

For **load-test** scenarios only: `report.json` may exist with seiload TPS summary. Parse it if present.

**Pass/fail signal from `workflow-vars.yaml`:**
- `EXIT_REASON: pass` → provisioning succeeded; validators reached Ready and produced blocks
- `EXIT_REASON: infra-fail` / `EXIT_REASON: task-fail` → scenario failed
- File absent → scenario did not run (MISSING)

Note: `EXIT_REASON: pass` means provisioning completed, not that seiload sent transactions. Seiload TPS is determined from Thanos metrics in step 4.

### 3. Open Thanos and Grafana port-forwards (with cleanup trap)

Capture PIDs and register a trap so port-forwards and the ephemeral SA (created in step 5) are cleaned up even on early exit / Ctrl-C / shell death.

```bash
PROD_CTX="${PROD_CTX:-prod}"
# Verify the context exists — fail fast with a useful error
kubectl --context "${PROD_CTX}" -n monitoring get ns >/dev/null 2>&1 \
  || { echo "Context '${PROD_CTX}' not found or has no monitoring ns. Set PROD_CTX."; exit 1; }

cleanup() {
  [ -n "${SA_ID:-}" ] && curl -s -u "${ADMIN_USER}:${ADMIN_PASS}" -X DELETE \
    "${GRAFANA}/api/serviceaccounts/${SA_ID}" >/dev/null 2>&1 || true
  kill ${THANOS_PF:-} ${GRAFANA_PF:-} 2>/dev/null || true
}
trap cleanup EXIT

# Thanos Query Frontend — no auth required
kubectl --context "${PROD_CTX}" -n monitoring port-forward svc/thanos-query-frontend ${THANOS_PORT}:9090 >/dev/null 2>&1 &
THANOS_PF=$!

# Grafana — admin creds from cluster secret
kubectl --context "${PROD_CTX}" -n monitoring port-forward svc/grafana ${GRAFANA_PORT}:80 >/dev/null 2>&1 &
GRAFANA_PF=$!

THANOS="http://localhost:${THANOS_PORT}"
GRAFANA="http://localhost:${GRAFANA_PORT}"

# Wait up to 10s for readiness before proceeding
for i in 1 2 3 4 5 6 7 8 9 10; do
  curl -sf -m 2 "${THANOS}/-/ready" >/dev/null 2>&1 && \
  curl -sf -m 2 "${GRAFANA}/api/health" >/dev/null 2>&1 && break
  sleep 1
done

# Sanity preflight: the chaos_suite recording rules must produce data for SOME chain
# (catches the silent-false-pass mode where rules return empty and all scenarios look healthy)
SANITY=$(curl -sfG -m 5 "${THANOS}/api/v1/query" \
  --data-urlencode 'query=count(chaos_suite:block_time_p50:rate2m or vector(0))' \
  --data-urlencode "time=$(date +%s)" \
  | python3 -c "import json,sys; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else '0')")
[ "${SANITY%.*}" -gt 0 ] || echo "WARN: chaos_suite:block_time_p50:rate2m returns no data — recording rules may be empty or misconfigured. Inline fallbacks will be used."
```

Every curl in the procedure below uses `-m 30 --retry 2 --retry-connrefused` to survive transient port-forward death.

### 4. Derive per-scenario observation windows and query Thanos

For each scenario with `EXIT_REASON: pass`, derive the chain_id from `workflow-vars.yaml` (`CHAIN_ID` field).

**Time window — query Thanos directly from the chain's own telemetry:**

```bash
CHAIN_ID="chaos-cpu-stress-20260528-080004"   # from workflow-vars.yaml

# Window start: first block height sample
# Note: real seid emits tendermint_consensus_latest_block_height; mock-nightly emits tendermint_consensus_height
HEIGHT_METRIC="tendermint_consensus_latest_block_height or tendermint_consensus_height"
START_S=$(curl -sG "${THANOS}/api/v1/query" \
  --data-urlencode "query=timestamp(min_over_time((tendermint_consensus_latest_block_height{chain_id=\"${CHAIN_ID}\"} or tendermint_consensus_height{chain_id=\"${CHAIN_ID}\"})[24h:]))" \
  --data-urlencode "time=$(date +%s)" \
  | python3 -c "import json,sys; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else '')")

# Window end: last block height sample
END_S=$(curl -sG "${THANOS}/api/v1/query" \
  --data-urlencode "query=timestamp(last_over_time((tendermint_consensus_latest_block_height{chain_id=\"${CHAIN_ID}\"} or tendermint_consensus_height{chain_id=\"${CHAIN_ID}\"})[24h:]))" \
  --data-urlencode "time=$(date +%s)" \
  | python3 -c "import json,sys; r=json.load(sys.stdin)['data']['result']; print(r[0]['value'][1] if r else '')")

# Guard: empty → chain emitted no telemetry
if [ -z "$START_S" ] || [ -z "$END_S" ]; then
  echo "NO_TELEMETRY: ${CHAIN_ID}"
  continue
fi

START_MS=$(( ${START_S%.*} * 1000 ))
END_MS=$(( ${END_S%.*} * 1000 ))
# Padded window for instant queries (60s inward to skip scrape-lag warmup):
INNER_START_S=$(( ${START_S%.*} + 60 ))
INNER_END_S=$(( ${END_S%.*} - 60 ))
WINDOW_S=$(( ${END_S%.*} - ${START_S%.*} ))

**Metric queries (use inner window for instant/summary queries):**

Shared Python value-extraction helpers (NaN/+Inf filtered):

```python
# Use these inline in the curl pipelines below.
# avg_over: extract finite, positive samples and return average (or None)
# max_over: same, return max
# last_or_none: instant query, return scalar or None
```

```bash
# Curl helper — sanity timeouts and retries on every call
CURL_OPTS="-sfG -m 30 --retry 2 --retry-connrefused"

# TPS — match dashboard ordering: seiload final → cosmos throughput → tx_result_total → legacy app counter.
# mode="deliver" filter avoids double-counting check/recheck on sei_cosmos_throughput_transaction_count.
TPS=$(curl ${CURL_OPTS} "${THANOS}/api/v1/query_range" \
  --data-urlencode "query=max(last_over_time(seiload_run_tps_final_per_second{chain_id=\"${CHAIN_ID}\"}[24h])) or sum by (chain_id)(rate(sei_cosmos_throughput_transaction_count{chain_id=\"${CHAIN_ID}\",mode=\"deliver\"}[1m])) or sum by (chain_id)(rate(sei_chain_tx_result_total{chain_id=\"${CHAIN_ID}\",code=\"0\"}[1m])) or sum by (chain_id)(rate(sei_chain_app_tx_count_total{chain_id=\"${CHAIN_ID}\",result=\"successful\"}[1m]))" \
  --data-urlencode "start=${INNER_START_S}" --data-urlencode "end=${INNER_END_S}" --data-urlencode "step=30" \
  | python3 -c "import json,sys,math; r=json.load(sys.stdin)['data']['result']; vals=[float(v) for _,v in (r[0]['values'] if r else []) if v not in ('NaN','+Inf','-Inf') and math.isfinite(float(v)) and float(v)>0]; print(round(sum(vals)/len(vals),2) if vals else 0)")

# Block time p50 — recording rule, inline fallback for mock-nightly (no consensus_block_interval_seconds_bucket).
BLOCK_TIME_P50=$(curl ${CURL_OPTS} "${THANOS}/api/v1/query_range" \
  --data-urlencode "query=chaos_suite:block_time_p50:rate2m{chain_id=\"${CHAIN_ID}\"} or 1/sum by (chain_id)(rate(tendermint_consensus_height{chain_id=\"${CHAIN_ID}\"}[5m]))" \
  --data-urlencode "start=${INNER_START_S}" --data-urlencode "end=${INNER_END_S}" --data-urlencode "step=60" \
  | python3 -c "import json,sys,math; r=json.load(sys.stdin)['data']['result']; vals=[float(v) for _,v in (r[0]['values'] if r else []) if v not in ('NaN','+Inf','-Inf') and math.isfinite(float(v)) and float(v)>0]; print(round(sum(vals)/len(vals),3) if vals else None)")

# Universal triage triplet — recording-rule first, inline fallback covering mock-nightly chains.
HALTED_RAW=$(curl ${CURL_OPTS} "${THANOS}/api/v1/query" \
  --data-urlencode "query=min_over_time((chaos_suite:block_height_delta:rate2m{chain_id=\"${CHAIN_ID}\"} or sum by (chain_id)(rate(tendermint_consensus_height{chain_id=\"${CHAIN_ID}\"}[2m])))[${WINDOW_S}s:])" \
  --data-urlencode "time=${INNER_END_S}" \
  | python3 -c "import json,sys; r=json.load(sys.stdin)['data']['result']; print(float(r[0]['value'][1])<0.01 if r else 'unknown')")
# Translate Python bool → yes/no for the report
case "$HALTED_RAW" in True) HALTED="yes" ;; False) HALTED="no" ;; *) HALTED="unknown" ;; esac

BLOCK_TIME_P95=$(curl ${CURL_OPTS} "${THANOS}/api/v1/query_range" \
  --data-urlencode "query=chaos_suite:block_time_p95:rate2m{chain_id=\"${CHAIN_ID}\"} or histogram_quantile(0.95, sum by (chain_id, le)(rate(tendermint_consensus_round_duration_bucket{chain_id=\"${CHAIN_ID}\"}[5m])))" \
  --data-urlencode "start=${INNER_START_S}" --data-urlencode "end=${INNER_END_S}" --data-urlencode "step=60" \
  | python3 -c "import json,sys,math; r=json.load(sys.stdin)['data']['result']; vals=[float(v) for _,v in (r[0]['values'] if r else []) if v not in ('NaN','+Inf','-Inf') and math.isfinite(float(v)) and float(v)>0]; print(round(max(vals),3) if vals else None)")

MEMPOOL_MAX=$(curl ${CURL_OPTS} "${THANOS}/api/v1/query_range" \
  --data-urlencode "query=chaos_suite:mempool_size:max{chain_id=\"${CHAIN_ID}\"} or max by (chain_id)(tendermint_mempool_size{chain_id=\"${CHAIN_ID}\"})" \
  --data-urlencode "start=${INNER_START_S}" --data-urlencode "end=${INNER_END_S}" --data-urlencode "step=60" \
  | python3 -c "import json,sys,math; r=json.load(sys.stdin)['data']['result']; vals=[float(v) for _,v in (r[0]['values'] if r else []) if v not in ('NaN','+Inf','-Inf') and math.isfinite(float(v))]; print(int(max(vals)) if vals else 0)")

# BLOCKS — use the same OR pattern as time-window derivation for mock-nightly compatibility.
BLOCKS=$(curl ${CURL_OPTS} "${THANOS}/api/v1/query" \
  --data-urlencode "query=max_over_time((tendermint_consensus_latest_block_height{chain_id=\"${CHAIN_ID}\"} or tendermint_consensus_height{chain_id=\"${CHAIN_ID}\"})[24h:15s]) - min_over_time((tendermint_consensus_latest_block_height{chain_id=\"${CHAIN_ID}\"} or tendermint_consensus_height{chain_id=\"${CHAIN_ID}\"})[24h:15s])" \
  --data-urlencode "time=$(date +%s)" \
  | python3 -c "import json,sys; r=json.load(sys.stdin)['data']['result']; print(int(float(r[0]['value'][1])) if r else 0)")
```

Record per scenario: `CHAIN_ID`, `BLOCKS`, `TPS`, `BLOCK_TIME_P50`, `BLOCK_TIME_P95`, `MEMPOOL_MAX`, `HALTED`, `EXIT_REASON`, `STATUS`.

**STATUS classification:**
- `EXIT_REASON=pass` + `BLOCKS>0` + `TPS>0` → **PASS**
- `EXIT_REASON=pass` + `BLOCKS>0` + `TPS==0` → **PASS (no load)** — chain ran, seiload produced no transactions
- `EXIT_REASON=pass` + `BLOCKS==0` (window query returned empty) → **NO_TELEMETRY** — chain provisioned but no telemetry in Thanos (may indicate out-of-retention if workflow >14d old)
- `EXIT_REASON=infra-fail` or `task-fail` → **FAIL**
- Any Thanos query returns HTTP non-2xx → **ERROR** (distinct from NO_TELEMETRY — query failed, not "no data")
- File absent → **MISSING**

NO_TELEMETRY and ERROR are intentionally distinct: ERROR means the skill itself couldn't read data; NO_TELEMETRY means data was queried successfully but the chain emitted nothing. If a scenario hits the `continue` guard in the window-derivation code path, mark it NO_TELEMETRY in the summary table — do not let it silently drop out of the report.

### 5. Create ephemeral Grafana service account and render panels

The SA created here is cleaned up by the trap registered in step 3 — no explicit DELETE needed, and the cleanup fires even on Ctrl-C / shell death.

```bash
ADMIN_USER=$(kubectl --context "${PROD_CTX}" -n monitoring get secret grafana \
  -o jsonpath='{.data.admin-user}' | base64 -d)
ADMIN_PASS=$(kubectl --context "${PROD_CTX}" -n monitoring get secret grafana \
  -o jsonpath='{.data.admin-password}' | base64 -d)

# Create ephemeral Viewer SA (TRAP captures SA_ID and deletes on EXIT)
SA_RESP=$(curl -sf -m 10 -u "${ADMIN_USER}:${ADMIN_PASS}" -X POST \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"validate-release-$(date +%s)\",\"role\":\"Viewer\",\"isDisabled\":false}" \
  "${GRAFANA}/api/serviceaccounts")
SA_ID=$(echo "$SA_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

# Create token
TOKEN=$(curl -sf -m 10 -u "${ADMIN_USER}:${ADMIN_PASS}" -X POST \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"render-$(date +%s)\"}" \
  "${GRAFANA}/api/serviceaccounts/${SA_ID}/tokens" \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['key'])")

# Drop admin creds from env so they aren't visible to subsequent steps
unset ADMIN_PASS
```

**Render panels per scenario** (use the outer window — `START_MS` to `END_MS`):

Panel IDs for dashboard `nightly` (verify with `curl -sf "${GRAFANA}/api/dashboards/uid/${GRAFANA_DASHBOARD_UID}"` — IDs may drift across dashboard edits):
| ID | Title | Use |
|---|---|---|
| 10 | Block Time Percentiles | Block time timeseries |
| 7 | Run TPS | Transaction throughput |
| 8 | Tx Success Ratio | Success/failure rate |

```bash
for PANEL_ID in 10 7 8; do
  curl -sf -m 30 --retry 2 -H "Authorization: Bearer ${TOKEN}" \
    -o "/tmp/chaos-suite-${SUITE_ID}/${CHAIN_ID}-panel${PANEL_ID}.png" \
    "${GRAFANA}/render/d-solo/${GRAFANA_DASHBOARD_UID}/${GRAFANA_DASHBOARD_UID}?orgId=1&panelId=${PANEL_ID}&from=${START_MS}&to=${END_MS}&var-chain_id=${CHAIN_ID}&var-benchmark_namespaces=nightly&width=1600&height=600&tz=UTC&theme=dark" \
    || echo "render-failed: ${CHAIN_ID} panel ${PANEL_ID}"
done
```

Upload and presign:
```bash
aws s3 cp /tmp/chaos-suite-${SUITE_ID}/${CHAIN_ID}-panel10.png \
  s3://${S3_BUCKET}/chaos-suite-reports/${SUITE_ID}/${CHAIN_ID}-block-time.png --profile sei
PRESIGNED=$(aws s3 presign \
  s3://${S3_BUCKET}/chaos-suite-reports/${SUITE_ID}/${CHAIN_ID}-block-time.png \
  --expires-in 604800 --profile sei)
```

If a render returns a non-PNG (check `file` output) or HTTP 500, log the failure and embed `[Grafana panel unavailable — renderer missing or chain out of retention]` as a text block instead. Do not abort the run.

If ≥50% of renders fail across all scenarios, halt before pushing to Notion — a near-empty report is worse than no report. Surface the Grafana health to the user.

(SA cleanup is handled by the EXIT trap registered in step 3.)

### 6. Write local report before Notion push

Render `references/report-template.md` section by section, substituting the recorded fields from step 4 (CHAIN_ID, BLOCKS, TPS, BLOCK_TIME_P50, BLOCK_TIME_P95, MEMPOOL_MAX, HALTED, STATUS) and the presigned URLs from step 5.

Output: `/tmp/chaos-suite-${SUITE_ID}/report.md` — the durable fallback if Notion push fails.

### 7. Push to Notion via MCP

Use `mcp__claude_ai_Notion__notion-create-pages`. Find the parent page:

```
mcp__claude_ai_Notion__notion-search query="Platform" → get the Platform page id
```

Page structure per `references/report-template.md`:
- **Header callout**: SUITE_ID, seid image tag, date, overall N/13 pass
- **Summary table**: 13 rows — Scenario / Status / Blocks / Block Time p50 / TPS
- **Per-scenario sections**: heading + metrics paragraph + up to 3 Grafana PNG image blocks (presigned S3 URLs)
- **Footer**: S3 artifact paths, run duration

For each Grafana PNG: `image` block with `type: external` and the presigned URL.

After page creation, echo the URL. Offer to search for the previous run's report and show a trend diff.

## Error handling

- **Scenario MISSING from S3** — mark `MISSING` in summary table; skip metrics and panels for that scenario; continue.
- **Thanos returns empty for a chain_id** — chain either never emitted telemetry or is out of Prometheus retention (14d). Mark `NO_TELEMETRY`; note in the report.
- **Thanos query fails (non-2xx)** — mark scenario `ERROR`; continue; surface in report footer with the query that failed.
- **Sanity preflight warns** — `chaos_suite:*` recording rules return no series. Continue with inline fallbacks (built into every metric query) but flag the recording-rule gap in the report footer.
- **Grafana render fails** — embed placeholder text; continue. If ≥50% renders fail across all scenarios, halt before Notion push.
- **Notion API error** — dump the full assembled page content to `/tmp/chaos-suite-<SUITE_ID>/notion-payload.json`; print the path to the user.
- **Port-forward dies mid-run** — every curl uses `-m 30 --retry 2 --retry-connrefused`. If retries still fail, re-establish the forward and re-invoke the step once before aborting.
- **AWS credential expiry mid-run** — STS sessions can expire in 1h. On `ExpiredToken`, refresh credentials and re-run from the first scenario whose marker file is absent.
- **Notion MCP not connected** — `mcp__claude_ai_Notion__notion-search query="Platform"` should return a hit at session start. If it doesn't, halt before any data collection.
- **All 13 MISSING** — likely wrong SUITE_ID format. Stop and show the available IDs from both formats.

## Common confusions

| Symptom | Likely cause |
|---|---|
| All scenarios MISSING | SUITE_ID format mismatch — check both `{sha7}-{YYYYMMDD}` and `{YYYYMMDD}-{HHMMSS}` |
| All scenarios show PASS (no load) | Seiload didn't submit transactions (often: mock chain working as designed, OR seiload pod crashed) |
| All metric values None | Recording rules empty AND inline fallback also empty — chain may not be emitting `tendermint_consensus_*` at all |
| Notion page in wrong workspace | MCP connected to personal Notion, not the org workspace |
| All Grafana panels show no-data | Renderer plugin missing OR chains GC'd before query window OR datasource doesn't reach harbor |
| Numbers don't match Grafana UI | Skill uses `or` fallback chains; Grafana panel may use a different metric — embed query provenance in report |

## Known metric naming

The nightly seid binary emits metrics via OTel which sanitizes hyphens to underscores:
- `sei-chain_tx_result_total` (on-wire from seid) → `sei_chain_tx_result_total` (stored in Prometheus)
- Mock-nightly emits `tendermint_consensus_height` not `tendermint_consensus_latest_block_height` — both queried via `or` in the skill.
- Mock-nightly does NOT emit `tendermint_consensus_block_interval_seconds_bucket` — block time falls back to `1/rate(tendermint_consensus_height[5m])`.
- TPS metric cascade (in order of preference, matching the dashboard): `seiload_run_tps_final_per_second` → `sei_cosmos_throughput_transaction_count{mode="deliver"}` → `sei_chain_tx_result_total{code="0"}` → `sei_chain_app_tx_count_total{result="successful"}`.
- Recording rules (`chaos_suite:*`) are real-seid-biased — they reference `tendermint_consensus_latest_block_height` and `tendermint_consensus_block_interval_seconds_bucket`. For mock-nightly the skill provides inline `or` fallbacks in every metric query. **Follow-up:** patch the recording rules to use OR-fallback at source.
- `EXIT_REASON` semantics are owned by the chaos-runner — this skill is a consumer.
