# The review-gate (a consensus gate that composes `/xreview`)

The third gate kind, alongside the human `checkpoint` and the signal `guard`. A **review-gate**
makes *"merge once the reviewer slate is unanimously RATIFY (zero open concerns) and the declared
automated checks pass"* a **declarable, enforceable** primitive — the gate operators reach for when
they delegate `pr-sign-off` to expert-consensus + CI.

It is the machine counterpart of `pr-sign-off`, the way a `guard` is the machine counterpart of a
human-watched cutover: it discharges the *routine* merge the operator pre-authorized; the human
still owns every one-way door.

## Compose, never reimplement — the one-direction coupling

`/xreview` (PLT-535 / Design 08) is the **provider**: it owns the slate, the routing, the
blinded dispatch, the assigned dissent, and the **review-ledger** schema + gate-read contract. The
review-gate is the **consumer**: it *invokes* the slate (the verify-to-convergence loop) and
*reads* the review-ledger (the gate evaluation). 

There is **one** coupling surface — `/xreview`'s **gate-read contract** (Design 08, *How it
composes*). The review-gate reads exactly the latest round's header fields that contract names, and
nothing else. Per Design 08's stated tie-break, **that contract is canonical**; the review-gate
adapts to it and never re-derives review state. If the contract changes, the review-gate follows
it — there are not two contracts.

> **Never reimplement** the slate, the routing, the steward-wiring, or the review-ledger schema
> here. If you find yourself defining how a round is dispatched or what the ledger header looks
> like, stop — that is `/xreview`'s, and duplicating it forks the contract.

## The ledger entry

A third entry kind in the workstream's checkpoint ledger, declared up front:

```
- review-gate: <short identifier, kebab-case>
  slate:    <the declared reviewer slate — or "routed by /xreview per change-class">
  checks:   <the declared automated checks that must be green — e.g. cursor-bugbot, named CI workflows>
  ledger:   <the /xreview review-ledger path — target-derivable per PLT-535; the gate computes it from the target, no registry>
  satisfied_when: the review-ledger's latest round reads a PASSING TERMINAL per /xreview's gate-read contract (currently State RESOLVED|RESOLVED-WITH-ACCEPTED-RISK, OpenFindings 0, Dissenter non-empty, cross-field consistent) AND Convergence is unanimous (the consensus refinement — see gate evaluation) AND every declared check has passed
  on_fail:  surface + route to a PRE-DECLARED human checkpoint (e.g. pr-sign-off) — never self-merge on a fail
```

## The gate evaluation (fail-closed — reads the ledger, never the transcript)

When the ship step is reached and a `review-gate` was declared, evaluate it:

1. **Compute the review-ledger path** from the target (PLT-535's target-derivable rule — no
   registry, no handoff token).
2. **Read the latest round's header block** and apply `/xreview`'s passing-terminal gate-read
   **verbatim** (the provider's ledger-validity check — see the *Gate-read contract* table in
   `/xreview/references/review-ledger.md`; this gate reads that table, it does not re-list or
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
   `/xreview`'s own rule a *resolved* split is re-recorded `unanimous` (and a genuinely
   unresolved split is `OPEN-BLOCKED`, `OpenFindings ≥ 1`), so requiring `unanimous` rejects only a
   non-consensus / ill-formed `split`-with-zero-open round — never a legitimately-converged ledger.
   The provider check answers "is this a valid resolved ledger"; the consensus refinement is the
   review-gate's own merge policy layered on top.
3. **Confirm every declared check is green.** A check that is pending / in-progress / neutral /
   errored / never reported is **not** green — fail closed on it.
4. **Satisfied ⇒ merge** per the operator's up-front authorization (no per-PR token). **Not
   satisfied ⇒** surface the specific reason + the ledger evidence and route to the pre-declared
   human checkpoint (`on_fail`). Never self-merge on a fail; never error-into-pass.

**Fail-closed is the load-bearing property.** Two sources of FAIL, kept distinct:

- **Provider-owned (the contract this gate reads verbatim — `/xreview`'s gate-read contract):**
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
`/xreview`:

```
loop:
  invoke /xreview on the integrated artifact
    → it classifies, routes the slate, dispatches blinded, synthesizes the review-ledger round
  read the latest round (the gate-read contract above)
  if passing terminal (incl. Convergence: unanimous):  converged → the review-gate can read pass
  elif OPEN-BLOCKED:           honest fail-to-human exit (a genuine split the tie-break didn't resolve)
  else (open findings):        apply the fixes → re-invoke /xreview (it appends a NEW round) → repeat
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
- The workstream **sequences** the rounds and **applies** the fixes; `/xreview` owns *how*
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
- **Not authorized ad hoc.** The up-front declaration is the authorization. A review-gate added,
  widened, or relaxed silently mid-workstream is not a gate.
- **Not a re-implementation of `/xreview`.** It reads the ledger and invokes the slate; it
  owns neither.
