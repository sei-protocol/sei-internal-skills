# Review Ledger — the durable synthesis record

One ledger per xreview **target**, written by the orchestrator at synthesis (Step 4) and
updated through resolution (Step 5). It is the artifact the dogfood (07 design) had to hand-
write into frontmatter — now produced by the skill. It is **PLT-536's `/workstream` review-gate
done-evidence** (see *Gate-read contract*); that consumer relationship drives the schema.

## Where it lives (target-derivable, no registry)

The ledger is a **lineage artifact**. It lives in the DRI's `<engineer>-designs` repo, under a
`xreview/` directory in the target's work-arc, named for the target. Design 13 covers that
process-lineage relocation, and names the same DRI-repo home `/design` and `/research` use.

```
designs/<arc>/xreview/<target-slug>.md
```

For a design-doc target the arc is **already a path segment of the target**. One example is
`designs/sei-agentic-mesh/xreview/08-cross-review-slate-and-ledger.md`, fully target-derivable.
For a **diff/PR with no natural artifact directory** the target carries no arc. The ledger then
lands under the **code repo's deterministic default arc**, where repo identity picks a fixed arc.
For `sei-internal-skills` that arc is `sei-internal-skills-stack`, so the ledger lands at
`designs/sei-internal-skills-stack/xreview/<target-slug>.md`. The in-repo
`.xreview/<target-slug>.md` fallback applies **only when no DRI repo is resolvable** (the user confirms).

**Two resolution faces — they are not the same contract (Design 13 §1):**
- **Producer (write-time, may be interactive).** Resolve the DRI repo as `/design` does
  (`--designs-repo` → sibling `<engineer>-designs` checkout → ask). In a **non-interactive
  (headless/cron) run, HALT and surface — never write to a guessed path** (the `/design` headless-halt
  clause).
- **Consumer (read-time, MUST be deterministic — no prompt, no registry).** The `/workstream`
  review-gate computes the ledger location from the target alone. It checks **two deterministic
  candidate paths in order**, both target-derivable, so this is a fixed lookup, not a search.

  - **(1)** the DRI-repo path `designs/<arc>/xreview/<slug>.md`, where **arc** is the target's
    path segment (design-doc) or the code repo's **default arc** (code-PR/diff).
  - **(2)** the in-repo `.xreview/<slug>.md` fallback, where the producer writes when no DRI repo
    resolves. **slug** per below, identical for both.

  The gate reads whichever exists; if **neither** exists it fails
  closed. It never asks and never reads a registry — checking both known producer-output locations is
  what keeps a fallback-written ledger findable without a prompt.

**Slug derivation:** a single-artifact target → that artifact's slug. A **multi-file diff with no
single artifact path** takes the **PR/branch identifier** as `<target-slug>`: single-valued, still
target-derivable. It is never a synthesized compound of the file paths. (`Target:` may list the files
for the human; the *filename* derives from the PR/branch so the gate can locate it deterministically.)
Within an arc's `xreview/` folder the slug namespace stays **single-valued** (a `pr-217` ledger never
collides with a design slugged `pr-217`).

It is **committed** — it is review evidence, not scratch. (Per-run scratch — in-progress
dispatch notes — stays in the skill's `state/`, gitignored.)

## Schema (fixed markdown sections)

The header fields are **typed, one-per-line, exact-token** — the gate matches on these lines,
so they are a contract, not prose. `State:` and `OpenFindings:` are **separate lines** (the gate
reads the count as an integer, not by parsing a parenthetical); `Convergence:` and `Blinded:`
are **separate lines** (two independent facts).

**Per-round headers (so the gate never reads a stale header).** Each `## Round <N>` section
carries **its own typed header block** — the six round-scoped fields (`State:`, `OpenFindings:`,
`Convergence:`, `Blinded:`, `Dissenter:`, `Lenses:`). The top-of-file block below *is round 1's header*; a
re-review appends a `## Round 2` section with a fresh header block of its own, and so on. The
target-scoped fields (`Target:`/`Class:`/`Tier:`) sit once at the top — they do not change across
rounds. The gate reads the **latest round's** header block (see *Gate-read contract*), never the
top block when a later round exists.

```markdown
# xreview ledger — <target>

Target:       <path or PR/branch of the artifact under review>
Class:        <doc-only | mechanical | component | cross-component | shared-stack | skill-package>
Tier:         <T1 | T2 | T3>   (+ override note if the operator changed it)

## Round 1
Round:        1                (incremented per re-review of the same target)
State:        <OPEN | RESOLVED | RESOLVED-WITH-ACCEPTED-RISK | OPEN-BLOCKED>
OpenFindings: <integer>        (count of still-open findings)
Convergence:  <unanimous | split | degenerate>  (this round only, tokens only — a prior-round split that this round resolved + re-ratified is `unanimous`; a single-lens round is `degenerate`, never `unanimous`; never free prose)
Blinded:      <yes | no>       (no downgrades confidence — say so in the Verdict)
Dissenter:    <which lens held assigned dissent this round — required, never empty>
Lenses:       <integer>        (how many lenses reported this round — 1 means a degenerate
                                single-reviewer pass, and `Convergence: unanimous` over one
                                lens corroborates nothing; the count makes that legible to a
                                consumer that reads only the header)

## Routing
- Slate: <lenses dispatched, each tagged domain / steward / dissenter (a lens may hold more than one — e.g. a steward also assigned the dissent)>
- Auto-wired stewards: <which, and why — e.g. "prose + rubric lens: skill-package change">
- Overrides: <none | operator lowered T3→T2, reason: "…", risk accepted: yes>

## Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| security-specialist | RATIFY | … cites contract/field/line … | n/a |
| network-specialist  | DISSENT | … the strongest objection … | artifact updated @ <commit/section> OR accepted-risk: <stated> |
| rubric lens         | RATIFY | rubric ids cited: T1 pass, S2 pass, C1 pass, D3 fail (warn), S4 skipped/inapplicable, **P7 pass (4 scenarios)** | n/a |

## Boundary findings  (the COMPATIBLE / MISMATCH / MISSING table — unchanged schema)
| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|

## Idiom addendum     (if code reviewed — correctness-grade blocks, style advisory)
## Prose addendum     (if prose reviewed — same gating rule)

## Rejected findings  (Rule 4 made auditable)
| Finding (as raised) | Raised by | Why rejected, and how verified |
|---|---|---|
| "worktree artifact X references a deleted file" | network-specialist | misread a round-1 excerpt; the file was removed in the diff under review — verified by reading the current diff, file absent |

## Verdict
<COMPATIBLE (overall) — confidence high/low + why | OPEN — N findings, each with what closes it | OPEN-BLOCKED — the split the tie-break did not resolve, escalated to a human>

## Round 2                       (appended on re-review — prior round stays verbatim above)
Round:        2
State:        <OPEN | RESOLVED | RESOLVED-WITH-ACCEPTED-RISK | OPEN-BLOCKED>
OpenFindings: <integer>
Convergence:  <unanimous | split | degenerate>
Blinded:      <yes | no>
Dissenter:    <lens — this round, required, never empty>
Lenses:       <integer>

### Routing / Per-lens verdicts / Boundary findings / … (this round's sections)
<the round's own Routing, Per-lens, Boundary, addenda, Rejected — parallel to Round 1>
```

Each appended `## Round <N>` repeats its **own** header block (the six round-scoped fields)
followed by that round's sections. The gate reads the **latest** round's header. For a one-round
ledger that is the top block (Round 1). Once Round 2 exists, Round 2's block is authoritative and
the top block is stale-by-design. It holds Round 1's record, kept verbatim, not the current state.

## `State:` enum (exact tokens — the gate matches these literally)

| Token | Meaning | Gate result |
|---|---|---|
| `OPEN` | findings remain; review not concluded | **fail closed** |
| `RESOLVED` | every finding closed; `OpenFindings: 0` | pass |
| `RESOLVED-WITH-ACCEPTED-RISK` | findings closed; ≥1 closed by a *stated, operator-accepted* risk (not a fix); `OpenFindings: 0` | pass |
| `OPEN-BLOCKED` | a genuine split the provider tie-break did **not** resolve — escalated to a human; `OpenFindings: ≥1` | **fail to human** |

`State` and `OpenFindings` are **independent fields** — the gate reads each, never deriving one
from the other. The consistency rules: `RESOLVED` and `RESOLVED-WITH-ACCEPTED-RISK` need
`OpenFindings: 0`; `OPEN-BLOCKED` needs `OpenFindings: ≥1`; `OPEN` may be either.

`OPEN-BLOCKED` is the honest exit for the convergence loop: a boundary where reviewers genuinely
split and no provider/consumer tie-break resolves it. It is a **terminal** state that **fails
the gate to a human**, never a pass. Nobody may relabel a split as
`RESOLVED-WITH-ACCEPTED-RISK` to make the loop stop; that laundering is exactly what
`OPEN-BLOCKED` exists to forbid. The only passing terminals are `RESOLVED` and
`RESOLVED-WITH-ACCEPTED-RISK`.

## Per-lens verdict = RATIFY / DISSENT

The reviewer-level roll-up that sits **above** the boundary table. RATIFY = "this lens reviewed
and endorses, with cited evidence." DISSENT = "this lens objects, here is the strongest
objection." Every lens lands one or the other; the orchestrator **rejects bare approval** and re-dispatches. The
boundary table (COMPATIBLE/MISMATCH/MISSING) is the *finding-level* schema and stays untouched —
the ledger carries both altitudes (which lens, and which boundaries).

## Rejected findings are first-class

Rule 4 forbids silently dropping a MISMATCH/MISSING. A *rejected* finding is **not "dropped"** — say,
the reviewer read a worktree artifact wrong. The orchestrator adjudicates it and records the
adjudication, so the next reader sees *rejected-with-rationale*, not *vanished*.
The `Rejected findings` table has one **"Why rejected, and how verified"** column. It carries the
finding as raised, who raised it, the reason for rejecting it, and the check that confirmed the
rejection. Reading the file and pushing back on a wrong finding now lands in the record rather
than passing through the transcript.

## Convergence and dissenter

Every ledger **must** carry `Lenses:`, and `Lenses: 1` **must** carry `Convergence: degenerate`.
Unanimity across a single reviewer is the consensus theater the assigned-dissent rule exists to
catch, with the field filled in. The lens agrees with itself. `SKILL.md` already tells the
orchestrator to label a single-reviewer pass "a degenerate xreview, not dressed up as a full
one". The token puts that obligation in the contract instead of in prose no gate reads.

**Verdict vocabulary (exact tokens).** A slate row's verdict cell opens with one of:
`RATIFY` · `RATIFY-with-advisory` · `DISSENT` · `NOT-RE-DISPATCHED`. An automated reviewer's
native `APPROVED` / `CHANGES_REQUESTED` also resolve, to `RATIFY` and `DISSENT`. Commentary follows the
token; the token is what a gate reads.

`NOT-RE-DISPATCHED` is the append-only rule's carried row: the lens did not report this round, and
its verdict is **whatever it last issued**. A carried `DISSENT` is still a `DISSENT`. Writing it
as prose ("not re-dispatched, its round 5 DISSENT stands") makes it invisible to every check that
reads verdicts. That is the substitution the append-only rule exists to stop, one step later.

`Lenses:` counts **reporters** — rows carrying one of the first three tokens. The slate table is
append-only so its row count only grows; carried rows make up the surplus.

**Three ways a round with a `DISSENT` reaches a passing `State:`** — and only three.

1. **Re-dispatch.** The lens sees the fix and issues a new verdict. The new verdict belongs to
   a new round; it never overwrites the old one.
2. **Accept-with-risk.** The operator names the risk and accepts it. `State:`
   is `RESOLVED-WITH-ACCEPTED-RISK`, never plain `RESOLVED`.
3. **Every finding that DISSENT raised closes.** `State: RESOLVED`, `OpenFindings: 0` — and
   `Convergence:` stays `split` or `degenerate`, because the *verdict* did not change. This is
   the common case, and the one most easily mistaken for the substitution below.

The ledger reads `Convergence:` off the per-lens verdicts **as the lenses issued them**. An
orchestrator never restates a lens's verdict. The ledger records a resolved finding against the
standing `DISSENT`, and the verdict changes only when that lens re-reviews and says so itself. A round holding
any `DISSENT` row is `split`, never `unanimous`.

The round's slate table measures `Lenses:`, and the field is never self-reported. A round
declaring N lenses must list N of them. A round with no slate table can only honestly be one lens.

`Convergence: unanimous | split` and `Blinded: yes | no` are **two separate lines** (two
independent facts). A split is a finding, not a rounding error. An un-blinded review
(`Blinded: no`) downgrades confidence and says so in the Verdict. `Dissenter:` **must** carry a
lens and never sit empty. A `Convergence: unanimous` line is only honest when the slate assigned
a dissenter and that dissenter still concluded RATIFY. Unanimity without an assigned dissenter is
consensus theater.

## Single-round MVP — append, never merge in place

MVP is a **single-round committed ledger**. A re-review **appends a new `## Round <N>` section**
(or a sibling file) — **never an in-place edit of a prior round's rows.** Each appended round
**carries its own typed header block** — the six round-scoped fields `State:`/`OpenFindings:`/
`Convergence:`/`Blinded:`/`Dissenter:`/`Lenses:`. The latest round's state stays self-contained,
and the gate never reads a stale top header.

The ledger has **no cross-round dedup engine**. Nothing matches a round-2
re-raise to a round-1 row to merge them. This is **per-round headers, not row-merging**. The
prior round stays verbatim as append-only history; the new round stands beside it with its own
header.

**Dedup within a round:** one row per boundary, one row per lens, *within a single committed
round*. That is the whole MVP dedup rule.

**Resumability (read-forward, not merge-back).** A resumed or re-run xreview **reads the
prior ledger first** for context: what the last round concluded, and which findings it rejected
and why. That context keeps it from re-litigating settled findings or re-raising rejected ones
without new evidence. It records its conclusions in a **new round with its own header block**,
never by editing the old one or its header. The reader (and 536's gate) reads the **latest
round's** header block. The top-of-file block is Round 1's; the latest `## Round <N>` block is
authoritative once it exists.

**Concurrency assumption (single writer per target).** "The latest round is unambiguous" holds
only under a **single writer per target per re-review** — the MVP's human-driven, serial model.
Two concurrent re-reviews of the same target could both compute the same next round number and
both append `## Round <N>`, making "read the latest round" ambiguous. Concurrent re-review of one
target is **out of MVP scope**. The MVP defers the locking / round-number-CAS *mechanism* (YAGNI)
until `/workstream` ever drives cross-reviews programmatically or in parallel. This file states
the single-writer **assumption** so the next implementer does not trip on it as an unstated
contract.

*(Multi-round in-place merge + cross-round dedup is CUT from MVP. It waits until a single target
draws two or more reviews and append-only history turns noisy enough to justify a row-merge
engine.)*

## Gate-read contract (PLT-536 `/workstream` review-gate consumer)

The review-gate computes the ledger path from the target path (above) and reads the **latest
round's header block**. For a one-round ledger that is the top-of-file block (Round 1). Once a
`## Round <N>` section exists, **that round's own header block is authoritative**. The top
block is then Round 1's stale-by-design record, never the field source. The gate passes only on
the **conjunction of all of** the rows below, read from the latest round's block.

**Round-selection fails closed too (the selection step is total).** "Read the latest round" must
itself resolve to FAIL when it cannot resolve cleanly. Take the **highest-numbered** `## Round <N>`
section, **present but unparseable**: its header block missing, or its `Round:` line absent,
non-integer, or **out of sequence**. Rounds run **contiguous from 1**, so a gap such as Round 1 →
Round 3 is out-of-sequence. The gate then **FAILS closed**, and it **never falls back** to reading
an earlier round's header. A stale earlier `RESOLVED` must not satisfy the gate when the latest
round does not parse; a malformed latest round is identical to a malformed ledger.

This "highest-numbered = latest" selector is unambiguous only under the single-writer assumption
stated in *Single-round MVP* above. A duplicate `## Round <N>` is out of MVP scope.

| Schema line | Pass requires | Fail if |
|---|---|---|
| `State:` | `RESOLVED` or `RESOLVED-WITH-ACCEPTED-RISK` (exact token) | `OPEN`, `OPEN-BLOCKED`, any other/missing token |
| `OpenFindings:` | parses to integer `0` | non-zero, non-integer, or absent |
| `Convergence:` | `unanimous`, `split` or `degenerate` (present, parseable, **latest round only, tokens only — never free prose**) | absent, unparseable, or **any token other than `unanimous`\|`split`\|`degenerate`** (out-of-enum fails identical to absent) |
| `Blinded:` | `yes` or `no` (advisory — `no` downgrades confidence, does not fail) | — |
| `Lenses:` | a positive integer, counting the lenses that **reported** this round (a `NOT-RE-DISPATCHED` row is not a reporter) | absent, non-integer, or `< 1` |
| `Dissenter:` | non-empty (the slate assigned a dissenter) | empty / absent |
| **Cross-field consistency** | `State` and `OpenFindings` agree | `State: RESOLVED`\|`RESOLVED-WITH-ACCEPTED-RISK` with `OpenFindings ≠ 0`, **OR** `OPEN-BLOCKED` with `OpenFindings: 0` — a contradictory-but-parseable header **fails closed identical to an absent one**. (`OPEN` fails on the `State:` row regardless of count — it is not a cross-field case; per the enum, `OPEN` may carry either count.) |

**Fail closed — the load-bearing correctness property.** The gate fails closed on an **absent,
malformed, or self-contradictory** ledger, not only on `State: OPEN`. The gate **FAILS**,
identical to `State: OPEN`, whenever a grep finds **no** ledger file. The same holds for an
unparseable or missing `State:`, an absent or non-integer `OpenFindings:`, an **out-of-enum**
`Convergence:`, an **empty** `Dissenter:`, or a contradiction between `State:` and
`OpenFindings:`.

A
parseable-but-contradictory header (e.g. `State: RESOLVED` with `OpenFindings: 3`) is **not** a
pass. The gate reads both fields, and the cross-check is part of the conjunction. A grep that
finds `RESOLVED` without cross-checking `OpenFindings` is the exact error this row forbids.

The gate must **never error-into-pass**. A search that finds no clean `RESOLVED` — token present
*and* count `0` *and* fields consistent — resolves to FAIL, never to a skipped check that
proceeds. (The pre-design "synthesis evaporated into the transcript" status quo — no ledger —
must not pass the gate.)

**Provider/consumer:** `/xreview` is the **provider** of this schema; `/workstream`'s
review-gate is the **consumer**. Per the skill's tie-break, this schema is canonical. 536's gate
adapts to it and reads the ledger, never re-deriving review state from the transcript.
