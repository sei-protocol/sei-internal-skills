# Data-architecture-concern kit (TEMPLATE)

A kit is **data** the method loads for one data-architecture concern (domain decomposition, data products & contracts, governance, …). It teaches the pattern the cross-org use-case actually needs, cites the external Data Mesh canon beneath it, and gives review cues + the failure modes to catch. Adding a concern = drop one file conforming to this template at `references/kit-<concern>.md`.

Each kit provides the five sections below, in order, so the method stays concern-agnostic. Copy the skeleton; see `kit-data-products-contracts.md` for a worked kit.

This section schema is a **soft one-way door** — changing it churns every kit. Revise deliberately.

---

```markdown
# <Concern> kit

## 1. What this concern is
One paragraph: the concern as the cross-org/federated use-case needs it, and what
generic single-org Data Mesh habit gets wrong here (the override).

## 2. The pattern (how to do it)
The concrete shape — the design moves, the standard, the convention — cited to the
external canon (`sources.md` §anchor) and to the use-case the profile encodes.
"Do it this way."

## 3. Anti-patterns / failure modes
Named smells with a detection cue and the correct rewrite — the generic habits
that are wrong cross-org (e.g. trusting a pipeline instead of a verifiable product;
manual/operator-dependent governance; whole-graph dumps; bonding a judgment claim;
the deprecated Data Contract Spec; mesh where a lakehouse would do).

## 4. Review cues
What a reviewer looks for, mapped to the method's six dimensions. Cite the profile
rule / `sources.md` anchor each cue rests on. Always write `Dimension N (name)`.

## 5. One-way doors in this concern
The irreversible / consumer-depended decisions (namespace-authority bindings,
published contracts/ports, the shared ontology, governance type hashes, deployed
gating/bonding configs) that must be flagged for human approval, not asserted.
```

---

**Authoring rules:**
- **Cite both layers:** the external Data Mesh canon (`sources.md` §anchor) AND the cross-org use-case the profile encodes. A claim with neither is not a kit entry.
- The **profile** (`data-mesh-profile.md`) holds the cross-cutting cross-org conventions — kits reference it, don't restate it.
- Where the cross-org use-case **overrides** generic single-org mesh habit, say so explicitly and cite the generic as the floor (`sources.md`).
- **Respect the currency flags** in `sources.md`: ODCS (not the deprecated Data Contract Spec); the six attributes (not "DATSIS" as Dehghani's term); "five pillars"/"policy-as-code" are industry, not Dehghani, vocabulary; the product-descriptor standards are unsettled.
- Keep review cues mapped to the six method dimensions so findings stay rankable. **Always write the dimension as `Dimension N (name)`** — keep the parenthetical name, never a bare `Dimension N`. The number→name map lives only in `method.md`.

## Kit roster (shipped + deferred)

Shipped:
- `kit-domain-decomposition.md` — DDD domain boundaries, real ownership vs governance theater, the cross-org trust boundary, **and the fit-check** (is mesh even right?).
- `kit-data-products-contracts.md` — the architectural quantum, the six usability attributes, ODCS v3.x contracts + ports/SLOs, the verifiable-certificate model.
- `kit-federated-governance.md` — global/local policy split, computational/policy-as-code enforcement, the thin-neutral-registry moat, the org-boundary trust switch.
- `kit-self-serve-platform.md` — the three platform planes, cognitive-load reduction, bounded/scoped/push-capable access.
- `kit-data-quality-observability.md` — the five observability pillars, OpenLineage, SLOs, the claim-class taxonomy, authenticity≠correctness≠truth.
- `kit-interoperability-lineage.md` — global addressing/identity, polyglot ports, extend-without-migration, the product-descriptor standards layer (ODCS/ODPS/DPDS).

Deferred (add as a conforming kit when first encountered — the corpus grows by use):
- `kit-lakehouse-fabric-adjacency` — data mesh vs data fabric vs lakehouse vs warehouse; when each fits; how they compose (the fit-check's deeper treatment).
- `kit-data-contract-tooling` — deep ODCS v3.x authoring + the Data Contract CLI (validation, quality tests, format export) once contract tooling is in active use.
