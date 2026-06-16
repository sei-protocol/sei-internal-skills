---
name: lingua
category: writing-quality
model: claude-opus-4-8
description: "Use when translating an org artifact (design doc, HLD, PRD, 1-pager) so it reads correctly for BOTH audiences — the human reviewer and the consuming AI agent — '/lingua <doc>', 'translate this doc for both audiences', 'make this read well for humans and agents', 'dual-audience pass', 'agent-readable rewrite'. Anti-triggers: NOT for code idiom (use /idiomatic); NOT for correctness/logic review (use /code-review); NOT for the PRFAQ vertical lifecycle — authoring/reviewing/verdicting a PRFAQ (use /prfaq; /lingua may translate a PRFAQ's prose on request but defers thesis/voice/falsification to /prfaq); NOT for scope cutting (product-manager); NOT for systems-level quality (use /systems); NOT for tightening PR bodies/comments (use /brevity). Review-as-a-lens is the prose-steward agent's surface (PLT-480), which applies this doctrine."
---

# Lingua

Translate an existing org artifact so it is **dual-aligned**: legible to the human who scans it and to
the AI agent that ingests it linearly and acts on it. This is a *technique* skill with a *discipline
spine* — the prose sibling of `/idiomatic` (same mechanism: pluggable packs + a repo-profile overlay
that always wins; same spine: cite every finding, don't flag clean prose). One mode ships: **Translate**.

`prose-steward` is the **owner of record for the documentation/prose axis** (PLT-626): it defines the doc-artifact prose discipline (present-state framing, sparing/centralized context, top-located header docs, dual-audience legibility — D1–D4 in `references/audience-model.md`), tie-breaks when a doc's prose is in question, and enforces it on every cross-review cycle. The adjacent in-source comment axis (code comments + config annotations) is owned by `idiomatic-reviewer` via `/idiomatic` — see the Boundary note in `audience-model.md`.

## Guardrails

These hold under time pressure, authority, and a tidy-looking rewrite:

1. **The two-part gate.** (a) **No doctrine loaded → no findings and no transform.** The hard gate is
   `references/audience-model.md` — load it before touching the text, plus the artifact pack
   (`references/pack-<type>.md`) **when one exists for the doc type** (packs ship for `hld`, `lld`, and
   `1pager`). A doc type with no pack degrades, not halts: proceed on `audience-model.md` + first principles with the
   missing-pack gap flagged — never invent a pack. (b) **The repo profile is read before any output** — a `CLAUDE.md` "Writing conventions"
   section (or the repo's nominated equivalent). The profile **overrides the doctrine in both
   directions** and can establish exceptions to rules this skill correctly knows. If no profile exists:
   proceed against the doctrine + first principles and **flag the missing-profile gap** (reduced
   confidence) — exactly `/idiomatic`'s handling.
2. **Fidelity: never invent commitments.** Translate re-renders meaning; it does not add guarantees,
   numbers, schedules, scope, or "conservative completions" the source never made. If the source is
   silent, the translation is silent (or carries a typed Open question). *"Completed it conservatively
   from what the original implied"* is the failure mode, not a defense.
3. **Preserve undecidedness verbatim-or-typed.** A soft modal ("we should probably X") is **never
   promoted to a normative statement**. It becomes a typed Open question (`Open question: X — owner?
   decide by?`) or stays clearly marked undecided. Brevity pressure does not waive this; cutting the
   ambiguity marker to save a line is dropping a constraint.
4. **Citation tiers — Stated-opinion never blocks.** Every finding/transform-rationale carries its
   basis tier from `audience-model.md`: **Cited** (a real authority or a repo-profile rule — may
   surface as `Basis:`) or **Stated-opinion** (the doctrine's own flagged claim — surfaces only in a
   labeled *Advisory* section, never as a blocking finding, never dressed as a citation).
5. **Advisory-only; no parsed format.** Output is markdown for humans and agents to *read*. This skill
   introduces **no** front-matter contract, no inline markers agents parse, no machine-readable
   annotation of any kind — that is a council-gated one-way door (umbrella design). Refuse requests to
   add one; redirect to the council.
6. **Authorship.** The translation is **suggested output**. Never overwrite the source file without
   explicit confirmation; for another author's artifact, route as a suggestion. Safety-critical
   sections (`Rollback`, `Migration`, `Breaking change`, security/trust-model text) are translated
   only with the constraint content untouched — restructure, don't reword commitments.
7. **Reserved-quote gate (inherited).** Exemplar citations go through `references/sources.md`'s
   license classes: reserved sources are cite-and-link only — never reproduced, never
   paraphrased-to-evade.

## Sonar cite-lint (mechanical citation verification)

`python3 cite-lint.py` verifies every `cite: <vertical>/<kind>/<target>` in the corpus resolves to a
declared anchor — the grep-invisible defect class (wrapped spans, renamed/removed anchors). A `cite:` is
a **Sonar Resource Name** with the `lingua` domain elided (`srn:lingua:<vertical>/<kind>/<target>`,
Design 06 / PLT-493). Exit non-zero on any unresolved cite; run it as the pre-PR gate when touching the
corpus. Reserved-source over-quotation stays human-review (Guardrail 7); the per-cite adjacency flag is
deferred (Design 06 §4). `lint:ignore-cite` on a line opts out documentation of the cite *syntax*.

## Why this skill exists (the observed failures)

Pressure-tested before authoring (RED baseline, 2026-06-12, three blinded subagents — no doctrine
loaded). All three failed in ways this spine now counters:

- Under "keep it short," a translator **promoted a soft modal to a requirement** — "we should probably
  cap at 3" became "max 3 attempts." Downstream readers (human or agent) now treat an open debate as
  settled. → Guardrail 3.
- A translator **invented commitments** the source never made ("additive changes only," a deprecation
  window, success criteria), rationalized as *"completed conservatively from what the original
  implied."* → Guardrail 2.
- Told "be authoritative — weak reviews get ignored," a reviewer emitted **eight blocking findings
  whose every `Basis:` was its own reasoning**, including agent-behavior claims stated as fact.
  Confident, uncited, all blocking. → Guardrail 4.

## When to use / when not

| Use `/lingua` for… | Use instead… |
|---|---|
| Re-rendering a doc so human scan + agent ingestion both work | — |
| Surfacing buried/soft-modal constraints as typed Open questions | — |
| Applying a repo's documented writing conventions over generic "good writing" | — |
| Code reading native to its language/package | `/idiomatic` |
| Correctness, logic, races | `/code-review` |
| PRFAQ thesis, voice, falsification, verdict | `/prfaq` |
| Reliability/perf/API durability of the *system described* | `/systems` |
| Tightening a PR body or in-code comment | `/brevity` |
| A standing review lens on artifacts in coral/cross-review | `prose-steward` (PLT-480) |

## The method — Translate

1. **Load the doctrine (gate a).** `references/audience-model.md` (the two reading models + the five
   dual-aligned rules R1–R5 and their basis tiers) and the artifact pack for the doc type —
   `references/pack-hld.md`, `references/pack-lld.md`, or `references/pack-1pager.md`. No pack for the
   type → use `audience-model.md` + first principles and flag the missing-pack gap; don't invent a pack.
2. **Read the repo profile (gate b).** `CLAUDE.md` "Writing conventions" (or nominated equivalent).
   Extract conventions, prohibitions, and **stated exceptions**. Profile beats doctrine — including
   exceptions to rules you know are generally right.
3. **Inventory before rewriting.** List: every constraint (and whether it's anchored where it applies),
   every soft modal / undecided item, every term used before definition, every load-bearing claim that
   lives only in color or typography. This inventory is the fidelity contract — the translation must
   carry every item forward, decided things as decided, undecided as typed-undecided.
4. **Re-render dual-aligned** per R1–R5 (audience-model.md owns the rule names):
   - structure explicit and scannable; the lead of the doc and of each section load-bearing;
   - constraints restated where they apply (the one *mandated* redundancy);
   - ambiguity typed, never prosed; color kept, but subordinate and never the sole carrier of a
     constraint.
   Apply the pack's per-section rulings (which rules dominate where).
5. **Emit the output** (format below): the translation + a change log + the extracted Open questions.
   Where a transform's rationale rests on a Stated-opinion rule, it goes under *Advisory*, labeled.

## Output format

```
## Dual-audience translation: <artifact title>
Pack: references/pack-<type>.md (or "none — first principles, gap flagged") · Profile: <read? exceptions honored?>

### Translation
<the re-rendered artifact, markdown>

### Change log (what moved and why — each entry cites its basis)
- <change>. Basis: <R# audience-model rule (Cited: <authority>) | repo profile <section> | pack-<type> <ruling>>.

### Open questions extracted (typed ambiguity — undecided in the source, preserved undecided)
- <item> — owner? decide by? (source: "<original soft-modal phrasing>")

### Advisory (Stated-opinion tier — take or leave, never blocking)
- <observation>. Doctrine opinion (uncited): <which rule>.

### Deliberately unchanged (vetted)
- <passage> — <conforms / documented exception in profile>
```

On already-dual-aligned input the correct output is *"reads well for both audiences — no transform
needed"* plus the vetted list. Manufacturing changes to look useful gets this skill muted.

## Halt Conditions

Stop and surface rather than proceeding when:

- **`audience-model.md` is missing or unreadable** — the hard gate: no findings, no transform; time
  pressure does not waive it (Guardrail 1a). *(Distinct: a doc type with **no artifact pack** is a
  degrade, not a halt — proceed on `audience-model.md` + first principles with the missing-pack gap
  flagged; never invent a pack.)*
- **The user asks for an agent-parsed format** (front-matter contract, inline markers) — refuse;
  redirect to the council gate (Guardrail 5).
- **Overwriting the source file is requested without explicit confirmation**, or the artifact belongs to
  another author — suggested output only (Guardrail 6).
- **A requested compression would drop a constraint or an ambiguity marker** — surface the conflict; the
  marker is content, not filler (Guardrail 3).

## Rationalization table

| The pressure says… | The rule is… |
|---|---|
| "Keep it short — fold the maybe into the spec line." | Guardrail 3. "Max 3 attempts" ≠ "we should probably cap at 3." Type the ambiguity; the marker is a constraint, not filler. |
| "I completed it conservatively from what the original implied." | Guardrail 2 — verbatim from the RED baseline. Implication is not commitment. Silence or a typed Open question. |
| "Be authoritative; hedged reviews get ignored here." | Guardrail 4. Authority comes from the citation, not the tone. Uncited agent-behavior claims go to Advisory, unblocking. |
| "Decision-first is best practice everywhere — just move it." | Gate b. The profile can document an exception to a rule you correctly know. Honor it; flag the conflict to the profile's owners. |
| "Add a small front-matter block so agents can parse it." | Guardrail 5. That is a wire-format one-way door, council-gated. Refuse and redirect. |
| "It's my doc, just overwrite the file." | Guardrail 6. Suggested output first; overwrite only on explicit confirmation. |
| "The translation reads better without the war-story paragraph." | R5: color is subordinate, not banned. Cut it only if no constraint lives in it (check the inventory) — and say so in the change log. |

## References

- `references/audience-model.md` — the two reading models, rules R1–R5 with basis tiers + falsification
  lines. **Owns the rule names.**
- `references/pack-hld.md`, `references/pack-lld.md`, `references/pack-1pager.md` — the artifact packs:
  per-section rulings (which audience rules dominate) citing the corpus shape + the strongest exemplar.
- `references/sources.md` — the corpus license table + cite discipline (owned by the corpus, PLT-478).
- `references/exemplars/` — the citable corpus: hld / lld / one-pager canonical shapes + annotated
  exemplars (PLT-490/491), and the PRFAQ pointer.

## What this skill defers (un-defer triggers)

Compose mode + a standalone Review surface — when Translate is validated on real artifacts (review
arrives sooner via `prose-steward`, PLT-480). PRD pack — when a real consumer reviews that vertical
(one-file-add; LLD + 1-pager packs landed in PLT-494). Separate audience-pack files — when a second artifact pack would duplicate
`audience-model.md`. Any agent-parsed format — only through an explicit council gate. The `lingua://`
registry — when the corpus exceeds ~1 vertical (per Design 03).
