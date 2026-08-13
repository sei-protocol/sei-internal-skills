# Interview-format kit (TEMPLATE)

A kit is **data** the method loads for one interview format (a coding take-home, a system-design write-up, …). It specializes the eight method dimensions with **behavioral anchors** for the format and supplies the **vertical seeds** the agent tailors to the candidate's artifact. Adding a format = drop one file conforming to this template at `references/kit-<format>.md`.

Every kit provides the six sections below, in order, so the method stays format-agnostic. Copy the skeleton and fill it. See `kit-coding-takehome.md` for a complete worked kit.

This section schema is a **soft one-way door**: changing it churns every kit. Revise deliberately.

---

```markdown
# <Format> kit

## 1. What this format tests
One paragraph: the signal this format is good at eliciting, and what it is blind to (so the
verticals + live discussion cover the blind spots).

## 2. The prompt being evaluated
The deliverables the candidate was asked for (summarized — link the canonical prompt). Scoring
judges the artifact against THIS; do not penalize the absence of scope the prompt didn't request.

## 3. Dimensions in play
Which of the eight method dimensions apply to this format and their relative weight (per the
Sei profile). A format may de-emphasize a dimension (e.g. a design write-up has no "tests")
— say so explicitly rather than scoring an N/A as a low number.

## 4. Behavioral anchors (1–4 per dimension)
For each in-play dimension, the four anchors — poor (1) / borderline (2) / solid-hire-bar (3) /
bar-raising (4) — written as OBSERVABLE behaviors keyed to THIS format, not abstract adjectives.
A reviewer matches the artifact to a description, not a number. (BARS — see sources.md.)

| Dimension | 1 poor | 2 borderline | 3 solid (bar) | 4 bar-raising |
|---|---|---|---|---|
| <dim> | <observable> | <observable> | <observable> | <observable> |

## 5. Vertical seeds (productionization north-star)
The menu of deep-dive tradeoffs this format's artifacts commonly open — each a (hook → ask →
strong-vs-weak) the agent FIRES and SHARPENS against the candidate's actual choices. The agent
ships only the seeds that trace to something the candidate built or deliberately omitted.

## 6. Level notes (L4/5 vs L6 for this format)
What separates a solid-IC signal from a senior/staff signal specifically on this format's artifact.
```

---

**Authoring rules:**
- Anchors are **observable behaviors**, not adjectives — "justifies the priority structure for this workload and names the gas/tx-count trade-off" beats "good design sense." Two reviewers must match the same artifact to the same anchor.
- Vertical seeds must be **derivable from an artifact**, not free-floating trivia. Each names the *hook* — the thing in the work that opens it.
- Cite the dimension's basis (`sources.md`) where the format inherits it; mark format-specific judgment calls as the team's own.
- Keep the Sei bar in `sei-hiring-profile.md`, not duplicated here — the profile overrides these anchors when they conflict.
