# Description Crafting

The description is the **only** routing signal. Get it wrong and the skill never fires. Get it right and the skill fires reliably under pressure.

## The hard limit

**1024 characters total**, including the frontmatter overhead. Aim for 600–900 for room to add anti-triggers as edge cases surface.

## The structure

A working description has three parts in order:

```
[Use when ...]  [Anti-triggers / NOT for ...]  [Sibling redirects / For X use /other]
```

### Part 1 — "Use when..." triggers

- **Third person.** "Used when X" or "Use when X" — never "I can help you" or "You should call this for X".
- **Concrete phrases the user would say.** Not synonyms, not paraphrases. The runtime matches text. If a user says "spin up a chain", the description must include "spin up a chain", not "provision an ephemeral validator set".
- **Symptoms over solutions.** "Use when tests are flaky" beats "Use when you need timeout helpers". The trigger should fire on the *problem*, not the *fix*.
- **Multiple phrasings.** Real users will phrase the same intent five ways. List them: 'create a skill for X', 'we need a skill that handles Y', 'scaffold a new skill', 'author a skill', '/author-skill'.

### Part 2 — Anti-triggers

Anti-triggers prevent over-matching. Every skill has adjacent intents that *look* similar but should route elsewhere. Name them:

- "NOT for X" — when X is a sibling skill.
- "SKIP if Y" — when Y is a condition that makes the skill inappropriate.
- "Do NOT use for Z" — when Z is a common misuse.

If you can't think of any anti-triggers, the description is too narrow OR the skill is too broad. Re-examine.

### Part 3 — Sibling redirects

Explicit "for X, use /other-skill" clauses route adjacent intents away. Skills sit in a constellation; the description tells Claude how to navigate to neighbors.

## The big trap — never summarize workflow

**Obra's hardest-earned lesson.** When a description summarizes the workflow, Claude reads the description and skips the skill body. The skill becomes documentation Claude *ignores*.

### Example of the trap

```yaml
# ❌ BAD — summarizes workflow
description: Use when executing plans — dispatches subagent per task with code review between tasks

# Result: Claude does ONE code review (described in the field) instead of TWO (described in the skill body's flowchart).
```

### Fixed

```yaml
# ✅ GOOD — triggers only
description: Use when executing implementation plans with independent tasks in the current session
```

The description is *invocation routing*, not a tl;dr of the skill.

## Tide-local style

The existing workflow skills (coral, council, design, issue, bugbash) use a more verbose description style that includes some workflow summary. That style predates Obra's CSO insight. When authoring a *new* skill, lean toward Obra's style — triggers only — and put the workflow in the body. The trap is real.

If matching the local style is important (e.g., for consistency in catalog presentation), keep the workflow summary to one short clause and front-load the triggers:

```yaml
description: "Use when X / 'concrete user phrase' / 'another phrase'. Produces <one-clause workflow summary>. Anti-triggers: NOT for Y; NOT for Z. For W, use /other-skill."
```

## Keyword coverage

Use the words a user would type. Not synonyms.

- **Error messages:** "Hook timed out", "ENOTEMPTY", "race condition", "pod stuck in CrashLoopBackOff".
- **Symptoms:** "flaky", "hanging", "zombie", "pollution", "timeout".
- **Tools:** Actual command names (`kubectl`, `seictl`, `gh`, `forge`), library names, file extensions.
- **Synonyms only when the user genuinely uses both:** "timeout/hang/freeze", "cleanup/teardown/afterEach".

## Examples — bad → good

### Vague

```yaml
# ❌ BAD
description: For working with skills.

# ✅ GOOD
description: Use when authoring a new skill for a specific domain — 'create a skill for X', 'scaffold a skill for Y', 'author a skill', '/author-skill'. Anti-triggers: NOT for editing canonical workflow skills (coral, council, design, issue, bugbash). For multi-component design, use /council.
```

### Workflow summary trap

```yaml
# ❌ BAD — workflow summary
description: Use when authoring skills — runs intake, web research, RED-GREEN-REFACTOR pressure testing, then scaffolds the skill directory.

# ✅ GOOD — triggers only
description: Use when authoring a new skill for a specific domain — 'create a skill for X', 'we need a skill that handles Y'. For multi-component design work, use /council.
```

### First person

```yaml
# ❌ BAD
description: I can help you write skills when you need a new one.

# ✅ GOOD
description: Use when authoring a new skill for a specific domain.
```

### Missing anti-triggers

```yaml
# ❌ BAD — over-matches
description: Use when you want to author a skill.

# ✅ GOOD — bounded
description: Use when authoring a new skill for a specific domain — 'create a skill for X'. Anti-triggers: NOT for editing canonical workflow skills; NOT for in-conversation TODOs (use TaskCreate); NOT for Claude Code built-ins like /loop or /schedule.
```

## Checklist

Before finalizing the description:

- [ ] Starts with "Use when" (third person).
- [ ] Includes 3+ concrete trigger phrases a real user would say.
- [ ] Includes at least 2 anti-triggers (NOT for X / SKIP if Y / do NOT use for Z).
- [ ] Includes ≥1 sibling-skill redirect when adjacent skills exist.
- [ ] No first person ("I", "you should").
- [ ] No workflow summary in the description.
- [ ] Keyword coverage matches what the user would type, not synonyms.
- [ ] Under 1024 characters total.

## Iteration during author-skill

In Step 5 of the procedure:

1. Draft a description from the Intake + Research output.
2. Show it to the user.
3. Ask: "Are there phrases you'd actually say that aren't in here?" — adds real triggers.
4. Ask: "Are there adjacent intents this might mis-fire on?" — adds anti-triggers.
5. Iterate to convergence.

Run the description by the RED-phase subagents too. If the subagent doesn't pick up the skill from the description alone, the description is broken — iterate.
