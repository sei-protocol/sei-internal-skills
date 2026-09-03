# Review ledger — cutting `author-skill` and `audit-skill`

Target:       branch `refactor/cut-skill-authoring-pair` (`a9e512e`, `7e9461b`) —
deletes `.claude/skills/author-skill/` and `.claude/skills/audit-skill/`, moves the
skill-package rubric and its static checker into `/xreview`, rewrites the steward pin.

```
Class:        skill-package
Tier:         T3
```

**Path**: this is the documented in-repo fallback, `.xreview/<slug>.md` — one of the two
locations the consumer gate checks. It belongs in the DRI's designs repository under
`designs/sei-internal-skills-stack/xreview/`; that move is pending for this ledger and for the
two older ones still under `docs/xreview/`, which is a third location no gate can find.

## Round 1

```
Round:        1
State:        RESOLVED
OpenFindings: 0
Convergence:  split
Blinded:      yes
Dissenter:    security-specialist
Lenses:       4
```

Three of four lenses returned DISSENT; all findings closed in rounds 1-2. Blinded: four
independent briefs, no reviewer saw another's findings.

### Slate

| Lens | Why pinned | Verdict |
|---|---|---|
| `prose-steward` | agent-steward, `skill-package` unconditional | DISSENT |
| rubric lens | `skill-package` unconditional | DISSENT — cited `P7`, `E5`, `B6`, `R4`, `D1`, `C3`, `A1`, `R3` |
| `systems-engineer` | §4a — the diff touches `scripts/` | DISSENT |
| `security-specialist` | §4a — agent-instruction surface; **assigned dissenter** | DISSENT |

### Boundary findings

| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|
| steward pin: `SKILL.md` ↔ `slate-routing.md` | `slate-routing.md` (self-declared authority, §1 "the table wins") | `SKILL.md` | **MISMATCH** | `slate-routing.md:106-115` still resolved the lens against `.claude/skills/` and HALTed on absence; `SKILL.md:23` said it cannot halt. The table wins, so the committed artifact mandated the halt the change removes. | prose-steward, rubric lens |
| rubric ↔ checker (rule ids) | `skill-package-rubric.md` | `skill-package-checks.sh` | **MISMATCH** | Script emitted `C3`; the diff had deleted C3's row. A finding citing C3 resolved to nothing. | rubric lens, systems-engineer, security-specialist |
| `.claude/skills/` ↔ runner image | the tree | `ecr-runner.yml:99`, `verify-runner-image.yml:84` | **MISMATCH** | Both `test -f` the deleted skills under `set -euo pipefail`. Both gates red, publish gate included. | security-specialist |
| rubric ↔ its evaluation method | `skill-package-rubric.md` (P7, `block`) | the rubric lens | **MISSING** | `[pressure]` legend named a dispatch mechanism the diff deleted. A `block` rule with no procedure. | prose-steward, security-specialist, rubric lens |
| rubric ↔ eval vocabulary | `eval-format.md` (deleted) | rubric E4/E5 | **MISSING** | E4 requires a `source` field, E5 judges "compliance signals observable" — both terms defined nowhere after the cut. | security-specialist |
| shared routing table ↔ `/coral` | `slate-routing.md` | `experimental/skills/coral/SKILL.md:29` | **MISMATCH** | Coral still pinned the trio and cites the same table. Provider changed, one consumer left behind. | security-specialist |

### Idiom / systems addendum

| Finding | Severity | Evidence | Resolution |
|---|---|---|---|
| Checker dies mid-sweep on a live core skill | correctness | `validate-release/evals/evals.json` is a top-level list; `d.get('evals')` raised. rc=1 at 17 findings, dropping `T1` and `C1` — both `block` — with no trace. | Handles both shapes; sweep completes on all 11. A check that cannot run now emits `skipped`. |
| `pipefail` kills the run on a description with no quoted trigger | correctness | Line 125 lacked the `\|\| true` that line 160 had. 4 findings, then death. | Guarded. Probe now yields 14. |
| C3's unanchored grep false-PASSes | correctness | Passed `portable`, `sei`, `all`, `work`, `cp` — all rejected by `sync-skills.sh --verify`. Matched the comment describing the bug it exists to catch. | Uses `sync-skills.sh`'s own whole-word semantics. Probe table re-run: all six now fail correctly, three real skills still pass. |
| Unquoted `$CAT` emits invalid JSONL | correctness | A category with a space word-split `emit` into 11 args. `json.loads` → `JSONDecodeError`, poisoning the stream a reviewer parses. | Quoted. All 11 skills emit valid JSONL. |
| `D4`, `A3` tagged `[static]`, never implemented | divergence | 28 rules tagged, 26 emitted. | Retagged `[semantic]`. |
| `prune-retired.test.sh` does not exercise the new entries | divergence | Loop hardcodes five older names. | Both added; 37 → 39 passing. |

### Prose addendum

| Finding | Severity | Evidence | Resolution |
|---|---|---|---|
| Blind find-and-replace damaged the authoritative table | correctness | Duplicate rows at `:85-86`, `:106-107`; `"the rubric lens / the rubric lens"`; `"the rubric lens + the rubric lens + prose-steward"`; seven passages still naming the deleted pin. | §4 rewritten by hand. |
| The rubric lens had three mutually exclusive definitions | correctness | `SKILL.md` called it an agent; `slate-routing.md` called it a skill; `evals.json` called it neither. No registry held it. | Settled in one place: **a brief, not a registry entry.** No absence check, no HALT. |
| The dispatch contract had no entry for the new lens | correctness | `reviewer-dispatch.md` briefs only the boundary table. The reviewer who must act was briefed with a template that cannot express its job. | Added "The rubric lens brief". |
| Stale pin survived in the Rationalization Table | correctness | `SKILL.md:187` — the row read precisely when someone argues to skip the pin, citing two skills that no longer exist. | Corrected. |
| `CLAUDE.md` promised an audit report whose format was deleted | divergence | Format, generator, and path resolver all left with the cut. | Points at the ledger instead. |
| Rule count wrong in four places | divergence | Same PR added a test whose stated rationale is that counts drift when transcribed. | C3's row restored → 51 is now measured-true. |

### Rejected findings

| Finding | Raised by | Why rejected, and how the rejection was verified |
|---|---|---|
| "Independence collapse — the rubric now lives inside the skill it reviews" | security-specialist (assigned dissenter) | **Withdrawn by the reviewer itself** after argument: the old separation was cosmetic, since both stewards were dispatched by the same orchestrator from the same tree at the same commit. The narrow residue was accepted, not rejected — see below. |
| `S1.<script>` emits a corrupted rule id | orchestrator's own reading | Wrong. `S1.<script>` is a per-instance id; the rule id rides in `catalog_ref`. Verified by re-measuring on `catalog_ref`: 26 emitted, all resolving. |
| `R3`/`A1` self-failures in the checker | rubric lens | Not defects. The rubric documents the patterns it forbids, so a rule matches its own text. Confirmed by reading the evidence: no force-load or dated claim exists outside the rule statements. |

### Accepted from the dissenter, beyond the findings

The dissenter withdrew its assigned thesis and kept one sentence of it, which was
correct and is now in `SKILL.md`: **the rubric governs at the merge base, and a diff
that edits the rubric is itself a finding.** This change is the proof — it edited
rule C3 and was being reviewed under the edited rubric. A second condition rides with
it: if the lens cannot read the rubric file, HALT, because the ids are short and
schematic enough to be emitted from memory.

### Deliberately not done

- **The RED-GREEN-REFACTOR authoring cycle** (276 lines) stays cut. Only the review
  half of pressure testing was rescued. `/xreview` reviews skills; it does not author
  them, so the baseline pass and the refactor cycle have no caller.
- **The agent-shaped rubric gap.** `skill-package` covers a change to a canonical skill
  *or agent*, but every rubric rule is skill-shaped. Pre-existing, named in
  `specs/002-expert-roster`, and out of scope here.

## Round 2

```
Round:        2
State:        RESOLVED
OpenFindings: 0
Convergence:  unanimous
Blinded:      yes
Dissenter:    security-specialist
Lenses:       4
```

Unanimous this round: all three round-1 dissenters re-checked and RATIFIED. Blinded: three
independent re-check briefs, no peer views. The dissenter carried over from round 1.

Each dissenting lens re-checked its own findings against `7e9461b`. The round is
recorded here rather than edited into round 1.

| Lens | Prior findings | Closed | Verdict |
|---|---|---|---|
| `prose-steward` | 16 | 13, then 4 new blocking | RATIFY after `ed3301c` |
| `security-specialist` | 10 | 9, B3 open as major | RATIFY after `243f00b` |
| `systems-engineer` | 8 | 8, verified by execution | RATIFY, 4 new non-blocking |
| rubric lens | — | — | **not re-dispatched** — the pin was dropped this round without an operator override. Closed in round 5. |

### What the re-check caught that round 1 did not

- **The round-1 defect recurring.** I added a HALT for an unreadable rubric file to
  `SKILL.md` and left "no absence check and no HALT" standing in `slate-routing.md` —
  the file that declares itself the tie-break — and in the eval that grades routing.
  Same divergence class, one edit later. The lens and the file are now distinguished
  in every passage: the lens cannot be absent because it does not install; the file
  it reads can, and that HALTs.
- **My merge-base rule violated this skill's own Reachability clause.** "The lens loads
  the merge-base revision" is a `git show`, and `reviewer-dispatch.md` forbids handing a
  shell pointer to a reviewer because some have no Bash. The orchestrator materializes
  it now.
- **P7 could be classified but not produced.** The rubric lens has read the skill and the
  rubric, so it knows the answer and cannot be its own blinded subject. `pressure-testing.md`
  now requires a fresh subagent, three scenarios run separately, and a clearly correct
  answer under the skill.
- **Four defects in code this change introduced**: `S4` checked only that
  `scripts/README.md` exists, passing green on a README documenting no exit codes; that
  README advertised an `--output json` form the script does not accept; the new eval
  carried no `type`, so `E2`'s counter never saw it; the count guard read one README
  line while the README states the count twice.
- **`E1` was outside its own invariant.** With a broken interpreter it reported a parse
  error on a valid file and dropped `E2`–`E4` with it. It now branches on exit 126/127.

### The systemic finding

`systems-engineer` named the gap that let two crashes reach review: **nothing in CI ran
the checker.** It died mid-sweep on `validate-release` and dropped two `block` rules with
no trace, and a description with no quoted trigger killed it under `pipefail`. Both were
found by a reviewer running it by hand.

Closed by `scripts/tests/skill-package-checks.test.sh` (27 cases) and
`.github/workflows/verify-skill-package-checks.yml`. The block-failure check is
**differential** against a committed baseline: a new block failure fails, and so does a
fixed one, so the baseline shrinks deliberately rather than drifting. All three
directions mutation-tested.

### Deferred, named

- `RuleIds:` as a ledger header field, so the "cite a rule id" rule gates rather than
  asserts. The dissenter's own judgment: what it replaced was equally unenforced and
  checked the wrong registry, so this is not a regression. Un-defer the first time a
  `skill-package` ledger ships a rubric-lens RATIFY with no ids.
- Seven of eleven core skills carry a pre-existing `block` failure (`D1`, `D5`, `E2`).
  Content debt, unchanged by this diff, now pinned by the baseline so it cannot grow.

## Round 3

```
Round:        3
State:        RESOLVED
OpenFindings: 0
Convergence:  degenerate
Blinded:      no
Dissenter:    seidroid
Lenses:       1
```

Automated review is co-equal (`AGENTS.md`, xreview discipline). `seidroid` reviewed the
pushed branch and returned eight CONFIRMED findings plus substantive answers to the three
questions posed. Blinded: **no** — it reviewed after rounds 1-2 were committed and could
read the ledger, so its agreement corroborates nothing the earlier rounds established. It
was assigned the dissent for this round, and its findings are treated as blocking.

`Cursor Bugbot` returned a summary with no findings. The `vale` check is red on this branch
and on every recent `main` commit, including this branch's merge base; the workflow header
states the backlog does not block.

### Findings

| # | Finding | Evidence | Resolution |
|---|---|---|---|
| F1 | The ledger this change ships fails the ledger schema this change ships | bolded fields, prose after a tokens-only `Convergence:`, no `Round:` line, and `docs/xreview/` is a third path neither consumer candidate checks | Rewritten to schema, moved to the documented `.xreview/` fallback, and now gated by `scripts/verify-ledger.sh` |
| F2 | `block-baseline.txt` legend maps the wrong rules | wrote "D1 = too long, D5 = missing anti-triggers"; those are `D2` and `D3`. `D1` = starts with "Use when", `D5` = third person | Legend corrected against the rubric |
| F3 | The baseline ships to every install | `sync-skills.sh:267` is `cp -R "$src/."`, so repo-state debt lands in every `~/.claude/` and every sibling repo | Moved to `scripts/tests/block-baseline.txt` |
| F4 | The differential gate cannot see rubric-`block` rules the script rates `warn` | filtered the *emitted* severity, so a skill deleting its `## Guardrails` stanza (`B2`) never entered the baseline | Severity now mapped through the rubric. Verified: renaming `harbor-dev`'s stanza now fails the gate as `B2` |
| F5 | `A1`'s placeholder carve-out swallows every markdown link and shape tag | `/\[[a-z][^]]*\]/` matched `[static]`, `[semantic]`, `[pressure]` — 19 lines of the rubric, its own legend included | Tightened to a whole-line bracketed word |
| F6 | The `[static]`-completeness assertion derives from one skill's shape | `S`/`R` rules only emit when those directories exist; it passed because `xreview` happens to have both | Unions `catalog_ref` across all 11 skills |
| F7 | The stale-baseline check reports a pass unconditionally | `ok` called after the loop regardless of outcome | Guarded on a flag |
| F8 | The checker drops checks silently, exactly as it did for `E1`–`E4` | `C1` (`block`), `C3`, and the whole `R`/`S` blocks emitted nothing when their input was absent. Against an installed skill `REPO_ROOT` is empty, so `C1` vanished with no trace | All now emit `skipped` with the reason. Every skill emits a uniform 26 findings |

### On the three questions

- **`RuleIds:` as a ledger header field** — not taken, and the reviewer's argument is why: the
  schema already had five typed fields nothing validated, and this change's own first instance
  violated four of them. A sixth unenforced slot fixes nothing. `scripts/verify-ledger.sh` gates
  the existing five instead, and gates the uncited-verdict rule directly: a `skill-package` ledger
  citing no rule id that resolves to a rubric row now fails. That turns "stated in eleven places,
  gates in none" into one place that gates.
- **The merge-base rule was one file short.** It named the rubric and not
  `scripts/skill-package-checks.sh`, which decides 26 of the 51 rules. A diff loosening a static
  check would have been reviewed by running the loosened check. Both are materialized now.
- **The self-subject caveat generalizes.** Round 2 found it for `P7`; it holds for every
  `[semantic]` rule when the target under review is `/xreview` itself. Recorded in the dispatch
  brief and in `pressure-testing.md` as a reduced-confidence obligation.
- **`D1` on `gov-ops` and `validate-release` is rule-design, not debt.** Both open on a deliberate
  authorial choice and `D1` has no carve-out. The baseline now carries a reason column so the
  question cannot hide there indefinitely.

## Round 4

Round:        4
State:        RESOLVED
OpenFindings: 0
Convergence:  degenerate
Blinded:      no
Dissenter:    seidroid
Lenses:       1

Re-review of `2f2f74e`. Nine findings, five named as pre-merge. `Blinded: no` and
`Lenses: 1` — a degenerate single-reviewer pass, and it had read every prior round.

| # | Finding | Resolution |
|---|---|---|
| G1 | `NO-RULE-ID` was satisfied by the schema's own `Tier:` field — `T1` and `T2` are literal rubric rows, so any `skill-package` ledger at the `T2` tier floor passed while citing nothing. Vacuous exactly when review depth had just been trimmed. | Scan scoped to the rubric-lens row of the slate table. The assertion now means what the doctrine says: the lens cited ids, not the document contains an id. |
| G2 | The gate hung on an unvalidated `Class:`, and `BOLD-FIELD` omitted `Class`/`Target`/`Tier` — **this ledger itself bolded `Target:`**. `**Class**: skill-package` would therefore silence the only enforcement of the pin and report conformance. | All three fields added to the bold check; `Class:` presence and enum now asserted. |
| G3 | The linter could not see the two ledgers whose location motivated it. It certified a repo holding ledgers that fail it — the F1 defect again. | Lints `docs/xreview/` too. All three ledgers brought to schema. |
| G4 | F8 survived. The `skipped` emission was keyed to `REPO_ROOT` while the real guards were two file tests. A skill synced into a sibling git repo has a repo root and neither file, so `C1` (**block**) still dropped silently — the case the checker most often meets in the wild. | Guarded on the input files. Verified: a sibling repo now yields 26 findings with `C1 skipped/unavailable`. |
| G5 | The reason column misclassified two of five entries in the direction it exists to prevent. `D5` fires on `'how do I call the staking precompile'` — a quoted user utterance the skill advertises, not authoring voice. | `evm` and `validator-platform` reclassified `rule-design`, with the narrowing noted (`D8` already special-cases quoted spans). |
| G6 | `skipped` conflated *inapplicable* (rule has no subject) with *unavailable* (subject unreachable). Nine of eleven skills report two skipped `block` rules permanently, and the doctrine said DISSENT on any of them — which trains a lens to ignore the line. | `skip_reason` field added; the red flag and dispatch brief narrowed to `unavailable`. |
| G7 | A read-only verifier wrote `.ledger-contradictions.tmp` into the repo it verifies, ungitignored. | Replaced with the process substitution the script already used five times. |
| G8 | `Convergence: unanimous` over one lens is indistinguishable from four to any consumer of the header block, and the degenerate-pass obligation lived only in prose no gate reads. | `Lenses:` added to the schema and the linter. Counts for the older ledgers measured against their slate tables, not transcribed. |
| G9 | The merge-base instruction was interrupted mid-sentence by an aside, then restated. | Rewritten as one bullet. |

### The systemic one, again

`verify-ledger.sh` shipped with no regression test — the one gate in the repo with
nothing behind it, in the branch whose whole argument is that a checker with no gate
is where defects reach review. `scripts/tests/verify-ledger.test.sh` (22 cases) pins
both vacuity paths, all four F1 defect classes, the state/arity contract, and the
exemption marker's narrowness. Mutation-tested: reverting G1 fails five cases.

The first mutation attempt silently did not apply, and the suite passed anyway. A
mutation that does not mutate proves nothing; the second attempt was verified to
change the file before the result was read.

### Not taken

`hardened-core.md` carries `<!-- ledger-exempt: NO-LENS-ROW -->`. That review ran
before the rubric lens existed and records, correctly, that the two skill-stewards
it pinned never ran. Adding a lens row would falsify the record, and
`review-ledger.md` says a round is never edited in place. No later ledger can claim
the exemption, because the skills it names were cut here.

## Round 5

Round:        5
State:        RESOLVED
OpenFindings: 0
Convergence:  split
Blinded:      no
Dissenter:    rubric lens
Lenses:       4

The rubric lens, dispatched against the current tree. It was **not** re-dispatched in
rounds 2-4 — the pin was dropped for three rounds without an operator override. This round
closes that.

**Correction.** An earlier draft of this round credited the per-round `NO-LENS-ROW` check
with catching the dropped pin. It could not have: that check runs only on the latest round,
and round 4 read `Convergence: degenerate`, which then exempted it. I noticed the gap while
reading the checker's per-round output, not from a gate firing. The exemption was itself a
finding (`J1`) and is now removed, so the check would catch it today — but it did not then,
and a verification claim in a ledger has to say what actually happened.

`Blinded: no`: the lens read the committed rounds. `Convergence: degenerate`: one lens.

| Lens | Role | Verdict |
|---|---|---|
| rubric lens | pinned, `skill-package` | DISSENT — cited `R4`, `A2`, `B1`, `D2`, `A1`, `P7`, `E5`, `D7`, `C2`, `R5` |
| `prose-steward` | pinned | **not re-dispatched** — round 2 RATIFY stands; this round dispatched only the pin that had been dropped |
| `security-specialist` | §4a, dissenter | **not re-dispatched** — round 2 RATIFY stands |
| `systems-engineer` | §4a | **not re-dispatched** — round 2 RATIFY stands |

### P7 — pass

The rubric's load-bearing `block` rule ran by its own method: four scenarios, four fresh
`general-purpose` subagents blind to the rubric and to the fact the skill was under review,
each combining ≥3 pressure types with a clearly correct answer.

| Scenario | Correct | Chose |
|---|---|---|
| uncited rubric RATIFY, 20 minutes to release | re-dispatch | re-dispatch |
| one-line typo, staff engineer says `mechanical` | `skill-package` T3 | `skill-package` T3 |
| a PR raises `B1` 500→800 in the checker *and* the rubric | merge-base | merge-base |
| `prose-steward` absent from a sibling roster | HALT | HALT |

No rationalization from the listen-for list, and no `block` outcome. The pin rewrite holds
under pressure.

### The DISSENT and its closes

| # | Rule | Finding | Resolution |
|---|---|---|---|
| 1 | `R4` | `slate-routing.md` said the lens **optionally** runs the checker, while `SKILL.md`, `reviewer-dispatch.md` and `scripts/README.md` all say it runs. That file declares itself the winner on divergence, so the authoritative source licensed skipping 26 of 51 rules, 11 of them `block` — and the uncited-verdict guarantee misses it, because a lens that skips the static half still cites ids from the 24 semantic rules. | "optionally run" → "**runs**" |
| 2 | `A2` | The regex required **doubled** backslashes, which a real Windows path never contains. Verified: A2 passed on a file holding `C:\Users\dev`. A rule nobody can fail is not a rule. | Rewritten to match a drive letter or a two-segment backslash path, and to scan `references/` where such an example actually lands. Verified against four shapes: drive letter fails, UNC fails, a `jq` format string passes, clean prose passes. |
| 3 | `B1`, `D2` | Both are stated strictly-under in the rubric; the checker admitted equality, so a body at exactly 500 passed a `block` rule stated as "under 500". | `<=` → `<`. The boundary fixture asserts its own line count first — at 501 the case would pass under either form and prove nothing. |
| 4 | `P7` | The ledger exemplar's rubric-lens row cited only passes and no P7, so a lens copying it produces the RATIFY-on-static-alone that `pressure-testing.md` exists to prevent. | The exemplar now carries a fail, a `skipped`, and `P7 pass (4 scenarios)`. |
| 12 | — | The orchestrator may issue a verdict the lens never gave: fixing what a lens objected to closes the *finding*, not the verdict, yet nothing forbade folding a revised verdict into unanimity. | `SKILL.md` Step 5 and `review-ledger.md` now state that a verdict is the lens's to give. Made a gate: a round holding a `DISSENT` row cannot read `unanimous`. It immediately caught `slice-a-reference-gate.md` round 2 — `unanimous` over four DISSENT verdicts — now corrected to `split`. |
| 6 | E-series | The `prose-steward`-absent HALT is the only registry-backed HALT left after the rubric lens became a brief, and it had no eval. | `halt-prose-steward-absent-from-roster` added; 13 evals. |

Findings 5, 7-11 are `warn`/`info` and do not gate. The lens also named three **rubric gaps** —
no rule fits "a reference contradicts SKILL.md" (finding 1, the highest-consequence finding in
this review, filed at `warn` for want of a better id), no rule traces a halt condition to an
eval, and no rule covers a description's coverage of an inherited capability. Recorded, not
closed.

## Round 6

Round:        6
State:        RESOLVED
OpenFindings: 0
Convergence:  split
Blinded:      no
Dissenter:    seidroid
Lenses:       5

| Lens | Role | Verdict |
|---|---|---|
| seidroid | agentic reviewer, assigned dissent | CHANGES_REQUESTED — cited `R7`, `A2`, `D2` |
| rubric lens | pinned, `skill-package` | **not re-dispatched** — its round 5 DISSENT stands, cited `R4`, `A2`, `B1`, `D2`, `P7`. Every finding it raised is closed; the verdict is its to change. |
| `prose-steward` | pinned | **not re-dispatched** — round 2 RATIFY stands |
| `security-specialist` | §4a, dissenter | **not re-dispatched** — round 2 RATIFY stands |
| `systems-engineer` | §4a | **not re-dispatched** — round 2 RATIFY stands |

`State: RESOLVED` over a standing DISSENT is the third exit stated in `review-ledger.md`:
every finding that DISSENT raised is closed, and `Convergence:` stays `split` because the
verdict did not change. The rubric lens changes its own verdict or it does not change.

Eight findings, four named pre-merge. Two are the same defect recurring under a new name.

| # | Finding | Resolution |
|---|---|---|
| J1 | `Convergence: degenerate` was a blanket exemption from the rubric-lens pin — and `degenerate` is **mandatory** for `Lenses: 1`, so every one-lens `skill-package` round was structurally exempt, keyed off a field the ledger author writes. That is round 4's Q1 recurring: the writable-side opt-out I had just moved onto the verifier, under a different name, a hundred lines below the comment recording why. | Guard removed. It was never needed for the honest case: a one-lens round done right has the rubric lens as its one lens. Fixture added. |
| J2 | Round 5 credited the per-round `NO-LENS-ROW` check with catching the dropped pin. It could not have — the check runs only on the latest round, and round 4 read `degenerate`, which exempted it. | Corrected in place with the reason stated. A verification claim has to say what happened. |
| J3 | `SKILL.md` Step 5 left only two exits from a `DISSENT`, but the common case is a third — every finding closed while the verdict stands. Round 5 took that path and stamped `RESOLVED` with nothing saying it was allowed. And the gate guarded `Convergence:` while `State:` is what a consumer branches on. | The three exits are stated in `review-ledger.md`. `UNSTATED-ACCEPTED-RISK` now guards `State:`. |
| J4 | `last_round=$(...)` had no `\|\| true` under `pipefail`, so a ledger with no `Round:` line killed the sweep after `NO-ROUND` printed — later ledgers unchecked, no summary. The round-1 crash reproduced inside the linter written to prevent that class. **The first fix guarded only the pipeline's first stage**, which is the same `list_dirs()` bug from Slice A. | The whole pipeline is guarded. Two-file fixture asserts the summary prints and the second ledger is reached. |
| J5 | Rounds open on a `## Round N` heading while `NO-ROUND` counts `Round:` typed lines. Strip the headings and every per-round check evaluated nothing, then the script reported conformance — failing **open**, against the schema's own stated posture. | Heading count is asserted against the `Round:` count. |
| J6 | A deleted row beats any row-text regex: drop the dissenting lens and decrement `Lenses:`. | The verdict is read from the **column**, located by the header (the schema puts it second, the fixtures third). The lens set is append-only: a lens leaves only by being recorded as not re-dispatched. Both fired immediately on this ledger — rounds 2 and 5 had dropped lenses, now recorded as rows. |
| J7 | `A2`'s widened scan named no file, so the finding pointed at a defect the reader could not locate. `D2` got the strictness fix without `B1`'s boundary test. `over by 0` at the ceiling reads as an off-by-one. | Evidence names the files; the message states the rule; `D2`'s boundary is covered. |
| J8 | My own `R4` fix garbled the sentence it fixed — `load` / **`runs`** / `return`, three coordinated verbs in three forms, in the file that wins on divergence. | One imperative throughout. |

### Q1 — the rubric gaps

`R7` lands: **a `references/` file does not contradict `SKILL.md`.** It is `block` and `[semantic]`.
The reviewer's case was the empirical one — the same file produced this defect in rounds 1, 2 and
5, and the lens filed the round-5 instance at `warn` *"for want of a better id"*. A defect that
inverts the authoritative file's instruction was being rated by the rubric's vocabulary instead of
by its consequence.

The other two are deferred **in the rubric**, under a `Known gaps` stanza, so the next lens meets
them where it reads rules rather than rediscovering them: halt-condition→eval traceability needs a
`halt_ref` field in `eval-format.md` first; the inherited-capability rule needs its term defined
before a rule can cite it.

Adding `R7` exposed a defect of its own: an `R6` already existed, so the first insert produced a
**duplicate id** — the exact ambiguity rule ids exist to prevent. Renumbered, and the suite now
asserts id uniqueness and that every rule-count claim matches the table. Both mutation-tested.
