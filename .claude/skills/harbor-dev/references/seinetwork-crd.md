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
- `Ready` — **terminal "up" phase.** Genesis assembled, validator pool reconciled. `seictl network watch --until=Ready` matches this. (Distinct from a SeiNode, whose terminal is `Running` — do not carry one vocab onto the other.)
- `Paused` — reconciliation held
- `Degraded` — running but below the expected validator readiness
- `Failed` — terminal failure
- `Terminating` — teardown in progress

## Spec fields you will touch (operator's view)

The spec is flat (no `spec.template`):

- `spec.image` — full container image ref for the validators
- `spec.genesis.chainId` — the chain identifier (immutable)
- `spec.replicas` — validator count (immutable; default 4 via the preset)
- `spec.genesis.accounts[]` — funded accounts at genesis (`--genesis-account`)
- `spec.genesis.overrides{}` — flat dotted cosmos-module keys patched into the assembled `app_state` after collect-gentxs; module must exist, sub-fields unchecked (`--genesis-override`). The assembler injects wrong deeper field names silently, and every node then crashes at InitChain. Take keys from a real genesis, never from upstream-Cosmos docs (see the Genesis params section + sharp-edge note in `seictl-cli.md`)
- `spec.configOverrides{}` — per-node `config.toml`/`app.toml` overrides (the SeiNetwork equivalent of a SeiNode's `spec.overrides`; reached via `--set spec.configOverrides...`). `network apply` has **no `--override` flag**. Init-path only: an edit on a Running network never reaches the pool. Prefer `spec.configValues`.
- `spec.configValues[]` — typed `{fileName, key, value}` overlay on every validator's generated `config.toml`/`app.toml` (≤100 entries; `fileName` `^[A-Za-z0-9_-]+\.toml$`; dotted `key`; JSON `value`, `null` refused). Authored with `--config-value <file>.toml:<key>=<value>`. The network's list is **authoritative for its children**: the controller validates it (`ConfigValuesValid` condition, `InvalidConfigValues` warning event; on failure children keep their last good set), then copies it into each child `SeiNode.spec.configValues` and overwrites any direct child edit. Layered after the base config and the controller's `[p2p]` keys, so it wins over both and over `configOverrides`. **Day-2 semantics:** a change on a Running network builds a `config-update` plan on every child at once — regenerate base → patch peers → overlay → validate → restart seid — so the whole validator pool restarts together and block production pauses until >2/3 are back. Nothing rolls the StatefulSet or image. Regeneration also resets out-of-band `[statesync]` and giga-migration keys unless they are expressed here too. Rules and verification: `seictl-cli.md` → *Typed config values*.
- `spec.resources` — the seid container footprint for every validator in the pool (`--cpu` / `--memory`). Request-only; `requests` accepts **only** `cpu` and `memory`. **Create-only.**
- `spec.dataVolume.storage` — the data-PVC size for each pool validator (`--storage`), at the nested path `spec.dataVolume.storage.resources.requests.storage`. **Create-only.**
- `spec.dataVolume.storage.volumeAttributesClassName` — the storage performance selection for each pool validator, a sibling of the size path above. Names a platform-managed VolumeAttributesClass, which carries the gp3 IOPS and throughput. `network apply` resolves the name from the `--iops` and `--throughput` pair you supply. Unset means the standard tier. **Create-only.**

## Immutability (the new `updateStrategy`-class trap)

`spec.genesis`, `spec.replicas`, `spec.resources`, and `spec.dataVolume.storage` are **admission-immutable** (CEL). The apiserver rejects a re-apply of `network apply <same-name>` that changes `--chain-id`, `--replicas`, `--cpu`, `--memory`, `--storage`, `--iops`, or `--throughput`, with `metav1.Status.reason=Invalid` — not a silent no-op. To change any of them: `delete` + re-create. A network minted at 4 replicas cannot be re-applied at 1, and a pool minted at 4 CPU cannot be re-applied at 8.

The resource fields are create-only for mechanical reasons, not policy. Each child StatefulSet uses `OnDelete` with image-only drift detection, so a changed footprint never reaches a running pod. The controller creates each data PVC once and never updates it, so a changed size never reaches the volume. The VolumeAttributesClass name binds at that same provision, so a re-selected tier never reaches it either. Resizing a pool is therefore a new chain — `delete`, fresh chain-id, re-create — and the pool's data PVCs go with it.

**Provenance:** the resource immutability reasons and the limits rules here come from `sei-k8s-controller` main @ `c3fabbf` (2026-09-10) — `api/v1alpha1/seinetwork_types.go` plus the shared `DataVolume*` types, and the generated `config/crd/sei.io_seinetworks.yaml`.

**`resources.limits`:** `limits` accepts only `memory`, and `limits.memory` must equal `requests.memory`. The CRD rejects a CPU limit outright. The controller derives the memory limit from the request, so a rendered CR normally carries no `limits` block; `seictl` refuses to render `spec.resources.limits.cpu` from any source.

## Status fields you will read when debugging

- `.status.phase` — coarse-grained state (`Ready` is the terminal "up")
- `.status.readyReplicas` / `.status.replicas` — validator-pool readiness math
- `.status.plan[*]` — genesis-assembly + rollout plan; on terminal `Failed`, `.status.plan.failedTaskDetail.error` carries the cause, and `seictl network watch` lifts it to stderr
- `.status.observedGeneration` — drift detection

**Validators serve no EVM.** `ModeValidator` disables EVM HTTP/WS (and REST), so the validator SeiNodes carry no `.status.endpoint` — **never point load traffic at them.** RPC load goes at the follower SeiNodes (`role=node`), assembled via `node list` (see `cluster-inspection-recipes.md` recipe #1).

## Deletion

`spec.deletionPolicy` defaults to `Delete`: the generated validator SeiNodes go with the network by ownerReference. Each SeiNode's finalizer then deletes its StatefulSet, pod, and controller-managed PVC. An imported PVC (`spec.dataVolume.import`) survives. `Retain` orphans the validators instead and stamps each with `sei.io/retained-from-seinetwork` and `sei.io/retain-reason`. Set it only when a validator's consensus identity must outlive the network.

The default applies at admission, so a SeiNetwork created before it changed keeps `Retain` in its persisted spec. Read the live value before a teardown:

```sh
kubectl get seinetwork <name> -n eng-<alias> -o jsonpath='{.spec.deletionPolicy}'
```

On `Retain`, either set `deletionPolicy: Delete` in the manifest and merge that one PR ahead of the `git rm`, or delete the retained SeiNodes by hand afterwards.

`spec.deletionPolicy` is orthogonal to the client-side `--cascade` propagation policy on `seictl network delete`; both apply.

## Everything else

- `kubectl explain seinetwork.spec --recursive` (live cluster)
- `sei-protocol/sei-k8s-controller/api/v1alpha1/seinetwork_types.go` (source)

Do not enumerate the schema here. This file documents the operator workflow, not the field list.
