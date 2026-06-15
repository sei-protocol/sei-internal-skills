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
  slate:    <the declared reviewer slate — or "routed by /cross-review per change-class">
  checks:   <the declared automated checks that must be green — e.g. cursor-bugbot, named CI workflows>
  ledger:   <the /cross-review review-ledger path — target-derivable per PLT-535; the gate computes it from the target, no registry>
  satisfied_when: the review-ledger's latest round reads a PASSING TERMINAL (per /cross-review's gate-read contract) AND every declared check has passed
  on_fail:  surface + route to a PRE-DECLARED human checkpoint (e.g. pr-sign-off) — never self-merge on a fail
```

## The gate evaluation (fail-closed — reads the ledger, never the transcript)

When the ship step is reached and a `review-gate` was declared, evaluate it:

1. **Compute the review-ledger path** from the target (PLT-535's target-derivable rule — no
   registry, no handoff token).
2. **Read the latest round's header block** and apply `/cross-review`'s gate-read contract
   **verbatim**. The gate passes only on the **conjunction**:
   - `State:` is a passing terminal (`RESOLVED` | `RESOLVED-WITH-ACCEPTED-RISK`), **and**
   - `OpenFindings:` parses to integer `0`, **and**
   - `Convergence:` is in-enum (`unanimous` | `split`), **and**
   - `Dissenter:` is non-empty (a dissenter was assigned), **and**
   - cross-field consistency holds (a passing terminal with `OpenFindings ≠ 0`, or `OPEN-BLOCKED`
     with `OpenFindings: 0`, is a self-contradictory header that fails closed).
3. **Confirm every declared check is green.** A check that is pending / in-progress / neutral /
   errored / never reported is **not** green — fail closed on it.
4. **Satisfied ⇒ merge** per the operator's up-front authorization (no per-PR token). **Not
   satisfied ⇒** surface the specific reason + the ledger evidence and route to the pre-declared
   human checkpoint (`on_fail`). Never self-merge on a fail; never error-into-pass.

**Fail-closed is the load-bearing property.** An **absent** review-ledger, an **unparseable /
missing** field, an **out-of-enum** `Convergence`, an **empty** `Dissenter`, a **self-
contradictory** header (e.g. `State: RESOLVED` with `OpenFindings: 3`), **or a malformed latest
round** (which never falls back to an earlier round) ⇒ the gate **FAILS**, identical to
`State: OPEN`. A grep that finds `RESOLVED` without cross-checking the count, or a chat assertion
that "the reviewers ratified it," is exactly the error this gate forbids — the committed ledger is
the only evidence.

## The verify-to-convergence loop (lifecycle step 3)

The review-gate's *pass condition* is produced by the verify loop. The loop **composes**
`/cross-review`:

```
loop:
  invoke /cross-review on the integrated artifact
    → it classifies, routes the slate, dispatches blinded, synthesizes the review-ledger round
  read the latest round (the gate-read contract above)
  if passing terminal:        converged → the review-gate can read pass
  elif OPEN-BLOCKED:           honest fail-to-human exit (a genuine split the tie-break didn't resolve)
  else (open findings):        apply the fixes → re-invoke /cross-review (it appends a NEW round) → repeat
```

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
- **Not authorized ad hoc.** The up-front declaration is the authorization. A review-gate added,
  widened, or relaxed silently mid-workstream is not a gate.
- **Not a re-implementation of `/cross-review`.** It reads the ledger and invokes the slate; it
  owns neither.
