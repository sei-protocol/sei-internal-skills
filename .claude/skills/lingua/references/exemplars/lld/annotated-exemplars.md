# Annotated LLD exemplars — what good looks like, with provenance

> **Corpus content (Design 03; harvested under PLT-490, verified 2026-06-12).** Positive exemplars only.
> Each entry is an our-own-words annotation **pointing at** a process-vetted document — **public**
> (merged or Final; one deliberate exception, noted) **or org-owned** (an internal Sei doc, merged or
> Draft-under-review; see the `org-owned` class in `../../sources.md`). Annotations name what
> the exemplar demonstrates, keyed to `lld/shape/<anchor>` sections and audience-model rules (R1–R5).
> Cite an exemplar as `cite: lld/exemplar/<slug>` → the `##` anchors below. **The slugs are stable cite
> targets — renaming one is a breaking change for citing rules.** Nothing is reproduced beyond nominative
> titles and section names; license class per ecosystem is in `../../sources.md` (Linux entries are
> cite-and-link only).

## rust-rfc-2094-nll

**Non-lexical lifetimes** — rust-lang/rfcs `text/2094-nll.md` · implemented (default since the 2018
edition). License class: Apache-2.0/MIT.

- Problem-before-solution at its best: four named borrow-checker failure cases, each demonstrated in
  code, before any design appears (→ `lld/shape/motivation-and-problem`; R2).
- The design is layered (definitions → borrow checking, built incrementally) instead of presented as a
  monolith — explicit structure serving the linear reader (→ R1).
- Treats diagnostics as design: specifies the error-message narrative (borrow point / invalidating
  action / use point) with worked examples — the reader's experience is part of the contract.
- Keeps an empty `unresolved questions` section ("none at present") rather than deleting it — the typed-
  ambiguity convention holding even at zero items (→ `lld/shape/unresolved-questions`; R4).

## rust-rfc-2394-async-await

**async_await** — rust-lang/rfcs `text/2394-async_await.md` · implemented (stabilized Rust 1.39).
License class: Apache-2.0/MIT.

- Merged with its own final `await` syntax **explicitly unresolved** — four open topics typed, scoped to
  post-merge, and later resolved during stabilization. The canonical proof that a design can ship while
  honestly carrying typed open questions (→ `lld/shape/unresolved-questions`; R4).
- Rationale rejects whole paradigms by name (green threads, monads/do-notation, effects systems,
  stackful coroutines) with reasons — the alternatives record a reviewer can pressure-test
  (→ `lld/shape/rationale-and-alternatives`).
- Follows the guide-level / reference-level split exactly — the genre's clearest dual-audience structure
  (→ `lld/shape/guide-level-explanation`; R1).

## rust-rfc-2000-const-generics

**const_generics** — rust-lang/rfcs `text/2000-const-generics.md` · implemented incrementally
(`min_const_generics` in Rust 1.51). License class: Apache-2.0/MIT.

- Motivation quantifies the ecosystem pain concretely (stdlib array impls capped at length 32, making
  arrays second-class) — numbers, not adjectives (→ `lld/shape/motivation-and-problem`).
- Three unresolved questions explicitly deferred to implementation — and the staged, multi-year
  stabilization that followed tracked exactly those deferrals: typed ambiguity functioning as a real
  plan, not a disclaimer (→ R4).
- Separates the shippable core from speculation via a Future Extensions section
  (→ `lld/shape/future-possibilities`).

## go-type-parameters

**Type Parameters Proposal** (Taylor & Griesemer) — golang/proposal `design/43651-type-parameters.md` ·
accepted, shipped in Go 1.18. License class: BSD-3-Clause.

- Opens with "How to read this proposal" plus a very-high-level overview before any specification —
  reader-orientation scaffolding for two audiences reading at different depths (→ R1, R2;
  `lld/shape/guide-level-explanation`).
- An explicit **Omissions** section names rejected capabilities (specialization, metaprogramming,
  variadic type parameters, parameterized methods…) — non-goals as first-class structure, above what the
  Go template requires (→ `lld/shape/future-possibilities`).
- Records its own lineage in-text: the Background states how the rejected 2019 "contracts" draft became
  interface-constraints — the multi-year alternatives-actually-changed-the-design trail, readable in one
  document (→ `lld/shape/rationale-and-alternatives`).
- An *accepted* design that still ends with Issues and named design costs (complexity, pervasiveness,
  efficiency) — honest drawback accounting after the decision (→ `lld/shape/drawbacks-and-risks`).

## go-error-values

**Go 2 Error Inspection** (Amsterdam, Cox, van Lohuizen, Neil) — golang/proposal
`design/29934-error-values.md` · accepted, shipped as `errors.Is`/`errors.As`/`%w` in Go 1.13. License
class: BSD-3-Clause.

- The cleanest by-the-book instance of the Go template (Abstract / Background / Proposal / Rationale /
  Compatibility / Implementation) — the floor, executed exactly (→ the full `lld/shape` spine, `summary` through `future-possibilities`).
- Rationale argues specific rejected alternatives (why `As` over a generic mechanism; why no automatic
  wrapping in `fmt.Errorf`) (→ `lld/shape/rationale-and-alternatives`).
- Defers multi-error support explicitly in response to review — which later landed separately as
  `errors.Join` (Go 1.20): a deferral with a trigger that actually fired
  (→ `lld/shape/future-possibilities`).

## go-try-builtin

**A built-in Go error check function: `try`** (Griesemer) — golang/proposal
`design/32437-try-builtin.md` · **declined** (golang/go#32437) — included deliberately: the *withdrawal*
is the exemplar. License class: BSD-3-Clause.

- The design doc carries a "Design iterations" trail (check/handle → try) and a pre-emptive FAQ
  steelmanning anticipated objections — decision transparency as document structure
  (→ `lld/shape/rationale-and-alternatives`).
- The closing decline (2019-07-16) is positive-signal documentation of decision discipline: it concedes
  what review surfaced (debugging-print and coverage implications) and names the deeper finding — for
  much of the community the problem wasn't a problem — with the same rigor an acceptance gets. Writing
  the *no* well is part of the genre.

## eip-1559-fee-market

**EIP-1559: Fee market change** — eips.ethereum.org/EIPS/eip-1559 · Final, Core. License class: CC0.

- The specification is executable — the full base-fee mechanism as runnable pseudocode — leaving an
  implementer nothing to invent (→ `lld/shape/reference-level-specification`; the EIP-1
  competing-implementations bar, met literally).
- Security Considerations enumerates the chosen design's admitted costs as named risk subsections
  (block-size growth, ordering effects, supply implications) — the author arguing against the proposal
  in public (→ `lld/shape/drawbacks-and-risks`).
- Motivation carries a full economic argument (auction inefficiency, fee volatility vs real cost) — the
  problem case made at the same rigor as the mechanism (→ `lld/shape/motivation-and-problem`).

## erc-721-nft-standard

**ERC-721: Non-Fungible Token Standard** — eips.ethereum.org/EIPS/eip-721 · Final, ERC. License class:
CC0.

- Adopts RFC-2119 normative keywords and attaches each MUST/SHOULD/MAY to a concrete function signature
  in the embedded interfaces — **typed commitment at word granularity**, the harvest's best instance of
  precision-of-commitment language (→ `lld/shape/reference-level-specification`; R4's typed-language
  discipline applied to the decided, not just the undecided).
- Rationale runs eight named subsections (identifiers, transfer mechanism, gas, privacy, metadata…) —
  every contested choice gets its own argued entry (→ `lld/shape/rationale-and-alternatives`).
- A Caveats subsection discloses toolchain constraints (compiler-version workarounds, with issue links)
  — environmental honesty inside a Final standard (→ `lld/shape/compatibility`).

## eip-4844-blob-transactions

**EIP-4844: Shard Blob Transactions** — eips.ethereum.org/EIPS/eip-4844 · Final, Core. License class:
CC0.

- Opens its specification with a named-constants table (14 parameters with types and validation rules)
  before any prose — agent-explicit structure for the values an implementer must get exactly right
  (→ R1; `lld/shape/reference-level-specification`).
- Rationale subsections justify the design *forward* ("on the path to sharding", "how rollups would
  function") — sequencing context inside a component spec (→ `lld/shape/rationale-and-alternatives`).
- Delegates test vectors to canonical external repos with explicit pointers — spec/test separation done
  with resolvable references (→ `lld/shape/compatibility`).

## linux-rcu-requirements

**A Tour Through RCU's Requirements** (McKenney) — docs.kernel.org
`RCU/Design/Requirements/Requirements.html` · in-tree kernel design doc. License class: **GPL-2.0 —
cite-and-link only; never adapt.**

- Organized by requirement class — including a first-class **"Fundamental Non-Requirements"** section:
  what the design deliberately does *not* promise, stated with the same structure as what it does
  — the strongest known instance of explicit negative-space contracting
  (→ `lld/shape/reference-level-specification` minimum-contract framing).
- Embedded "Quick Quiz" Q&A pairs force comprehension checks inline — the document tests its reader
  (→ R1, a structure serving both audiences).
- Carries a "Possible Future Changes" section — evolution expectations made explicit
  (→ `lld/shape/future-possibilities`).

## linux-memory-barriers

**LINUX KERNEL MEMORY BARRIERS** (Howells, McKenney, Deacon, Zijlstra) — kernel.org
`Documentation/memory-barriers.txt` · in-tree. License class: **GPL-2.0 — cite-and-link only.**

- Opens with a disclaimer section that scopes its own authority — declaring itself deliberately
  incomplete and delegating formal authority to the memory-model tooling — a document stating precisely
  how much to trust it (→ R4 applied to the document itself).
- Models the **minimum-contract framing**: it states the floor a consumer may rely on per barrier, and
  treats anything an architecture provides below that floor as a defect — guaranteed behavior explicitly
  separated from incidental behavior (→ `lld/shape/reference-level-specification`).

## linux-submitting-patches

**Submitting patches** — docs.kernel.org `process/submitting-patches.html` · in-tree process doc.
License class: **GPL-2.0 — cite-and-link only.**

- The change-description discipline in miniature: stating the problem is the mandated first move,
  before the change is mentioned at all (→ `lld/shape/motivation-and-problem`; R2 at changelog scale).
- One-logical-change-per-patch with a built-in self-test (a description growing long signals a needed
  split) — scope discipline as a falsifiable rule.

## config-manager-design

**sei-chain configuration manager** — `bdchatham-designs/designs/config-manager/DESIGN.md` (relocated from sei-config docs/, PLT-497) · merged · License
class: **org-owned** (adapt w/ attribution).

- The MVP seam ships minimal (only stamps `schema_version` on write); `doctor` / refuse-on-newer /
  `migrate` **ride with the deferred CLI** — sequencing stated as what-ships-now vs what-rides-later,
  not a flat feature list (→ `lld/shape/rollout-and-sequencing`; R4).
- "Future (deferred)" names two consolidations, each **reversible and out of scope now**, with the
  dependency arrow that makes them safe — typed deferral with its condition
  (→ `lld/shape/rollout-and-sequencing`, `lld/shape/future-possibilities`).
- A one-way door called out inline ("auto-migrate + no-downgrade is a per-pod one-way door that breaks
  rollback") — reversibility made visible at the decision (→ `lld/shape/rationale-and-alternatives`).

## validation-substrate-ship-cut

**seictl validation substrate** — `bdchatham-designs/designs/validation-substrate/validation-substrate.md` (relocated from seictl docs/, PLT-497) · Draft (coral
cross-review) · License class: **org-owned** (adapt w/ attribution).

- The `v1 ship cut` table (Artifact | v1 | Trigger-to-un-defer) ships effectively no new code in v1 and
  lands each primitive only on a named trigger, "not speculatively." The cleanest corpus instance of a
  rollout/ship-cut driven by typed un-defer triggers (→ `lld/shape/rollout-and-sequencing`; R4).
- Each deferred primitive names the painkiller that un-defers it — e.g. an engineer reporting a bench
  that passed while validators OOM-ed the whole window — so a bare "later" is replaced by a falsifiable
  signal (→ `lld/shape/rollout-and-sequencing`).
