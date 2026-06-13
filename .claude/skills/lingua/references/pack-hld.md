# Artifact pack — HLD (architecture / system-level design doc)

The first artifact pack. For each section of the HLD spine it rules **which audience-model rules dominate**
and cites the corpus shape (`cite: hld/shape/<anchor>` → `exemplars/hld/canonical-shape.md#<anchor>` —
the stable cite contract from PLT-478). Rule names (R1–R5) and basis tiers are owned by
`audience-model.md`; rulings below inherit each rule's tier.

An HLD's two audiences pull hardest in different places: the human reviewer decides whether to trust
the design at the top; the implementing agent builds from the middle. The pack's job in Translate is to
say, per section, which way to lean.

| Section (`cite: hld/shape/…`) | Dominant rules | Ruling |
|---|---|---|
| `context-and-problem` | R2, R5 | Human-leaning: orientation prose and color are allowed here — but the problem statement itself leads, and no constraint may live only in the anecdote. |
| `goals-and-non-goals` | R4, R1 | Agent-leaning: list form, falsifiable phrasing. A "goal" with a soft modal is an Open question, not a goal. Non-goals are hard constraints for the agent reader — keep them explicit. |
| `system-overview` | **R2** (peak), R1 | The human's scan target — load-bearing on its own ("assume the reader stops here"). Every component named here must reappear in the detail; nothing appears later unintroduced. |
| `component-view` | **R1** (peak), R3 | Agent-leaning at its peak: per-component entries explicit and self-contained (responsibility, inputs, outputs, dependencies) — structure over narrative thread. |
| `interfaces-and-contracts` | R3, R4 | The most agent-critical section: contracts stated where they bind, error paths included; an under-specified field is an invitation to invent one (see the fidelity guardrail). Undecided contract points are typed Open questions, never soft modals. |
| `key-decisions-and-alternatives` | R2, R4 | Decision first, then rationale and rejected alternatives. One-way doors named explicitly — for both audiences, reversibility must be visible. |
| `cross-cutting-concerns` | **R3** (peak) | The mandated-redundancy section: each concern anchored here *and* restated where it bites. Translating away the "duplicate" mention of a constraint is dropping it. |
| `sequencing-and-milestones` | R1, R4 | Deferred items carry their un-defer trigger ("deferred — when X"); a bare "later" is untyped ambiguity. |
| `open-questions` | **R4** (peak) | The structural home of typed ambiguity. Translate moves every soft modal found elsewhere into this section (owner + decide-by), preserving the original phrasing as provenance. |

## Translate notes specific to HLDs

- **Inventory pass priorities** (method step 3): constraints living only in `context-and-problem`
  color; soft modals inside `interfaces-and-contracts` (highest blast radius — an agent builds to
  these); components mentioned in detail sections but absent from `system-overview`.
- **Trust boundaries:** where the HLD describes on-chain/off-chain/TEE or service-to-service handoffs,
  the "who verifies what" statement is constraint content — restructure around it, never reword it
  (SKILL.md Guardrail 6 safety-critical handling). `cite: hld/shape/interfaces-and-contracts`.
- **Diagrams:** a diagram is typography — load-bearing content in a diagram must also exist as text
  (R5: no constraint lives only in visual form). Keep the diagram; add the sentence.
- **What good looks like end-to-end:** the corpus shape's per-section "Without it:" clauses
  (`cite: hld/shape/system-overview`, etc.) are the citable rationale when a change-log entry needs to
  justify a structural move.

## Deferred (per the design's MVP cut)

PRD pack — one-file-add when a real consumer reviews that vertical (LLD + 1-pager packs landed in
PLT-494). Until then, PRDs translate against `audience-model.md` + first principles with the missing-pack
gap flagged.
