# The audience model — how humans and agents read

The doctrine's heart, and the **owner of the rule names** (R1–R5): other files (including the corpus's
`exemplars/hld/canonical-shape.md`) reference these rules descriptively and track this file. Every rule
carries a **basis tier**:

- **Cited** — grounded in a named authority (linked below) and/or a repo-profile rule. May surface as a
  finding/transform rationale with a `Basis:` line.
- **Stated-opinion** — the doctrine's own claim where settled consensus doesn't exist yet. Surfaces
  **only** in a labeled *Advisory* section, never blocking, never dressed as a citation. Each carries a
  falsification line; the later corpus harvest pressure-tests these — survivors graduate to Cited,
  failures get cut.

Authority citations here are **direct prose-string + URL** (the `/idiomatic`/`/systems` pattern);
exemplar citations go through the corpus (`sources.md`). All sources below are cite-and-link unless
noted (NN/g and Anthropic posts are free-to-read but reserved; plain-language guidance is US-gov public
domain).

## The human reader — Cited

Scans first, reads second, holds little, asks when lost.

- **Scanning dominates.** Readers follow an F-shaped pattern: first lines and line-starts get read;
  content buried mid-paragraph or low on the page often never does. *Cited:* Nielsen Norman Group,
  "F-Shaped Pattern of Reading on the Web" — https://www.nngroup.com/articles/f-shaped-pattern-reading-web-content/
- **Working memory is small.** A reader holds only a handful of chunks at once; every undefined term or
  forward reference taxes it. *Cited:* Miller, "The Magical Number Seven, Plus or Minus Two" (Psych.
  Review 63, 1956) — https://psychclassics.yorku.ca/Miller/ ; Sweller, "Cognitive Load During Problem
  Solving" (Cognitive Science 12(2), 1988) — https://doi.org/10.1207/s15516709cog1202_4
- **Plain, front-loaded language lands.** Short sentences, one idea each, the point first. *Cited:* US
  federal plain-language guidance — https://digital.gov/guides/plain-language/ (public domain)
- **Failure under ambiguity is graceful:** a human who hits an unclear passage asks, or flags it.
  *Tier:* **Stated-opinion** — the contrast assumption to the agent's silent-completion claim
  (falsification: *we'd revise this if humans routinely built from ambiguous specs without asking*).

## The agent reader — Cited where possible, opinion flagged

Ingests linearly, holds everything, weights explicit structure.

- **Explicit structure and unambiguous wording are load-bearing** for agent consumers: clear sections,
  defined terms, disambiguated instructions measurably improve agent behavior — the same properties
  Anthropic mandates for tool descriptions ("describe it like you would to a new hire") and curated
  context. *Cited:* Anthropic, "Effective context engineering for AI agents" —
  https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents ; Anthropic,
  "Writing effective tools for agents — with agents" —
  https://www.anthropic.com/engineering/writing-tools-for-agents
- **Tolerates length and strategic redundancy** better than a human scanner; a constraint restated at
  its point of use is cheap insurance for a linear reader. *Tier:* **Stated-opinion** (falsification:
  *we'd revise this if repetition measurably degraded agent task performance on long docs*).
- **Degrades silently under ambiguity — resolves by plausible completion.** A soft modal ("we should
  probably X") tends to be read as settled-X, or silently resolved either way; the RED baseline for this
  very skill reproduced it twice (a "probably cap at 3" promoted to "max 3 attempts"; invented
  "conservative completions"). *Tier:* **Stated-opinion** — directionally supported by Anthropic's
  disambiguation guidance, but the causal claim is ours. (Falsification: *we'd revise this if agents
  reliably surfaced soft-modal instructions as questions instead of completing them.*)

## The five dual-aligned rules (R1–R5)

**R1 — Structure explicit AND scannable.** Headings, lists, and defined terms serve the agent's
structural weighting *and* the human's scan — these compose; there is no trade-off. *Tier:* Cited
(NN/g; Anthropic context-engineering).

**R2 — The lead is load-bearing for both.** The document and each section open with the decision/point
and its "so what" — the human who stops early and the agent weighting everything both get the thesis.
Generalizes `/prfaq`'s "assume the reader stops here." *Tier:* Cited (NN/g F-pattern; plain-language
guidance; `/prfaq` canonical shape — `cite: prfaq/shape/press-release--section-by-section`).

**R3 — Constraints are anchored locally, not only globally.** State the constraint where it applies,
even if that repeats it. The one place redundancy is *mandated*: the human skips it while scanning; the
linear reader stops dropping it. *Tier:* the structure half is Cited (Anthropic context-engineering:
curation and placement of load-bearing context); the repetition-helps half is **Stated-opinion**
(falsification above).

**R4 — Ambiguity is typed, not prosed.** Anything undecided is *marked* — `Open question: … owner?
decide by?` — never left as a soft modal. **Highest-value rule.** *Tier:* split — "explicit,
disambiguated text serves agent consumers" is Cited (Anthropic tool-writing), and the *practice* claim
("typed open-questions sections are canonical good practice") is now additionally Cited via convergent
mandate across four independent ecosystems, verified 2026-06-12 (PLT-490 harvest): the Rust RFC template
mandates an unresolved-questions section partitioned into before-merge / during-implementation /
out-of-scope; the Go proposal template mandates "Open issues"; EIP-1 hard-gates Security Considerations
(submissions without it are rejected); the kernel's submission discipline mandates problem-before-
solution. The *causal* claim — "an unmarked soft modal gets silently resolved by an agent reader" —
remains **Stated-opinion** (falsification above; the convergence evidences the convention, not the
mechanism).

**R5 — Color is subordinate to constraint and never load-bearing-only.** Narrative, analogy, and
war stories are allowed — humans engage with them — but no constraint may live *only* in color or only
in typography. When color and explicitness compete for the same line, explicitness wins. *Tier:*
Stated-opinion as a precedence rule (falsification: *we'd revise if dual-purpose prose consistently
carried constraints without loss for agent readers*); the underlying scan behavior is Cited (NN/g).

## Precedence

Repo profile > artifact pack > this model. The profile can establish an exception to any rule here —
honoring documented local convention over generic doctrine is the spine, not a loophole. When two rules
collide in one passage, constraint explicitness (R3/R4) outranks scan polish (R1/R2) outranks color (R5).
