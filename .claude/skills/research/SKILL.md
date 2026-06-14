---
name: research
category: investigation
model: claude-opus-4-8
description: "Use when a research question needs a durable, verified, lineage-threaded answer — 'research X', 'do a deep dive on Y', 'survey the options for Z', 'investigate the state of the art on W', 'gather evidence on whether ...', '/research'. Runs a scoped, multi-modal sweep, adversarially verifies each finding, runs a completeness pass, and captures a research artifact threaded to issues/bets. Anti-triggers: NOT incident/bug root-causing in the Sei platform stack (use /root-cause); NOT capturing a design decision (use /design — research discovers, design decides); NOT a quick one-off lookup that needs no durable artifact (just answer); NOT launching a workstream (a research effort may be checkpoint-gated by /workstream but never launches one). Reuses /cross-review's assigned-dissent primitive for finding-refutation; composes /design-style capture + /execution-plan lineage."
---

# Research

Answer a research question with a **durable, verified, lineage-threaded** artifact — not a chat reply that evaporates. This is a *technique* skill (a four-stage method you adapt to the question) with a *discipline spine* (no finding ships unverified; a vague question is refused). It generalizes the skill-authoring research recipe (`author-skill/references/research-recipe.md`) into a first-class capability the rest of the stack can use.

It composes the framework rather than reinventing it: it reuses `/cross-review`'s **assigned-dissent primitive** for finding-refutation (not the skill — see the spine), captures like `/design` does, and threads lineage via `/execution-plan`.

## Why this skill exists (read this first)

Research outputs today are less reusable, less verifiable, and don't accrue into the bet↔design↔issue↔PR graph the way the rest of the stack does. The failure mode the spine prevents: an agent fans out a sweep, finds a plausible-sounding claim, and **ships it without refutation** because it "looks right" and the operator wants an answer. A research answer is only as good as the weakest unverified finding in it — so the differentiator is not the sweep (any agent can search), it is the **adversarial verify gate** and the **completeness pass**.

## Guardrails

Refusal conditions — these hold under "just give me a quick answer" pressure:

1. **No finding ships unverified.** Every material finding gets a refutation pass before it's trusted. A finding that survives refutation is **verified** (with the refutation move recorded — an *unattempted* refutation yields `unverified`, not `verified`); one the refutation *disproves* is **refuted** and dropped; one it can neither confirm nor disprove is **unverified**, kept only with that label. Never present an unverified finding as established.
2. **Refuse a vague question.** If the question doesn't name what a *useful answer* looks like (the decision it informs, the falsifiable claims sought), push back and sharpen it before sweeping. A sweep with no target returns noise.
3. **Reuse the dissent primitive, don't invoke `/cross-review`.** `/cross-review` reviews *interface boundaries* (provider/consumer, COMPATIBLE/MISMATCH/MISSING) — that table doesn't map onto a research finding. `research` implements its **own** refutation pass *modeled on* cross-review's assigned-dissent primitive (tag a skeptic to argue the finding is wrong). Do not call `/cross-review` on findings.
4. **Discover, don't decide.** Research surfaces findings + a recommendation; it does not capture a design decision (that's `/design`) or file work (that's `/issue`). Keep the artifact a *findings* artifact.
5. **Never launch a workstream.** A research effort may be *checkpoint-gated by* a `/workstream` (an `outcome-alignment` gate after synthesis), but it never *launches* one. At scale it spawns a Workflow (an execution engine) for the parallel sweep — deferred from MVP; inline is the default.

## The method (four stages)

### 1. Scope

State the question and what a useful answer is: the **decision it informs**, the **falsifiable claims** sought, and the **scope boundary** (what's in/out). Refuse a vague question (Guardrail 2). Write the scoped question at the top of the artifact — it's the contract the completeness pass checks against.

### 2. Multi-modal sweep

Fan out searches, each blind to the others' angle, so one search angle's blind spot doesn't sink the whole effort. Canonical angles (pick the ones the question needs):

- **by-source** — official docs / primary sources / specs
- **by-entity** — the named tools, projects, people, prior art
- **by-time** — recent evolution (what changed in the last 12–24 months)
- **by-counter-thesis** — search for the *opposite* of the expected answer (what would refute the hypothesis)

Inline for ≤3 angles (the MVP norm). Broader sweeps want a Workflow (deferred). Record, per finding, *which angle surfaced it* and *the source* — a finding with no retrievable source is not a finding.

### 3. Adversarially verify (the differentiator)

For each material finding, run a **refutation pass**: assign a skeptic stance and argue the finding is *wrong* — find the contradicting source, the stale citation, the overgeneralization, the sample-of-one. This reuses `/cross-review`'s assigned-dissent primitive (a tagged red-team), applied to findings rather than boundaries (Guardrail 3). Outcome per finding:

- **verified** — survived refutation; cite the source and note what the refutation tried and failed to do.
- **refuted** — dropped; note why (so the next sweep doesn't re-surface it).
- **unverified** — couldn't confirm or refute; kept *only if* labeled unverified, never presented as established.

### 4. Completeness pass + synthesize

Run **one** completeness pass: "what modality wasn't run, what claim is unverified, what source is unread, what part of the scoped question is unanswered?" **Report** the gaps; the human decides whether to run another sweep round (no auto-loop in MVP). Then synthesize: the findings (tagged), the gaps, and a recommendation that answers the scoped question's decision — **grounded only in verified findings** (unverified findings inform open questions, not the recommendation).

## The artifact

Capture as `design/research/<slug>.md` (the existing Tide research-doc home). Shape:

```markdown
# Research: <Title>

**Status:** <Draft | Final>
**Date:** YYYY-MM-DD
**Issue:** <#n | EID — url>     (omit if none)
**Impact:** <slug> — <url>      (omit if no bet)
**Authors:** <user>, ...

## Question
<the scoped question: the decision it informs, the falsifiable claims, the scope boundary>

## Sweep coverage
<which angles were run; what each covered; what was deliberately not swept>

## Findings
- **[verified]** <finding>. Source: <url>. Refutation tried: <what the skeptic attempted; why it failed>.
- **[unverified]** <finding>. Source: <url>. Why unverified: <couldn't confirm/refute>.
  (refuted findings are dropped, noted once below)

## Completeness assessment
<gaps: modality not run, claim unverified, source unread, scope unanswered — and whether to re-sweep>

## Synthesis & recommendation
<the answer to the scoped question's decision, grounded only in verified findings>

## References
<sources, prior research, related designs/issues>
```

**Lineage.** Thread exactly as `/design` does: frontmatter `Issue:`/`Impact:` + offer an `/execution-plan` decoration call so the research accrues into the graph. **Precision (do not get this wrong):** `betGraph`'s `designLinked` means "links the *bet's design* URL" — a research-doc URL is *not* the bet's design URL. So a research effort that advances a bet threads through the **issue that already carries the `impact:<slug>` label and the bet's design-URL link**; the research doc is an *additional reference* on that issue, not a competing discriminator. (Whether `betGraph` should treat a research-doc URL as a second valid discriminator is a `/execution-plan` contract question — logged as a follow-up, out of scope here.)

## Rationalization table

| The pressure says… | The rule is… |
|---|---|
| "This finding looks obviously right — verifying it wastes time." | Obvious-looking findings are exactly the ones that ship stale or overgeneralized. Run the refutation pass; cite what it tried and failed to do. |
| "The operator wants an answer now — ship the sweep results." | A sweep is raw material, not an answer. Unverified findings shipped as established is the failure this skill exists to prevent. Label unverified as unverified. |
| "I only found one source, but it's authoritative." | One source is a sample of one — the by-counter-thesis angle exists precisely to test it. Either corroborate or label unverified. |
| "Cross-review is the verification skill — I'll just run `/cross-review` on the findings." | Cross-review's boundary table doesn't fit findings. Reuse only its assigned-dissent *primitive*; run the refutation pass yourself. |
| "I should capture this as a design so it threads lineage." | Research *discovers*; design *decides*. Capture a findings artifact; thread lineage the same way `/design` does, but don't masquerade findings as a decision. |
| "I ran a quick refutation and nothing jumped out — that's verified." | A refutation that surfaces no recorded contradicting-source / freshness / overgeneralization / sample check was *not attempted*, not *passed*. No recorded refutation move → `unverified`, never `verified`. |

## Halt Conditions

Stop and surface rather than proceeding when:

- **The question is vague** — refuse and sharpen it (the decision it informs, the falsifiable claims) before sweeping (Guardrail 2).
- **A material finding can't be verified or refuted** — keep it only labeled *unverified*; never promote it to established.
- **The completeness pass surfaces a gap that changes the recommendation** — report it; let the human decide whether to re-sweep (no auto-loop).
- **The sweep needs >3 angles / broad parallelism** — note that an inline sweep is too narrow; the Workflow engine is deferred, so surface the limit rather than running an under-powered sweep silently.

## State

Per-run sweep notes and the in-progress finding ledger live in `state/` (gitignored). The durable output is the artifact in `design/research/`.

## References

- `references/method.md` — the four stages in depth: sweep angles, the refutation-pass protocol (the assigned-dissent primitive adapted to findings), the completeness checklist, the artifact template.

## What this skill defers

The Workflow engine for broad parallel sweeps (ships inline; *deferred — when an inline sweep is observably too narrow, i.e. >3 angles needed repeatedly*); an auto-looping completeness critic (ships one reporting pass; *deferred — when verified-but-incomplete artifacts cause a real re-research*); a research result cache / `research://` registry (*deferred — when artifacts are re-read across ≥3 workstreams*).
