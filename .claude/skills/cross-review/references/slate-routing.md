# Slate Routing — the shared change-type × blast-radius → slate rule

The canonical routing rule for **both** `/cross-review` (review-phase slate) and `/coral`
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
doc-only  <  mechanical  <  component  <  cross-component  <  shared-stack  ≈  skill-package
```

- A `doc-only` design that ships a `shared-stack` diff routes as `shared-stack`.
- The `shared-stack ≈ skill-package` tie (both top of the order) **resolves to `skill-package`**.
  `skill-package` is the strict-superset wiring — it adds the unconditional audit+author+prose
  pin (§4), so picking it never under-pins. A diff touching both a shared library *and* a
  `.claude/` skill is `skill-package`.

## 3. Blast-radius tiers (the depth dial — authoritative class→tier map)

Three tiers. The class sets the default tier; blast-radius can bump it **up** (never silently
down).

| Tier | Slate depth | Default for (authoritative class→tier map) |
|---|---|---|
| **T1 — light** | 1–2 lenses; single-reviewer pass allowed (labeled as such) | `mechanical`; `doc-only` **only if mechanical-equivalent** (typo/formatting/link — see §3a) |
| **T2 — domain** | the domain slate covering the boundaries (provider + consumer + any cross-cutting specialist) | `component`, `cross-component`, and any `doc-only` that proposes or alters a decision |
| **T3 — full + stewards** | full domain slate **plus** the auto-wired standards-stewards (§4) | `shared-stack`, `skill-package` |

**The floor rule:** a `shared-stack` or `skill-package` change is **T3 by default and cannot
drop below T2.** That floor is the dogfood lesson — the canonical-stack change is the one that
must not get a rubber-stamp slate.

### 3a. `doc-only` T1-vs-T2 split (a stated test, not a vibe)

A `doc-only` artifact is **T1 only if it is mechanical-equivalent** — a typo fix, a
formatting/whitespace change, a link/path correction, with **no change to any decision or
claim**. **Any doc-only change that proposes or alters a decision is T2** (a decision needs a
reviewer who can judge it). When in doubt, **T2**.

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
> always, at T3, not subject to silent omission. A skill *is* prose (→ prose-steward), it *is*
> a discipline that must survive pressure (→ audit-skill), and it *is* an authored artifact
> with triggers/guardrails/evals (→ author-skill). Dropping any of the three on a skill change
> requires an **operator override with a stated reason**.

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

## 5. Operator override (recorded, never silent)

The operator may name the slate or the tier explicitly (e.g. "cross-review this as T2, drop the
prose-steward — it's a code-only refactor"). An override is **recorded in the ledger's Routing
section** with the operator's stated reason — never silent. An override may **lower** the tier;
lowering a `skill-package` change below T2, or dropping an auto-wired steward, **requires the
operator to state the risk** (mirrors the skill's "explicitly accepted by the user with the
risk named" guardrail).

## 6. The dissenter (the floor — §4 of Design 08)

Every routed slate names a `Dissenter`. It is **assigned, not emergent** — one lens tagged
red-team, picked as the lens *most likely to find the breaking boundary*. In a T1 single-
reviewer pass the dissent obligation **folds into the one reviewer** (an adversarial pass:
"argue this rename breaks a caller"), recorded as `Dissenter: <lens> (self, single-reviewer
pass)`. Dissent is never waived because the slate is one — it gets folded.

## How each skill applies this table

- **`/cross-review` (review phase)** — classify the artifact under review, read the tier off
  §3, assemble the review slate (domain lenses covering the boundaries + the §4 stewards),
  name the §6 dissenter. The orchestrator's remaining judgment is *which domain specialists*
  cover the boundaries; the *depth* and *steward wiring* are mechanical.
- **`/coral` (production phase)** — classify the slice being built, read the tier off §3, pull
  the production specialists plus the §4 stewards the file-types present demand (so a
  `shared-stack` slice pulls the stewards in *production* too, not only review). Coral's
  scope-cutter (`product-manager` on design briefs) and idiom-pass-on-code rules are the
  production-phase application of this same table.
