# The method — operating & reviewing K8s-managed Sei validators for governance

Two modes, one spine: **author** (the operator writing per-node SeiNodeTask manifests for a gov submit/vote) and **review** (a lens over existing gov manifests + the on-chain result). Both load the profile + kit first, judge against the platform's actual behavior before generic CRD/gov intuition, and rank idempotency/double-submit + mainnet-adjacency hazards above style. The gates, fan-out template, fee-floor numbers, and mainnet allowlist belong to `/gov-ops` — this method **cites** them.

## The four stages

1. **Load the profile + the kit(s).** `sei-validator-profile.md` (always first — its behavior overrides generic CRD/gov habit) + the kit for the concern: `kit-platform-machinery` (the controller-behavior invariants) or `kit-seinodetask-gov-manifests` (authoring the 3 gov kinds). If working *in* the controller / seictl repo, read its code at HEAD — the live repo wins over this snapshot; flag drift. The seinode-task LLD is loadable for the kind/lifecycle/signing contract **only** (STALE on topology).
2. **Read the whole target.** For review: the SeiNodeTask manifest(s), `spec.target.nodeRef.name`, the typed payload, the `keyName` / operatorKeyring config, the per-node fan-out set, and the **on-chain** state (proposalId / txHash via the `taskID=` memo). For author: the SeiNode set to fan out across (no selector — one manifest per node), the kind's idempotency class, the signing key the ladder resolves. Never reason about a gov submit from a generic CRD mental model — `status.outputs` is empty here, there is no selector, and two of the three kinds are non-idempotent.
3. **Apply the five dimensions** (below), profile-first. Each finding cites a `sources.md` anchor and/or a profile rule. Flag a genuinely-uncertain call rather than forcing it. Where a fact is `/gov-ops`'s (a gate, the fan-out template, the fee floor, the allowlist), cite `/gov-ops` — don't restate it.
4. **Rank + surface.** Idempotency/double-submit hazards (delete-recreate on a non-idempotent submit kind) and mainnet-adjacency lead; encoding traps and on-chain verification next; structure/style is bundled. Refuse a delete-recreate retry on a submit kind; defer the mainnet refuse to `/gov-ops`.

## The five dimensions (the scorecard)

Grounded in the corpus (`sources.md`) and specialized by the profile. Kits map review cues to these — always written as `Dimension N (name)`.

**Dimension 1 (Manifest correctness & encoding).** The `kind` matches its typed payload; fields are **camelCase**; an integer parameter that the chain expects as a JSON **string** is encoded as `"5"` not `5`; a struct param value is a **JSON object**, not a double-encoded escaped string; the fee clears the floor. *Basis:* `sources.md` §controller (CRD payload shapes), §seictl (the `gov_*` handlers); the **fee floor is `/gov-ops`'s** — cite it, don't restate.

**Dimension 2 (Idempotency & recovery).** Reason per-kind: `GovVote` is chain-idempotent (last-write-wins on proposalId/voter) → re-apply safe; `GovSoftwareUpgrade` and `GovParamChange` are **NOT** idempotent (crash-window double-broadcast; both covered by the open `seictl#174`), `GovParamChange` higher-blast-radius. **Re-apply re-joins** the run via the task-ID (UUIDv5); **delete-recreate mints a new run → a duplicate submit + deposit** — refuse it on a submit kind. `requirePhaseTimeout` is **terminal** (a timed-out task does not retry). *Basis:* §controller (CRD idempotency comments, task-ID), §seictl (`gov_*` REHYDRATION WARNING headers, `seictl#174`); profile.

**Dimension 3 (Context & mainnet-adjacency safety).** The kube-context targets the intended `(context, network, namespace)` triple; an arctic-1 task is not aimed at a context co-hosting pacific-1 mainnet. The **enforcement allowlist + co-host refuse is `/gov-ops`'s** — point at it, never restate a weaker prod-pattern rule. *Basis:* profile §safety; `/gov-ops` (the allowlist).

**Dimension 4 (Result verification).** `status.outputs` is **unpopulated for sidecar gov kinds** and `proposalId` is **not** returned — correlate via the `taskID=` memo and read the **chain** (proposalId / txHash), not `status`. Know `includedAt` semantics. *Basis:* §controller (status not populated on the sidecar path); profile §execution.

**Dimension 5 (Citation discipline).** Cite `/gov-ops` / the LLD / controller / seictl / sei-skill at their pins; never restate a fact another skill owns — above all `/gov-ops`'s gates, fan-out template, fee floor, and mainnet allowlist. *Basis:* `sources.md`; SKILL.md Guardrail 2.

## Author discipline (when writing manifests, not just reviewing)

- Argue the **maximum scope you'd defend** (fan the submit/vote across exactly the intended SeiNode set, with the right kind/payload/key), then name what you'd **cut first** and the condition that un-defers it — the orchestrator/human picks the minimum.
- A **submit kind** (`GovSoftwareUpgrade` / `GovParamChange`) is a one-way door: it broadcasts a real tx + deposit. Author it as submit-once; never delete-recreate to retry. Route the gates + fee floor through `/gov-ops`.
- Reach for the platform's *actual* behavior over a generic one: there is **no CRD selector** (one manifest per node — fan-out is yours); `status.outputs` is **empty** (read the chain); the sidecar is reachable at **`:8443`** (the RBACProxyPort), not `7777`.
