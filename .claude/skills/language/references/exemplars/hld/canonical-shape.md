# Canonical HLD shape

> **Corpus content — owned by BG-1 / Design 03 (the exemplar corpus).** This file is original,
> our-own-words analysis of what a strong High-Level Design carries. `pack-hld.md` (BG-2 / PLT-479)
> **cites** it via prose-string cites of the form `cite: hld/shape/<anchor>`. The section anchors below
> are the **stable cite targets** — renaming one is a breaking change for every citing rule. Add anchors
> freely; rename or remove only with a coordinated update to the citing pack.
>
> Copyright: the public sources named here (arc42, C4, "Design Docs at Google", AWS Well-Architected,
> Google SRE) are **cited and linked, never reproduced or closely paraphrased**. We name the concept and
> point at the source; the analysis is ours. License posture lives in `references/sources.md`.
>
> Forward references: `audience-model.md` and `pack-hld.md` are the doctrine files that land with BG-2
> (PLT-479) in this same skill — cited here by name so the shape↔doctrine hooks are explicit from day one.
> The named-rule phrases in prose below ("local-anchoring rule", "typed ambiguity", a load-bearing lead)
> are **descriptive pointers, not stable IDs**: `audience-model.md` owns the rule names and numbering, and
> this file tracks it. Only the section **anchors** are the stable cite contract.

A High-Level Design is an **architecture/system-level** document: it commits a team to a shape — the
boundaries, the major components, the contracts between them, and the decisions that are expensive to
reverse — *before* the code that implements them exists. It is read by two audiences with different needs.
A human reviewer **scans** for the decision and its "so what," then reads selectively; the lead of each
section must carry weight (see `audience-model.md`). An agent or downstream implementer **ingests linearly
and holds everything**, and weights explicit structure and locally-anchored constraints. A strong HLD
serves both: scannable spine for the human, explicit and self-contained sections for the agent. The shape
below is the spine that does that. It draws on publicly described practice — arc42's section catalogue
(<https://arc42.org>), the C4 model's context/container/component altitudes (<https://c4model.com>), Malte
Ubl's "Design Docs at Google" (<https://www.industrialempathy.com/posts/design-docs-at-google/>), AWS
Well-Architected (<https://docs.aws.amazon.com/wellarchitected/>), and the design chapters of the Google
SRE book (<https://sre.google/books>) — none of it reproduced.

An HLD is not an LLD: it stops at component boundaries and contracts, not class layouts or function
signatures. It is not a PRD: it assumes the *what* and *why* are settled enough to commit to a *how*. When
the spine is mostly empty, the design isn't ready — write the PRD first.

---

## context-and-problem

Sets the stage: the problem being solved, who has it, and the environment the system lives in — the
external actors, upstream/downstream systems, and the boundary that says what is in scope and what is not.
For the human, this is the orientation that makes the rest legible in one read; for the agent, it is the
ground truth that prevents inventing a context that isn't there. The C4 "system context" altitude
(<https://c4model.com>) and arc42's context/scope sections (<https://arc42.org>) both insist a design open
here. **Without it:** reviewers re-derive the problem from the solution (and disagree about it), and scope
creep has no fence to push against.

## goals-and-non-goals

States what this design must achieve and — equally load-bearing — what it explicitly will *not* do. Goals
that are measurable or falsifiable are worth more than aspirations. The non-goals are the cheap fence: they
end "but what about X" debates before they start and they tell an implementer where the boundary is. Humans
scan this list to calibrate ambition; agents read non-goals as hard constraints and will otherwise dutifully
"solve" things you meant to defer. **Without it:** unbounded scope, and a review that argues about
requirements instead of the design.

## system-overview

The one-screen mental model: the major pieces and how they fit, ideally with a diagram a reader can hold in
their head. This is the human's scan target — the place they decide whether to trust the rest — so it must
be load-bearing on its own, the "assume the reader stops here" section generalized from `/prfaq`. The C4
container view (<https://c4model.com>) is the canonical altitude here: major deployable/runtime units, not
internal structure. For the agent, the overview should name every component the detail sections will later
expand, so nothing appears without introduction. **Without it:** the reader assembles the system from
fragments and never sees the whole; the diagram-free wall of prose hides the architecture.

## component-view

The worked decomposition: each major component, its single responsibility, what it owns, and what it
depends on. This is where the human stops scanning and starts reading selectively, and where the agent's
"hold everything / weight explicit structure" need is highest — so each component gets an explicit,
self-contained entry (responsibility, inputs, outputs, dependencies) rather than a narrative the reader
must thread together. The C4 component altitude (<https://c4model.com>) and arc42's building-block view
(<https://arc42.org>) describe this level: inside the containers, one level deep, not into code. State
responsibilities crisply enough that two components never claim the same one. **Without it:** ownership is
ambiguous, the same logic gets built twice, and the boundaries the overview promised turn out not to exist.

## interfaces-and-contracts

The contracts at every component boundary and every system edge: the APIs, message schemas, events, and
data shapes — what goes in, what comes out, who produces it, who consumes it, and what each side may assume.
This is the design's most agent-critical section: an implementer (human or agent) builds *to* these
contracts, and an under-specified field becomes a silently-invented one. State error and failure responses,
not just the happy path. For trust boundaries (the on-chain/off-chain/TEE handoffs this org cares about),
say explicitly **who verifies what** at each handoff. **Without it:** components integrate by guesswork,
the seams leak, and the failure modes are discovered in production.

## key-decisions-and-alternatives

The decisions that are costly to reverse, each with the alternatives considered and the reason this one
won — the design's accountability record, in the spirit of an ADR and of "Design Docs at Google"'s
alternatives-considered discipline (<https://www.industrialempathy.com/posts/design-docs-at-google/>). A
decision without its rejected alternatives reads as arbitrary and gets relitigated; the alternatives are
what let a reviewer pressure-test the *reasoning*, not just the conclusion. Name one-way doors explicitly.
Humans scan the decision; agents and future maintainers read the rationale to know which constraints are
load-bearing. **Without it:** the design is a list of assertions, every decision is reopened on every
review, and reversibility is invisible until someone hits the wall.

## cross-cutting-concerns

The properties that don't belong to any one component because they span all of them: security and trust
model, failure and recovery, scaling and performance, observability, cost, data lifecycle, compliance. AWS
Well-Architected's pillars (<https://docs.aws.amazon.com/wellarchitected/>) and the Google SRE design
chapters (<https://sre.google/books>) catalogue this space. These are anchored here *and* locally where they
bite — the one place the doctrine *mandates* strategic redundancy (state the constraint where it applies, so
the agent reading a component section doesn't miss it), per `audience-model.md`'s local-anchoring rule.
**Without it:** security and operability are bolted on after the shape is frozen, when they're most expensive
to add, and the design optimizes one property silently at the cost of others.

## sequencing-and-milestones

How the system gets built incrementally: the phases, what ships first, what each milestone depends on, and
what is explicitly deferred (with the trigger that un-defers it). This is the YAGNI floor written down — the
smallest subset that delivers value, with everything else named as "deferred — when X" rather than silently
omitted. Humans use it to plan; agents use the deferral triggers to avoid building ahead of need. **Without
it:** the design implies a big-bang delivery, scope can't be cut defensibly, and "deferred" becomes
indistinguishable from "forgotten."

## open-questions

What is genuinely undecided, stated as **typed ambiguity** — an explicit Open-question or "TBD — decide by
X / owner Y," never a soft modal buried in prose. This is the doctrine's highest-value rule made structural
(`audience-model.md`): a human skims this list to know what's still in play, and an agent reads an explicit
open question as a stop sign rather than filling the gap by plausible completion. The honest open-questions
list is also what keeps the rest of the document trustworthy — it signals where the confidence ends.
**Without it:** uncertainty leaks into the confident sections as soft hedges ("we might," "probably"), the
human can't tell settled from unsettled, and the agent resolves the ambiguity for you — quietly, and
possibly wrong.
