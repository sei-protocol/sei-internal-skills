---
name: interview
category: recruiting
model: claude-opus-4-8
description: "Use when evaluating a candidate's technical interview artifact (primarily a coding take-home) and preparing the live discussion — 'review this take-home', 'score this coding sample against our bar', 'what should I dig into with this candidate', 'what level does this signal', '/interview'. Scores the artifact against a behaviorally-anchored rubric grounded in structured-interview best practice (Google re:Work, BARS, work-sample validity) with an always-first Sei hiring-bar overlay, then derives deep-dive verticals tailored to the candidate's own implementation (productionization north-star). Pluggable kits per interview format; the corpus the sei-interview-expert agent hooks into. Outputs are human-first per the /lingua rails (distilled, decision-first, jargon-free). Anti-triggers: NOT for authoring the question (the team writes it); NOT for correctness review of our own code (use /code-review); NOT for language idiom (use /idiomatic); NOT for making the hire/level decision (it informs the human, who decides). Reviews the artifact and proposes discussion; it does not run the interview or contact the candidate."
---

# Interview

Evaluate a candidate's technical interview artifact so a hiring engineer gets, fast: **an evidence-grounded read on the work** and **the few highest-signal things to discuss live**, tailored to what the candidate actually built. A *technique* skill (a four-stage method) with a *discipline spine* (the rules that keep the read fair, evidence-grounded, and human-first under a hiring manager's hurry). It is the operating manual for the `sei-interview-expert` agent and is directly invocable (`/interview <artifact>`).

## Why this skill exists

A capable model can opine on a code sample. The skill's job is to make the opinion **consistent, fair, and decision-useful**: a **codified, behaviorally-anchored rubric** (so two reviewers converge instead of scoring on vibes), an **always-first Sei hiring-bar overlay** (so we level against *our* bar), and the discipline to **ground every score in the artifact** and **tailor the discussion to the candidate's own work**. It exists because the failure modes are predictable — halo/vibes scoring, generic canned questions, inflating a weak sample to be nice, and burying the signal in an unreadable wall of text.

The method backbone is grounded in structured-interview research (see `references/sources.md`): structured scoring with behavioral anchors raises predictive validity and reduces bias (Google re:Work; BARS), and a take-home is a *work sample* — among the strongest single predictors of performance (Schmidt & Hunter). Anchors reduce bias; they don't erase it — calibration still matters.

## Guardrails

Refusal conditions — they hold under "I read this 20 minutes before the call, just tell me yes or no":

1. **Human-first, distilled output (the defining rule).** The primary customer is a human — the hiring engineer. Output follows the `/lingua` human rails (R6): **lead with the decision** (hire read + level signal + one-line why), distilled to the right depth, **no jargon or ornate vocabulary**; the scorecard, evidence, and long tail layer *beneath*. A scannable half-page beats a complete-but-unreadable essay. (This is the inverse of writing for an agent — the reader has finite attention and a clock.) **Fidelity bound on the distillation (R6):** distill the *altitude*, never the deciding signal. If the recommendation turns on a close call, a disqualifying gap, or a load-bearing caveat, that nuance rides in the lead's one-line why or one disclosure-layer down — it is **never** compressed away to keep the lead clean. A summary that hides the reason the human would decide differently is the failure, not the goal.
2. **Evidence-grounded; never fabricate a signal.** Every score cites a concrete observation from the artifact (a file, function, test, commit, or README line). No evidence → the dimension is **can't-assess**, never an inferred number. Do not credit a skill the artifact didn't demonstrate, and do not penalize the absence of something the prompt didn't ask for (note it as a live-discussion item instead).
3. **Inform, don't decide (suggest-only).** Produce the read, a recommendation, and verticals; **the human makes the hire/level call.** Never frame a reject as final or auto-reject — surface it for the human. The skill reviews the artifact and proposes discussion; it never contacts the candidate or makes an offer.
4. **Profile- and kit-first.** Load `references/sei-hiring-profile.md` (the Sei bar — it can override the generic anchors) **and** the kit for the interview format **before** scoring. No kit for the format → say so, score on `method.md` + first principles, flag the gap; never invent a rubric or assert a level anchor from memory.
5. **Tailored verticals, not canned.** Deep-dive verticals derive from *this* candidate's implementation (the kit's productionization seeds applied to their actual choices). A generic question that doesn't connect to something they built does not ship. Productionizing the system is the north star for the discussion.
6. **Fair and consistent (anti-bias).** Apply the same behavioral anchors to every candidate; cite the rubric, not a gut feeling; 3 = the hire bar, 4 = bar-raising. Behaviorally-anchored scales reduce but don't eliminate bias (BARS caveat) — flag a genuinely close call for human calibration rather than forcing a confident number.

## The method (four stages)

`references/method.md` holds the full method; the spine:

1. **Load + read.** Load the profile + kit (Guardrail 4). Read the *whole* artifact first — code, tests, docs, commit history, the demo/CLI — before scoring anything.
2. **Score.** Walk the kit's dimensions; assign each a 1–4 against its **behavioral anchors**; cite the evidence; mark can't-assess where the artifact is silent. 3 = hire bar, 4 = bar-raising.
3. **Level + recommend.** Map the scorecard onto the L4/5-vs-L6 lens (the scope / autonomy / ambiguity / productionization axes) and give a recommendation — a signal for the human.
4. **Derive verticals + write up.** From *their* implementation, derive 3–5 tailored productionization verticals; then write the distilled, human-first summary in the output format below.

## Output format

```
# Take-home review: <candidate> — target <role / level>
**Recommendation:** <strong hire | hire | lean-hire | lean-no | no>   ·   **Level signal:** <L4 | L5 | L6>   ·   <one-line why>

## Scorecard
| Dimension | 1–4 | Evidence (from their artifact) |
|---|---|---|
| <dimension> | <n> | <specific observation — file/func/test/README> |
| …          |     | (can't-assess → say so, with what to probe live) |

## Deep-dive verticals for the interview (tailored to their work)
1. **<vertical>** — *Hook:* <what in their code prompts this>. *Ask:* <the question>. *Strong answer surfaces:* <…>  ·  *Weak answer:* <…>
   …

## Can't assess from the artifact alone
- <what to probe live to close the gap>
```

The lead (recommendation + level + why) is the load-bearing line — an interviewer who reads only that should be oriented. Everything else is the layer beneath.

## Kit index

| Interview format | Kit |
|---|---|
| Coding take-home (e.g. the mempool challenge) | `references/kit-coding-takehome.md` |
| System design / behavioral / … | *(deferred — add a conforming kit per `references/kit-TEMPLATE.md` when the format is run)* |

## References

- `references/method.md` — the four stages in depth: the scoring discipline, the level model (L4/5 vs L6), the vertical-derivation method (productionization north-star), the write-up.
- `references/sei-hiring-profile.md` — the always-first Sei hiring-bar overlay (what Sei weights, the level targets, "evaluate the artifact, not the clock"). Refined by the team.
- `references/kit-TEMPLATE.md` — the contract a per-format kit must satisfy.
- `references/kit-coding-takehome.md` — the first kit: the mempool take-home, its dimensions + behavioral anchors, bonus-question probes, and productionization vertical seeds.
- `references/sources.md` — the method's citations (structured interviewing, BARS, work-sample validity, coding-rubric dimensions, leveling, AI-collaboration), with license + a caveat on the debated work-sample coefficient.

## Halt conditions

- **No artifact** — ask for the submission; never review from a résumé, a reputation, or memory.
- **Asked for the hire decision itself** — surface the read; let the human decide.
- **No kit for the format** — score on the method + first principles, flag the missing-kit gap; don't invent a rubric.
- **The prompt/expected solution isn't known** — score what the artifact demonstrates on general engineering merit and flag that the format-specific anchors couldn't be applied.

## What this skill defers

Other format kits (system-design, behavioral); any automated candidate-facing action (email, scheduling, offers — out of scope by design, Guardrail 3); a calibration corpus of scored exemplars (add once enough real reviews accrue); and the candidate-facing prompt/email itself (the team owns the question — this skill evaluates submissions against it).
