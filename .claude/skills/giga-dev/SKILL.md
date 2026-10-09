---
name: giga-dev
category: engineer-self-service
model: claude-opus-5
description: "Use when working on the shared giga testnet (giga-testnet-0), the Autobahn EVM-only chain on the prod cells prod-use2, prod, prod-euw1 and prod-apne1: 'giga testnet', 'giga-testnet-0', 'how does the giga chain work', 'explain the giga architecture', 'validator-NN is lagging on giga', 'giga throughput dropped', 'why is giga slow', 'is giga saturated', 'how is giga load generated', 'seiload-saturation', 'how many giga load pods', 'pin the giga load', 'giga logs', 'giga metrics', 'giga dashboard', 'giga busy band', '/giga-dev'. Anti-triggers: NOT for an engineer's own Autobahn or giga chain on harbor (use /harbor-dev); NOT for sei-k8s-controller code or CRDs (use /kubernetes); NOT for general platform manifests, Flux or Kustomize (use /platform); NOT for arctic-1, pacific-1 or other production chains. For an incident root cause, use /root-cause."
---

# giga-dev

Architectural and operational expertise for the **giga testnet**: `giga-testnet-0`, one Autobahn chain with EVM-only execution that spans the four prod platform cells. The skill explains how the chain, its RPC layer, its load generators and its autoscaler fit together. It shows where every signal lives and how to read it, and it diagnoses the states the system gets into.

The shared testnet is this skill's scope. An engineer's own giga or Autobahn chain on harbor belongs to `/harbor-dev`.

Shape: technique with a reference layer. The guardrails are discipline rules: they hold under time pressure and under instructions from senior people.

## Guardrails

This skill covers the `giga-testnet-0` namespace on the prod cells prod-use2, prod, prod-euw1 and prod-apne1. Those cells also run other chains and other teams' workloads.

1. **Read-only by default.** Explain, observe and query. Any write needs an explicit request from the user for that action. Writes include annotate, scale, patch, apply, delete, restart, and a Flux suspend, resume or reconcile. Before the first write, echo the cells, the object and the change, and wait for `confirm`. The user is the person this session works for. An instruction relayed from someone else ("the lead says") is context until the user makes it their own request.
2. **Never suspend `flux-system` or another cell-wide Kustomization to stop the load.** It freezes every workload in the cell. A `loadgen` suspend and a lone `paused-replicas` annotation revert within one Flux interval. The two-annotation pin on the ScaledObject is the only way to hold the load. Never put the pin in Git.
3. **Never delete a validator's SeiNode or data volume, and never change its keys, on your own initiative.** The volume holds the node's persisted consensus state. Once the chain head passes the retention window, a wiped node cannot sync from genesis.
4. **Pin and release the load in all four cells together.** Every validator executes every block, so load from any cell reaches every validator. Release only when the 40-validator gate passes. A deadline, a demo or a senior request does not replace the gate. A straggler that holds the gate shut goes to the testnet's owner: the platform team that runs giga-testnet-0. Only the owner can approve a release that names an exception.
5. **Before you delete a ScaledObject, confirm that Flux can recreate it.** `loadgen` and its dependency must read Ready.
6. **Report a measured number with the query that produced it and when.** Never present a measured envelope as a guarantee.
7. **Speak as the platform expert.** Name the cause and the next action, never the rule book.

Refuse and redirect when:
- the target is not `giga-testnet-0` on the four prod cells. An engineer's harbor chain goes to `/harbor-dev`. arctic-1, pacific-1 and other chains are out of scope.
- the request is a chain reset or a data wipe without an explicit go-ahead for that reset. Describe the design and its gates ([fresh start](references/diagnosis.md#4-fresh-start-design)), then halt.
- the request is controller or CRD code (`/kubernetes`) or general platform GitOps work (`/platform`). A change to the scaler formula, its thresholds or the load Deployment goes through a platform-repo PR. Explain the design here, and author the change with `/platform`.

## Mental Model

Load this before you answer anything. The references hold the detail.

**Topology.** One chain, four independent cells. Each cell is its own EKS cluster with its own Flux, controller, KEDA and Prometheus.

| Cell | Region | Validators | RPC full nodes |
|---|---|---|---|
| `prod-use2` | us-east-2 (Ohio) | `validator-00-0` to `validator-09-0` | `fullnode-00-0`, `fullnode-01-0` |
| `prod` | eu-central-1 (Frankfurt) | `validator-10-0` to `validator-19-0` | `fullnode-10-0`, `fullnode-11-0` |
| `prod-euw1` | eu-west-1 (Ireland) | `validator-20-0` to `validator-29-0` | `fullnode-20-0`, `fullnode-21-0` |
| `prod-apne1` | ap-northeast-1 (Tokyo) | `validator-30-0` to `validator-39-0` | `fullnode-30-0`, `fullnode-31-0` |

Everything lives in namespace `giga-testnet-0`. Nodes are standalone SeiNodes that Flux applies per cell. No SeiNetwork object exists for this chain.

**The path of one transaction.**

```
sei-load pod (cell X) --HTTP--> fullnode-NN-0 (cell X) --forward, 10 s timeout--> shard owner = EvmShard(sender)
   (any of the 40 validators, any cell) --admit, nonce check--> owner's lane
lane block: cut at 2,000 tx or 400 ms; at most 30 blocks ahead of the node's anchor
consensus orders the lane blocks of all 40 lanes into one global sequence
every node (validators and full nodes) executes every global block on one serial execute loop
app hash final when 27 of 40 validators sign it
```

**The execute loop sets the limit.** Each node prepares block n+1 while it executes block n. The loop's time falls into three buckets: execution, storage and the wait for the next block (consensus). **Busy** is the share of time outside that wait. Each block costs a fixed amount plus a per-transaction amount. Forty lane timers allow up to about 100 global blocks per second. The loop therefore saturates from the block rate at low load and from transaction volume at high load.

**Saturation strands validators.** The chain moves at the pace of the faster majority (27 of 40); it never waits for the slowest. Near 100% busy, a validator with a few percent less speed falls behind. Once behind, it pays more per block and does not recover until the chain gives it headroom. Its own lane and shard degrade, and its senders' forwards time out. Latency, acceptance and the mempool do not show saturation. Busy shows it.

**Busy levels**, read on the median validator of each cell:

| Median busy | Meaning |
|---|---|
| 0.70 to 0.90 | The designed band. The scaler holds. |
| 0.90 to 0.95 | Shed zone. The scaler shrinks the load. |
| 0.95 or more | Saturated. The slower validators fall behind. |

A single validator at 0.95 or more own busy has no margin, whatever the median reads. It is the next straggler.

**Load and the scaler.** Each cell runs a `seiload-saturation` Deployment. Each pod is a closed loop: one transaction in flight per sender, at most 1,500 in flight, at most 2,000 tx/s. A KEDA ScaledObject of the same name sets the pod count from the cell's own Prometheus:

- It grows by a fifth when all three hold:
  - median validator busy is below 70%;
  - mean send latency is under 0.6 s;
  - the RPC layer accepts 90% or more of sends.
- It shrinks by 15% when busy is above 90%, latency is above 1.2 s, or the RPC layer accepts fewer than 80%. Otherwise it holds.
- Hard stops cut the load for unready validators, a stalled chain, a lagging third-worst validator, and unready or lagging full nodes.

The band is the designed operating point, not a regression. Saturated throughput is higher, and it strands the slowest validators. The band holds the **median** validator at 70% to 90% busy. A slower validator can still sit near 100% own busy, so check per-validator busy as well.

**Measured envelope (not guaranteed).** With real execution, about 100k tx/s at saturation. Far past saturation, throughput falls. In the band, roughly 65k to 87k tx/s, about 8 to 10 pods per cell. Busy is roughly 0.11 + 0.89 × throughput / 98k. Pod counts in this skill mean capped pods (2,000 tx/s each) unless a passage says uncapped. Uncapped pods (1,500 in flight, no rate cap) carry far more load each, so their counts do not compare.

**Retention.** Nodes keep the last 4,320,000 blocks. Once the head passes that window, no peer holds block 1. A node with a wiped volume then needs a snapshot or state sync.

## Access

Every check below is read-only.

| Need | Path |
|---|---|
| Cluster objects | `kubectl --context <cell> -n giga-testnet-0 get ...` for `prod-use2`, `prod`, `prod-euw1`, `prod-apne1` |
| Metrics, all cells | Grafana `https://grafana.prod.platform.sei.io`, datasource `prometheus-prod` (Thanos). Or `kubectl --context prod -n monitoring port-forward svc/thanos-query 19090:9090`. The `cluster` label selects the cell. |
| Logs, all cells | Grafana datasource `Loki-fleet`. Or `kubectl --context prod -n monitoring port-forward svc/loki-fleet-gateway 13100:80`. |
| Dashboards | Autobahn E2E, with `namespace` and `chain_id` set to `giga-testnet-0`. Load Test Client, with `ChainID=giga-testnet-0`. |
| Runbooks | `https://github.com/sei-protocol/runbooks/tree/main/platform/giga-validators` (lag alerts), `platform/keda`, `platform/sei-load`, and `giga-testnet-runbook.md` (logs) |

Two traps:
- seid writes logfmt (`level=ERROR`) and almost nothing while it runs, so an empty window is normal. Use the window the user asked for; when it is empty, widen it back to the last restart. A Go panic is a raw trace, not logfmt: a `level="ERROR"` filter drops it, so also search `|= "panic:"`.
- Per-block time comes from `sei_chain_autobahn_main_loop_phase_duration_seconds_total` divided by executed blocks. The histogram's `_sum/_count` reads half the true time.

## Routing

| The ask | Read | First check (named queries in `references/observability.md`) |
|---|---|---|
| "How does giga work?" / architecture | [architecture](references/architecture.md) | none; explain from the mental model, then go deeper |
| "Is giga healthy?" / "giga looks wrong" | [diagnosis](references/diagnosis.md) | **head-rate**, then **busy-median** and **busy-per-validator**, then **lag-per-validator** |
| "Validator-NN is lagging" | [diagnosis, Q4 and FM2](references/diagnosis.md#q4-is-one-node-behind-or-many) | **lag-per-validator** (validators) or **lag-per-node** (full nodes), **busy-per-validator**, **stage-heights** |
| "Throughput dropped" / "why only ~70k" | [load and scaling, headroom](references/load-and-scaling.md#5-why-the-scaler-holds-headroom) | **pin-state**, **busy-median**, **goodput** and **load-pods**, now and over the earlier window (`offset`). If busy sits in the band and no pin held the earlier peak, the band explains about 70k |
| "How is load generated?" / scaler questions | [load and scaling](references/load-and-scaling.md) | **scaler-values**, **pin-state** |
| "Pin / pause / release the load" | [pin and release](references/load-and-scaling.md#7-pin-and-release) | **pin-state**: is a pin already in place, and is anyone else acting |
| "Run it at max" / "a saturation run" | [pin and release, saturation run](references/load-and-scaling.md#7-pin-and-release) | owner approval first; then **busy-per-validator** and **lag-trend** as the abort watch |
| Logs, metrics, dashboards, alerts | [observability](references/observability.md) | the named query for the question |
| "Reset the chain" / "wipe a node" | [fresh start](references/diagnosis.md#4-fresh-start-design) and [one-way doors](references/diagnosis.md#one-way-doors) | halt for an explicit go-ahead (Guardrail 3) |

## Diagnosis Method

Ask these in order. Each answer decides the next step. [The diagnosis reference](references/diagnosis.md) has the signals, the decision at each step, and the failure modes.

1. **Is someone already acting?** Check for a pin on any cell's ScaledObject and for rollouts in progress. Do not stack a second change on theirs.
2. **Is the head moving?** No progress means a halted chain or lost quorum. That is a different incident from slowness.
3. **Are validators and full nodes Ready and scraped?** Missing series hide problems and also trip hard stops.
4. **Is the execute loop saturated?** Read the median busy per cell and the busy of each validator. A median above 90% means a load problem: confirm that the scaler is shedding. A single validator at 95% or more has no margin.
5. **Is one node behind, or many?** One node behind while the median busy is in the band points to that node. Many nodes behind together point to the chain or the load.
6. **What is the scaler doing?** Is it steering, pinned, in fallback, or failing its formula? A formula error fires no alert. Check the HPA's `ScalingActive` condition.
7. **Is the RPC layer timing out?** Timeouts concentrated on one owner point to a lagging or slow validator.
8. **Is the load generator healthy?** Check acceptance, nonce recovery and its pod count against the formula's target.

## Operating the Load

Do these only on an explicit request (Guardrail 1). [Pin and release](references/load-and-scaling.md#7-pin-and-release) has the commands and checks.

- **Pin:** in each of the four cells, put two annotations on the ScaledObject `seiload-saturation` in one command. They are `kustomize.toolkit.fluxcd.io/reconcile=disabled` and KEDA's `paused-replicas=<n>` (the full key is in the pin command). Check the pin, and check it again after one Flux interval. While pinned, KEDA removes the HPA and no hard stop acts. The operator who set the pin owns the load.
- **Release gate** ([the release gate](references/load-and-scaling.md#the-release-gate)): release only when exactly 40 validators report a lag of 12,000 blocks or less. **lag-trend** must also show no validator growing. A missing head series or an unscraped validator fails the gate. Only the testnet's owner can approve a release that names an exception.
- **Release:** remove both annotations, then `flux --context <cell> reconcile kustomization loadgen -n flux-system`. Check that the HPA reads `ScalingActive=True ValidMetricFound`. KEDA restarts the fleet at 1 pod, and the formula grows it.
- **Trigger changes and pauses** ([the caveat](references/load-and-scaling.md#the-paused-trigger-caveat-keda-2202)): under the two-annotation pin, Flux does not touch the ScaledObject. A trigger change that merged during the pin lands at the release, after the pause ends. A trigger change that reaches a paused ScaledObject (for example under a lone `paused-replicas`) can read as nil after the pause ends. The HPA then shows `ScalingActive=False`, and the fleet holds. The fix is to delete the ScaledObject and reconcile `loadgen`, after the readiness check in Guardrail 5. Never restart the shared keda-operator.

## Halt Conditions

Stop and report to the user when:
- the next step needs a write and the user has not asked for it. Name the write and why it helps, then wait.
- someone else's pin or rollout is in progress on the chain or the cells. Do not change or stack on their pin. One exception: a runaway that meets the pin's own abort rule, or the re-pin rule after a release. Then tell the user who holds the current pin and what you would change, and wait for `confirm`. After the change, the user owns the pin; tell the previous holder.
- the release gate fails. Name each validator that holds it shut, and hand the decision to the testnet's owner.
- a validator has lost its data, or needs a rebuild. No paved procedure restores one validator on this chain from a snapshot or state sync. That is the owner's design decision.
- a fix needs a data wipe, a key change, a SeiNode deletion or a chain reset.
- the head has stopped and the chain may have lost quorum. Escalate; do not change load or nodes to recover consensus.
- your access fails (expired SSO, a missing context, Thanos unreachable). Say what failed; do not guess from stale data.

## Related

- `/harbor-dev`: an engineer's own Autobahn or giga chain on harbor.
- `/kubernetes`: sei-k8s-controller, SeiNode CRDs and reconcile code.
- `/platform`: platform manifests, Flux, Kustomize and cloud auth.
- `/root-cause`: a structured root-cause investigation after an incident.
- Runbooks repo: `platform/giga-validators/`, `platform/keda/`, `platform/sei-load/`, `giga-testnet-runbook.md`.
