# Skill Shapes — Choosing the Right Form

Skills come in distinct shapes. Each shape implies different content, persuasion style, and testing approach. Pick the shape **before** drafting; the wrong shape produces a skill that misses its mark.

## The four shapes

### 1. Discipline-enforcing

**What:** Enforces a rule under pressure. The agent would naturally skip the rule without the skill; the skill exists specifically to make the rule survive rationalization.

**Examples:** TDD, verification-before-completion, "no merge without review", refuse-on-mocked-data-tests.

**Persuasion stack:** Authority + Commitment + Social Proof. Heavy use of "YOU MUST", "Never", "No exceptions". Rationalization table is mandatory. Red-flags list is mandatory.

**Testing:** RED-GREEN-REFACTOR with maximum-pressure scenarios (combine time + sunk cost + authority + exhaustion). The skill is bulletproof when the agent picks the right option *under all combined pressures*.

**Body structure:**
- Overview (1-2 sentences)
- The rule itself (in imperative form)
- Rationalization table
- Red-flags list
- Examples of correct vs. violating behavior

### 2. Technique

**What:** A concrete how-to with steps. The agent knows roughly what they want to do but lacks the specific procedure that makes it work reliably.

**Examples:** Condition-based waiting in async tests, root-cause-tracing for production bugs, defensive-programming patterns.

**Persuasion stack:** Moderate Authority + Unity. "Always do X" for steps that can't be skipped, "Prefer Y over Z" for judgment calls. The agent should *adapt* the technique to the context, not mechanically apply it.

**Testing:** Application scenarios. Give the subagent a new problem; verify they apply the technique correctly. Variation scenarios test edge cases. Missing-information scenarios reveal gaps.

**Body structure:**
- Overview + when to use
- Core pattern (before/after if applicable)
- Step-by-step procedure
- Common mistakes
- One excellent example

### 3. Pattern

**What:** A mental model for thinking about problems. The agent needs to *recognize* when the pattern applies, then *apply* it. The skill is half-recognition, half-procedure.

**Examples:** Reducing-complexity-with-flags, information-hiding, separation-of-concerns.

**Persuasion stack:** Unity + Commitment. "We use this pattern because...", "When you see X, choose this pattern over Y."

**Testing:** Recognition scenarios (does the agent see the pattern in a new context?), application scenarios (can they use the mental model?), counter-examples (do they know when NOT to apply?).

**Body structure:**
- Overview
- When the pattern applies (with concrete signals)
- The mental model (1-2 sentence core)
- Application example
- Counter-examples (when not to apply)

### 4. Reference

**What:** API docs, syntax guides, tool documentation. The agent knows what they want; they just need the specific incantation.

**Examples:** Office docs (pptxgenjs API), CLI command references, library cookbooks.

**Persuasion stack:** Clarity only. **No persuasion.** Authority language in reference content makes it harder to read, not easier.

**Testing:** Retrieval scenarios (can they find the right entry?), application scenarios (can they use what they found?), gap testing (are common cases covered?).

**Body structure:**
- TOC
- One section per item, alphabetical or by use case
- Each entry: signature, parameters, return value, one example
- Cross-references to related entries

## Tide-local procedural shape

Tide's `SKILL-TEMPLATE.md` describes a *procedural skill* — a skill that executes a fixed sequence of steps with side effects on external systems. Procedural skills are usually a hybrid of discipline (the guardrails) and technique (the procedure).

When the proposed skill matches the procedural shape (has external side effects, has scripts, has state), use the SKILL-TEMPLATE.md structure:

- `SKILL.md` — frontmatter, guardrails stanza first, preconditions, numbered procedure, halt conditions, state management, output format.
- `scripts/` — one logical step per script.
- `references/` — guardrails.md, summary-template.md if there's an artifact, plus domain references.
- `evals/evals.json` — happy-path + halt-condition.
- `state/` — gitignored, per-run subdirectories.

When the proposed skill is purely a technique/pattern/reference (no side effects, no state), use the Obra-flat structure:

- `SKILL.md` — frontmatter, overview, when-to-use, core pattern, quick reference, implementation.
- One-level references for heavy detail.

## Decision: which shape is this skill?

Ask the user (or yourself) these questions in order:

1. **Does the skill enforce a rule the agent would naturally skip under pressure?** → Discipline-enforcing.
2. **Does the skill execute a fixed procedure with external side effects (cluster, contracts, deployments)?** → Procedural (Tide template).
3. **Does the skill teach a concrete how-to that the agent should adapt to context?** → Technique.
4. **Does the skill teach a way of thinking about problems?** → Pattern.
5. **Does the skill provide lookup material the agent retrieves on demand?** → Reference.

If two shapes apply, the heavier shape wins (Procedural beats Technique; Discipline beats Pattern). Hybrids are fine — note the secondary shape in the SKILL.md overview so future readers know which testing approach applies to which section.

## When Obra and SKILL-TEMPLATE.md conflict

- **Description style** — Obra wins (Use-when, no workflow summary). The Tide skills predate Obra's CSO insight; the description trap is real, lean toward Obra.
- **Directory structure for procedural skills** — SKILL-TEMPLATE.md wins. The scripts/state/evals layout encodes Tide's safety conventions and the cross-repo sync pipeline.
- **Directory structure for technique/pattern/reference skills** — Obra wins. Flatter layout, fewer ceremonies.
- **Persuasion intensity for discipline skills** — Obra wins (heavy authority + commitment + social proof). The Tide skills are mostly workflow/orchestration, not discipline — they don't need heavy persuasion.
- **Eval minimum** — Tide template (1 happy + 1 halt) is the floor; Obra's 3-eval ideal is the target.
