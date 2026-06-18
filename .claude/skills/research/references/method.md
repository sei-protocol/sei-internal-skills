# The research method in depth

The four stages from SKILL.md, with the protocols each needs. The discipline is in the verify gate and the completeness pass; the sweep is just fuel.

## Stage 1 — Scope

A useful scoped question names three things:

- **The decision it informs** — "should we adopt X?", "which of A/B/C fits?", "is claim C true?". Research with no decision behind it is a literature dump.
- **The falsifiable claims sought** — what statements would the answer assert, that could be shown false? ("X supports streaming" is falsifiable; "X is good" is not.)
- **The scope boundary** — what's in and out, so the completeness pass has something to check coverage against.

If the operator's question lacks these, sharpen it with them first (Guardrail 2). Write the scoped question verbatim at the top of the artifact — it is the contract.

## Stage 2 — Multi-modal sweep

Run searches whose angles are *blind to each other* so no single framing's blind spot sinks the effort. The four canonical angles and what each is for:

| Angle | What it surfaces | Guards against |
|---|---|---|
| by-source | official docs, specs, primary sources | hearsay / second-hand summaries |
| by-entity | the named tools/projects/people/prior-art | missing the obvious incumbent |
| by-time | what changed in the last 12–24 months | stale conclusions from old material |
| by-counter-thesis | sources arguing the *opposite* of the expected answer | confirmation bias / sample-of-one |

Pick the angles the question needs (not all four mechanically). Inline for ≤3 angles. For each finding record **which angle surfaced it** and **a retrievable source** — a finding with no source is not a finding, it's a recollection.

## Stage 3 — The refutation pass (the differentiator)

This adapts `/xreview`'s **assigned-dissent primitive** — "tag exactly one reviewer to red-team and produce the strongest objection" — to findings. It does **not** use xreview's boundary table (provider/consumer/COMPATIBLE/MISMATCH/MISSING), which doesn't map onto a finding.

Per material finding, take the skeptic stance and try to break it:

- **Find the contradicting source** (the by-counter-thesis angle feeds this).
- **Check the citation's freshness** — is it superseded?
- **Test for overgeneralization** — does the source actually claim what the finding says, or a narrower thing?
- **Test the sample size** — is this one anecdote dressed as a pattern?

Outcome, recorded per finding:

- **verified** — the refutation pass *ran* and failed to break the finding. Record (a) which refutation move was tried (contradicting-source / freshness / overgeneralization / sample-size) and (b) the specific source or reasoning that defeated it. **A refutation that was not attempted does not yield `verified` — it yields `unverified`.** "I didn't find a contradiction" with no recorded search is `unverified`, not `verified`.
- **refuted** — dropped; note why once, so a later sweep round doesn't resurrect it.
- **unverified** — neither confirmed nor refuted; kept only with the `[unverified]` label, never presented as established.

The bar (Guardrail 1): **no finding ships unverified.** "Unverified" is an allowed *label*, not an allowed *silent state*.

## Stage 4 — Completeness pass + synthesize

One pass, four questions against the scoped contract:

1. **Modality** — was an angle the question needed not run?
2. **Verification** — is a load-bearing finding still unverified?
3. **Sources** — is a named primary source unread?
4. **Scope** — is part of the scoped question unanswered?

**Report** the gaps in the artifact's Completeness assessment. If a gap would change the recommendation, surface it and let the human decide whether to re-sweep — **do not auto-loop** (MVP). Then synthesize the recommendation, grounding it **only in verified findings** (unverified findings inform open questions, not the recommendation).

## The artifact template

See SKILL.md "The artifact" for the full template. Two rules that are easy to get wrong:

- **Tag every finding** `[verified]` / `[unverified]` (refuted ones are dropped with a one-line note). An untagged finding reads as established — that's the failure mode.
- **Lineage threads through the issue, not the research doc's URL.** `betGraph.designLinked` keys on the *bet's design* URL; a research doc that advances a bet rides on the issue already carrying the label + design link, as an additional reference. Don't expect the research-doc URL to be a plan discriminator (that's a deferred `/execution-plan` contract question).
