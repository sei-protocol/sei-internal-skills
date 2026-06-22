# The method — designing & reviewing controller code

Two modes, one spine: **design** (the kubernetes-specialist authoring a controller/CRD) and **review** (a lens over existing controller code). Both load the profile + kit first, judge against the profile's hard conventions before the upstream canon, and rank one-way-door / correctness findings above style.

## The four stages

1. **Load.** `sei-controller-profile.md` (always first — its conventions override generic habit) + the kit(s) for the concern (reconciliation, CRD design, sidecar integration). If working *in* a repo, read its `CLAUDE.md`; the live repo wins over this skill's snapshot — flag drift.

2. **Read the whole target.** For review: the CRD types, the reconciler(s), the child-resource generators, the tests, the RBAC markers. For design: the existing CRDs/controllers in scope + the interface(s) you consume/provide. Never design or review a controller from memory of "how operators usually work" — this codebase has specific patterns (plan-driven reconcile, sidecar HTTP tasks) a generic mental model gets wrong.

3. **Apply the five dimensions** (below), profile-first. Each finding cites a `sources.md` anchor and/or a profile rule. Mark a genuinely-uncertain call as such rather than forcing it.

4. **Rank + surface.** One-way doors (incompatible CRD-field/semantics changes, event/sidecar-contract changes) and correctness defects lead; idiom/style is bundled and never leads (idiom is `/idiomatic`'s pass anyway). Flag one-way doors for human approval — never assert the breaking change as the fix.

## The five dimensions (the scorecard)

Grounded in the upstream canon (`sources.md`) and specialized by the profile.

1. **Reconcile correctness & idempotency.** Level-triggered — converges to desired state from observed cluster state regardless of what changed; safe to run N times; doesn't drive off an event delta; doesn't read its own status to decide (reconstruct each run). In this repo specifically: the plan-driven model (build plan → persist with optimistic lock → execute) and the stateless executor. **Also under this dimension:** *watch/predicate correctness* — a wrong `GenerationChangedPredicate` or custom predicate is a silent missed-or-storming-reconcile bug (see `kit-child-resource-lifecycle` / the deferred `kit-watches-and-requeue`); *SSA field-ownership* — child writes use `FieldOwner` + `ForceOwnership`, and a field-manager conflict is a correctness defect; *cache staleness* — the controller-runtime client read is cache-backed, so don't assume read-your-write right after a write — rely on the level-triggered requeue. *Basis:* `sources.md` §reconcile, §api-conventions(level); profile §1–§2, §4.

2. **CRD-contract durability.** spec vs status separation; **no incompatible in-version change** to a served field's shape, validation, or semantics (the one-way-door law); evolution via a new version + storage version + conversion strategy. CEL `XValidation` immutability on create-only fields; `make manifests generate` (never hand-edit generated files); deprecated fields retained. *Basis:* `sources.md` §api_changes, §crd-versioning; profile §5.

3. **Failure-mode handling.** Errors/`RequeueAfter`/backoff used correctly; **no panic** that crashes the manager; finalizer cleanup is idempotent and removal-gated on a non-zero `DeletionTimestamp`; transient vs terminal distinguished (`Terminal(err)`). *Basis:* `sources.md` §reconcile, §finalizers; profile §1.

4. **RBAC least-privilege.** Generated RBAC (`+kubebuilder:rbac` markers) scoped to the exact verbs/resources used — nothing wildcard it doesn't need; the controller's cloud access scoped per-ServiceAccount (IRSA/Pod-Identity), IMDS not exposed. *Basis:* `sources.md` §IRSA, §rbac-markers.

5. **Observability via status.** Conditions are always-present, CamelCase `Reason` (the public alerting/runbook API), `ObservedGeneration` populated so consumers can tell reconciled-from-stale; events/metrics on reconcile outcomes. *Basis:* `sources.md` §conditions, §observedGeneration; profile §3.

**+ Testability (the sixth, recommended).** envtest / fake-client coverage of the reconcile paths *including* deletion/finalizer and requeue branches — not just the happy path. *Basis:* `sources.md` §envtest.

## Design discipline (when authoring, not just reviewing)

- Argue the **maximum scope you'd defend** in the controller domain, then name what you'd **cut first** for an MVP and the condition that un-defers it — the orchestrator/human picks the minimum (the established agent output-discipline).
- A new CRD field or event signature is a **one-way door** once a consumer depends on it — design it as if you can't change it, and flag it for human approval before finalizing.
- Reach for the kit's pattern over a generic one: a status write is the optimistic-lock single-patch (profile §2), not a `Status().Update`; a sidecar side-effect is an HTTP task submit (the sidecar kit), not an annotation.
