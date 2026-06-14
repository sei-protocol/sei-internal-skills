---
name: cross-review
category: workflow
model: claude-opus-4-8
description: "Use when an orchestrator has produced or gathered engineering work — a design, plan, diff, or set of specialist outputs — and wants the relevant specialists to INDEPENDENTLY review it for consistency, gaps, and interface mismatches — 'cross-review this', 'cross review', 'have the experts cross-review this', 'check this design for consistency across components', 'review these specialist outputs against each other', '/cross-review'. The review counterpart to producing work with /coral; /coral offers it at synthesis and /council invokes it as its review phase. Anti-triggers: NOT for producing or iterating work with experts (use /coral); NOT for full-ceremony multi-component design (use /council — it dispatches this as its review phase); NOT for adversarial pre-launch hardening (use /bugbash); NOT for line-level diff correctness (use /code-review); NOT for capturing a finished design (use /design); NOT for incident investigation (use /root-cause)."
---

# Cross-Review

Independent multi-specialist review of a produced artifact. The orchestrator has work in hand — a design, a plan, a diff, a set of expert outputs — and needs the relevant specialists to review it *independently*, then a synthesized COMPATIBLE / MISMATCH / MISSING findings table that surfaces the seams.

This is the cross-review action between the orchestrator (root agent) and the coral/council experts. It is **distinct from the per-specialist dispatches that produced the work**: those built the parts; cross-review checks the integrated whole, especially the boundaries where one specialist's output is another's input.

This skill exists because **review collapses into rubber-stamp under pressure**. Under time, sunk cost, a confident senior voice, or "they already weighed in," the natural path is: trust the prior reads, declare it consistent, synthesize a green light from agreement nobody independently gave. That path is fast. It is also how interface mismatches reach integration — or production.

The skill refuses that path. It enforces independent (blinded) review, evidence-bearing findings, an assigned dissenter, and a structured table that resolves before it passes.

## Guardrails

Cross-review operates on **a concrete artifact, reviewed by independent specialists**. Before any verdict:

1. **Artifact required.** Read the actual work under review — the design doc, the spec, the diff, the specialist outputs. If it can't be located or pasted, halt and ask for it. Never review from a summary, from memory, or from "what a spec like this usually contains." A synthesized verdict over an artifact you never read is fabrication.
2. **Roster required.** Cross-review selects its reviewer slate from a `.claude/agents/` roster, so the calling repo must have one. Without it, halt and ask the user to point at a roster or invoke from a repo that has it. The slate is usually several specialists; when exactly one is genuinely relevant, run a single-reviewer pass but label it as such — a degenerate cross-review, not dressed up as a full one.
3. **Refusal conditions** — this skill will refuse to:
   - **Equate prior per-specialist dispatch with cross-review.** "The specialists already gave input during design" describes the *production* of the work, not a review of the integrated whole. The seams between their contributions are exactly what no one has reviewed. Re-dispatch them against the final, combined artifact.
   - **Accept convergence as corroboration when reviewers were not blinded.** If reviewers saw each other's assessments before committing (a shared thread, a summarized peer view in the brief), their agreement is anchoring, not independent confirmation — consensus theater. Re-run with independent briefs.
   - **Accept bare approval.** "LGTM" / "looks good" is not a finding. Every COMPATIBLE, MISMATCH, or MISSING must cite the specific contract, field, signature, or line it is about. A finding with no evidence is noise.
   - **Launder a sign-off through wording.** Phrasing it "their input was incorporated" instead of "they approved" does not convert production into review. If the specialists did not review the final artifact, the cross-review did not happen.
   - **Declare COMPATIBLE while MISMATCH or MISSING findings are open.** Open findings are resolved (artifact updated, provider/consumer reconciled) or explicitly accepted by the user with the risk named — never silently dropped.

See `references/reviewer-dispatch.md` for the blinded dispatch contract and `references/findings-protocol.md` for the findings schema, mismatch categories, and the research grounding.

## The Four Rules

Non-negotiable. Every step exists to enforce one or more.

1. **Read the artifact, review the whole.** You review what's actually written, and you review the *integrated* artifact — including the parts each specialist didn't author. The boundaries are the point.
2. **Independent before synthesized.** Each reviewer commits findings before seeing peers'. Convergence only counts as corroboration if it was reached independently.
3. **Findings carry evidence.** Every finding names the specific contract / field / signature / line. Provider owns the interface; consumers adapt — that's the tie-break when reviewers disagree.
4. **Resolve before pass — the raiser confirms.** MISMATCH and MISSING block a COMPATIBLE verdict until fixed or explicitly accepted. A finding addressed by an artifact change closes only when its **raiser re-reviews and confirms** (re-dispatch them) — never on orchestrator judgment. A clean table with open findings, or one self-cleared by the orchestrator, is a lie.

## Procedure

The orchestrator runs the loop. Specialists do the domain review. Both are bound by the Four Rules.

### Step 1 — Frame the target

State, in the first turn:

- **What is under review** — the artifact(s), by path or pasted content. Read them now.
- **The boundaries at stake** — the interfaces/contracts where components meet (a provider produces, a consumer adapts). Cross-review's value concentrates here.
- **Provider and consumer per boundary** — name them. This sets who owns each interface and who must adapt.
- **What "done" looks like** — a resolved findings table, or a labeled punt with the open items.

If the artifact can't be read, halt (Guardrail #1).

### Step 2 — Pick the reviewer slate

Read `.claude/agents/` from the calling repo. Choose the smallest set whose combined domains cover the boundaries. For each interface, you want at least the provider-domain specialist and the consumer-domain specialist. Add a domain specialist for any cross-cutting concern the artifact raises (security boundary → `security-specialist`; capacity/cost → `k8s-capacity-management`; etc.).

**When the artifact includes a code diff or implementation** (not solely a design doc), add `idiomatic-reviewer` to the slate. It brings a distinct axis the boundary reviewers don't cover — does the code read *native* to its language, framework, and the package's own documented patterns — and it reports separately from the boundary table (see Step 4). Skip it when the artifact under review is a pure design/spec with no code.

**When the artifact carries prose a reader acts on** — runbooks, dual-audience design docs, dense load-bearing comments — add `prose-steward` to the slate. It brings the prose axis the boundary reviewers don't cover — does the prose read correctly for both the human reviewer and the consuming agent — and it rides in a Prose addendum alongside Idiom (see Step 4). Skip it when the artifact carries no prose a reader acts on.

**The invariant:** the slate must cover *every axis the artifact has* — boundaries + idiom (if code) + prose (if prose). A domain-only slate over an artifact that carries code or prose is incomplete.

If only one specialist is genuinely relevant, this is a single-reviewer pass — run it, but label the output accordingly. Don't manufacture reviewers to look thorough.

### Step 3 — Dispatch independent reviews (blinded)

Dispatch contract (mandatory — see `references/reviewer-dispatch.md` for the brief template):

- **Independent.** Each specialist reviews the same artifact without seeing peers' reviews. Do not summarize one reviewer's view into another's brief.
- **Assigned dissent.** Tag one reviewer red-team: their job is to argue the design is wrong and produce the strongest objection. Without an assigned dissenter you get consensus theater.
- **Structured brief.** Ask each reviewer: "Review this artifact for the boundaries you own or consume. For each, return COMPATIBLE / MISMATCH / MISSING with the specific contract/field/line as evidence. Name anything the design assumes but doesn't state." Not "take a look."
- **Evidence required.** Reject bare approval in the returned findings; re-dispatch if a reviewer returns "looks good" with nothing cited.

### Step 4 — Synthesize the findings table

Merge the independent reviews into one de-duplicated table:

| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|

- **Status** is COMPATIBLE / MISMATCH / MISSING (see `references/findings-protocol.md` for mismatch categories: signature, type, error-contract, naming, sequencing/behavioral, doc-divergence — the last always blocking, source of truth wins).
- **Surface disagreement — don't smooth it.** If two reviewers reached opposite conclusions on the same boundary, that's a finding, not a rounding error. Record both and reason from first principles; provider-owns-the-interface is the tie-break, not seniority or recency.
- **Convergence is corroboration only if independent.** If the reviews agree and were blinded, say the confidence is high. If they weren't blinded, downgrade and note it.

**Idiom and prose findings ride in addenda, not the boundary table.** `idiomatic-reviewer` reports two-altitude findings (design + surgical) keyed to files and packages, not to interface boundaries, so they don't fit the COMPATIBLE / MISMATCH / MISSING schema. Record them in a separate **Idiom** section below the table, each carrying its cited basis and severity (correctness-grade / idiom-divergence-with-consequence / style). `prose-steward` findings (dual-audience legibility) ride the same way in a **Prose** section, each with its cited basis and severity.

### Step 5 — Resolve and report

- Every **MISMATCH** and **MISSING** is resolved (artifact updated; provider/consumer reconciled — provider definition wins, consumer adapts) or **explicitly accepted by the user** with the risk stated. Nothing is silently dropped.
- **The raiser confirms a fix closes their finding.** A finding marked resolved by an artifact change re-dispatches to its raiser, who re-reviews and confirms — the orchestrator does not self-clear a peer's finding. (User acceptance-with-risk is the only path that doesn't require the raiser.)
- **Correctness-grade idiom findings block too.** A runtime-consequence idiom finding (e.g. a status patch missing the optimistic lock, an always-present condition removed) is resolved or explicitly accepted before a COMPATIBLE verdict — the same bar as a MISMATCH. Pure-style idiom findings are **advisory**: surfaced in the Idiom addendum, never gating.
- Output: the findings table, the verdict (COMPATIBLE overall / OPEN with N findings), the resolved items with what changed, and any accepted-with-risk items.
- If cross-review can't reach a clean verdict — reviewers split, an artifact gap nobody can close — say so explicitly: "cross-review open, 2 MISMATCH unresolved, needs a provider decision on X." A labeled open state beats a fabricated COMPATIBLE.

## Rationalization Table

Documented failure modes during cross-review. When your own reasoning aligns with the left column, **stop**. The right column is the reframe. (Citations in `references/findings-protocol.md`.)

| Excuse | Reality |
|--------|---------|
| "The staff engineer already read both specs separately — I can just confirm they're consistent." | Separate single-artifact reads do not cover the boundary between them. The seam is the one thing prior reads can't have checked, and it's where mismatches live. Review the integrated whole. |
| "The specialists already gave input during the design, so cross-review is just writing up that they agree." | Per-component input incorporated serially is *production*, not review. Each specialist reacted to their slice against a moving target; none reacted to the final combined artifact. Re-dispatch them against the whole. |
| "They helped build the design, so they obviously agree with the final version." | Contribution is not endorsement. People routinely disagree with how their piece landed once they see the integration. Authorship of a part ≠ sign-off on the whole. |
| "All three experts converged, so it's COMPATIBLE." | If two read the first's comment before agreeing, that's anchoring, not corroboration (Asch). Convergence counts only when reviewers committed independently. Re-run blinded. |
| "It's just synthesis — I'm not the one endorsing the design." | Writing the COMPATIBLE verdict *is* the endorsement. The table carries your authority once it's filed. Own it or don't write it. |
| "I can phrase it 'their input was incorporated' rather than 'approved' — technically true, and it ships." | That launders a sign-off that never happened. The next reader sees "cross-reviewed." If the specialists didn't review the final artifact, say so plainly. |
| "The demo's in an hour — a quick 'looks consistent' is the helpful move." | Speed is a reason to be efficient, not to skip the check or fake the result. A 10-minute boundary review is cheap insurance; a fabricated green light just moves the failure to the demo. |
| "I don't have the doc, but I can write a plausible findings table from what such designs usually contain." | That's fabrication, not review. No artifact, no cross-review — halt and get it. |
| "'Looks good' from two senior people is enough." | Absence of objection is not presence of review. A finding with no cited contract is not a finding. Require evidence. |

## Red Flags — STOP and Reset

Phrases that signal a rationalization is firing — in your reasoning or the user's framing:

- "They already reviewed it" / "they already weighed in" / "they already agree"
- "Just confirm it's consistent" / "just write up that they agree"
- "All three converged" / "everyone signed off" / "the team agrees"
- "LGTM" / "looks good" — offered *as* a finding
- "Quick cross-review" / "we're behind, keep it fast"
- "Their input was incorporated" — standing in for "they reviewed the final"
- "It's just synthesis" / "I'm only writing it up"
- "I'll review from the summary" / "I don't need the actual doc"

**All of these mean: read the artifact, dispatch independent reviewers, require evidence per finding, or label the verdict honestly.**

## Halt Conditions

Stop and report to the user if:

- The artifact under review can't be located or pasted — never synthesize a review of work you haven't read.
- The calling repo has no `.claude/agents/` roster and the user can't point at one.
- A reviewer returns bare approval with no cited evidence — re-dispatch with the evidence requirement.
- Reviewers were not blinded (saw each other's assessments first) — the convergence is invalid; re-run with independent briefs.
- Reviewers split on a boundary and the provider-owns tie-break doesn't resolve it — surface the disagreement and ask the user / provider for the call.
- MISMATCH or MISSING findings remain open and the user has not explicitly accepted the risk — do not stamp COMPATIBLE.
- A finding is marked resolved without its raiser re-reviewing — re-dispatch them; self-clearing a peer's finding is the rubber-stamp this skill exists to prevent.

**Never declare COMPATIBLE to be helpful.** An honest OPEN verdict with named findings is the valuable output; a premature green light is the failure this skill exists to prevent.

## How this fits with coral and council

- **`/coral`** produces work with specialists, then *offers* `/cross-review` at synthesis when outputs touch a shared boundary. Coral builds; cross-review checks.
- **`/council`** runs cross-review as a distinct phase of its scope-tier process by invoking this skill — it does not perform cross-review itself.
- **`/code-review`** is line-level diff correctness; **`/bugbash`** is adversarial hardening of a running system; **`/root-cause`** is incident investigation. Cross-review is consistency review of a produced artifact across the specialists who own its boundaries.
- **`idiomatic-reviewer`** (the `/idiomatic` skill) is the **idiom-conformance** lens — does the code read native to its language, framework, and the package's documented patterns. It's a distinct axis from boundary consistency: cross-review dispatches it as part of the slate when code is under review, and its findings ride in the Idiom addendum (correctness-grade blocks; style is advisory). It reviews idiom; it does not author the system or check boundaries.
- **`prose-steward`** (the `/lingua` skill) is the **dual-audience prose** lens — does prose a reader acts on (runbooks, dual-audience docs, dense comments) read correctly for both the human reviewer and the consuming agent. A distinct axis from boundaries and idiom: cross-review dispatches it when the artifact carries such prose, and its findings ride in the Prose addendum. Read-only (no Bash) — materialize the artifact to disk before dispatch (see `references/reviewer-dispatch.md`). It reviews prose; it does not author the artifact.

## Output

End-of-session summary: the artifact reviewed, the reviewer slate (and who held dissent), the findings table verdict (COMPATIBLE / OPEN with N findings), what was resolved and how (and confirmed by its raiser), and any accepted-with-risk items. If code was reviewed, include the Idiom addendum (with any blocking correctness-grade idiom findings called out); if prose was reviewed, include the Prose addendum. If open, name the unresolved findings and what would close them.
