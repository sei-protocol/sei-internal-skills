---
name: validate-release
category: release-operations
model: claude-opus-5
description: "Produces a truthful chaos release report from a real nightly harness run by consuming what the live harness emits — raw harbor Prometheus metrics for the per-scenario story and the harness Job (spec env for the release image, pod-log for the authoritative PASS/FAIL verdict) — then pushes an executive-quality report to Notion for a go/no-go decision. The report is a LIVENESS gate (the chain stayed live and recovered under fault); tx-correctness is NOT validated. Trigger on 'validate release', 'generate release report', 'chaos suite report', 'summarize the chaos run', '/validate-release [RUN_TOKEN]'. RUN_TOKEN = the shared base36 token of a chaos run (chains are chaos-<token>-<scenario>); omit to resolve the latest run from Prometheus. NOT for running the chaos suite itself. NOT for debugging a single scenario (read the harness Job log directly). NOT for production monitoring (use the Grafana dashboards)."
---

# validate-release

Turn a real nightly chaos run into one executive-quality Notion report and a
clear **liveness** go/no-go. The report names the release image under test,
reports each scenario's authoritative PASS/FAIL from the harness Job log, and
annotates it with the raw harbor chaos release signals (halt + block-interval)
scoped to the run window. One invocation = one Notion page. (TPS + mempool are
~0 by design — chaos runs no load generator — carried transparency-only.)

**Scope of the verdict:** `TestNightlyChaosSuite` asserts *liveness* — the chain
stayed live under the fault and recovered. It is blind to partial tx-correctness.
The headline reads **`LIVENESS GO` / `LIVENESS NO-GO`**, never a bare "GO", and
carries the "tx-correctness not validated" caveat inline.

## Data model (what the live harness emits)

- **Metrics** — raw harbor series via the federated `prometheus-prod` datasource
  (prod `thanos-query` fans out to harbor). Live-confirmed names:
  `tendermint_consensus_height`, `tendermint_consensus_block_interval_seconds_bucket`,
  `sei_cosmos_throughput_transaction_count`, `tendermint_mempool_size`. Forced to
  raw resolution (`max_source_resolution=0`). NOT the prod-scoped chaos-suite
  recording rules (empty for harbor). Series are **ephemeral** — every query is
  time-scoped to the run window, never instant-now. The chaos release signals are
  **halt + block-interval**; `sei_cosmos_throughput_transaction_count` and
  `tendermint_mempool_size` are **~0 by design** (the chaos suite runs no load
  generator; seiload is benchmark-only) — carried transparency-only, NOT release
  signals; throughput is the deferred phase-2 benchmark report.
- **Verdict + release image** — the harness Job (harbor, ns `nightly`): the release
  image `SEID_IMAGE_CHAOS` from the Job **spec env**, and each authoritative
  `--- PASS|FAIL: TestNightlyChaosSuite/<scenario>` from the pod **log**.
- **Run identity** — the shared base36 `<token>`; chains are `chaos-<token>-<scenario>`.
- **Retention** — raw metrics ≈ 15d; Job log ≈ 7d. The 7d/15d band is where metrics
  survive but the verdict does not (see Halt Conditions).

## Guardrails

Read-only. It queries the federated Prometheus datasource and the harbor Job
(read), and it writes one Notion page (plus panel PNGs to S3 for embedding). It
does **not** apply, delete, or modify any cluster object, and it never launches a
Job. See `references/guardrails.md`.

Before proceeding:

1. **Token check** — `scripts/check-grafana.sh` must exit 0 (validates Grafana **auth**
   via `/api/org` only — NOT the `prometheus-prod` datasource's presence). On 401,
   surface the service-account setup steps and stop.
2. **Cluster read** — `kubectl` must reach the harbor context (ns `nightly`) to read
   the Job log. If it cannot, the run still renders — as `VERDICT UNAVAILABLE`, never
   a synthesized pass.
3. **Notion check** — Notion MCP authenticated and `NOTION_DATABASE_ID` set.
4. **Scope confirmation** — before dispatching the agent, echo the target and wait
   for explicit `confirm`:
   ```
   Run token:     <RUN_TOKEN>
   Release image: <SEID_IMAGE_CHAOS or "resolved from Job">
   Chains found:  <N>/10 scenarios in Prometheus
   Run age:       <D.d>d  (raw bound: 15d)
   Notion DB:     <NOTION_DATABASE_ID>
   Type 'confirm' to proceed, or anything else to abort.
   ```
5. **Refusal conditions** — refuse if `GRAFANA_TOKEN` is unset / returns 401, the
   Notion MCP is unauthenticated, or `NOTION_DATABASE_ID` is unset.

## Permissions (pre-approved happy-path)

Add to `.claude/settings.json` under `permissions.allow`:
```json
"Bash(python3 .claude/skills/validate-release/scripts/*.py *)",
"Bash(bash .claude/skills/validate-release/scripts/check-grafana.sh)",
"Bash(kubectl --context harbor -n nightly get jobs *)",
"Bash(kubectl --context harbor -n nightly logs *)"
```

## Preconditions

- `GRAFANA_TOKEN` — Grafana service-account token (Viewer) on the federated
  datasource. Create at: Administration → Service Accounts → Add service account
  (Viewer) → Generate token.
- `GRAFANA_BASE_URL` — defaults to `https://grafana.prod.platform.sei.io`
- `PROM_DS_UID` — federated datasource UID; defaults to `prometheus-prod`
- `GRAFANA_DASHBOARD_UID` — defaults to `nightly` (panel renders)
- `kubectl` context `harbor` with read on ns `nightly` (Job spec + pod logs)
- Notion MCP authenticated; `NOTION_DATABASE_ID` set
- `AWS_PROFILE=sei` — only for `upload-images.py` (panel PNG → S3 for Notion embed)

## Procedure

Dispatches `platform-release-manager` as a **background agent**. The user gets a
confirmation, then Claude stays free; the agent notifies on completion.

### Step 0 — Auth check, then resolve the run token

First run `scripts/check-grafana.sh` — halt on non-zero exit and surface the
service-account setup steps. This MUST precede discovery: `resolve-run.py` queries
the federated datasource and needs `GRAFANA_TOKEN`, so a missing/401 token caught
here gives the setup guidance instead of a terse Prometheus error mid-discovery.
Then run `scripts/resolve-run.py [--run <token>] [--out state/run-<ts>/]`. With no
`--run`, it lists the recent runs discovered in Prometheus and returns the latest
token. Echo the scope block and wait for `confirm`.

### Step 1 — Dispatch background agent

Spawn `platform-release-manager` with `run_in_background: true`. Pass `RUN_TOKEN`,
`GRAFANA_TOKEN`, `GRAFANA_BASE_URL`, `PROM_DS_UID`, `GRAFANA_DASHBOARD_UID`,
`NOTION_DATABASE_ID`, and the state dir `state/run-<ISO-timestamp>/`.

Tell the user: **"Generating the report for run `<RUN_TOKEN>`. I'll notify you when
the Notion page is ready — a few minutes. You can keep working."**

### Step 2 — Agent work (background)

1. `scripts/query-grafana.py --run <TOKEN> --out state/run-<ts>/metrics/` — raw
   harbor metrics per scenario, time-scoped to the run window, raw resolution.
   Per-scenario stats dict; distinguishes `NO DATA` from measured 0; marks a cell
   `PARTIAL` on a Thanos partial response.
2. `scripts/collect-run-log.py --run <TOKEN> --out state/run-<ts>/run-log/` — joins
   the token to the nightly Job by **window-containment + log-verification** (a Job
   whose run window contains the decoded UnixNano AND whose log references this run's
   `chaos-<token>-` chains; no match → VERDICT UNAVAILABLE), extracts `SEID_IMAGE_CHAOS`
   from the Job spec and the per-scenario `--- PASS|FAIL` lines from the log,
   reconciles against the 10-scenario set (missing → `DID NOT RUN`), and flags a
   truncated log read.
3. `scripts/compute-stats.py --run-log state/run-<ts>/run-log/ --metrics-dir
   state/run-<ts>/metrics/ --out state/run-<ts>/verdicts/` — per-scenario outcome =
   the Job-log verdict (authoritative), annotated with the metric summary; applies
   the verdict-unavailable rule and computes the headline.
4. `scripts/render-panels.py --run <TOKEN> --metrics-dir state/run-<ts>/metrics/ --out
   state/run-<ts>/panels/` — block-time / TPS / mempool panel PNGs scoped to each
   scenario's window and `chaos-<token>-<scenario>`.
5. `scripts/upload-images.py --dir state/run-<ts>/panels/ --suite-id <TOKEN> --out state/run-<ts>/panels/image-urls.yaml`
   — uploads PNGs to S3 and writes the scenario→panel→presigned-URL map to
   `panels/image-urls.yaml` (the run token is the S3 namespace). The `--out` is
   REQUIRED: `push-notion.py` reads `panels/image-urls.yaml`; without it the URLs
   only print to stdout and panel embeds are dropped from the Notion page.
6. Report assembly — the agent narrates `verdicts/*.json` (no arithmetic); see
   `references/analysis-guide.md`.
7. `scripts/push-notion.py --run <TOKEN> --state-dir state/run-<ts>/` — assembles the
   report. Authoring rules for a line that renders cleanly and stays re-matchable on
   edit are `references/notion-flavored-markdown.md` (this skill owns them; it is the
   only core skill that writes Notion).
   Notion payload (headline + run-identity header + per-cell provenance markers);
   triggers `mcp__claude_ai_Notion__notion-create-pages`.

### Step 3 — Notify

- **Success**: Notion URL + one line — `LIVENESS GO`/`LIVENESS NO-GO` and the count
  (e.g. "9/10 PASS — 1 FAIL: pod-failure").
- **Verdict unavailable**: state it plainly — "log GC'd, metrics survive; re-run to
  get a verdict; do NOT ship on metrics alone."
- **Failure**: full error + `state/run-<ts>/` path.

## Halt Conditions

Stop and report without auto-remediation if:

- `GRAFANA_TOKEN` returns 401 — print the service-account setup steps.
- **Run older than the ~15d raw bound** — `resolve-run.py` reports "raw expired" (no
  chains discovered); do not crash or write an empty page. Offer to re-run the nightly.
- **Log GC'd (7d) but metrics survive (15d)** — `compute-stats.py` renders each
  scenario `VERDICT UNAVAILABLE — metrics-only` and **suppresses** the headline. Never
  synthesize a pass; recommend a re-run.
- **`kubectl` cannot reach harbor** — the report renders as verdict-unavailable, not a
  crash; state the cluster-read gap.
- **A scenario has no series** — mark it `NO DATA` (absent), never a green 0; a scenario
  the log never reports is `DID NOT RUN`.
- **Notion push fails** — the full payload remains at `state/run-<ts>/notion-payload.json`
  for manual paste.

## State Management

Per-run state in `state/run-<ISO-timestamp>/`:
- `metrics/<scenario>.json` — per-scenario raw-metric stats dicts
- `run-log/verdicts.json` — per-scenario verdicts (from the pod log) + release image (from the Job spec env)
- `verdicts/<scenario>.json` + `verdicts/summary.json` — joined outcomes + headline
- `panels/` — rendered PNGs + `image-urls.yaml`

State is gitignored. On an interrupted run, the next invocation detects the incomplete
state dir and offers: resume from the last completed step / archive and start fresh.

## Summary

The Notion page follows `references/report-template.md`:
- **Run identity** — token, release image, run age vs the 15d bound
- **Headline** — `LIVENESS GO` / `LIVENESS NO-GO` (or suppressed) + the tx-correctness caveat
- **Per-scenario sections** — outcome (Job-log), metric annotation, provenance marker, panels
- **Appendix** — raw metric tables per scenario

The in-chat notification is a single line: the Notion URL + the liveness outcome.
