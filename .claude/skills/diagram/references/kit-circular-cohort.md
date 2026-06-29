# circular-cohort kit

`token: circular-cohort` — one of the 7 spec discriminator values. The conceptual archetype is **Circular-cohort** (Design 14 §"The archetype set", #3): a central **work-artifact** node ringed by **expert** nodes, with arrows showing the iterative cross-examination cycle (inward = review a boundary; around-the-ring = rounds; a convergence indicator; the **dissenter** called out). For `/xreview`, `/council`, `/bugbash`, `/coral`. This kit is the **template** for that token: the slot schema it fills, the slot allow-list it declares, the recipe to populate it from a target, and a worked spec fragment.

It **cites** the grammar and the house-profile; it does not restate them. Role→shape/color tokens (expert = hexagon/purple; central work-artifact = paper `#ECF0F1`/`#2C3E50`; the ring-arrow `lineType: curved` clockwise + dashed inward review arrows confirmed by the probe), the header/legend rules, and ASCII/`#RRGGBB`/`opacity` realizations live in Design 14 §"The shared visual grammar" / §"Probe outcome" and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. The conformance floor for this token is Design 14 §Conformance: **a central work-artifact node + ≥3 ring nodes incl. one `role: dissenter`**.

## 1. Slot schema (the token's fillable shape)

A `circular-cohort` instance fills these semantic-model slots. Field names and the two-layer split (semantic model vs `style: house@<Grammar-version>` binding) are normative per Design 15; roles are grammar tokens, not literals.

- **`token`**: `circular-cohort` (the discriminator).
- **`title`**: the skill's plain human title (Design 14 header rule — never the archetype taxonomy). The centered header + subtitle + top-left legend are rendered by the profile, not authored here.
- **`nodes[]`**:
  - **`role: artifact`** — the **central work-artifact under review** (exactly one). Uses the same paper fill as a terminal artifact — "work artifact" is one color everywhere (Design 14). This is the ring's hub; it is **not** a drilldown-bearing slot for this token (see §2).
  - **`role: expert`** — a ring node: a named reviewing expert/lens. **≥3 required.** Each `{ id, role: expert, label }`; `label` is the lens name (ASCII-only).
  - **`role: dissenter`** — exactly one ring node tagged as the assigned dissenter (a distinguished `expert`; the grammar highlights it). Required by the conformance floor.
  - Ring **order** is positional-around-the-ring; layout (ring radius, angular placement) is a **pure function of (template + ring-node count/order + grammar constants)** stable-sorted by `id` (Design 15 stage-3 determinism). Authors set ring membership and order, never coordinates.
- **`edges[]`** — **archetype-derived for this token; author NONE.** The around-the-ring `iterates` cycle (curved, clockwise per the grammar) AND the dashed inward review arrows from each ring node to the central artifact are **derived at lowering from (ring nodes + central artifact)** — they are not authored spec edge-kinds, and there is no `reviews` spec edge-kind. The author sets nodes only; the lowering rule synthesizes the ring cycle and the inward review arrows from the ring membership + the hub. A convergence indicator is likewise a grammar attribute, not a new node. (The schema's closed `edges[].kind` enum is the grammar set for tokens that DO author edges, e.g. `linear-pipeline` with `feeds-into`/`gate-blocks` — circular-cohort does not author edges.)
- **`drilldown`** — **none for this token in the MVP** (see §2). A circular-cohort is a leaf in the MVP composite; it is the *child* of a `linear-pipeline.stage`, holding no back-pointer to its parent.
- **`style: house@<Grammar-version>`** + **`schemaVersion`** — the two independent version pins (Design 15). In a composite, this instance pins the *same* `Grammar-version` as its parent (grammar-homogeneous).

The conformance gate (Design 15 stage-2, resolved via the house-profile) rejects an instance lacking the central artifact, with `<3` ring nodes, with no `role: dissenter`, or with a per-node color/shape literal.

## 2. Slot allow-list (composition contract)

This token declares **no outbound drilldown slot in the MVP**:

```
circular-cohort.<role> -> { }   # no allowed child token (MVP)
```

- In the MVP pipeline→cohort-cycle composite, `circular-cohort` is the **drilldown target** (the child), reached *from* `linear-pipeline.stage` — see `kit-linear-pipeline.md` §2 (`linear-pipeline.stage -> { circular-cohort }`). It does not itself drill further down.
- Because every slot-role's allowed-child set here is empty, **any** `drilldown` authored on a circular-cohort node fails the resolver with **`token-not-allowed`** (the allow-list is keyed per slot-role; an empty set permits nothing).
- The child holds **no back-pointer** to its parent (Design 15) — parent→child is by the parent's `ref` only. The resolver's **visited-set** cycle check (`cycle-detected`), not the no-back-pointer convention, is what actually prevents a cycle; the **fixed depth bound of 3** (root = depth 1, fires at ≥4 → `depth-exceeded`) and `unresolved-ref` / `grammar-version-mismatch` are enforced engine-side per Design 15 stage 1.
- Widening this is deferred (Design 15 Non-goals): adding an allowed child token is an additive edit to this allow-list plus a new fixture, only when a second typed pairing is requested.

## 3. Fill-recipe (populate the slot from a target)

Given a slate/cohort skill that the assignment rule routed to `circular-cohort` (Design 14 rule predicate #2), or a `linear-pipeline.stage` that expands into one:

1. **Identify the work-artifact under review** — the design/PR/diff/spec the cohort examines. Author it as the single central `role: artifact` node (paper fill, profile-resolved).
2. **Enumerate the reviewing lenses** as `role: expert` ring nodes (≥3) — the named specialists/lenses doing the cross-examination. Label each with its lens name (ASCII-only).
3. **Tag the dissenter** — exactly one ring node is `role: dissenter` (the assigned-dissent lens). The floor requires it; an untagged cohort fails the gate.
4. **Do NOT author edges** — for circular-cohort the cycle is archetype-derived. The around-the-ring `iterates` arrows (curved, clockwise) and the dashed inward review arrows (each ring node → central artifact) are synthesized at lowering from (ring nodes + central artifact); there is no `reviews` spec edge-kind and the author writes no `edges:`. The convergence is a grammar attribute, not a node.
5. **No outbound drilldown** — a circular-cohort is an MVP leaf; do not author a `drilldown` on it (it would be `token-not-allowed`).
6. **Stamp the pins** — `schemaVersion`, `style: house@<current Grammar-version>`. As a child in a composite, pin the **same** `Grammar-version` as the parent pipeline.
7. **Validate before render** — the two-tier gate must pass; the legend lists exactly the roles present (work-artifact, expert, dissenter).

## 4. Worked example (spec fragment)

The MVP pipeline→cohort-cycle composite, child half. This is the `circular-cohort` instance the `workstream-pipeline` stage `s3` drills into via `drilldown: { ref: xreview-cohort, rel: expands-to }` (see `kit-linear-pipeline.md` §4). It is a separately-addressable top-level instance with its own `id`/`token`/pins, holds **no back-pointer**, and pins the same `house@14.1.0`.

```yaml
- id: xreview-cohort
  schemaVersion: 1
  style: house@14.1.0
  token: circular-cohort
  title: "Xreview"
  nodes:
    - { id: work,   role: artifact,  label: "Design under review" }
    - { id: sys,    role: expert,    label: "systems-engineer" }
    - { id: prod,   role: expert,    label: "product-manager" }
    - { id: prose,  role: expert,    label: "prose-steward" }
    - { id: data,   role: dissenter, label: "data-architecture (dissenter)" }
# No `edges:` — the ring `iterates` cycle and the dashed inward review arrows are
# archetype-derived at lowering from (ring nodes + central artifact).
```

Why it conforms: one central `role: artifact` hub; 4 ring nodes (floor is ≥3) including exactly one `role: dissenter`; **no authored `edges:`** — the around-the-ring `iterates` cycle and the dashed inward review arrows are derived at lowering from (ring nodes + central artifact); no outbound drilldown (a leaf — authoring one would be `token-not-allowed`); pins `house@14.1.0` matching its parent (grammar-homogeneous composite). No per-node color/shape literal — style resolves from the grammar via the profile.

## 5. Authoring notes

- **Cite, don't restate.** Shapes, colors, the curved-ring/dashed-inward arrow realizations, header/legend rules → Design 14 via `diagram-house-profile.md` at the pinned version. This file owns only the *slot schema + allow-list + recipe + example* for the token.
- **No coordinates, no style literals here.** Ring layout is the IR's pure-function lowering (Design 15 stage 3); style is profile-resolved.
- **MVP leaf.** circular-cohort is the drilldown *target*, not a source; its allow-list is empty until a second typed pairing is requested (Design 15 Deferred).
- **One-way door:** a `Grammar-version` bump re-renders this instance as part of its composite's all-or-nothing re-render and forces a reviewed golden-IR re-baseline (Design 15 §Versioning) — flag it.
