# The review-gate (a consensus gate that composes `/cross-review`)

The third gate kind, alongside the human `checkpoint` and the signal `guard`. A **review-gate**
makes *"merge once the reviewer slate is unanimously RATIFY (zero open concerns) and the declared
automated checks pass"* a **declarable, enforceable** primitive — the gate operators reach for when
they delegate `pr-sign-off` to expert-consensus + CI.

It is the machine counterpart of `pr-sign-off`, the way a `guard` is the machine counterpart of a
human-watched cutover: it discharges the *routine* merge the operator pre-authorized; the human
still owns every one-way door.

## Compose, never reimplement — the one-direction coupling

`/cross-review` (PLT-535 / Design 08) is the **provider**: it owns the slate, the routing, the
blinded dispatch, the assigned dissent, and the **review-ledger** schema + gate-read contract. The
review-gate is the **consumer**: it *invokes* the slate (the verify-to-convergence loop) and
*reads* the review-ledger (the gate evaluation). 

There is **one** coupling surface — `/cross-review`'s **gate-read contract** (Design 08, *How it
composes*). The review-gate reads exactly the latest round's header fields that contract names, and
nothing else. Per Design 08's stated tie-break, **that contract is canonical**; the review-gate
adapts to it and never re-derives review state. If the contract changes, the review-gate follows
it — there are not two contracts.

> **Never reimplement** the slate, the routing, the steward-wiring, or the review-ledger schema
> here. If you find yourself defining how a round is dispatched or what the ledger header looks
> like, stop — that is `/cross-review`'s, and duplicating it forks the contract.

## The ledger entry

A third entry kind in the workstream's checkpoint ledger, declared up front:

```
- review-gate: <short identifier, kebab-case>
  slate:    the FULL standards-champion cohort — /cross-review resolves it (provider) so every codified standard is championed-or-recorded-N/A; see "The standards-coverage requirement"
  checks:   <the declared automated checks that must be green — e.g. cursor-bugbot, named CI workflows>
  ledger:   <the /cross-review review-ledger path — target-derivable per PLT-535; the gate computes it from the target, no registry>
  satisfied_when: the review-ledger's latest round reads a PASSING TERMINAL per /cross-review's gate-read contract (currently State RESOLVED|RESOLVED-WITH-ACCEPTED-RISK, OpenFindings 0, Dissenter non-empty, cross-field consistent) AND Convergence is unanimous (the consensus refinement — see gate evaluation) AND the round's slate is the full standards-champion cohort (the standards-coverage requirement) AND every declared check has passed
  on_fail:  surface + route to a PRE-DECLARED human checkpoint (e.g. pr-sign-off) — never self-merge on a fail
```

## The standards-coverage requirement (the slate is the full champion cohort)

A review-gate discharges a *merge*, and a merge must clear **every** standard the org maintains —
not only the domain boundaries the change obviously touches. So a review-gate's slate is **not** the
"smallest set that covers the boundaries" that `/cross-review` picks for a lightweight consult: it is
the **full standards-champion cohort**. Every codified standard is either **championed by a reviewer
on the slate** or **explicitly recorded N/A with a reason** — never silently absent.

The standards a champion is convened for (this skill names the *standards*; `/cross-review` — the
slate provider — resolves each to the reviewer in this repo's `.claude/agents` roster):

- **Domain correctness + interface boundaries** — the specialists whose components the change touches.
- **Idiom** + the comment-register / no-tombstone bar.
- **Systems quality** — reliability, performance, failure-modes-by-design, observability-by-design, API durability.
- **Dual-audience prose** — the artifact reads correctly for the human reviewer *and* the consuming agent.
- **Security** — adversarial / confused-deputy / boundary review.
- **SRE** — SLO / alerting / runbook soundness, where the change carries an operational signal.
- **Observability** — telemetry-backend / query correctness, where the change emits or reads telemetry.
- **Capacity** — scheduling / right-sizing, where the change is workload-affecting.
- **Lineage** — the bet↔design↔issue↔PR graph stays intact.
- **Scope / YAGNI** — for design artifacts.

This is the review-gate's own merge policy layered on `/cross-review`'s contract — exactly like the
unanimous-convergence refinement, and bounded the same way. `/cross-review` still **owns** the slate
(routing, blinded dispatch, the assigned dissenter, the ledger); the review-gate does **not**
reimplement any of it. It requires one property of the slate `/cross-review` convenes — **full
standards coverage** — and then reads the result. A standard recorded N/A must carry its reason in
the ledger; an *applicable* standard with **no** champion is a coverage gap that **fails the gate
closed**, identical to an open finding.

**Not a license to manufacture reviewers.** "Full coverage" is *championed-or-N/A-with-reason*, not
"every agent in the roster on every gate." A standard with no bearing on the change — capacity for a
doc-only edit, security for a comment-wording fix — is recorded **N/A**; the discipline is that the
call is *explicit and recorded*, never silent. The point is that no standard is dropped by oversight,
not that every lens manufactures a finding.

## The gate evaluation (fail-closed — reads the ledger, never the transcript)

When the ship step is reached and a `review-gate` was declared, evaluate it:

1. **Compute the review-ledger path** from the target (PLT-535's target-derivable rule — no
   registry, no handoff token).
2. **Read the latest round's header block** and apply `/cross-review`'s passing-terminal gate-read
   **verbatim** (the provider's ledger-validity check — see the *Gate-read contract* table in
   `/cross-review/references/review-ledger.md`; this gate reads that table, it does not re-list or
   re-derive it):
   - `State:` is a passing terminal (`RESOLVED` | `RESOLVED-WITH-ACCEPTED-RISK`), **and**
   - `OpenFindings:` parses to integer `0`, **and**
   - `Convergence:` is present + parseable + in-enum, **and**
   - `Dissenter:` is non-empty (a dissenter was assigned), **and**
   - cross-field consistency holds (a passing terminal with `OpenFindings ≠ 0`, or `OPEN-BLOCKED`
     with `OpenFindings: 0`, is a self-contradictory header that fails closed).

   **Then apply the review-gate's *consensus refinement*: the latest round's `Convergence:` must be
   `unanimous`.** A review-gate is a *consensus* gate — a recorded `Convergence: split` latest round
   means the slate has not converged to consensus and is **not merge-ready**, so it does not satisfy
   the gate even with `OpenFindings: 0`. This is not a fork of the provider's contract: per
   `/cross-review`'s own rule a *resolved* split is re-recorded `unanimous` (and a genuinely
   unresolved split is `OPEN-BLOCKED`, `OpenFindings ≥ 1`), so requiring `unanimous` rejects only a
   non-consensus / ill-formed `split`-with-zero-open round — never a legitimately-converged ledger.
   The provider check answers "is this a valid resolved ledger"; the consensus refinement is the
   review-gate's own merge policy layered on top.

   **Then apply the *standards-coverage refinement*: the round's slate must be the full
   standards-champion cohort.** Confirm the ledger's per-lens verdicts cover every codified standard
   — each is championed by a reviewer or recorded N/A-with-reason (see the standards-coverage
   requirement). An *applicable* standard with no champion is a coverage gap that **fails closed**,
   identical to an open finding. Like the consensus refinement, this is the review-gate's own merge
   policy on top of the provider's contract, not a re-derivation of the slate — `/cross-review`
   convenes and records it; the gate only confirms the coverage is whole.
3. **Confirm every declared check is green.** A check that is pending / in-progress / neutral /
   errored / never reported is **not** green — fail closed on it.
4. **Satisfied ⇒ merge** per the operator's up-front authorization (no per-PR token). **Not
   satisfied ⇒** surface the specific reason + the ledger evidence and route to the pre-declared
   human checkpoint (`on_fail`). Never self-merge on a fail; never error-into-pass.

**Fail-closed is the load-bearing property.** Two sources of FAIL, kept distinct:

- **Provider-owned (the contract this gate reads verbatim — `/cross-review`'s gate-read contract):**
  an **absent** review-ledger, an **unparseable / missing** field, an **out-of-enum** `Convergence`,
  an **empty** `Dissenter`, a **self-contradictory** header (e.g. `State: RESOLVED` with
  `OpenFindings: 3`), **or a malformed latest round** — which, per that contract, *includes* an
  out-of-sequence `Round:` / round-number gap, and **never falls back** to an earlier round.
- **The review-gate's own consensus refinement (not the provider's — the provider *passes*
  `Convergence: split` as in-enum):** a `Convergence: split` latest round fails *this gate* because
  a split is not merge-ready consensus, even though it passes the provider's ledger-validity check.

Any of the above ⇒ the gate **FAILS**, identical to `State: OPEN`. A grep that finds `RESOLVED`
without cross-checking the count, or a chat assertion that "the reviewers ratified it," is exactly
the error this gate forbids — the committed ledger is the only evidence. (Do not "correct" the
provider's enum to reject `split`; the provider legitimately passes it, and other consumers may
merge on a resolved split — the `split`-rejection is *this* gate's policy, not the contract's.)

## The verify-to-convergence loop (lifecycle step 3)

The review-gate's *pass condition* is produced by the verify loop. The loop **composes**
`/cross-review`:

```
loop:
  invoke /cross-review on the integrated artifact
    → it classifies, routes the slate, dispatches blinded, synthesizes the review-ledger round
  read the latest round (the gate-read contract above)
  if passing terminal (incl. Convergence: unanimous):  converged → the review-gate can read pass
  elif OPEN-BLOCKED:           honest fail-to-human exit (a genuine split the tie-break didn't resolve)
  else (open findings):        apply the fixes → re-invoke /cross-review (it appends a NEW round) → repeat
```

**Loop bound (the open-findings branch is bounded by the human-driven serial model — MVP).** The
re-review branch always fails *closed* (an `OpenFindings ≥ 1` round never merges), but it has no
*progress* bound: a fix→re-review cycle that keeps surfacing new findings without splitting has no
declared terminal of its own. In the MVP the loop is **human-driven and serial** (the operator
sequences each round), so an unbounded spin is implausible — that is the de-facto bound. The
*mechanism* (a max-rounds-then-route-to-`on_fail`, or a no-progress detector that escalates when
round N's open set isn't shrinking) is **deferred** — un-defer the moment `/workstream` ever drives
the verify loop programmatically or unattended (the same trigger as the review-ledger's single-
writer/locking deferral). Stated here so the next implementer does not inherit it as an unstated
contract (mirrors the guard primitive's recursion bound).

- The loop's record **is** the review-ledger's appended rounds (per-round headers, per-lens
  verdicts, rejected-findings). The workstream keeps **no second convergence log** — "show that
  done was met" = point at the review-ledger and show its latest round passes the gate-read
  contract.
- The terminals are exactly the review-ledger's terminals: `RESOLVED` /
  `RESOLVED-WITH-ACCEPTED-RISK` (converged) or `OPEN-BLOCKED` (fails to a human; **never** relabeled
  to terminate the loop).
- The workstream **sequences** the rounds and **applies** the fixes; `/cross-review` owns *how*
  each round is dispatched and recorded. That is the compose/reimplement line.

## What the review-gate is NOT

- **Not a replacement for human gates.** It is additive. A workstream declares `design-approval`
  (human) + a `review-gate` (delegated merge), or keeps `pr-sign-off` (human). The operator chooses
  at declaration time.
- **Not a one-way-door gate.** A persisted-schema / wire-format / signed-ID / prod-touching merge
  keeps council's one-way-door **human** checkpoint. The slate may review it; the review-gate does
  not discharge the irreversible call.
- **Not self-relaxable.** "Unanimous RATIFY" means zero open concerns. An open DISSENT/MISMATCH or
  `OpenFindings ≥ 1` fails — "the dissent is minor/stale" is the rubber-stamp the gate forbids.
- **Not a partial-standards gate.** A merge clears every standard. A review-gate whose slate leaves
  an *applicable* standard unchampioned — no security lens on a boundary change, no prose lens on a
  doc, no systems lens on a concurrency change — is **not a valid gate**: the missing standard fails
  closed, identical to an open finding. Full coverage is championed-or-N/A-with-reason, never silent
  omission (the standards-coverage requirement).
- **Not authorized ad hoc.** The up-front declaration is the authorization. A review-gate added,
  widened, or relaxed silently mid-workstream is not a gate.
- **Not a re-implementation of `/cross-review`.** It reads the ledger and invokes the slate; it
  owns neither.
