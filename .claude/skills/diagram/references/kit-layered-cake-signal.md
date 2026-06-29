# layered-cake-signal kit

`token: layered-cake-signal` — one of the 7 spec discriminator values. The conceptual archetype is **Layered-cake** (Design 14 §"The archetype set", #1), **signal variant**: a vertical stack that reads **bottom→top** = *named data sources* -> an **MCP-tool "decipher" band** (the standard tools that read/normalize the sources) -> a **reasoning/decision band** -> an **agentic expert or cohort** at the top, terminating in the **work-artifact** that *is* the skill's verdict/decision. (Recall: layered-cake is the one archetype that splits into two tokens — `layered-cake-kit` and `layered-cake-signal`; this is the signal half.) This kit is the **template** for that token: the slot schema it fills, the slot allow-list it declares for composition, the recipe to populate it from a target, and a worked spec fragment.

It **cites** the grammar and the house-profile; it does not restate them. The role->shape/color tokens (data-source = cylinder/blue; MCP-tool band = diamond/grey; reasoning = amber; expert = hexagon/purple; work-artifact = paper `#ECF0F1`/`#2C3E50`), the bottom->top `feeds-into` arrow semantics, the header/legend/terminal-artifact rules, and ASCII/`#RRGGBB`/`opacity` realizations live in Design 14 §"The shared visual grammar" / §"Probe outcome" and are resolved at the pinned `Grammar-version` through `references/diagram-house-profile.md`. The conformance floor for this token is Design 14 §Conformance: **>=2 source nodes -> MCP-tool band -> reasoning/decision band -> expert-or-cohort node**. (Design 14 §Terminal artifact additionally mandates that signal — like linear-pipeline and meta-skill — ends in an explicit work-artifact node, the decision/verdict, arrowed from the producing step.)

## 1. Slot schema (the token's fillable shape)

A `layered-cake-signal` instance fills these semantic-model slots. Field names and the two-layer split (semantic model vs `style: house@<Grammar-version>` binding) are normative per Design 15; roles are grammar tokens, not literals. This token carries **both** `bands[]` (the stacked decipher/reasoning layers) **and** `nodes[]` (the source nodes that sit in the bottom band, plus the expert and the terminal artifact above the stack); a node's `band` field places it inside a band.

- **`token`**: `layered-cake-signal` (the discriminator).
- **`title`**: the skill's plain human title (Design 14 header rule — never the archetype taxonomy). Set per-instance; the centered header + subtitle + top-left legend are rendered by the profile, not authored here.
- **`bands[]`** — the stacked layers, each `{ id, role, label, order }`, ordered **bottom->top** (`order` 1..N, the lowering input for stack geometry):
  - **`role: source`** (bottom band, `order: 1`) — the info-source band (grammar: blue) that holds the source nodes. Carries the `>=2` source nodes via their `band` back-reference.
  - **`role: mcp-tool`** — the **"decipher" band** (grammar: diamond/grey): the standard MCP tools that read/normalize the sources. Required by the floor.
  - **`role: reasoning`** — the **reasoning/decision band** (grammar: amber) where the deciphered signal is weighed. Required by the floor.
  - `order` is a total order over the stack; band geometry in the IR is a **pure function of (template + band order + grammar constants)** stable-sorted by `id` (Design 15 stage-3 layout determinism) — authors set `order`, never coordinates.
- **`nodes[]`**:
  - **`role: source`** — the named data sources (`>=2` required by the floor), each `{ id, role: source, label, band: <source-band-id> }`, sitting **in** the bottom source band. Rendered as the cylinder shape per the grammar.
  - **`role: expert`** (or a cohort) — the **agentic expert or cohort** at the top of the stack that owns the decision (exactly one terminus role). Rendered as the hexagon/purple expert shape.
  - **`role: artifact`** — the **terminal work-artifact** the skill produces: the decision/verdict/report itself (grammar paper fill `#ECF0F1` / `#2C3E50` text). Present because signal is a terminal-output token (Design 14 §Terminal artifact); arrowed `feeds-into` from the expert.
- **`edges[]`** — `{ from, to, kind: feeds-into }`, authored along the bottom->top flow: source nodes -> mcp-tool band -> reasoning band -> expert -> artifact. The `feeds-into` arrow token is grammar-fixed and reads upward for layered-cake (Design 14 §Lifecycle direction); unlike `circular-cohort`, this token **does** author its edges (they are not archetype-derived).
- **`drilldown`** — **none for this token in the MVP** (see §2).
- **`style: house@<Grammar-version>`** + **`schemaVersion`** — the two independent version pins every spec carries (Design 15). A composite's full drilldown-reachable set pins one `Grammar-version` (grammar-homogeneous).

The conformance gate (Design 15 stage-2, resolved via the house-profile) rejects an instance with `<2` source nodes, missing the mcp-tool or reasoning band, lacking the terminal artifact (the verdict), or carrying a per-node color/shape literal (style is resolved from the grammar, never inlined).

## 2. Slot allow-list (composition contract)

This token declares **no outbound drilldown slot in the MVP**:

```
layered-cake-signal.<role> -> { }   # no allowed child token (MVP)
```

- The only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }` (see `kit-linear-pipeline.md` §2); the **general any-slot->any-token resolver is deferred** (Design 15 Non-goals: "A general any-slot->any-class composition resolver" — MVP builds the one typed pairing only). `layered-cake-signal` therefore exposes **no composable slot-role** and is neither a drilldown source nor (in the MVP) a drilldown target.
- Because every slot-role's allowed-child set here is empty, **any** `drilldown` authored on a `layered-cake-signal` node fails the resolver with **`token-not-allowed`** (the allow-list is keyed per slot-role; an empty set permits nothing).
- Resolver invariants the engine enforces around this (Design 15 stage-1, not by this file): a `drilldown.ref` must resolve (`unresolved-ref`); the child `token` must be in the slot's set (`token-not-allowed`); the drilldown graph is a DAG checked by a **visited-set** (`cycle-detected` — not the no-back-pointer convention); depth is bounded at **3** with the root at depth 1, so a chain firing at depth >= 4 is **`depth-exceeded`**; every reachable instance shares one `Grammar-version` (`grammar-version-mismatch`). The five codes are the closed kebab-case enum — do not invent or rename.
- Widening this is deferred (Design 15 Non-goals/Deferred): adding an allowed child token is an additive edit *here*, in this allow-list, plus a new fixture — only when a second typed pairing is requested (Design 15 Deferred un-defer trigger).

## 3. Fill-recipe (populate the slot from a target)

Given a signal/decision skill the assignment rule routed to `layered-cake-signal` (Design 14 rule predicate #4 — "reading pre-existing external signals/data it did not generate, through tools, to reach an operational decision, and the skill's terminal output *is* that decision/verdict"):

1. **Enumerate the named data sources** as `role: source` nodes (`>=2`) — the concrete signals the skill reads (Prometheus/Grafana metrics, on-chain state, S3 reports, logs, ...). Author a single `role: source` band (`order: 1`) and set each source node's `band` to it. Label each with its plain source name (ASCII-only — grammar authoring constraint).
2. **Add the MCP-tool "decipher" band** — one `role: mcp-tool` band (next `order`) naming the standard tools that read/normalize those sources. Required by the floor.
3. **Add the reasoning/decision band** — one `role: reasoning` band (next `order`) where the deciphered signal is weighed into a verdict. Required by the floor.
4. **Add the expert-or-cohort terminus** — one `role: expert` node (or a cohort) at the top that owns the decision.
5. **Add the terminal artifact** — append one `role: artifact` node (the decision/verdict/report), arrowed `feeds-into` from the expert. Signal is a terminal-output token (Design 14 §Terminal artifact); this node is mandatory, not optional.
6. **Wire the bottom->top edges** — `feeds-into` from the source nodes up through the mcp-tool band, the reasoning band, the expert, to the artifact. (This token authors its edges; they are not archetype-derived.)
7. **No outbound drilldown** — `layered-cake-signal` is an MVP leaf; do not author a `drilldown` on it (it would be `token-not-allowed`).
8. **Stamp the pins** — `schemaVersion`, `style: house@<current Grammar-version>`.
9. **Validate before render** — the two-tier gate (JSON-Schema + house rule set) must pass; only then does the LucidAdapter emit. The legend lists exactly the roles present (source, mcp-tool, reasoning, expert, work-artifact), per the profile.

## 4. Worked example (spec fragment)

`/root-cause` — the disciplined data-driven investigation skill: it reads pre-existing signals (it did not generate) through tools and its terminal output *is* the verdict (the root cause), so the assignment rule routes it to `layered-cake-signal`. Sources sit in the bottom band; the decipher band reads them; the reasoning band weighs hypotheses; the multi-expert cohort owns the call; the verdict is the terminal artifact.

```yaml
- id: root-cause-signal
  schemaVersion: 1
  style: house@14.1.0
  token: layered-cake-signal
  title: "Root Cause Analysis"
  bands:
    - { id: b-src,    role: source,    order: 1, label: "Retrieved signals" }
    - { id: b-mcp,    role: mcp-tool,  order: 2, label: "Decipher: query the signals" }
    - { id: b-reason, role: reasoning, order: 3, label: "Hypotheses, falsified to a cause" }
  nodes:
    - { id: metrics, role: source,   band: b-src, label: "Grafana metrics" }
    - { id: logs,    role: source,   band: b-src, label: "Pod logs" }
    - { id: chain,   role: source,   band: b-src, label: "On-chain state" }
    - { id: cohort,  role: expert,   label: "Specialist cohort" }
    - { id: verdict, role: artifact, label: "Root-cause verdict" }
  edges:
    - { from: metrics, to: b-mcp,    kind: feeds-into }
    - { from: logs,    to: b-mcp,    kind: feeds-into }
    - { from: chain,   to: b-mcp,    kind: feeds-into }
    - { from: b-mcp,   to: b-reason, kind: feeds-into }
    - { from: b-reason, to: cohort,  kind: feeds-into }
    - { from: cohort,  to: verdict,  kind: feeds-into }
```

Why it conforms: 3 source nodes in the bottom `source` band (floor is `>=2`) -> an `mcp-tool` "decipher" band -> a `reasoning/decision` band -> an `expert` cohort terminus, exactly the signal floor; a terminal `artifact` node (the verdict) arrowed `feeds-into` from the expert, per the signal terminal-output rule; bands ordered bottom->top, edges authored upward; no outbound drilldown (an MVP leaf — authoring one would be `token-not-allowed`); pins `house@14.1.0`; no authored `legend` (profile-derived) and no per-node color/shape literal — style resolves from the grammar via the profile.

## 5. Authoring notes

- **Cite, don't restate.** Shapes, colors, arrow kinds, header/legend/terminal-artifact rules -> Design 14 via `diagram-house-profile.md` at the pinned version. This file owns only the *slot schema + allow-list + recipe + example* for the token.
- **No coordinates, no style literals here.** Stack layout is the IR's pure-function lowering (Design 15 stage 3); style is profile-resolved. Authoring a fill = semantic model only.
- **The terminal artifact is mandatory for signal.** Unlike `layered-cake-kit` (which ends in the expert), the signal variant ends in the **work-artifact** that is the decision/verdict (Design 14 §Terminal artifact: linear-pipeline, meta-skill, and signal end in an explicit artifact). Omitting it fails the gate.
- **MVP leaf — empty allow-list.** The only MVP typed pairing is `linear-pipeline.stage -> { circular-cohort }`; the general resolver is deferred (Design 15 Non-goals), so this token's allow-list is empty. It stays empty until a second typed pairing is requested (Design 15 Deferred).
- **One-way door:** a `Grammar-version` bump re-renders this instance as part of its composite's all-or-nothing re-render and forces a reviewed golden-IR re-baseline (Design 15 §Versioning) — flag it, don't silently re-pin.
