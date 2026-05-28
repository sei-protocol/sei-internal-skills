# Notion Report Template

Structure of the Notion page created by `/validate-release`.

## Page metadata

Use `mcp__claude_ai_Notion__notion-search` to find the Platform parent page, then create under it:

```
parent: { page_id: <PLATFORM_PAGE_ID> }
icon: ✅ (all pass) | ⚠️ (partial) | 🔴 (majority fail)
title: "Chaos Suite Validation — <seid-image-tag> — <YYYY-MM-DD> (<SUITE_ID>)"
```

## Status classification

| Status | Condition |
|---|---|
| ✅ PASS | EXIT_REASON=pass, blocks>0, TPS>0 |
| ⚠️ PASS (no load) | EXIT_REASON=pass, blocks>0, TPS=0 |
| ⚠️ PASS (no telemetry) | EXIT_REASON=pass, blocks=0 |
| 🔴 FAIL | EXIT_REASON=infra-fail or task-fail |
| — MISSING | No S3 data for scenario |

## Page blocks (in order)

### 1. Callout — suite summary

Icon ✅/⚠️/🔴 based on overall result.

```
Suite <SUITE_ID> — <N>/13 pass  |  <M>/13 fail  |  <K>/13 missing
seid image: <nightly-YYYYMMDD-sha7>   date: <YYYY-MM-DD>   cluster: harbor/nightly
```

### 2. Summary table

Columns: Scenario | Status | Blocks | Block Time p50 | Avg TPS | Duration

One row per scenario. Bold any block time >0.504s (>20% over ~0.42s baseline).

### 3. Divider

### 4. Per-scenario sections (repeat for each non-MISSING scenario)

#### Heading: `<scenario-name>`

**Chain:** `<chain_id>`  **Window:** `<HH:MM>–<HH:MM> UTC`  **Status:** `<STATUS>`

Metrics paragraph:
```
Blocks produced: <N> (<rate> blk/s)
Block time p50: <Xs>  (baseline ~0.42s)
Avg TPS: <N>  (0 = seiload ran but submitted no transactions)
Halted during run: <yes/no>
```

Panel 10 image (Block Time Percentiles timeseries) — presigned S3 URL

Panel 7 image (Run TPS) — presigned S3 URL

Panel 8 image (Tx Success Ratio) — presigned S3 URL

### 5. Data sources section

```
Workflow artifacts: s3://harbor-validation-results/nightly/<scenario>/<SUITE_ID>/
  - workflow-vars.yaml  (chain endpoints, EXIT_REASON)
  - workflow.yaml       (Workflow CR, seid image, task specs)
  - workflownodes.yaml  (task pod records, empty after GC)
Metrics: prod Thanos (harbor seid scraped via prod Prometheus EC2 SD + PodMonitors)
Dashboard: https://grafana.prod.platform.sei.io/d/nightly  (UID: nightly)
Grafana panel IDs: 10=block-time-percentiles, 7=run-tps, 8=tx-success-ratio, 20=http-errors
```

### 6. Known gaps / follow-ups (include when relevant)

List any of these that applied to this run:
- TPS=0: seiload ran but submitted no transactions (check EVM RPC reachability)
- Recording rules returned empty (sei_chain_tx_result_total absent — older seid binary)
- Grafana panels show no-data (chains GC'd before render, or renderer plugin missing)
- N scenarios MISSING (likely image rollout timing)
