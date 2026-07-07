---
name: gov-ops
category: release-operations
model: claude-opus-4-8
description: "Orchestrates a Sei governance proposal lifecycle — submit → confirm → vote → verify — on a target chain, GitOps-native, with fail-closed safety gates. Trigger on 'submit a governance proposal', 'run a param-change', 'vote on proposal N across the validators', 'orchestrate the TimeoutParams change', '/gov-ops'. REFUSES on a kube context that hosts a non-target chain (mainnet-adjacency). NOT for chain spin-up (use /harbor-dev); NOT for release validation or chaos (use /validate-release, /chaos-suite); NOT for deciding WHAT to change (the operator/PM authors the proposal); NOT for software-upgrade/text/community-pool proposals (param-change only for now). One proposal lifecycle per invocation."
---

# Gov-Ops — Governance Proposal Orchestration

Drive one Sei governance proposal end-to-end on a target chain: submit it, confirm it landed in voting with the intended content, fan validator votes via GitOps, verify it passed and applied, and surface rollback. Built so an **agent** can run it against a **prod-hosted** chain without re-learning the footguns that bit the arctic-1 `TimeoutParams` op (platform #995). Operational facts (fee floor, value encoding, voting window) are **cited** from the SeiNodeTask reference, not restated — `sei-protocol/bdchatham-designs designs/seinode-task/seinode-task.md` §`Reconciliation cadence & gotchas`.

**One invocation = one proposal lifecycle.** Param-change only.

## Guardrails

This skill writes to **one target chain in one allowlisted `(kube-context, network, namespace)` triple**. Every safety property below is a **hard, fail-closed gate** — not a preference. Before and at **every** side-effecting step:

1. **Positive allowlist (not a prod-pattern refuse).** The target chain may legitimately live in a `prod-*` cluster, so this skill does **not** refuse on a `prod` pattern — it requires the live `(context, network, namespace)` to match an explicit allowlist entry (`references/guardrails.md`). **It refuses any context that co-hosts a non-target chain** — e.g. refuse `prod`/eu-central-1 for `arctic-1` because it co-hosts `pacific-1` (mainnet) + `atlantic-2`; allow `prod-use2`/`prod-euw1`. Re-assert the triple before **each** side-effecting step (context can drift mid-run).
2. **Pinned RPC endpoint.** The endpoint verified in pre-flight (`seid status` → expected `network`, `catching_up=false`) is the **only** endpoint used for every subsequent tx — no `--node` override, no re-resolution (closes the verify-node-A / broadcast-node-B TOCTOU).
3. **Verbatim `confirm` before each irreversible act** — (a) the proposal broadcast (the deposit spend is irreversible on every path — see step 2 for what idempotency does and does not cover), (b) the vote-merge to `main` (Flux auto-reconciles onto the validators). Echo chain / network / namespace / proposalId / value-shape / deposit / fee / validator-count. Any response other than `confirm` aborts.
4. **Refusal conditions** — refuse to start if: the live triple isn't on the allowlist; the context co-hosts a non-target chain; `seid status` network ≠ expected or `catching_up=true`; a prior unfinished run exists in `state/`.

The fast-path (imperative `kubectl`/`flux suspend` instead of GitOps) is **GitOps-by-default, authorization-gated**: it runs only with explicit authorization — a **named human + a verbatim token echoing the exact action + context + command + an append-only `state/` audit entry**. Anything less → refuse and stay GitOps. See `references/guardrails.md`.

## Procedure

Steps are tagged **P0** (gates + fan-out, the core), **P1** (thin), **P2** (referenced runbook).

1. **Pre-flight — P0.** Resolve and assert the allowlist triple (Guardrail 1); pin the RPC endpoint (Guardrail 2); quorum-reachability check (controlled bonded power / total bonded power ≥ `quorum` gov param — a one-line arithmetic gate, not a stake model). Snapshot the **current** live value of each target `subspace/key` (the byte-exact source for the rollback artifact).
2. **Submit — P1, gates-only.** Two paths. The **`GovParamChange` SeiNodeTask** is the default: the sidecar signs, writes a durable pre-broadcast marker, then broadcasts — so re-running the **same task** after a crash adopts the prior tx (already on-chain → build from it; not → re-broadcast the identical bytes; never re-sign). The marker covers only that task: **a fresh task manifest is a fresh broadcast**, and the CLI path (out-of-band funded **non-validator** key) is non-idempotent outright — the duplicate-submit guard below is load-bearing on **both** paths. Gates, all fail-closed:
   - **Value-shape (blocking):** `seid tx ... --generate-only`; assert the rendered `.body.messages[0].content.changes[0].value` is a **JSON value of the param's type, not a quoted/escaped string**, and shape-matches the on-chain value. Block broadcast until it passes (cite the double-encode gotcha).
   - **Deposit (blocking):** `initial_deposit >= min_deposit` (else it sits in deposit-period and votes are wasted).
   - **Duplicate-submit guard:** before submitting, detect whether a prior submit already landed (retry-prone context); do not create a second proposal.
   - **Confirm** (Guardrail 3a), then broadcast. On the task path, read `proposal_id` from the task's structured result (`wire.GovTxResult`, surfaced on the SeiNodeTask status) — the sidecar parses both event shapes for you. On the CLI path, resolve `proposalId` from `submit_proposal` events under `.logs[].events[]` (attr `proposal_id`); **never "latest proposal"**. Fall back to top-level `resp.Events` if `.logs` is empty on a newer build.
3. **Confirm gate — P0 (highest value).** Fetch the proposal by the **resolved** id; assert content (title, `subspace`/`key`/`value`, `is_expedited`) matches intent, AND `status == PROPOSAL_STATUS_VOTING_PERIOD`, AND capture the **effective window** (the expedited period when `is_expedited`, else `voting_period`). This prevents fanning votes at the wrong proposalId.
4. **Vote fan-out — P0.** Generate `GovVote` SeiNodeTasks from the **live validator list per cluster**, each `nodeRef` re-validated to exist in the target namespace (reuse the allowlisted triples + pinned endpoint). **Fee gate (blocking):** each vote's `fees >= gas × chain-enforced-min-gas-price` (cite the fee-floor gotcha — arctic-1 `0.02usei/gas`, not app.toml's `0.01`). **Assert fanned `proposalId` == the resolved submit id.** Deliver via **GitOps** (merge → `flux reconcile`); **confirm** before the merge (Guardrail 3b). Intra-account retry is per-account-sequenced (code-32); cross-account fan-out has no nonce contention.
5. **Active failure detector — P0.** After broadcast and after fan-out, poll for **CheckTx code-13 (under-fee)** and **tally-not-moving** → **HALT loudly** — in addition to, never instead of, reading task status. Task status is authoritative on completion: a committed tx whose DeliverTx code ≠ 0 reports **`Failed` (terminal)**; inclusion-undetermined stays **pending** — the single observable retry signal (the controller re-submits the same task; the marker makes that safe); timeout-while-pending lands `Failed` with `InclusionUndetermined`. A tx that never committed never reports `Completed`.
6. **Verify — P0.** Poll tally → `PASSED`; then **independently assert the param applied** (query the `subspace/key`, byte-exact to the submitted value — `PASSED` ≠ applied). For consensus-affecting changes, check **chain health post-apply** (block production, no liveness regression). On wrong-content / liveness-regression → trigger the pre-staged revert (step 1 snapshot), window permitting; if the window has closed, HALT loudly (don't silently skip).
7. **Voting-window awareness — P1.** Compare the effective window (step 3) against **worst-case fan-out completion** + the tally poll. If too short, surface the trade-off and offer (a) a bootstrap `gov/votingparams` extension op, or (b) the authorization-gated fast-path. Never auto-suspend Flux / auto-`kubectl` to shared prod.

**Rollback — P2.** See `references/rollback.md`. The skill executes only "submit the pre-staged revert proposal" (re-entry into steps 2–6); the revert faces a **full voting cycle** (not fast). A **chain-halted** consensus-param change is unrecoverable by governance — operator-side timeout/config override + coordinated restart, **never scale validator replicas** (double-sign). The byte-exact revert proposal (step 1 snapshot) is staged before the change applies.

## Credential handling

Submit with a **non-validator funded key** (the operator `node_admin` key stays sidecar-sealed). `--generate-only` for review; sign via the keyring, **never an inline mnemonic on the command line**. Generate-only / signed-tx artifacts go to a **gitignored `state/` path and are shredded post-run** (a signed tx is a broadcastable bearer artifact). **No mnemonic / key material in logs, audit, or chat.**

## Halt Conditions

Stop and report (do not proceed) if any of:

- the live `(context, network, namespace)` triple isn't on the allowlist, or the context co-hosts a non-target chain (Guardrail 1);
- `seid status` network ≠ expected, or `catching_up == true`;
- any blocking gate fails — value-shape, `deposit < min_deposit`, `fees < gas × min-gas-price`, or fanned `proposalId` ≠ resolved id (`references/guardrails.md` gate table);
- the on-chain proposal content/id doesn't match intent, or `status != VOTING_PERIOD` (step 3);
- **CheckTx code-13 or a non-moving tally** after broadcast/fan-out (step 5) — HALT loudly; do not let the sidecar's silent retry stand;
- the effective voting window is shorter than the pipeline can serve and no fast-path authorization is given (step 7);
- a prior unfinished run exists in `state/` (see State Management).

On any halt: leave the run dir intact, surface the failing gate, await operator direction. **Never** auto-suspend Flux or imperative-apply to recover.

## State Management

Each run writes to `state/run-<ISO-8601-timestamp>/`: the resolved triple + pinned endpoint, the `--generate-only` artifact, the resolved `proposalId`, the **pre-flight param snapshot** (the revert source), and the fast-path `audit.log` (if used). `state/` is gitignored — artifacts (including any signed tx) are never committed and are shredded post-run.

**Interrupted runs — refuse, don't auto-resume.** Given the prod/mainnet adjacency, an unfinished run in `state/` is a **refuse-to-start** condition. The operator inspects the run dir, confirms on-chain state (was the proposal submitted? did votes land?), then explicitly archives the dir before a fresh run. Re-running the **same** interrupted task adopts its prior tx; a **fresh task manifest is a fresh broadcast**, and a CLI submit can always double-broadcast — the inspection is what prevents paying a second deposit, and half-fanned votes need human eyes regardless.

> **Intentionally prose-driven, not scripted** (no `scripts/`): the value is live sequencing / timing / safety decisions across a short voting window, not a deterministic runbook. Read-path commands (`seid q …`, `seid status`, `kubectl get`, `flux reconcile`) are happy-path; **write/destructive commands (`kubectl delete`, `flux suspend`, `kubectl apply` to shared prod) are never pre-approved** — they run only via the authorization-gated fast-path.

## Preconditions

- `seictl`/`seid` reachable for the target chain; `flux` CLI; the GitOps repo checked out at `main`.
- The proposal `.json` (operator-authored content) is an **input** — this skill does not decide *what* to change.
- An allowlist entry for the target `(context, network, namespace)` in `references/guardrails.md`.

## Acceptance (self-test before relying on it)

See `evals/evals.json`. Minimum: a **dry-run E2E on a dev chain** (NOT a prod-hosted chain) reaching a tally read with no live gov tx, and an **injected fail-closed** proof per gate (mainnet-co-hosting context refused; escaped-string value blocked; under-floor fee rejected; short window halts for authorization; resolved-id ≠ fanned-id blocks). A real `PASSED` on a prod-hosted chain is an operation, not a test.

## References

- `references/guardrails.md` — the allowlist, the gate definitions, the authorization mechanism.
- `references/fan-out.md` — the `GovVote` manifest template + per-cluster generation.
- `references/rollback.md` — the rollback + chain-halted runbook.
- Operational facts (cited, not restated): `sei-protocol/bdchatham-designs designs/seinode-task/seinode-task.md` (`#reconciliation-cadence--gotchas`, `#signing-topology`, `#statusoutputs`). Consumed by the `platform-release-manager` agent.
- The gov completion contract + submit idempotency marker: `sei-protocol/bdchatham-designs#98` (gov UX design), shipped via seictl#219 (sidecar) and sei-k8s-controller#453 (`wire.GovTxResult` consumer).
