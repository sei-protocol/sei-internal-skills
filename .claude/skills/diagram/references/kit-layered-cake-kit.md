# layered-cake-kit kit

`token: layered-cake-kit` — one of the 7 spec discriminator values. The conceptual archetype is **Layered-cake** (Design 14 §"The archetype set", #1, **kit variant**): a vertical stack of labeled knowledge **bands** feeding **upward** into a single **skill** node, which feeds the **agentic domain expert** (persona-backed-by-skill) at the top — "what knowledge composes into this expert." A rich pluggable-kit skill fills all four bands (citable corpus pins -> always-first profile overlay -> per-domain kits + TEMPLATE -> evals); a citable-corpus skill (e.g. `systems`) is a **thin instance** with just the corpus band -> skill -> expert — the *same token at a smaller band-count*, not a separate shape. This kit is the **template** for that token: the slot schema it fills, the slot allow-list it declares, the recipe to populate it from a target, and a worked spec fragment.

It **cites** the grammar and the house-profile; it does not restate them. Role->shape/color tokens (knowledge band = wide background rectangle / info-source blue; skill = rounded rectangle / teal; agentic expert = hexagon / purple), the bottom->top `feeds-into` direction, the header/legend rules, and ASCII/`#RRGGBB`/`opacity` realizations live in Design 14 §"The shared visual grammar" / §"Probe outcome" and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. The conformance floor for this token is Design 14 §Conformance: **>=1 knowledge band -> skill node -> expert node** *(the kit-strategy seed AC additionally targets >=3 bands; the general floor is 1, so a thin instance conforms)*.

## 1. Slot schema (the token's fillable shape)

A `layered-cake-kit` instance fills these semantic-model slots. Field names and the two-layer split (semantic model vs `style: house@<Grammar-version>` binding) are normative per Design 15; roles are grammar tokens, not literals.

- **`token`**: `layered-cake-kit` (the discriminator).
- **`title`**: the skill's plain human title (Design 14 header rule — kit skills use "<Domain> Knowledge Kit", e.g. "Platform Knowledge Kit"; never the archetype taxonomy). Set per-instance; the centered header + subtitle + top-left legend are rendered by the profile, not authored here.
- **`bands[]`** — the **stacked knowledge corpus**; this is the token's ordered slot (the stack, NOT a node spine). Each band `{ id, role, label, order }`, **ordered bottom->top by `order`**:
  - **`role: knowledge`** — a knowledge band: a citable corpus pin, the always-first profile/overlay, a per-domain-kits (+ TEMPLATE) layer, an evals layer. A rich kit fills all four; a thin instance fills one. **>=1 required** (the floor). Rendered as the wide background rectangle behind its members per the grammar; info-source color, profile-resolved.
  - `order` is a total order over the stack (bottom = lowest `order`); layout in the IR is a **pure function of (template + band order + grammar constants)** stable-sorted by `id` (Design 15 stage-3 layout determinism) — authors set `order`, never coordinates. (Layered-cake carries its stacked sequence in **`bands[]`**, not in a `nodes[].order` spine — that ordering field is for the node-spine tokens, linear-pipeline / cross-cutting base.)
- **`nodes[]`** — the **culmination**, sitting above the band stack:
  - **`role: skill`** — the single skill node the bands feed into (rounded rectangle / teal, profile-resolved). Exactly one.
  - **`role: expert`** — the single agentic domain expert (persona-backed-by-skill) the skill feeds (hexagon / purple, profile-resolved). Exactly one. **This is the terminus** — layered-cake-kit ends in the **expert**; it adds **no** terminal work-artifact node (Design 14 §Terminal artifact — only linear-pipeline / meta-skill / signal carry one).
- **`edges[]`** — `{ from, to, kind: feeds-into }`, reading **bottom->top**: each knowledge band -> the skill node, and the skill node -> the expert node. `feeds-into` is the grammar-fixed kind for the upward stack; the author wires these (unlike circular-cohort, whose edges are archetype-derived).
- **`drilldown`** — **none for this token in the MVP** (see §2).
- **`style: house@<Grammar-version>`** + **`schemaVersion`** — the two independent version pins every spec carries (Design 15). In a composite, every reachable instance pins the same `Grammar-version` (grammar-homogeneous).

The conformance gate (Design 15 stage-2, resolved via the house-profile) rejects an instance with `<1` knowledge band, lacking the skill or expert node, ending in anything other than the expert (no extra terminal artifact), or carrying a per-band/per-node color/shape literal (style is resolved from the grammar, never inlined).

## 2. Slot allow-list (composition contract)

This token declares **no outbound drilldown slot in the MVP**:

```
layered-cake-kit.<role> -> { }   # no allowed child token (MVP)
```

- The only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }` (see `kit-linear-pipeline.md` §2); the **general any-slot->any-token resolver is deferred** (Design 15 Non-goals / Deferred). `layered-cake-kit` is therefore an **MVP leaf** — neither a drilldown source nor (in the MVP) a target.
- Because every slot-role's allowed-child set here is empty, **any** `drilldown` authored on a `layered-cake-kit` band or node fails the resolver with **`token-not-allowed`** (the allow-list is keyed per slot-role; an empty set permits nothing).
- Resolver invariants the engine enforces around this (Design 15 stage-1, enforced engine-side, not by this file): a `drilldown.ref` must resolve (`unresolved-ref`); the child `token` must be in the slot's set (`token-not-allowed`); the drilldown graph is a DAG checked by a **visited-set** (`cycle-detected` — not the no-back-pointer convention alone); depth is bounded at **3** with the root at depth 1, so a chain firing at depth >=4 is **`depth-exceeded`**; every reachable instance shares one `Grammar-version` (`grammar-version-mismatch`). The five codes are the closed kebab-case enum — do not invent or rename.
- Widening this is deferred (Design 15 Non-goals): adding an allowed child token is an additive edit to this allow-list plus a new fixture, only when a second typed pairing is requested.

## 3. Fill-recipe (populate the slot from a target)

Given a knowledge skill backing a named agentic expert that the assignment rule routed to `layered-cake-kit` (Design 14 rule predicate #3 — either the pluggable-kit model or a citable-corpus operating manual):

1. **Enumerate the knowledge bands** bottom->top as `role: knowledge` bands, set `order` 1..N: the citable **corpus** pins (lowest), the **always-first profile/overlay**, the **per-domain kits (+ TEMPLATE)** layer, the **evals** layer (highest). A rich pluggable-kit skill fills all four; a citable-corpus skill fills just the corpus band (a thin instance — still conforms, floor is >=1). Label each with its plain layer name (ASCII-only — grammar authoring constraint).
2. **Add the skill node** — one `role: skill` node above the stack: the skill itself (its plain "<Domain> Knowledge Kit" identity).
3. **Add the expert node** — one `role: expert` node above the skill: the agentic domain expert (persona-backed-by-skill) the skill composes into. This is the terminus; do **not** append a terminal work-artifact node (layered-cake-kit ends in the expert).
4. **Wire the edges** `kind: feeds-into`, bottom->top: each knowledge band -> the skill node; the skill node -> the expert node.
5. **No outbound drilldown** — layered-cake-kit is an MVP leaf; do not author a `drilldown` on a band or node (it would be `token-not-allowed`).
6. **Stamp the pins** — `schemaVersion`, `style: house@<current Grammar-version>`. In a composite, every reachable instance pins the *same* `Grammar-version`.
7. **Validate before render** — the two-tier gate (JSON-Schema + house rule set) must pass; only then does the LucidAdapter emit. The legend lists exactly the roles present (knowledge / skill / expert), per the profile.

## 4. Worked example (spec fragment)

A rich pluggable-kit cake for `/platform` (the platform-engineer agent's knowledge kit): four knowledge bands bottom->top feeding one skill node feeding the agentic expert. Ends in the expert; no terminal-artifact node.

```yaml
- id: platform-kit
  schemaVersion: 1
  style: house@14.1.0
  token: layered-cake-kit
  title: "Platform Knowledge Kit"
  bands:
    - { id: corpus,  role: knowledge, order: 1, label: "Citable corpus: OpenGitOps, Kustomize, PSS, EKS, NSA/CISA" }
    - { id: profile, role: knowledge, order: 2, label: "Always-first Sei-platform profile (Flux, two-layer Kustomize, Pod Identity, SOPS)" }
    - { id: kits,    role: knowledge, order: 3, label: "Per-domain kits + TEMPLATE" }
    - { id: evals,   role: knowledge, order: 4, label: "Evals" }
  nodes:
    - { id: skill,  role: skill,  label: "/platform" }
    - { id: expert, role: expert, label: "platform-engineer" }
  edges:
    - { from: corpus,  to: skill,  kind: feeds-into }
    - { from: profile, to: skill,  kind: feeds-into }
    - { from: kits,    to: skill,  kind: feeds-into }
    - { from: evals,   to: skill,  kind: feeds-into }
    - { from: skill,   to: expert, kind: feeds-into }
```

Why it conforms: 4 ordered `role: knowledge` bands bottom->top (floor is >=1) -> one `role: skill` node -> one `role: expert` node, all `feeds-into`; ends in the **expert** with no extra terminal-artifact node (Design 14 §Terminal artifact); no `legend` authored (profile-derived — lists exactly knowledge/skill/expert); no outbound drilldown (an MVP leaf — authoring one would be `token-not-allowed`); pins `house@14.1.0`. No per-band/per-node color/shape literal — style resolves from the grammar via the profile. (A thin instance — e.g. `systems` — drops `profile`/`kits`/`evals` and keeps one corpus band -> skill -> expert; same token, fewer bands, still conforms.)

## 5. Authoring notes

- **Cite, don't restate.** Shapes, colors, the bottom->top `feeds-into` direction, header/legend/terminal-artifact rules -> Design 14 via `diagram-house-profile.md` at the pinned version. This file owns only the *slot schema + allow-list + recipe + example* for the token.
- **No coordinates, no style literals here.** Stack layout is the IR's pure-function lowering (Design 15 stage 3); style is profile-resolved. Authoring a fill = semantic model only.
- **Ordered `bands[]`, not a node spine.** The stacked sequence is carried by `order` on `bands[]` (bottom->top); the skill + expert are the culmination `nodes[]`. Do not invent a different ordering field.
- **Ends in the expert.** layered-cake-kit adds no terminal work-artifact node — only linear-pipeline / meta-skill / signal do (Design 14 §Terminal artifact).
- **MVP leaf.** The allow-list is empty; the general any-slot->any-token resolver is deferred (Design 15) — widening is an additive edit here plus a fixture, only when a second typed pairing is requested.
- **One-way door:** a `Grammar-version` bump re-renders this instance as part of its composite's all-or-nothing re-render and forces a reviewed golden-IR re-baseline (Design 15 §Versioning) — flag it, don't silently re-pin.
