---
name: grafana
description: "Use when an agent needs Sei metrics, logs or alert state from prod Grafana with no port-forward: Prometheus through Thanos (every cell, harbor included), Loki and the prod Alertmanager. Triggers: 'query prometheus', 'query loki', 'check the logs', 'what alerts are firing', 'is anything firing', 'grafana query', 'set up grafana access', 'sei-grafana', '/grafana'. Anti-triggers: NOT for writing or tuning alert rules, recording rules or dashboards (use the sre-engineer agent, then a sei-protocol/platform PR); NOT for kubectl reads of cluster objects; NOT for giga-testnet-0 diagnosis (use /giga-dev, which uses this skill for its queries); NOT for the lifecycle of an engineer's harbor chain (use /harbor-dev)."
---

# grafana

Read-only access to prod Grafana for an agent, with no port-forward. The skill connects the `sei-grafana` MCP server, and it says which datasource answers which question. One Grafana at `https://grafana.prod.platform.sei.io` holds the metrics of every cell, the logs of every cell, and the alert state.

`/harbor-dev` and `/giga-dev` use this skill for their metric, log and alert queries. Any other skill that needs prod metrics, logs or alerts uses it the same way.

## Guardrails

This skill reads prod Grafana only. It acts as the `agent-viewer` service account, which has the Grafana Viewer role. Before any side effect:

1. **Context check**: `scripts/grafana-mcp.sh --check` passes before the skill registers the server or tells the user that access works.
2. **Scope confirmation**: before `--register`, show the user the line that `--register --dry-run` prints, and wait for "confirm". Registration writes the user's Claude Code configuration.
3. **Refusal conditions**: the skill refuses to:
   - print, paste, log, store or commit the token, or give it to any program except `scripts/grafana-mcp.sh`. The token reads every datasource in prod Grafana, CloudWatch included.
   - widen the server: change `--enabled-tools`, remove `--disable-write`, or add the `admin` or `cloudwatch` category.
   - create, change or delete a dashboard, an alert rule, a silence or a contact point. The token cannot do it, and the skill does not look for another path.
   - skip the checksum check, or run an mcp-grafana binary that the script did not check.
   - pull raw log lines over more than 1 hour, or with no stream selector, before a `count_over_time` query gives the size. Loki returns at most 10,000 lines per query, and a wide pull loads the shared fleet store.
   - register the server without the user's "confirm".

## Preconditions

- The AWS CLI v2, with an SSO profile for account `189176372795` that has EKS access to the `prod` cluster. Every engineer with EKS access holds it through the `EngWithLimitedPacific-1` role.
- `kubectl`, `curl`, `tar` and `base64`. Registration also needs `claude`.
- On prod: the GrafanaServiceAccount `agent-viewer` and the Secret `monitoring/grafana-agent-viewer-token`. sei-protocol/platform holds both in `clusters/prod/monitoring/grafana-agent-viewer.yaml`.

## Procedure

1. **Use the tools if the session has them.** Look for tools named `mcp__sei-grafana__*`. If they exist, call `mcp__sei-grafana__list_datasources` and go to step 4. Success: the call returns the datasources in the table below.
2. **Prove access.** Run `bash scripts/grafana-mcp.sh --check --aws-profile <profile>`. If `AWS_PROFILE` is unset and the user named no profile, ask for one. Success: exit 0, and the last line starts with `ok:`. Any other exit code is a halt condition.
3. **Register the server.** Run `bash scripts/grafana-mcp.sh --register --dry-run --aws-profile <profile>` and show the user the line it prints. After "confirm", run it again without `--dry-run`. Tell the user to restart Claude Code, or to reconnect `sei-grafana` from the Claude Code MCP menu. Success: the `mcp__sei-grafana__*` tools appear in the next session.
4. **Pick the datasource.** Use the table in [Datasources](#datasources). Success: you can name the datasource uid for the question before you query.
5. **Query.** Use the tools in [Queries](#queries). Count before you pull log lines. Success: each result carries its query, its datasource uid and its time range.
6. **Report.** Give each number or log excerpt with its query, its datasource uid and its absolute time range in UTC. When a result is empty, say so, and say what the empty result means.

## Datasources

| Datasource | uid | Holds |
|---|---|---|
| `Prometheus` (default) | `prometheus-prod` | Thanos: the metrics of every cell, harbor included, with 90 days of raw data |
| `Loki-fleet` | `loki-fleet` | The logs of every prod cell |
| `Loki` | `loki` | The `prod` cell logs only |
| `Loki-prod-use2`, `Loki-prod-euw1`, `Loki-prod-apne1` | the name in lowercase | That cell's logs only |
| `Loki-harbor` | `loki-harbor` | The harbor cell logs |
| `Alertmanager` | `alertmanager` | The prod Alertmanager |

- The `cluster` label names the cell. Thanos adds it, so add `cluster` to `by (...)` to split a result by cell.
- Each prod cell writes every log line to its own Loki and to `Loki-fleet`. Start with `Loki-fleet`. Use a per-cell datasource for break-glass, or for history older than the fleet store. Each Loki store keeps 90 days.
- Grafana has no datasource for one cell's own Prometheus. The exact value that a KEDA trigger reads lives only there. `/giga-dev` gives that read.

## Queries

| Question | Tool | Notes |
|---|---|---|
| A metric value or a series | `query_prometheus` on `prometheus-prod` | Use `instant` for now, and `range` with a step for a trend. |
| Which metrics or labels exist | `list_prometheus_metric_names`, `list_prometheus_label_names`, `list_prometheus_label_values` | Pass a selector, so the answer stays small. |
| How many log lines match | `query_loki_stats`, or `query_loki_logs` with `count_over_time` | Run this first. |
| The log lines | `query_loki_logs` on `loki-fleet` | Always give a stream selector: at least `namespace`, plus `pod` or `container`. |
| Which alerts fire now, in every cell | `query_prometheus` on `prometheus-prod` with `ALERTS{alertstate="firing"}` | Split by `alertname`, `cluster` and `severity`. |
| The state of each alert rule | `alerting_manage_rules`, operation list | It covers Grafana-managed rules and the rules of `prometheus-prod`. |
| Silences, routes and receivers | `alerting_manage_routing` | It reads the `alertmanager` datasource. |

seid writes logfmt, and it writes almost nothing while it runs, so an empty window is normal. A Go panic is a raw trace, not logfmt. A `level="ERROR"` filter drops it, so also search `|= "panic:"`.

## Halt Conditions

Stop and report to the user if:

- `grafana-mcp.sh` exits 2: no AWS profile. Ask which SSO profile to use.
- It exits 3: a tool is missing. Name it.
- It exits 4: AWS or EKS access failed, or the token Secret is missing. Ask the user to run `aws sso login` with the profile. If that does not fix it, the user needs EKS access to `prod` from the platform team. Report the message as printed.
- It exits 5: Grafana is unreachable, or it rejected the token. On HTTP 401, the token has expired, or someone revoked it. The platform team rotates it in `grafana-agent-viewer.yaml`. Do not look for other credentials.
- It exits 6: the download or the checksum failed. Report the message, and do not bypass the check.
- It exits 7: registration failed. Report the message.
- The `mcp__sei-grafana__*` tools are still absent after registration and a restart.
- A tool call returns a permission error. Report the error and the tool name. The token is read-only by design.
