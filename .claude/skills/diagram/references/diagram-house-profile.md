# Diagram house profile (always-first overlay)

Loaded before any spec authoring, validation, or render. It **pins the house style** (`style: house@14.1.0`) and **resolves the Design-14 visual grammar by reference** — it does **not** copy the grammar's tables. This profile is the **authoritative source for conformance rule-set (b)**: the full house-standard semantic rule set the two-tier gate runs *before* render (tier (a) is JSON-Schema validity against `schema/diagram-spec.schema.json`; tier (b) is this profile's rules, resolved from the pinned grammar). The kits (`kit-<token>.md`) cite this profile and the grammar; they do not restate the grammar either.

**Authority order:** Design 14 at the pinned `Grammar-version` is the standard. This profile *resolves* it for the gate and adds the Lucid-render constraints. If this profile and the live Design-14 grammar disagree, the grammar at the pinned version wins — flag the drift, do not follow a stale copy.

## The pin (stated once, used everywhere)

- **`style: house@14.1.0`** — every spec carries this; the standard is Design 14 at `Grammar-version 14.1.0`: `designs/sei-agentic-mesh/14-skill-diagram-visual-grammar.md`.
- A spec missing `style` (or `schemaVersion`) **fails validation**. Both pins are mandatory.
- A composite is **grammar-homogeneous**: every drilldown-reachable instance pins the **same** `Grammar-version`. A mixed-version composite fails with `grammar-version-mismatch`.
- Staleness is a committed, greppable fact off the manifest: a record is stale **iff** `renderedGrammarVersion` != current `Grammar-version` **or** `specHash` != hash(the current committed spec). No Lucid round-trip.

## Two-tier conformance gate (this profile owns tier (b))

Run **before** any render. A spec that fails either tier **does not render**.

- **Tier (a) — JSON-Schema validity.** Validate against `schema/diagram-spec.schema.json` (`schemaVersion` supported; `token` in the 7-value enum; the per-`token` `oneOf` slot floor; ASCII text; `#RRGGBB` legend colors). Mechanical.
- **Tier (b) — the full house-standard semantic rule set, resolved from the Design-14 grammar at the pinned version.** This profile is authoritative for tier (b). The rules below are the resolution; they cite the grammar rather than restate it.

### Tier-(b) rules (resolved from Design 14 §"The shared visual grammar" + §Conformance)

1. **One token per role.** Every `nodes[].role`, `bands[].role`, and `legend[].role` is a role token defined in the Design-14 role tables; every role maps to **exactly one shape** and **exactly one color** there. A role not in the grammar tables fails. *(Resolves: Design 14 role->shape table + role->color table.)*
2. **Legend lists exactly the roles present.** Non-empty; one row per role that actually appears in this diagram's nodes/bands — no more, no fewer. Each `legend[].color` is the grammar's color for that role. *(Resolves: Design 14 §Legend.)*
3. **Plain title.** `title` is the plain human Title-Case name (kit skills: "<Domain> Knowledge Kit"); **never** the internal archetype taxonomy. Optional `subtitle` is the one-line human descriptor. *(Resolves: Design 14 §Header.)*
4. **Header is centered, legend is compact top-left.** The title heads the flow (top-center); the legend is a compact top-left swatch+label stack kept visually separate from the title. *(Resolves: Design 14 §Header + §Legend — layout obligations the LucidAdapter honors at lowering/emit.)*
5. **Paper work-artifact.** A work-artifact node (central in `circular-cohort`; terminal where a skill produces a durable output) uses the neutral paper fill `#ECF0F1` with `#2C3E50` text everywhere — never a role color. *(Resolves: Design 14 §Terminal artifact.)*
6. **Terminal-output rule.** `linear-pipeline`, `meta-skill`, and `layered-cake-signal` end in an explicit work-artifact node arrowed from the producing step (inputs -> process -> artifact). `layered-cake-kit`/`cross-cutting` end in the **expert**; `hub-and-spoke` in the **central identity**; `circular-cohort` in the **central work-artifact under review** — these add no extra terminal node. *(Resolves: Design 14 §Terminal artifact.)*
7. **Per-token required slot (the conformance floor).** The token's required slot from Design 14 §Conformance is present: `layered-cake-kit` >=1 knowledge band -> skill -> expert; `layered-cake-signal` >=2 source nodes -> MCP-tool band -> reasoning/decision band -> expert-or-cohort; `cross-cutting` >=2-stage base + exactly 1 overlay band touching >=2; `circular-cohort` central work-artifact + >=3 ring nodes incl. one `role: dissenter`; `linear-pipeline` >=2 ordered stages left->right, gates bold-bordered; `hub-and-spoke` 1 central identity + >=2 spokes; `meta-skill` 1 skill-artifact target + a >=2-stage loop acting on it. *(Resolves: Design 14 §Conformance per-token slot table.)*
8. **Arrow semantics.** Every `edges[].kind` is in the grammar arrow vocabulary (`feeds-into`, `applies-across`, `iterates`, `fans-in`, `acts-on`, `gate-blocks`) and reads in the grammar's fixed direction (layered-cake bottom->top; pipeline/lifecycle left->right; circular-cohort clockwise). *(Resolves: Design 14 §Arrow semantics + §Lifecycle direction.)*

## LucidAdapter constraints (the render boundary)

These are the verified, probe-confirmed constraints the stage-4 adapter applies. They are part of tier-(b) authority where they constrain the spec (ASCII, `#RRGGBB`); the rest govern emit.

- **Standard-Import JSON only.** Read `lucid://diagram-specification` before each `lucid_create_diagram_from_specification` call; emit the house schema mapped to Standard Import (role -> `type` shape + `style.fill.color` + `text`; band -> background rectangle earlier in the array; overlay -> rectangle `opacity` 30-40 placed last, label at the band edge; gate -> rectangle `stroke.width: 5`).
- **ASCII-only labels and titles.** No non-ASCII (em-dash rendered as `?`), no emoji. The schema enforces this on `title`/`subtitle`/`label`; the adapter sanitizes.
- **Colors are `#RRGGBB`.** The grammar palette is the source (data-source `#4A90D9`, skill `#1ABC9C`, expert `#8E44AD`, MCP-tool `#95A5A6`, reasoning `#F39C12`, gate `#E74C3C`, overlay `#F1C40F`@~35%, identity `#E67E22`, work-artifact `#ECF0F1`/text `#2C3E50`) — cited from Design 14, not redefined here.
- **Probe-confirmed primitives only.** cylinder=`database`, expert=`hexagon`, MCP-tool=`diamond`, identity=`circle`, artifact=`document`, skill=`rectangle`(rounded), gate=`rectangle`+`stroke.width:5`; ring=`lineType:curved`; translucent overlay=`opacity`+z-order. No unconfirmed primitive.
- **Create-then-move + manifest-before-done.** `lucid_create_diagram_from_specification` takes no folder arg: create in root, write the `specId` correlation token as a **sentinel shape whose author-assigned `id` is `__specid__<specId>`** (build-1-probe-verified: shape ids round-trip via `fetch`; `customData` does NOT and is rejected), **commit the manifest record before the render is considered done**, then move the doc into folder `444905424`. Ordering makes a strand the only failure mode, never a double-create.
- **Repair matches the sentinel token, never the title.** A lost manifest record is repaired by scanning folder `444905424` and matching the sentinel `__specid__<specId>` exactly; ambiguous/orphaned matches **halt and require human confirmation** — never auto-bind.

## The five resolver error codes (closed kebab-case enum)

Stage-1/2 reject with exactly these literals (distinct from the `Grammar-version` field name): `unresolved-ref` (a `drilldown.ref` resolves to no instance id), `token-not-allowed` (child `token` not in the slot allow-list declared per slot-role in `kit-<token>.md`; MVP: `linear-pipeline.stage -> { circular-cohort }`), `cycle-detected` (visited-set check on the drilldown DAG — *not* the no-back-pointer convention), `depth-exceeded` (drilldown depth > 3; root is depth 1, so this fires at depth >= 4), `grammar-version-mismatch` (a composite with mixed `Grammar-version` pins).
