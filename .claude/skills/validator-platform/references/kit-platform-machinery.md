# Platform-machinery kit

## 1. What this concern is

The **canonical home for every controller-behavior invariant** an operator must know to drive a gov submit/vote through the K8s validator platform: how a SeiNetwork owns a SeiNode, how a SeiNodeTask reaches the seictl sidecar, which kinds are idempotent, why `status.outputs` is empty, what a re-apply vs a delete-recreate does, how `requirePhase` terminates, and why RPC pinning is closed by construction. The generic CRD mental model gets this wrong here: it assumes `status` carries the result, a selector fans the work out, and a re-create is a safe retry — **none of those hold**. This kit scopes the sidecar to **operator-observable behavior** (the reachable address, what's returned, what a kind does on re-run); for the controller-**author** view (reconcile-correctness, the sidecar handshake internals, CRD durability) cite the **`/kubernetes` profile + `kit-sidecar-task-integration`**, don't re-derive it here. *Cited:* `sources.md` §controller, §seictl, §kubernetes.

## 2. The pattern (how this platform does it)

- **Ownership + no selector.** SeiNetwork owns SeiNode (controller ownerRef, `internal/controller/seinetwork/nodes.go:124`); a SeiNodeTask soft-name-refs **one** SeiNode (`spec.target.nodeRef.name`). **No selector — fan-out is one manifest per node.** *Cited:* §controller; profile §1.
- **Sidecar execution, operator-observable.** A gov SeiNodeTask is a typed HTTP submit to the sidecar at **`:8443`** — the kube-rbac-proxy / `RBACProxyPort` (`noderesource.go:65`), the cross-pod-reachable address. **`7777` is the in-pod plaintext bind *behind* the proxy — not reachable cross-pod.** The sidecar builds the `Msg` in-process on the gov path (`sidecar/tasks/gov_*.go` — no `seid` shell-out), signs, and `BroadcastSync(...)` (`sign_and_broadcast.go:206` call; decl `:120`) over the co-located node RPC, behind `guardChainID` (`sign_and_broadcast.go:316`). The author-view of this handshake is `/kubernetes` `kit-sidecar-task-integration`'s. *Cited:* §seictl `noderesource.go:65`, `sign_and_broadcast.go:206/316`; profile §2/§3; §kubernetes.
- **Idempotency-per-kind.** `GovVote` chain-idempotent → re-apply safe; `GovSoftwareUpgrade` + `GovParamChange` **submit-once** — the pre-broadcast marker (seictl#219) makes a crash-window re-run of the **same** task adopt the prior tx, and covers nothing else; `GovParamChange` higher-blast-radius. *Cited:* §seictl (the pre-broadcast marker, seictl#219); profile §5.
- **Task-ID re-join vs delete-recreate.** Re-applying a task re-joins the existing run via its **task-ID (UUIDv5)** — idempotent at the orchestration layer. **Delete-recreate mints a new run → a duplicate submit + deposit** on a non-idempotent kind. *Cited:* §controller (task-ID); profile §5.
- **`requirePhase` terminality.** `requirePhaseTimeout` is **terminal** — a timed-out task does not auto-retry; re-driving means a deliberate re-apply (same task-ID), never a delete-recreate. *Cited:* §controller (CRD lifecycle comments); profile §5.
- **Status + structural RPC pin.** `status.outputs` is **unpopulated for all sidecar kinds** — read the result from its kind-specific sink: for **gov**, `proposalId` is not returned, correlate via the `taskID=` memo and read the chain; for **shadow `result-export`**, read S3 + Prometheus. One sidecar ↔ one co-located node, so the verify/broadcast TOCTOU is closed **by construction** (no `seid --node` pin needed). *Cited:* §controller (status not populated); profile §6/§7.

## 3. Anti-patterns / failure modes

- **Reading `status.outputs` for the result.** Cue: a flow that polls `status.outputs.proposalId`. Rewrite: read the result from its sink — for gov, the chain (proposalId/txHash via the `taskID=` memo); for shadow `result-export`, S3 + Prometheus — `status` is empty for all sidecar kinds (Dimension 4 (Result verification), profile §6).
- **Delete-recreate to retry a submit.** Cue: `kubectl delete seinodetask … && apply` to re-run a `GovSoftwareUpgrade`/`GovParamChange`. Rewrite: **refuse** — re-apply to re-join the run (same task-ID); a delete-recreate mints a new run → duplicate submit + deposit (Dimension 2 (Idempotency & recovery), profile §5).
- **Targeting `7777` cross-pod.** Cue: a manifest/probe aimed at the sidecar's `7777`. Rewrite: `:8443` (RBACProxyPort) is the reachable address; `7777` is behind the proxy (profile §2).
- **Assuming a selector fans out.** Cue: one task expected to hit N nodes. Rewrite: there is no selector — author one manifest per SeiNode (profile §1).
- **Expecting a timed-out task to retry.** Cue: relying on `requirePhaseTimeout` to re-drive. Rewrite: it is terminal — re-apply deliberately (profile §5).

## 4. Review cues

- **Dimension 2 (Idempotency & recovery):** per-kind class correct; re-apply (task-ID re-join) not delete-recreate on a submit kind; `requirePhaseTimeout` treated as terminal. *Basis:* profile §5, §seictl (`seictl#174`).
- **Dimension 4 (Result verification):** result read from the chain, not `status.outputs`; `taskID=` memo correlation; structural RPC pin understood. *Basis:* profile §6/§7, §controller.
- **Dimension 1 (Manifest correctness & encoding):** `spec.target.nodeRef.name` resolves to a real SeiNode; the sidecar address is `:8443`. *Basis:* profile §1/§2, §controller/§seictl.
- **Dimension 5 (Citation discipline):** the controller-author view (reconcile internals, CRD durability) is cited to `/kubernetes`, not re-derived. *Basis:* §kubernetes.

## 5. One-way doors in this concern

- **A delete-recreate on a non-idempotent submit kind** — refuse; it broadcasts a duplicate proposal + deposit (route a genuine retry through a re-apply / `/gov-ops`).
- **A submit broadcast itself** (`GovSoftwareUpgrade` / `GovParamChange`) — a real on-chain tx + deposit; submit-once, gates owned by `/gov-ops`.
- **A mainnet-adjacent context** (arctic-1 co-hosted with pacific-1) — defer to `/gov-ops`'s allowlist refuse; never assert a weaker rule.
