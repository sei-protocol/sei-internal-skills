---
name: research
category: investigation
model: claude-opus-5
description: "Use when a research question needs a durable, verified, lineage-threaded answer — 'research X', 'do a deep dive on Y', 'survey the options for Z', 'investigate the state of the art on W', 'gather evidence on whether ...', '/research'. Runs a scoped, multi-modal sweep, adversarially verifies each finding, runs a completeness pass, and captures a research artifact threaded to issues/bets. Anti-triggers: NOT incident/bug root-causing in the Sei platform stack (use /root-cause); NOT capturing a design decision (use /design — research discovers, design decides); NOT a quick one-off lookup that needs no durable artifact (just answer); NOT launching a workstream (a research effort may be checkpoint-gated by /workstream but never launches one). Reuses /xreview's assigned-dissent primitive for finding-refutation; composes /design-style capture."
---

# Research

Answer a research question with a **durable, verified, lineage-threaded** artifact — not a chat reply that evaporates. This is a *technique* skill (a four-stage method you adapt to the question) with a *discipline spine*. The spine: no finding ships unverified, and the skill refuses a vague question. It generalizes the skill-authoring research recipe into a first-class capability the rest of the stack can use. That is the deep-research recipe (cut with `author-skill`; the method is inline below).

It composes the framework rather than reinventing it. It reuses `/xreview`'s **assigned-dissent primitive** for finding-refutation (not the skill — see the spine), and captures like `/design` does.

## Why this skill exists (read this first)

Research outputs today are less reusable and less verifiable. They do not accrue into the bet↔design↔issue↔PR graph the way the rest of the stack does. The failure mode the spine prevents: an agent fans out a sweep, finds a plausible-sounding claim, and **ships it without refutation**. It does so because the claim "looks right" and the operator wants an answer.

A research answer is only as good as the weakest unverified finding in it. The differentiator is therefore not the sweep (any agent can search). It is the **adversarial verify gate** and the **completeness pass**.

## Guardrails

Refusal conditions — these hold under "just give me a quick answer" pressure:

1. **No finding ships unverified.** Every material finding gets a refutation pass before you trust it. A finding that survives refutation counts as **verified**, with the refutation move recorded — an *unattempted* refutation yields `unverified`, not `verified`. One the refutation *disproves* counts as **refuted** and drops out. One it can neither confirm nor disprove counts as **unverified**, kept only with that label. Never present an unverified finding as established.
2. **Refuse a vague question; confirm scope before sweeping.** If the question does not name what a *useful answer* looks like (the decision it informs, the falsifiable claims sought), push back and sharpen it. Then **echo the scoped question** (decision + claims + scope boundary) and get the operator's go-ahead before the sweep. A sweep with no confirmed target returns noise.
3. **Reuse the dissent primitive, do not invoke `/xreview`.** `/xreview` reviews *interface boundaries* (provider/consumer, COMPATIBLE/MISMATCH/MISSING) — that table does not map onto a research finding. `research` implements its **own** refutation pass *modeled on* xreview's assigned-dissent primitive (tag a skeptic to argue the finding is wrong). Do not call `/xreview` on findings.
4. **Discover, do not decide.** Research surfaces findings + a recommendation; it does not capture a design decision (that is `/design`) or file work (that is `/issue`). Keep the artifact a *findings* artifact.
5. **Never launch a workstream; surface a too-wide sweep.** A `/workstream` may *checkpoint-gate* a research effort (an `outcome-alignment` gate after synthesis), but research never *launches* one. Inline covers **≤3 sweep angles**. If the question needs more, **surface the limit** rather than running a narrow sweep silently. The parallel-sweep Workflow engine stays deferred from MVP.

## The method (four stages)

### 1. Scope

State the question and what a useful answer is: the **decision it informs**, the **falsifiable claims** sought, and the **scope boundary** (what's in/out). Refuse a vague question (Guardrail 2). **Echo the scoped question to the operator and get go-ahead before the sweep** (Guardrail 2). Write the scoped question at the top of the artifact — it is the contract the completeness pass checks against.

### 2. Multi-modal sweep

Fan out searches, each blind to the others' angle, so one search angle's blind spot does not sink the whole effort. Canonical angles (pick the ones the question needs):

- **by-source** — official docs / primary sources / specs
- **by-entity** — the named tools, projects, people, prior art
- **by-time** — recent evolution (what changed in the last 12–24 months)
- **by-counter-thesis** — search for the *opposite* of the expected answer (what would refute the hypothesis)

Inline for ≤3 angles (the MVP norm). Broader sweeps want a Workflow (deferred). Record, per finding, *which angle surfaced it* and *the source* — a finding with no retrievable source is not a finding.

### 3. Adversarially verify (the differentiator)

For each material finding, run a **refutation pass**: assign a skeptic stance and argue the finding is *wrong*. Find the contradicting source, the stale citation, the overgeneralization, the sample-of-one. This reuses `/xreview`'s assigned-dissent primitive (a tagged red-team), applied to findings rather than boundaries (Guardrail 3). Outcome per finding:

- **verified** — survived refutation; cite the source and note what the refutation tried and failed to do.
- **refuted** — dropped; note why (so the next sweep does not re-surface it).
- **unverified** — could not confirm or refute; kept *only if* labeled unverified, never presented as established.

### 4. Completeness pass + synthesize

Run **one** completeness pass: "what modality was not run, what claim is unverified, what source is unread, what part of the scoped question remains unanswered?" **Report** the gaps; the human decides whether to run another sweep round (no auto-loop in MVP). Then synthesize: the findings (tagged), the gaps, and a recommendation that answers the scoped question's decision, **grounded only in verified findings**. Unverified findings inform open questions, not the recommendation.

## The artifact

Capture in the DRI's designs repo: the engineer's `<name>-designs` repo. The file sits under the work-arc folder as `designs/<arc>/research/<slug>.md`. Per Design 05, research artifacts live in the DRI repo, not the code/skills package. **Resolve the DRI repo as `/design` does** (`--designs-repo` flag → a sibling `<name>-designs` checkout → ask the user). Fall back to in-repo `docs/research/` only if the user confirms they have no designs repo. Shape:

```markdown
# Research: <Title>

**Status:** <Draft | Final>
**Date:** YYYY-MM-DD
**Issue:** <#n | EID — url>     (omit if none)
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

**Lineage.** Thread exactly as `/design` does: a frontmatter `Issue:` line on the findings artifact pointing at the ticket the research serves.


## Rationalization table

| The pressure says… | The rule is… |
|---|---|
| "This finding is obvious — verifying it wastes time." | Obvious-looking findings are exactly the ones that ship stale or overgeneralized. Run the refutation pass; cite what it tried and failed to do. |
| "The operator wants an answer now — ship the sweep results." | A sweep is raw material, not an answer. Unverified findings shipped as established is the failure this skill exists to prevent. Label unverified as unverified. |
| "I only found one source, but it is authoritative." | One source is a sample of one — the by-counter-thesis angle exists precisely to test it. Either corroborate or label unverified. |
| "xreview is the verification skill — I will just run `/xreview` on the findings." | xreview's boundary table does not fit findings. Reuse only its assigned-dissent *primitive*; run the refutation pass yourself. |
| "I should capture this as a design so it threads lineage." | Research *discovers*; design *decides*. Capture a findings artifact; thread lineage the same way `/design` does, but do not masquerade findings as a decision. |
| "I ran a quick refutation and nothing jumped out — that counts as verified." | A refutation that surfaces no recorded contradicting-source / freshness / overgeneralization / sample check was *not attempted*, not *passed*. No recorded refutation move → `unverified`, never `verified`. |

## Halt Conditions

Stop and surface rather than proceeding when:

- **The question is vague** — refuse and sharpen it (the decision it informs, the falsifiable claims) before sweeping (Guardrail 2).
- **You can neither verify nor refute a material finding** — keep it only labeled *unverified*; never promote it to established.
- **The completeness pass surfaces a gap that changes the recommendation** — report it; let the human decide whether to re-sweep (no auto-loop).
- **The sweep needs >3 angles / broad parallelism** — note that an inline sweep is too narrow. The Workflow engine remains deferred, so surface the limit rather than running an under-powered sweep silently.
- **The operator asks to "turn this into a workstream"** — research never *launches* one (Guardrail 5). Surface that launching a `/workstream` is the operator's call. A workstream can *gate* research, e.g. an `outcome-alignment` checkpoint after synthesis.

## State

Per-run sweep notes and the in-progress finding ledger live in `state/` (gitignored). The durable output is the findings artifact in the DRI's designs repo (`designs/<arc>/research/<slug>.md`).

## References

- `references/method.md` — the four stages in depth: sweep angles, the refutation-pass protocol (the assigned-dissent primitive adapted to findings), the completeness checklist, the artifact template.

## What this skill defers

- The Workflow engine for broad parallel sweeps (ships inline; *deferred — when an inline sweep is observably too narrow, i.e. >3 angles needed repeatedly*).
- An auto-looping completeness critic (ships one reporting pass; *deferred — when verified-but-incomplete artifacts cause a real re-research*).
- A research result cache / `research://` registry (*deferred — when ≥3 workstreams re-read the same artifacts*).
