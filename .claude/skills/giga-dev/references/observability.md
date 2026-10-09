# Observability of giga-testnet-0

This file tells you how to reach the logs and metrics of giga-testnet-0, and which query answers
which question. Each query has a short bold name, so other files can cite it. Every query here
returned data on the live fleet. A "normal" value is an approximate envelope, measured at steady
load inside the scaler's busy band. It is not a guarantee. All access here is read-only.

Contents:

1. [Access](#1-access)
2. [Logs](#2-logs)
3. [Metrics by question](#3-metrics-by-question)
4. [Dashboards](#4-dashboards)
5. [Alerts and blind spots](#5-alerts-and-blind-spots)

## 1. Access

### Cells and contexts

| Cell (`cluster` label) | Region | Validators | Full nodes |
|---|---|---|---|
| `prod-use2` | us-east-2 | `validator-00-0` to `-09-0` | `fullnode-00-0`, `fullnode-01-0` |
| `prod` | eu-central-1 | `validator-10-0` to `-19-0` | `fullnode-10-0`, `fullnode-11-0` |
| `prod-euw1` | eu-west-1 | `validator-20-0` to `-29-0` | `fullnode-20-0`, `fullnode-21-0` |
| `prod-apne1` | ap-northeast-1 | `validator-30-0` to `-39-0` | `fullnode-30-0`, `fullnode-31-0` |

The EKS cluster name and the kubectl context are the cell name:

```sh
aws eks update-kubeconfig --region us-east-2 --name prod-use2 --alias prod-use2
```

The `prod` cell also hosts Grafana, the federated Thanos querier, Thanos Ruler and the fleet Loki
store. EVM-only nodes serve no CometBFT RPC: nothing listens on 26657. Read heights from metrics.

### Grafana

`https://grafana.prod.platform.sei.io`, Google login. `@seinetwork.io` accounts get Editor.
`@sei.io` and `@seifdn.org` accounts get Viewer, which is enough here.

| Datasource | uid | Holds |
|---|---|---|
| `Prometheus` (default) | `prometheus-prod` | Thanos: the metrics of every cell, harbor included |
| `Loki-fleet` | `loki-fleet` | The logs of every cell |
| `Loki` | `loki` | `prod` cell logs only |
| `Loki-prod-use2`, `Loki-prod-euw1`, `Loki-prod-apne1` | the name in lowercase | That cell's logs only |
| `Alertmanager` | `alertmanager` | The `prod` Alertmanager only |

The runbooks call the metrics datasource `prometheus-prod`; Grafana shows it as `Prometheus`.
Each cell writes every log line to its own Loki and to `Loki-fleet`. Start with `Loki-fleet`.
Use a per-cell datasource for break-glass, or for history older than the fleet store. Both keep 90
days and return at most 10,000 lines per query, so count with `count_over_time` first.

### Thanos, the cell Prometheus and the `cluster` label

Each cell runs its own Prometheus (`prometheus-operated.monitoring.svc:9090`, 30 days). The Thanos
querier in `prod` federates all cells, with 90 days of raw data.

The `cluster` label is the cell discriminator. Each cell's Prometheus adds `cluster` and `region`
as external labels, so only Thanos shows them. On a cell's own Prometheus, a `cluster` matcher
returns nothing.

| You want | Read from |
|---|---|
| Head, lag, alerts or any cross-cell comparison | Thanos. Add `cluster` to `by (...)` to split by cell |
| The exact value that a scaler trigger reads | That cell's Prometheus, with the trigger's own query |

Port-forward to the Service, not a pod. Pod names change on a rollout.

```sh
kubectl --context prod -n monitoring port-forward svc/thanos-query 19090:9090
curl -s 'http://localhost:19090/api/v1/query' \
  --data-urlencode 'query=count by (cluster, job) (up{namespace="giga-testnet-0"})'
kubectl --context prod-use2 -n monitoring port-forward svc/prometheus-operated 19092:9090
```

Loki needs the tenant header `X-Scope-OrgID: 1`. A cell's own Loki is `svc/loki-gateway`.

```sh
kubectl --context prod -n monitoring port-forward svc/loki-fleet-gateway 13100:80
curl -s -G -H 'X-Scope-OrgID: 1' 'http://localhost:13100/loki/api/v1/query_range' \
  --data-urlencode 'query=sum by (cluster, container) (count_over_time({namespace="giga-testnet-0"}[1h]))'
```

### What a read-only operator needs

| Task | Needs |
|---|---|
| Dashboards, Explore, alert history | Grafana Viewer |
| `kubectl get`, `describe`, `logs` | EKS access to the cell. A view policy covers it |
| Port-forward to Thanos, Prometheus or Loki | `create` on `pods/portforward` in `monitoring`. A view role lacks it; use Grafana |

The checks used most, per cell:

```sh
kubectl --context prod-use2 -n giga-testnet-0 get pods -o wide
kubectl --context prod-use2 -n giga-testnet-0 get scaledobject seiload-saturation -o yaml
kubectl --context prod-use2 -n giga-testnet-0 get hpa keda-hpa-seiload-saturation \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}'
```

A healthy ScaledObject prints `READY ACTIVE FALLBACK PAUSED` as `True True False False`. In its
`status.health`, each trigger (`s0-prometheus` to `s8-prometheus`) reads `Happy`. The HPA shows
`ScalingActive=True ValidMetricFound`.

## 2. Logs

### Labels and formats

| Label | Values |
|---|---|
| `namespace` | `giga-testnet-0`; `keda` for the KEDA operator |
| `cluster` | `prod-use2`, `prod`, `prod-euw1`, `prod-apne1` |
| `pod` | `validator-NN-0`, `fullnode-NN-0`, `seiload-saturation-<hash>-<id>` |
| `container` | `seid` (node), `sei-sidecar`, `seid-init`, `kube-rbac-proxy`, `seiload` (load) |
| `sei_role` | `validator`, `node` (full node) |
| `chain_id` | `giga-testnet-0` |

- arctic-1 uses the same pod names (`validator-10-0` and others) in other namespaces. Always set
  `namespace="giga-testnet-0"`.
- Load pods carry `sei.io/chain-id`, not `sei.io/chain` and `sei.io/role`. Their streams have no
  `chain_id` and no `sei_role`. Select them with `container="seiload"`.

`seid` and `sei-sidecar` write logfmt with uppercase levels
(`level=ERROR msg="..." logger=tendermint/node`); filter with `| logfmt | level="ERROR"`. Panics
come as raw Go traces. `seiload` writes Go standard log lines. `keda-operator` writes
tab-separated lines with a JSON tail.

### Quiet logs

A healthy `seid` writes nothing while it runs. Over days of observation, each `seid` line was one of
these: the wrapper's start lines (`sidecar ready, starting seid`), a panic and its stack, or an
`ERROR` at shutdown (`problem closing blockstore ... leveldb: closed`). No `INFO` or `WARN` line
appeared, even at start. An empty last hour is normal. Widen the range back to the node's last
start. **pod-age-hours**:

```promql
sort_desc((time() - max by (cluster, pod) (kube_pod_start_time{namespace="giga-testnet-0", pod=~"validator-.*|fullnode-.*"})) / 3600)
```

If `up` is 1 and **lag-per-node** moves, a quiet node runs and has nothing to say.

### Recipes

Datasource `Loki-fleet`. Use the window the user asked for. If it is empty, widen it back to the
node's last start (**pod-age-hours**). A Go panic is a raw trace, not logfmt: a `level="ERROR"`
filter drops it, so also search `|= "panic:"`.

**log-node**, one node:

```logql
{cluster="prod-use2", namespace="giga-testnet-0", pod="validator-05-0", container="seid"}
```

**log-triage**, the runbooks' filter for one pod:

```logql
{cluster="<cluster>", namespace="giga-testnet-0", pod="<pod>", container="seid"} |~ "(?i)panic|error|pruned|dial"
```

**log-errors-per-pod**:

```logql
sum by (cluster, pod) (count_over_time({namespace="giga-testnet-0", container="seid"} | logfmt | level="ERROR" [24h]))
```

**log-panics**. The same `panic:` line at each restart is a crash loop. The first line after
`panic:` names the cause.

```logql
sum by (cluster, pod) (count_over_time({namespace="giga-testnet-0", container="seid"} |= "panic:" [24h]))
```

**log-load-tps**, offered load per cell from the load pods' reports. In a report, `max=` is the
largest latency since the pod started.

```logql
sum by (cluster) (avg_over_time({namespace="giga-testnet-0", container="seiload"} |= "throughput tps=" | regexp "tps=(?P<tps>[0-9.]+)" | unwrap tps [5m]))
```

**log-timeouts-by-owner**. sei-load logs each failed send, and the proxy error names the owner's
NLB hostname. This recipe maps forward timeouts to a validator name. Keep the range short: a
lagging owner makes millions of lines an hour.

```logql
topk(5, sum by (owner) (count_over_time({namespace="giga-testnet-0", container="seiload"} |= "shard owner did not answer in time" | regexp "Post \"http://(?P<owner>validator-[0-9]+)-p2p" [30m])))
```

**log-keda-formula**. A formula error has no metric; this line is the evidence:

```logql
{cluster="prod-use2", namespace="keda", container="keda-operator"} |= "scalingModifiers.Formula"
```

Each load pod logs `exemplar labels have 150 runes, exceeding the limit of 128`. It is harmless.
Drop it with `!= "exemplar labels"`.

## 3. Metrics by question

Conventions:

- Prometheus scrapes nodes every 30 s and load pods every 15 s.
- Every node executes every block, so take `avg` of an execution rate across validators. A lane
  counts only its own transactions, so take `sum` across producers.
- Add `cluster!="harbor"` to a fleet-wide `max`. Thanos also federates harbor.
- To compare with an earlier period, run the same query with `offset` (for example `offset 7d`),
  or graph it over the range. Read **pin-state** over the same range: a pin during the earlier
  period explains a different load.

**scrape-census**. Expect 12 `monitoring/seid` targets per cell and one
`monitoring/seiload-giga-testnet-0` target per load pod:

```promql
count by (cluster, job) (up{namespace="giga-testnet-0"} == 1)
```

### 3.1 Is the chain moving?

`tendermint_internal_autobahn_avail_commit_global_block_number` is a node's view of the committed
head. The fleet head is the `max` over the 40 validators.

**head**:

```promql
max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"})
```

**head-rate**, blocks per second:

```promql
deriv(max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"})[10m:30s])
```

**window-eta**, hours until the head passes the 4,320,000-block peer window. A negative value means
that it has passed.

```promql
(4320000 - max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"})) / scalar(deriv(max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"})[30m:1m])) / 3600
```

**prune-watermark**. It reads 0 until the block store prunes its first block:

```promql
max by (cluster) (tendermint_autobahn_blockstore_prune_watermark{namespace="giga-testnet-0", cluster!="harbor"})
```

- **Normal.** About 100 blocks/s below saturation. Near saturation, blocks fill and the rate falls,
  roughly to 55 to 85 blocks/s (measured). Read it with **txs-per-block**.
- **Trap.** Take the head across all 40 validators, never from one cell or the suspect node. A
  validator far behind can stop reporting the series.

### 3.2 Is each node keeping up?

`tendermint_internal_autobahn_data_next_block{stage}` is the next block that a node has not yet
passed through a stage: `qc`, `receive`, `execute`, `certify` or `evict`. Execute lag is the head
minus a node's `execute` height.

**nodes-ready**, nodes with a scrape and a Ready pod. Expect 10 validators and 2 full nodes per
cell:

```promql
count by (cluster, sei_role) ((max by (cluster, pod, sei_role) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", stage="execute"})) and on (cluster, pod) (kube_pod_status_ready{namespace="giga-testnet-0", condition="true"} == 1))
```

**lag-per-validator**, the alert form. `last_over_time` bridges a pod restart:

```promql
scalar(max(last_over_time(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"}[5m]))) - max by (cluster, namespace, chain_id, pod) (last_over_time(tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", stage="execute"}[5m]))
```

**lag-per-node**, validators and full nodes, worst first:

```promql
sort_desc(scalar(max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"})) - max by (cluster, pod, sei_role) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", stage="execute"}))
```

**lag-trend**, each validator's lag now minus its lag 10 minutes ago. A healthy validator moves by
tens of blocks, from scrape noise. A positive value in the thousands means the validator loses
ground:

```promql
sort_desc((max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"}) - on() group_right() max by (pod) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", stage="execute"})) - (max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"} offset 10m) - on() group_right() max by (pod) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", stage="execute"} offset 10m)))
```

**lag3-per-cell**, the scaler's `lag3` (third-worst validator per cell), through Thanos:

```promql
min by (cluster) (topk by (cluster) (3, scalar(max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", sei_role="validator"})) - max by (cluster, pod) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", sei_role="validator", stage="execute"})))
```

**exec-backlog**, `receive` minus `execute` on one pod. One scrape gives both, so no scrape skew:

```promql
sort_desc(max by (cluster, pod) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", stage="receive"}) - max by (cluster, pod) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", stage="execute"}))
```

**stage-heights**, one node's stages against the head (0 is the head):

```promql
max by (stage) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", pod="<pod>"}) - on() group_left() scalar(max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"}))
```

- **Normal.** Lag is never zero. The head and each pod come from different 30 s scrapes while the
  chain moves about 100 blocks a second. The median validator reads about 1,000 to 1,500 and the
  worst healthy one about 2,000 to 3,500. Full nodes read the same.
- **Normal.** **exec-backlog** reads tens of blocks.
- **Read.** Judge a lag by its trend over 10 to 30 minutes, as the alerts do. **exec-backlog** over
  about 1,000 and growing means that the node receives faster than it executes. All stages flat
  while the head moves means that the node stopped receiving blocks: a peer or network fault.
- **Trap.** A node without a scrape has no lag series, so every lag query and alert misses it.
  Check **nodes-ready**.

### 3.3 How loaded is the execute loop?

`sei_chain_autobahn_main_loop_phase_duration_seconds_total{phase}` counts the seconds that a node's
single execute loop spends in each bucket. [architecture.md](architecture.md) defines them:
`consensus` is the wait for the next block, `execution` is OCC execution inside `FinalizeBlock`,
and `storage` is everything after it until the next wait. Busy is everything outside `consensus`.

**busy-median**, the scaler's steering value per cell:

```promql
quantile by (cluster) (0.5, sum by (cluster, pod) (rate(sei_chain_autobahn_main_loop_phase_duration_seconds_total{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", phase!="consensus"}[2m])))
```

**busy-per-validator**, each validator's own busy, highest first:

```promql
sort_desc(sum by (cluster, pod) (rate(sei_chain_autobahn_main_loop_phase_duration_seconds_total{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", phase!="consensus"}[10m])))
```

**busy-edge**, validators with no margin:

```promql
sum by (cluster, pod) (rate(sei_chain_autobahn_main_loop_phase_duration_seconds_total{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", phase!="consensus"}[10m])) > 0.95
```

**exec-ms-per-validator**, execution milliseconds per block on each validator:

```promql
sort_desc(1000 * sum by (cluster, pod) (rate(sei_chain_autobahn_main_loop_phase_duration_seconds_total{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", phase="execution"}[10m])) / sum by (cluster, pod) (rate(tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", stage="execute"}[10m])))
```

**block-ms-by-phase**, fleet milliseconds per block in each bucket:

```promql
1000 * avg by (phase) (rate(sei_chain_autobahn_main_loop_phase_duration_seconds_total{namespace="giga-testnet-0", sei_role="validator"}[5m])) / scalar(avg(rate(tendermint_internal_autobahn_data_latency_count{namespace="giga-testnet-0", sei_role="validator", resource="blocks", stage="execute"}[5m])))
```

**storage-tail**, the share of wall time in each step after execution returns:

```promql
avg by (phase) (rate(sei_chain_autobahn_storage_tail_phase_duration_seconds_total{namespace="giga-testnet-0", sei_role="validator"}[5m]))
```

- **Normal.** **busy-median** stays between 0.70 and 0.90. Measured, busy is about
  0.11 + 0.89 x throughput / 98,000 tx/s. At about 72,000 tx/s a block costs about 5 ms of
  execution and 2.6 ms of storage. `vault_commit`, the hash-vault fsync, is the largest tail step
  at about 1 ms. `app_commit` only moves a cursor.
- **Read every validator, not only the median.** Some validators spend 30 to 40% more time per
  block than peers on the same instance type, even while caught up. With **busy-median** near
  0.76, such a validator can read 0.95 to 1.0 on **busy-per-validator**. The scaler does not see
  it. A validator on **busy-edge** has no margin and is the next straggler. Confirm with
  **exec-ms-per-validator**.
- **Danger.** At 0.95 or more **busy-median**, every validator is near the edge. A slower one falls
  behind and recovers only when the median drops below about 0.90.
- **Trap.** A validator that catches up reads about 1.0, because it never waits. With
  **lag-per-validator**: high busy and growing lag is a straggler; high busy and steady lag is no
  margin.
- **Trap.** The histogram `sei_chain_autobahn_main_loop_phase_latency_seconds` records `execution`
  and `storage` twice per block. Its `_sum / _count` reads half the true time. Use the counter
  divided by blocks per second.
- **Trap.** Full nodes report the series as `sei_role="node"`. They run less busy, and the scaler
  ignores them.

### 3.4 What throughput is the chain delivering?

**goodput**, successful transactions per second:

```promql
avg(sum by (cluster, pod) (rate(sei_chain_giga_evmonly_txs_executed_total{namespace="giga-testnet-0", sei_role="validator", status="success"}[5m])))
```

Add `status` to the `by` and drop its matcher to see `reverted`, `failed` and `rejected`.

**lane-production**, transactions packed into lanes:

```promql
sum(rate(tendermint_internal_autobahn_avail_produced_txs{namespace="giga-testnet-0", sei_role="validator"}[5m]))
```

**lane-per-producer**, slowest lane first:

```promql
sort(rate(tendermint_internal_autobahn_avail_produced_txs{namespace="giga-testnet-0", sei_role="validator"}[5m]))
```

**txs-per-block**:

```promql
sum(increase(tendermint_internal_autobahn_data_latency_count{namespace="giga-testnet-0", sei_role="validator", resource="txs", stage="execute"}[5m])) / sum(increase(tendermint_internal_autobahn_data_latency_count{namespace="giga-testnet-0", sei_role="validator", resource="blocks", stage="execute"}[5m]))
```

- **Normal.** With headroom, **goodput** equals the offered load: load pods x 2,000 tx/s. The
  measured ceiling with real execution is roughly 100,000 tx/s. **lane-production** matches it,
  and each lane carries about 1/40. **txs-per-block** reads a few hundred at about 70,000 tx/s
  and rises toward the 2,000-transaction lane cut near saturation.
- **Trap.** A `sum` of the executed counter reads 40 times the network rate.
- **Read.** A lane far below its peers points at its owner: admission, lag or forward timeouts.

### 3.5 How is the RPC layer doing?

A full node's EVM proxy forwards each send to the validator that owns the sender's shard. That
owner can sit in any region. The proxy waits up to 10 s.

**proxy-outcomes** (`ok`, `rpc_error`, `timeout`, `transport_error`, `canceled`; full nodes only):

```promql
sum by (cluster, outcome) (rate(tendermint_p2p_evm_proxy_requests{namespace="giga-testnet-0"}[10m]))
```

**timeouts-by-owner**. `owner` is a key, `validator:ed25519:public:<hex>`:

```promql
topk(5, sum by (owner) (rate(tendermint_p2p_evm_proxy_requests{namespace="giga-testnet-0", cluster!="harbor", outcome="timeout"}[10m])))
```

**proxy-latency**, median forward time per cell:

```promql
histogram_quantile(0.5, sum by (cluster, le) (rate(tendermint_p2p_evm_proxy_request_seconds_bucket{namespace="giga-testnet-0"}[5m])))
```

**inserts-by-result**, admission on the owners. `full` and `pending_full` mean a full lane mempool:

```promql
sum by (result) (rate(tendermint_internal_autobahn_producer_inserts{namespace="giga-testnet-0", sei_role="validator"}[5m]))
```

**capacity-waits**, validators whose inserts waited for mempool space:

```promql
sum by (cluster, pod) (rate(tendermint_internal_autobahn_producer_capacity_wait_latency_count{namespace="giga-testnet-0", sei_role="validator"}[5m])) > 0
```

- **Normal.** `ok` equals the offered load and `timeout` reads about 0. **inserts-by-result** shows
  only `ok`. **proxy-latency** follows distance: about 30 ms in prod-euw1 to about 170 ms in
  prod-apne1 (measured).
- **Read.** One owner with most timeouts does not answer, most often because it lags. About 1/40
  of senders then wait 10 s. **log-timeouts-by-owner** names it. **capacity-waits** on one
  validator marks a slow owner; check its **busy-per-validator**.
- **Trap.** `cluster:giga_validators_trapped:count` counts validators with any `pending_full`.
  Under real execution it reads 0 or 1 even near saturation. Use busy.

### 3.6 How is the load generator doing?

Each load pod sends at most 2,000 tx/s, with at most 1,500 sends in flight
([load-and-scaling.md](load-and-scaling.md)).

**send-latency-mean**, the scaler's `lat` per cell:

```promql
sum by (cluster) (rate(seiload_send_latency_seconds_sum{namespace="giga-testnet-0"}[2m])) / sum by (cluster) (rate(seiload_send_latency_seconds_count{namespace="giga-testnet-0"}[2m]))
```

**send-latency-p50**:

```promql
histogram_quantile(0.5, sum by (cluster, le) (rate(seiload_send_latency_seconds_bucket{namespace="giga-testnet-0"}[5m])))
```

**acceptance**, accepted over all sends:

```promql
sum by (cluster) (rate(seiload_txs_accepted_total{namespace="giga-testnet-0"}[2m])) / (sum by (cluster) (rate(seiload_txs_accepted_total{namespace="giga-testnet-0"}[2m])) + sum by (cluster) (rate(seiload_txs_rejected_total{namespace="giga-testnet-0"}[2m])))
```

**rejections-by-reason** (`rpc` or `mempool_full`):

```promql
sum by (cluster, reason) (rate(seiload_txs_rejected_total{namespace="giga-testnet-0"}[10m]))
```

**load-landed**, accepted load over offered load:

```promql
sum by (cluster) (rate(seiload_txs_accepted_total{namespace="giga-testnet-0"}[5m])) / on (cluster) (2000 * sum by (cluster) (kube_deployment_spec_replicas{namespace="giga-testnet-0", deployment="seiload-saturation"}))
```

**slow-sends**, sends per second over 10 s (forward timeouts):

```promql
sum by (cluster) (rate(seiload_send_latency_seconds_bucket{namespace="giga-testnet-0", le="+Inf"}[5m])) - sum by (cluster) (rate(seiload_send_latency_seconds_bucket{namespace="giga-testnet-0", le="10.0"}[5m]))
```

**nonce-recovery**, accounts that wait in a nonce re-read after a failed send:

```promql
sum by (cluster) (seiload_nonce_recovery_inflight{namespace="giga-testnet-0"})
```

**load-pods**:

```promql
max by (cluster) (kube_deployment_spec_replicas{namespace="giga-testnet-0", deployment="seiload-saturation"})
```

- **Normal.** Mean latency about 0.1 to 0.25 s, highest in prod-apne1, far from most owners.
  **acceptance** and **load-landed** about 1. **slow-sends** and **nonce-recovery** about 0.
  About 8 to 10 **load-pods** per cell.
- **Trap.** The p50 stays flat until overload. Under saturation the mean read 1.0 to 1.5 s while
  the p50 stayed near 0.12 to 0.19 s. The 10 s timeouts sit in the tail, so watch the mean.
- **Trap.** A timeout counts as an `rpc` rejection. One lagging owner fails about 1/40 of sends, so
  acceptance stays near 0.975 and no load alert fires.
- **Trap.** **load-landed** under 1 means that the in-flight budget binds. A pod sends at most
  1,500 / mean latency per second (Little's law). A mean over 0.75 s holds it under 2,000.
- **Trap.** The `endpoint` label on `seiload_*` series reads `metrics`, not the RPC target.

### 3.7 What is the scaler deciding?

[load-and-scaling.md](load-and-scaling.md) owns the triggers and the formula. These queries show
what the scaler reads and does.

**scaler-values**, each trigger's last value. `scaler="composite-metric"` is the pod count that the
formula asks for:

```promql
max by (cluster, scaler) (keda_scaler_metrics_value{exported_namespace="giga-testnet-0", scaledObject="seiload-saturation"})
```

**scaler-step**, the pending change per cell. The HPA acts on a nonzero value after its 180 s
window:

```promql
max by (cluster) (keda_scaler_metrics_value{scaledObject="seiload-saturation", scaler="composite-metric"}) - on (cluster) max by (cluster) (kube_deployment_spec_replicas{namespace="giga-testnet-0", deployment="seiload-saturation"})
```

**pin-state**, 1 while a pin holds:

```promql
max by (cluster) (keda_scaled_object_paused{exported_namespace="giga-testnet-0", scaledObject="seiload-saturation"})
```

**hpa-not-active**, 1 when the HPA cannot read its metric:

```promql
max by (cluster) (kube_horizontalpodautoscaler_status_condition{namespace="giga-testnet-0", horizontalpodautoscaler="keda-hpa-seiload-saturation", condition="ScalingActive", status="false"})
```

**scaler-errors**, failed trigger queries:

```promql
sum by (cluster, scaler) (rate(keda_scaler_detail_errors_total{scaledObject="seiload-saturation"}[5m]))
```

- **Normal.** `composite-metric` equals `pods`. `busy` reads 0.70 to 0.90 and `lat` under 0.6.
  `accept` reads about 1, `ready` 10 and `fnready` 2. `lag3` and `fnlag` read about 1,000 to
  3,000. **pin-state**, **hpa-not-active** and **scaler-errors** read 0.
- **Trap.** A formula error moves no KEDA counter. The HPA holds its count and no hard stop acts.
  Only **hpa-not-active** and **log-keda-formula** show it.
- **Trap.** Fallback has no metric; read the ScaledObject's `FALLBACK` column. One failed poll
  makes the object inactive, so the 30 s cooldown can reach 0 pods first. The fallback engages
  after more than 3 failed polls.
- **Trap.** While a pin holds, KEDA can remove the HPA and **scaler-values** goes absent.
- **Trap.** A trigger query has no `cluster` matcher. Reproduce it on the cell's Prometheus, or add
  `by (cluster)` on Thanos.

## 4. Dashboards

**Autobahn E2E**:
`https://grafana.prod.platform.sei.io/d/autobahn-e2e/autobahn-e2e?var-namespace=giga-testnet-0&var-chain_id=giga-testnet-0`

| Row | Panels that matter |
|---|---|
| Overview | TPS (executed), Blocks / sec, Production -> execute p50 and worst-validator p99 |
| Validator lag (collapsed) | Lag behind chain head (one panel per cell); Stage heights (one per pod, so pick a pod) |
| Throughput, Lane production | Tx QPS, Txs / block, produced tx/s per producer |
| Latency breakdown, Storage | Main-loop buckets per validator; state commit, FlatKV, Pebble compaction |
| Pipeline stages, Producer ingest | Execute lag behind receive; inserts by result; capacity wait |

**Load Test Client**:
`https://grafana.prod.platform.sei.io/d/ae5fc894-d19f-4ae9-8dbc-d968aa0eab3d/load-test-client?var-ChainID=giga-testnet-0`.
Use Transaction Throughput and Transaction Send Latency.

Known gaps:

1. The lag panel draws its runaway line at 5,000. `GigaValidatorRunawayLag` fires at 12,000.
2. The Trapped validators text says that 13 to 18 validators trap under saturation. That describes
   the mock app. Under real execution the count reads 0 or 1.
3. No panel shows **busy-median** or **busy-per-validator**. "Main loop time spent" is a mean, and
   the per-validator panel draws execution and storage as separate lines.
4. No panel shows the scaler, the pin, the load pods or the EVM proxy. No KEDA dashboard exists.
5. The two "Consensus finalize" stats show the top finite bucket (about 4.9 s), not a latency.
   Every proposal-to-commit observation falls in `+Inf`. **finalize-overflow** reads 0 while
   this holds:
   ```promql
   sum(rate(tendermint_internal_autobahn_avail_proposal_to_commit_latency_bucket{namespace="giga-testnet-0", sei_role="validator", le="4.922235242952021"}[5m])) / sum(rate(tendermint_internal_autobahn_avail_proposal_to_commit_latency_count{namespace="giga-testnet-0", sei_role="validator"}[5m]))
   ```
6. The "Txs / block" description says that it reads empty on EVM-only builds. It returns data.
7. Load Test Client has no `cluster` variable. Its `Endpoint` list holds only `metrics`. Its block,
   gas, receipt and worker panels are empty for this chain.
8. Giga Release and Giga OCC list chains from `tendermint_consensus_latest_block_height`, which
   Autobahn nodes do not emit, so giga-testnet-0 is not in their list.

## 5. Alerts and blind spots

| Alert | Fires when | `for` | Severity | Runbook |
|---|---|---|---|---|
| `GigaValidatorRunawayLag` | Lag > 12,000 and wider than 10m ago | 2m | warning | [giga-validator-runaway-lag](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/giga-validator-runaway-lag.md) |
| `GigaValidatorFallingBehind` | Lag > 20,000 and wider than 30m ago | 15m | warning | [giga-validator-falling-behind](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/giga-validator-falling-behind.md) |
| `GigaValidatorNearBlockWindow` | Lag > 2,160,000, half the peer window | 10m | critical | [giga-validator-near-block-window](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/giga-validator-near-block-window.md) |
| `GigaFullnodeFallingBehind` | Full-node lag > 20,000 and wider than 30m ago | 15m | warning | None |
| `GigaNodeNearMockAppRetention` | Lag > 6,000 on a mock-app node | none | warning | None; cannot fire under real execution |
| `SeiLoadGeneratorNotSending` | No sends for 10m while the Deployment wants pods | none | warning | [platform README](https://github.com/sei-protocol/platform/blob/main/manifests/base/monitoring/alerts/sei-load/README.md) |
| `SeiLoadGeneratorMetricsAbsent` | No send series for 15m while the Deployment wanted pods | none | warning | platform README |
| `SeiLoadGeneratorSendsRejected` | Over half of sends rejected, `mempool_full` excluded, over 10m | 5m | warning | platform README |
| `SeiLoadGeneratorNonceRecoveryStalled` | Over 50 accounts in recovery and under 1 tx/s accepted | 10m | warning | [seiload-generator-nonce-recovery-stalled](https://github.com/sei-protocol/runbooks/blob/main/platform/sei-load/seiload-generator-nonce-recovery-stalled.md) |
| `KedaScalerErrors` | A trigger query fails in every 5m window | 15m | warning | [keda-scaler-errors](https://github.com/sei-protocol/runbooks/blob/main/platform/keda/keda-scaler-errors.md) |
| `KedaMetricsAPIServiceUnavailable` | The external metrics APIService is down | 5m | warning | [keda-metrics-apiservice-unavailable](https://github.com/sei-protocol/runbooks/blob/main/platform/keda/keda-metrics-apiservice-unavailable.md) |
| `KubeHpaMaxedOut` | The load HPA sits at 30 pods | 15m | warning | upstream `kubernetes-mixin` |
| `KubePodCrashLooping` | A container is in `CrashLoopBackOff` | 15m | warning | upstream `kubernetes-mixin` |

Thanos Ruler in `prod` evaluates the five `Giga*` rules each minute and sends to the `prod`
Alertmanager. Each cell's Prometheus evaluates the rest and sends to its own Alertmanager. By the
platform convention only `critical` pages. Rule files in `sei-protocol/platform`:
`clusters/prod/monitoring/alerts/platform/alerts-giga-validator-lag.yaml`,
`manifests/base/monitoring/alerts/sei-load/`,
`clusters/<cell>/monitoring/alerts/sei-load/` and `manifests/base/monitoring/alerts/keda/`.

**alert-history**, every cell's giga alerts. The Grafana `Alertmanager` datasource shows only
`prod`.

```promql
count by (alertname, cluster, severity) (max_over_time(ALERTS{alertstate="firing", alertname=~"Giga.*|SeiLoad.*|Keda.*|KubeHpa.*"}[2d]))
```

Blind spots:

1. **A formula error fires no alert.** The load runs on with no brake. See **hpa-not-active**.
2. **A pin left in place fires no alert.** No hard stop acts, and the load rules skip a Deployment
   that wants 0 pods. No rule reads **pin-state**.
3. **Saturation fires no alert.** No rule reads busy. If the scaler cannot act, the first alert is
   a validator that already runs away.
4. **A validator with no margin fires no alert** until its lag passes 12,000. Check **busy-edge**.
5. **An unscraped node is invisible.** `TargetDown` is disabled fleet-wide, and no rule watches the
   node scrape. Check **nodes-ready**.
6. **No head, no lag.** If no validator reports the head, `scalar()` returns NaN and no lag alert
   fires.
7. **One lagging owner stays under every load threshold.** It fails about 1/40 of sends. Only its
   lag alerts, **timeouts-by-owner** and **slow-sends** show it.
8. **Full nodes have thin coverage.** `GigaFullnodeFallingBehind` has no runbook, and the scaler
   ignores full-node busy.
9. **No log-coverage alert covers giga nodes.** `LokiCoverageZeroOnSeiNode` selects only arctic-1,
   pacific-1 and atlantic-2. Healthy nodes are quiet, so an empty log query proves nothing.
10. **The lag rules depend on one evaluator.** If Thanos Ruler in `prod` stops, or a cell's sidecar
    link fails, they go quiet for the affected validators.
