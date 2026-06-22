# Controller-concern kit (TEMPLATE)

A kit is **data** the method loads for one controller concern (reconciliation, CRD design, sidecar integration, …). It teaches the pattern this codebase actually uses, cites the upstream canon beneath it, and gives review cues + the failure modes to catch. Adding a concern = drop one file conforming to this template at `references/kit-<concern>.md`.

Each kit provides the five sections below, in order, so the method stays concern-agnostic. Copy the skeleton; see `kit-plan-driven-reconciliation.md` for a worked kit.

This section schema is a **soft one-way door** — changing it churns every kit. Revise deliberately.

---

```markdown
# <Concern> kit

## 1. What this concern is
One paragraph: the pattern as sei-k8s-controller actually does it, and what
generic mental model gets it wrong here.

## 2. The pattern (how this repo does it)
The concrete shape — the types, the call sequence, the convention — cited to the
repo (file path) and to the upstream canon (`sources.md` §anchor). This is the
"do it this way" reference an author follows and a reviewer checks against.

## 3. Anti-patterns / failure modes
Named smells with a detection cue and the correct rewrite — the things a generic
controller habit gets wrong here (e.g. a non-optimistic-lock status write; a
condition expressed by removal; an annotation where an HTTP sidecar task belongs).

## 4. Review cues
What a reviewer looks for, mapped to the method's five dimensions (which dimension
this concern most exercises). Cite the profile rule / `sources.md` anchor each cue
rests on.

## 5. One-way doors in this concern
The irreversible decisions (a served CRD field's shape/semantics, an event/sidecar
contract) that must be flagged for human approval, not asserted.
```

---

**Authoring rules:**
- **Cite both layers:** the repo pattern (a file path in sei-k8s-controller) AND the upstream canon (`sources.md`) it specializes. A claim with neither is not a kit entry.
- The **profile** (`sei-controller-profile.md`) holds the cross-cutting hard conventions (optimistic-lock status, always-present conditions, no-panic) — kits reference it, don't restate it.
- Keep review cues mapped to the five method dimensions so findings stay rankable.

## Kit roster (shipped + deferred)

Shipped:
- `kit-plan-driven-reconciliation.md` — the ResolvePlan → persist → ExecutePlan model.
- `kit-sidecar-task-integration.md` — the seictl HTTP task API.
- `kit-crd-design.md` — discriminated unions, CEL validation/immutability, status subresource, the `kubectl wait` latch.

Deferred (add as a conforming kit when first encountered — the corpus grows by use):
- `kit-child-resource-lifecycle` — StatefulSet/Service/PVC/Job via server-side apply, `OnDelete` + readiness-blind `replace-pod`, image-rollout observation, UID-impostor detection, owner refs + finalizers.
- `kit-chain-bootstrap-modes` — genesis-ceremony two-level S3 rendezvous, the fail-closed state-sync witness gate (≥2 canonical syncers), the bootstrap-Job-then-teardown, replayer/archive.
- `kit-watches-and-requeue` — `GenerationChangedPredicate`, `Owns` vs `Watches`, the child-phase-changed predicate, requeue cadences (5s task poll, 30s steady, immediate-persist).
- `kit-controller-deployment-eks` — manager setup (leader election, secure metrics, OTel), kubebuilder RBAC markers, IRSA/Pod-Identity (cross-links `/platform`), Karpenter scheduling, S3 buckets. Keep thin; the deployment *manifests* are `/platform`.
