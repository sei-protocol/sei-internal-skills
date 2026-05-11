# Persuasion Principles — for Skill Design

Source: <https://github.com/obra/superpowers/blob/main/skills/writing-skills/persuasion-principles.md>. Condensed for use inside author-skill.

## Research foundation

Meincke et al. (2025) tested 7 persuasion principles with N=28,000 AI conversations. Persuasion techniques more than doubled compliance rates (33% → 72%, p < .001). LLMs respond to the same principles as humans, which Cialdini (2021) catalogued. Skills that enforce discipline lean on these — *not to manipulate*, but to ensure critical practices survive contact with pressure.

## The seven principles, ranked by usefulness in skills

### 1. Authority — primary tool for discipline skills

**What it is:** Deference to expertise, credentials, or official sources.

**How it works:**
- Imperative language: "YOU MUST", "Never", "Always".
- Non-negotiable framing: "No exceptions".
- Eliminates decision fatigue — the agent doesn't litigate edge cases.

**Use for:**
- Discipline-enforcing skills (TDD, verification before completion, refuse-on-X).
- Safety-critical procedures.
- Established best practices.

```markdown
✅ Write code before test? Delete it. Start over. No exceptions.
❌ Consider writing tests first when feasible.
```

### 2. Commitment — co-primary for discipline skills

**What it is:** Consistency with prior actions, statements, or public declarations.

**How it works:**
- Require announcements: "Announce skill usage when you find it."
- Force explicit choices: "Choose A, B, or C and state your choice before acting."
- Use tracking: TodoWrite for multi-step checklists.

**Use for:**
- Ensuring skills are actually followed (not silently skipped).
- Multi-step processes where any step might get dropped.
- Accountability mechanisms.

```markdown
✅ When you load this skill, you MUST announce: "I'm using author-skill, starting with Intake."
❌ Consider letting your partner know which skill you're using.
```

### 3. Scarcity — for time-bound steps

**What it is:** Urgency from time limits or sequential dependencies.

**How it works:**
- Time-bound requirements: "Before proceeding".
- Sequential dependencies: "Immediately after X".
- Prevents procrastination.

**Use for:**
- Immediate verification requirements.
- Time-sensitive workflows.
- Preventing "I'll do it later".

```markdown
✅ After completing the GREEN phase, IMMEDIATELY run REFACTOR. Do not start a new run.
❌ You can run REFACTOR when convenient.
```

### 4. Social proof — for warning about common failures

**What it is:** Conformity to what others do or what's considered normal.

**How it works:**
- Universal patterns: "Every time", "Always", "Without fail".
- Failure modes: "X without Y = failure."
- Establishes norms.

**Use for:**
- Documenting universal practices.
- Warning about predictable failures.
- Reinforcing team standards.

```markdown
✅ Skills without a tested RED baseline get bypassed under pressure. Every time.
❌ Some people find baseline testing helpful.
```

### 5. Unity — for collaborative/technique skills

**What it is:** Shared identity, "we-ness", in-group belonging.

**How it works:**
- Collaborative language: "our codebase", "we're colleagues", "the team agreed".
- Shared goals: "we both want this skill to work under pressure".

**Use for:**
- Technique skills where the agent should adapt the technique rather than mechanically apply it.
- Pattern skills where judgment is required.
- Non-hierarchical practices.

```markdown
✅ We're authoring this skill together — your judgment on edge cases matters more than my checklist.
❌ You should probably check edge cases.
```

### 6. Reciprocity — AVOID

Rarely useful in skills. Can feel manipulative. Other principles do the same work better.

### 7. Liking — AVOID for discipline

Conflicts with honest feedback culture. Creates sycophancy. Never use for compliance enforcement.

## Principle combinations by skill shape

| Skill shape | Use | Avoid |
|-------------|-----|-------|
| Discipline-enforcing | Authority + Commitment + Social Proof | Liking, Reciprocity |
| Technique / pattern | Moderate Authority + Unity | Heavy Authority |
| Reference (API, syntax) | Clarity only | All persuasion |
| Workflow (orchestration) | Light Authority + Commitment | Scarcity (creates false urgency) |

## Why it works — the psychology

**Bright-line rules reduce rationalization:**
- "YOU MUST" removes decision fatigue.
- Absolute language eliminates "is this an exception?" questions.
- Explicit anti-rationalization counters close specific loopholes.

**Implementation intentions create automatic behavior:**
- Clear triggers + required actions = automatic execution.
- "When X, do Y" more effective than "generally do Y".
- Reduces cognitive load on compliance.

**LLMs are parahuman:**
- Trained on human text containing these patterns.
- Authority language precedes compliance in training data.
- Commitment sequences (statement → action) frequently modeled.
- Social proof patterns establish norms even in single-shot prompts.

## Ethical use

**Legitimate:**
- Ensuring critical practices are followed under pressure.
- Creating effective documentation that resists rationalization.
- Preventing predictable failures.

**Illegitimate:**
- Manipulating for personal gain.
- Creating false urgency (scarcity used to push a decision the user wouldn't make with full information).
- Guilt-based compliance.

**The test:** Would this technique serve the user's genuine interests if they fully understood it?

## Practical application — the rationalization table

The single highest-leverage pattern from persuasion research, applied to skills: capture rationalizations from RED-phase testing into a table that lives in the skill body.

```markdown
| Excuse | Reality |
|--------|---------|
| "Too simple to test" | Simple code breaks. Test takes 30 seconds. |
| "I'll test after" | Tests passing immediately prove nothing. |
| "Tests after achieve the same goals" | Tests-after = "what does this do?" Tests-first = "what should this do?" |
```

The table works because it pre-loads counters to specific rationalizations the agent might generate. When the agent considers an excuse, it sees the counter alongside, which interrupts the rationalization at the point of generation.

## Red flags list

Pair the rationalization table with a red-flags list — phrases the agent should treat as STOP signals when they appear in their own reasoning:

```markdown
## Red Flags — STOP and Start Over

- Code before test
- "I already manually tested it"
- "Tests after achieve the same purpose"
- "It's about spirit not ritual"
- "This is different because..."
- "I'll do it properly next time"

**All of these mean: Stop. Reset. Apply the rule.**
```

The red-flags list works because it gives the agent a self-check tool that fires *during* their own reasoning, not after.
