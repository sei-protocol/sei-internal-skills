# Platform observability

This file tells you how to read the platform's metrics, logs and alerts, and which query answers which question. Each query has a short bold name, so other files can cite it. Every query here returned data on the live fleet through Thanos. All access here is read-only.

Contents:

1. [Access through /grafana](#1-access-through-grafana)
2. [Named queries](#2-named-queries)
3. [Logs](#3-logs)
4. [The alert path](#4-the-alert-path)
5. [Dashboards](#5-dashboards)
6. [Blind spots](#6-blind-spots)

## 1. Access through /grafana

An agent reads metrics, logs and alert state through `/grafana`. That skill connects the `sei-grafana` MCP server, which acts as the read-only `agent-viewer` Grafana service account, with no port-forward and no token on disk. If the `mcp__sei-grafana__*` tools are absent, load `/grafana` and follow its procedure first.

| Question | Tool | Datasource uid |
|---|---|---|
| A metric value or trend, any federated cell | `query_prometheus` (`instant` or `range`) | `prometheus-prod` |
| Which metrics or labels exist | `list_prometheus_metric_names`, `list_prometheus_label_values` | `prometheus-prod` |
| How many log lines match | `query_loki_stats`, or `query_loki_logs` with `count_over_time` | `loki-fleet` |
| The log lines | `query_loki_logs` | `loki-fleet` |
| Which alerts fire, every federated cell | `query_prometheus` with **firing** | `prometheus-prod` |
| The state of each rule | `alerting_manage_rules`, operation list | `prometheus-prod` |
| Silences, routes and receivers | `alerting_manage_routing` | `alertmanager` (prod only) |

[The `/grafana` datasource table](../../grafana/SKILL.md#datasources) lists every datasource, the per-cell Loki stores included.

Three facts shape every query:

- The `cluster` label names the cell. Thanos adds it, so it exists only on `prometheus-prod`. Add `cluster` to `by (...)` to split a result by cell.
- The federated cells are prod, prod-use2, prod-euw1, prod-apne1 and harbor. Dev is not in Thanos or in `loki-fleet`. Its metrics and alerts live only in its own `monitoring` namespace.
- `alerting_manage_routing` reads the prod Alertmanager. A silence on another cell lives in that cell's Alertmanager, `monitoring/alertmanager-operated`.

## 2. Named queries

Run each query on `prometheus-prod` with `query_prometheus`, `instant` unless the row says otherwise. Report the result with the query, the datasource uid and the time in UTC.

### Fleet and Flux

- **cells-reporting**: `count by (cluster) (up == 1)`. Each federated cell returns one row. A missing cell has left the hub's view, and **firing** no longer shows its alerts. Whether the cell still pages depends on the cause ([FM8](diagnosis.md#fm8-a-cell-leaves-the-hubs-view)).
- **scrape-down**: `count by (cluster, job) (up == 0)`. prod carries legacy static scrape jobs that read down at all times. Compare with the same query at `offset 1d` before you call a target new.
- **flux-revision**: `count by (cluster, revision) (gotk_resource_info{customresource_kind="Kustomization", exported_namespace="flux-system", name="flux-system"})`. Each cell shows the `main` commit that it applied. prod-apne1 writes the ref as `refs/heads/main@sha1:...`. Compare the sha with `git rev-parse origin/main`.
- **flux-not-ready**: `max by (cluster, customresource_kind, exported_namespace, name) (gotk_resource_info{customresource_kind=~"Kustomization|HelmRelease|GitRepository|HelmRepository|OCIRepository", ready=~"False|Unknown", suspended!="true"})`. An object that shows for a few minutes during an apply is normal. `FluxObjectNotReady` fires after 30 minutes.
- **flux-suspended**: `count by (cluster, customresource_kind, exported_namespace, name) (gotk_resource_info{suspended="true"})`. An empty result is the healthy state. No alert covers a suspend.

### Alerts

- **firing**: `count by (alertname, cluster, severity, team) (ALERTS{alertstate="firing", alertname!="Watchdog"})`. `Watchdog` fires in every cell at all times by design.
- **pending**: `count by (alertname, cluster) (ALERTS{alertstate="pending"})`. A pending alert has met its expression but not its `for` duration yet.
- **metric-exists**: `count by (cluster) ({__name__="<metric>"})`. A cell missing from the result has no series, so a rule built on the metric never fires in that cell. An empty result means that the rule fires nowhere. Some metrics exist only while a job runs or after a first error, so read the rule before you call it dead.

### Hub and pipeline

- **thanos-store-errors**: `sum by (grpc_method, grpc_code) (rate(grpc_client_handled_total{job="thanos-query", grpc_code!~"OK|Canceled|Unavailable"}[5m]))`. A value above zero means the hub fails to read a sidecar or a store gateway. `ThanosQueryStoreFailures` fires above 0.5.
- **thanos-stores**: `count by (store_type) (thanos_store_nodes_grpc_connections)`. The number of sidecars, store gateways and rulers that the hub reads. A drop means a cell left the hub's view.
- **loki-ingest**: `sum by (cluster) (rate(loki_write_sent_entries_total{job="alloy-logs"}[15m]))`. Log lines per second that each cell's Alloy sends. A cell at zero has lost its logs.

### Traffic

- **gateway-replicas**: `max by (cluster) (kube_deployment_status_replicas_available{namespace="gateway", deployment="sei-gateway-istio"})`. Each cell shows 2.
- **gateway-5xx**: `sum by (cluster) (rate(istio_requests_total{response_code=~"5.."}[15m]))`. Only prod exports the Istio metrics for requests.
- **cert-days-left**: `bottomk(5, min by (cluster, namespace, name) (certmanager_certificate_expiration_timestamp_seconds - time()) / 86400)`. A certificate from the `letsencrypt` ClusterIssuer renews at about 30 days left. Only prod exports cert-manager metrics; on another cell, read `kubectl --context <cell> get certificates -A`.

### Capacity and workloads

- **pvc-fill**: `topk(10, max by (cluster, namespace, persistentvolumeclaim) (kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes))`. `PVCNearFull` fires at 0.85.
- **nodepool-usage**: `topk(10, max by (cluster, nodepool) (karpenter_nodepools_usage{resource_type="cpu"} / karpenter_nodepools_limit{resource_type="cpu"}))`. A pool near 1.0 cannot add a node. Run **metric-exists** on the two metrics if a cell is missing.
- **node-not-ready**: `count by (cluster) (kube_node_status_condition{condition="Ready", status="true"} == 0)`. An empty result is the healthy state.
- **crashloop**: `sum by (cluster, namespace) (kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff"}) > 0`.
- **chain-heights**: `max by (cluster, chain_id) (tendermint_consensus_latest_block_height)`. Use a `range` query to see whether each chain's head moves. Harbor also shows engineer and nightly chains.

## 3. Logs

Use `loki-fleet`. Give a stream selector of at least `cluster` and `namespace`, and count with `count_over_time` before you pull lines. Loki returns at most 10,000 lines per query.

| Component | Stream selector | Look for |
|---|---|---|
| Flux apply | `{cluster="<cell>", namespace="flux-system", container="manager", pod=~"kustomize-controller-.*"}` | `\|= "error"`, the Kustomization name |
| Flux Helm | `{cluster="<cell>", namespace="flux-system", pod=~"helm-controller-.*"}` | the HelmRelease name |
| external-dns | `{cluster="<cell>", namespace="external-dns"}` | `\|= "AccessDenied"`, the hostname |
| cert-manager | `{cluster="<cell>", namespace="cert-manager"}` | the Certificate name, `\|= "error"` |
| Gateway | `{cluster="<cell>", namespace="gateway"}` | the hostname, a 5xx code |
| Karpenter | `{cluster="<cell>", namespace="kube-system", pod=~"karpenter-.*"}` | `\|= "error"`, the NodePool name |
| The controller | `{cluster="<cell>", namespace="sei-k8s-controller-system"}` | the SeiNode name |
| A SeiNode | `{cluster="<cell>", namespace="<chain>", pod="<node>-0", container="seid"}` | see below |

cert-manager and Karpenter write few lines, so widen their window to 24 hours before you call it empty. seid writes logfmt and writes almost nothing while it runs, so an empty window is normal. A Go panic is a raw trace, not logfmt. A `level="ERROR"` filter drops it, so also search `|= "panic:"`.

## 4. The alert path

An alert moves in four steps:

1. **Rule.** A rule is a PrometheusRule object. Fleet rules live in `manifests/base/monitoring/alerts/`, hub-only rules in `clusters/prod/monitoring/alerts/`, and one-cell rules in `clusters/<cell>/monitoring/alerts/`.
2. **Evaluation.** Each cell's Prometheus evaluates its own rules. On prod, a rule with the label `evaluate-by: thanos-ruler` runs in the Thanos Ruler against the whole fleet instead. Use that label only for an expression across cells, or a window of 3 days or more.
3. **Routing.** Each cell's Alertmanager pages PagerDuty directly. `team: platform` goes to the platform service. On prod only, `team: protocol` goes to the protocol service and `public="true"` goes to the public service. Elsewhere, a `team: protocol` alert pages the platform service. No Slack route exists.
4. **Silence.** A silence lives in the Alertmanager of the cell that fires the alert. `Watchdog` routes to the `"null"` receiver in every cell.

To read a rule, call `alerting_manage_rules` and find it by name, or search the repository: `grep -rn "alert: <Name>" manifests/base clusters/*/monitoring`. Each rule carries a `runbook_url` and a generated `grafana_explore_url`.

[Procedure E](procedures.md#e-add-an-alert) adds an alert. The `sre-engineer` agent designs the expression and the threshold.

## 5. Dashboards

Dashboards are GrafanaDashboard objects in git. grafana-operator syncs them into the hub Grafana and overwrites edits made in the UI, so change a dashboard in git. Never hand-write a `grafana_explore_url` annotation. Generate it with `scripts/grafana_explore_url.py`, because CI rebuilds each URL and fails on any difference.

## 6. Blind spots

- **A dead rule is silent.** A rule whose metric has no series never fires. Metric names change when an exporter upgrades, and a component can lose its scrape target. The [rule-metric sweep](maintenance.md#4-rule-metric-sweep) finds these.
- **A cell whose own alerting stops is silent.** If a cell's Prometheus or Alertmanager stops, that cell pages nobody, and no external heartbeat watches `Watchdog`. A break in the sidecar or the hub path is different: the cell still pages, but the fleet view loses it. Check **cells-reporting** at the start of any health question.
- **A suspended Kustomization is silent.** Check **flux-suspended**.
- **Only the log names a failed DNS record.** `ExternalDNSRegistryErrors` reports that external-dns failed. The external-dns log names the record and the `AccessDenied`.
- **Some exporters run on one cell only.** Istio request metrics and cert-manager metrics come from prod only. Read the other cells with `kubectl`.
- **The phase gauge keeps deleted nodes.** `sei_controller_seinode_phase` keeps a series for a deleted SeiNode. Count SeiNodes with `kubectl`.
