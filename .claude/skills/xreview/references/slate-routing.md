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
  `skill-package` is the strict-superset wiring — it adds the unconditional prose + rubric-lens
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
a `.claude/` skill body is still `skill-package` (T3, floor T2, prose + rubric pinned). "It's
just a typo in a skill" does **not** demote it to `mechanical`. The *classification* keys off
*what kind of file changed* (a `.claude/` skill body present → `skill-package`), not how many
lines; the prose + rubric-lens pin then follows from that class **unconditionally** — a skill is
prose and an authored artifact whether the diff is one line or a hundred. Dropping the
pin requires an **operator override with a stated reason, never a size judgment**.

## 4. Auto-wired standards-stewards

The stewards review *how the artifact is built*, orthogonal to its domain boundaries. The
wiring keys off **what kind of file changed**, evaluated independently of the domain slate (a
change can pull a steward without that steward owning a boundary):

| Steward | Auto-included when the change touches… | Axis it owns |
|---|---|---|
| `prose-steward` | any prose artifact — a design doc, a skill body, a README, a runbook | reads-native-for-its-reader (the prose-steward doctrine) |
| `idiomatic-reviewer` | any code diff / implementation | reads-native to language/framework/package patterns |
| the rubric lens | a `.claude/` **skill** body or its references | is the skill authored correctly, and does its discipline survive pressure — triggers, anti-triggers, guardrails, evals |

**The load-bearing wiring rule (state it in the citing skill):**

> A **`skill-package` change auto-pulls `prose-steward` + the rubric lens** —
> always (T3 by default; the pin survives a tier override down to the T2 floor, see §5), not
> subject to silent omission. A skill *is* prose (→ prose-steward), and it *is* an authored
> artifact whose discipline must survive pressure (→ the rubric lens). Dropping either on a
> skill change requires an **operator override with a stated reason**.

### Steward dispatch — two kinds (registry stewards vs the rubric lens)

The stewards are **not all the same kind of artifact**. Two are registry entries that can be
missing. The third is not a registry entry at all:

| Steward | Kind | Where it comes from | How it's dispatched | Can it be absent? |
|---|---|---|---|---|
| `prose-steward` | **agent** (self-contained) | `.claude/agents/` | dispatched as a subagent | yes → HALT |
| `idiomatic-reviewer` | **agent** (backed by `/idiomatic`) | `.claude/agents/` | dispatched as a subagent | yes → HALT when pinned |
| the rubric lens | **a brief, not an entry** | `references/skill-package-rubric.md` in this skill | any dispatched reviewer, briefed to load the rubric and cite rule ids | the **lens**, no — the **rubric file**, yes → HALT |

**The rubric lens has no registry, so the *lens* has no absence check.** It is a brief:
the orchestrator picks any capable reviewer and tells it to load this skill's own
`references/skill-package-rubric.md`, **runs** `scripts/skill-package-checks.sh` for the
static rules, and return findings that **name rule ids**. The rubric ships with `/xreview`, so
it cannot go missing the way a separate installed skill could — that dependency is exactly what
the pre-cut wiring had, and it halted the review whenever an install lacked one.

**The rubric *file* is a different object, and it can still be missing or truncated in a broken
install.** If the lens cannot read `references/skill-package-rubric.md`, **HALT** (`SKILL.md`
Halt Conditions). The rule ids are short and schematic — `D1`, `B2`, `S2` — so an unread rubric
yields plausible ids emitted from memory: a review that looks cited and is not. Distinguish the
two: the lens cannot be absent because it is not a thing that installs; the file it reads can.

Losing the absence check means the pin needs a different guarantee, and this is it:

> **A rubric-lens verdict that cites no rule id is not a rubric review.** Re-dispatch it. The
> rule id is what makes the verdict falsifiable — a reader can look it up and disagree. An
> uncited "the skill looks fine" is the bare approval Rule 3 already rejects, wearing a
> steward's name.

Do **not** look for the rubric lens in `.claude/agents/` or `.claude/skills/` and halt on its
absence. It was never going to be there.

An agent-steward absent from `.claude/agents/` **is** a HALT, not a silent drop — same posture
as dropping the pin. The skill cannot quietly run a `skill-package` review without
`prose-steward`; it halts and asks the operator, who may override with a stated reason.

`idiomatic-reviewer` stays wired to **code presence**. `shared-stack` pulls stewards
**by file-type-present** (idiom if code, prose if prose, the rubric lens only if a `.claude/`
skill body is in the diff); `skill-package` **pins `prose-steward` + the rubric lens
unconditionally**, because a skill is prose *and* an authored artifact regardless of which files
the diff happens to touch. This is why the two classes stay separate (§2 tie-break picks `skill-package`,
the strict superset).

The stewards report on their **own axes**, not the boundary table:
- `idiomatic-reviewer` → the **Idiom addendum** (correctness-grade blocks, style is advisory).
- `prose-steward` → a **Prose addendum** (parallel structure: clarity/audience findings;
  correctness-grade — e.g. an ambiguous guardrail an agent would misread — blocks; style is
  advisory).
- the rubric lens → a **per-lens verdict in the ledger** (RATIFY/DISSENT on "is this skill
  authored correctly and does its discipline hold"), carrying **rule ids** as its evidence. A
  DISSENT here blocks the same as a MISMATCH.

## 4a. Cross-cutting risk lenses (mandatory by concern)

§3/T2's "+ any cross-cutting specialist" pulls non-boundary specialists **discretionarily** — which is how a risk that isn't an interface boundary gets skipped (the dogfood lesson: a concurrency/race fix reviewed without the systems lens, because a race is not a boundary). For the two highest-consequence such concerns the pull is **mandatory, not discretionary** — wired off *what the change touches* (like the §4 stewards), not off a defect already being visible:

| Concern the change **touches** | Mandatory lens |
|---|---|
| concurrency, shared mutable state, lock/goroutine/async ordering, filesystem or resource lifecycle, back-pressure/timeout/retry | `systems-engineer` |
| trust boundary, untrusted/external input, credential/secret handling, authn/authz | `security-specialist` |
| **the agent-instruction surface itself** — `.claude/skills/**`, `.claude/agents/**`, `agents/**`, `scripts/managed-settings.json`, `scripts/agent-permissions.json`, `Dockerfile.runner*`, `.github/workflows/**` | `security-specialist` |

**Why the instruction surface is a trust boundary.** `.claude/skills/` is not only a
menu an engineer picks from. It is also the discovery scope of a headless bundle that
approves its own tool calls, seeded there by the runner image, and the agents it can
reach carry their own tool grants. A change to that surface is a change to what an
auto-approving agent can be told to do. A `skill-package` change pins the prose steward
and the rubric lens unconditionally and did **not** pin security — so the
highest-consequence review in this repository, of this repository, did not mandate the
lens that found this.

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
from T3 to its T2 floor does **not** drop the unconditional `prose-steward` + rubric-lens pin (§4) — the pin keys off the **class** (`skill-package`) and is unconditional
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
