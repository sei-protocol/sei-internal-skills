# Review Ledger — the durable synthesis record

One ledger per xreview **target**, written by the orchestrator at synthesis (Step 4) and
updated through resolution (Step 5). It is the artifact the dogfood (07 design) had to hand-
write into frontmatter — now produced by the skill. It is **PLT-536's `/workstream` review-gate
done-evidence** (see *Gate-read contract*); that consumer relationship drives the schema.

## Where it lives (target-derivable, no registry)

The ledger is a **lineage artifact** — it lives in the DRI's `<engineer>-designs` repo, under a
`xreview/` directory in the target's work-arc, named for the target (Design 13 — process-lineage
relocation; the same DRI-repo home `/design` and `/research` use):

```
designs/<arc>/xreview/<target-slug>.md
```

For a design-doc target the arc is **already a path segment of the target**
(`designs/sei-agentic-mesh/xreview/08-cross-review-slate-and-ledger.md`) — fully target-derivable.
For a **diff/PR with no natural artifact directory** the target carries no arc, so the ledger lands
under the **code repo's deterministic default arc** (repo identity → a fixed arc, e.g. `sei-internal-skills` →
`sei-internal-skills-stack`): `designs/sei-internal-skills-stack/xreview/<target-slug>.md`. The in-repo
`.xreview/<target-slug>.md` fallback is used **only when no DRI repo is resolvable** (the user confirms).

**Two resolution faces — they are not the same contract (Design 13 §1):**
- **Producer (write-time, may be interactive).** Resolve the DRI repo as `/design` does
  (`--designs-repo` → sibling `<engineer>-designs` checkout → ask). In a **non-interactive
  (headless/cron) run, HALT and surface — never write to a guessed path** (the `/design` headless-halt
  clause).
- **Consumer (read-time, MUST be deterministic — no prompt, no registry).** The `/workstream`
  review-gate computes the ledger location from the target alone and checks **two deterministic
  candidate paths in order** (both target-derivable, so this is a fixed lookup, not a search):
  **(1)** the DRI-repo path `designs/<arc>/xreview/<slug>.md` — **arc** = the target's path segment
  (design-doc) or the code repo's **default arc** (code-PR/diff); **(2)** the in-repo
  `.xreview/<slug>.md` fallback (where the producer writes when no DRI repo was resolvable). **slug**
  per below, identical for both. The gate reads whichever exists; if **neither** exists it fails
  closed. It never asks and never reads a registry — checking both known producer-output locations is
  what keeps a fallback-written ledger findable without a prompt.

**Slug derivation:** a single-artifact target → that artifact's slug. A **multi-file diff with no
single artifact path** → `<target-slug>` is the **PR/branch identifier** (single-valued, still
target-derivable) — never a synthesized compound of the file paths. (`Target:` may list the files
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
carries **its own typed header block** — the five round-scoped fields (`State:`, `OpenFindings:`,
`Convergence:`, `Blinded:`, `Dissenter:`). The top-of-file block below *is round 1's header*; a
re-review appends a `## Round 2` section with a fresh header block of its own, and so on. The
target-scoped fields (`Target:`/`Class:`/`Tier:`) sit once at the top — they don't change across
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
| rubric lens         | RATIFY | rubric ids cited: T1 pass, S2 pass, C1 pass | n/a |

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

Each appended `## Round <N>` repeats its **own** header block (the five round-scoped fields)
followed by that round's sections. The gate reads the **latest** round's header — for a one-round
ledger that is the top block (Round 1); once Round 2 exists, Round 2's block is authoritative and
the top block is stale-by-design (it is Round 1's record, kept verbatim, not the current state).

## `State:` enum (exact tokens — the gate matches these literally)

| Token | Meaning | Gate result |
|---|---|---|
| `OPEN` | findings remain; review not concluded | **fail closed** |
| `RESOLVED` | every finding closed; `OpenFindings: 0` | pass |
| `RESOLVED-WITH-ACCEPTED-RISK` | findings closed; ≥1 closed by a *stated, operator-accepted* risk (not a fix); `OpenFindings: 0` | pass |
| `OPEN-BLOCKED` | a genuine split the provider tie-break did **not** resolve — escalated to a human; `OpenFindings: ≥1` | **fail to human** |

`State` and `OpenFindings` are **independent fields** — the gate reads each, never deriving one
from the other. The consistency rules: `RESOLVED` and `RESOLVED-WITH-ACCEPTED-RISK` require
`OpenFindings: 0`; `OPEN-BLOCKED` requires `OpenFindings: ≥1`; `OPEN` may be either.

`OPEN-BLOCKED` is the honest exit for the convergence loop: a boundary where reviewers genuinely
split and no provider/consumer tie-break resolves it. It is a **terminal** state that **fails
the gate to a human** — it is *not* a pass, and a split must **not** be relabeled
`RESOLVED-WITH-ACCEPTED-RISK` to make the loop terminate (that laundering is exactly what
`OPEN-BLOCKED` exists to forbid). The only passing terminals are `RESOLVED` and
`RESOLVED-WITH-ACCEPTED-RISK`.

## Per-lens verdict = RATIFY / DISSENT

The reviewer-level roll-up that sits **above** the boundary table. RATIFY = "this lens reviewed
and endorses, with cited evidence." DISSENT = "this lens objects, here is the strongest
objection." Every lens lands one or the other; **bare approval is rejected** (re-dispatch). The
boundary table (COMPATIBLE/MISMATCH/MISSING) is the *finding-level* schema and stays untouched —
the ledger carries both altitudes (which lens, and which boundaries).

## Rejected findings are first-class

Rule 4 forbids silently dropping a MISMATCH/MISSING. A *rejected* finding (the reviewer was
wrong — e.g. the worktree-artifact misread) is **not "dropped"** — it is adjudicated, and the
adjudication must be recorded so the next reader sees *rejected-with-rationale*, not *vanished*.
The `Rejected findings` table has one **"Why rejected, and how verified"** column: the finding
as raised, who raised it, why it was rejected, and how that was verified (read the file, push
back if the finding is wrong — now recorded rather than transient).

## Convergence and dissenter

`Lenses:` is **required**, and `Lenses: 1` **must** carry `Convergence: degenerate`. Unanimity
across a single reviewer is the consensus theater the assigned-dissent rule exists to catch, with
the field filled in — the lens agreeing with itself. `SKILL.md` already requires a single-reviewer
pass be labelled "a degenerate xreview, not dressed up as a full one"; the token puts that
obligation in the contract instead of in prose no gate reads.

`Lenses:` is measured against the round's slate table, not self-reported: a round declaring N
lenses must list N of them, and a round with no slate table can only honestly be one lens.

`Convergence: unanimous | split` and `Blinded: yes | no` are **two separate lines** (two
independent facts). A split is a finding, not a rounding error. An un-blinded review
(`Blinded: no`) downgrades confidence and says so in the Verdict. `Dissenter:` is **required and
never empty** — a `Convergence: unanimous` line is only honest if a dissenter was assigned and
still concluded RATIFY (unanimity without an assigned dissenter is consensus theater).

## Single-round MVP — append, never merge in place

MVP is a **single-round committed ledger**. A re-review **appends a new `## Round <N>` section**
(or a sibling file) — **never an in-place edit of a prior round's rows.** Each appended round
**carries its own typed header block** (the five round-scoped fields: `State:`/`OpenFindings:`/
`Convergence:`/`Blinded:`/`Dissenter:`) so the latest round's state is self-contained and the gate
never reads a stale top header. There is **no cross-round dedup engine** (no matching a round-2
re-raise to a round-1 row to merge them) — this is **per-round headers, not row-merging**. The
prior round stays verbatim as append-only history; the new round stands beside it with its own
header.

**Dedup within a round:** one row per boundary, one row per lens, *within a single committed
round*. That is the whole MVP dedup rule.

**Resumability (read-forward, not merge-back):** a resumed or re-run xreview **reads the
prior ledger first** for context (what the last round concluded, what was rejected and why) so it
does not re-litigate settled findings or re-raise rejected ones without new evidence — but it
records its conclusions in a **new round with its own header block**, not by editing the old one
or its header. The reader (and 536's gate) reads the **latest round's** header block — the
top-of-file block is Round 1's; the latest `## Round <N>` block is authoritative once it exists.

**Concurrency assumption (single writer per target).** "The latest round is unambiguous" holds
only under a **single writer per target per re-review** — the MVP's human-driven, serial model.
Two concurrent re-reviews of the same target could both compute the same next round number and
both append `## Round <N>`, making "read the latest round" ambiguous. Concurrent re-review of one
target is **out of MVP scope**; the locking / round-number-CAS *mechanism* is deferred (YAGNI)
until `/workstream` ever drives cross-reviews programmatically or in parallel — but the
single-writer **assumption** is stated here so the next implementer does not trip on it as an
unstated contract.

*(Multi-round in-place merge + cross-round dedup is CUT from MVP — deferred until a single target
is re-reviewed ≥2× and append-only history is demonstrably noisy enough to justify a row-merge
engine.)*

## Gate-read contract (PLT-536 `/workstream` review-gate consumer)

The review-gate computes the ledger path from the target path (above), reads the **latest
round's header block** — for a one-round ledger that is the top-of-file block (Round 1); once a
`## Round <N>` section exists, **that round's own header block is authoritative** and the top
block is Round 1's stale-by-design record, never the field source. The gate passes only on the
**conjunction of all of** (read from the latest round's block):

**Round-selection fails closed too (the selection step is total).** "Read the latest round" must
itself resolve to FAIL when it cannot resolve cleanly: if the **highest-numbered** `## Round <N>`
section is **present but unparseable** — its header block missing, or its `Round:` line absent /
non-integer / **out of sequence** (rounds are **contiguous from 1**; a gap such as Round 1 → Round
3 is out-of-sequence) — the gate **FAILS closed**; it **never falls back** to reading an earlier
round's header (a stale earlier `RESOLVED` must not satisfy the gate when the current round is
malformed). A malformed latest round is identical to a malformed ledger. (This "highest-numbered =
latest" selector is unambiguous only under the single-writer assumption stated in *Single-round
MVP* above — a duplicate `## Round <N>` is out of MVP scope.)

| Schema line | Pass requires | Fail if |
|---|---|---|
| `State:` | `RESOLVED` or `RESOLVED-WITH-ACCEPTED-RISK` (exact token) | `OPEN`, `OPEN-BLOCKED`, any other/missing token |
| `OpenFindings:` | parses to integer `0` | non-zero, non-integer, or absent |
| `Convergence:` | `unanimous` or `split` (present, parseable, **latest round only, tokens only — never free prose**) | absent, unparseable, or **any token other than `unanimous`\|`split`** (out-of-enum fails identical to absent) |
| `Blinded:` | `yes` or `no` (advisory — `no` downgrades confidence, does not fail) | — |
| `Dissenter:` | non-empty (a dissenter was assigned) | empty / absent |
| **Cross-field consistency** | `State` and `OpenFindings` agree | `State: RESOLVED`\|`RESOLVED-WITH-ACCEPTED-RISK` with `OpenFindings ≠ 0`, **OR** `OPEN-BLOCKED` with `OpenFindings: 0` — a contradictory-but-parseable header **fails closed identical to an absent one**. (`OPEN` fails on the `State:` row regardless of count — it is not a cross-field case; per the enum, `OPEN` may carry either count.) |

**Fail closed — the load-bearing correctness property.** The gate fails closed on an **absent,
malformed, or self-contradictory** ledger, not only on `State: OPEN`. A grep that finds **no**
ledger file, an **unparseable/missing** `State:`, a non-integer/absent `OpenFindings:`, an
**out-of-enum** `Convergence:`, an **empty** `Dissenter:`, **or a header whose `State:` and
`OpenFindings:` contradict each other** ⇒ the gate **FAILS**, identical to `State: OPEN`. A
parseable-but-contradictory header (e.g. `State: RESOLVED` with `OpenFindings: 3`) is **not** a
pass: the gate reads both fields and the cross-check is part of the conjunction, so a grep that
finds `RESOLVED` without cross-checking `OpenFindings` is the exact error this row forbids. The
gate must **never error-into-pass**: a search that finds no clean `RESOLVED` (token present *and*
count `0` *and* fields consistent) resolves to FAIL, never to a skipped check that proceeds. (The
pre-design "synthesis evaporated into the transcript" status quo — no ledger — must not pass the
gate.)

**Provider/consumer:** `/xreview` is the **provider** of this schema; `/workstream`'s
review-gate is the **consumer**. Per the skill's tie-break, this schema is canonical; 536's gate
adapts to it and reads the ledger, never re-deriving review state from the transcript.
