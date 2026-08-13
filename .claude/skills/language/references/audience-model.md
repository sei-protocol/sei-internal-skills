# The audience model — how humans and agents read

The doctrine's heart, and the **owner of the rule names** (R1–R6): other files (including the corpus's
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
- **Distillation is a force multiplier; volume is a cost.** A human needs the core points condensed to
  the right technical depth and delivered first — exactly as much as the decision requires, no more.
  Lead-with-the-answer is a *convergent* standard reached independently across four domains, which is
  itself evidence it is fundamental, not fashion. *Cited:* US Army AR 25-50, *Preparing and Managing
  Correspondence* — Bottom Line Up Front, "the reader must be able to understand the writer's ideas in a
  single reading" — https://armypubs.army.mil/epubs/DR_pubs/DR_a/ARN42124-AR_25-50-007-WEB-13.pdf (public
  domain); Minto, *The Pyramid Principle* (answer-first, grouped support, SCQA) — https://www.barbaraminto.com/
  (cite-and-link); Purdue OWL, "The Inverted Pyramid" (most important first, stop-anywhere) —
  https://owl.purdue.edu/owl/subject_specific_writing/journalism_and_journalistic_writing/the_inverted_pyramid.html ;
  Grice, "Logic and Conversation" — maxim of Quantity, "make your contribution as informative as is
  required" and "no more" — via SEP https://plato.stanford.edu/entries/grice/
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

## The six dual-aligned rules (R1–R6)

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
("typed open-questions sections are canonical good practice") is now additionally Cited via direct
convergent mandate in two independent ecosystems, verified 2026-06-12 (PLT-490 harvest): the Rust RFC
template mandates an unresolved-questions section partitioned into before-merge / during-implementation
/ out-of-scope, and the Go proposal template mandates an "Open issues" section. (The same harvest found
EIP-1 hard-gating Security Considerations and the kernel mandating problem-before-solution — convergent
evidence for *mandated explicit sections* generally, which supports R1's structure claim; those two map
to the drawbacks and motivation conventions respectively, not to typed open questions — see
`exemplars/lld/canonical-shape.md`.) The *causal* claim — "an unmarked soft modal gets silently resolved
by an agent reader" — remains **Stated-opinion** (falsification above; the convergence evidences the
convention, not the mechanism).

**R5 — Color is subordinate to constraint and never load-bearing-only.** Narrative, analogy, and
war stories are allowed — humans engage with them — but no constraint may live *only* in color or only
in typography. When color and explicitness compete for the same line, explicitness wins. *Tier:*
Stated-opinion as a precedence rule (falsification: *we'd revise if dual-purpose prose consistently
carried constraints without loss for agent readers*); the underlying scan behavior is Cited (NN/g).

**R6 — Distill for the human; layer for the agent.** The human reader has finite attention and needs the
core points distilled to the right technical depth and delivered first — exactly as much as the decision
requires, no more (the executive-summary intent). Condensing information-rich content to its load-bearing
core is a **force multiplier**, not a courtesy. R6 turns the volume asymmetry R3 already names into a
move: the agent ingests linearly and tolerates (often benefits from) the completeness a human would
experience as noise, so layered detail costs the agent nothing while a distilled lead saves the human.
Resolve the divergence by **progressive disclosure**, not by choosing one reader — lead with the distilled
answer/summary for the human who stops there (this composes R2's load-bearing lead), then layer the
complete detail beneath for the agent and the human who needs depth.
**Fidelity bound:** distill the *altitude*, never the load-bearing content — the summary sits atop
preserved detail, it never *replaces* it, and a decided constraint, number, or safety/rollback/migration
section is never compressed away to save a line (R3/R4 outrank R6; summarization is lossy compression).
*Tier:* the answer-first/distillation practice is **Cited** — convergent across four independent ecosystems
(US Army BLUF; Minto's Pyramid Principle; the journalistic inverted pyramid; NN/g progressive disclosure —
https://www.nngroup.com/articles/progressive-disclosure/) with Grice's maxim of Quantity as the linguistic
root, and the executive-summary convention as the artifact form — a decision-first, stand-alone overview
(USC Libraries — https://libguides.usc.edu/writingguide/executivesummary). The fidelity bound above
(distill the altitude, layer not replace) is **this skill's own** rule, not attributed to USC. The *agent-benefits-from-what-a-human-finds-noisy* contrast inherits the **Stated-opinion**
tier of R3's length/redundancy claim it builds on (falsification: *we'd revise if distilled-but-layered docs
measurably degraded agent task performance versus flat, complete ones*).

## Doc-artifact prose discipline (prose-steward owned)

`prose-steward` is the **owner of record** for the documentation/prose axis: design docs (HLD/LLD/PRD/1-pager), READMEs, runbooks, and file/package/module **header-doc prose**. It defines this axis, tie-breaks it, and enforces it every xreview cycle. It does **not** cover in-source comments interleaved with code or config-field annotations — that axis is owned by `idiomatic-reviewer` (`/idiomatic`); see Boundary. These four are the doc-axis specialization of R1–R6 (*Tier:* the **convention** — present-state, sparing, top-located, dual-aligned — is Cited as a repo-profile rule, PLT-626, may carry a `Basis:` and may block; but the **agent-cognition rationale** embedded in D1/D3/D4 — *"reads as a current instruction," "weights what it reads first," nothing load-bearing "only in a diagram"* — inherits the **Stated-opinion** tier of the R4/R5 reader-model claims it specializes, and is advisory, not a blocking citation).

**D1 — Present state only.** A doc artifact states what is true *now* — never change, history, or why-removed (that is the PR/commit's job). Banned in durable docs: "this used to…", "we changed X to Y", "(was: …)", migration-pin hints. To the linearly-ingesting agent, "we removed the X path" reads as a current instruction. Present-state rationale ("X is foo because the registry owns the type") is fine; history ("X used to be bar") is not.

**D2 — Sparingly and centralized.** Comprehensive context → one dense, cohesive owner-doc, not scattered shallow notes that drift in N directions. Everything else points to it rather than re-stating it.

**D3 — Top-located.** Header docs lead the file/package/module they describe — a unit-governing constraint must not be buried mid-file or only inside one example; the agent weights what it reads first.

**D4 — Dual-audience legibility.** Human-scannable (load-bearing lead, decision up front, skimmable headings) **and** agent-ingestible (constraints anchored locally to what they govern; ambiguity typed as ambiguity; terms defined before use; nothing load-bearing living only in a diagram, a table color, or an aside).

**Fidelity guard.** A suggested rewrite never invents a commitment, never promotes a soft modal to a requirement, never weakens a decided constraint to read friendlier. Removing history under D1 removes *history*, not the present-state contract it wrapped; undecided stays typed-undecided.

### Boundary — doc artifacts vs. in-source comments

This table is the **canonical home** for the cross-champion boundary; the in-source axis (`/idiomatic` `references/comment-discipline.md`) points here rather than restating it.

| Axis | Owner | Surface |
|---|---|---|
| Documentation artifacts + prose | `prose-steward` (`/language`) | design docs, READMEs, runbooks; the **narrative prose** of a file/package/module header doc |
| In-source comments + config annotations | `idiomatic-reviewer` (`/idiomatic`) | comments **interleaved with code**, inline/config-field annotations; **whether a header-doc comment should exist and where it sits** |

The dividing line is **the unit being described, not the file extension** — *and, for a header doc, the aspect*. Whether a top-of-file package/type doc should exist and where it sits is `idiomatic-reviewer`'s call (placement/existence); the quality of its narrative wording is `prose-steward`'s (prose). A comment three lines into a function is wholly `idiomatic-reviewer`. When one artifact carries both (an LLD with embedded code), the two review separately with no shared findings; disputes on existence/placement resolve to `idiomatic-reviewer`, disputes on narrative wording to `prose-steward`. The shared PLT-626 philosophy means rulings converge rather than conflict.

## Precedence

Repo profile > artifact pack > this model. The profile can establish an exception to any rule here —
honoring documented local convention over generic doctrine is the spine, not a loophole. When two rules
collide in one passage, constraint explicitness (R3/R4) outranks scan polish and distillation (R1/R2/R6)
outranks color (R5) — i.e. distillation (R6) never compresses away a constraint (R3/R4).
