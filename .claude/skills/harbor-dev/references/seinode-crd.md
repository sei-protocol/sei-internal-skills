# SeiNode CRD reference (operations view)

> If this disagrees with the in-cluster CRD, **the CRD wins**.
> Run `kubectl explain seinode.spec --recursive` for the source of truth.
> File an issue at sei-protocol/sei-k8s-controller/issues if you spot drift.

## What it is

A `SeiNode` (`sei.io/v1alpha1`) describes a single Sei blockchain node — its image, role (validator / fullnode / archive / replayer), peer discovery strategy, storage, and bootstrap method.

The controller reconciles each `SeiNode` into:

- A **StatefulSet** (1 replica) running `seid` with a `seictl` sidecar
- A **headless Service** for peer connectivity
- A **PersistentVolumeClaim** (data volume)
- A **task plan** executed via the seictl sidecar (snapshot-restore, configure-genesis, config-apply, peer-discovery, mark-ready)

## Lifecycle phases

`status.phase` (`SeiNodePhase = Pending;Initializing;Running;Failed;Terminating`):

- `Pending` — not yet picked up by the controller
- `Initializing` — task plan executing
- `Running` — **terminal "up" phase.** Task plan complete, steady-state polling. **There is no `Ready` on a SeiNode** — an operator (or a `node watch --until=Ready`) that waits for `Ready` waits forever; `--until=Running` is the correct terminal. (`Ready` is a `SeiNetwork` phase, not a node phase.)
- `Failed` — terminal; operator must delete and recreate
- `Terminating` — being torn down (finalizer running)

**`Running` is discoverability, not serve-readiness.** It means config applied + sidecar self-marked ready, NOT that the EVM listener is accepting connections — there is a real post-Running window where `.status.endpoint` is published but a dial gets connection-refused. Before pointing a load tool at a follower, run `seictl node watch <name> --until=caught-up` (the SDK serve-readiness gate: `catching_up=false` with height>1, plus EVM serving when the node publishes an EVM endpoint).

## Spec fields you'll touch (operator's view)

The 6 fields engineers actually edit:

- `spec.chainId` — the chain identifier this node joins
- `spec.image` — full container image ref (with tag or digest); flat (no `spec.template`)
- `spec.peers` — peer discovery (one of `EC2Tags`, `Static`, `Label`). For a network's follower, the `Label` selector keys `sei.io/seinetwork` (set automatically by `seictl node apply --network <X>`).
- `spec.fullNode | archive | replayer | validator` — mutually exclusive role marker
- `spec.fullNode.snapshot` — bootstrap-from-snapshot config (exactly one of `s3` | `stateSync`); `snapshot.rpcServers` declares ≥2 light-client witness endpoints (bare `host:port`) replacing the platform syncer registry — the self-service path for state-syncing onto your own chain. See `state-sync-bootstrap.md`
- `spec.sidecar` — seictl sidecar overrides (image, env, resources)
- `spec.overrides` — TOML config patches applied via seictl `config patch`

## Object labels (producer↔consumer contract)

A follower minted by `seictl node apply --network <X>` carries `metadata.labels{sei.io/seinetwork=<X>, sei.io/role=node}` — this is exactly what the fleet recipes match (`node list -l sei.io/seinetwork=<X>,sei.io/role=node`). The same contract is shared with seitask-rendered SeiNodes. A hand-rolled SeiNode that omits these labels is **invisible** to the fleet recipes. The SeiNetwork controller stamps the same `sei.io/seinetwork=<X>` plus `sei.io/role=validator` on the validators it generates.

## Status fields you'll read when debugging

The fields engineers actually read:

- `.status.phase` — coarse-grained state (see lifecycle above; terminal "up" is `Running`)
- `.status.endpoint.{evmJsonRpc, evmWs, tendermintRpc, tendermintRest}` — the node's published in-cluster service URLs (scalar leaves; present for `fullNode`/`archive`, **absent** for `validator`/`replayer`, which serve no EVM/REST). **Use these verbatim** — the controller owns the per-node headless DNS form; never reconstruct them. This is the field every follower read in the recipes depends on.
- `.status.conditions[type=Ready]` — boolean readiness condition with `.message` for cause (a condition, distinct from the phase — the phase has no `Ready`)
- `.status.plan[*].state` and `.status.plan[*].lastError` — per-task execution state; on terminal `Failed`, `.status.plan.failedTaskDetail.error` carries the cause
- `.status.observedGeneration` — drift detection (controller hasn't seen latest spec yet)

## Everything else

The full schema lives at:

- `kubectl explain seinode.spec --recursive` (live cluster)
- `kubectl explain seinode.status.endpoint` (the published endpoint leaf)
- `sei-protocol/sei-k8s-controller/api/v1alpha1/seinode_types.go` (source)

Don't enumerate the schema here. This file documents the operator workflow, not the field list.
