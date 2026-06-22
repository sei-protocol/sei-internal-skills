# Sidecar task-integration kit

## 1. What this concern is

The controller's primary side-effect channel to a running node is a **per-node seictl sidecar over an HTTP task API** — not annotations, not ConfigMap signaling, not exec-into-pod. A task is submitted, then its completion is learned by **polling**. The generic mental model ("write an annotation / patch a field and let the workload notice") is wrong here: signaling is an explicit HTTP submit/poll contract. *Cited:* `internal/task/task.go:140-147`, `internal/task/sidecar.go`.

## 2. The pattern (how this repo does it)

- **The HTTP contract:** `SubmitTask` / `GetTask` / `Healthz` / `GetNodeID` against the node's sidecar. Params are `Validate()`'d **locally before submit**. *Cited:* `internal/task/task.go:140-147`.
- **Completion by polling:** after submit, `GetTask` is polled; `ErrNotFound` → still Running. **Fire-and-forget** tasks (`mark-ready`, `config-validate`) complete on the submit ack itself. *Cited:* `internal/task/sidecar.go:40-117` (the submit/poll/fire-and-forget mechanics); the 5s `TaskPollInterval` + bounded backoff live in `internal/planner/executor.go:21` (applied at :165/:178/:202), not in `sidecar.go`.
- **Restart-safe submits:** deterministic UUIDv5 task IDs (`task/task.go:132-138`) let a restarted controller rejoin an in-flight task instead of double-submitting.
- **The task-type registry is split** sidecar-backed vs controller-side (StatefulSet/Service/PVC/Job via SSA). A reviewer must know which side a task lives on. *Cited:* `internal/task/task.go:201-243`.
- **Health drives re-approval:** a separate 2s `Healthz` probe sets `SidecarReady`; only `False/NotReady` (503) triggers a `mark-ready` re-approval plan. *Cited:* `internal/planner/sidecar_probe.go:17-59`.
- **Deployment ordering hazard:** the controller must tolerate the sidecar not yet being reachable (it polls `Healthz`); a deploy that brings the controller up assuming the sidecar is present will churn. (Cross-links `/platform` for the deployment ordering.)

## 3. Anti-patterns / failure modes

- **Annotation/ConfigMap signaling.** Reaching for a CR annotation or a ConfigMap write to tell the node to do something. Cue: a `client.Patch` of an annotation as a side-effect channel. Rewrite: submit an HTTP task.
- **Submit-as-completion for a polled task.** Treating a `SubmitTask` ack as "done" for a task that is actually long-running. Cue: no `GetTask` poll loop for a non-fire-and-forget task. Rewrite: poll `GetTask`; `ErrNotFound` = still Running.
- **Non-deterministic task IDs.** Random/timestamp IDs → a restart double-submits. Cue: `uuid.New()` instead of the UUIDv5 derivation. Rewrite: derive deterministically from the stable inputs.
- **Skipping local `Validate()`.** Submitting params the sidecar will reject, turning a local error into a remote round-trip + opaque failure. Cue: submit without `params.Validate()`. Rewrite: validate before submit.
- **Treating sidecar-unreachable as terminal.** A 503/unreachable sidecar should drive the `SidecarReady=False` re-approval path, not fail the plan. Cue: `Terminal(err)` on a health/connection error. Rewrite: classify it transient — drive the `SidecarReady=False` re-approval and requeue; don't fail the plan.

## 4. Review cues

- **Dimension 1/3:** submit→poll contract honored; fire-and-forget vs polled correctly classified; deterministic IDs; unreachable-sidecar → re-approval not terminal. *Basis:* the cited call sites; profile §1.
- **Dimension 5:** `SidecarReady` condition reflects the `Healthz` probe (reason-as-API). *Basis:* profile §3.

## 5. One-way doors in this concern

- **The seictl HTTP task API contract** (endpoint shapes, task-type names, param schemas) is a cross-binary contract with the sidecar (an external dependency). A change must be coordinated with the sidecar; flag for human approval — the controller cannot change it unilaterally.
- **The fire-and-forget vs polled classification** of a task type is observable behavior; changing it changes completion semantics.
