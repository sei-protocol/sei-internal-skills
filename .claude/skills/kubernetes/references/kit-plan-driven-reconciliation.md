# Plan-driven reconciliation kit

## 1. What this concern is

Every sei-k8s-controller reconciler is **plan-driven and level-triggered**: it builds an ordered `TaskPlan` from observed state, **persists that plan to `.status.plan` before executing anything**, then drives the plan's tasks. The generic mental model — "diff desired vs actual and imperatively apply the delta inline" — is wrong here: side effects are mediated through an explicit, observable plan, and plan-building is separated from plan-execution. *Cited:* `internal/planner/doc.go`; `sources.md` §reconcile (level-based), §api-conventions.

## 2. The pattern (how this repo does it)

The reconcile loop is three phases under the profile's single-patch status discipline:

1. **Resolve** — `ResolvePlan` / `ForGroup` reads current state and builds an ordered `TaskPlan` of `PlannedTask`s. The **planner owns conditions/phase**; it sets `TargetPhase` (empty = retry, non-empty = terminal `FailedPhase`). *Cited:* `internal/planner/`, profile §4.
2. **Persist atomically** — when a freshly built plan differs from `.status.plan`, the controller **flushes it and requeues immediately (`ResultRequeueImmediate`) WITHOUT executing** — so an observer (and a restart) sees the plan before any side effect. *Cited:* `internal/controller/node/controller.go:146-166`.
3. **Execute** — on the next reconcile, `Executor.ExecutePlan` drives each task. The executor is **stateless per reconcile** and **never writes the cluster** — it mutates only in-memory status (plan/task state, phase). Each task implements `Execute` (idempotent submit) / `Status` (poll) / `Err`, with `Terminal(err)` distinguishing a terminal failure from a transient one (retry with bounded backoff). Deterministic UUIDv5 task IDs let a restarted controller rejoin an in-flight task. *Cited:* `internal/task/task.go:48-77,132-138`, `internal/planner/doc.go`.

All status mutations across the phases accumulate in the single `obj.DeepCopy()` snapshot and flush through **one** `MergeFromWithOptimisticLock` patch (profile §2).

## 3. Anti-patterns / failure modes

- **Execute-on-build (the headline bug).** Building a plan and running its tasks in the *same* reconcile. Cue: a `ResolvePlan` followed by `ExecutePlan` with no intervening persist+requeue. Rewrite: persist the new plan and `ResultRequeueImmediate`; execute next pass. (Otherwise observers/restarts never see the plan that produced the side effects.)
- **Executor writes the cluster.** A task's `Execute` doing a `client.Patch`/`Update` on something other than its target side effect, or setting a condition. Cue: cluster writes or `setCondition` inside the executor/task path. Rewrite: planner sets conditions; the task performs its one idempotent side effect (SSA child resource, or HTTP sidecar submit) and reports status.
- **Reading status to decide.** Branching on `.status` from the prior reconcile instead of reconstructing from observed state. Cue: `if obj.Status.X` driving control flow. Rewrite: reconstruct each run (level-triggered).
- **Plain status write.** `Status().Update` / non-optimistic `MergeFrom` — the concurrent-plan race the profile bans (§2).
- **Swallowing terminal vs transient.** Returning a bare error for a terminal failure (infinite requeue) or marking a transient as terminal. Cue: no `Terminal(err)` discrimination. Rewrite: classify; terminal → `FailedPhase`, transient → requeue/backoff.

## 4. Review cues

- **Dimension 1 (reconcile correctness/idempotency):** plan built from observed state; persist-before-execute; stateless executor; level-triggered (no status-driven branching). *Basis:* profile §1/§4, `sources.md` §reconcile.
- **Dimension 3 (failure handling):** `Terminal(err)` vs transient; bounded backoff; deterministic task IDs for restart-safety; no panic. *Basis:* profile §1, `sources.md` §reconcile.
- **Dimension 5 (observability):** the plan is observable in `.status.plan`; phase/conditions reflect it. *Basis:* profile §3.

## 5. One-way doors in this concern

- **The `.status.plan` / `TaskPlan` shape** is consumed by observers, alerts, and restart-rejoin logic — a change to its structure or the phase-transition semantics is a contract change; flag for human approval.
- **Task-ID determinism** (the UUIDv5 derivation) is a restart-rejoin contract — changing the derivation orphans in-flight tasks on upgrade. Flag it.
