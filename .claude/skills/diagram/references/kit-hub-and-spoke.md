# hub-and-spoke kit

`token: hub-and-spoke` — one of the 7 spec discriminator values. The conceptual archetype is **Hub-and-spoke (fan-in)** (Design 14 §"The archetype set", #5): a central **identity** node (a bet, or the bet<->design<->issue<->PR graph) with spoke artifacts that feed or hang off it, arrowed INTO the hub. For the graph-decorating skills: `/execution-plan`, `/impact-weekly`, `/impact-portfolio`. This kit is the **template** for that token: the slot schema it fills, the slot allow-list it declares for composition, the recipe to populate it from a target, and a worked spec fragment.

It **cites** the grammar and the house-profile; it does not restate them. The role->shape/color tokens (central identity = circle/identity-orange; spoke work-artifact = document/paper `#ECF0F1`/`#2C3E50`), the `fans-in` arrow semantics, the header/legend/terminal-artifact rules, and ASCII/`#RRGGBB`/`opacity` realizations live in Design 14 §"The shared visual grammar" / §"Probe outcome" and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. The conformance floor for this token is Design 14 §Conformance: **1 central-identity node + >=2 fan-in spokes**.

## 1. Slot schema (the token's fillable shape)

A `hub-and-spoke` instance fills these semantic-model slots. Field names and the two-layer split (semantic model vs `style: house@<Grammar-version>` binding) are normative per Design 15; roles are grammar tokens, not literals.

- **`token`**: `hub-and-spoke` (the discriminator).
- **`title`**: the skill's plain human title (Design 14 header rule — never the archetype taxonomy). Set per-instance in the spec; the centered header + subtitle + top-left legend are rendered by the profile, not authored here.
- **`nodes[]`**:
  - **`role: identity`** — the **central identity** the spokes converge on (exactly one): a bet, or the bet<->design<->issue<->PR graph. Rendered as the identity shape per the grammar (circle, identity-orange). This is the diagram's **terminus** — `hub-and-spoke` ends in the central identity and adds **no extra terminal-artifact node** (Design 14 §Terminal artifact; contrast linear-pipeline/meta-skill/signal, which append one). It is **not** a drilldown-bearing slot for this token (see §2).
  - **`role: artifact`** — a **spoke** that feeds the hub. **>=2 required.** Each `{ id, role: artifact, label }`; the spoke artifacts (a week's PRs, a set of issues, the per-bet reports) use the paper work-artifact fill (same paper everywhere — Design 14). The catalog note that a graph-decorator's *produced* output (e.g. the appended Weekly-log entry) is itself a paper artifact off the hub is honored by authoring that produced output as one more `role: artifact` spoke — it is a fan-in member, not a separate terminal node.
  - Spokes carry no `order`: fan-in is a SET converging on the hub, not an ordered spine. (`order` is for the node-spine tokens — linear-pipeline, cross-cutting base; ordered `bands[]` for layered-cake.) Spoke layout (angular placement around the hub) is a **pure function of (template + spoke-node count/membership + grammar constants)** stable-sorted by `id` (Design 15 stage-3 layout determinism) — authors set spoke membership, never coordinates.
- **`edges[]`** — `{ from: <spoke-id>, to: <hub-id>, kind: fans-in }`, one per spoke, each arrowed INTO the central identity. Unlike `circular-cohort` (whose ring/inward arrows are archetype-derived and authored as NONE), `hub-and-spoke` **DOES author its edges**: the `fans-in` kind is a member of the closed grammar arrow vocabulary (Design 14 §Arrow semantics: *fans-in* = solid arrow into the central identity) and the spec's `edges[].kind` enum carries it. Every edge points `to` the hub.
- **`drilldown`** — **none for this token in the MVP** (see §2).
- **`style: house@<Grammar-version>`** + **`schemaVersion`** — the two independent version pins every spec carries (Design 15). A composite's full drilldown-reachable set pins one `Grammar-version` (grammar-homogeneous).

The conformance gate (Design 15 stage-2, resolved via the house-profile) rejects an instance lacking the central identity, with `<2` spokes, with a spoke edge that does not arrow INTO the hub, or with a per-node color/shape literal (style is resolved from the grammar, never inlined).

## 2. Slot allow-list (composition contract)

This token declares **no outbound drilldown slot in the MVP**:

```
hub-and-spoke.<role> -> { }   # no allowed child token (MVP)
```

- The only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }` (declared in `kit-linear-pipeline.md` §2); the **general any-slot->any-token resolver is deferred** (Design 15 Non-goals / Deferred). `hub-and-spoke` is therefore neither a drilldown source nor a typed drilldown target in the MVP — its allow-list is **empty**.
- Because every slot-role's allowed-child set here is empty, **any** `drilldown` authored on a `hub-and-spoke` node fails the resolver with **`token-not-allowed`** (the allow-list is keyed per slot-role; an empty set permits nothing).
- Resolver invariants enforced engine-side (Design 15 stage-1, not by this file): a `ref` must resolve (`unresolved-ref`); the child `token` must be in the slot's set (`token-not-allowed`); the drilldown graph is a DAG checked by a **visited-set** (`cycle-detected` — not the no-back-pointer convention alone); depth is bounded at **3** with the root at depth 1, so a chain firing at depth >=4 is **`depth-exceeded`**; every reachable instance shares one `Grammar-version` (`grammar-version-mismatch`). The five codes are the closed kebab-case enum — do not invent or rename.
- Widening this is deferred (Design 15 Deferred): adding an allowed child token is an additive edit to this allow-list plus a new fixture, only when a second typed pairing is requested.

## 3. Fill-recipe (populate the slot from a target)

Given a graph-decorating skill that the assignment rule routed to `hub-and-spoke` (Design 14 rule predicate #5 — reading or decorating the bet<->design<->issue<->PR graph, fan-in to a central identity):

1. **Identify the central identity** — the one thing the spokes converge on (a bet, a week's bet entry, a cross-project report, or the bet<->design<->issue<->PR graph). Author it as the single central `role: identity` node (identity shape/color, profile-resolved).
2. **Enumerate the spokes** as `role: artifact` nodes (>=2) — the artifacts that feed or hang off the hub (a week's PRs, the mapped issues, the per-bet reports). Label each with its plain artifact name (ASCII-only — grammar authoring constraint).
3. **Add the produced output as a spoke iff the skill emits one** — a graph-decorator's durable output (e.g. `/impact-weekly`'s appended Weekly-log entry) is authored as one more `role: artifact` spoke off the hub (per the catalog note), NOT as a separate terminal node — `hub-and-spoke` terminates in the central identity.
4. **Wire the fan-in edges** — one `{ from: <spoke>, to: <hub>, kind: fans-in }` per spoke; every edge points INTO the central identity. (These ARE authored, unlike circular-cohort's archetype-derived ring.)
5. **No outbound drilldown** — `hub-and-spoke` is an MVP leaf (empty allow-list); do not author a `drilldown` on it (it would be `token-not-allowed`).
6. **Stamp the pins** — `schemaVersion`, `style: house@<current Grammar-version>`. In a composite, every reachable instance pins the *same* `Grammar-version`.
7. **Validate before render** — the two-tier gate (JSON-Schema + house rule set) must pass; only then does the LucidAdapter emit. The legend lists exactly the roles present (identity, work-artifact), per the profile.

## 4. Worked example (spec fragment)

`/impact-weekly` — a graph-decorating skill: a week's work (Linear issues + linked PRs) fans IN to one bet's Weekly-log entry, and the appended entry itself hangs off the hub as a produced artifact (the catalog's paper-terminal-off-the-hub note).

```yaml
- id: impact-weekly
  schemaVersion: 1
  style: house@14.1.0
  token: hub-and-spoke
  title: "Impact Weekly"
  subtitle: "A week of work fans in to one bet entry"
  nodes:
    - { id: bet,     role: identity, label: "Sei Agentic Mesh (bet)" }
    - { id: prs,     role: artifact, label: "This week's merged PRs" }
    - { id: issues,  role: artifact, label: "Linear issues closed" }
    - { id: reviews, role: artifact, label: "PR reviews + xreviews" }
    - { id: entry,   role: artifact, label: "Appended Weekly-log entry" }
  edges:
    - { from: prs,     to: bet, kind: fans-in }
    - { from: issues,  to: bet, kind: fans-in }
    - { from: reviews, to: bet, kind: fans-in }
    - { from: entry,   to: bet, kind: fans-in }
```

Why it conforms: one central `role: identity` hub; 4 fan-in spokes (floor is 1 identity + >=2 spokes) each arrowed INTO the hub with `kind: fans-in` (the grammar's fan-in arrow); ends in the central identity with no extra terminal node — the produced Weekly-log entry is itself a paper spoke off the hub (catalog note), not a separate terminus; no outbound drilldown (an MVP leaf — authoring one would be `token-not-allowed`); pins `house@14.1.0`. No per-node color/shape literal and no authored `legend` — style and the roles-present legend resolve from the grammar via the profile.

## 5. Authoring notes

- **Cite, don't restate.** Shapes, colors, the `fans-in` arrow realization, header/legend/terminal-artifact rules -> Design 14 via `diagram-house-profile.md` at the pinned version. This file owns only the *slot schema + allow-list + recipe + example* for the token.
- **No coordinates, no style literals here.** Spoke layout is the IR's pure-function lowering (Design 15 stage 3); style is profile-resolved. Authoring a fill = semantic model only.
- **Author the fan-in edges (this token is not edge-derived).** Unlike circular-cohort, `hub-and-spoke` writes its `fans-in` edges explicitly; every edge points INTO the central identity.
- **Terminates in the identity, not a terminal artifact.** `hub-and-spoke` adds no extra terminal-output node; a produced output is one more spoke off the hub (catalog note), per Design 14 §Terminal artifact.
- **MVP leaf.** The only MVP pairing is `linear-pipeline.stage -> { circular-cohort }`; this token's allow-list is empty (the general resolver is deferred per Design 15) until a typed pairing is requested.
- **One-way door:** a `Grammar-version` bump re-renders this instance as part of its composite's all-or-nothing re-render and forces a reviewed golden-IR re-baseline (Design 15 §Versioning) — flag it, don't silently re-pin.
