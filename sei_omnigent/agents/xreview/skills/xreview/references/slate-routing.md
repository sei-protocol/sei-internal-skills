# Slate Routing — the shared change-type × blast-radius → slate rule

The canonical routing rule for **both** `/xreview` (review-phase slate) and `/coral`
(production-phase specialist set). One mechanism, not two: this table is the source of truth;
the citing skills apply it to their phase. If a skill's prose ever diverges from this table,
**the table wins**.

`route(change-type, blast-radius) → { slate, tier, auto-stewards, dissenter }`

## 1. Change-type taxonomy (six classes — the minimum that routes)

Classify the artifact under review into exactly **one** primary class. The class is the
primary axis; the blast-radius **tier** (§3) is the depth dial. The class→tier map is
single-sourced in §3 — this table does not restate tiers (a tier hint here would be a second,
drift-prone source of truth).

| Class | What it is |
|---|---|
| `doc-only` | a design/spec/prose artifact, no code |
| `mechanical` | rename, move, formatting, dependency bump — no behavior change |
| `component` | a change scoped to one component/package's own code |
| `cross-component` | a change spanning ≥2 components or ≥1 interface boundary |
| `shared-stack` | a change to infrastructure many components depend on (a shared library, a CI/CD path, a base image, a cluster-wide config) |
| `skill-package` | a change to a canonical `.claude/` skill or agent (the meta-stack) |

## 2. Spanning artifacts — classify by the highest-radius component present

When an artifact spans classes (a design doc *and* its implementing diff), classify by the
highest point on this **total order**:

```
doc-only  <  mechanical  <  component  <  cross-component  <  shared-stack  <  skill-package
```

- A `doc-only` design that ships a `shared-stack` diff routes as `shared-stack`.
- `skill-package` sits **strictly above** `shared-stack` in the order (the tie already resolves to
  `skill-package`, so the order states it as a strict `<` — no scan contradiction). A change
  touching both **resolves to `skill-package`**.
  `skill-package` is the strict-superset wiring — it adds the unconditional audit+author+prose
  pin (§4), so picking it never under-pins. A diff touching both a shared library *and* a
  `.claude/` skill is `skill-package`.

## 3. Blast-radius tiers (the depth dial — authoritative class→tier map)

Three tiers. The class sets the default tier; blast-radius can bump it **up** (never silently
down).

| Tier | Slate depth | Default for (authoritative class→tier map) |
|---|---|---|
| **T1 — light** | 1–2 lenses; single-reviewer pass allowed (labeled as such) | `mechanical`; `doc-only` **only if mechanical-equivalent** (typo/formatting/link — see §3a) |
| **T2 — domain** | the domain slate covering the boundaries (provider + consumer + any cross-cutting specialist — the mandatory-by-concern ones pinned in §4a) | `component`, `cross-component`, and any `doc-only` that proposes or alters a decision |
| **T3 — full + stewards** | full domain slate **plus** the auto-wired standards-stewards (§4) | `shared-stack`, `skill-package` |

**The floor rule:** a `shared-stack` or `skill-package` change is **T3 by default and cannot
drop below T2** — not by blast-radius and **not by operator override** (§5: an override may lower
the *default* tier but never pierces this floor). That floor is the dogfood lesson — the
canonical-stack change is the one that must not get a rubber-stamp slate.

### 3a. `doc-only` T1-vs-T2 split (a stated test, not a vibe)

A `doc-only` artifact is **T1 only if it is mechanical-equivalent** — a typo fix, a
formatting/whitespace change, a link/path correction, with **no change to any decision or
claim**. **Any doc-only change that proposes or alters a decision is T2** (a decision needs a
reviewer who can judge it). When in doubt, **T2**.

**The mechanical-equivalent carve-out is `doc-only` ONLY.** A `skill-package` change has **no
triviality exemption** — it routes by **file-type-present, not change-size**: a one-line typo in
a `.claude/` skill body is still `skill-package` (T3, floor T2, audit+author+prose pinned). "It's
just a typo in a skill" does **not** demote it to `mechanical`. The *classification* keys off
*what kind of file changed* (a `.claude/` skill body present → `skill-package`), not how many
lines; the audit+author+prose pin then follows from that class **unconditionally** — a skill is
prose + discipline + an authored artifact whether the diff is one line or a hundred. Dropping the
pin requires an **operator override with a stated reason, never a size judgment**.

## 4. Auto-wired standards-stewards

The stewards review *how the artifact is built*, orthogonal to its domain boundaries. The
wiring keys off **what kind of file changed**, evaluated independently of the domain slate (a
change can pull a steward without that steward owning a boundary):

| Steward | Auto-included when the change touches… | Axis it owns |
|---|---|---|
| `prose-steward` | any prose artifact — a design doc, a skill body, a README, a runbook | reads-native-for-its-audience (the `/lingua` doctrine) |
| `idiomatic-reviewer` | any code diff / implementation | reads-native to language/framework/package patterns |
| `audit-skill` | a `.claude/` **skill** body or its references | does the skill hold its discipline under pressure (RED) |
| `author-skill` | a `.claude/` **skill** body (RED/GREEN authoring) | is the skill authored correctly — triggers, anti-triggers, guardrails, evals |

**The load-bearing wiring rule (state it in the citing skill):**

> A **`skill-package` change auto-pulls `audit-skill` + `author-skill` + `prose-steward`** —
> always (T3 by default; the pin survives a tier override down to the T2 floor, see §5), not
> subject to silent omission. A skill *is* prose (→ prose-steward), it *is*
> a discipline that must survive pressure (→ audit-skill), and it *is* an authored artifact
> with triggers/guardrails/evals (→ author-skill). Dropping any of the three on a skill change
> requires an **operator override with a stated reason**.

### Steward dispatch — two kinds (agent-stewards vs skill-stewards)

The four stewards are **not all the same kind of artifact**, and the availability check keys off
the right registry per steward:

| Steward | Kind | Registry | How it's dispatched |
|---|---|---|---|
| `prose-steward` | **agent** (backed by `/lingua`) | `.claude/agents/` | dispatched as a subagent |
| `idiomatic-reviewer` | **agent** (backed by `/idiomatic`) | `.claude/agents/` | dispatched as a subagent |
| `audit-skill` | **skill** | `.claude/skills/` | a dispatched reviewer **loads the skill as its review rubric** |
| `author-skill` | **skill** | `.claude/skills/` | a dispatched reviewer **loads the skill as its review rubric** |

A **pinned** steward (audit/author/prose) absent from **its own registry** is a **HALT, not a
silent drop** — same posture as dropping the pin. A skill-steward (`audit-skill`/`author-skill`)
is checked against `.claude/skills/`; the agent-steward (`prose-steward`) against `.claude/agents/`.
The skill cannot quietly run a `skill-package` review with two of three stewards because the third
is absent from its registry; it halts and asks the operator (who may then override with a stated
reason). (`audit-skill`/`author-skill` will **never** appear in `.claude/agents/` — they are
skills, not agents — so checking them against the agent roster is the bug this rule forecloses.)

`idiomatic-reviewer` stays wired to **code presence**. `shared-stack` pulls stewards
**by file-type-present** (idiom if code, prose if prose, audit/author only if a `.claude/`
skill body is in the diff); `skill-package` **pins audit + author + prose unconditionally**,
because a skill is prose + discipline + an authored artifact regardless of which files the diff
happens to touch. This is why the two classes stay separate (§2 tie-break picks `skill-package`,
the strict superset).

The stewards report on their **own axes**, not the boundary table:
- `idiomatic-reviewer` → the **Idiom addendum** (correctness-grade blocks, style is advisory).
- `prose-steward` → a **Prose addendum** (parallel structure: clarity/audience findings;
  correctness-grade — e.g. an ambiguous guardrail an agent would misread — blocks; style is
  advisory).
- `audit-skill` / `author-skill` → **per-lens verdicts in the ledger** (RATIFY/DISSENT on
  "does this skill hold / is it authored correctly"), carrying their evidence. A DISSENT here
  blocks the same as a MISMATCH.

## 4a. Cross-cutting risk lenses (mandatory by concern)

§3/T2's "+ any cross-cutting specialist" pulls non-boundary specialists **discretionarily** — which is how a risk that isn't an interface boundary gets skipped (the dogfood lesson: a concurrency/race fix reviewed without the systems lens, because a race is not a boundary). For the two highest-consequence such concerns the pull is **mandatory, not discretionary** — wired off *what the change touches* (like the §4 stewards), not off a defect already being visible:

| Concern the change **touches** | Mandatory lens |
|---|---|
| concurrency, shared mutable state, lock/goroutine/async ordering, filesystem or resource lifecycle, back-pressure/timeout/retry | `systems-engineer` |
| trust boundary, untrusted/external input, credential/secret handling, authn/authz | `security-specialist` |

This table **mandates** these two; it does **not cap** the cross-cutting set — other concerns (capacity/cost → `k8s-capacity-management`; observability; etc.) stay on the §3/T2 discretionary route. A concern-lens here is the mandatory **subset** of §3/T2's "cross-cutting specialist," wired **once** (by this table) — not double-listed.

These are **domain lenses** (they report boundary-style findings, not an addendum), so an absent one degrades coverage without voiding the review's premise — **unlike a pinned §4 steward, whose absence HALTs because it voids the rubric, an absent concern-lens does not HALT.** Name it as a gap in the ledger's Routing section and proceed; an operator may drop one only with a stated reason (§5), recorded there.

## 5. Operator override (recorded, never silent)

The operator may name the slate or the tier explicitly (e.g. "xreview this as T2, drop the
prose-steward — it's a code-only refactor"). An override is **recorded in the ledger's Routing
section** with the operator's stated reason — never silent. Dropping a §4a mandatory
concern-lens is such an override — allowed, but recorded with the reason like any slate edit
(its default, unlike a pinned §4 steward's absence, is proceed-with-gap-named, not HALT).

**An override may lower the *default* tier, but never below the T2 floor for `shared-stack` /
`skill-package`** (the §3 floor rule is absolute — an override does not pierce it; the lowest a
`shared-stack`/`skill-package` change can be routed is T2). Lowering any change's default tier is
recorded with the operator's stated reason.

**The `skill-package` steward pin survives a tier override.** Lowering a `skill-package` change
from T3 to its T2 floor does **not** drop the unconditional `audit-skill` + `author-skill` +
`prose-steward` pin (§4) — the pin keys off the **class** (`skill-package`) and is unconditional
regardless of which file-types the diff touches, so it is orthogonal to tier. Dropping any pinned
steward is a **separate, explicit override with its own stated reason** (mirrors the skill's
"explicitly accepted by the user with the risk named" guardrail) — never a side effect of lowering
the tier.

## 6. The dissenter (the floor — §4 of Design 08)

Every routed slate names a `Dissenter`. It is **assigned, not emergent** — one lens tagged
red-team, picked as the lens *most likely to find the breaking boundary*. In a T1 single-
reviewer pass the dissent obligation **folds into the one reviewer** (an adversarial pass:
"argue this rename breaks a caller"), recorded as `Dissenter: <lens> (self, single-reviewer
pass)`. Dissent is never waived because the slate is one — it gets folded.

## How each skill applies this table

- **`/xreview` (review phase)** — classify the artifact under review, read the tier off
  §3, assemble the review slate (domain lenses covering the boundaries + the §4a concern-lenses
  + the §4 stewards), name the §6 dissenter. The orchestrator's remaining judgment is *which
  domain specialists* cover the boundaries; the *depth*, the *§4a concern-lenses*, and the
  *steward wiring* are mechanical.
- **`/coral` (production phase)** — classify the slice being built, read the tier off §3, pull
  the production specialists plus the §4a concern-lenses (a slice that *touches* a §4a surface
  pulls `systems-engineer`/`security-specialist` in production too) and the §4 stewards the
  file-types present demand (so a `shared-stack` slice pulls the stewards in *production* too, not
  only review). Coral's
  scope-cutter (`product-manager` on design briefs) and idiom-pass-on-code rules are the
  production-phase application of this same table.
