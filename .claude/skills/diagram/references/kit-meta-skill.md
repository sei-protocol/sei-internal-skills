# meta-skill kit

`token: meta-skill` — one of the 7 spec discriminator values. The conceptual archetype is **Meta-skill** (Design 14 §"The archetype set", #6): a **skill/agent artifact** as the target, with a process loop of stages acting **on** it, iterating-with-cycles (not a one-pass sequence), terminating in the work-artifact the loop produces (the new/audited skill or its findings report). For the skills that operate on the catalog itself — `/audit-skill` (RED→GREEN→REFACTOR against an existing skill), `/author-skill` (the authoring loop against a blank skill). This kit is the **template** for that token: the slot schema it fills, the slot allow-list it declares for composition, the recipe to populate it from a target, and a worked spec fragment.

It **cites** the grammar and the house-profile; it does not restate them. The role→shape/color tokens, the `acts-on` solid loop-into-artifact arrow and the `iterates` loop-back arrow, the header/legend/terminal-artifact rules, and ASCII/`#RRGGBB`/`opacity` realizations live in Design 14 §"The shared visual grammar" / §"Probe outcome" and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. The conformance floor for this token is Design 14 §Conformance: **1 skill-artifact target node + a process loop of >=2 stages acting on it**.

## 1. Slot schema (the token's fillable shape)

A `meta-skill` instance fills these semantic-model slots. Field names and the two-layer split (semantic model vs `style: house@<Grammar-version>` binding) are normative per Design 15; roles are grammar tokens, not literals.

- **`token`**: `meta-skill` (the discriminator).
- **`title`**: the skill's plain human title (Design 14 header rule — never the archetype taxonomy). Set per-instance in the spec; the centered header + subtitle + top-left legend are rendered by the profile, not authored here.
- **`nodes[]`**:
  - **`role: skill-artifact`** — the **target the loop operates on** (exactly one): the SKILL.md+evals artifact being authored or audited. It is a work-shaped artifact; its fill is the neutral paper `#ECF0F1`/`#2C3E50` the grammar gives every work-artifact (Design 14 §Terminal artifact — "work artifact is one color everywhere"). This is the loop's object, not a drilldown-bearing slot for this token (see §2).
  - **`role: stage`** — a step in the process loop (**>=2 required** by the floor). Each `{ id, role: stage, label, order }`; `order` is the position around the loop (e.g. RED → GREEN → REFACTOR). Rendered as the skill-role shape per the grammar.
  - **`role: artifact`** — the **terminal work-artifact** the loop produces (the new/audited skill, or the findings report), present because meta-skill is a terminal-output archetype (Design 14 mandates the terminal artifact for linear-pipeline, meta-skill, and signal). Same paper fill as the central skill-artifact. Last in `order`.
  - `order` is the position in the loop sequence; layout in the IR is a **pure function of (template + node order + grammar constants)** stable-sorted by `id` (Design 15 stage-3 layout determinism) — authors set `order`, never coordinates.
- **`edges[]`** — **authored for this token** (meta-skill is not archetype-derived like circular-cohort; Design 14 §Arrow semantics names its edge kinds explicitly):
  - **`kind: feeds-into`** along the loop stages in `order` (the forward sweep of the process).
  - **`kind: iterates`** for the **loop-back** edge that closes the cycle (a later stage returning to an earlier one — the iterative-with-cycles property that distinguishes meta-skill from a one-pass `linear-pipeline`).
  - **`kind: acts-on`** — the **solid arrow from the loop into the skill artifact** (Design 14 §Arrow semantics: `acts-on` = the loop acting *on* the skill artifact). At least one stage `acts-on` the central `skill-artifact`.
  - **`kind: feeds-into`** from the producing stage into the terminal `artifact`. All four kinds are members of the closed grammar arrow vocabulary (`feeds-into`, `applies-across`, `iterates`, `fans-in`, `acts-on`, `gate-blocks`); do not invent a kind.
- **`drilldown`** — **none for this token in the MVP** (see §2).
- **`style: house@<Grammar-version>`** + **`schemaVersion`** — the two independent version pins every spec carries (Design 15). A composite's full drilldown-reachable set pins one `Grammar-version` (grammar-homogeneous).

The conformance gate (Design 15 stage-2, resolved via the house-profile) rejects an instance lacking the single `skill-artifact` target, with `<2` loop `stage`s, with no loop acting on the artifact (no `acts-on`), or with a per-node color/shape literal (style is resolved from the grammar, never inlined).

## 2. Slot allow-list (composition contract)

This token declares **no outbound drilldown slot in the MVP**:

```
meta-skill.<role> -> { }   # no allowed child token (MVP)
```

- The only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }` (see `kit-linear-pipeline.md` §2); the general any-slot→any-token resolver is **deferred** (Design 15 Non-goals). `meta-skill` therefore has an **empty allow-list** — it is neither a drilldown source nor a typed child in the MVP composite.
- Because every slot-role's allowed-child set here is empty, **any** `drilldown` authored on a meta-skill node fails the resolver with **`token-not-allowed`** (the allow-list is keyed per slot-role; an empty set permits nothing).
- Resolver invariants the engine enforces around this (Design 15 stage-1, enforced engine-side, not by this file): `ref` must resolve (`unresolved-ref`); the drilldown graph is a DAG checked by a **visited-set** (`cycle-detected` — not the no-back-pointer convention alone); depth is bounded at **3** with the root at depth 1, so a chain firing at depth >=4 is **`depth-exceeded`**; every reachable instance shares one `Grammar-version` (`grammar-version-mismatch`). The five codes are the closed kebab-case enum — do not invent or rename.
- Widening this is deferred (Design 15 Non-goals): adding an allowed child token is an additive edit to this allow-list plus a new fixture, only when a second typed pairing is requested.

## 3. Fill-recipe (populate the slot from a target)

Given a skill whose primary object is a skill/agent artifact, which the assignment rule routed to `meta-skill` (Design 14 rule predicate #1):

1. **Identify the skill-artifact target** — the SKILL.md+evals being authored or audited. Author it as the single central `role: skill-artifact` node (paper fill, profile-resolved).
2. **Enumerate the process-loop stages** as `role: stage` nodes (>=2) in loop order; set `order` 1..N (e.g. RED → GREEN → REFACTOR for `/audit-skill`; research → draft → pressure-test for `/author-skill`). Label each with its plain step name (ASCII-only).
3. **Wire the loop** — `kind: feeds-into` along the stages in `order`; add the **`kind: iterates` loop-back** edge that closes the cycle (the iterative property); wire at least one stage `kind: acts-on` into the central `skill-artifact`.
4. **Add the terminal artifact** — meta-skill always produces a durable output, so append one `role: artifact` node, last in `order`, arrowed `feeds-into` from the producing stage (the new/audited skill, or the findings report).
5. **No outbound drilldown** — meta-skill is an MVP leaf with an empty allow-list; do not author a `drilldown` on it (it would be `token-not-allowed`).
6. **Stamp the pins** — `schemaVersion`, `style: house@<current Grammar-version>`.
7. **Validate before render** — the two-tier gate (JSON-Schema + house rule set) must pass; only then does the LucidAdapter emit. The legend lists exactly the roles present (skill-artifact, stage, work-artifact), per the profile.

## 4. Worked example (spec fragment)

`/audit-skill` — the RED→GREEN→REFACTOR loop running over an existing skill's SKILL.md+evals, producing a findings report. The loop acts on the target artifact and closes back to RED.

```yaml
- id: audit-skill-loop
  schemaVersion: 1
  style: house@14.1.0
  token: meta-skill
  title: "Audit Skill"
  subtitle: "RED to GREEN to REFACTOR loop over a skill artifact"
  nodes:
    - { id: target,   role: skill-artifact, label: "SKILL.md + evals under audit" }
    - { id: red,      role: stage, order: 1, label: "RED: find convention gaps" }
    - { id: green,    role: stage, order: 2, label: "GREEN: meet the conventions" }
    - { id: refactor, role: stage, order: 3, label: "REFACTOR: tighten + re-audit" }
    - { id: findings, role: artifact, order: 4, label: "Findings report" }
  edges:
    - { from: red,      to: green,    kind: feeds-into }
    - { from: green,    to: refactor, kind: feeds-into }
    - { from: refactor, to: red,      kind: iterates }
    - { from: red,      to: target,   kind: acts-on }
    - { from: refactor, to: findings, kind: feeds-into }
```

Why it conforms: one central `role: skill-artifact` target; 3 loop `stage`s (floor is >=2) acting on it via an `acts-on` edge; an `iterates` loop-back (REFACTOR→RED) carries the iterative-with-cycles property that distinguishes meta-skill from a one-pass pipeline; a terminal `role: artifact` node arrowed `feeds-into` from the producing stage (meta-skill is a terminal-output archetype); no outbound drilldown (an MVP leaf — authoring one would be `token-not-allowed`); pins `house@14.1.0`. No authored `legend` (profile-derived) and no per-node color/shape literal — style resolves from the grammar via the profile.

## 5. Authoring notes

- **Cite, don't restate.** Shapes, colors, the `acts-on`/`iterates` arrow realizations, header/legend/terminal-artifact rules → Design 14 via `diagram-house-profile.md` at the pinned version. This file owns only the *slot schema + allow-list + recipe + example* for the token.
- **No coordinates, no style literals here.** Loop layout is the IR's pure-function lowering (Design 15 stage 3); style is profile-resolved. Authoring a fill = semantic model only.
- **MVP leaf.** meta-skill's allow-list is empty — the only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }`; the general resolver is deferred (Design 15 Non-goals). Widening is an additive edit to this allow-list plus a new fixture.
- **One-way door:** a `Grammar-version` bump re-renders this instance (and any composite it joins) all-or-nothing and forces a reviewed golden-IR re-baseline (Design 15 §Versioning) — flag it, don't silently re-pin.
