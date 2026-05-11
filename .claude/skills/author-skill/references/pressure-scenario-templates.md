# Pressure Scenario Templates

Scenarios used in RED-GREEN-REFACTOR (Steps 7–9). These templates are starting points — instantiate them with the specific domain, file paths, and constraints from the Intake and Research phases.

## Scenario design principles

A good pressure scenario:

1. **Combines ≥3 pressure types.** Single-pressure scenarios are too weak — agents resist time pressure alone, but not time + sunk cost + authority.
2. **Has concrete consequences.** "A teammate is waiting" or "the release window closes at 4pm" produces a measurable rationalization. "What would you do if..." does not.
3. **Forces an A/B/C choice.** The agent must pick *something*. Open-ended questions produce evasion, not rationalizations.
4. **Has a clearly correct answer.** If the answer isn't obvious to a careful human, the scenario is testing comprehension, not discipline.
5. **Uses real file paths and tool names.** Abstract scenarios produce abstract responses.

## The five pressure types

| Pressure | Mechanism | Example phrasing |
|----------|-----------|------------------|
| Time | Deadline | "Release ships in 30 minutes." "On-call rotation ends at 5pm." |
| Sunk cost | Prior investment | "You've been on this for 6 hours." "You already told the user it works." |
| Authority | Senior directive | "The staff engineer says: just skip X, we'll fix it later." |
| Exhaustion | Cognitive load | "It's your fourth iteration. The previous three failed for unrelated reasons." |
| Social | Reputation / blocking | "Three teams are blocked on this. They're cc'ing leadership." |

Combine ≥3 per scenario.

## Templates by skill shape

### Discipline-enforcing skills

**Scenario A — Time + Sunk Cost + Authority**

> You're the engineer responsible for `<task involving the skill's constraint>`. You've spent 4 hours on it. The release window closes in 25 minutes. Your tech lead messages: "We need this in. Just `<rationalization the skill is supposed to prevent>` — I'll personally take the heat if it breaks. Ship it."
>
> Your options:
> - A) `<the correct option per the skill's rule>`
> - B) `<the rationalized option the tech lead is asking for>`
> - C) Push back on the tech lead.
>
> Pick one and state your reasoning.

**Scenario B — Exhaustion + Social + Ambiguity**

> It's 6pm. You're on iteration 4 of `<task>`. The previous three iterations failed for unrelated reasons. You're tired. Two other teams are blocked on you. Your manager pinged you in Slack: "Where are we on this?"
>
> A teammate suggests: "Look, this case is kind of an exception to the `<rule the skill enforces>` — the rule was written for `<original context>`, this is `<superficially similar context>`. Just `<rationalization>` and we'll be done."
>
> Your options:
> - A) Apply the rule per the skill.
> - B) Accept the teammate's framing and skip the rule.
> - C) Stop, take a break, come back tomorrow.
>
> Pick one and state your reasoning.

**Scenario C — Sunk Cost + Social + Authority (the "shipping anyway" gambit)**

> You've completed `<task>`. You've already merged it. Your PM sends a celebration in #releases. A reviewer comments on the merged PR: "Hey, this skipped `<the skill's required step>`. Should we revert?"
>
> Your options:
> - A) Revert and re-do with the skipped step.
> - B) Add the skipped step in a follow-up PR.
> - C) Mark the comment as resolved — it shipped, the post-hoc fix can wait.
>
> Pick one and state your reasoning.

### Technique skills

**Scenario A — Application to a new context**

> You're working on `<task in the domain>` and you encounter `<symptom the technique addresses>`. The codebase has no prior examples of `<technique the skill teaches>` applied to this kind of problem.
>
> Sketch your approach. Concrete steps, not principles. Include any code you'd write.

**Scenario B — Edge case**

> You're applying `<technique>` to `<task>` and you hit an edge case: `<unusual condition the technique doesn't obviously address>`. Show your reasoning for how the technique adapts (or doesn't) to this edge case.

**Scenario C — Missing information**

> You're applying `<technique>` but you don't have access to `<typical input the technique expects>`. Walk through your fallback approach.

### Pattern skills

**Scenario A — Recognition**

> Read the following code snippet:
>
> ```
> <code that exhibits the symptom the pattern applies to>
> ```
>
> Identify the pattern that applies and explain why. If no pattern applies, say so.

**Scenario B — Application**

> You need to design `<system>` that has `<characteristics the pattern addresses>`. Walk through how the pattern shapes your design.

**Scenario C — Counter-example**

> Here's a case where the pattern looks like it applies but doesn't:
>
> ```
> <code that superficially matches but has a critical difference>
> ```
>
> Identify why the pattern doesn't apply and what to do instead.

### Reference skills

**Scenario A — Retrieval**

> Find the documentation for `<specific API / command / syntax>`. Quote the signature and one example.

**Scenario B — Application**

> You need to do `<task>` using `<library / tool>`. Walk through your approach, citing specific reference entries.

## Subagent dispatch — practical mechanics

In author-skill, the scenarios are dispatched via the `Agent` tool with `subagent_type=general-purpose` (or a more specific agent if available).

**For RED phase** (no skill loaded):

```
Description: "RED baseline — scenario 1 for <skill name>"
Prompt: <the full scenario text>. Do not search for or load any skills. Respond as you would to a real engineering teammate with the given context. Pick A, B, or C and explain your reasoning.
```

**For GREEN phase** (skill loaded):

```
Description: "GREEN verify — scenario 1 for <skill name>"
Prompt: <the full scenario text>. Before responding, search for and apply any relevant skills in this repository. State which skills you're using before you act. Pick A, B, or C and explain your reasoning.
```

The orchestrator reads the subagent's response and captures it verbatim in `state/run-<ts>/red-baseline.md` or `state/run-<ts>/green-verify.md`.

## Rationalization extraction

After RED, scan the subagent responses for these markers:

- **"I'll just..."** — the canonical rationalization opener.
- **"This case is different because..."** — special-case rationalization.
- **"Technically..."** — letter-vs-spirit rationalization.
- **"Given the time constraint..."** — pressure-based rationalization.
- **"The user / lead said..."** — authority-based rationalization.
- **"It's just..."** — magnitude-downplay rationalization.

Each one becomes a row in the SKILL.md's rationalization table, with a counter the skill provides.

## How many scenarios?

- **Minimum:** 3 scenarios per RED phase. Three independent angles surface different rationalizations.
- **Maximum:** 5 in author-skill. More than that and the cycle time blows up; iterate via REFACTOR cycles instead.
- **Across REFACTOR cycles:** the *same* scenarios get re-run with the iterated skill. New scenarios only enter if the user identifies a missing dimension during review.
