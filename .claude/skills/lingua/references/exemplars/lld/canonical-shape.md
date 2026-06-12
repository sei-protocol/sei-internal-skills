# Canonical LLD shape

> **Corpus content — owned by the exemplar corpus (Design 03; harvested under PLT-490).** This file is
> original, our-own-words analysis of what a strong Low-Level Design (component / RFC-flavored) document
> carries. Doctrine rules cite it via `cite: lld/shape/<anchor>`; the section anchors below are **stable
> cite targets** — renaming one is a breaking change for citing rules.
>
> The shape is synthesized from four independently evolved, process-vetted templates, each verified
> first-hand on 2026-06-12: the Rust RFC template (rust-lang/rfcs `0000-template.md`, Apache-2.0/MIT),
> the Go proposal template (golang/proposal `design/TEMPLATE.md`, BSD-3-Clause), the EIP format (EIP-1,
> CC0), and the Linux kernel's change-description discipline (`Documentation/process/`, GPL-2.0 —
> cite-and-link only). Where all four converge, the convention is load-bearing; convergence is noted
> per section. Nothing is reproduced from any source; license posture per source lives in
> `references/sources.md`.

An LLD commits a component to **implementable precision**: exact behavior, interfaces, trade-offs, and
the boundary between decided and undecided — at the altitude where a competing implementation could be
built from the text alone (the EIP-1 bar). It differs from an HLD (`../hld/canonical-shape.md`)
by stopping *inside* one component rather than at the seams between components, and it differs from a
PRD by assuming the what/why are settled. Its agent-reader stakes are the highest of any vertical: an
implementer — human or machine — builds *to* this text, so every imprecision becomes code.

---

## summary

One paragraph stating what the change is, readable in isolation. All four ecosystems open here (Rust
"Summary", Go "Abstract", EIP "Abstract", kernel changelogs' subject-plus-first-paragraph). The
load-bearing lead for both audiences: the human decides whether to read on; the agent gets the claim
every later section must be consistent with. **Without it:** the reader reverse-engineers the proposal
from its mechanics.

## motivation-and-problem

The problem before any solution — who hits it, how often, what it costs. The kernel's submission
discipline makes stating the problem the mandated first move, before the change is described at all;
Rust's template demands use cases and expected outcomes; EIP-1 calls motivation critical for protocol
changes.
The strongest harvested exemplars quantify the pain (const-generics cites the stdlib's array-length-32
ceiling; NLL shows four named borrow-checker failure cases in code before proposing anything).
**Without it:** the review argues about whether a problem exists instead of whether the design solves it.

## guide-level-explanation

The design taught as if it already shipped: how a practitioner would think about and use it, with worked
examples. Rust's template splits explanation into guide-level and reference-level — the genre's most
explicit dual-audience move, and the harvested precedent for serving the scanning human (narrative,
examples) separately from the linear implementer (precision, below). Go's type-parameters doc does the
same with "How to read this proposal" + a high-level overview before the spec. **Without it:** only
authors and reviewers ever understand the feature; users meet it cold in the release notes.

## reference-level-specification

The precise contract: syntax, semantics, corner cases, interactions with every existing feature — at
EIP-1's bar, detailed enough that **competing, interoperable implementations** could be written from it.
Typed commitment language belongs here: EIP-1 encourages RFC-2119 keywords (MUST/SHOULD/MAY) inside
specifications, and ERC-721 attaches each normative keyword to a concrete function signature. The
strongest harvested specs are executable — EIP-1559 and EIP-4844 specify mechanisms as runnable
pseudocode with constants-as-tables. memory-barriers.txt models the **minimum-contract framing**: state
the floor a consumer may rely on, and scope the document's own authority explicitly. **Without it:**
each implementer resolves the gaps differently, and the differences ship.

## rationale-and-alternatives

Why this design wins the design space: the alternatives considered, their trade-offs, and the objections
raised in review — all four ecosystems mandate this section (Rust "Rationale and alternatives", Go
"Rationale", EIP "Rationale", and the kernel's context-problem-solution changelog ordering). The
harvested gold standard is evolution-recorded-in-text: Go's type-parameters doc states in its own
Background how the rejected "contracts" design became constraints; Go's declined `try` proposal carries
a "Design iterations" trail and a pre-emptive FAQ. **Without it:** every decision is relitigated on
every read, and the reviewer cannot pressure-test reasoning they cannot see.

## drawbacks-and-risks

The argument against the proposal, made by its author — Rust mandates "Drawbacks"; EIP-1 hard-gates
"Security Considerations" (submissions without it are rejected, and an EIP cannot reach Final without
one deemed sufficient). The harvested exemplars name their own costs concretely: EIP-1559's risk
section enumerates max-block-size growth, ordering effects, and supply implications as admitted
downsides of the chosen design. An LLD that only sells is a brochure. **Without it:** the costs are
discovered by the people least able to refuse them — operators and downstream implementers.

## compatibility

What existing code, data, or peers the change affects, and the migration story. Go mandates checking
against its compatibility guidelines; EIP-1 requires a Backwards Compatibility section whenever any
incompatibility exists. For a component LLD this is where wire/persisted-format one-way doors get named
explicitly. **Without it:** the breakage is found by the consumers, in production, one at a time.

## unresolved-questions

Typed ambiguity as a mandated, structural section — the single strongest cross-ecosystem convergence in
the harvest. Rust's template partitions open items into *resolve before merge / resolve during
implementation / out of scope*; Go's template defines "Open issues" as the problems the author cannot
yet solve; and merged, implemented designs ship with them honestly (async/await merged with its final
`await` syntax explicitly unresolved; const-generics deferred three named questions that its multi-year
stabilization then tracked). The convention even survives emptiness — NLL keeps the section and writes
"none at present" rather than deleting it. **Without it:** uncertainty leaks into the confident
sections as soft modals, and the implementer resolves it silently.

## future-possibilities

The parking lot that protects scope: natural extensions acknowledged and explicitly *not* in this
design (Rust "Future possibilities"; const-generics' "Future Extensions"; Go's error-values deferring
multi-error support, which later landed separately). The positive twin of a non-goals list — it lets
the author decline scope without appearing to have missed it. **Without it:** every review thread
re-opens the adjacent feature, and the core proposal grows until it stalls.
