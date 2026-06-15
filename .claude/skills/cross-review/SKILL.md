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
   - **Declare COMPATIBLE / stamp a passing `State:` while *any* correctness-grade finding is open** — MISMATCH/MISSING, correctness-grade idiom *or* prose, or a per-lens DISSENT (incl. a pinned steward). All are resolved (artifact updated, provider/consumer reconciled) or explicitly accepted-with-risk by the user — never silently dropped. (The full gating set; see Rule 4 / Step 5.)
   - **Drop a pinned steward, or proceed without one.** A `skill-package` change pins `audit-skill` + `author-skill` + `prose-steward` *unconditionally* (per `references/slate-routing.md` — by file-type-present, never by change-size). If a pinned steward is absent from the `.claude/agents/` roster, **HALT** — never silently proceed pin-less; the operator may override only with a stated reason. (Also a Halt Condition.)

See `references/reviewer-dispatch.md` for the blinded dispatch contract, `references/findings-protocol.md` for the findings schema, `references/slate-routing.md` for the change-type → slate routing rule (shared with `/coral`), and `references/review-ledger.md` for the durable synthesis record.

## §0 — Classify before dispatch (HALT gate)

**This is the load-bearing precondition, and it is not a competing "first step" — it is the
*output* of the first-turn framing in Step 1.** The order is fixed and singular:

> **read + frame + classify (Step 1) → HALT if `Class:` absent → dispatch (Step 3).**

You classify **from** the read artifact: Step 1's read-and-frame (artifact, boundaries,
provider/consumer per boundary) produces exactly the inputs classification needs, so the `Class:`
emission lands **as part of / immediately after** that first-turn framing — never before reading
(you cannot classify an artifact you haven't read) and never skipped in favor of framing alone.
There are not two first steps; there is one first turn whose output **must include** the
classification.

As that first-turn output, and **before dispatching any reviewer**, emit:

- `Class:` — one of the six (`doc-only | mechanical | component | cross-component | shared-stack | skill-package`, per `references/slate-routing.md`; authoritative list: `references/slate-routing.md` §1),
- the resulting `Tier:` (T1/T2/T3, read off the routing table — never re-derived by hand), and
- the assembled **slate** (domain lenses + auto-wired stewards + the assigned dissenter).

**If `Class:` is absent or unresolvable, HALT and do not dispatch.** This is the same posture as
Guardrail #1 ("no artifact ⇒ halt"): **no classification ⇒ no review.** Everything mechanical —
tier, slate, the steward pin, the dissenter floor — is a *function of* a populated `Class:`; the
HALT is the gate **before Step 3 dispatch** — it is what forces the classification to happen at
all. An operator override (naming a slate or tier directly) still satisfies the precondition — it
produces a recorded `Class:`/`Tier:` — it does not bypass it.

The emitted classification is the ledger's header (`references/review-ledger.md`). A
cross-review that dispatches reviewers without an emitted `Class:` is **non-compliant**.

## The Four Rules

Non-negotiable. Every step exists to enforce one or more.

1. **Read the artifact, review the whole.** You review what's actually written, and you review the *integrated* artifact — including the parts each specialist didn't author. The boundaries are the point.
2. **Independent before synthesized.** Each reviewer commits findings before seeing peers'. Convergence only counts as corroboration if it was reached independently.
3. **Findings carry evidence.** Every finding names the specific contract / field / signature / line. Provider owns the interface; consumers adapt — that's the tie-break when reviewers disagree.
4. **Resolve before pass.** A passing verdict requires *every* lens's correctness-grade findings closed — MISMATCH/MISSING, correctness-grade idiom *and* prose findings, and any per-lens DISSENT (incl. pinned stewards) — fixed or explicitly accepted-with-risk (see Step 5). A clean table with open findings is a lie.

## Procedure

The orchestrator runs the loop. Specialists do the domain review. Both are bound by the Four Rules.

### Step 1 — Read, frame, and classify the target (one first turn)

This is the single first turn. Its output **includes the §0 classification** — read and frame
first, then classify *from* what you read; do not split these into separate steps or dispatch
between them. State, in this first turn:

- **What is under review** — the artifact(s), by path or pasted content. Read them now.
- **The boundaries at stake** — the interfaces/contracts where components meet (a provider produces, a consumer adapts). Cross-review's value concentrates here.
- **Provider and consumer per boundary** — name them. This sets who owns each interface and who must adapt.
- **The classification (§0)** — emit `Class:`/`Tier:`/slate, derived *from* the artifact you just read and the boundaries you just framed. This is the HALT gate before any dispatch (Step 3).
- **What "done" looks like** — a **committed review ledger** (`references/review-ledger.md`) whose latest-round `State:` is a passing terminal (`RESOLVED`/`RESOLVED-WITH-ACCEPTED-RISK`, `OpenFindings: 0`) or the `OPEN-BLOCKED` fail-to-human terminal — not merely an in-conversation findings table.

If the artifact can't be read, halt (Guardrail #1) — you cannot classify or review what you
haven't read. If the read-and-frame yields no resolvable `Class:`, halt (§0) before dispatching.

### Step 2 — Route the slate (per `references/slate-routing.md`)

The slate is **routed, not re-derived by hand.** Apply the shared routing table
(`references/slate-routing.md` — the one mechanism, also cited by `/coral`):

1. **Classify** the artifact into one of the six classes (already emitted in Step 1 per §0 — this step reuses that `Class:`, it does not re-classify).
2. **Read the tier off the table** (T1/T2/T3) — class sets the default; blast-radius bumps it
   up, never silently down. `shared-stack`/`skill-package` are T3 by default and **cannot drop
   below T2**.
3. **Assemble the slate:** read `.claude/agents/` and pick the domain lenses whose combined
   domains cover the boundaries (provider + consumer per interface, plus any cross-cutting
   specialist — security boundary → `security-specialist`; capacity/cost →
   `k8s-capacity-management`; etc.). The orchestrator's remaining judgment is *which domain
   specialists* cover the boundaries; the depth and steward wiring are mechanical.
4. **Auto-wire the stewards** by file-type-present (table §4): `prose-steward` on any prose;
   `idiomatic-reviewer` on any code diff; `audit-skill`+`author-skill` on a `.claude/` skill
   body. **A `skill-package` change pins `audit-skill` + `author-skill` + `prose-steward`
   unconditionally** — dropping any of the three requires an operator override with a stated
   reason.
5. **Assign the dissenter** (see The Four Rules / Step 3) and record it.

The stewards report on their own axes (Idiom addendum / Prose addendum / per-lens RATIFY-DISSENT
verdicts in the ledger), not the boundary table — see Step 4 and `references/review-ledger.md`.

If only one specialist is genuinely relevant (a T1 `mechanical` pass), this is a single-reviewer
pass — run it, but label the output accordingly, and **fold the dissent obligation into the one
reviewer** (an adversarial pass; recorded as `Dissenter: <lens> (self, single-reviewer pass)`).
Don't manufacture reviewers to look thorough; don't waive the dissent because the slate is one.

### Step 3 — Dispatch independent reviews (blinded)

Dispatch contract (mandatory — see `references/reviewer-dispatch.md` for the brief template):

- **Independent.** Each specialist reviews the same artifact without seeing peers' reviews. Do not summarize one reviewer's view into another's brief.
- **Assigned dissent (default, not droppable).** Tag one reviewer red-team: their job is to argue the design is wrong and produce the strongest objection — picked as the lens *most likely to find the breaking boundary*, not the least busy. This is the **floor**, not an opt-in: the ledger's `Dissenter:` field is **required and never empty**, and a `Convergence: unanimous` verdict is only valid if a dissenter was assigned and still concluded RATIFY (unanimity without an assigned dissenter is consensus theater). A **T1 single-reviewer pass folds** the dissent into the one reviewer (an adversarial pass), recorded as `Dissenter: <lens> (self, single-reviewer pass)` — never waived.
- **Structured brief.** Ask each reviewer: "Review this artifact for the boundaries you own or consume. For each, return COMPATIBLE / MISMATCH / MISSING with the specific contract/field/line as evidence. Name anything the design assumes but doesn't state." Not "take a look."
- **Evidence required.** Reject bare approval in the returned findings; re-dispatch if a reviewer returns "looks good" with nothing cited.

### Step 4 — Synthesize into the review ledger

Write the durable synthesis record per `references/review-ledger.md` — the committed, target-
derivable ledger at `<artifact-dir>/cross-review/<target-slug>.md` (or `.cross-review/<slug>.md`
at repo root when the target has no natural directory). It carries the typed header — target-scoped
`Class:`/`Tier:` once at top, and the per-round `State:`/`OpenFindings:`/`Convergence:`/`Blinded:`/`Dissenter:`
(one-per-line, exact-token). **PLT-536's review-gate reads the latest round's five
`State:`/`OpenFindings:`/`Convergence:`/`Blinded:`/`Dissenter:` lines** (not `Class:`/`Tier:`) — per the
gate-read contract in `references/review-ledger.md`. The ledger also carries the per-lens RATIFY/DISSENT verdicts, the
boundary table below, the Idiom/Prose addenda, and the **Rejected findings** table (Rule 4 made
auditable — a finding the orchestrator rejected, who raised it, and *how the rejection was
verified*). A re-review **appends a new `## Round N`** — never edits a prior round in place.

Merge the independent reviews into one de-duplicated boundary table inside the ledger:

| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|

- **Status** is COMPATIBLE / MISMATCH / MISSING (see `references/findings-protocol.md` for mismatch categories: signature, type, error-contract, naming, sequencing/behavioral).
- **Surface disagreement — don't smooth it.** If two reviewers reached opposite conclusions on the same boundary, that's a finding, not a rounding error. Record both and reason from first principles; provider-owns-the-interface is the tie-break, not seniority or recency.
- **Convergence is corroboration only if independent.** If the reviews agree and were blinded, say the confidence is high. If they weren't blinded, downgrade and note it.

**Idiom and Prose findings ride in addenda, not the boundary table.** `idiomatic-reviewer` reports two-altitude idiom findings (design + surgical) keyed to files/packages; `prose-steward` reports dual-audience legibility findings (R1–R5) keyed to passages. Neither fits the COMPATIBLE / MISMATCH / MISSING boundary schema. Record idiom findings in a separate **Idiom addendum** and prose findings in a separate **Prose addendum** below the table (both defined in `references/review-ledger.md`), each carrying its cited basis and severity (correctness-grade / divergence-with-consequence / style). **Correctness-grade findings in either addendum gate the verdict per Step 5; pure-style ones are advisory.**

### Step 5 — Resolve and report

- Every **MISMATCH** and **MISSING** is resolved (artifact updated; provider/consumer reconciled — provider definition wins, consumer adapts) or **explicitly accepted by the user** with the risk stated. Nothing is silently dropped.
- **Correctness-grade idiom findings block too.** A runtime-consequence idiom finding (e.g. a status patch missing the optimistic lock, an always-present condition removed) is resolved or explicitly accepted before a COMPATIBLE verdict — the same bar as a MISMATCH. Pure-style idiom findings are **advisory**: surfaced in the Idiom addendum, never gating.
- **Per-lens DISSENT and correctness-grade prose findings block too.** `RESOLVED` means *every* lens's correctness-grade findings are closed — not just the boundary table. Any unresolved per-lens `DISSENT` (including from a pinned steward — `audit-skill`/`author-skill`/`prose-steward`) and any correctness-grade prose-addendum finding (a misleading or ambiguous load-bearing instruction, not pure style) must be resolved or explicitly accepted-with-risk before `RESOLVED`, the same bar as a MISMATCH. Pure-style prose findings are advisory.
- Output: the committed ledger with its typed header `State:`, the verdict, the resolved items with what changed, and any accepted-with-risk items. Set `State:` per the enum in `references/review-ledger.md` — `RESOLVED` / `RESOLVED-WITH-ACCEPTED-RISK` are the only passing terminals; `OpenFindings:` is `0` for those.
- If cross-review can't reach a clean verdict — reviewers split, an artifact gap nobody can close — say so explicitly and set `State: OPEN-BLOCKED` with `OpenFindings: ≥1`: it **fails the gate to a human**. A split must **never** be relabeled `RESOLVED-WITH-ACCEPTED-RISK` to make the loop terminate (accepted-risk needs an operator decision on a *named* risk, not mere disagreement). A labeled open state beats a fabricated COMPATIBLE.

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
| "The skill change is a one-line typo / obviously small — it's mechanical, skip the steward pin." | A `skill-package` change routes by **file-type-present, not change-size** (`references/slate-routing.md` §3a). The mechanical-equivalent carve-out is `doc-only` only. A one-line typo in a `.claude/` skill body is still `skill-package` (T3, floor T2, audit+author+prose pinned). "It's just a typo in a skill" does not demote it to `mechanical`; dropping the pin needs an operator override with a stated reason, never a size judgment. |

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
- "It's just a typo in the skill" / "we've done dozens of these" — offered to demote a `skill-package` change to `mechanical` and drop the steward pin

**All of these mean: read the artifact, dispatch independent reviewers, require evidence per finding, or label the verdict honestly.**

## Halt Conditions

Stop and report to the user if:

- `Class:` was not emitted before dispatch (§0) — no classification ⇒ no review. HALT and classify before dispatching any reviewer.
- The artifact under review can't be located or pasted — never synthesize a review of work you haven't read.
- The calling repo has no `.claude/agents/` roster and the user can't point at one.
- A **pinned** steward (audit/author/prose on a `skill-package` change) is absent from the calling repo's `.claude/agents/` roster — HALT, not a silent drop (same posture as dropping the pin). Ask the operator, who may override with a stated reason.
- A reviewer returns bare approval with no cited evidence — re-dispatch with the evidence requirement.
- Reviewers were not blinded (saw each other's assessments first) — the convergence is invalid; re-run with independent briefs.
- Reviewers split on a boundary and the provider-owns tie-break doesn't resolve it — surface the disagreement and ask the user / provider for the call.
- *Any* correctness-grade finding remains open — MISMATCH/MISSING, correctness-grade idiom/prose, or a per-lens DISSENT (incl. a pinned steward) — and the user has not explicitly accepted the risk. Do not stamp a passing ledger `State:` (`RESOLVED`/`RESOLVED-WITH-ACCEPTED-RISK`); set `OPEN` or `OPEN-BLOCKED` (see Rule 4 / Step 5).

**Never declare COMPATIBLE to be helpful.** An honest OPEN verdict with named findings is the valuable output; a premature green light is the failure this skill exists to prevent.

## How this fits with coral and council

- **`/coral`** produces work with specialists, then *offers* `/cross-review` at synthesis when outputs touch a shared boundary. Coral builds; cross-review checks.
- **`/council`** runs cross-review as a distinct phase of its scope-tier process by invoking this skill — it does not perform cross-review itself.
- **`/code-review`** is line-level diff correctness; **`/bugbash`** is adversarial hardening of a running system; **`/root-cause`** is incident investigation. Cross-review is consistency review of a produced artifact across the specialists who own its boundaries.
- **`idiomatic-reviewer`** (the `/idiomatic` skill) is the **idiom-conformance** lens — does the code read native to its language, framework, and the package's documented patterns. It's a distinct axis from boundary consistency: cross-review dispatches it as part of the slate when code is under review, and its findings ride in the Idiom addendum (correctness-grade blocks; style is advisory). It reviews idiom; it does not author the system or check boundaries.

## Output

End-of-session summary: the emitted `Class:`/`Tier:`/slate (§0), the committed ledger path, the reviewer slate (and who held the required dissent), the verdict with its `State:` token, what was resolved and how, any accepted-with-risk items, and any rejected findings with how the rejection was verified. If code was reviewed, include the Idiom addendum; if prose, the Prose addendum (correctness-grade blocks, style advisory). If open, name the unresolved findings and what would close them — and if a split could not be resolved, `State: OPEN-BLOCKED` to a human.
