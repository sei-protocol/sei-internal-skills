# validate-release guardrails

## Scope

This skill is **read-only**. It reads from:
- `s3://harbor-validation-results/` — seiload JSON reports
- `https://grafana.prod.platform.sei.io` — dashboard data + panel renders
- The Notion MCP (claude.ai Notion connection)

It writes to:
- `s3://harbor-validation-results/chaos-suite-reports/` — rendered panel PNGs only
- Notion — the generated report page

It does **not**:
- Touch any Kubernetes cluster
- Apply, delete, or modify any chaos CRs or Workflows
- Modify the platform repo
- Access production chain nodes

## Pre-flight checks

Before any side-effecting action (S3 write, Notion push):

1. **Grafana token** — `scripts/check-grafana.sh` must exit 0. HTTP 401 → stop and print setup instructions.
2. **Notion** — the MCP must be authenticated and `NOTION_DATABASE_ID` must be set.
3. **SUITE_ID** — at least one S3 report must exist for the suite. Zero reports → stop.

## Scope confirmation

Before the Notion push, echo to the user:

```
About to create Notion page:
  Suite:    <SUITE_ID>
  Database: <NOTION_DATABASE_ID>
  Scenarios: <N>/13 with data
  Images:   <N> Grafana panels uploaded to S3

Type 'confirm' to proceed, or anything else to abort.
```

Wait for explicit 'confirm' before calling `mcp__claude_ai_Notion__notion-create-pages`.

## Refusal conditions

Refuse to run (exit immediately with explanation) if:

- `GRAFANA_TOKEN` is unset or invalid
- Notion MCP is not authenticated  
- `NOTION_DATABASE_ID` is unset
- SUITE_ID resolves to zero S3 reports
- Fewer than 4 of 13 reports found (suite likely incomplete; confirm with user before partial report)

## Anti-corruption patterns

If the skill is interrupted mid-run:
- All scripts write to `state/run-<ts>/` before any Notion push
- On re-invocation, detect the latest `state/run-*/` directory and offer: resume from last step / archive and start fresh
- The Notion page is created once at the end — if creation fails, the full JSON payload is written to `state/run-<ts>/notion-payload.json` for manual recovery

## Never

- Never push a report page without user confirmation (Guardrail: scope confirmation)
- Never delete or modify an existing Notion page (use a new page each invocation)
- Never expose `GRAFANA_TOKEN` in output, logs, or state files
- Never auto-retry a Notion push without user awareness — failed pushes go to `notion-payload.json`
