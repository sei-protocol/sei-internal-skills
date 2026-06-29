# linear-pipeline kit

`token: linear-pipeline` — one of the 7 spec discriminator values. The conceptual archetype is **Linear-pipeline** (Design 14 §"The archetype set", #4): an ordered left→right sequence of stages and gates for a *procedural* skill, terminating (where the skill produces a durable output) in a work-artifact node. This kit is the **template** for that token: the slot schema it fills, the slot allow-list it declares for composition, the recipe to populate it from a target, and a worked spec fragment.

It **cites** the grammar and the house-profile; it does not restate them. The role→shape/color tokens, arrow semantics, header/legend/terminal-artifact rules, and ASCII/`#RRGGBB`/`opacity` realizations live in Design 14 §"The shared visual grammar" / §"Probe outcome" and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. The conformance floor for this token is Design 14 §Conformance: **≥2 ordered stages, left→right; any gate rendered as a bold-bordered node**.

## 1. Slot schema (the token's fillable shape)

A `linear-pipeline` instance fills these semantic-model slots. Field names and the two-layer split (semantic model vs `style: house@<Grammar-version>` binding) are normative per Design 15; roles are grammar tokens, not literals.

- **`token`**: `linear-pipeline` (the discriminator).
- **`title`**: the skill's plain human title (Design 14 header rule — never the archetype taxonomy). Set per-instance in the spec; the centered header + subtitle + top-left legend are rendered by the profile, not authored here.
- **`nodes[]`** — the ordered spine. Each node `{ id, role, label, order }`:
  - **`role: stage`** — an ordinary procedural step. The **drilldown-bearing slot-role** for this token (see §2). Rendered as the skill-role shape per the grammar.
  - **`role: gate`** — a checkpoint / decision gate. Rendered as the **bold-bordered gate node** (grammar: `rectangle` + `stroke.width:5`, gate-red). A gate is a stage that blocks; it is still part of the ordered `order` sequence.
  - **`role: artifact`** — the **terminal work-artifact** the skill produces (grammar paper fill `#ECF0F1` / `#2C3E50` text), present iff the skill emits a durable output. Last in `order`. (Design 14 mandates the terminal artifact for linear-pipeline, meta-skill, and signal.)
  - `order` is a total order over the spine; layout in the IR is a **pure function of (template + node order + grammar constants)** stable-sorted by `id` (Design 15 stage-3 layout determinism) — authors set `order`, never coordinates.
- **`edges[]`** — `{ from, to, kind: feeds-into }` along the spine; the edge **into** a gate stays `feeds-into`, and the gate's **outbound** edge (gate → the step it guards) is `kind: gate-blocks` (the blocked transition). Arrow tokens are grammar-fixed; pipelines read left→right. (See §4 + the golden IR: `feeds-into` arrives at a gate, `gate-blocks` leaves it.)
- **`drilldown`** (optional, on a `stage` node only) — `{ ref: <child-id>, rel: <relationship> }`, by id, no back-pointer. See §2.
- **`style: house@<Grammar-version>`** + **`schemaVersion`** — the two independent version pins every spec carries (Design 15). A composite's full drilldown-reachable set pins one `Grammar-version` (grammar-homogeneous).

The conformance gate (Design 15 stage-2, resolved via the house-profile) rejects an instance with `<2` stages, with a gate not rendered bold-bordered, or with a per-node color/shape literal (style is resolved from the grammar, never inlined).

## 2. Slot allow-list (composition contract)

This token declares exactly one composable slot-role and its permitted child tokens:

```
linear-pipeline.stage -> { circular-cohort }
```

- A `stage` node MAY carry `drilldown: { ref, rel }` pointing at a separately-addressable child instance whose `token` is in the set above. **MVP set is `{ circular-cohort }` only** — a stage expands to a cohort-cycle (e.g. a "verify" stage → an `/xreview` ring). Any other child `token` in this slot fails the resolver with **`token-not-allowed`**.
- Only the `stage` role bears a drilldown here. A `gate` or `artifact` node carrying a drilldown is also `token-not-allowed` (the allow-list is keyed per slot-role).
- Resolver invariants the parent inherits (Design 15 stage-1, enforced by the engine, not by this file): `ref` must resolve (`unresolved-ref`); the drilldown graph is a DAG checked by a **visited-set** (`cycle-detected` — not by the no-back-pointer convention alone); depth is bounded at **3** with the root at depth 1, so a chain firing at depth ≥4 is **`depth-exceeded`**; every reachable instance shares one `Grammar-version` (`grammar-version-mismatch`).
- This is the **MVP hardcoded pairing**; a general any-slot→any-token resolver is deferred (Design 15 Non-goals). Adding a second allowed child token is an additive edit *here*, in this allow-list, and a new fixture.

## 3. Fill-recipe (populate the slot from a target)

Given a procedural skill/flow that the assignment rule routed to `linear-pipeline` (Design 14 rule predicate #7):

1. **Enumerate the ordered steps** of the procedure as `stage` nodes, in execution order; set `order` 1..N. Label each with its plain step name (ASCII-only — grammar authoring constraint).
2. **Mark checkpoints/gates** — any step that blocks progress (a human checkpoint, a fail-closed guard, a review-gate) becomes `role: gate`, keeping its `order` slot. The edge **into** the gate is `feeds-into`; wire the gate's **outbound** edge (gate → the step it guards) as `kind: gate-blocks` (the blocked transition).
3. **Add the terminal artifact** iff the skill produces a durable output: append one `role: artifact` node, last in `order`, arrowed `feeds-into` from the producing stage. Omit it for a skill with no durable terminus.
4. **Decide drilldowns** — for any `stage` that is *itself* a cohort cross-examination (a slate examining a work product), set `drilldown: { ref: <child-id>, rel: expands-to }` to a separately-authored `circular-cohort` instance. Keep the child a peer top-level instance with its own `id`/`token`/pins; the parent references it by id only.
5. **Stamp the pins** — `schemaVersion`, `style: house@<current Grammar-version>`. In a composite, every reachable instance pins the *same* `Grammar-version`.
6. **Validate before render** — the two-tier gate (JSON-Schema + house rule set) must pass; only then does the LucidAdapter emit. The legend lists exactly the roles present (stage/gate/artifact), per the profile.

## 4. Worked example (spec fragment)

The MVP pipeline→cohort-cycle composite, parent half. A `/workstream`-style pipeline whose verify stage drills down into an `/xreview` cohort (the child instance is authored in `kit-circular-cohort.md` §4; both pin one grammar version).

```yaml
- id: workstream-pipeline
  schemaVersion: 1
  style: house@14.1.0
  token: linear-pipeline
  title: "Workstream"
  nodes:
    - { id: s1, role: stage, order: 1, label: "Design via /council" }
    - { id: s2, role: gate,  order: 2, label: "Checkpoint: scope sign-off" }
    - { id: s3, role: stage, order: 3, label: "Verify via /xreview",
        drilldown: { ref: xreview-cohort, rel: expands-to } }
    - { id: s4, role: gate,  order: 4, label: "Review-gate: unanimous + Bugbot" }
    - { id: s5, role: artifact, order: 5, label: "Merged PR" }
  edges:
    - { from: s1, to: s2, kind: feeds-into }
    - { from: s2, to: s3, kind: gate-blocks }
    - { from: s3, to: s4, kind: feeds-into }
    - { from: s4, to: s5, kind: gate-blocks }
```

Why it conforms: 5 ordered stages left→right (floor is ≥2); two gates rendered bold-bordered; a terminal `artifact` node (skill produces a durable output); `s3.drilldown` targets a `circular-cohort` child — in the `linear-pipeline.stage -> { circular-cohort }` allow-list; child held by id with no back-pointer; both instances pin `house@14.1.0`. No per-node color/shape literal — style resolves from the grammar via the profile.

## 5. Authoring notes

- **Cite, don't restate.** Shapes, colors, arrow kinds, header/legend/terminal-artifact rules → Design 14 via `diagram-house-profile.md` at the pinned version. This file owns only the *slot schema + allow-list + recipe + example* for the token.
- **No coordinates, no style literals here.** Layout is the IR's pure-function lowering (Design 15 stage 3); style is profile-resolved. Authoring a fill = semantic model only.
- **The allow-list is the composition contract.** Changing `linear-pipeline.stage -> { … }` is the one place composition for this token widens; it ships with a fixture and stays the MVP single pairing until a second is requested (Design 15 Deferred).
- **One-way door:** a `Grammar-version` bump on a composite re-renders its full drilldown-reachable set all-or-nothing and forces a reviewed golden-IR re-baseline (Design 15 §Versioning) — flag it, don't silently re-pin.
