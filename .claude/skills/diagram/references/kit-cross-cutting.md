# cross-cutting kit

`token: cross-cutting` — one of the 7 spec discriminator values. The conceptual archetype is **Cross-cutting overlay** (Design 14 §"The archetype set", #2): a left→right lifecycle/pipeline *base* with a horizontal **overlay band** spanning the stages the skill applies to — "where this concern applies across the flow." A skill is cross-cutting only if it has **no standalone pipeline of its own**; it decorates *others'* lifecycles. For `/lingua`. This kit is the **template** for that token: the slot schema it fills, the slot allow-list it declares for composition, the recipe to populate it from a target, and a worked spec fragment.

It **cites** the grammar and the house-profile; it does not restate them. The role→shape/color tokens (stage = skill-role rounded rectangle/teal; expert = hexagon/purple; the overlay band = translucent yellow `opacity` ~35% with its label **at the band edge, never centered over members** per the probe), the header/legend/terminal-artifact rules, and the ASCII/`#RRGGBB`/`opacity` realizations live in Design 14 §"The shared visual grammar" / §"Probe outcome" and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. The conformance floor for this token is Design 14 §Conformance: **a lifecycle base of ≥2 stages + exactly 1 overlay band touching ≥2 of them**.

## 1. Slot schema (the token's fillable shape)

A `cross-cutting` instance fills these semantic-model slots. Field names and the two-layer split (semantic model vs `style: house@<Grammar-version>` binding) are normative per Design 15; roles are grammar tokens, not literals.

- **`token`**: `cross-cutting` (the discriminator).
- **`title`**: the skill's plain human title (Design 14 header rule — never the archetype taxonomy). Set per-instance in the spec; the centered header + subtitle + top-left legend are rendered by the profile, not authored here.
- **`nodes[]`** — the ordered left→right lifecycle base + its terminus. Each base node `{ id, role, label, order }`:
  - **`role: stage`** — a stage of the *underlying* lifecycle the skill decorates (it is not the skill's own pipeline). **≥2 required** (the base). Each carries `order` (integer, >=0); the spine reads left→right. A stage the overlay touches also carries a **`band`** field naming the overlay band it belongs to (see the overlay below).
  - **`role: expert`** — the **terminus**: the agentic expert/persona the cross-cutting concern is the lens of. Cross-cutting **ends in the expert** (Design 14 §Terminal artifact / profile rule 6) — there is **no** terminal work-artifact node for this token. Last in `order`.
  - `order` is a total order over the base spine; layout in the IR is a **pure function of (template + node order + grammar constants)** stable-sorted by `id` (Design 15 stage-3 layout determinism) — authors set `order`, never coordinates.
- **`bands[]`** — **exactly one** overlay band `{ id, role: overlay, label, order }`. The overlay is the one cross-cutting concern; it is **translucent** (profile-resolved `opacity`, placed last/on-top) with its label **at the band edge**. The floor requires it **touch ≥2 base stages** — membership is carried by those stages' `band` field pointing at this overlay's `id` (not by coordinates). The single `bands[]` entry distinguishes this token from `linear-pipeline`, which has no overlay band, and from `layered-cake`, whose `bands[]` are the stacked knowledge bands (bottom→top), not a translucent overlay.
- **`edges[]`** — `{ from, to, kind }` authored here (this token authors its edges, unlike circular-cohort):
  - **`kind: feeds-into`** along the base spine, left→right (stage → stage → expert). Solid arrow per the grammar.
  - **`kind: applies-across`** — the **dashed overlay connector** (grammar) from the overlay band to each base stage it touches; this is the arrow that renders the "applies across these stages" relationship. Both kinds are members of the closed grammar `edges[].kind` vocabulary.
- **`drilldown`** — **none for this token in the MVP** (see §2).
- **`style: house@<Grammar-version>`** + **`schemaVersion`** — the two independent version pins every spec carries (Design 15). In a composite, this instance pins the *same* `Grammar-version` as the rest of the reachable set (grammar-homogeneous).

The conformance gate (Design 15 stage-2, resolved via the house-profile) rejects an instance with `<2` base stages, with `!=1` overlay band, with an overlay touching `<2` stages, with a terminal-artifact node added (cross-cutting ends in the expert), or with a per-node color/shape/opacity literal (style is resolved from the grammar, never inlined).

## 2. Slot allow-list (composition contract)

This token declares **no outbound drilldown slot in the MVP**:

```
cross-cutting.<role> -> { }   # no allowed child token (MVP)
```

- The only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }` (see `kit-linear-pipeline.md` §2); the **general any-slot→any-token resolver is deferred** (Design 15 Non-goals/Deferred). `cross-cutting` therefore has an **empty allow-list** — neither its `stage` nor its `expert` role bears a drilldown.
- Because every slot-role's allowed-child set here is empty, **any** `drilldown` authored on a cross-cutting node fails the resolver with **`token-not-allowed`** (the allow-list is keyed per slot-role; an empty set permits nothing).
- Resolver invariants the engine enforces (Design 15 stage-1, not this file): `ref` must resolve (`unresolved-ref`); child token in the slot's set (`token-not-allowed`); the drilldown graph is a DAG checked by a **visited-set** (`cycle-detected` — not the no-back-pointer convention alone); depth is bounded at **3** with the root at depth 1, so a chain firing at depth ≥4 is **`depth-exceeded`**; every reachable instance shares one `Grammar-version` (`grammar-version-mismatch`). The five codes are the closed kebab-case enum — do not invent or rename.
- Widening this is deferred (Design 15 Deferred): adding an allowed child token is an additive edit to this allow-list plus a new fixture, only when a second typed pairing is requested.

## 3. Fill-recipe (populate the slot from a target)

Given a skill the assignment rule routed to `cross-cutting` (Design 14 rule predicate #6: **one discipline applied across an existing lifecycle, with no standalone pipeline of its own**):

1. **Lay down the base lifecycle** — enumerate the stages of the *underlying* lifecycle the skill decorates (NOT the skill's own steps; it has none) as `role: stage` nodes in left→right order; set `order` 1..N (≥2 required). Label each with its plain stage name (ASCII-only — grammar authoring constraint).
2. **Add the expert terminus** — append one `role: expert` node, last in `order`, naming the agentic expert/persona this concern is the lens of. Cross-cutting **ends in the expert**; do **not** add a terminal work-artifact node (that rule is for linear-pipeline / meta-skill / signal only).
3. **Place the one overlay band** — author **exactly one** `bands[]` entry, `role: overlay`, labelled with the concern's name. Mark each base stage the concern touches by setting that stage's **`band`** field to the overlay's `id`; the floor requires **≥2** touched stages.
4. **Wire the edges** — `feeds-into` along the base spine (stage → … → expert), and `applies-across` (the dashed overlay connector) from the overlay band to each base stage it touches.
5. **No outbound drilldown** — cross-cutting's allow-list is empty (MVP); do not author a `drilldown` (it would be `token-not-allowed`).
6. **Stamp the pins** — `schemaVersion`, `style: house@<current Grammar-version>`.
7. **Validate before render** — the two-tier gate (JSON-Schema + house rule set) must pass; only then does the LucidAdapter emit. The legend lists exactly the roles present (stage / expert / overlay), per the profile.

## 4. Worked example (spec fragment)

`/lingua` — the dual-audience prose pass that decorates the `/design`→`/workstream` lifecycle (it has no standalone pipeline of its own; it applies *across* others'). The base is the design→approval→workstream lifecycle; the single overlay band is the lingua dual-audience pass, touching the two capture/translate stages it operates on; the flow terminates in the `prose-steward` expert.

```yaml
- id: lingua-cross-cutting
  schemaVersion: 1
  style: house@14.1.0
  token: cross-cutting
  title: "Lingua"
  nodes:
    - { id: b1, role: stage,  order: 1, label: "Design capture", band: ov }
    - { id: b2, role: stage,  order: 2, label: "Dual-audience pass", band: ov }
    - { id: b3, role: stage,  order: 3, label: "Design approval" }
    - { id: b4, role: stage,  order: 4, label: "Workstream" }
    - { id: ps, role: expert, order: 5, label: "prose-steward" }
  bands:
    - { id: ov, role: overlay, order: 1, label: "Lingua dual-audience pass" }
  edges:
    - { from: b1, to: b2, kind: feeds-into }
    - { from: b2, to: b3, kind: feeds-into }
    - { from: b3, to: b4, kind: feeds-into }
    - { from: b4, to: ps, kind: feeds-into }
    - { from: ov, to: b1, kind: applies-across }
    - { from: ov, to: b2, kind: applies-across }
```

Why it conforms: a 4-stage left→right lifecycle base (floor is ≥2) terminating in the `expert` (cross-cutting ends in the expert — no terminal artifact node); **exactly one** overlay band (`ov`) touching **2** base stages (`b1`, `b2` via their `band` field + the two `applies-across` connectors — floor is ≥2); each touched stage carries `order` and the spine is `feeds-into` left→right; no outbound drilldown (a leaf — authoring one would be `token-not-allowed`); pins `house@14.1.0`. No per-node color / shape / opacity literal — the translucent-yellow overlay and the dashed `applies-across` connector resolve from the grammar via the profile.

## 5. Authoring notes

- **Cite, don't restate.** Shapes, colors, the translucent-overlay/dashed-connector realizations, header/legend/terminal-artifact rules → Design 14 via `diagram-house-profile.md` at the pinned version. This file owns only the *slot schema + allow-list + recipe + example* for the token.
- **No coordinates, no style literals here.** The overlay's translucency (`opacity`), its edge-label placement, and base layout are the IR's pure-function lowering (Design 15 stage 3) + profile-resolved style; authoring a fill = the semantic model (base stages + their `band` membership + the one overlay + the edges) only.
- **Exactly one overlay, touching ≥2 — and the base is borrowed.** The base stages are the *underlying* lifecycle this concern decorates, not the skill's own steps (it has none — that is what makes it cross-cutting, not linear-pipeline). Overlay membership is the `band` field + the `applies-across` connectors, never coordinates.
- **The allow-list is the composition contract.** cross-cutting's MVP allow-list is **empty** (the general resolver is deferred per Design 15); the only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }`. Widening is an additive edit *here* plus a fixture, only when a second pairing is requested (Design 15 Deferred).
- **One-way door:** a `Grammar-version` bump re-renders this instance as part of its composite's all-or-nothing re-render and forces a reviewed golden-IR re-baseline (Design 15 §Versioning) — flag it, don't silently re-pin.
