# SeiNetwork CRD reference (operations view)

> If this disagrees with the in-cluster CRD, **the CRD wins**.
> Run `kubectl explain seinetwork.spec --recursive` for the source of truth.
> File an issue at sei-protocol/sei-k8s-controller/issues if you spot drift.

## What it is

A `SeiNetwork` (`sei.io/v1alpha1`) is the genesis-required Kind: it describes a chain born from a fresh genesis ceremony and mints **N validator SeiNodes** from it. The validators are controller-generated (`generateSeiNode`) — they are not user-supplied SeiNodes. One `SeiNetwork` per chain; an RPC fleet is separate, standalone `SeiNode` followers (see `seinode-crd.md`), each peered in via `seictl node apply --network <id>`.

Used by `seictl network apply` to materialize the validator pool from the `genesis-chain` preset.

## Lifecycle phases

`status.phase` (`SeiNetworkPhase = Pending;Initializing;Ready;Paused;Degraded;Failed;Terminating`):

- `Pending` — not yet picked up by the controller
- `Initializing` — genesis ceremony + validator rollout in progress
- `Ready` — **terminal "up" phase.** Genesis assembled, validator pool reconciled. `seictl network watch --until=Ready` matches this. (Distinct from a SeiNode, whose terminal is `Running` — don't carry one vocab onto the other.)
- `Paused` — reconciliation held
- `Degraded` — running but below the expected validator readiness
- `Failed` — terminal failure
- `Terminating` — being torn down

## Spec fields you'll touch (operator's view)

The spec is flat (no `spec.template`):

- `spec.image` — full container image ref for the validators
- `spec.genesis.chainId` — the chain identifier (immutable)
- `spec.replicas` — validator count (immutable; default 4 via the preset)
- `spec.genesis.accounts[]` — funded accounts at genesis (`--genesis-account`)
- `spec.genesis.overrides{}` — flat dotted cosmos-module keys patched into the assembled `app_state` after collect-gentxs; module must exist, sub-fields unchecked (`--genesis-override`). Wrong deeper field names are injected silently and crash every node at InitChain — take keys from a real genesis, never from upstream-Cosmos docs (see the Genesis params section + sharp-edge note in `seictl-cli.md`)
- `spec.configOverrides{}` — per-node `config.toml`/`app.toml` overrides (the SeiNetwork equivalent of a SeiNode's `spec.overrides`; reached via `--set spec.configOverrides...`). There is **no `--override` flag** on `network apply`.

## Immutability (the new `updateStrategy`-class trap)

`spec.genesis` and `spec.replicas` are **admission-immutable** (CEL). Re-applying `network apply <same-name>` with a changed `--chain-id` or `--replicas` is rejected with `metav1.Status.reason=Invalid` — not a silent no-op. To change either: `delete` + re-create. A network minted at 4 replicas cannot be re-applied at 1.

## Status fields you'll read when debugging

- `.status.phase` — coarse-grained state (terminal "up" is `Ready`)
- `.status.readyReplicas` / `.status.replicas` — validator-pool readiness math
- `.status.plan[*]` — genesis-assembly + rollout plan; on terminal `Failed`, `.status.plan.failedTaskDetail.error` carries the cause and is lifted to stderr by `seictl network watch`
- `.status.observedGeneration` — drift detection

**Validators serve no EVM.** `ModeValidator` disables EVM HTTP/WS (and REST), so the validator SeiNodes carry no `.status.endpoint` — **never point load traffic at them.** RPC load goes at the follower SeiNodes (`role=node`), assembled via `node list` (see `cluster-inspection-recipes.md` recipe #1).

## Deletion — `deletionPolicy` is the disk-leak field

`spec.deletionPolicy` defaults to **`Retain`**. Under `Retain` the controller does not delete the generated validator SeiNodes on deletion — it **strips their owner reference** and leaves them running. Each orphan keeps its PVC and its EBS disk, garbage collection has no owner reference left to follow, and Flux prune never reaches them because the controller created them and Flux never held them in its inventory. The teardown looks clean and the spend continues.

The storage class is not the lever. A `Delete` reclaim policy releases a disk only when the PVC is deleted, and an orphaned SeiNode never releases its PVC.

**Mutable, unlike the immutable fields above.** No CEL rule and no webhook covers `deletionPolicy`, so the value can move from `Retain` to `Delete` on an existing network:

```sh
kubectl get seinetwork <id> -n eng-<alias> -o jsonpath='{.spec.deletionPolicy}'   # empty means Retain
```

**For a Flux-owned SeiNetwork the change goes through git, not `kubectl patch`.** The manifest rendered at spin-up normally carries the server-defaulted `Retain`, so Flux owns the field and reverts a live patch on its next reconcile — often while a removal PR is still in review, which is exactly when the revert does the damage. Set `deletionPolicy: Delete` in `engineers/<alias>/<task>/seinetwork-<id>.yaml`, merge, reconcile, and read back both git and the live object. `kubectl patch seinetwork <id> -n eng-<alias> --type=merge -p '{"spec":{"deletionPolicy":"Delete"}}'` is correct only where no reconcile owns the object.

**Either way it works only before deletion.** Once a `Retain` deletion has stripped the owner references and removed the parent, nothing restores the cascade — the leftover SeiNodes and PVCs need manual cleanup (`teardown.md`).

Under `Delete` the chain runs end to end: SeiNetwork deleted → validators deleted through their owner references → each SeiNode's finalizer deletes its data PVC → the storage class's `Delete` reclaim policy releases the EBS volume. The finalizer skips an **imported** PVC (`spec.import` on the SeiNode) by design.

Keep `Retain` only to preserve a validator's disk for forensics after the network goes away, and say so where the choice is made — a retained disk is a cost somebody chose.

`deletionPolicy` is orthogonal to the client-side `--cascade` propagation policy on `seictl network delete`; both apply. Full teardown procedure: `teardown.md`.

## Everything else

- `kubectl explain seinetwork.spec --recursive` (live cluster)
- `sei-protocol/sei-k8s-controller/api/v1alpha1/seinetwork_types.go` (source)

Don't enumerate the schema here. This file documents the operator workflow, not the field list.
