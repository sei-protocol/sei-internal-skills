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
