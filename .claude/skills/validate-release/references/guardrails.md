# validate-release guardrails

## Scope

This skill is **read-only**. It reads from:
- the federated `prometheus-prod` datasource (prod thanos-query → harbor) — raw metrics
- the harbor cluster (kubectl, ns `nightly`) — the harness Job spec + pod log
- the Notion MCP (claude.ai Notion connection)

It writes to:
- `s3://harbor-validation-results/chaos-suite-reports/` — rendered panel PNGs only
- Notion — the generated report page

It does **not**:
- Apply, delete, or modify any Kubernetes object (read-only kubectl: `get jobs`, `logs`)
- Launch a Job or run the chaos suite (that is the deferred phase-2 driver)
- Modify the platform repo or access production chain nodes

## Pre-flight checks

Before any side-effecting action (S3 write, Notion push):

1. **Grafana auth** — `scripts/check-grafana.sh` must exit 0 (validates `/api/org` auth
   only — NOT the `prometheus-prod` datasource's presence). HTTP 401 → stop and print the
   service-account setup steps.
2. **Cluster read** — kubectl must reach the harbor context (ns `nightly`). If it
   cannot, the run renders as `VERDICT UNAVAILABLE`, never a synthesized pass.
3. **Notion** — MCP authenticated and `NOTION_DATABASE_ID` set.

## Anti-fabrication (the core discipline)

- The per-scenario outcome IS the Job-log `--- PASS|FAIL` — metrics are supporting
  context, never a substitute verdict.
- **Verdict-unavailable rule:** log GC'd (7d) but metrics survive (15d) ⇒ each scenario
  renders `VERDICT UNAVAILABLE — metrics-only` and the go/no-go headline is
  **suppressed**. Never synthesize an "N/N passed."
- **Run expired:** neither log nor raw metrics survive ⇒ graceful "run expired," not a
  crash or empty page. Recommend a re-run.
- `NO DATA` (absent series) is never rendered as a measured 0; a Thanos partial response
  marks the cell `PARTIAL` (degraded), not a clean measurement.
- A scenario absent from a complete log is `DID NOT RUN`; a truncated log marks
  unmatched scenarios `UNKNOWN`, not `DID NOT RUN`.

## Scope confirmation (pre-dispatch)

The only interactive gate is **before dispatching the background agent**. Once
dispatched, `platform-release-manager` runs unattended and pushes the Notion page
**without further interaction** — an interactive pre-push confirm is impossible for a
background agent. So `/validate-release` echoes the scope and waits for explicit
`confirm` up front (SKILL.md Step 0 / Guardrails step 4):

```
Run token:     <RUN_TOKEN>
Release image: <SEID_IMAGE_CHAOS or "resolved from Job">
Chains found:  <N>/10 scenarios in Prometheus
Run age:       <D.d>d  (raw bound: 15d)
Notion DB:     <NOTION_DATABASE_ID>
Type 'confirm' to proceed, or anything else to abort.
```

Wait for explicit `confirm` before dispatching the agent. There is **no** second confirm
before the Notion write — the pre-dispatch `confirm` authorizes the whole run.

## Refusal conditions

Refuse to run (exit with explanation) if:
- `GRAFANA_TOKEN` is unset or returns 401
- Notion MCP is not authenticated
- `NOTION_DATABASE_ID` is unset

Note: a missing Job log does NOT refuse — it renders verdict-unavailable (the
anti-fabrication path), which is the designed behavior, not an error.

## Never

- Never emit a bare "GO" — the headline is `LIVENESS GO` / `LIVENESS NO-GO`, with the
  tx-correctness caveat inline.
- Never synthesize a pass when the verdict log is unavailable.
- Never push a report page without user confirmation.
- Never delete or modify an existing Notion page (new page each invocation).
- Never expose `GRAFANA_TOKEN` in output, logs, or state files.
