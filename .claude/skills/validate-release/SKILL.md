---
name: validate-release
category: release-operations
model: claude-opus-4-8
description: "Queries Grafana metrics and S3 seiload reports for a completed chaos suite run, analyzes per-scenario results against BFT theory, and produces an executive-quality release validation report pushed to Notion — written for engineering leaders and stakeholders making a go/no-go release decision. Trigger on 'validate release', 'generate release report', 'chaos suite report', 'summarize the chaos run', '/validate-release [SUITE_ID]'. SUITE_ID format: <sha7>-<YYYYMMDD> (e.g. abc1234-20260527); omit to derive from latest S3 run. NOT for running the chaos suite itself (use /chaos-suite). NOT for debugging a single scenario (inspect the Workflow pod logs directly). NOT for production system monitoring (use the Grafana dashboards)."
---

# validate-release

Produce a complete release validation report from a chaos suite run. The report is written for engineering leaders and stakeholders — it leads with a clear go/no-go recommendation, explains what each test means for BFT consensus in plain language, and includes embedded Grafana panels scoped to the exact injection windows. One invocation = one Notion page, ready to share.

## Guardrails

This skill is **read-only** — it reads S3, Grafana, and writes to Notion. It does not touch any cluster.

Before proceeding:

1. **Token check** — verify `GRAFANA_TOKEN` is set and valid with `scripts/check-grafana.sh`. If the token is missing or expired, surface the service-account setup instructions and stop.
2. **Notion check** — verify the Notion MCP is authenticated (`/mcp` → claude.ai Notion) and `NOTION_DATABASE_ID` is set.
3. **Data check** — verify the SUITE_ID has at least one S3 report before starting full collection. A suite with zero reports should halt, not produce an empty page.
4. **Scope confirmation** — before dispatching the background agent, echo the target and wait for explicit user confirmation:
   ```
   Suite:     <SUITE_ID>
   seid SHA:  <sha7>
   Reports:   <N>/13 found in S3
   Notion DB: <NOTION_DATABASE_ID>
   Type 'confirm' to proceed, or anything else to abort.
   ```
5. **Refusal conditions** — refuse to run if:
   - `GRAFANA_TOKEN` is unset or returns 401
   - Notion MCP is not authenticated
   - `NOTION_DATABASE_ID` is unset
   - SUITE_ID resolves to zero S3 reports

See `references/guardrails.md` for the detailed safety model.

## Permissions (pre-approved happy-path)

Add to `.claude/settings.json` under `permissions.allow`:
```json
"Bash(aws s3 ls *)",
"Bash(aws s3 cp *harbor-validation-results*)",
"Bash(aws s3 presign *)",
"Bash(python3 .claude/skills/validate-release/scripts/*.py *)",
"Bash(bash .claude/skills/validate-release/scripts/check-grafana.sh)"
```

## Preconditions

- `AWS_PROFILE=sei` (or equivalent credentials with read on `s3://harbor-validation-results`)
- `GRAFANA_TOKEN` — Grafana service account token with Viewer role on `https://grafana.prod.platform.sei.io`. Create at: Administration → Service Accounts → Add service account (Viewer) → Generate token.
- `GRAFANA_BASE_URL` — defaults to `https://grafana.prod.platform.sei.io`
- `GRAFANA_DASHBOARD_UID` — defaults to `nightly` (matches `metadata.name` in `clusters/prod/monitoring/grafana-dashboards-nightly.yaml`)
- Notion MCP authenticated
- `NOTION_DATABASE_ID` — target database for reports

## Procedure

This skill dispatches `platform-release-manager` as a **background agent**. The user gets a confirmation, then Claude remains free. The agent notifies on completion.

### Step 0 — Resolve SUITE_ID

Run `scripts/resolve-suite-id.py`. If no SUITE_ID was provided, the script lists the five most recent runs from S3 and returns the latest. Confirm with the user:

```
Suite:     <SUITE_ID>
seid SHA:  <sha7>
Date:      <YYYYMMDD>
Grafana:   <GRAFANA_BASE_URL>/d/<GRAFANA_DASHBOARD_UID>
Reports:   <N> of 13 scenarios found in S3
Notion DB: <NOTION_DATABASE_ID>
```

Then run `scripts/check-grafana.sh` — halt on non-zero exit.

### Step 1 — Dispatch background agent

Spawn `platform-release-manager` with `run_in_background: true`. Pass:
- `SUITE_ID`, `GRAFANA_TOKEN`, `GRAFANA_BASE_URL`, `GRAFANA_DASHBOARD_UID`
- `NOTION_DATABASE_ID`
- `S3_BUCKET=harbor-validation-results`
- The run state directory: `state/run-<ISO-timestamp>/`

Tell the user: **"Generating report for suite `<SUITE_ID>`. I'll notify you when the Notion page is ready — this takes 5–10 minutes. You can keep working."**

### Step 2 — Agent work (background)

The `platform-release-manager` agent runs:

1. `scripts/collect-reports.py --suite-id <ID> --out state/run-<ts>/reports/` — downloads all 13 seiload JSON reports from S3.
2. `scripts/query-grafana.py --suite-id <ID> --out state/run-<ts>/metrics/` — queries Grafana's datasource proxy (recording-rule-backed scalars) for each scenario's time window; returns consistent stats dicts.
3. `scripts/compute-stats.py --metrics-dir state/run-<ts>/metrics/ --out state/run-<ts>/verdicts/` — deterministic outcome classification and delta computation; emits `verdict.json` per scenario with `outcome`, `deltas`, `recovery_seconds`, `noise_flag`.
4. `scripts/render-panels.py --suite-id <ID> --metrics-dir state/run-<ts>/metrics/ --out state/run-<ts>/panels/` — renders TPS, block-time, and error-rate panel PNGs scoped to each scenario's chaos window.
5. `scripts/upload-images.py --dir state/run-<ts>/panels/ --suite-id <ID>` — uploads PNGs to S3, returns presigned URLs (7-day expiry).
6. Report assembly — agent narrates `verdict.json` outputs (no arithmetic); see `references/analysis-guide.md`.
7. `scripts/push-notion.py --suite-id <ID> --state-dir state/run-<ts>/` — assembles Notion payload from verdicts + image URLs; triggers `mcp__claude_ai_Notion__notion-create-pages`.

### Step 3 — Notify

When the agent completes, surface the result:
- **Success**: Notion page URL + one-line outcome ("13/13 scenarios passed" or "11/13 passed — 2 FAILED: pod-failure, rpc-chaos").
- **Partial**: which steps succeeded, which failed, offer to retry failed step.
- **Failure**: full error + `state/run-<ts>/audit.log` path for debugging.

## Halt Conditions

Stop and report without auto-remediation if:

- `GRAFANA_TOKEN` returns 401 — print the service account token setup steps.
- Fewer than 4 of 13 S3 reports found — the suite likely didn't complete; confirm with user before generating a partial report.
- Grafana data API returns no data for a scenario — mark it `NO DATA` in the report; do not silently skip.
- Notion page creation fails — dump full JSON payload to `state/run-<ts>/notion-payload.json`; user can paste manually.
- S3 upload of panel images fails — embed `[Grafana panel unavailable]` text block rather than a broken image.

## State Management

Per-run state in `state/run-<ISO-timestamp>/`:
- `reports/` — downloaded seiload JSON files
- `metrics/` — Grafana data API responses (JSON time series per scenario)
- `panels/` — rendered PNG files
- `audit.log` — timestamped log of every script call, exit code, and output

State is gitignored. On interrupted run, the next invocation detects the incomplete state directory and offers: resume from last completed step / archive and start fresh.

## Summary

The agent produces a Notion page structured per `references/report-template.md`:
- **Executive Summary** — recommendation, overall BFT assessment, recovery quality, action items
- **What Was Tested** — fault families table
- **Per-scenario sections** — Summary / Key Signals / Release Significance + 3 Grafana panel embeds
- **Platform Action Items** — table linking to GitHub issues
- **Appendix** — raw metric tables (baseline/chaos/recovery), S3 links

The in-chat notification is a single line: the Notion URL + outcome.
