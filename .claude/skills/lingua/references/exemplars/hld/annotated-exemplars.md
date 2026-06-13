# Annotated HLD exemplars — what good looks like, with provenance

> **Corpus content (Design 03; harvested under PLT-490, verified 2026-06-12).** Positive exemplars only,
> annotated in our own words against the HLD spine (`canonical-shape.md` — `cite: hld/shape/<anchor>`)
> and audience-model rules R1–R5. Cite an entry as `cite: hld/exemplar/<slug>` → the `##` anchors below.
> **The slugs are stable cite targets — renaming one is a breaking change for citing rules.** Inclusion
> bar matches the LLD set: process-vetted (merged/Final/in-tree, or **org-owned** merged/Draft-under-review
> per the `org-owned` class in `../../sources.md`), heavily reviewed, widely consumed.
> Most harvested documents are LLD-genre (see `../lld/annotated-exemplars.md`); the entries below are the
> harvest's genuinely architecture-level reads — the PLT-490 harvest closed at four; new entries enter
> when a genuinely architecture-level public or org-owned doc clears the bar. Nothing reproduced beyond nominative
> titles and section names; license classes in `../../sources.md`.

## linux-rcu-design-tree

**The RCU Design documentation set** (McKenney et al.) — docs.kernel.org `RCU/Design/` (Requirements,
Data-Structures, Memory-Ordering, Expedited-Grace-Periods) · in-tree kernel design docs · License class: **GPL-2.0 — cite-and-link
only.**

- A whole subsystem's architecture documented as a navigable tree: requirements separate from data
  structures separate from ordering guarantees — the system-level decomposition our
  `hld/shape/component-view` anchor describes, sustained across documents (→ R1 at directory scale).
- "Fundamental Non-Requirements" as a first-class section is the best public instance of
  `hld/shape/goals-and-non-goals` done with teeth: the non-promises carry the same structural weight as
  the promises.
- Authored, dated, provenance-stated (LWN 2015 origin named in-text) — design rationale with a named
  owner (→ `hld/shape/key-decisions-and-alternatives`).

## go-type-parameters-orientation

**Type Parameters Proposal — the orientation layer** — golang/proposal `design/43651-type-parameters.md`
· accepted, shipped in Go 1.18 · License class: BSD-3-Clause.

- "How to read this proposal" + a very-high-level overview before any detail is
  `hld/shape/system-overview` discipline applied to a language change: the one-screen mental model
  first, every later concept introduced there (→ R2 at its pack-designated peak).
- The **Omissions** section is `hld/shape/goals-and-non-goals` modeled for both audiences: a human scans
  the fence; an agent reads each omission as a hard constraint against building ahead.

## eip-4844-system-context

**EIP-4844 — the sharding-path framing** — eips.ethereum.org/EIPS/eip-4844 · Final, Core · License class: CC0.

- The rationale's forward-design subsections ("on the path to sharding", "how rollups would function")
  place a component change inside the system's multi-year trajectory —
  `hld/shape/sequencing-and-milestones` written from inside a spec: what ships now, what it
  deliberately under-builds, and what un-defers it later.
- Cross-component blast radius (consensus + execution + rollups) is named with the consumers identified
  (→ `hld/shape/interfaces-and-contracts`: who builds to what at each seam).

## eip-1559-cross-cutting

**EIP-1559 — the risk accounting** — eips.ethereum.org/EIPS/eip-1559 · Final, Core · License class: CC0.

- The Security Considerations section reads as `hld/shape/cross-cutting-concerns` done honestly:
  block-size growth, ordering incentives, and monetary-supply effects are properties of the *system*,
  not of any one function — anchored in the section and argued where they bite (→ R3).
- The economic motivation models `hld/shape/context-and-problem` for a mechanism change: the
  environment (auction dynamics, user overpayment) is established before the mechanism appears.

## seinode-import-volume-shapes

**SeiNode import-volume — shapes evaluated** — `sei-k8s-controller/docs/design-seinode-import-volume.md`
· Draft/RFC · License class: **org-owned** (adapt w/ attribution).

- The **decision-matrix** done right: a `## Shapes evaluated` table scoring options A–E across named
  axes (e.g. interaction-with-Retain-orphan) *before* the decision — humans scan the rows, an agent
  reads each `(axis, option, value)` cell (→ the decision-matrix affordance on
  `hld/shape/key-decisions-and-alternatives`; R1+R4).
- "Additional options considered" types each rejected option with its reason and a revisit-trigger
  ("Evaluated but **not adopted**… Revisit if orphan adoption becomes frequent enough") — alternatives a
  reviewer can pressure-test, not assertions (→ `hld/shape/key-decisions-and-alternatives`).
