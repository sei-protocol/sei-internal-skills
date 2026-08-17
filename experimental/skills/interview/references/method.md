# The method — evaluating an interview artifact

Domain-agnostic across interview formats (a coding take-home, later a system-design write-up). The format-specific anchors live in the kit; this file owns the *how*. Backbone grounded in `sources.md` (structured interviewing + BARS + work-sample validity).

## 0. Load (the gate)

Load `sei-hiring-profile.md` (the Sei bar — it can override a generic anchor) and the kit for the format. Missing kit → score on this method + first principles, flag the gap, don't invent anchors.

## 1. Read the whole artifact first

Before scoring anything, read it all: source, tests, docs/README, commit history, the demo or CLI, and any notes the candidate sent. Two reasons: a score is only fair against the *whole* submission, and the **tailored verticals come from what they actually built** — you can't derive those from a skim. Note assumptions they stated, scope they cut deliberately, and where they signposted "if I had more time."

## 2. Score the dimensions (behaviorally anchored, evidence-grounded)

The eight dimensions. The kit gives each its 1–4 **behavioral anchors** specialized to the format; here is what each measures and its basis (`sources.md`):

| # | Dimension | Measures | Basis |
|---|-----------|----------|-------|
| 1 | Correctness & verification | Does it work; edge cases handled; bugs self-found and fixed | cited (F4/F5) |
| 2 | Code quality & maintainability | Clean structure, naming, right-sized abstractions, no dead code | cited (F4) |
| 3 | Design & trade-off articulation | Sound approach; alternatives weighed; choices justified for the scenario | cited (F4) |
| 4 | Testing discipline | Common + corner cases; tests are meaningful, not incidental | cited (F4) |
| 5 | Communication & documentation | README, commits, stated assumptions; decisions are narrated | cited (F4) |
| 6 | Handling ambiguity & scope judgment | Sensible assumptions stated; scope cut deliberately; problem understood deeply | cited (leveling, F6) |
| 7 | Production / operational thinking | Failure modes, error handling, "will it hold up"; the senior separator and the vertical north-star | team-judgment, leveling-informed |
| 8 | AI-collaboration quality | Decomposes; uses AI for well-scoped subtasks; **owns the decisions, reviews + verifies output, overrides when wrong** | cited *direction* (F7); weighting is ours |

**The 1–4 scale (re:Work poor / borderline / solid / outstanding):**

- **1 — poor:** the work is below bar on this dimension; the gap is concrete, not stylistic.
- **2 — borderline:** present but thin; meets the letter, misses the depth; unexplained choices.
- **3 — solid (the hire bar):** does the right thing for the right, stated reason; what a strong hire delivers.
- **4 — bar-raising:** goes beyond what was asked in a way that signals seniority — weighs compounding costs, reframes scope, builds for production unprompted.

**Scoring discipline:**

- **Evidence or it didn't happen.** Each score cites a specific observation (`mempool.go:add()` uses a slice + linear scan; `pool_test.go` covers the empty-pool and tie-break cases; the README states the fee-vs-timestamp assumption). No evidence → **can't-assess**, with what to probe live. Never an inferred number.
- **Don't penalize un-asked scope.** If the prompt didn't ask for persistence, its absence isn't a low score — it's a live-discussion vertical (Dimension 7). Score what was asked; discuss what wasn't.
- **AI use is expected, not penalized.** They were given AI. Dimension 8 reads *how* they used it (decomposition, verification, override), not *whether*. Polished AI-generated code with no evidence the candidate understands it is a **gap**, not a plus — that's exactly what the live discussion tests.
- **Close calls are flagged, not forced.** When a dimension sits between two anchors, say so and name the deciding question for the human; don't manufacture false precision (BARS caveat — anchors reduce, don't erase, subjectivity).

## 3. Level signal + recommendation

Map the scorecard onto the leveling axes (`sources.md` F6 — scope / autonomy / ambiguity / influence), not a raw average:

- **L4 / L5 (solid IC):** delivers a correct, clean, well-tested solution to the problem **as posed**; autonomous on moderate complexity; articulates trade-offs *within* the task. A row of 3s with sound reasoning is a solid IC signal.
- **L6 (senior / staff):** treats the ambiguous prompt as a **design problem** — reframes scope, surfaces the trade-offs that compound over time, and builds for production / operability **beyond what was literally asked**. Dimensions 3, 6, and 7 at 4 are the senior tell.

The recommendation (strong hire → no) is a **signal for the human**, paired with the one or two observations that drive it. A weak sample is reported as weak with evidence — never inflated to be kind (that wastes the interviewer's 30 minutes and is unfair to stronger candidates).

## 4. Derive tailored verticals (productionization north-star)

The take-home is built in a vacuum; the **interview's value is the discussion** of productionizing it. Derive 3–5 verticals from *their* implementation:

1. **Scan their choices** for the points where a reasonable take-home decision diverges from what production demands — the data structure, the hashing, the concurrency model, the persistence story, the resource bounds, the observability.
2. **For each, write:** the **hook** (the specific thing in their code that opens the question), the **ask** (the question, in plain words), and **strong-vs-weak** (what a senior answer surfaces vs a shallow one). The kit's vertical seeds are the menu; the candidate's code picks which fire and sharpens them.
3. **Rank by signal** — lead with the vertical that best separates levels for *this* submission.

A vertical must trace to something they built or deliberately omitted. "How would you do consistent hashing?" only ships if their code hashes (or should). Generic resilience questions that don't connect to their work do not ship — they test nothing about this candidate.

## 5. Write it up (human-first)

Render the output in the SKILL.md format, human-first: **lead with the recommendation + level + one-line why** (the interviewer who reads only that is oriented), then the scorecard, then the tailored verticals, then can't-assess. Distilled, decision-first, **plain words** — no ornate vocabulary, no jargon where a common word works. The evidence and verticals are the layer beneath the lead, for the reader who drills in. Crisp and information-dense; never a wall of text. **Fidelity bound (R6):** distill the altitude, never the deciding signal — a close call, a disqualifying gap, or the caveat the recommendation turns on rides in the lead's one-line why or one layer down, never compressed out of sight to keep the lead clean (R3/R4 outrank R6).
