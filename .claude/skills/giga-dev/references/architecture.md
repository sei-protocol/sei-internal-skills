# giga-testnet-0 architecture

This file tells how the platform builds giga-testnet-0 and why. The load fleet and its scaler are
in [load-and-scaling.md](load-and-scaling.md). The queries are in
[observability.md](observability.md). Failure modes are in [diagnosis.md](diagnosis.md).

Contents: [Terms](#terms) · [In brief](#in-brief) · [Cells and regions](#cells-and-regions) ·
[Declaration and reconcile](#declaration-and-reconcile) · [Node roles](#node-roles) ·
[Autobahn consensus](#autobahn-consensus) · [EVM-only execution](#evm-only-execution) ·
[The execute loop](#the-execute-loop) · [Block retention](#block-retention) ·
[Performance envelope](#performance-envelope)

## Terms

| Term | Meaning |
|---|---|
| cell | One platform EKS cluster in one AWS region, with its own Flux, controller, KEDA and Prometheus. |
| lane | The block sequence of one validator. Each validator has one lane. |
| lane block | One block of one lane: up to 2,000 transactions. |
| commit range | The global blocks that one CommitQC finalizes. |
| global block | One lane block at its place in the chain order. Its number is the chain height. |
| head | One past the last global block of the newest CommitQC that a validator holds. |
| anchor | The newest commit range that a node has executed and persisted, and that has an AppQC. |
| shard owner | The validator whose lane takes the transactions of one sender address. |
| busy | The share of time that the execute loop spends outside the wait for the next block. |

## In brief

giga-testnet-0 is one Autobahn chain with EVM-only execution. It has 40 validators and 8 RPC full
nodes in four cells, one cell per AWS region. Load generators send EVM transfers to their cell's
two full nodes. A full node forwards each transaction to the sender's shard owner, in any region.
The owner admits it into its lane. Consensus orders the blocks of all 40 lanes into one sequence.
Every node executes every global block on one serial loop, and that loop sets the throughput limit.

## Cells and regions

| Cell | AWS region | Validator pods | Full node pods |
|---|---|---|---|
| `prod-use2` | us-east-2 (Ohio) | `validator-00-0` to `validator-09-0` | `fullnode-00-0`, `fullnode-01-0` |
| `prod` | eu-central-1 (Frankfurt) | `validator-10-0` to `validator-19-0` | `fullnode-10-0`, `fullnode-11-0` |
| `prod-euw1` | eu-west-1 (Ireland) | `validator-20-0` to `validator-29-0` | `fullnode-20-0`, `fullnode-21-0` |
| `prod-apne1` | ap-northeast-1 (Tokyo) | `validator-30-0` to `validator-39-0` | `fullnode-30-0`, `fullnode-31-0` |

All objects live in namespace `giga-testnet-0`. The SeiNode of pod `validator-NN-0` is
`validator-NN`. Each cell also runs 0 to 30 load generator pods.

No Kubernetes object spans two cells. Only the Autobahn network spans them.

| Each cell owns | The cells share |
|---|---|
| Its 10 validators and 2 full nodes | One genesis, one committee, one `autobahn.json` |
| One public NLB per validator, `validator-NN-p2p` | The platform Git repository |
| Its own controller, Flux and genesis bucket | Hub monitoring in cell `prod`: Thanos, Thanos Ruler, Grafana, `Loki-fleet` |
| Its own load Deployment and ScaledObject, on its own Prometheus | |

### Cross-cell connectivity

Every validator dials every other validator, and every full node dials every validator. A
validator's address is its NLB hostname, `validator-NN-p2p.giga-testnet-0.<cell>.platform.sei.io`:

- Port `26656` carries the Autobahn peer traffic: lane blocks, votes, QCs and block fetches.
- Port `8545` is the EVM JSON-RPC. Nodes forward transactions to the shard owner on it.

The `p2p-source-ranges` component limits each NLB to the NAT gateway IPs of the four prod VPCs, the
only egress of the nodes. Full nodes have no public endpoint. A recreated `validator-NN-p2p` Service gets a new NLB, and peers lose
the validator until DNS moves. The Services therefore carry `kustomize.toolkit.fluxcd.io/prune:
disabled` and `sei.io/deletion-protected: "true"`.

### Quorum

All 40 validators have voting power 1. The committee tolerates f = 13 faulty validators. Each quorum
that finalizes blocks or an app hash needs 27 of 40. The chain keeps going without one cell (30
remain). It stops without two cells (20 remain).

## Declaration and reconcile

giga-testnet-0 uses one standalone `SeiNode` per node. No `SeiNetwork` exists for it. The manifests
are in the platform repository:

- `clusters/<cell>/giga-testnet-0/validators/validator-NN/`: SeiNode, p2p Service, signing-key and
  node-key Secrets (SOPS).
- `clusters/<cell>/giga-testnet-0/fullnodes/fullnode-NN/`: SeiNode.
- `manifests/base/giga-testnet-0/`: the load fleet base, its PodMonitor, the NLB source ranges.

| SeiNode field (validator) | Value |
|---|---|
| `spec.consensus.engine` | `Autobahn`: points `config.toml` at `autobahn.json` |
| `spec.executionEngine.mode` | `EvmOnly`: the EVM-only executor replaces the Cosmos application |
| `spec.validator.signingKey`, `nodeKey` | Secrets |
| `spec.externalAddress` | the NLB hostname, port 26656 |
| `spec.resources.requests` | 28 CPU, 128Gi (memory limit 128Gi) |
| `spec.dataVolume.storage` | 8000Gi |
| `spec.configValues` | `app.toml` `giga.execution.parse_workers = 16`, the only override |
| `spec.image` | one sei-chain image, pinned by digest, on all 48 nodes |

A full node has `spec.fullNode: {}` and no keys or NLB. Every SeiNode carries the prune-disabled and
deletion-protected annotations, so a removed manifest does not delete the node.

For each SeiNode, the cell's controller creates a StatefulSet (one replica, `OnDelete`), a headless
Service and PVC `data-<name>` on storage class `gp3-10k-750`. It rolls at most 25% of the nodes in a
namespace at once (`rollout.driftUpdateBudgetPercent`). Before `seid` starts, the sidecar fetches
`giga-testnet-0/genesis.json` and `autobahn.json` from bucket `<cell>-sei-k8s-genesis-artifacts`. A
node is Ready when its EVM JSON-RPC answers (condition `EvmServing`).

Flux: `flux-system` applies `clusters/<cell>/giga-testnet-0/`, except on `prod-apne1`, where `apps`
applies it. In all four cells, `flux-system` is the parent of the `loadgen` Kustomization
(`clusters/<cell>/flux-system/loadgen.yaml`). `loadgen` applies the load fleet after the KEDA CRD
exists. On `prod-apne1` it depends on `apps`.

### The chain configuration

`autobahn.json` lists each validator's `validator_key`, `node_key`, `address` (NLB, 26656) and
`evmrpc` (`http://<NLB>:8545`), plus these chain-wide settings:

| Key | Value | Effect |
|---|---|---|
| `max_txs_per_block` | 2000 | Maximum transactions in a lane block. The code allows no more. |
| `block_interval` | `400ms` | A lane cuts a block that is not full 400 ms after its window has room. |
| `allow_empty_blocks` | `true` | The timer cuts a block even with no transactions. |
| `persistent_state_dir` | `data/autobahn` | Root of the durable Autobahn state on the PVC. |
| `dial_interval` | `10s` | Wait before a failed peer dial retries. |
| `evm_proxy_timeout` | absent: 10 s | Bound on one forward to a shard owner. |
| `evm_proxy_max_conns_per_owner` | absent: 512 | HTTP connections per shard owner. |
| `max_inbound_fullnode_peers` | absent: 10 | Non-committee peers that each validator serves. |

The view timeout comes from the `autobahn` section of `genesis.json`: 1.5 s when the genesis sets
none.

## Node roles

| Role | Count | Instance (observed) | Karpenter pool | Request | Data volume | Public endpoint |
|---|---|---|---|---|---|---|
| Validator | 40 | r6i.8xlarge, 32 vCPU | `sei-validator`: r5, r6i or r7i, 32 vCPU | 28 CPU, 128Gi | 8000Gi gp3, 10,000 IOPS, 750 MB/s | NLB |
| RPC full node | 8 | r6i.12xlarge, 48 vCPU | `sei-node`: r6i or r7i, 48 vCPU | 28 CPU, 128Gi | same | none |
| Load generator | 0 to 30 per cell | c6a, 16 or 32 vCPU | `default` | 2 CPU, 2Gi | none | none |

One giga node fits on each instance. The validator pool admits three instance families, so two
validators can differ in speed.

A full node has no keys and no mempool. It syncs from all 40 validators and executes every block.
It also serves its cell's load and forwards every transaction.

The chain has no seed, archive or snapshot nodes. Under `data/autobahn` each PVC holds the block
store (fsync on), the hash vault (one app hash per height), EVM state and receipts. A validator also
holds its persisted consensus state, so that a restarted validator does not vote twice.

## Autobahn consensus

### Lanes and the 30-block window

A lane cuts its next block when the open block is full: 2,000 transactions, or the byte or gas
limit. It also cuts when 400 ms pass after its window has room, even an empty block.

A lane may cut block n only when n is below the window start plus 30. That is three proposals of
the maximum lane advance (3 x 10). The window starts at the node's own anchor. The anchor moves after
the node executes a commit range, a quorum certifies the app hash, and the node persists both. The
same window bounds the other lanes' blocks that a node accepts and votes on. The mempool holds at
most 30 sealed lane blocks that the node has not executed.

Consequence: a lane runs at most 30 blocks ahead of what its own node has executed and the committee
has certified. A lane block becomes available when f + 1 = 14 validators vote for it (LaneQC).

### Ordering

Consensus runs one instance after another. Each view has a leader, picked by weighted random
choice. It proposes how far each lane advances: up to 10 blocks per lane with a LaneQC. The
proposal needs a PrepareQC and then a CommitQC, each 27 of 40. A view that passes the view timeout
ends with a TimeoutQC (27 of 40) and a new leader. Measured at about 100 blocks per second, the
chain finishes about 5 instances per second.

The CommitQC fixes the order of its range: lanes in committee order (sorted by public key), and each
lane's new blocks in order. Each global block is one lane block. Thus 40 lanes that each cut one
block per 400 ms give about 100 global blocks per second.

### The app hash

Every node executes every block and writes each app hash to its hash vault. A validator votes on one
app hash per commit range, the hash after its last block. The hash chains the previous hash, the
height, the block hash, the gas used and the change set. An AppQC needs 27 of 40 votes. A node whose
own result differs from an AppQC stops with `AppHash divergence`.

### Stage heights and the head

Each node reports `tendermint_internal_autobahn_data_next_block{stage}`, the next block not yet
through each stage:

| Stage | Next block not yet... |
|---|---|
| `qc` | covered by a CommitQC that the node holds |
| `receive` | received in contiguous order |
| `execute` | executed, with its app hash recorded |
| `certify` | covered by an AppQC |
| `evict` | persisted and dropped from memory |

A node holds at most 4,000 blocks in memory above its `evict` height. It accepts a CommitQC only
inside that window, so a node that is behind fetches as fast as it executes.

The head is `tendermint_internal_autobahn_avail_commit_global_block_number`. Lag is the highest head
of the 40 validators minus a node's `execute` height. Execution trails finalization a little, and
pod scrapes do not line up. A healthy node thus reads about 2,000 to 3,500 blocks of lag.

### Why the chain paces to the faster majority

1. A lane cuts blocks only inside its window above its own node's anchor.
2. The anchor needs an AppQC, and an AppQC needs 27 validators that have executed the range.
3. Thus the block rate follows the 27 fastest validators.
4. A slower validator does not slow the chain. It falls behind, and its lag grows while the chain
   outruns it.
5. A lagging validator still owns its shard. Its anchor lags, so its lane and mempool fill and stop
   admitting. Sends to its shard wait until the forward times out.

Code: sei-chain `sei-tendermint/autobahn/types/committee.go` (quorums, shards) and
`sei-tendermint/internal/autobahn/avail/state.go` (lane window).

## EVM-only execution

With `executionEngine.mode: EvmOnly`, `seid` runs the EVM-only application:

- EVM chain ID `713715`. The node serves only EVM JSON-RPC on 8545 and WebSocket on 8546. It serves
  no CometBFT RPC, REST or gRPC.
- Parallel execution with optimistic concurrency control (OCC): speculate, validate, merge.
- An account with no stored state reads a balance of 2^200 wei, so load accounts need no funding.
- Base fee 0. A transaction below 1 gwei effective gas price is invalid in a block.
- The executor records a transaction that it cannot apply as rejected and keeps the block valid.
  One bad transaction cannot halt the chain.

`EvmShard(sender)` reduces the SHA-256 of the sender address modulo the total committee weight, then
walks the committee in key order. Each validator owns about 1/40 of the senders, in all four
regions.

### The path of one transaction

1. A load pod signs an EVM transfer. It picks one of its cell's two full nodes by sender address and
   calls `eth_sendRawTransaction`.
2. The full node computes `EvmShard(sender)` and forwards the call to the owner's `evmrpc` URL.
3. The owner calls `InsertTx`:
   - `CheckTx` runs, at most half of `GOMAXPROCS` at a time. It recovers and caches the sender.
   - The nonce must equal the next nonce that the mempool expects: the tracked nonce of transactions
     not yet executed, else the executed nonce. Any other nonce fails with `bad nonce`.
   - The transaction joins the open lane block. If the mempool holds 30 unexecuted sealed blocks
     and the open block is not empty, the call waits in a first-in, first-out queue. With 4,096
     calls waiting, a new call fails with `mempool is full: too many pending inserts`
     (`pending_full`).
4. The owner answers when it admits the transaction. The full node returns the hash.
5. The lane cuts the block. A LaneQC forms, and consensus puts the block in a commit range.
6. Every node executes the block and votes on the app hash at the end of the range.

The full node also forwards `eth_getTransactionCount` with tag `pending` to the owner. A validator
forwards a transaction for another shard when it has a live link to the owner; if not, it admits it.

### The forward

Each node keeps one pool of at most 512 HTTP connections per owner. One forward has a 10 s deadline.
It covers the wait for a connection, the owner's admission wait and the response. A late forward
fails with `evm proxy: shard owner did not answer in time`. The counter
`tendermint_p2p_evm_proxy_requests{outcome}` counts `ok`, `rpc_error`, `timeout`,
`transport_error` and `canceled`. When an owner lags, its mempool stays full. Each send to its shard
then waits the full 10 s and holds a load generator slot that long.

### How full nodes follow the chain

A full node dials all 40 validators. Each validator serves at most 10 peers without a committee
claim by default, so the 8 full nodes fit. On each connection the full node streams CommitQCs and
AppQCs. It fetches blocks, up to 100 at once, with a 2 s timeout and a 1 s retry. It checks each QC,
executes every block and compares its app hash with the AppQC.

## The execute loop

A fetcher reads block n+1 and calls `PrepareBlock` on it while the loop executes block n.
`PrepareBlock` decodes the transactions and recovers the senders. It reuses the senders that
`CheckTx` cached, so an owner recovers its own lane's senders once. The loop then runs these steps
per block:

1. `FinalizeBlock` opens a state view and runs OCC.
2. `FinalizeBlock` encodes the change set and receipts, writes the receipts and starts the state
   commit. That commit runs in the background during the next block.
3. The loop writes the app hash to the hash vault, with fsync. A failure halts the node.
4. It calls the application `Commit`. Under real execution this only moves a cursor.
5. It does bookkeeping, then publishes the app hash. At the last block of a commit range, the
   publish waits until the block store persists the hash.
6. It prunes below the retain height from `Commit`.

The loop is in sei-chain `sei-tendermint/internal/p2p/giga_router_common.go`.

### The three timing buckets

The counter `sei_chain_autobahn_main_loop_phase_duration_seconds_total{phase}` holds seconds per
bucket:

| Bucket | Contains |
|---|---|
| `consensus` | The wait for the next prepared block. A node that is behind waits almost zero. |
| `execution` | From the start of `FinalizeBlock` until OCC returns. |
| `storage` | From the end of OCC to the next wait: encoding, receipt write, start of the state commit, hash vault fsync, `Commit`, bookkeeping, app-hash publish, prune. |

Busy is everything outside `consensus`. For milliseconds per block, divide the counter rate by the
executed block rate. The `..._phase_latency_seconds` histogram records `execution` and `storage`
twice per block, so its `_sum/_count` reads half the true time. The counters
`sei_chain_evmonly_block_phase_duration_seconds_total` and
`sei_chain_autobahn_storage_tail_phase_duration_seconds_total` split the buckets further.

### Why the loop saturates

1. Each block has a fixed cost, whatever its size: the vault fsync, the publish, the prune, and the
   view, OCC and encoder setup. Measured: roughly 1 to 1.5 ms.
2. Each transaction adds roughly 9 microseconds (measured), for the most part OCC and encoding.
3. The block rate does not follow the load. Each of 40 lanes cuts at least every 400 ms, so below
   saturation the chain makes about 100 blocks per second.
4. The loop thus spends roughly 10 to 15% of each second on fixed cost. That is the intercept of the
   measured relation busy ≈ 0.11 + 0.89 x (tx/s) / 98,000.
5. At about 98,000 tx/s the loop is full. The lane windows fill, blocks grow toward 2,000
   transactions, and the block rate falls to about 55 per second. Larger blocks spread the fixed
   cost, and throughput settles near 100,000 tx/s.
6. Near this edge the block rate is not stable. Smaller blocks at the same load raise the block rate
   and the fixed cost. A validator with a few percent less margin then crosses 100% busy.
7. Every validator executes every block, so no spare capacity exists. A validator that is behind
   waits zero in `consensus` and runs at 100% busy. Measurements show recovery only after median
   busy falls below about 90%.

Speed varies per validator. Measurements show some validators 30 to 40% slower per block than peers
on the same instance type, even while caught up. A validator that is behind measured 10 to 50%
slower than the median for the same blocks. No one knows the cause yet.

The scaler steers on each cell's median busy and does not see one slow validator. That validator
can run at 95 to 100% own busy while the median reads about 76%. Check per-validator own busy, not
only the median. Own busy at 95% or more means no margin: that node is the next straggler. Mean
send latency and acceptance do not show execute-loop saturation. Busy shows it.

## Block retention

### Real execution

- Every 5 minutes, a storage collector on each node takes the lowest store head, minus a rollback
  window of 1,000 blocks. It subtracts a lookback window of 4,320,000 blocks and prunes the block
  store, the state history and the receipts below that line.
- The lookback window is `giga.storage.lookback_window`, the sei-db default of 12 hours at 100
  blocks per second. The manifests set no override.
- The block store rounds the line down to the start of a commit range, and never prunes above the
  newest certified range.
- The application returns no retain height, so the loop's own prune does nothing.
- Each node thus keeps about 4,321,000 blocks below its own executed head.

When the head passes the window:

- No peer holds block 1. A node with a new or wiped volume cannot sync from the network. It needs a
  snapshot or state sync (see the fresh-start design in [diagnosis.md](diagnosis.md)).
- A node more than the window behind its peers cannot fetch its next block.
- A restarted node resumes from its committed height. If its block store lacks that height, it
  stops with `app tip ... is unavailable in BlockStore; restore matching BlockStore data or
  state-sync the node`.
- A validator volume holds its persisted votes. A new volume with the same signing key loses them,
  so a volume rebuild is never a quick fix.

The defaults are in sei-chain `sei-db/config/gc_config.go`.

### The mock-app mode

`mock-app = true` in `config.toml` (marked TEST-ONLY) replaces the EVM-only application with an
in-memory nonce application. The consensus path, block store and hash vault stay real.

| | Real execution | Mock app |
|---|---|---|
| Execution | EVM, state on disk | sender recovery, nonce check, app-hash roll; no EVM state |
| After a restart | resumes from the committed height | state lost; starts again at block 1 |
| Retention | about 4,321,000 blocks, pruned every 5 minutes | 10,000 blocks, pruned after each block |
| Catch-up limit | about 4,321,000 blocks behind | 10,000 blocks behind |

Under the mock app, a restarted node crash-loops once the prune passes block 1:
`panic: ... r.data.GlobalBlock(1): blockStore.ReadBlockByNumber(1): pruned: below retention watermark`.
The lag brakes for real execution (12,000 and 16,000 blocks) sit above the mock-app window.

To tell the mode, use `sei_chain_giga_evmonly_txs_executed_total{status="success"}`. It grows at
the chain's tx rate under real execution, and is absent or flat under the mock app. Without
`status="success"`, a rate over all series mixes in the failure statuses.

## Performance envelope

Measurements on this fleet give the values below. They are not guarantees. A new image, instance
family or load shape moves them.

| Regime | tx/s | Blocks/s | tx per lane block | Median busy | ms per block: execution + storage |
|---|---|---|---|---|---|
| Light load | low | about 100 (timer) | few | about 10 to 25% | fixed cost only |
| Scaler band | about 70,000 | about 97 to 100 | about 700 to 750 | about 75 to 77% | about 5 + 2.7 |
| Edge of saturation | about 98,000 to 102,000 | about 65 to 85 | about 1,150 to 1,550 | about 96 to 98% | about 8 to 11 + 3 to 4 |
| Saturated | about 100,000 to 107,000 | about 54 to 60 | about 1,800 to 1,960 | about 98 to 99% | about 12 to 14 + 4 to 7 |
| Far past saturation | about 72,000 | under 40 | about 1,950 | 100% | execution about 23 |
| Mock app, saturated | about 220,000 | about 120 | about 1,850 | about 97 to 99% | about 4.2 + 3.8 |

- Real execution saturates near 100,000 tx/s. More offered load adds queue time and failed sends.
  Far past saturation (60 uncapped pods per cell, with no busy band), throughput falls.
- At saturation a block takes about 18 ms. OCC speculation takes about 8.5 ms and OCC validation
  about 3.4 ms. The vault fsync takes about 1 ms, and `Commit` under 0.1 ms.
- At about 70,000 tx/s, a lane block reaches `receive` about 0.4 s after its cut (mean). It reaches
  `execute` after about 0.85 s and `certify` after about 0.95 s.
- The scaler band keeps the median validator at 70 to 90% busy. It does not by itself give every
  validator margin. See [Why the loop saturates](#why-the-loop-saturates).
- Full nodes run less busy than validators: about 65% against 76% at 70,000 tx/s, on larger
  instances.

Runbooks: [giga-validators](https://github.com/sei-protocol/runbooks/blob/main/platform/giga-validators/README.md),
[giga-testnet-runbook](https://github.com/sei-protocol/runbooks/blob/main/giga-testnet-runbook.md).
