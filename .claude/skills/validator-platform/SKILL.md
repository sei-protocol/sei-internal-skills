---
name: validator-platform
category: release-operations
model: claude-opus-4-8
description: "Use when operating K8s-managed Sei validators to submit governance proposals/per-node votes or drive a shadow result-export comparison safely — the machinery behind /gov-ops + the sidecar shadow tasks: '/validator-platform', 'is this SeiNodeTask manifest right', 'why is status.outputs empty', 'can this task be delete-recreated to retry', 'which keyring key signs this', 'is this gov kind idempotent', 'how do I fan a vote across the validators', 'how do I run a shadow result-export comparison'. A citable corpus (controller@5730aa4, seictl@79f74a5, seinode-task LLD, /gov-ops, public sei-skill) + an always-first Sei-validator profile (SeiNetwork→SeiNode→SeiNodeTask topology, seictl-sidecar execution at :8443, idempotency-per-kind, keyring ladder, requirePhase terminality, structural RPC pin) + pluggable kits. Backs the platform-release-manager agent. NOT orchestration/gates/fan-out template/fee floor (cite /gov-ops); NOT controller/CRD code (/kubernetes); NOT platform GitOps infra (/platform)."
---

# Validator platform

Operate the **K8s-managed Sei validator platform** — the SeiNetwork→SeiNode→SeiNodeTask topology, the seictl-sidecar execution model, and the controller-behavior invariants an operator must know to drive governance proposals and per-node votes through it **safely**. A *reference/technique* skill with a discipline spine. It is the operator's manual for the `platform-release-manager` agent's governance work and is directly invocable (`/validator-platform <target>`).

## Why this skill exists

A capable model knows generic Kubernetes + Cosmos governance. The skill's job is the **citable corpus** (the controller / seictl / LLD pins) plus the **always-first Sei-validator profile** — the platform's real, non-obvious behavior that *overrides* generic habit. The failure mode it prevents: treating the validator fleet like a textbook CRD-driven system — assuming every kind carries its result on `status.outputs` (**gov kinds do** — decoded `wire.GovTxResult` with `txHash`/`proposalId`; the other sidecar kinds don't — shadow reads S3 + Prometheus), reaching for **delete-recreate to retry** a submit (mints a new task-ID run → a duplicate proposal + deposit), assuming a CRD selector fans a vote out (there is **none** — one manifest per node), or assuming `node_admin` signs (it depends on the keyring ladder).

It is the **consumer** of the controller contract `/kubernetes` authors, not a second copy of it. The corpus cites pins; it never restates a fact another skill owns — above all **`/gov-ops`**, whose gates, GovVote fan-out template, fee-floor numbers, and mainnet-adjacency allowlist this skill **cites, never restates**.

## Guardrails

Refusal conditions — they hold under time pressure and a "just submit the task" urge:

1. **Profile- and kit-first.** Load `references/sei-validator-profile.md` (the always-first overlay — it encodes the platform's hard behavior and **overrides generic CRD/gov habit**) **and** the relevant kit before authoring or reviewing. When working *in* the controller repo, read its live code at HEAD — the live repo wins over this skill's snapshot; flag drift.
2. **Cite, don't restate /gov-ops gates.** `/gov-ops` owns the fail-closed gates, the **GovVote fan-out template**, the **fee-floor numbers**, and the **mainnet-adjacency allowlist** (a shipped, anchor-coupled skill — **left unchanged**). Point at it; never restate a weaker version of a rule it owns. The seinode-task LLD is cited for the kind/lifecycle/idempotency contract **only** — it is **STALE on topology and signing-topology**; take topology and the keyring/signing ladder from §controller (the live pin), never from the LLD.
3. **Submit-once; never delete-recreate to retry a submit.** Re-applying a task re-joins the existing run via its task-ID (UUIDv5); deleting + recreating mints a *new* run → a duplicate broadcast. `GovSoftwareUpgrade` and `GovParamChange` write a **durable pre-broadcast marker** (shipped via seictl#219, closing `seictl#174`), so a crash-window re-run of the **same** task adopts the prior tx — the marker covers only that task; a fresh run is a fresh broadcast + deposit, and `GovParamChange` is the higher-blast-radius case. Refuse delete-recreate on a submit kind.
4. **Read each result from where the platform puts it.** **Gov** kinds populate `status.outputs` — the controller decodes the sidecar's `wire.GovTxResult` into `status.outputs.gov{ParamChange,SoftwareUpgrade,Vote}` (`txHash`/`height`, plus `proposalId` on the submit kinds once committed; zero while pending) — read gov results there; the `taskID=` memo + chain query (`seid q tx <hash>` → `code`/`raw_log`) is the fallback for a **pending / zero-`proposalId`, or `Failed`/`Timeout`-with-empty-outputs** case — the last masks the real DeliverTx error behind a bare `Timeout` (seictl#236), so the chain, not `status`, is authoritative on *why* a submit failed. The **other sidecar kinds do not populate `status.outputs`** — **shadow `result-export`** reads the S3 comparison artifacts (`.compare.ndjson.gz` / divergence reports) + Prometheus metrics.
5. **Mainnet-adjacency defers to /gov-ops.** The arctic-1-vs-pacific-1 co-host refuse is `/gov-ops`'s allowlist — point at it, don't restate a weaker prod-pattern rule.

## The method

`references/method.md` holds the full method; the spine:

1. **Load the profile + the kit(s)** for the concern (controller machinery, gov-manifest authoring). Read the controller at HEAD if working in it.
2. **Read the whole target** — the SeiNodeTask manifest(s), the `spec.target.nodeRef`, the payload, the keyring config, the on-chain state — before judging. Never reason about a gov submit from a generic CRD mental model.
3. **Apply the five dimensions** (below), profile-first. Each finding cites a `sources.md` anchor and/or a profile rule.
4. **Rank + surface.** Idempotency/double-submit and mainnet-adjacency hazards lead; encoding traps and verification next; style is bundled. Defer the gates/fan-out/fee-floor/allowlist to `/gov-ops`.

## The five dimensions (the scorecard)

`references/method.md` enumerates them; kits map review cues to these.

1. **Manifest correctness & encoding** — kind↔payload match, camelCase, integer-as-JSON-string, param-struct-as-JSON-object (double-encode), fee floor (cite `/gov-ops`).
2. **Idempotency & recovery** — per-kind idempotency; re-apply (task-ID re-join) vs delete-recreate (duplicate submit); `requirePhaseTimeout` terminal.
3. **Context & mainnet-adjacency safety** — kube-context targeting; defer the enforcement allowlist to `/gov-ops`.
4. **Result verification** — gov kinds read from `status.outputs.gov*` (decoded `wire.GovTxResult`: txHash/height, proposalId on committed submits; memo+chain fallback while pending). On a submit that lands `Failed`/`Timeout` with empty outputs / zero proposalId, resolve the real cause on-chain — `seid q tx <hash>` (hash from the `taskID=` memo or the sidecar `proposal broadcast` log) → `code`/`raw_log` (e.g. `code 5` unaffordable deposit, `code 13` under-fee; the pre-submit gate that should have blocked it is `/gov-ops`'s, and a committed code≠0 tx retried-to-timeout is seictl#236). The other sidecar kinds read their sink (shadow → S3 `.compare.ndjson.gz`/divergence reports + Prometheus).
5. **Citation discipline** — cites `/gov-ops` / LLD / controller / seictl / sei-skill at pins; never restates a fact another skill owns.

## Kit index

| Concern | Kit |
|---|---|
| Controller-behavior invariants — ownership, sidecar execution at `:8443`, idempotency-per-kind, per-kind result location, task-ID re-join, `requirePhase` terminality, structural RPC pin | `references/kit-platform-machinery.md` |
| Authoring + GitOps-applying the 3 gov kinds as per-node manifests — payload encoding traps, keyring ladder, per-node fan-out (cite `/gov-ops`), poll/verify-on-chain | `references/kit-seinodetask-gov-manifests.md` |
| Driving a shadow `result-export` (comparison-mode) task against a node already running the supported shadow features, then reading results — the typed-client gap (raw params map for the EVM/migration params), the L0/L1/L2 + watermark model, `migrationMode`, S3 + Prom outputs | `references/kit-shadow-comparison.md` |
| `kit-gitops-networking` — SeiNetwork/SeiNode + manual-NLB/HTTPRoute model | *(deferred — un-defer at M2 (`/harbor-dev`); see `references/kit-TEMPLATE.md` roster)* |

## How platform-release-manager + /gov-ops hook in

- `platform-release-manager` gains a **"Backed by `/validator-platform`"** line (alongside `/validate-release` + `/gov-ops`). Its governance mode loads `sei-validator-profile.md` + the kit for the K8s/sidecar machinery, then drives the submit/vote through `/gov-ops` (which owns the gates).
- `/gov-ops` keeps its existing seinode-task **anchor citations as-is** — don't re-point a shipped, anchor-coupled skill through this one. `validator-platform` is **additive** K8s/fleet/operator knowledge `/gov-ops` and the agent can cite; it does not re-own `/gov-ops`'s content.

## Halt conditions

- **No target** to author/review — ask for the manifest / nodeRef / on-chain state; never reason about a gov submit from memory.
- **A delete-recreate to retry a submit** — refuse (Guardrail 3); re-apply to re-join the run, or accept the duplicate hazard is real and route to `/gov-ops`.
- **A mainnet-adjacent context** (arctic-1 co-hosted with pacific-1) — defer to `/gov-ops`'s allowlist refuse (Guardrail 5); don't assert a weaker rule.
- **The work is really another lens** — orchestration/gates (`/gov-ops`), controller code (`/kubernetes`), platform GitOps infra (`/platform`) — redirect.

## What this skill defers

`kit-gitops-networking` (the SeiNetwork/SeiNode + manual-NLB/HTTPRoute model) is deferred — **un-defer at M2 (`/harbor-dev`)**; see `references/kit-TEMPLATE.md`'s roster. The Sei-validator profile is a *snapshot* of the controller @ `5730aa4` + seictl @ `79f74a5` — when working in the live repos, their HEAD is authoritative; flag drift. `/gov-ops`'s gates/fan-out/fee-floor/allowlist are never restated here — cited.
