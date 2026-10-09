# Diagnosis and safe operation of giga-testnet-0

This file tells a responder what to ask when the giga chain looks wrong, and what is safe to do
on shared prod cells. [architecture.md](architecture.md) explains the mechanisms. [load-and-scaling.md](load-and-scaling.md)
holds the scaler and the pin and release procedure. [observability.md](observability.md)
defines each bold query name, such as **busy-median**.

Contents: [1. Diagnosis](#1-diagnosis-the-order-of-questions) ·
[2. Failure modes](#2-failure-modes) · [3. Operating rules](#3-operating-rules-on-shared-prod-cells) ·
[4. Fresh start](#4-fresh-start-design) · [5. Change delivery](#5-how-changes-reach-the-testnet)

Evidence tags: **(code)** is the running code. **(manifest)** is the platform repo.
**(observed ...)** is live data under the named condition. **(not verified)** is reasoning only.

Terms: *lag* is the fleet head minus the next block a node has not executed (healthy: up to
about 3,500, from scrape skew). *Median busy* is the scaler's per-cell median of the 10
validators' execute-loop busy share. *Own busy* is one node's busy share. To *fence the load* is
to pin all four cells at 0 pods.

## 1. Diagnosis: the order of questions

Ask in this order. Each answer names a failure mode (FM) or sends you to the next question.

### Q0. Is someone already acting, and can you see the chain?

| Check | Reading | Decision |
|---|---|---|
| **pin-state**, or the ScaledObject's `PAUSED` column | 1 or True | A pin holds, and no hard stop acts. Find its owner; do not release it. |
| `flux --context <cell> get kustomizations -n flux-system` | Suspended or not Ready | Git does not land in that cell. Learn why first. |
| SeiNode condition `NodeUpdateInProgress`, all namespaces | True | A rollout runs. In another namespace it is another team's. Do nothing cell-wide. |
| A firing alert against the live reading (**lag-per-validator**) | They disagree | Trust the live reading. Check the alert's labels in **alert-history**: arctic-1 reuses pod names, so confirm `namespace="giga-testnet-0"`. |
| **scrape-census** | Fewer than 12 `monitoring/seid` targets in a cell | Part of the chain is invisible, to you and to the lag alerts. Fix visibility first. |

### Q1. Is the head moving?

Read **head-rate**.

- **It moves:** go to Q2.
- **It is flat:** the chain has halted. Every cell's scaler stops the load within about 3 minutes
  (manifest). Go to FM10.
- **No series:** Prometheus scrapes no validator, or the chain has no first block yet. No lag
  alert can fire.

### Q2. Are the validators and full nodes Ready and scraped?

Read **nodes-ready**, or `get seinode,pods` in the cell.

- **Fewer than 7 of 10 validators in a cell:** the cell's load stops by design. `Pending` points
  at capacity. A rising restart count points at a crash loop: read the `--previous` log.
- **One of two full nodes down:** the cap of 15 pods does not bite at 8 to 10 pods. The senders
  mapped to the missing node fail (FM3).
- **All Ready:** Ready only proves that port 8545 answers (code). Go to Q3.

### Q3. Is the execute loop saturated?

Read **busy-median** per cell, then **busy-per-validator**. The median hides slow validators.
**busy-edge** lists the validators with no margin:

```promql
sum by (cluster, pod) (rate(sei_chain_autobahn_main_loop_phase_duration_seconds_total{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", phase!="consensus"}[10m])) > 0.95
```

| Reading | Meaning |
|---|---|
| Median 0.90 or more | The load is too high. At 0.95 or more, a validator that falls behind does not recover (FM1). |
| Median in the band; some own busy 0.95 or more; their lag steady | Those validators have no margin. They are the next stragglers (FM2). |
| Same, but their lag grows | A straggler (FM2). Confirm with **exec-ms-per-validator**. |
| Same, but their lag falls | They are catching up. A node that is behind never waits, so it reads near 1. |
| Median below 0.70 at 30 pods | The load generators are the limit, not the chain. |
| Median near 1.0, under 40 blocks per second, throughput about 72,000 | Far past saturation: blocks grow large and slow. Only an uncapped or pinned overload gets here. Shed the load (FM1). |

All cells read about the same median. A cell that differs has a local fault or a scrape fault.

### Q4. Is one node behind, or many?

Read **lag-per-node** for all validators and full nodes.

| Pattern | Next |
|---|---|
| Many validators behind in two or more cells, median busy 0.90 or more | FM1 |
| A few validators behind, with high own busy | FM2 |
| Every validator of one cell behind | Q0, then the cell's NLBs |
| A full node behind | FM3 |
| A node near 2,160,000 behind | FM4 |

Then read **stage-heights** for each node that is behind:

```promql
max by (stage) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", pod="<pod>"}) - on() group_left() scalar(max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"}))
```

- **Execution-bound:** all stages advance, and `qc` and `receive` sit about 4,000 blocks above
  `execute`. Own busy reads near 1. The data layer takes a QC only within 4,000 blocks of `evict`,
  and `evict` follows `execute` (code). A node more than 4,000 behind thus never shows
  `receive` at the head (observed on lagging validators).
- **Not receiving:** all stages stand still while the head moves. Read the `seid` log for dial
  and fetch errors, and check the node's NLB. Escalate to `sei-network-specialist`.
- **Fetch-bound:** `qc` sits near `execute`, and both advance slower than the head (not verified
  as a live case).

### Q5. Is the scaler steering, pinned or in fallback?

Read **scaler-values**, **pin-state**, **hpa-not-active** and the ScaledObject's `FALLBACK`
column ([observability.md](observability.md#37-what-is-the-scaler-deciding)).

| State | Reading |
|---|---|
| Steering | **hpa-not-active** 0; `composite-metric` near `pods` |
| Shedding | `composite-metric` below `pods`: read which input caused it |
| Pinned | **pin-state** 1; no HPA; **scaler-values** absent (Q0) |
| Fallback or formula error | `FALLBACK` True, or **hpa-not-active** 1: FM5 |

### Q6. Is the RPC layer timing out?

Read **timeouts-by-owner**. `owner` is a validator public key.

```promql
topk(5, sum by (owner) (rate(tendermint_p2p_evm_proxy_requests{namespace="giga-testnet-0", cluster!="harbor", outcome="timeout"}[10m])))
```

One owner dominates: FM8. Every owner, from one full node (split by `pod`): FM3. Every owner, every cell: back to
Q1 and Q3. **slow-sends** shows the effect on the load in each cell.

### Q7. Is the load generator healthy?

Read **load-landed** per cell, **nonce-recovery** and **rejections-by-reason**. **goodput** gives
the network-wide rate.

- **load-landed about 1:** healthy.
- **load-landed low, nonce-recovery high:** senders wait on a silent node (FM9).
- **load-landed low, rejections high:** the node refuses. Read the reason in the `seiload` log.
- **Deployment at 0:** the sei-load rules skip this state, so no load alert fires. Back to Q5.

## 2. Failure modes

### FM1. Execute-loop saturation

- **Mechanism.** Each node runs one serial execute loop, and every validator executes every
  block. Busy rises with throughput (measured: about 0.11 + 0.89 x tx/s / 98,000). Near 97% median
  busy, a validator a few percent slower crosses 100% and falls behind. It recovers only when the
  median falls below about 90% (observed under saturation).
- **Symptom.** Median busy 0.95 or more everywhere; the slowest validators' lag grows;
  `GigaValidatorRunawayLag` fires. Send latency and acceptance stay normal (observed).
- **Check.** **busy-median** over an hour, **busy-per-validator**, **lag-per-validator**.
- **Safe response.** Above 0.90 the scaler sheds 15% per 3-minute step. If a validator runs away
  and the scaler does not shed within 10 minutes, fence the load. One cell's pin is not enough.

### FM2. Straggler validator

- **Mechanism.** Some validators run each block 30% to 40% slower than their peers on the same
  instance type, even while caught up. The cause is not known (observed in steady state). See
  [load-and-scaling.md](load-and-scaling.md#per-validator-speed-varies). The band keeps
  the median validator at 70% to 90%, not every validator. A slow one can sit at 95% to 100% own
  busy while the median reads about 76% (observed). It falls behind first when blocks get more
  expensive, and it does not catch up near saturation.
- **What degrades.** Its shard's forwards wait up to 10 s (FM8, FM9), and the quorum margin
  shrinks.
- **Symptom.** Own busy 0.95 or more with a steady lag means no margin. Under load a straggler
  swings up to about 9,000 behind and recovers. Past 12,000 and growing, the runaway alert fires.
  Inside the retention window, a straggler catches up once the median busy falls into the band.
  One recovered from more than 150,000 behind at about 1,000 blocks per minute (observed).
- **Check.** **busy-edge**, **exec-ms-per-validator** against peers, **lag-per-validator** and
  **stage-heights** (Q4). The `lag3` brake reads the third-worst
  validator per cell, so one or two stragglers do not move it.
- **Safe response.** Recommend a pin of all four cells when the validator passes 12,000 and
  **lag-trend** shows it still growing (the runaway alert's condition). Act only on the user's
  explicit request. If it runs away, fence the load. A validator N minutes of chain behind can
  lose ground for about N more minutes after the load falls (observed). Release only at the gate.
  Hand a CPU or disk ceiling to `k8s-capacity-management`; `spec.resources` and the volume size
  are create-only.
- **Runbooks.** [runaway lag](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/giga-validator-runaway-lag.md),
  [falling behind](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/giga-validator-falling-behind.md).

### FM3. A full node falls behind

- **Mechanism.** Two full nodes per cell execute every block and carry the cell's RPC work.
  Senders spread over them by address (manifest). A lagging full node still forwards sends and
  nonce reads to the shard owner (code), but it serves stale receipts and `latest` reads.
- **The brake.** `fnlag` reads the worst full node. From 12,000 the scaler caps the cell at 15
  pods, and at 16,000 the load stops. At 8 to 10 pods only the stop acts. A restarting full node
  drops out of `fnlag`, so its restart does not stop the load (manifest).
- **Symptom.** A `sei_role="node"` pod in **lag-per-node**. `GigaFullnodeFallingBehind`
  fires past 20,000 and growing. No runbook covers it.
- **Safe response.** Let the brake act. Fence the load before you restart a full node.

### FM4. The head passes the retention window

- **Mechanism.** Every node keeps about 4,321,000 blocks below its own head, and prunes older
  blocks every 5 minutes ([architecture.md](architecture.md#block-retention)).
- **Effect.** Past the window, no peer holds block 1. A node with a new volume then needs a
  snapshot or state sync, and no runbook covers that. A node whose lag passes the window cannot
  catch up.
- **Symptom.** **prune-watermark** rises above 0. `GigaValidatorNearBlockWindow` pages at
  2,160,000 behind.
- **Check.** **window-eta** gives the hours until the head passes the window. A negative value
  means it has passed.

- **Safe response.** Until then, block sync can rebuild a lost node. After it, no paved procedure
  restores one validator on this chain. The testnet's owner chooses a snapshot, state sync or a
  fresh start. Never delete a volume to fix lag.
- **Mock-app mode.** The window is 10,000 blocks. After a restart, every node asks for pruned
  block 1 and crash-loops with `pruned: below retention watermark`. Only a fresh start recovers.
  A mock-app run needs lag brakes below 10,000.
- **Runbook.** [near block window](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/giga-validator-near-block-window.md).

### FM5. Scaler faults

[load-and-scaling.md](load-and-scaling.md#how-the-scaler-fails) has the full table. Runbook: [keda-scaler-errors](https://github.com/sei-protocol/runbooks/blob/main/platform/keda/keda-scaler-errors.md).

| Fault | Symptom | Safe response |
|---|---|---|
| Queries keep failing | The cell sits at 0 pods with no chain fault; `FALLBACK` True; **scaler-errors** above 0; `KedaScalerErrors` after 15 m. One failed poll makes the object inactive, so the 30 s cooldown can reach 0 before the fallback engages. | Read the `Health` map and `keda_scaler_http_requests_total` by `status_code`. Fix the query or the cell's Prometheus. Zero is the safe state. |
| Formula reads nil (a trigger change reached a paused ScaledObject) | **hpa-not-active** 1; the count holds; no alert; no hard stop | **log-keda-formula** shows `invalid operation: <nil> >= int`. Recreate the ScaledObject as the [paused-trigger caveat](load-and-scaling.md#the-paused-trigger-caveat-keda-2202) describes. Never restart the shared keda-operator. |
| Metrics server down | `KedaMetricsAPIServiceUnavailable`; the HPA holds its count | Fence the load if the chain is unhealthy. Garbage collection stalls cell-wide, so do not start a fresh start. |

### FM6. Flux reverts a hand-set pause

- **Mechanism.** In all four cells, `flux-system` applies the `loadgen` Kustomization from
  `clusters/<cell>/flux-system/loadgen.yaml`, which sets no `suspend`. A `loadgen` suspend thus
  clears within 3 minutes. Then `loadgen` re-applies the ScaledObject and removes a bare
  `autoscaling.keda.sh/paused-replicas` (observed).
- **Symptom and response.** The load returns a few minutes after a pause. Use the
  two-annotation [pin](load-and-scaling.md#7-pin-and-release).

### FM7. Slow storage after a fresh start

- **Mechanism.** After a fresh start, the hash-vault fsync runs about 3 times slower than its
  steady 1 ms per block, for tens of minutes. Then it recovers at the same load (observed after
  fresh starts). The cause is not established; background initialization of the new ext4
  volumes is a candidate (not verified).
- **Symptom.** Busy above the calibration for the load.
- **Check.** The vault commit time per block in milliseconds (**storage-tail** shows its share):

  ```promql
  quantile(0.5, 1000 * sum by (cluster, pod) (rate(sei_chain_autobahn_storage_tail_phase_duration_seconds_total{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", phase="vault_commit"}[5m])) / sum by (cluster, pod) (rate(tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", stage="execute"}[5m])))
  ```

- **Safe response.** Hold the load at zero until it settles. Do not measure throughput before.

### FM8. Forward timeouts concentrate on one owner

- **Mechanism.** Shards spread senders over all 40 validators by address (code), so every cell
  sends to every owner. A slow owner holds its 512 forward connections, and forwards to it time
  out at 10 s (code).
- **Symptom.** One owner dominates **timeouts-by-owner**. **slow-sends** rises in every cell. The scaler's `lat`
  includes them, and above 1.2 s it sheds load although the chain has headroom.
- **Check.** Find the owner key in the committee in `autobahn.json` (genesis bucket key
  `giga-testnet-0/autobahn.json`). Each entry pairs a `validator_key` with its
  `validator-NN-p2p` address. Then read that validator's lag and own busy.
- **Safe response.** Treat the owner as a straggler (FM2). Do not restart the full nodes.

### FM9. Stuck nonce recovery in the load generator

- **Mechanism.** After a failed send, sei-load re-reads the account's pending nonce until the
  node answers. The backoff runs from 250 ms to 5 s (code). The full node forwards that read to the
  shard owner, so an account on a silent owner waits with no error of its own.
- **Symptom.** **nonce-recovery** high; tens to hundreds per pod is normal while a
  validator lags. `SeiLoadGeneratorNonceRecoveryStalled` fires at over 50 waiting accounts with under
  1 tx/s landing for 10 minutes.
- **Safe response.** Fix the silent node or owner. A generator restart only starts new accounts
  against the same node.
- **Runbook.** [nonce recovery stalled](https://github.com/sei-protocol/runbooks/blob/main/platform/sei-load/seiload-generator-nonce-recovery-stalled.md).

### FM10. Chain halt from lost quorum

- **Mechanism.** A block is final when 27 of 40 validators agree. With 10 per cell, the chain runs
  without one cell and stops without two. Each SeiNode replaces its own pod with no fleet pacing
  (code), so a rollout of every validator halts the chain until 27 are back.
- **Check.** Ready validators per cell, `NodeUpdateInProgress`, pod events.
- **Safe response.** After a rollout, wait: each validator resumes from its own state. If 27 or
  more run and the head stays flat, escalate a consensus fault to `sei-network-specialist` and the
  testnet's owner. Never delete SeiNodes or volumes to restart the chain.

## 3. Operating rules on shared prod cells

prod-use2, prod and prod-euw1 also run arctic-1, atlantic-2 and pacific-1 nodes. Every chain in a
cell shares one controller, one KEDA operator, one Prometheus and the NodePools. `flux-system`
applies the giga nodes, through its child `apps` in prod-apne1.

1. **Read-only by default:** `kubectl get`, `describe` and `logs`, `flux get`, and queries.
2. **Each write needs an explicit request.** Writes are: a pin or release, any delete, any
   restart, any reconcile, annotate or patch, and any merge to platform main.
3. **Never suspend `flux-system`, or `apps` in prod-apne1.** It freezes other teams' rollouts and
   the other chains. A `loadgen` suspend does not hold (FM6).
4. **Never delete a validator's SeiNode or volume. Never change its keys.**
   - A SeiNode delete also deletes the PVC and the EBS volume (code).
   - The volume holds the node's consensus votes, so the node does not vote twice. A new volume
     with the same key loses that record (code).
   - Peers check each validator's keys against the committee (code).
   - `prune: disabled` only stops Flux from deleting a validator whose manifest leaves Git. It does
     not stop a manual delete. Full nodes have neither annotation, so removing a full node's
     manifest deletes the node and its volume.
   - A pod delete keeps the PVC, but it is still a restart (rule 2).
5. **Release only at the [release gate](load-and-scaling.md#the-release-gate).** That section
   states the gate once: the count, the **lag-trend** rule and the scaler check.
6. **Pin and release all four cells together.** Every validator executes every block.
7. **Before anything cell-wide, check other teams' rollouts:** Flux Kustomizations and
   HelmReleases, SeiNodes updating in any namespace, and recent merges to `clusters/<cell>/`.
   Cell-wide means a forced reconcile of `flux-system` or `apps`, a shared-controller restart or a
   NodePool change.
8. **The pin owner owns the load.** No hard stop acts while a pin holds.
9. **Verify after an interrupted command.** A loop over four cells can stop part way. Read every
   cell before you act again.
10. **A reset is the testnet owner's decision.** The testnet's owner is the platform team that runs
    giga-testnet-0.

## 4. Fresh start design

A fresh start restarts the chain from genesis with new volumes on every node. It is a one-way
door: it erases all history and every consensus record. The testnet's owner approves it, and no
runbook covers it yet.

What stays:

- the SeiNode manifests in git;
- the key Secrets, which the controller never deletes;
- the `validator-NN-p2p` NLB Services, which have no owner and no Flux prune;
- the genesis artifacts in each cell's bucket `<cell>-sei-k8s-genesis-artifacts`, keys
  `giga-testnet-0/genesis.json` and `giga-testnet-0/autobahn.json`.

What goes: the SeiNodes, StatefulSets, pods and every data volume.

| Phase | Action | Gate |
|---|---|---|
| A. Preconditions | Check all four cells. | No other rollout. All Kustomizations Ready. `KedaMetricsAPIServiceUnavailable` quiet. Git holds the target spec for 48 SeiNodes. Genesis artifacts match in all four buckets. |
| B. Fence | Pin all four cells at 0. | Every Deployment at `0/0`; every pin holds after one Flux interval. |
| C. Remove | Delete every SeiNode in `giga-testnet-0` in all four cells. The controller deletes each PVC, and garbage collection removes the StatefulSet and pod (code). | No SeiNode, no PVC, and no `validator-` or `fullnode-` pod in any cell. |
| D. Recreate | Flux applies 48 SeiNodes. Each creates a volume, fetches genesis from its cell's bucket (retrying for up to about 30 minutes) and starts `seid` (code). | 48 nodes Running on the target image; the head moves; 40 validators report; every lag healthy. |
| E. Release | Wait out FM7, then release at the gate. | Valid scaler metrics (FM5); a ramp to about 8 to 10 pods per cell. |

Why the order matters:

- **Fence first.** A cell that runs the new chain before the others reads `commit > 0` and
  `ready >= 7`, and its scaler starts load. The pin holds zero across the whole sequence.
- **Remove all four cells before any node starts.** Every generation has the same chain ID, keys
  and addresses. The giga handshake checks only keys against the committee, with no chain ID or
  genesis check (code: `sei-tendermint/internal/p2p/giga_router_common.go`). A new node would peer
  with a surviving old node, and two generations would exchange votes signed by one key set.
  Whether old blocks fail verification on the new chain is not verified. Treat any surviving old
  node as a safety hazard.
- **Wait until no old volume remains.** A recreated SeiNode has a new UID. Its `ensure-data-pvc`
  task fails terminally on a PVC it does not own (code). A surviving volume also keeps the old
  genesis marker, and the node resumes the old chain (code).
- **Avoid the Flux race without a suspend.** Flux re-applies the SeiNodes every 3 minutes. Two
  shapes work:
  - Force a reconcile just before the delete, and clear gate C inside one interval. This is a
    cell-wide act, so gate A must hold.
  - Remove the SeiNodes from git, delete them, clear gate C, then restore them in git. This takes
    two merges. Flux deletes the full nodes itself on removal.
- **Start together.** No block commits until 27 validators run, and late cells start behind.
- **Hold the load.** New volumes fsync slowly (FM7), and a node behind under load may not recover.

## 5. How changes reach the testnet

Each giga SeiNode has its own manifest under `clusters/<cell>/giga-testnet-0/` with its own
`image` and `configValues`, so a fleet change touches 48 files. Flux fetches main every 1 to 3
minutes and applies the SeiNodes every 3 minutes. `loadgen` applies the load objects separately;
a profile edit rolls the load pods through the ConfigMap's content hash.

| Change | What the controller does (code) |
|---|---|
| `image` | Applies the StatefulSet and the config, deletes the pod at the old revision, marks the node ready. The pod returns on the same PVC. The StatefulSet is `OnDelete`, so only the controller restarts pods. |
| `configValues` | Patches and validates the config, then restarts `seid` through the sidecar. |
| Create-only fields (engine, execution mode, `spec.resources`, volume size and class, key Secret names) | Admission refuses it. It needs a new node and a new volume. |

Each SeiNode plans on its own. A 48-file change thus restarts a cell's nodes together, and the
four cells follow within minutes.

| Restart | Effect on the chain | Effect on the load |
|---|---|---|
| All validators at once | A short halt. Each validator resumes from its own state, so none returns far behind. On a testnet this is acceptable, and often better than a slow roll. | `commit` reads 0, so every cell stops. |
| One cell at a time | The other 30 keep making blocks. Each restarted validator returns about 6,000 blocks behind per minute of downtime. | Other cells keep sending, and senders on the restarting owners time out (FM8). |
| Full nodes | No effect on consensus. | Nothing stops the load (FM3). |

Fence the load for the restart window, then release at the gate.

An image that changes execution results changes the app hash, so it cannot run beside the old one.
A node whose app hash differs from the quorum's stops with an AppHash divergence error (code). Roll
it to all nodes at once. Autobahn supports no freeze height, so a fresh start is the clean path
(not verified live).

### One-way doors

These need the testnet owner's approval before they land. A git revert does not undo them.

| Change | Why |
|---|---|
| Genesis artifacts | A new chain: every node starts from them together (section 4). |
| Validator keys | Peers reject a key the committee does not name. A reused key on a new volume loses its vote record. |
| A data wipe | It erases history and votes. Past the window, the node cannot rebuild from the network. |
| A smaller `giga.storage.lookback_window` | Every node prunes within 5 minutes. A larger value later does not restore blocks. |
| `mock-app` on or off | A new application and app hash: a fresh start of all nodes. |
| Create-only fields | A new node and volume. |
| An image that changes on-disk formats | A rollback needs a wipe (not verified for any image). |
| `giga.storage.receipts` to false | A node with receipt history on disk then refuses to start (code). |

Two-way doors: a pin and its release; an image that keeps formats and results; a `configValues`
change that does not touch retention or storage.
