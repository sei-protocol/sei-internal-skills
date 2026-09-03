# Review ledger — cutting `author-skill` and `audit-skill`

**Target**: branch `refactor/cut-skill-authoring-pair` (`a9e512e`, `7e9461b`) —
deletes `.claude/skills/author-skill/` and `.claude/skills/audit-skill/`, moves the
skill-package rubric and its static checker into `/xreview`, rewrites the steward pin.

**Class**: `skill-package`
**Tier**: T3

**Note**: belongs in the DRI's designs repository under
`designs/sei-agentic-mesh/xreview/`. It sits here beside its sibling pending that move.

## Round 1

**State**: RESOLVED
**OpenFindings**: 0
**Convergence**: split — 3 DISSENT, 1 RATIFY-with-advisory
**Blinded**: yes — four independent briefs, no peer views
**Dissenter**: `security-specialist` (assigned)

### Slate

| Lens | Why pinned | Verdict |
|---|---|---|
| `prose-steward` | agent-steward, `skill-package` unconditional | DISSENT |
| rubric lens | `skill-package` unconditional | DISSENT |
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

**State**: RESOLVED
**OpenFindings**: 0
**Convergence**: unanimous — 3 RATIFY, after 3 DISSENT in round 1
**Blinded**: yes — three independent re-check briefs, no peer views
**Dissenter**: `security-specialist` (assigned, carried from round 1)

Each dissenting lens re-checked its own findings against `7e9461b`. The round is
recorded here rather than edited into round 1.

| Lens | Prior findings | Closed | Verdict |
|---|---|---|---|
| `prose-steward` | 16 | 13, then 4 new blocking | RATIFY after `ed3301c` |
| `security-specialist` | 10 | 9, B3 open as major | RATIFY after `243f00b` |
| `systems-engineer` | 8 | 8, verified by execution | RATIFY, 4 new non-blocking |

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
