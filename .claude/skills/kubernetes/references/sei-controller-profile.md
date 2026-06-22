# Sei-controller profile (always-first overlay)

Loaded before any design or review. It encodes **sei-k8s-controller's own enforced conventions** and **overrides generic controller-runtime best-practice** in either direction — the way `/idiomatic`'s repo profile outranks generic idiom. These are distilled (and cited) from that repo's `CLAUDE.md` + code.

**Snapshot caveat:** this is a portable distillation for review and for controller work elsewhere. When you are working *inside* sei-k8s-controller, its live `CLAUDE.md` is the authority — read it and flag any drift from this profile rather than following a stale copy.

## The architecture in one paragraph

Single Go binary (`cmd/main.go`), controller-runtime v0.23.1 / kubebuilder v4.12.0, three reconcilers under one manager, all `sei.io/v1alpha1`: **SeiNetwork** (genesis-ceremony orchestration — bootstraps genesis.json + the founding validator set, owns child SeiNodes), **SeiNode** (single-node lifecycle), **SeiNodeTask** (one-shot sidecar-driven ops). The defining trait: a **plan-driven, level-triggered reconcile** — each controller builds an ordered `TaskPlan` in `.status.plan`, persists it atomically before executing, then drives tasks split between controller-side (StatefulSet/Service/PVC/Job via server-side apply) and sidecar tasks submitted over HTTP to a per-node seictl sidecar. It does **not** index chain events or use WebSockets, and it uses **no field indexers** (label-`List` lookups).

## Hard conventions (these override generic habit)

1. **No panic. Idempotent. Level-triggered.** No `panic` in controller code — return the error and let the reconciler retry. Every reconcile converges toward desired state regardless of current state; never drive off an event delta. *Cited:* repo `CLAUDE.md` ("No `panic` in controller code…"; "Keep reconcile loops idempotent…"); upstream `sources.md` §reconcile, §api-conventions (level-based).

2. **Optimistic-lock single-patch status writes (the headline rule).** Two near-simultaneous reconciles can both observe `status.plan == nil`, both build a plan, and without a resourceVersion-checked patch the second silently wins. So: snapshot `obj.DeepCopy()` once at the top, accumulate **all** status mutations in memory, and flush **one** patch at the end with `client.MergeFromWithOptimisticLock{}`. **Banned:** plain `MergeFrom` on status, `Status().Update` without rV verification, `client.Apply` on `.status`. The finalizer `Update` happens *before* snapshotting the base (it bumps resourceVersion). *Cited:* repo `CLAUDE.md` (status-patch section). **Review check:** every `r.Status().Patch` base uses `MergeFromWithOptimisticLock{}`.

3. **Conditions are always-present; never expressed by removal.** Once a controller sets a condition it stays present, transitioning between `True`/`False`/`Unknown` with a stable CamelCase `Reason`. "Feature off" / "feature broken" is `Status=False, Reason=<stable enum>` — never absence; spec-shape changes set `False`/`NotApplicable`, never `removeCondition`. Naming: `<Subject>Ready` / `InProgress` / `Complete` / `Needed`; don't mix polarities for one subject. **`Reason` is a public API** for runbooks + PromQL alerts — CamelCase enum, no dynamic data in the reason (that goes in the message). Every `setCondition` populates `ObservedGeneration = obj.Generation`. *Cited:* repo `CLAUDE.md` (conditions section); upstream `sources.md` §conditions, §observedGeneration.
   - **The one documented exception — the `kubectl wait` latch:** `SeiNodeTask.Status.Conditions[Ready|Failed]` latch *independently* on terminal state (mixed polarity), because the seitask-runner depends on `kubectl wait --for=condition=Ready=true` / `--for=condition=Failed=true`. Encode this as the sanctioned exception, not a violation.

4. **Planner owns conditions; the executor never does.** The planner builds the plan and manages all condition/phase transitions; the executor only mutates plan/task state in memory and never writes the cluster or sets conditions. *Cited:* repo `CLAUDE.md`; `internal/planner/doc.go`.

5. **The CRD contract is a one-way door.** Edit types in `api/v1alpha1/`, then run `make manifests generate` — never hand-edit `manifests/` or `zz_generated.deepcopy.go`. Field removal is a one-way door: deprecated fields are *retained*, not deleted. Immutable-after-create fields use CEL `XValidation` `self == oldSelf` (e.g. `replicas`/`genesis`/`dataVolume` on SeiNetwork, `chainId`, validator key Secret refs — rotating a live consensus key is a slashing risk). Discriminated-union specs (`fullNode|archive|replayer|validator` exactly-one) use CEL exactly-one rules. *Cited:* repo `CLAUDE.md`; upstream `sources.md` §api_changes (the compatibility law), §crd-versioning.

6. **Config keys are hyphenated** in seid `config.toml` (`persistent-peers`, not `persistent_peers`). *Cited:* repo `CLAUDE.md`.

7. **GovParamChange double-encode trap.** A `SeiNodeTask` param-change `changes[].value` is `apiextensionsv1.JSON` — pass a structured JSON **object**, not a pre-escaped string; integer params must be JSON strings. *Cited:* `api/v1alpha1/seinodetask_types.go`. (Matches the team's known double-encode hazard.)

8. **Lint is non-negotiable; idiom is the repo's.** All code passes `golangci-lint` — fix, don't suppress. Imports grouped stdlib / external / `github.com/sei-protocol/sei-k8s-controller`. "Three similar lines are better than a premature helper." Idiom conformance itself is `idiomatic-reviewer`'s pass (it digests this repo's `CLAUDE.md` + `doc.go` into a local idiom profile); this skill owns controller *correctness/contract*, not idiom. *Cited:* repo `CLAUDE.md`.

## The agent boundary (confirmed from the repo's subagent contract)

- **kubernetes-specialist** (this skill): CRDs, RBAC, kustomize *that the controller owns*, StatefulSet specs, reconcile logic — the controller code and its CRD contract.
- **idiomatic-reviewer**: the pure-idiom pass (Go + controller-runtime + repo patterns), backed by `/idiomatic`.
- **platform-engineer** (`/platform`): the controller's *deployment* — manifests, the `?ref=` staged rollout, manager-patch/config ConfigMap, IRSA/Pod-Identity, Karpenter. The controller *code* is this skill; the manifests *around* it are `/platform`.
- **k8s-capacity-management**: request/limit values, NodePool/scheduling.

## Known repo drift (flag, don't propagate)

The research that built this profile flagged stale spots in the repo to *not* treat as ground truth: `README.md` says "two CRDs" but there are three (SeiNodeTask is real); the `genesis-ceremony.yaml` sample shows a `template:` block the SeiNetwork spec doesn't have (children are synthesized in code); no webhook server is registered (admission validation is CEL-only). When you hit these, trust the code + `CLAUDE.md`, and flag the stale doc.
