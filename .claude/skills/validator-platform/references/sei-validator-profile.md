# Sei-validator profile (always-first overlay)

Loaded before any author or review. It encodes the **K8s-managed Sei validator platform's own behavior** and **overrides generic CRD/gov intuition** — the way `/idiomatic`'s repo profile outranks generic idiom. Distilled (and cited) from the controller @ **`sei-k8s-controller@5730aa4`** + the **seictl** sweep (`seictl@79f74a5`). Signing/idempotency/lifecycle facts also cite the `seinode-task` LLD for the **contract only**.

**Snapshot caveat:** a portable distillation for authoring and review. When you are working *inside* the controller / seictl repo, its live code at HEAD is the authority — read it and flag drift, don't follow a stale copy. **The seinode-task LLD/reference is STALE on topology** (predates #417/#442: still describes `SeiNodeDeployment` + Chaos-Mesh-Workflow fan-out) — cite it for kind/lifecycle/signing/idempotency, **never** for topology.

## The platform in one paragraph

A gov proposal or per-node vote is a **typed `SeiNodeTask`** the controller hands to the node's **seictl sidecar**, which builds the Cosmos SDK `Msg` in-process, signs with a keyring entry, and broadcasts over the co-located node RPC behind a chain-ID guard. There is **no fan-out selector** — one manifest per SeiNode, fan-out is the caller's job. `status.outputs` is **unpopulated** for these kinds — the result lives on the chain. Two of the three gov kinds are **non-idempotent**, so the cardinal rule is **submit-once, never delete-recreate**. The gates, the GovVote fan-out template, the fee-floor numbers, and the mainnet-adjacency allowlist are **`/gov-ops`'s** — this profile points at them, it does not restate them.

## Hard behavior (these override generic habit)

1. **Topology — owned chain, no selector.** SeiNetwork **owns** SeiNode (controller ownerRef, `internal/controller/seinetwork/nodes.go:124`); a SeiNodeTask is a **soft name-ref** to exactly one SeiNode (`spec.target.nodeRef.name`); `SeiNodeDeployment` was **removed** (#417). **There is no selector — fan-out is the caller's job: one manifest per node.** *Cited:* controller@5730aa4 `internal/controller/seinetwork/nodes.go:124`, `spec.target.nodeRef.name`. **(Generic habit assumes a selector fans the work out; here you author N manifests. NEVER take topology from the stale LLD.)**

2. **Execution — typed HTTP submit to the sidecar at `:8443`.** A gov SeiNodeTask is a typed HTTP submit to the node's seictl sidecar at **`:8443`** — the kube-rbac-proxy / `RBACProxyPort` (`noderesource.go:65`), the **cross-pod-reachable** address. `7777` is the sidecar's in-pod plaintext bind *behind* the proxy — **not reachable cross-pod**. The sidecar builds the Cosmos SDK `Msg` **in-process on the gov path** (`sidecar/tasks/{gov_vote,gov_software_upgrade,gov_param_change}.go` — **no `seid` CLI shell-out** for gov kinds), signs with a keyring entry, and broadcasts **sync** (`sign_and_broadcast.go:206` calls `BroadcastSync(...)`, declared `:120`) over the co-located node RPC. *Cited:* seictl@79f74a5 `noderesource.go:65`, `sidecar/tasks/gov_*.go`, `sign_and_broadcast.go:118`; controller@5730aa4.

3. **Chain-ID confusion guard (in seictl).** Broadcast is behind `guardChainID` (`sign_and_broadcast.go:316`): the `chainId` must equal the **`SEI_CHAIN_ID` env** *and* the node's `/status` `node_info.network`. *Cited:* seictl@79f74a5 `sign_and_broadcast.go:316`.

4. **Signing topology — the keyName ladder (cite controller, NOT the stale LLD).** `keyName` resolution ladder: **explicit** → **`node_admin`** (if the operatorKeyring secret is set) → the **`validator` gentx key** (if unset). Implemented in `internal/task/seinodetask_params.go:231` (`resolveSigningUID`), documented in the CRD comments (`api/v1alpha1/seinodetask_types.go:227-231,286-292,355-362`), with `DefaultOperatorKeyName="node_admin"` (`api/v1alpha1/validator_types.go:7`). **Footgun:** a gentx-bootstrapped validator that *assumes* `node_admin` signs with the wrong key. *Cited:* controller@5730aa4 (the live contract — **NOT** the stale LLD).

5. **Idempotency-per-kind.** *Cited:* CRD comments + seictl `gov_*.go` handler headers.
   - **`GovVote`** — chain-idempotent (last-write-wins on proposalId/voter) → **re-apply safe**.
   - **`GovSoftwareUpgrade`** — **NOT** idempotent (crash-window double-broadcast).
   - **`GovParamChange`** — **NOT** idempotent, and the **higher-blast-radius** case (no "applies once" net — a duplicate is two real proposals + two deposits).
   - **Both submit kinds are covered by the open `seictl#174`** — the cross-handler pre-broadcast txHash marker (the `REHYDRATION WARNING` header is identical in `gov_software_upgrade.go:8` and `gov_param_change.go:7`). The fix is **open**, so until it lands the rule below holds for both.
   - **Rule:** **submit-once; never delete-recreate to retry a submit.** Re-applying re-joins the existing run via the task-ID (UUIDv5); deleting + recreating mints a *new* run → a duplicate submit.

6. **Result verification — read the chain, not status.** `status.outputs` is **unpopulated for sidecar gov kinds**, and `proposalId` is **not** returned — correlate via the **`taskID=` memo** the sidecar appends. Read the **chain** (proposalId / txHash), not `status`.
   - **`includedAt` semantics.** The broadcast is **`BroadcastSync`** — it returns on **CheckTx acceptance** (the tx entered the mempool), **NOT** on block inclusion. `includedAt` (`sign_and_broadcast.go:83`) is a **separate inclusion poll** the sidecar runs after broadcast; it is **`nil` when that poll deadline elapses without inclusion** (a non-Terminal success — the tx may still land later). So a `nil`/undetermined `includedAt` is not a failure; confirm on the chain.
   - **Verify recipe:** take the **txHash** (from sidecar logs / the `taskID=` memo correlation) → `seid query tx <txHash>` to confirm inclusion + parse the `proposalId` from the proposal-submit events → `seid query gov proposal <proposalId>` for status/tally (§sei-skill vocabulary). For a vote, query the proposal's votes for the voter. **Re-pin note:** if a poll deadline elapsed with `includedAt` nil, re-query the chain by txHash before assuming failure — never re-apply a submit kind to "force" inclusion.
   *Cited:* controller@5730aa4 (status not populated on the sidecar path); seictl@79f74a5 (`BroadcastSync` + `includedAt` poll, `sign_and_broadcast.go`); §sei-skill (`seid query` vocabulary).

7. **RPC pinning is structural.** One sidecar ↔ one co-located node, so the verify/broadcast TOCTOU that `/gov-ops` guards via `seid --node` pinning is closed *by construction* here — name it so the operator knows why. *Cited:* the execution model (§2) + `/gov-ops`.

8. **Safety — mainnet-adjacency defers to /gov-ops.** arctic-1 (testnet) vs pacific-1 (mainnet): this profile **points at `/gov-ops`'s allowlist + co-host refuse**, it does **not** restate a weaker rule. *Cited:* `/gov-ops` (the allowlist — owner of the gate).

## The owner-per-fact boundary

- **This skill owns (net-new):** the operator's read of the K8s validator platform — the topology, the sidecar execution model, and the controller-behavior invariants needed to submit safely (idempotency-per-kind, status-outputs-unpopulated, keyring ladder, `requirePhase` terminality + task-ID re-join, structural RPC pin).
- **Cites, never restates:** **`/gov-ops`** for the gates + GovVote fan-out template + fee-floor numbers + mainnet-adjacency allowlist (shipped, anchor-coupled, **left unchanged**); the seinode-task LLD for the kind/signing/idempotency *contract*; **`/kubernetes`** (profile + `kit-sidecar-task-integration`) for the controller-**author** view; public `sei-skill` for dev-facing `seid`/gov/validator primitives; the controller @ pin for topology.
- **NOT:** orchestration (`/gov-ops`), controller/CRD dev (`/kubernetes`), platform GitOps infra (`/platform`).
