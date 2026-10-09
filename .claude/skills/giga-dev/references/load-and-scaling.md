# Load generation and autoscaling

This file covers the giga-testnet-0 load fleet. It explains the sei-load pods, their closed loop,
the KEDA ScaledObject and its formula, and why the scaler holds headroom. It also gives the pin and
release procedure. The transaction path and the execute loop are in
[architecture.md](architecture.md). Queries, dashboards and alerts are in
[observability.md](observability.md). Failure modes are in [diagnosis.md](diagnosis.md).

Contents:

1. [The load fleet](#1-the-load-fleet)
2. [sei-load's closed loop](#2-sei-loads-closed-loop)
3. [What the closed loop implies](#3-what-the-closed-loop-implies)
4. [The ScaledObject](#4-the-scaledobject)
5. [Why the scaler holds headroom](#5-why-the-scaler-holds-headroom)
6. [How the cells interact](#6-how-the-cells-interact)
7. [Pin and release](#7-pin-and-release)

## 1. The load fleet

Each prod cell runs one Deployment, `seiload-saturation`, in namespace `giga-testnet-0`. Its pods
send EVM transfers to the cell's two full nodes. A KEDA ScaledObject of the same name sets the pod
count from 0 to 30.

| Object | Source in `sei-protocol/platform` |
|---|---|
| Deployment `seiload-saturation` | `manifests/base/giga-testnet-0/loadgen/deployment.yaml` |
| ScaledObject `seiload-saturation` | `manifests/base/giga-testnet-0/loadgen/scaledobject.yaml` |
| Full-node brake (a Kustomize component on the ScaledObject) | `manifests/base/giga-testnet-0/loadgen/fullnode-brake/kustomization.yaml` |
| HPA `keda-hpa-seiload-saturation` | KEDA creates it |
| Profile, mounted as `profile.json` | `clusters/<cell>/giga-testnet-0/loadgen/profile-fullnodes.json` |
| Flux Kustomization `flux-system/loadgen` | `clusters/<cell>/flux-system/loadgen.yaml` |

The `loadgen` Kustomization applies the fleet apart from the chain, so a KEDA fault cannot hold
back the validators. Its interval is 3 minutes. It depends on `flux-system`, or on `apps` in
prod-apne1. In all four cells, its parent is `flux-system`. The Deployment has no `replicas` field.

The four profiles differ only in their endpoints, the cell's two full nodes. Each pod has 40,000
accounts and sends `EVMTransferNoop`, a zero-value transfer to self. The profile has no `seed`,
`funding` or `shardRouting` block, so each pod draws new keys at start.

| Pod flag | Value | Effect |
|---|---|---|
| `--max-in-flight` | `1500` | Bounds the pod's send queue and sizes its HTTP idle pool per endpoint |
| `--tps` | `2000` | A rate cap per pod. The offered load of a cell is its pod count times 2,000 tx/s |

Without the rate cap, one pod moved a cell's load by about 25%. That step is too coarse to hold a
band. With the cap, each scaling step moves the load in proportion to the pod count.

## 2. sei-load's closed loop

The arrival model is `closed_loop`. A new transaction enters only when an old one leaves, so the
generator slows down when the chain slows down.

1. The generator takes a sender and then a receiver from one round-robin pool. The pool size is
   even, so senders always come from the same 20,000 accounts. The other 20,000 never send.
2. It builds the transaction with the account's next nonce and pushes it into the send queue. The
   push blocks while the queue holds 1,500 transactions.
3. The send loop takes one token from the rate limiter (2,000 per second, burst 2,000).
4. A worker signs the transaction and calls `eth_sendRawTransaction` on the full node that the
   sender's address selects (address modulo 2).
5. On success, the queue frees the slot and readies the account's next transaction.

| Bound | Effect |
|---|---|
| One transaction in flight per sender | The queue readies an account only after its last send returns, so nonces never race |
| At most 1,500 transactions per pod | A transaction keeps its slot through the token wait, the RPC, retry waits and nonce re-reads |
| At most 2,000 send attempts per second per pod | Every attempt spends a token, retries included. Nonce re-reads spend none |

### Failure handling

| Send result | What sei-load does |
|---|---|
| `mempool is full` | Offers the same signed bytes again, up to 4 attempts in all. It draws each wait from `[bound/2, bound)`, with bounds of 1 s, 2 s and 4 s |
| Any other error, or a fourth `mempool is full` | Re-reads the nonce with a pending `eth_getTransactionCount`, with backoff from 250 ms to 5 s, until the node answers. Then it resets the account to that nonce |

A full mempool clears with the next block, so a re-offer is safe. A nonce re-read then would rewind
past transactions that the lane holds but the chain has not executed. The re-read spends no token:
it is recovery traffic, and a node outage must not turn the send budget into nonce reads.

The full node forwards the send and the nonce read to the shard owner with a 10 s deadline. A
lagging or full owner makes the forward wait the full 10 s and fail with `reason="rpc"`.

### What the generator's metrics mean

| Metric | Meaning |
|---|---|
| `seiload_send_latency_seconds` | The RPC round trip of every attempt, with label `status` `success` or `failure`. It excludes queue time. A forward timeout records about 10 s |
| `seiload_txs_accepted_total` | The owner admitted the transaction into its lane. It does not prove execution. The series is absent until the first success |
| `seiload_txs_rejected_total` | Sends with an error, label `reason` `rpc` or `mempool_full` |
| `seiload_nonce_recovery_inflight` | Accounts in a nonce re-read now. Near 0 while every owner answers; tens to hundreds per pod while an owner lags |


## 3. What the closed loop implies

### Little's law per pod

Let `R` be the mean RPC round trip that `seiload_send_latency_seconds` measures.

| Regime | Condition | Pod send rate |
|---|---|---|
| Rate-capped | `R` below 0.75 s (1,500 / 2,000) | 2,000 tx/s. About `2,000 x R` sends are on the wire, and the rest wait for a token |
| In-flight bound | `R` above 0.75 s | `1,500 / R`, under 2,000 tx/s |

Below the scaler's 0.6 s growth limit, every pod is rate-capped, so one more pod adds 2,000 tx/s.
Above its 1.2 s shrink limit, each pod sends at most 1,250 tx/s. Chain-wide, mean latency equals
sends in flight divided by throughput. Past saturation, more pods add latency, not throughput: in a
measured staircase, throughput stayed flat from 6 to 38 uncapped pods per cell.

### Latency and acceptance do not see execute-loop saturation

A send returns when the owner admits it into its lane, before execution. The execute loop can
therefore run near 100% busy while sends still return fast. Measured: at about 98% median busy,
mean latency was about 0.26 s and acceptance about 99%. Only busy sees the saturation.

### A lagging owner uses up the in-flight budget

Each validator owns about 1/40 of the senders. If one owner lags, about 50 of a pod's 2,000 sends
per second wait 10 s each. Their nonce re-reads go to the same owner. That holds 500 or more of the
1,500 slots (derived). The pod's accepted rate falls, and the 10 s samples add about 0.25 s to the
mean latency. That alone can block growth or force a shrink.

This needs no saturated chain. Before you read a low pod count as saturation, check busy and the
forward timeouts per owner.

## 4. The ScaledObject

KEDA 2.20.2 runs in namespace `keda` in each cell. Each ScaledObject reads only its own cell's
Prometheus (`http://prometheus-operated.monitoring.svc:9090`). The cells do not coordinate.

| Field | Value | Effect |
|---|---|---|
| `minReplicaCount`, `maxReplicaCount` | `0`, `30` | A hard stop takes the cell to 0 |
| `pollingInterval` | `15` | KEDA reads the triggers every 15 s |
| `cooldownPeriod` | `30` | KEDA scales to 0 when 30 s pass with no active poll |
| `fallback` | `failureThreshold: 3`, `replicas: 0` | A query that keeps failing holds the cell at 0 |
| `scalingModifiers` | `target: "1"`, `metricType: AverageValue`, `activationTarget: "0"` | The formula's value is the replica count. The object is active while it reads above 0 |
| HPA stabilization | `scaleUp` and `scaleDown` 180 s | At most one step per 3 minutes |
| Every trigger | `type: prometheus`, `ignoreNullValues: "false"` | NaN or an empty result is an error. Each query ends in `or vector(<value>)`, so an absent series gives a fixed value |

KEDA gives the HPA `minReplicas: 1` and the metric `composite-metric`, with an average target of 1.
`kubectl get hpa` therefore shows `1/1 (avg)` while the formula holds.

### Triggers

The base file has seven triggers, and `fullnode-brake` adds two. Every cell uses the component.
KEDA names them `s0-prometheus` to `s8-prometheus` in the `Health` map. The `scaler` label of
`keda_scaler_metrics_value` carries the trigger name.

| Trigger | Reads | If absent | Role |
|---|---|---|---|
| `ready` | The cell's validators that report an execute height and are Ready | 0 | Stop below 7 |
| `commit` | Blocks the chain head advanced over 2m | 0 | Stop at 0 or below |
| `lag3` | The third-worst validator lag in the cell | 1e9 | Cap 15 from 12,000; stop at 16,000 |
| `busy` | The median over the cell's validators of the execute-loop busy share over 2m | 1 | Band 0.70 to 0.90 |
| `lat` | Mean send latency of the cell's pods over 2m, all sends | 99 | Grow only under 0.6 s; shrink above 1.2 s |
| `accept` | `(accepted + 1) / (accepted + rejected + 1)` over 2m | 1 | Grow only at 0.90 or more; shrink below 0.80 |
| `pods` | `kube_deployment_spec_replicas` of the Deployment | 0 | The base of every step |
| `fnready` | The cell's full nodes that report an execute height and are Ready | 0 | Cap 15 below 2; stop at 0 |
| `fnlag` | The worst full-node lag behind the validator head | 1e9 | Cap 15 from 12,000; stop at 16,000 |

Design notes:

- An absent health series reads as a stop, never as healthy (the "If absent" column).
- `busy` sums `sei_chain_autobahn_main_loop_phase_duration_seconds_total{phase!="consensus"}` per
  pod. Every validator executes the same blocks, so the median reads the chain, and a lagging
  validator does not move it. Absent `busy` shrinks the cell but does not stop it.
- `lag3` takes the third-worst validator, so one or two wedged validators cannot hold the load at
  0. The cost: the brake ignores the two worst validators in each cell.
- `commit` reads negative after a chain reset, so load waits until the new chain has run 2 minutes.
- A full node's Ready proves only that port 8545 answers, so `fnlag` stops load into a wedged or
  syncing full node.
- A restarting node drops out of `lag3` and `fnlag`. Pin the load before a rollout restarts nodes.

### The formula

`fullnode-brake` repeats the base formula with the full-node terms. Change both together.

```
ready >= 7 && commit > 0 && lag3 < 16000
  ? (let L = lag3 >= 12000 ? 15 : 30;
     let N = pods < 1 ? 3
           : (accept < 0.80 || lat > 1.2 || busy > 0.90 ? floor(pods * 0.85)
           : (accept >= 0.90 && lat < 0.6 && busy < 0.70
                ? (ceil(pods * 1.2) >= 27 ? 30 : ceil(pods * 1.2))
                : pods));
     pods > L ? max(1, floor(pods * 0.75)) : (N > L ? pods : max(1, N)))
  : 0
```

The component adds `fnready >= 1 && fnlag < 16000` to the stop condition. Its cap is
`L = (lag3 >= 12000 || fnready < 2 || fnlag >= 12000) ? 15 : 30`.

| Order | Condition | Result |
|---|---|---|
| 1 | Any hard stop | 0 |
| 2 | `lag3 >= 12000`, `fnready < 2` or `fnlag >= 12000` | Cap `L = 15`, else 30 |
| 3 | `pods > L` | Shrink by a quarter: `max(1, floor(pods x 0.75))` |
| 4 | `pods < 1` | 3 |
| 5 | `accept < 0.80` or `lat > 1.2` or `busy > 0.90` | Shrink by 15%: `floor(pods x 0.85)` |
| 6 | `accept >= 0.90` and `lat < 0.6` and `busy < 0.70` | Grow by a fifth: `ceil(pods x 1.2)`, or 30 from 27 |
| 7 | Otherwise | Hold |
| 8 | A result above `L` | Hold instead |
| 9 | Any result of 4 to 7 | At least 1 |

One guard out of range shrinks the cell. Growth needs all three in range.

### Why the steps have these sizes

- The HPA ignores a change within 10% of the current count. A grow is at least x1.15 and a shrink
  at most x0.85, so the HPA acts on every step.
- A grow that reaches 27 or more goes to 30. The HPA would never move from 28 or 29 to 30, and a
  grow past 30 holds. Without the jump, a cell stalls at 26.
- The largest step, 22 to 30 pods, moves busy about 15 points. The band is 20 points wide, so one
  step cannot cross it.

| Path | Pod counts |
|---|---|
| Grow from 1 | 1, 2, 3, 4, 5, 6, 8, 10, 12, 15, 18, 22, 30 |
| Shrink from 30 | 30, 25, 21, 17, 14, 11, 9, 7, 5, 4, 3, 2, 1 |
| Above the cap of 15 | 30, 22, 16, 12 |

### Start, floor and timing

- At 0 pods the formula reads 3, so the object goes active. KEDA's activation scales the
  Deployment to 1. The 3 applies only if the HPA's first read still sees 0 pods. Otherwise the
  cell climbs from 1. From 0 to about 10 pods takes roughly 15 to 25 minutes (derived).
- The floor of 1 lets the guards shrink a cell to 1 pod but not to 0. Only a hard stop reaches 0.
  The last pod keeps `lat` and `accept` reading the RPC path.
- The HPA scales up to the lowest answer of the last 180 s and down to the highest. A step
  therefore needs 180 s of agreeing answers, which covers a pod start and the 2m rate windows.
- A stop does not wait for the HPA. The object goes inactive, and KEDA scales to 0 after the 30 s
  cooldown. `commit` reads 0 only after 2 minutes with no new block, so a stalled head stops the
  load in about 3 minutes.

### Why the brakes sit at 12,000 and 16,000

A healthy validator reads up to about 3,500 blocks of lag, from scrape skew. Under saturation, a
slow validator can swing to about 9,000 behind and recover, so the cap sits above that. The stop
is a conservative margin, not a recovery limit. With real execution, a validator inside the
retention window catches up once the chain gives it headroom. One recovered from more than
150,000 behind at about 1,000 blocks per minute (observed). The values assume real execution. A
mock-app run keeps only 10,000 blocks and needs lower brakes.

### How the scaler fails

| Fault | Effect on the load | Signal |
|---|---|---|
| One failed poll between good ones | That poll is inactive. A good poll within 30 s keeps the load running | `keda_scaler_detail_errors_total` rises |
| Queries keep failing | The 30 s cooldown scales the cell to 0. After more than 3 failures in a row, the fallback also gives the HPA 0 | `Fallback=True`; `Health` shows `Failing` |
| The formula cannot run | The HPA holds its count and takes no step. Do not count on any hard stop | HPA `ScalingActive=False`; `KedaScalerErrors` stays quiet |
| Metrics server down | The HPA holds its count. KEDA still scales to 0 on a stop | `KedaMetricsAPIServiceUnavailable` |

A healthy object shows `Ready=True`, `Active=True`, `Fallback=False`, `Paused=False`, every
`Health` entry `Happy`, and HPA `ScalingActive=True ValidMetricFound`.

## 5. Why the scaler holds headroom

### The saturation trap

1. Every validator executes every block on one serial loop. Busy is the share of wall time in the
   `execution` and `storage` phases, as against `consensus`, the wait for the next block.
2. A validator falls behind once its own busy reaches 100%. Near 97% median busy, every validator
   slower than the median gets there.
3. A validator that is behind pays more execution time per block than its peers for the same blocks.
   The excess is tens of percent (measured; the cause is not known). It falls further behind and
   does not recover by itself. The chain does not wait, because the app-hash quorum is 27 of 40.
4. It recovers only when median busy falls below about 90%. A validator far behind then catches
   up at roughly 1,000 blocks a minute (measured).

Smaller, more frequent blocks can spring the trap at the same load. Latency and acceptance do not
warn of it (section 3). Busy does.

### Per-validator speed varies

Some validators run 30 to 40% slower per block than their peers on the same instance type, even
while caught up. The scaler steers on each cell's median busy, so it does not see them. A slow
validator can sit at 95 to 100% of its own busy while the median reads about 76%. A cell can hold
up to four such validators with no change in its median. The band keeps the median validator at 70
to 90% busy. It does not by itself give every validator headroom.

Check each validator's own busy, not only the median:

```promql
sort_desc(sum by (cluster, pod) (rate(sei_chain_autobahn_main_loop_phase_duration_seconds_total{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", phase!="consensus"}[10m])))
```

Own busy at 95% or more means the node has no margin. It is the next straggler.

### Calibration and the band

With real execution, median busy is close to a straight line in chain throughput:

```
busy ≈ 0.11 + 0.89 x throughput / 98,000 tx/s
```

| Pods per cell (uncapped) | Throughput (measured) | Median busy |
|---|---|---|
| 4 | about 43,000 tx/s | about 51% |
| 5 | about 74,000 tx/s | about 79% |
| 6 | about 99,000 tx/s | about 98% |

The line comes from one calibration. If the chain gets slower per transaction, the band still holds
busy at 70 to 90%, at a lower throughput. Through the line, the band is about 65,000 to 87,000
tx/s, or 8 to 11 pods per cell. Real execution saturates near 100,000 tx/s (measured).

The measured steady state is 8 to 10 pods per cell and about 70,000 tx/s, with each pod at its
cap. Median busy is about 76% in every cell, mean latency about 0.1 to 0.25 s, and acceptance 99%
or more. A model of the four cells on one chain agreed from any start.

## 6. How the cells interact

Every cell's `busy` reads the same chain, and every cell's `lat` and `accept` include forwards to
owners in all four cells. Load from any cell moves every cell's signals.

When one cell stops:

1. Chain throughput falls by that cell's share, and busy falls in every cell.
2. The other cells grow by a fifth per window, each on its own clock, until the chain is back in
   the band. Each now carries more pods.
3. When the stopped cell returns, it starts from 1 or 3. It grows only while busy is below 70%, so
   it often holds at a low count.

Cells therefore settle unevenly, as in a modeled split of 1, 6, 12 and 18 pods, while the chain
total stays in the band. One lagging owner raises `lat` in all four cells, so all four can shed
together. A cell held at 30 (`KubeHpaMaxedOut`) most often means the other cells send little. If
you pin one cell to 0, the other three grow into its share: pin all four.

## 7. Pin and release

### Rules

- A pin is a write on a shared prod cell. Echo the cells, the object and the change, and wait for
  `confirm`.
- Pin and release all four cells together. Every validator executes every cell's blocks.
- The pin is two annotations on the ScaledObject, set together.
  `autoscaling.keda.sh/paused-replicas=<n>` makes KEDA hold the Deployment at `<n>`.
  `kustomize.toolkit.fluxcd.io/reconcile=disabled` makes Flux leave the object alone.
- Never put either annotation in Git. In Git, the pin has no owner. It turns off the hard stops,
  and it stops Flux from updating the object.
- While the pin holds, no hard stop and no cap acts. The operator who set the pin owns the load.

### Why other methods do not hold

| Method | Result |
|---|---|
| `paused-replicas` alone | `loadgen` re-applies the ScaledObject within one interval and removes the annotation |
| Suspend `loadgen` | The parent `flux-system` re-applies the `loadgen` object from Git and clears the suspend within one interval |
| Suspend `flux-system` | It freezes every workload in the cell, other teams' rollouts and arctic-1 included. Never use it |
| Scale the Deployment by hand | KEDA or the HPA overwrites it on its next poll |
| Delete the ScaledObject | Nothing stops the load on a chain fault until Flux recreates it |

The two-annotation pin survives forced reconciles of both `flux-system` and `loadgen`.

### Pin and check

Before you pin, read **pin-state**. If a pin already holds, find who set it before you change it.
`kubectl --context <cell> -n giga-testnet-0 get scaledobject seiload-saturation -o yaml
--show-managed-fields` shows when the annotations changed, not who changed them, so ask in the
testnet's thread. Change another person's pin only when its own abort rule or the re-pin rule fires
and the user confirms. The person who changes a pin owns the load from then on.

```
for cell in prod prod-use2 prod-euw1 prod-apne1; do
  kubectl --context "$cell" -n giga-testnet-0 annotate scaledobject seiload-saturation \
    kustomize.toolkit.fluxcd.io/reconcile=disabled \
    autoscaling.keda.sh/paused-replicas=0 --overwrite
done

for cell in prod prod-use2 prod-euw1 prod-apne1; do
  kubectl --context "$cell" -n giga-testnet-0 get scaledobject seiload-saturation \
    -o jsonpath='paused={.metadata.annotations.autoscaling\.keda\.sh/paused-replicas} reconcile={.metadata.annotations.kustomize\.toolkit\.fluxcd\.io/reconcile} Paused={.status.conditions[?(@.type=="Paused")].status}{"\n"}'
  kubectl --context "$cell" -n giga-testnet-0 get deployment seiload-saturation
done
```

Use `0` to stop the load, or another count to hold it. Each cell must show
`paused=<n> reconcile=disabled Paused=True` and the Deployment at `<n>/<n>`. Run the check again
after 5 minutes. The annotations must still be there.

While paused, KEDA stops the scale loop, deletes the HPA and holds the Deployment at the paused
count. Check the pin with the `Paused` condition, because the HPA does not exist then. The sei-load
alerts skip giga-testnet-0 while the Deployment wants 0 pods.

### The release gate

Release only when every validator has caught up. The `lag3` brake ignores the two worst validators
in each cell, so it cannot be the gate. This query counts the validators that report a lag of
12,000 blocks or less:

```promql
count((max(tendermint_internal_autobahn_avail_commit_global_block_number{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator"}) - on() group_right() max by (pod) (tendermint_internal_autobahn_data_next_block{namespace="giga-testnet-0", cluster!="harbor", sei_role="validator", stage="execute"})) <= 12000)
```

It must return exactly 40. Any other result blocks the release. Fewer than 40 means a validator
lags, is down or is not scraped. An empty result means the head series is missing, so the gate has
no data. Every lag must also still fall. Read **lag-trend** in
[observability.md](observability.md#32-is-each-node-keeping-up). No validator may grow by more than
about 1,000 blocks in 10 minutes. Any validator more than 6,000 behind must shrink. If you
pinned for a straggler, check its own busy too. While it catches up, it reads about 1.0. Once it
reaches the head, its own busy must drop below 0.95. If it stays at 0.95 or more while caught up,
it has no margin (section 5).

### A release with a named exception

Only the testnet's owner, the platform team that runs giga-testnet-0, can approve a release while a
validator fails the gate. Record who approved it, which validator, and when. The gate query must
then return 40 minus the number of excepted validators, and only the named validators may be
missing from it. Then release, and watch the others with **lag-trend**. Pin again if any of them crosses 12,000 and still grows. The
excepted validator stays the owner's to track.

### Release and confirm

```
for cell in prod prod-use2 prod-euw1 prod-apne1; do
  kubectl --context "$cell" -n giga-testnet-0 annotate scaledobject seiload-saturation \
    autoscaling.keda.sh/paused-replicas- kustomize.toolkit.fluxcd.io/reconcile-
  flux --context "$cell" reconcile kustomization loadgen -n flux-system
done
```

The reconcile makes Flux apply Git at once, including any ScaledObject change that merged during
the pin. In prod-apne1, the reconcile can fail when `apps` is not Ready. The release still holds,
and Flux applies Git at its next interval.

After about 1 minute, check each cell:

```
kubectl --context <cell> -n giga-testnet-0 get scaledobject seiload-saturation
kubectl --context <cell> -n giga-testnet-0 get hpa keda-hpa-seiload-saturation \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}'
```

Expect no pin annotations, `PAUSED False`, `FALLBACK False` and HPA
`ScalingActive=True ValidMetricFound`. The Deployment grows by one step every 3 minutes until busy
reaches the band. Watch the lags for 30 minutes, and pin again if a validator runs away.

### A time-boxed saturation run

A run above the band, to measure peak throughput, strands the slower validators by design. It
needs the testnet owner's approval, a time box and an abort rule before it starts.

- **Pin count.** Offered load is pods per cell x 4 x 2,000 tx/s. About 13 pods per cell offers
  about 104,000 tx/s, just past saturation.
- **Time box.** Keep it short, for example 30 minutes, and name the end time before you start.
- **Abort.** Pin to 0 when any validator crosses 12,000 and **lag-trend** shows it still growing.
- **End.** Pin to 0, wait for the release gate, then release. Never end a run by a plain release.

### The paused-trigger caveat (KEDA 2.20.2)

Under the two-annotation pin, Flux does not touch the ScaledObject. A trigger change that merged
during the pin lands at the release, after the pause ends, so the caveat does not fire. The caveat
fires when a trigger change reaches a paused ScaledObject, for example under a lone
`paused-replicas`. The new trigger then reads nil after the pause ends. The formula cannot run.
The HPA shows `ScalingActive=False`. keda-operator logs
`invalid operation: <nil> >= int`, with a caret under the trigger. The Deployment holds its count,
and `KedaScalerErrors` does not fire. To prevent it, release a pin before, or together with, a
change to the triggers.

To fix it, recreate the ScaledObject. Do not restart `keda-operator`, because every ScaledObject in
the cell shares it.

1. Confirm that Flux can recreate the object. `loadgen` and its dependency (`apps` in prod-apne1,
   `flux-system` elsewhere) must both read Ready `True`.
   ```
   flux --context <cell> get kustomizations -n flux-system
   ```
2. If either is not Ready, do not delete. Without the ScaledObject, the fleet stays at its last
   count with no hard stop. Pin the cell to 0, fix the dependency, release, and come back.
3. When both are Ready, run the two commands together:
   ```
   kubectl --context <cell> -n giga-testnet-0 delete scaledobject seiload-saturation
   flux --context <cell> reconcile kustomization loadgen -n flux-system
   ```
4. Confirm as for a release.

### Runbooks

- [sei-load domain](https://github.com/sei-protocol/runbooks/blob/main/platform/sei-load/README.md)
- [KEDA domain](https://github.com/sei-protocol/runbooks/blob/main/platform/keda/README.md)
- [KedaScalerErrors](https://github.com/sei-protocol/runbooks/blob/main/platform/keda/keda-scaler-errors.md)
- [GigaValidatorRunawayLag](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/giga-validator-runaway-lag.md)

The sei-load mechanics live in `sender/sharded_sender.go` (retries and nonce recovery) and
`sender/txs_queue.go` (the per-account queue) in `sei-protocol/sei-load`.
