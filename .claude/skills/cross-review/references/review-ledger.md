# Review Ledger — the durable synthesis record

One ledger per cross-review **target**, written by the orchestrator at synthesis (Step 4) and
updated through resolution (Step 5). It is the artifact the dogfood (07 design) had to hand-
write into frontmatter — now produced by the skill. It is **PLT-536's `/workstream` review-gate
done-evidence** (see *Gate-read contract*); that consumer relationship drives the schema.

## Where it lives (target-derivable, no registry)

Next to the artifact under review, under a `cross-review/` sibling directory, named for the
target:

```
<artifact-dir>/cross-review/<target-slug>.md
```

For a design doc that is `designs/sei-agentic-mesh/cross-review/08-cross-review-slate-and-ledger.md`.
For a **diff/PR with no natural artifact directory**, the default is `.cross-review/<target-slug>.md`
at the repo root. Either way the rule is the same: **the path is target-derivable** (target path
→ ledger path) so a downstream gate finds it by construction without a registry.

**Slug derivation:** a single-artifact target → that artifact's slug. A **multi-file diff with no
single artifact path** → `<target-slug>` is the **PR/branch identifier** (single-valued, still
target-derivable) — never a synthesized compound of the file paths. (`Target:` may list the files
for the human; the *filename* derives from the PR/branch so the gate can locate it deterministically.)

It is **committed** — it is review evidence, not scratch. (Per-run scratch — in-progress
dispatch notes — stays in the skill's `state/`, gitignored.)

## Schema (fixed markdown sections)

The header fields are **typed, one-per-line, exact-token** — the gate matches on these lines,
so they are a contract, not prose. `State:` and `OpenFindings:` are **separate lines** (the gate
reads the count as an integer, not by parsing a parenthetical); `Convergence:` and `Blinded:`
are **separate lines** (two independent facts).

```markdown
# Cross-review ledger — <target>

Target:       <path or PR/branch of the artifact under review>
Class:        <doc-only | mechanical | component | cross-component | shared-stack | skill-package>
Tier:         <T1 | T2 | T3>   (+ override note if the operator changed it)
Round:        <N>              (incremented per re-review of the same target)
State:        <OPEN | RESOLVED | RESOLVED-WITH-ACCEPTED-RISK | OPEN-BLOCKED>
OpenFindings: <integer>        (count of still-open findings)
Convergence:  <unanimous | split>   (LATEST round only, tokens only — a prior-round split that this round resolved + re-ratified is `unanimous`; never free prose)
Blinded:      <yes | no>       (no downgrades confidence — say so in the Verdict)
Dissenter:    <which lens held assigned dissent this round — required, never empty>

## Routing
- Slate: <lenses dispatched, each tagged domain | steward | dissenter>
- Auto-wired stewards: <which, and why — e.g. "audit+author+prose: skill-package change">
- Overrides: <none | operator lowered T3→T2, reason: "…", risk accepted: yes>

## Per-lens verdicts
| Lens | Verdict | Finding (evidence-bearing) | Resolution |
|---|---|---|---|
| security-specialist | RATIFY | … cites contract/field/line … | n/a |
| network-specialist  | DISSENT | … the strongest objection … | artifact updated @ <commit/section> | accepted-risk: <stated> |
| author-skill        | RATIFY | triggers/guardrails/evals present, cited | n/a |

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
```

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

`Convergence: unanimous | split` and `Blinded: yes | no` are **two separate lines** (two
independent facts). A split is a finding, not a rounding error. An un-blinded review
(`Blinded: no`) downgrades confidence and says so in the Verdict. `Dissenter:` is **required and
never empty** — a `Convergence: unanimous` line is only honest if a dissenter was assigned and
still concluded RATIFY (unanimity without an assigned dissenter is consensus theater).

## Single-round MVP — append, never merge in place

MVP is a **single-round committed ledger**. A re-review **appends a new `## Round <N>` section**
(or a sibling file) — **never an in-place edit of a prior round's rows.** There is **no
cross-round dedup engine** (no matching a round-2 re-raise to a round-1 row to merge them). The
prior round stays verbatim as append-only history; the new round stands beside it.

**Dedup within a round:** one row per boundary, one row per lens, *within a single committed
round*. That is the whole MVP dedup rule.

**Resumability (read-forward, not merge-back):** a resumed or re-run cross-review **reads the
prior ledger first** for context (what the last round concluded, what was rejected and why) so it
does not re-litigate settled findings or re-raise rejected ones without new evidence — but it
records its conclusions in a **new round**, not by editing the old one. The reader (and 536's
gate) reads the **latest round's** header fields.

*(Multi-round in-place merge + cross-round dedup is CUT from MVP — deferred until a single target
is re-reviewed ≥2× and append-only history is demonstrably noisy enough to justify a row-merge
engine.)*

## Gate-read contract (PLT-536 `/workstream` review-gate consumer)

The review-gate computes the ledger path from the target path (above), reads the **latest
round's** header, and passes only on the **conjunction of all of**:

| Schema line | Pass requires | Fail if |
|---|---|---|
| `State:` | `RESOLVED` or `RESOLVED-WITH-ACCEPTED-RISK` (exact token) | `OPEN`, `OPEN-BLOCKED`, any other/missing token |
| `OpenFindings:` | parses to integer `0` | non-zero, non-integer, or absent |
| `Convergence:` | `unanimous` or `split` (present and parseable) | absent / unparseable |
| `Dissenter:` | non-empty (a dissenter was assigned) | empty / absent |
| `Blinded:` | `yes` or `no` (advisory — `no` downgrades confidence, does not fail) | — |

**Fail closed — the load-bearing correctness property.** The gate fails closed on an **absent or
malformed** ledger, not only on `State: OPEN`. A grep that finds **no** ledger file, an
**unparseable/missing** `State:`, a non-integer/absent `OpenFindings:`, or an **empty**
`Dissenter:` ⇒ the gate **FAILS**, identical to `State: OPEN`. The gate must **never error-into-
pass**: a search that finds no `RESOLVED` resolves to FAIL, never to a skipped check that
proceeds. (The pre-design "synthesis evaporated into the transcript" status quo — no ledger —
must not pass the gate.)

**Provider/consumer:** `/cross-review` is the **provider** of this schema; `/workstream`'s
review-gate is the **consumer**. Per the skill's tie-break, this schema is canonical; 536's gate
adapts to it and reads the ledger, never re-deriving review state from the transcript.
