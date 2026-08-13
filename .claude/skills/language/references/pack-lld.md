# Artifact pack — LLD (component / RFC-flavored low-level design doc)

The second artifact pack. For each section of the LLD spine it rules **which audience-model rules
dominate** and cites the corpus shape (`cite: lld/shape/<anchor>` → `exemplars/lld/canonical-shape.md#<anchor>`)
and, per section, the single strongest harvested exemplar (`cite: lld/exemplar/<slug>` →
`exemplars/lld/annotated-exemplars.md#<slug>` — the stable cite contract from PLT-478). Rule names
(R1–R6) and basis tiers are owned by `audience-model.md`; rulings below inherit each rule's tier.

An LLD's two audiences pull hardest in different places: the human reviewer decides at the top whether
the design wins its design space; the implementing agent builds *to* the middle — and this is the
highest agent-reader-stakes vertical, because every imprecision in the spec becomes code. The pack's
job in Translate is to say, per section, which way to lean.

| Section (`cite: lld/shape/…`) | Dominant rules | Ruling |
|---|---|---|
| `summary` | **R2** (peak), R1 | Load-bearing lead for both — readable in isolation, and the claim every later section must stay consistent with ("assume the reader stops here"). `cite: lld/exemplar/go-error-values` (by-the-book Abstract opening the spine). |
| `motivation-and-problem` | **R2** (peak), R5 | Problem before any solution — quantify the pain, don't adjective it; color is allowed but no constraint may live only in the anecdote. `cite: lld/exemplar/rust-rfc-2094-nll` (four named failure cases in code before any design). |
| `guide-level-explanation` | R2, R1 | Human-leaning: the narrative/worked-example half of the dual-audience split, taught as if shipped. Keep separate from the spec below; don't fold examples into the contract. `cite: lld/exemplar/go-type-parameters` ("How to read this proposal" + overview before spec). |
| `reference-level-specification` | **R4** (peak), R3, R1 | The most agent-critical section of any vertical: the competing-implementations contract — syntax, semantics, corner cases, every interaction. A soft modal here is the highest blast radius in the corpus (an implementer builds to it). Typed commitment language for the *decided*: RFC-2119 MUST/SHOULD/MAY bound to concrete signatures. `cite: lld/exemplar/erc-721-nft-standard` (normative keyword per function signature). |
| `rationale-and-alternatives` | R2, R4 | Decision first, then the alternatives considered and the objections raised — recorded so the reviewer can pressure-test reasoning they can see. One-way doors and rejected paradigms named explicitly. When alternatives are weighed across multiple axes, the **decision-matrix table** is the preferred dual-track form (options × axes, typed cells — humans scan rows, agents read each `(axis, option, value)` fact). `cite: lld/exemplar/go-type-parameters` (in-text lineage: rejected "contracts" draft became constraints); `cite: hld/exemplar/seinode-import-volume-shapes` (the matrix form). |
| `drawbacks-and-risks` | R4, R3 | The author argues against the proposal: costs named concretely, security/trust implications stated where they bite. Safety-critical content — restructure, never reword (Guardrail 6). `cite: lld/exemplar/eip-1559-fee-market` (admitted costs as named risk subsections). |
| `compatibility` | **R3** (peak), R4 | Mandated-redundancy section: each break anchored here *and* restated at the wire/persisted-format door it threatens. Wire/persisted one-way doors named. `cite: lld/exemplar/erc-721-nft-standard` (Caveats disclosing toolchain constraints inside a Final standard). |
| `unresolved-questions` | **R4** (peak) | The structural home of typed ambiguity — the strongest cross-ecosystem convergence in the harvest, surviving even at zero items. Translate moves every soft modal found elsewhere here (owner + decide-by), preserving original phrasing as provenance; partition before-merge / during-implementation / out-of-scope. `cite: lld/exemplar/rust-rfc-2394-async-await` (merged with its own `await` syntax explicitly unresolved). |
| `future-possibilities` | R4, R1 | The scope-protecting parking lot: adjacent extensions acknowledged and explicitly *not* in this design — the positive twin of non-goals. Each deferral carries its un-defer trigger; a bare "later" is untyped ambiguity. `cite: lld/exemplar/go-error-values` (deferred multi-error support, trigger that later fired as `errors.Join`). |
| `rollout-and-sequencing` | **R3** (peak), R4 | How the component ships incrementally — **shares the HLD `sequencing-and-milestones` base** (phases, what-ships-first, typed deferral) and extends it with the infra-LLD deltas: per-phase rollback anchored at its phase, cross-repo merge-ordering typed, files-touched anchored by symbol/section. Does **not** restate `compatibility` (breaking changes), `drawbacks-and-risks` (risk), or `future-possibilities` (adjacent-feature deferral). `cite: lld/exemplar/config-manager-design` (deferral that rides with the CLI), `cite: lld/exemplar/validation-substrate-ship-cut` (v1-ship-cut table with named un-defer triggers). |

## Translate notes specific to LLDs

- **Inventory pass priorities** (method step 3): soft modals inside `reference-level-specification`
  (the corpus's highest blast radius — an implementer builds to these, with no graceful-failure
  fallback); decided constraints living only in prose where they should be typed against a signature
  or constant; normative-keyword discipline applied to the *decided*, not just the undecided.
- **Normative-keyword discipline:** for settled commitments, RFC-2119 MUST/SHOULD/MAY bound to a
  concrete function signature or named constant is the bar (`cite: lld/exemplar/erc-721-nft-standard`).
  This is R4's typed-language discipline turned on the decided — the inverse of typing the undecided in
  `unresolved-questions`. A decided constraint stated as soft prose is as under-specified as an unmarked
  open question; surface it.
- **Spec sections are commitments:** `reference-level-specification`, `drawbacks-and-risks` (security),
  `compatibility` (migration/breaking), and `rollout-and-sequencing` (per-phase rollback + cross-repo
  merge-ordering) carry constraint content the implementer/operator relies on — restructure around it,
  never reword it (SKILL.md Guardrail 6 safety-critical handling).
  `cite: lld/shape/reference-level-specification`, `cite: lld/shape/rollout-and-sequencing`.
- **File anchoring is by symbol/section, not bare line number** (line numbers rot). When `rollout-and-
  sequencing` or `reference-level-specification` cites the edit surface, prefer a symbol/heading anchor;
  this is a doc-authoring convention, not a corpus `cite:` form.
- **What good looks like end-to-end:** the corpus shape's per-section "Without it:" clauses
  (`cite: lld/shape/unresolved-questions`, etc.) are the citable rationale when a change-log entry needs
  to justify a structural move.

## Deferred (per the design's MVP cut)

PRD pack — one-file-add when a real consumer reviews that vertical. Until then, PRDs translate against
`audience-model.md` + first principles with the missing-pack gap flagged.
