# Sei + Kubernetes Signal Ladder

The first signals to retrieve for any Sei-platform incident, in order. The lower rungs localize the failure and frequently reveal hypotheses you wouldn't have written. Never skip rungs to chase a favored hypothesis.

## MVP deployment envelope (Grafana-MCP-only) — read first

In the current sei-omnigent MVP (Design 20) your **only** signal source is the Grafana MCP: metrics (Prometheus) and logs (Loki). No shell, no `kubectl`, no `seid`, no `curl`, no node RPC. Read the rungs below through this mapping — don't attempt a denied command.

| Rung | MVP path | Tool |
|---|---|---|
| 1. `describe pod` / events | out of envelope; restart/OOM/pressure survive as kube-state metrics (`kube_pod_container_status_restarts_total`, `kube_pod_status_phase`) — raw Events do not | `grafana__query_prometheus` |
| 2. `logs [--previous]` | every pod's logs ship to Loki (alloy-logs); LogQL bounded to the crash/incident window | `grafana__query_loki_logs` |
| 3. `seid status` sync_info | sync/height as metrics (`tendermint_consensus_latest_block_height`, catching-up series) | `grafana__query_prometheus` |
| 4. `net_info`/`dump_consensus_state` | peers + rounds as metrics (`tendermint_p2p_peers`, `tendermint_consensus_rounds`); raw round-state is out of envelope | `grafana__query_prometheus` |
| 5. Prometheus RED query | native | `grafana__query_prometheus` |

Datasource UIDs: **`prometheus`** (metrics), **`loki`** (logs). Bound every query to ±15 min of onset.

**Out of envelope → punt cleanly (Step 6), never fake.** Pod object-state/events, raw CometBFT round-state, `pprof`, `kubectl top`/node views, and `seid query` are unreachable here. If a hypothesis's decisive gate needs one, state the obstacle ("requires a cluster-state/RPC signal deferred to the read-only-k8s increment") — the metric substitutes above *localize*, they do not stand in for a raw object/RPC read at a final gate. Never substitute a weaker signal silently or fabricate output.

## The first five commands

Run these before you have a hypothesis. They establish the search space.

### 1. `kubectl describe pod <pod> -n <ns>`

The Events section is the highest-yield single signal in Kubernetes. Captures:

- Restart counts and reasons (`OOMKilled`, `Error`, `CrashLoopBackOff`)
- Image pull failures, scheduling reasons, eviction events
- Probe failures (liveness, readiness, startup) with exact timestamps
- Resource pressure annotations

If you skip this, you will re-derive what describe already told you.

### 2. `kubectl logs <pod> -n <ns> --previous --tail=200`

For `CrashLoopBackOff`, `--previous` is non-negotiable. The current instance's logs are post-restart noise; the previous instance's logs contain the actual crash. Pair with current-instance logs for context.

For long-running pods that haven't crashed but are misbehaving, drop `--previous` and bound with `--since=15m` (or whatever brackets the symptom).

### 3. `seid status | jq '.sync_info'`

(Or `curl -s :26657/status | jq '.result.sync_info'` if querying directly.)

Separates three failure classes:

- `catching_up: false` + `latest_block_height` advancing → node is alive and on the head. Problem is elsewhere (RPC, mempool, app layer).
- `catching_up: true` → node is behind. Investigate state sync, peer connectivity, disk I/O.
- `latest_block_height` static → node is wedged. Investigate consensus (next rung).

### 4. `curl -s :26657/net_info | jq '.result.peers | length'` + `curl -s :26657/dump_consensus_state | jq '.result.round_state'`

Peer count answers "am I isolated?" `dump_consensus_state` answers "do I see the same proposal/votes as the rest of the network at this height?"

Per Sei docs and CometBFT operational guidance, **AppHash mismatch** is a frequent root cause that *presents* as peer-connection symptoms — the node disconnects from peers because it computed a different state root and they reject its votes. Always grep the previous logs for `apphash` (case-insensitive) before blaming networking.

### 5. Prometheus query bounded to the incident window

For the RED triad on the affected service, scoped to `±15 min` of the symptom onset. Examples:

```promql
# Request rate
sum(rate(http_requests_total{job=~"<svc>"}[1m]))

# Error rate
sum(rate(http_requests_total{job=~"<svc>",status=~"5.."}[1m]))
  / sum(rate(http_requests_total{job=~"<svc>"}[1m]))

# p99 latency
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job=~"<svc>"}[1m])) by (le))
```

Aggregates here are for **localization**, not for declaring root cause. If the answer matters, descend from the metric to the trace to the log line — pre-aggregated metrics permanently discard the connective tissue you need to attribute the spike to a specific request.

## Ladder extensions

For harder incidents:

- **`kubectl get events -n <ns> --sort-by=.lastTimestamp`** — cluster-level context the pod's own Events section misses (controller messages, admission webhook denials, autoscaler decisions).
- **`seid query staking validator $(seid tendermint show-validator)`** — verify the validator is bonded and participating with non-zero voting power. A bonded validator whose voting power dropped to zero is invisible to consensus.
- **`kubectl top pod -n <ns> --containers`** + **`kubectl describe node <node>`** — resource pressure at the pod and node level. Combined with the USE-method lens below.
- **`go tool pprof` against `http://<pod>:<pprof-port>/debug/pprof/profile`** — CPU/heap profiles when the symptom is "process is hot" or "memory growing." Requires pprof exposed in the controller / sidecar / sei-chain binary.

## Framework mapping

Different signal frameworks apply to different layers. For a Sei-stack incident, **all of these are simultaneously relevant** because a Sei node is a resource, a service, *and* a consensus participant.

### USE method (Brendan Gregg) — for resources

For every resource: **U**tilization, **S**aturation, **E**rrors. Check errors first (cheap, often dispositive).

Resources to enumerate for a Sei node:

- CPU (utilization, run-queue saturation, throttling errors if cgroup limits are set)
- Memory (utilization, swap pressure, OOM events)
- Disk (utilization for SeiDB, iowait saturation, IO errors)
- Network (bandwidth utilization, TX/RX queue saturation, packet drops)
- File descriptors (open vs. limit, EMFILE errors)
- Mutex / RWMutex contention in the binary (visible via pprof contention profile)

### RED method (Tom Wilkie) — for services

For every request-driven service: **R**ate, **E**rrors, **D**uration. Applies to:

- EVM JSON-RPC (`eth_call`, `eth_blockNumber`, `eth_getTransactionReceipt`, etc.)
- CometBFT RPC (`/status`, `/abci_query`, `/broadcast_tx_*`)
- Sei controller's reconcile loop (treat each reconciliation as a request)
- Sidecar's proxied requests

### Four Golden Signals (Google SRE) — for user-facing systems

Latency, Traffic, Errors, Saturation. RED + an explicit saturation read. Use this lens when the symptom is user-visible (RPC clients impacted, dashboard alarms triggered).

### The fourth axis: consensus liveness

USE/RED/Golden Signals do not cover **consensus participation**. For sei-chain, the fourth axis is:

- Block height progression (is `latest_block_height` monotonically advancing?)
- Voting power participation (is this validator's vote being included?)
- AppHash consistency (does this node's AppHash match peers at the same height?)
- Round count per height (rounds > 0 means consensus is struggling)

These are CometBFT-specific. They are not optional for a sei-chain investigation.

## Anti-patterns

- **Citing log content without running the command.** "The logs probably show…" — fabrication. Run `kubectl logs` and paste the verbatim excerpt.
- **Quoting a dashboard without scoping to the incident window.** A 24-hour graph hides a 30-second incident. Bound to `±15 min` of onset.
- **Inferring causation from aggregates.** A p99 spike correlates with a deploy — but the trace for the slow request shows it hit a different code path. Descend to per-request data before attributing.
- **Skipping `--previous` on `kubectl logs` for crash loops.** The current-instance logs are post-restart noise. The previous instance's logs are where the crash is.
- **Trusting `seid status` alone for a healthy-looking node that's actually misbehaving.** A node can be on the head height-wise and still be returning stale state from a corrupted SeiDB. Pair with a `eth_call`-equivalent state read and cross-check against a known-good peer.
