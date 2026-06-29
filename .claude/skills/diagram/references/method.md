# The method — the 4-stage diagram generation engine

Two modes, one spine: **author/generate** (the diagram-architect turning a spec into a render) and **validate/review** (a gate over an existing spec or a staleness check over a render). Both load `diagram-house-profile.md` (always first — it pins `house@14.1.0` and **resolves** the Design-14 grammar; it does not restate it) + the `kit-<token>.md` for the diagram's `token`, then run the engine. The committed spec is the source of truth; the Lucid render is a regenerable downstream output (Design 15 §Goals).

The engine is a **4-stage pipeline** with one hard seam: stages 1-3 are **pure/deterministic** (no MCP, clock, RNG) and all side effects are confined to stage 4 (the adapter). Stages 1-3 emit a canonical IR artifact; only stage 4 touches Lucid.

```
spec --(1 parse+resolve)--> resolved --(2 validate)--> validated --(3 lower)--> IR --(4 adapt)--> Lucid (side effects)
       stages 1-3 PURE/DETERMINISTIC (no MCP/clock/RNG)              | stage 4 = the only side-effecting stage
```

## Stage 1 — parse+resolve (pure)

Load the spec(s) and resolve composition into a validated DAG. A composite is a *set* of separately-addressable instances, each with a stable `id` + `token`; a parent slot MAY carry `drilldown: { ref: <child-id>, rel: <relationship> }`, **by id**; the child holds no back-pointer.

Steps, in order:

1. **Parse** each instance: `id`, `schemaVersion`, `style: house@<Grammar-version>`, `token`, `title`, the semantic model (`nodes`/`slots`, `edges`, `drilldown`).
2. **Resolve `drilldown` refs into a DAG.** Each `drilldown.ref` must name a present instance `id`. An unresolved ref -> `unresolved-ref`.
3. **Slot allow-list check.** A child's `token` must be in the parent slot-role's allow-list, declared per slot-role in the parent's `kit-<token>.md`. MVP value: `linear-pipeline.stage -> { circular-cohort }`. A child token outside the list -> `token-not-allowed`.
4. **Cycle check — by visited-set, not by the no-back-pointer convention.** Walk the drilldown graph carrying a visited-set of instance ids on the current path; re-entering an id on the path -> `cycle-detected`. (The "child holds no back-pointer" convention is a spec discipline, **not** the cycle guard — the visited-set is the guard.)
5. **Depth bound — fixed at 3.** The root instance is depth 1, so the bound fires at depth > 3 (i.e. strictly greater than 3, the first violating instance being depth 4) -> `depth-exceeded`.
6. **Grammar-homogeneity.** Every instance reachable via drilldown from a root must pin the **same** `Grammar-version` (a composite is grammar-homogeneous). A mixed-version composite -> `grammar-version-mismatch`.

The five error codes are a **closed kebab-case enum** — `unresolved-ref`, `token-not-allowed`, `cycle-detected`, `depth-exceeded`, `grammar-version-mismatch` (error-code literals, distinct from the `Grammar-version` field name). Each negative case fails with exactly its code.

## Stage 2 — validate (pure): the two-tier conformance gate

Runs **before** any render. A spec that fails either tier does not reach stage 4.

- **Tier (a) — JSON-Schema validity.** Validate against the versioned spec JSON-Schema (`schemaVersion`), discriminated over the 7 `token` values. This enforces serialization shape: required fields present; both version pins present (a spec missing `schemaVersion` **or** `style: house@…` fails); `token` ∈ the 7-value enum (an out-of-enum `token` is rejected); the two-layer separation (a distinct semantic-model section vs the `style` binding) with **no per-node color/shape literals** — the standard is resolved from the pinned grammar, never inlined per node.
- **Tier (b) — the full house rule set, resolved from the Design-14 grammar at the pinned version via `diagram-house-profile.md`.** The profile is the authority; do **not** restate the grammar here. Illustrative (not exhaustive — the profile governs): one token per role; the legend lists exactly the roles present in this diagram; a plain human title (never the archetype taxonomy); the work-artifact rendered as the paper node where the token requires a terminal artifact; ASCII-only labels; the token's required conformance slot is satisfied (e.g. `circular-cohort` -> a central work-artifact + >=3 ring nodes incl. one `role: dissenter`; `linear-pipeline` -> >=2 ordered left->right stages; resolved per-token from the grammar via the profile).

A spec violating any pinned house rule fails the gate — reported as the failing rule, not rendered.

## Stage 3 — lower to IR (pure): the canonical, reproducible artifact

Lower the validated spec into a **canonical IR** (the RenderModel), git-committed beside the spec. Properties (the testable adapter seam + the reproducibility artifact):

- **Canonical JSON.** Sorted keys, no timestamps, no RNG — byte-identical across runs for the same (spec, grammar).
- **Two field-groups, distinct.** Semantic/structural fields (the resolved nodes/slots/edges/drilldown topology, role tags, ordering) separated from house-style/layout fields (resolved colors/shapes/legend/positions). This separation **is** the adapter seam: a future adapter (UML, mermaid) consumes the same IR; the house-style group is what the LucidAdapter binds.
- **Deterministic layout.** Layout is a **pure function** of (template + node count/order + grammar constants), per a named deterministic layout rule per token, **stable-ordered by node id**. No clock, no RNG, no MCP. Re-running stages 1-3 on an unchanged spec at a pinned grammar version yields the same IR (the reproducibility guarantee).

The golden-IR eval is keyed by **(specHash + Grammar-version)**: it diffs regenerated-IR against the committed golden; a grammar bump forces a reviewed re-baseline (IR = f(spec, grammar), so a grammar change legitimately changes the IR).

## Stage 4 — adapt (emit): the LucidAdapter, the only side-effecting stage

Render the IR via the Lucid MCP. Identity is **spec->doc**, authored by a git-tracked, **versioned manifest** (`manifestVersion`) — not by Lucid's create call (which has no upsert and no folder arg). The reconciler **rejects (fail-closed) a record whose `manifestVersion` it does not support**.

### The manifest (the idempotency authority + the staleness oracle)

A list of records `{ specId, lucidDocId, renderedGrammarVersion, renderedSchemaVersion, specHash }`, committed to git. `specId` **is** the spec's own `id`. `specHash` is a hash of the **spec source bytes as committed** (computable without re-running stages 1-3). A record is stale **iff** `renderedGrammarVersion` != current Grammar-version **or** `specHash` != hash(the current committed spec) — a committed, greppable check needing no Lucid round-trip. (`renderedSchemaVersion` is recorded but is not a third staleness predicate: a schema MAJOR migrates committed specs -> spec bytes change -> `specHash` fires.)

### Reconcile (steady state)

- **`specId` present in the manifest** -> update that `lucidDocId` in place: `lucid_update_document` (retitle/move), `lucid_edit_item`, `lucid_delete_items`. Same doc, never a new one.
- **`specId` absent** -> create with `lucid_create_diagram_from_specification`, then, in this order:
  1. **Write the `specId` correlation token into the created doc as a sentinel shape** whose author-assigned `id` encodes the `specId`: `id = "__specid__<specId>"`. Build-1-probe-verified: author-assigned shape `id`s round-trip through `fetch`; `customData` does **NOT** and was rejected. (Build-1 exit criterion: if `specId` does not survive create->fetch byte-exact, fall back to a verified-durable field or revisit this ordering before committing the strand-and-repair path.)
  2. **Commit the manifest record** (`{specId, lucidDocId, …}`) **before the render is considered done.**
  3. **Move the doc to folder `444905424`** (`lucid_create_diagram_from_specification` takes no folder arg, so the created doc lands in root and must be moved as a second step).

This ordering (create -> token -> manifest -> move) makes a **strand** the only failure mode (a created doc whose manifest write didn't land), never a double-create.

### Adapter constraints (probe-confirmed; honor verbatim)

- Emit **Standard-Import JSON** (read `lucid://diagram-specification` before each call).
- **ASCII-only labels and titles** (a non-ASCII char rendered as `?`; emoji banned).
- Colors as **`#RRGGBB`** (the grammar's pinned palette, resolved from the profile — not chosen here).
- **Probe-confirmed primitives only** (the role shapes, curved ring arrows, the translucent overlay band the Design-14 probe confirmed).
- **Create-then-move**: every created doc ends in folder `444905424`.

### Composite re-render — all-or-nothing at the render layer

Stage all N doc reconciles, then **commit all N manifest records last**, so a mid-sequence failure leaves the prior records intact and the new docs as `specId`-matched repairable strands — never a committed mixed-`renderedGrammarVersion` manifest state within one composite (the render-layer mirror of the stage-1 grammar-homogeneity invariant). The staleness oracle flags any composite whose member records carry mixed `renderedGrammarVersion` as inconsistent (requires re-render).

### Repair (degraded) — halt-and-confirm, never the title

If a manifest record was lost (create succeeded, manifest write failed), recover by matching the **`specId` correlation token in the doc — never the human title** (titles like "Workstream"/"Xreview" are not unique). This is a **manual procedure the diagram-architect runs**: scan folder `444905424`, read each doc's sentinel shape `id`, match the `specId` exactly, rewrite the manifest record. It is **report-and-confirm**: on an ambiguous or orphaned token match the architect MUST **halt, report the candidate set, and require human confirmation at the cohort gate** before rewriting the record — it MUST NOT proceed unconfirmed. (A create that failed *before* the token write — a strand with neither token nor record — lands in folder `444905424` and is a documented manual-cleanup edge.) The hardened `--reconcile` sweep is deferred code-CLI surface.

## Mode notes

- **Author/generate.** Run 1 -> 2 -> 3 -> 4. The MVP build-first target is the **pipeline -> cohort-cycle composite** (`linear-pipeline.stage -> circular-cohort`), regenerated from its spec to standard, with the golden-IR eval keyed by (specHash + Grammar-version).
- **Validate/review only.** Run 1 -> 2 and report; never render on a failed gate. For a render-staleness check, read the committed manifest and apply the oracle (no Lucid round-trip).
- **Realization.** The MVP realizes stages 1-4 as the documented method the diagram-architect executes via the Lucid MCP, with the spec JSON-Schema, the canonical-IR format, and the manifest as concrete committed artifacts. The hermetic stages 1-3 are designed to harden into a code CLI later (the deferred automation seam) without reworking the contracts.

## Citations

- Design 15 — the kit contract (engine stages, resolver, error codes, manifest, render identity, composition): `designs/sei-agentic-mesh/15-diagram-knowledge-kit.md`.
- Design 14 — `Grammar-version 14.1.0`, the house grammar this engine renders to (colors/shapes/legend/header/terminal-artifact/assignment rule) — resolved via `diagram-house-profile.md`, **cited, never restated**: `designs/sei-agentic-mesh/14-skill-diagram-visual-grammar.md`.
