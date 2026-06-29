# diagram-token kit (TEMPLATE)

A kit is **data** the method loads for one diagram **`token`** — one of the 7 spec discriminator values (Design 15 §Cardinality): `layered-cake-kit`, `layered-cake-signal`, `circular-cohort`, `linear-pipeline`, `hub-and-spoke`, `cross-cutting`, `meta-skill`. (Recall: **6 conceptual archetypes**, **7 tokens** — layered-cake splits into `-kit` and `-signal`.) Each token ships exactly one `references/kit-<token>.md`. A kit teaches how to **fill that token's slots** to the house grammar: the slot schema, the slot allow-list (composition contract), the fill-recipe from a target, and a worked spec fragment.

A kit **CITES** the grammar; it does **not** restate it. The role→shape/color tokens, arrow semantics, header/legend/terminal-artifact rules, and the Standard-Import realizations (ASCII-only labels, `#RRGGBB`, `opacity`, probe-confirmed primitives) live in Design 14 (`14-skill-diagram-visual-grammar.md`) and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. A kit names the token's **conformance floor** by pointing at Design 14 §Conformance — it never re-derives shapes or colors.

Adding a token's template = drop one file conforming to this template at `references/kit-<token>.md`, plus a passing fixture spec. The MVP ships two worked kits: `kit-linear-pipeline.md` and `kit-circular-cohort.md` (the pipeline→cohort-cycle composite). The other five fill in from this template.

Each kit provides the five sections below, in order, so `method.md` (the 4-stage engine) stays token-agnostic. Copy the skeleton; see `kit-linear-pipeline.md` for a worked kit with a non-empty allow-list and `kit-circular-cohort.md` for a worked leaf (empty allow-list).

This section schema is a **soft one-way door** — changing it churns every kit. Revise deliberately.

---

```markdown
# <token> kit

`token: <token>` — one of the 7 spec discriminator values. The conceptual
archetype is **<Archetype>** (Design 14 §"The archetype set", #<n>): <one-line
archetype description>. This kit is the template for that token. It cites the
grammar and the house-profile; it does not restate them. The conformance floor
for this token is Design 14 §Conformance: **<the token's required-slot line,
quoted from the per-token conformance table>**.

## 1. Slot schema (the token's fillable shape)
The semantic-model slots a `<token>` instance fills — `token`, `title`, the
`nodes[]` role set (each role is a GRAMMAR token, not a literal — cite the role's
shape/color to Design 14, do not inline it), `edges[]` (arrow `kind`s, also
grammar tokens), any optional `drilldown`, and the two version pins
(`schemaVersion`, `style: house@<Grammar-version>`). Note which role-slot (if
any) is the terminal work-artifact (linear-pipeline / meta-skill / signal carry
one; layered-cake/cross-cutting end in the expert; hub-and-spoke in the central
identity; circular-cohort in the central work-artifact). Layout is the IR's
pure-function lowering (Design 15 stage 3) — authors set order/membership, never
coordinates; style is profile-resolved, never inlined.

## 2. Slot allow-list (composition contract)
Declare, per slot-role, the closed set of child `token`s a `drilldown` may target:

    <token>.<slot-role> -> { <allowed child token>, ... }

An empty set (`-> { }`) means the token is an MVP leaf (no outbound drilldown);
any drilldown on it then fails with `token-not-allowed`. State which resolver
invariants the engine enforces around this (Design 15 stage 1): `ref` resolves
(`unresolved-ref`); child token in the slot's set (`token-not-allowed`); DAG via
a visited-set (`cycle-detected`); depth bound 3, root=depth-1, fires at >3
(`depth-exceeded`); one `Grammar-version` across the reachable set
(`grammar-version-mismatch`). The five codes are the closed kebab-case enum —
do not invent or rename. MVP pairings are hardcoded; widening is additive here +
a fixture (Design 15 Non-goals/Deferred).

## 3. Fill-recipe (populate the slot from a target)
The ordered procedure to turn a target (a skill the Design-14 assignment rule
routed to this archetype) into a filled instance: enumerate the role-bearing
nodes/bands, wire the edges, add the terminal artifact iff the skill produces a
durable output, decide drilldowns against §2, stamp the two pins (same
`Grammar-version` across a composite), validate before render.

## 4. Worked example (spec fragment)
A real YAML spec fragment for this token that PASSES the conformance floor, with
a one-line "why it conforms" note tying back to §1's floor and §2's allow-list.
If the token participates in the MVP composite, show its half and cross-reference
the paired kit.

## 5. Authoring notes
Cite-don't-restate; no coordinates / no style literals here; the allow-list is
the composition contract; flag the `Grammar-version`-bump one-way door
(all-or-nothing composite re-render + reviewed golden-IR re-baseline).
```

---

**Authoring rules:**

- **Cite the grammar, never restate it.** Every shape, color, arrow `kind`, header/legend/terminal-artifact rule, and Standard-Import realization is Design 14's, resolved via `diagram-house-profile.md` at the pinned `Grammar-version`. A kit that inlines a hex color or a shape name is a defect — the spec is two-layer (Design 15 AC): semantic model here, house-style resolved from the grammar. A claim with neither a grammar §-anchor nor a Design-15 contract cite is not a kit entry.
- **Roles are grammar tokens.** Every `nodes[].role` / `bands[].role` and every `edges[].kind` must be a token in Design 14's tables. Name the role; cite its shape/color to the grammar; do not author the literal.
- **The profile is always-first.** `diagram-house-profile.md` pins `house@<Grammar-version>` and holds the cross-cutting house rules (one token per role, legend lists exactly the roles present, plain title, paper work-artifact, ASCII labels). Kits reference it; they don't restate it.
- **Name the conformance floor, don't redefine it.** Quote the token's required-slot line from Design 14 §Conformance's per-token table; the gate (Design 15 stage 2) resolves the full rule set from the profile — the kit points, the profile is authoritative.
- **The allow-list is the only composition surface.** Use the exact `<token>.<slot-role> -> { … }` form and the five exact error-code literals (`unresolved-ref`, `token-not-allowed`, `cycle-detected`, `depth-exceeded`, `grammar-version-mismatch`). MVP is the single hardcoded pairing `linear-pipeline.stage -> { circular-cohort }`; every other token's MVP allow-list is empty unless its own design calls for a pairing.
- **No coordinates, no RNG, no clock.** Layout is the IR's pure deterministic lowering (Design 15 stages 1–3 are hermetic); authoring a fill is the semantic model + per-instance style binding only.
- **Two pins on every instance; one `Grammar-version` per composite.** Each fixture carries `schemaVersion` + `style: house@<Grammar-version>`; a composite's full drilldown-reachable set pins one `Grammar-version` (grammar-homogeneous, Design 15).

## Kit roster (shipped + remaining)

Shipped (MVP — the pipeline→cohort-cycle composite):
- `kit-linear-pipeline.md` — ordered stage/gate spine + terminal artifact; declares `linear-pipeline.stage -> { circular-cohort }`.
- `kit-circular-cohort.md` — central work-artifact ringed by ≥3 experts incl. one dissenter; MVP leaf (empty allow-list), the drilldown target of `linear-pipeline.stage`.

Remaining (author from this template + a passing fixture — one per the other 5 tokens):
- `kit-layered-cake-kit.md` — `layered-cake-kit`: ≥1 knowledge band → skill node → expert node (a rich kit fills all four bands — corpus → profile/overlay → per-domain kits/TEMPLATE → evals; a citable-corpus skill like `systems` is a thin 1-band instance — same token, fewer bands). Ends in the expert; no terminal-artifact node. MVP allow-list empty.
- `kit-layered-cake-signal.md` — `layered-cake-signal`: ≥2 source nodes → MCP-tool "decipher" band → reasoning/decision band → expert-or-cohort node. Carries a terminal work-artifact (the decision/verdict). MVP allow-list empty.
- `kit-cross-cutting.md` — `cross-cutting`: a lifecycle base of ≥2 stages + exactly 1 overlay band touching ≥2 of them (translucent yellow, label at the band edge per the probe). Ends in the expert. MVP allow-list empty.
- `kit-hub-and-spoke.md` — `hub-and-spoke`: 1 central-identity node + ≥2 fan-in spokes. Ends in the central identity; no terminal-artifact node. MVP allow-list empty.
- `kit-meta-skill.md` — `meta-skill`: 1 skill-artifact target node + a process loop of ≥2 stages acting on it. Carries a terminal work-artifact (e.g. the new/audited skill, findings report). MVP allow-list empty.
