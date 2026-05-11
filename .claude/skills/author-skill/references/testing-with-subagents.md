# Testing Skills with Subagents — RED-GREEN-REFACTOR

Source: <https://github.com/obra/superpowers/blob/main/skills/writing-skills/testing-skills-with-subagents.md>. Adapted for author-skill's procedure.

## The Iron Law

> **If you didn't watch an agent fail without the skill, you don't know if the skill prevents the right failures.**

The RED phase is non-negotiable. A skill written from training data alone is documentation Claude *might* follow under ideal conditions. A skill written against captured rationalizations is documentation Claude *does* follow under pressure.

## The cycle

| TDD concept | Skill creation |
|-------------|----------------|
| Test case | Pressure scenario with subagent |
| Production code | SKILL.md and references/ |
| Test fails (RED) | Agent violates rule without skill (baseline) |
| Test passes (GREEN) | Agent complies with skill present |
| Refactor | Close loopholes while maintaining compliance |
| Write test first | Run baseline scenario BEFORE writing skill |
| Watch it fail | Document exact rationalizations agent uses |
| Minimal code | Write skill addressing those specific violations |
| Watch it pass | Verify agent now complies |
| Refactor cycle | Find new rationalizations → plug → re-verify |

## RED phase — baseline

**Goal:** Capture verbatim rationalizations that a competent agent would use to bypass the constraint the skill is supposed to enforce.

**Procedure** (in author-skill, this is Step 7):

1. Pick 3 pressure scenarios from `pressure-scenario-templates.md`. Each combines ≥3 pressure types (see below).
2. For each scenario:
   - Dispatch a `general-purpose` subagent via `Agent` (with `isolation: worktree` if file edits are involved).
   - **The subagent does NOT have the skill loaded.** It works from training data and the scenario only.
   - Capture the subagent's response, decision, and stated reasoning *verbatim* into `state/run-<ts>/red-baseline.md`.
3. Look for patterns: which rationalizations show up across scenarios? Those are the load-bearing ones.

**Pressure types** (combine for realistic baselines):

- **Time** — deadline 30 min away, release window closes at 4pm, customer waiting.
- **Sunk cost** — already invested 6 hours, already merged the migration, already told the user it's done.
- **Authority** — senior engineer / staff engineer / VP says "just do X, we'll fix it later."
- **Exhaustion** — end of day, third on-call rotation, fourth iteration on the same problem.
- **Social** — appearing inflexible, blocking another team, getting cc'd by a frustrated stakeholder.
- **Ambiguity** — the rules don't *quite* fit this case; technically maybe this counts as an exception.

Each scenario should include concrete file paths, realistic consequences, and A/B/C choice points. **Not** "What would you do if..." — those are too abstract to produce real rationalizations.

## GREEN phase — verify the skill works

**Goal:** Confirm the same subagent, given the same scenario but with the skill loaded, complies with the constraint.

**Procedure** (Step 8):

1. Write the SKILL.md and references/ addressing the specific rationalizations captured in RED.
2. Re-dispatch the same subagents on the same scenarios, this time with the skill in their context.
3. Verify the subagent:
   - Cites the skill explicitly (a strong signal it read the skill).
   - Acknowledges the temptation but complies anyway (a strong signal the skill addresses the specific rationalization).
   - Chooses the correct option from the A/B/C.

**Sub-strong signals** that the skill is bulletproof:

- The subagent quotes the skill's rationalization table.
- The subagent self-corrects mid-response ("wait, that's exactly the loophole the skill closes").
- The subagent confirms the skill was clear on meta-test ("the skill was unambiguous, I should comply").

## REFACTOR phase — close loopholes

**Goal:** Capture rationalizations that the GREEN-phase subagents *still* produced despite the skill, and close them.

**Procedure** (Step 9):

1. Read the GREEN-phase responses carefully. Did any subagent comply *but* in their reasoning hint at a loophole? ("I'll comply this time, but if X were true I'd consider Y.") Those are the next iteration's RED.
2. Add explicit counters to the SKILL.md:
   - A rationalization table row for the new excuse.
   - A red-flags list addition.
   - A "no exceptions" clause that names the specific workaround.
3. Re-run the scenarios. The new compliance should hold.
4. Cap at **3 cycles** in author-skill. If REFACTOR isn't converging by cycle 3, the skill design has a structural problem — halt and surface.

## Persuasion principles — which to apply per skill type

(See `persuasion-principles.md` for the full breakdown.)

| Skill type | Use | Avoid |
|------------|-----|-------|
| Discipline-enforcing (TDD, verification, refuse-on-X) | Authority + Commitment + Social Proof | Liking, Reciprocity |
| Technique (how-to) | Moderate Authority + Unity | Heavy authority |
| Pattern (mental model) | Unity + Commitment | Authority |
| Reference (API, syntax) | Clarity only | All persuasion |

Authority language ("YOU MUST", "Never", "Always", "No exceptions") reduces rationalization by removing decision fatigue. Commitment language ("Announce the skill when you find it", "TodoWrite the checklist before starting") creates a public action that's hard to undo. Social proof ("checklists without TodoWrite tracking = steps get skipped, every time") establishes a norm.

## Common rationalizations to expect during RED

These show up across domains. If the baseline doesn't produce at least one of these, the pressure scenarios are too weak — escalate.

| Excuse | Counter to write into the skill |
|--------|---------------------------------|
| "This is a special case." | "Violating the letter of the rules is violating the spirit of the rules." |
| "I'll do it after." | "After = never. The rule fires now or it never fires." |
| "The user is in a hurry, the rule is overhead." | "The rule exists because the cost of skipping it is higher than the time it takes. Both are documented above." |
| "I'm 99% sure this case is safe." | "1% of N invocations = catastrophic failure. The rule covers the 1%." |
| "I've already invested too much to start over." | "Sunk cost is sunk. The choice now is between (a) finishing the wrong thing and (b) finishing the right thing. The cost gap is the marginal step, not the sunk cost." |
| "The senior person told me to skip it." | "Authority doesn't override constraints encoded into the skill. If the senior person disagrees, the escalation is a constitution-level edit to the skill, not a one-off override." |

## Meta-test

After REFACTOR converges, ask the subagent (with the skill loaded):

> "Was the skill clear on what to do in this scenario? If not, what was ambiguous?"

If the subagent says "the skill was clear", that's a bulletproof signal. If the subagent identifies an ambiguity, that's the next REFACTOR cycle.

## Test across models

(Optional in author-skill's procedure but strongly recommended for skills that will ship at user-scope.)

Run the GREEN-phase scenarios with subagents at different model tiers. A skill bulletproof on Opus may be ambiguous on Haiku. The model-dependent gaps are the next round of REFACTOR.
