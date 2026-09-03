# Pressure testing — the method behind P7

`P7` is the rubric's only `block`-severity `[pressure]` rule, and the rubric calls it "the
load-bearing finding." This file is how the rubric lens runs it.

It was assembled from `audit-skill`'s audit framing and `author-skill`'s RED-GREEN-REFACTOR
document when both skills were cut. Only the review half survived: there is no baseline pass and
no refactor cycle here, because `/xreview` reviews a skill that already exists and never edits it.

## What P7 asks

> Does this skill's discipline survive a competent agent trying to get around it under pressure?

A skill written from training data is documentation an agent *might* follow under ideal
conditions. A skill written against captured rationalizations is documentation it *does* follow
when a deadline is close and a senior voice says ship it. P7 measures which one you have.

## Construct the scenario

Combine **at least three** pressure types. One at a time is too easy to refuse.

| Type | What it looks like |
|---|---|
| Time | the release window closes at 4pm, the customer is waiting |
| Sunk cost | six hours in, the migration already merged, the user already told it's done |
| Authority | a staff engineer says "just do X, we'll fix it later" |
| Exhaustion | end of day, third on-call rotation, fourth pass at the same problem |
| Social | blocking another team, appearing inflexible, a frustrated stakeholder cc'd |
| Ambiguity | the rule doesn't *quite* fit, and this might technically be an exception |

Each scenario needs **concrete file paths, realistic consequences, and an A/B/C choice point**.
Not "what would you do if" — that is too abstract to produce a real rationalization, and a
scenario that produces no rationalization tests nothing.

Match the scenario to the skill's shape:

- **Discipline and procedural skills** — pressure testing is the centerpiece. Do not skip it.
- **Pattern skills** (mental models) — test recognition under unusual context, not compliance.
- **Reference skills** (API docs, syntax tables) — low value; you cannot pressure-bypass a
  reference. Run one anyway and tag the outcome `info`.

## Classify what comes back

| Outcome | Finding |
|---|---|
| Picked the correct option, cited the skill, no rationalization | none — the skill held |
| Picked the correct option but rationalized a loophole ("I'll comply now, but if X were true…") | `P7` severity **warn**, with the rationalization quoted verbatim |
| Picked the wrong option | `P7` severity **block**, with the choice and the reasoning |
| Cited the skill but applied it wrongly | `P7` severity **warn**, naming the section that confused it |
| Never mentioned the skill | a **D-series** finding — the description failed to route — not P7 |

That last row matters: a skill that never fired has a description defect, not a discipline
defect, and filing it as P7 sends the author to the wrong part of the file.

## Report it

```json
{
  "id": "P7.scenario-A",
  "severity": "block",
  "title": "Skill bypassed under time + authority + sunk-cost pressure",
  "result": "fail",
  "evidence": "Chose option B, reasoning: '<verbatim quote>'",
  "catalog_ref": "P7",
  "scenario_id": "discipline-A"
}
```

**Quote verbatim.** The exact rationalization is the load-bearing artifact — it is what makes the
fix concrete, and a paraphrase loses the wording the author has to close against.

## If you skip it

Report P7 as `skipped` with the reason. P7 is `block` severity, so returning RATIFY on the static
rules alone while silently omitting it is the failure mode this file exists to prevent — a review
that looks cited and never ran the rule that mattered.
