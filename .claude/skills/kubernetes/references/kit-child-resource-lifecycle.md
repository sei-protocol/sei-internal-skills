# Child-resource lifecycle kit

## 1. What this concern is

The controller-side half of every plan is **child-resource management** — generating and owning the StatefulSet + headless Service + PVC (and bootstrap Jobs) for each SeiNode — via **server-side apply (SSA)**. This is the controller's most common daily task, and it carries non-obvious patterns a generic "create-if-not-exists / update-if-changed" habit gets wrong: SSA field-ownership, an `OnDelete`-driven controller-managed pod lifecycle, impostor detection, and orphan-on-Retain deletion. *Cited:* `internal/controller/seinetwork/internal_service.go:37`; `internal/planner/doc.go` (controller-side tasks do the SSA writes the executor itself never does).

## 2. The pattern (how this repo does it)

- **Server-side apply with explicit field ownership.** Child resources are applied SSA under a fixed `FieldOwner` (e.g. `seinetwork-controller`) **with `ForceOwnership`**, so the controller claims the fields it manages and resolves conflicts with other field managers deterministically. *Cited:* `internal_service.go:37`; `sources.md` §api-conventions (apply semantics).
- **`OnDelete` StatefulSet update strategy — the controller owns pod lifecycle.** Pods are not rolled by the StatefulSet controller; the reconciler drives a revision-gated **`replace-pod`** that is **readiness-blind** so a rollout proceeds even when `seid` is intentionally halted at a chain-upgrade height (a readiness-gated rollout would deadlock there). *Cited:* repo `CLAUDE.md` (replace-pod), `internal/task/replace_pod.go`.
- **Image-rollout observation.** `observe-image` polls `UpdatedReplicas` with an `ObservedGeneration` freshness check before stamping `currentImage` — so status reflects an actually-rolled image, not a requested one. *Cited:* the node planner's image-observe task.
- **UID-impostor detection.** A child StatefulSet whose UID doesn't match the expected owner is treated as an impostor: delete-and-defer rather than adopt — guards against a stale/foreign object of the same name. *Cited:* the real test `seinode_statefulset_test.go` ("Forge a UID mismatch").
- **Owner refs + finalizer-gated deletion.** Children carry `SetControllerReference` to the SeiNode (cascading GC); `IsControlledBy` checks before acting. On `DeletionTimestamp`, the finalizer flow runs idempotent cleanup; a `DeletionPolicy: Retain` **orphans** children by stripping the owner ref before removing the finalizer (so the PVC/data survives). *Cited:* the node controller `handleDeletion`; `sources.md` §finalizers.
- **Storage:** EBS gp3 StorageClasses, `WaitForFirstConsumer`. *Cited:* `config/storage/storage-classes.yaml`.

## 3. Anti-patterns / failure modes

- **Plain create/update instead of SSA.** A `Create` then `Update` (or a `client.Update` read-modify-write) on a child the controller co-owns. Cue: no `client.Apply`/SSA with a `FieldOwner`. Rewrite: SSA with the fixed field owner + `ForceOwnership`.
- **Omitting `ForceOwnership`.** SSA without it → an `Apply` conflict error when another manager touched a field. Cue: SSA apply with no force option + conflict handling. Rewrite: `ForceOwnership` (the controller is the authority for its fields).
- **Readiness-gated rollout.** Driving `replace-pod` on pod-Ready. Cue: a readiness wait before replacing. Rewrite: the replace is revision-gated and readiness-blind (a halted-at-upgrade node is not Ready by design).
- **Adopting an impostor.** Acting on a same-named child without checking UID/owner. Cue: no `IsControlledBy`/UID check before mutate. Rewrite: verify ownership; delete-and-defer an impostor.
- **Deleting data on Retain.** A finalizer cleanup that removes the PVC when the policy is Retain. Cue: unconditional child deletion in `handleDeletion`. Rewrite: on Retain, strip the owner ref (orphan), don't delete.
- **Reading the child right after writing it (cache staleness).** Asserting read-your-write through the cached client immediately after an SSA. Cue: a cached `Get` of the child used to decide, same reconcile, right after the apply. Rewrite: rely on the level-triggered requeue, not read-after-write.

## 4. Review cues

- **Dimension 1 (reconcile correctness):** SSA + `ForceOwnership`; revision-gated readiness-blind replace; image-observe freshness; no cache read-after-write assumption. *Basis:* profile §1, `sources.md` §api-conventions.
- **Dimension 3 (failure handling):** finalizer cleanup idempotent + removal-gated; orphan-on-Retain; impostor delete-and-defer. *Basis:* `sources.md` §finalizers; profile §5.
- **Dimension 4 (RBAC):** the controller's role grants exactly the verbs for the children it manages (StatefulSet/Service/PVC/Job), nothing wildcard. *Basis:* `sources.md` §rbac-markers.

## 5. One-way doors in this concern

- **The `FieldOwner` string** is the SSA ownership identity — changing it abandons the fields the old owner managed (a silent ownership split). Treat as a one-way door; flag.
- **The owner-ref/finalizer + `DeletionPolicy: Retain` orphan contract** governs whether a node's data volume survives deletion — a change here can destroy data; flag for human approval.
