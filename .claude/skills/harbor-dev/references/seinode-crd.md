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

`status.phase`:

- `Pending` — not yet picked up by the controller
- `Initializing` — task plan executing
- `Running` — task plan complete, steady-state polling
- `Failed` — terminal; operator must delete and recreate

## Spec fields you'll touch (operator's view)

The 6 fields engineers actually edit:

- `spec.chainId` — the chain identifier this node joins
- `spec.image` — full container image ref (with tag or digest)
- `spec.peers` — peer discovery (one of `EC2Tags`, `Static`, `Label`)
- `spec.fullNode | archive | replayer | validator` — mutually exclusive role marker
- `spec.sidecar` — seictl sidecar overrides (image, env, resources)
- `spec.overrides` — TOML config patches applied via seictl `config patch`

## Status fields you'll read when debugging

The 4 fields engineers actually read:

- `.status.phase` — coarse-grained state (see lifecycle above)
- `.status.conditions[type=Ready]` — boolean readiness with `.message` for cause
- `.status.plan[*].state` and `.status.plan[*].lastError` — per-task execution state
- `.status.observedGeneration` — drift detection (controller hasn't seen latest spec yet)

## Everything else

The full schema lives at:

- `kubectl explain seinode.spec --recursive` (live cluster)
- `sei-protocol/sei-k8s-controller/api/v1alpha1/seinode_types.go` (source)

Don't enumerate the schema here. This file documents the operator workflow, not the field list.
