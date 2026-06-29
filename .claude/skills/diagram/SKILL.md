---
name: diagram
category: platform-infra
model: claude-opus-4-8
description: "Use when generating, regenerating, composing, or validating a house-grammar skill/architecture diagram from a git-tracked spec — '/diagram', 'diagram this skill', 'regenerate the xreview diagram', 'render the pipeline-to-cohort composite', 'validate this diagram spec', 'is this render stale'. The git-tracked spec is the single source of truth; an always-first house-style profile + per-token kits (one per the 7 `token` values) bind it to the Design-14 grammar by version, and it renders one-way to Lucid as a regenerable output. Backs the diagram-architect agent. NOT for defining or changing the grammar — colors/shapes/legend/assignment rule live in Design 14 (Grammar-version 14.1.0), cited not restated. NOT for editing diagrams directly in Lucid (one-way render; edits are lost). NOT for authoring skills (/author-skill) or capturing designs (/design). Generates diagrams to the house standard; does not author the standard."
---

# Diagram

Generate, compose, validate, and reproducibly regenerate **house-grammar diagrams** from a git-tracked spec. A diagram is authored once as a two-layer spec, lowered through a deterministic engine to a canonical IR, and rendered to Lucid via an adapter whose identity is a committed manifest — so re-running on an unchanged spec at a pinned grammar version produces the same IR and the same standard-conformant render, with no per-diagram style decisions. A *reference/technique* skill with a method spine. It is the operating manual for the `diagram-architect` agent and is directly invocable (`/diagram <target>`).

## Why this skill exists

A capable model can free-hand a Lucid call per diagram — and the result drifts: inconsistent colors, eyeballed conformance, duplicate docs on every re-run, no way to tell a stale render from a current one. This skill's job is the **reproducible, composable method** that removes those decisions: the spec carries the diagram's meaning, the **always-first house-style profile** binds it to the Design-14 grammar **by version** (`style: house@14.1.0`), and the engine's hermetic stages turn a spec into a byte-identical IR every run. The failure modes it prevents: treating Lucid as a source of truth (it is import-only — "results may vary over time"); restating the grammar instead of resolving it from the pinned version; rendering before validating; and re-rendering into a *new* doc because identity wasn't tracked.

The grammar itself — the role tokens, colors (`#RRGGBB`), shapes, legend rules, header, terminal-artifact rule, the assignment rule — lives in **Design 14 (`Grammar-version 14.1.0`)** and the always-first profile. The kits **cite** it; they never copy it.

## Guardrails

Refusal conditions — they hold under a "just render it in Lucid" urge:

1. **Profile- and kit-first.** Load `references/diagram-house-profile.md` (the always-first overlay — it pins `house@14.1.0` and **resolves** the Design-14 grammar; it does not restate it) **and** the `references/kit-<token>.md` for the diagram's `token` before authoring or rendering. The committed spec — never the Lucid doc — is the source of truth.
2. **Spec is the source of truth; the render is a regenerable output.** One-way: spec -> Standard Import -> Lucid. Lucid edits are not a source and are lost on re-render. Socialize "edit the spec, never the Lucid doc."
3. **Validate before render.** The two-tier conformance gate (JSON-Schema + the full house rule set resolved from the grammar via the profile) runs in stage 2, **before** any side-effecting adapter call. A spec that fails the gate does not render.
4. **Stages 1-3 are pure; all side effects live in the adapter.** No MCP, clock, or RNG in parse+resolve / validate / lower. Layout is a pure function of (template + node count/order + grammar constants), stable-ordered by id. The canonical IR is reproducible (sorted-key JSON, no timestamps/RNG).
5. **Render identity is the committed manifest + a sentinel-shape id — one-way doors flagged.** Idempotency is spec->doc via the git-tracked, versioned manifest; the `specId` correlation token lives in a **sentinel shape id** `__specid__<specId>` (build-1-probe-verified: shape ids round-trip through `fetch`; `customData` does NOT). A breaking `Grammar-version` bump obligates a re-render + a reviewed golden-IR re-baseline (a one-way door) — flag it, don't assert it. An ambiguous/orphaned manifest-repair match is **halt-and-confirm**, never auto-bound.
6. **Don't restate the grammar; don't author the standard.** Colors/shapes/legend/header/terminal-artifact/assignment-rule are Design 14's — cite them via the profile. Defining or changing the grammar is Design 14's surface, not this skill's.

## The method

`references/method.md` holds the full 4-stage engine; the spine:

1. **Load the profile + the kit** for the diagram's `token`. The profile pins `house@14.1.0` and resolves the grammar; the kit gives the token's slot schema, slot allow-list, fill-recipe, and worked example.
2. **Author/read the two-layer spec.** A notation-neutral **semantic model** (`token`, `nodes`/`slots` role-tagged, `edges`, `drilldown` pointers) separated from a thin **house-style binding** (`style: house@<Grammar-version>` + per-instance specifics only). Both version pins present: `schemaVersion` and `style: house@…`.
3. **Run the 4-stage engine** (`method.md`): parse+resolve (drilldown DAG, visited-set cycle check, fixed depth bound 3, grammar-homogeneity) -> validate (two-tier gate) -> lower-to-IR (canonical sorted-key JSON, two field-groups, deterministic layout) -> adapt (LucidAdapter: sentinel-id identity, manifest reconcile, create-then-move).
4. **Reconcile + commit.** Render via the manifest (reconcile in place if `specId` present; else create, write the sentinel, commit the manifest record **before the render is done**, move to folder `444905424`). A composite re-render is all-or-nothing at the render layer.

## The five dimensions (the scorecard)

Every spec and render is judged against five checkable dimensions, profile-first. Each maps to an engine stage and a closed contract.

1. **Spec well-formedness.** Two-layer (semantic model vs `style: house@…` binding); both pins present (`schemaVersion` + `Grammar-version`); `token` ∈ the 7-value enum; no per-node color/shape literals (the standard is resolved, not inlined). *Stage:* validate (a). *Basis:* Design 15 §The spec; profile.
2. **Composition integrity.** `drilldown: {ref, rel}` by id; the graph is a DAG (visited-set, not the no-back-pointer convention); fixed depth bound of 3 (root = depth 1, fires at depth > 3); child `token` ∈ the slot allow-list (MVP: `linear-pipeline.stage -> {circular-cohort}`); all drilldown-reachable instances pin one `Grammar-version`. The five error codes are a closed kebab-case enum: `unresolved-ref`, `token-not-allowed`, `cycle-detected`, `depth-exceeded`, `grammar-version-mismatch`. *Stage:* parse+resolve. *Basis:* Design 15 §resolver.
3. **House conformance.** The full house rule set resolved from the Design-14 grammar at the pinned version via the profile (one token per role; legend lists exactly the roles present; plain human title; paper work-artifact; ASCII labels; …) — the profile is authoritative, this list is illustrative. *Stage:* validate (b). *Basis:* Design 14; profile.
4. **IR reproducibility & determinism.** The canonical IR is sorted-key JSON with no timestamps/RNG, separating semantic/structural fields from house-style/layout fields; layout is a pure function of (template + node count/order + grammar constants), stable-ordered by id; stages 1-3 are byte-identical across runs with no external calls. *Stage:* lower-to-IR. *Basis:* Design 15 §engine.
5. **Render identity & staleness.** Idempotency is spec->doc via the versioned manifest (`manifestVersion`; records `{specId, lucidDocId, renderedGrammarVersion, renderedSchemaVersion, specHash}`; `specId` IS the spec id); the `specId` token lives in the sentinel shape id `__specid__<specId>`; a record is stale **iff** `renderedGrammarVersion` != current Grammar-version **or** `specHash` != hash(committed spec). LucidAdapter constraints: Standard-Import JSON, ASCII-only labels, `#RRGGBB`, probe-confirmed primitives only, create-then-move into folder `444905424`, commit the manifest record before the render is "done". *Stage:* adapt. *Basis:* Design 15 §Render identity.

## Kit index

One `kit-<token>.md` per the 7 `token` values (the discriminator; the 6 Design-14 archetypes, with layered-cake split into `-kit` and `-signal`). Each kit gives the token's slot schema + slot allow-list + fill-recipe + worked example, and **cites** the grammar — it does not restate colors/shapes.

| Token | Kit |
|---|---|
| `layered-cake-kit` — knowledge bands -> skill node -> expert node (thin instance = 1 band) | `references/kit-layered-cake-kit.md` |
| `layered-cake-signal` — source nodes -> MCP-tool decipher band -> reasoning band -> expert/cohort | `references/kit-layered-cake-signal.md` |
| `circular-cohort` — central work-artifact ringed by expert nodes, one `dissenter`, iterate-to-converge | `references/kit-circular-cohort.md` |
| `linear-pipeline` — ordered left->right stages/gates; the composition root (`.stage -> {circular-cohort}`) | `references/kit-linear-pipeline.md` |
| `hub-and-spoke` — a central identity (bet / graph) with fan-in spokes | `references/kit-hub-and-spoke.md` |
| `cross-cutting` — a lifecycle base with one overlay band touching >=2 stages | `references/kit-cross-cutting.md` |
| `meta-skill` — a skill-artifact target with a process loop acting on it | `references/kit-meta-skill.md` |

New token templates start from `references/kit-TEMPLATE.md` (the per-token shape). Kits cite Design 14; they never duplicate the grammar.

## How the diagram-architect agent hooks in

The `diagram-architect` persona's first step loads `diagram-house-profile.md` (pinning `house@14.1.0`) + the `kit-<token>.md` for the work, then authors/validates the spec and runs the engine. The agent owns the spec -> IR -> render method and the manifest reconcile (including the manual repair procedure); Design 14 owns the grammar it renders to. The MVP realizes stages 1-4 as the documented method the agent executes via the Lucid MCP, with the spec JSON-Schema, the canonical-IR format, and the manifest as concrete committed artifacts (the hermetic stages 1-3 are designed to harden into a code CLI later).

## Halt conditions

- **No target** to generate/validate/regenerate — ask for the spec (or the skill to diagram); never render from a generic mental model or eyeball conformance.
- **A spec fails the two-tier gate** (schema or a resolved house rule) — report the failing rule / error code; do not render.
- **A composition violation** — surface the exact closed-enum code (`unresolved-ref`, `token-not-allowed`, `cycle-detected`, `depth-exceeded`, `grammar-version-mismatch`); do not render.
- **A one-way door** — a breaking `Grammar-version` bump (obligates re-render + reviewed golden-IR re-baseline) — flag for human approval; don't assert it as the fix.
- **An ambiguous/orphaned manifest-repair match** — halt, report the candidate set, require human confirmation at the cohort gate; never auto-bind.
- **The build-1 sentinel-id exit criterion is unmet** — if `specId` does not round-trip byte-exact through `fetch`, fall back to a verified-durable field or revisit ordering before committing the strand-and-repair path.

## What this skill defers

Per Design 15 (each with an un-defer trigger): **UML / mermaid / other render targets** (the IR seam is designed for them; only the Lucid adapter is built — un-defer: a named target requested); **the capability-declaring fidelity-tier adapter matrix** (un-defer: a second adapter exists); **a general any-slot -> any-class composition resolver** (MVP hardcodes `linear-pipeline.stage -> circular-cohort` — un-defer: a second typed pairing requested); **portable Mermaid structure-only emit** (un-defer: PR-reviewed specs need an inline preview); **the code CLI + CI auto-sync** including the hardened `--reconcile` sweep (MVP is agent-executed via MCP; sync is manual — un-defer: specs observably drift from renders, or PR-process automation is greenlit). The house-style profile is a *pin* to Design 14's grammar — when the grammar bumps, the pin moves and affected diagrams re-render.
