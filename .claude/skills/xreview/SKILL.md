---
name: xreview
category: workflow
model: claude-opus-5
description: "Use when an orchestrator has produced or gathered engineering work — a design, plan, diff, or set of specialist outputs — and wants the relevant specialists to INDEPENDENTLY review it for consistency, gaps, and interface mismatches — 'xreview this', 'cross review', 'have the experts xreview this', 'check this design for consistency across components', 'review these specialist outputs against each other', '/xreview'. The review counterpart to producing work with /coral; /coral offers it at synthesis and /council invokes it as its review phase. Anti-triggers: NOT for producing or iterating work with experts (use /coral); NOT for full-ceremony multi-component design (use /council — it dispatches this as its review phase); NOT for adversarial pre-launch hardening (use /bugbash); NOT for line-level diff correctness; NOT for capturing a finished design (use /design); NOT for incident investigation (use /root-cause)."
---

# xreview

Independent multi-specialist review of a produced artifact. The orchestrator has work in hand — a design, a plan, a diff, a set of expert outputs. The relevant specialists must review it *independently*. A synthesized COMPATIBLE / MISMATCH / MISSING findings table then surfaces the seams.

This is the xreview action between the orchestrator (root agent) and the coral/council experts. It is **distinct from the per-specialist dispatches that produced the work**: those built the parts. Xreview checks the integrated whole, especially the boundaries where one specialist's output is another's input.

This skill exists because **review collapses into rubber-stamp under pressure**. Under time, sunk cost, a confident senior voice, or "they already weighed in," the natural path has three moves. Trust the prior reads, declare it consistent, and synthesize a green light from agreement nobody independently gave. That path is fast. It is also how interface mismatches reach integration — or production.

The skill refuses that path. It enforces independent (blinded) review, evidence-bearing findings, an assigned dissenter, and a structured table that resolves before it passes.

## Guardrails

xreview operates on **a concrete artifact, reviewed by independent specialists**. Before any verdict:

1. **Artifact required.** Read the actual work under review — the design doc, the spec, the diff, the specialist outputs. If you cannot locate or paste it, halt and ask for it. Never review from a summary, from memory, or from "what a spec like this typically contains." A synthesized verdict over an artifact you never read is fabrication.
2. **Roster required.** xreview selects its domain lenses and **agent-stewards** (`prose-steward`, `idiomatic-reviewer`) from a `.claude/agents/` roster, so the calling repo must have one. Without it, halt and ask the user to point at a roster or invoke from a repo that has it.

   On a `skill-package` change, brief one more reviewer as the **rubric lens**. It loads **this skill's own rubric**, `references/skill-package-rubric.md` (rules with ids and severities). It runs `scripts/skill-package-checks.sh` for the static subset, and returns findings that **name rule ids**.

   The rubric is a file this skill owns, not a registry entry. The *lens* has no absence check, and an uninstall cannot drop it — which is why it lives here rather than in a separate skill. The rubric *file* is a different object. A broken install can still truncate or omit it, and a lens that cannot read it **HALTs** (Halt Conditions). A rubric-lens verdict citing no rule id is not a rubric review; re-dispatch it.

   The slate normally holds a handful of specialists. When exactly one is genuinely relevant, run a single-reviewer pass but label it as such. Call it a degenerate xreview, not one dressed up as full.
3. **Refusal conditions** — this skill will refuse to:
   - **Equate prior per-specialist dispatch with xreview.** "The specialists already gave input during design" describes the *production* of the work, not a review of the integrated whole. The seams between their contributions are exactly what no one has reviewed. Re-dispatch them against the final, combined artifact.
   - **Accept convergence as corroboration when reviewers were not blinded.** Reviewers may see each other's assessments before committing: a shared thread, a summarized peer view in the brief. Their agreement is then anchoring, not independent confirmation. That is consensus theater. Re-run with independent briefs.
   - **Accept bare approval.** "LGTM" / "looks good" is not a finding. Every COMPATIBLE, MISMATCH, or MISSING must cite the specific contract, field, signature, or line it is about. A finding with no evidence is noise.
   - **Launder a sign-off through wording.** Phrasing it "we incorporated their input" instead of "they approved" does not convert production into review. If the specialists did not review the final artifact, the xreview did not happen.
   - **Declare COMPATIBLE / stamp a passing `State:` while *any* correctness-grade finding is open.** The gating set: a MISMATCH/MISSING, a correctness-grade idiom *or* prose finding, or a per-lens DISSENT (including a pinned steward). Each one resolves (artifact updated, provider/consumer reconciled), or the user marks it accepted-with-risk — never silently dropped. (This is the gating set, stated identically in Rule 4 and Halt Conditions, and enforced bullet-by-bullet in Step 5.)
   - **Drop a pinned steward, or proceed without one.** A `skill-package` change pins `prose-steward` *unconditionally* — **regardless of which file-types the diff touches**. Change-size never demotes it (per `references/slate-routing.md` §4). It also needs one reviewer holding the **rubric lens**, citing rule ids from `references/skill-package-rubric.md`. A verdict that cites no rule id is not a rubric review: presence of a file was never evidence that anyone read it. If `prose-steward` is absent from `.claude/agents/`, **HALT** — never silently proceed pin-less; the operator may override only with a stated reason. (Also a Halt Condition.)

See `references/reviewer-dispatch.md` for the blinded dispatch contract, `references/findings-protocol.md` for the findings schema, `references/slate-routing.md` for the change-type → slate routing rule (shared with `/coral`), and `references/review-ledger.md` for the durable synthesis record.

## §0 — Classify before dispatch (HALT gate)

**This is the load-bearing precondition, not a competing "first step". It is the
*output* of the first-turn framing in Step 1.** The order stays fixed and singular:

> **read + frame + classify (Step 1) → HALT if `Class:` absent → dispatch (Step 3).**

You classify **from** the read artifact. Step 1's read-and-frame (artifact, boundaries,
provider/consumer per boundary) produces exactly the inputs classification needs. The `Class:`
emission therefore lands **as part of / immediately after** that first-turn framing. It never
comes before reading — you cannot classify an artifact you have not read — and it never yields
to framing alone.
The flow has one first turn, not two first steps, and its output **must include** the
classification.

As that first-turn output, and **before dispatching any reviewer**, emit:

- `Class:` — one of the six (`doc-only | mechanical | component | cross-component | shared-stack | skill-package`, per `references/slate-routing.md`; authoritative list: `references/slate-routing.md` §1),
- the resulting `Tier:` (T1/T2/T3, read off the routing table — never re-derived by hand), and
- the assembled **slate** (domain lenses + auto-wired stewards + the assigned dissenter).

**If `Class:` is absent or unresolvable, HALT and do not dispatch.** This is the same posture as
Guardrail #1 ("no artifact ⇒ halt"): **no classification ⇒ no review.** Everything mechanical —
tier, slate, the steward pin, the dissenter floor — is a *function of* a populated `Class:`. The
HALT is the gate **before Step 3 dispatch**, and it is what forces the classification to happen
at all. An operator override (naming a slate or tier directly) still satisfies the precondition — it
produces a recorded `Class:`/`Tier:` — it does not bypass it.

The emitted classification is the ledger's header (`references/review-ledger.md`). A
xreview that dispatches reviewers without an emitted `Class:` is **non-compliant**.

## The Four Rules

Non-negotiable. Every step exists to enforce one or more.

1. **Read the artifact, review the whole.** You review what's actually written, and you review the *integrated* artifact — including the parts each specialist did not author. The boundaries are the point.
2. **Independent before synthesized.** Each reviewer commits findings before seeing peers'. Convergence only counts as corroboration when the reviewers reached it independently.
3. **Findings carry evidence.** Every finding names the specific contract / field / signature / line. Provider owns the interface; consumers adapt — that is the tie-break when reviewers disagree.
4. **Resolve before pass.** A passing verdict requires *every* lens's correctness-grade findings closed. The gating set: a MISMATCH/MISSING, a correctness-grade idiom *or* prose finding, or a per-lens DISSENT (including a pinned steward). Close each one by a fix or an explicit accept-with-risk (see Step 5). A clean table with open findings is a lie.

## Procedure

The orchestrator runs the loop. Specialists do the domain review. The Four Rules bind both.

### Step 1 — Read, frame, and classify the target (one first turn)

This is the single first turn. Its output **includes the §0 classification**. Read and frame
first, then classify *from* what you read. Do not split these into separate steps or dispatch
between them. State, in this first turn:

- **What is under review** — the artifact(s), by path or pasted content. Read them now.
- **The boundaries at stake** — the interfaces/contracts where components meet (a provider produces, a consumer adapts). xreview's value concentrates here.
- **Provider and consumer per boundary** — name them. This sets who owns each interface and who must adapt.
- **The classification (§0)** — emit `Class:`/`Tier:`/slate, derived *from* the artifact you just read and the boundaries you just framed. This is the HALT gate before any dispatch (Step 3).
- **What "done" looks like** — a **committed review ledger** (`references/review-ledger.md`), not merely an in-conversation findings table. Its latest-round `State:` is a passing terminal (`RESOLVED`/`RESOLVED-WITH-ACCEPTED-RISK`, `OpenFindings: 0`) or the `OPEN-BLOCKED` fail-to-human terminal.

If you cannot read the artifact, halt (Guardrail #1) — you cannot classify or review what you
have not read. If the read-and-frame yields no resolvable `Class:`, halt (§0) before dispatching.

### Step 2 — Route the slate (per `references/slate-routing.md`)

**Route the slate — never re-derive it by hand.** Apply the shared routing table
(`references/slate-routing.md` — the one mechanism, also cited by `/coral`):

1. **Classify** the artifact into one of the six classes (already emitted in Step 1 per §0 — this step reuses that `Class:`, it does not re-classify).
2. **Read the tier off the table** (T1/T2/T3) — class sets the default; blast-radius bumps it
   up, never silently down. `shared-stack`/`skill-package` default to T3 and **cannot drop
   below T2**.
3. **Assemble the slate:** read `.claude/agents/` and pick the domain lenses whose combined
   domains cover the boundaries (provider + consumer per interface). Then **wire the mandatory
   concern-lenses mechanically per table §4a**. A change that *touches* the §4a surfaces pins
   `systems-engineer` / `security-specialist`. Read the trigger list off §4a; do not restate it
   here. The orchestrator's remaining judgment is *which domain specialists* cover the boundaries;
   the depth, the §4a concern-lenses, and the steward wiring are mechanical.
4. **Auto-wire the stewards** by file-type-present (table §4) — the rule for `shared-stack` and
   every other class: `prose-steward` on any prose; `idiomatic-reviewer` on any code diff.
   **`skill-package` is the exception: it pins `prose-steward` *unconditionally* — regardless
   of which file-types the diff touches** — and adds the **rubric lens**. Dropping either
   requires an operator override with a stated reason.

   The two are **different kinds of thing** (§4's dispatch table).
   `prose-steward` is an **agent** from `.claude/agents/`, and its absence there is a HALT,
   not a silent drop. The rubric lens is **a brief, not a registry entry** — any dispatched
   reviewer told to load `references/skill-package-rubric.md`, run
   `scripts/skill-package-checks.sh`, and cite rule ids. Do not look for it in
   `.claude/agents/` or `.claude/skills/`; it was never going to be there, and it cannot go
   missing. What replaces its absence check: **a verdict citing no rule id is not a rubric
   review — re-dispatch it.** Two conditions ride with that:
   - **The rubric file itself can still be missing or truncated in a broken install.** If the
     lens cannot read `references/skill-package-rubric.md`, **HALT** — do not let it emit
     plausible-looking ids from memory. The ids are short and schematic (`D1`, `B2`, `S2`), so
     an unread rubric produces a review that looks cited and is not.
   - **The rubric governs at the merge base.** When the diff edits the rubric or the checker,
     the **orchestrator** materializes the merge-base revision of both to disk and briefs those
     paths. Those files are `references/skill-package-rubric.md` and
     `scripts/skill-package-checks.sh`. The lens cites ids from that copy. Never hand it a
     `git show` pointer — Reachability (`references/reviewer-dispatch.md`) applies, and some
     lenses have no Bash.

     The checker is in scope because it decides 26 of the 52 rules. Otherwise a diff that
     loosens a static check would face review under the loosened check. The edit is
     **itself a finding**, recorded in the ledger's routing section with a named justification.
     Until it carries one, treat it as correctness-grade. Without this, a change can weaken a
     rule and then meet review under the weakened rule in the same pass.
5. **Assign the dissenter** (see The Four Rules / Step 3) and record it.

The stewards report on their own axes — the Idiom addendum, the Prose addendum, per-lens
RATIFY-DISSENT verdicts in the ledger — not the boundary table. See Step 4 and
`references/review-ledger.md`.

If only one specialist is genuinely relevant (a T1 `mechanical` pass), this is a single-reviewer
pass. Run it, but label the output accordingly. **Fold the dissent obligation into the one
reviewer** — an adversarial pass, recorded as `Dissenter: <lens> (self, single-reviewer pass)`.
Do not manufacture reviewers to look thorough; do not waive the dissent because the slate is one.

### Step 3 — Dispatch independent reviews (blinded)

Dispatch contract (mandatory — see `references/reviewer-dispatch.md` for the brief template):

- **Independent.** Each specialist reviews the same artifact without seeing peers' reviews. Do not summarize one reviewer's view into another's brief.
- **Assigned dissent (default, not droppable).** Tag one reviewer red-team: their job is to argue the design is wrong and produce the strongest objection. Pick the lens *most likely to find the breaking boundary*, not the least busy. This is the **floor**, not an opt-in: the ledger's `Dissenter:` field **must** carry a lens and never sit empty. A `Convergence: unanimous` verdict only holds when the slate assigned a dissenter and that dissenter still concluded RATIFY; unanimity without one is consensus theater. A **T1 single-reviewer pass folds** the dissent into the one reviewer (an adversarial pass), recorded as `Dissenter: <lens> (self, single-reviewer pass)` — never waived.
- **Structured brief.** Ask each reviewer: "Review this artifact for the boundaries you own or consume. For each, return COMPATIBLE / MISMATCH / MISSING with the specific contract/field/line as evidence. Name anything the design assumes but does not state." Not "take a look."
- **Evidence required.** Reject bare approval in the returned findings; re-dispatch if a reviewer returns "looks good" with nothing cited.
- **Reachable.** Brief each reviewer with on-disk absolute paths or pasted content, never a `gh`/`git`/shell pointer. Some reviewers are Read-only (`prose-steward` has no Bash) and cannot fetch it, so the review halts or fabricates. The orchestrator materializes any remote artifact (a PR diff, a fetched doc) to disk before dispatch. See `references/reviewer-dispatch.md`.

### Step 4 — Synthesize into the review ledger

Write the durable synthesis record per `references/review-ledger.md`. The committed, target-
derivable ledger lives in the **DRI's `<engineer>-designs` repo** at `designs/<arc>/xreview/<target-slug>.md`.
A code-PR/diff target with no artifact arc uses the code repo's **default arc**, e.g. `sei-internal-skills-stack`.
The in-repo `.xreview/` fallback applies only when no DRI repo is resolvable **and the user confirms** (Design 13).
Resolve the DRI repo producer-side as `/design` does, halting on a non-interactive run rather than writing
to a guessed path. (The consumer gate then checks both locations — see `references/review-ledger.md`.)

It carries the typed header. Target-scoped
`Class:`/`Tier:` sit once at top, and the per-round `State:`/`OpenFindings:`/`Convergence:`/`Blinded:`/`Dissenter:`
follow (one-per-line, exact-token). **PLT-536's review-gate reads the latest round's five
`State:`/`OpenFindings:`/`Convergence:`/`Blinded:`/`Dissenter:` lines**, not `Class:`/`Tier:`. That follows the
gate-read contract in `references/review-ledger.md`. The ledger also carries the per-lens RATIFY/DISSENT verdicts, the
boundary table below, the Idiom/Prose addenda, and the **Rejected findings** table. That table makes Rule 4
auditable: a finding the orchestrator rejected, who raised it, and *how the orchestrator verified the
rejection*.

A re-review **appends a new `## Round N`** — never edits a prior round in place.

Merge the independent reviews into one de-duplicated boundary table inside the ledger:

| Interface / Boundary | Provider | Consumer | Status | Evidence | Raised by |
|---|---|---|---|---|---|

- **Status** is COMPATIBLE / MISMATCH / MISSING (see `references/findings-protocol.md` for mismatch categories: signature, type, error-contract, naming, sequencing/behavioral).
- **Surface disagreement — do not smooth it.** If two reviewers reached opposite conclusions on the same boundary, that is a finding, not a rounding error. Record both and reason from first principles; provider-owns-the-interface is the tie-break, not seniority or recency.
- **Convergence is corroboration only if independent.** If the reviews agree under blinded dispatch, say the confidence is high; if the dispatch did not blind them, downgrade and note it.

**Idiom and Prose findings ride in addenda, not the boundary table** — neither fits the COMPATIBLE / MISMATCH / MISSING boundary schema. `idiomatic-reviewer` reports two-altitude idiom findings (design + surgical) keyed to files/packages; `prose-steward` reports dual-audience legibility findings (R1–R6) keyed to passages. Record idiom findings in a separate **Idiom addendum** and prose findings in a separate **Prose addendum** below the table. `references/review-ledger.md` defines both. Each entry carries its cited basis and severity (correctness-grade / divergence-with-consequence / style). **Correctness-grade findings in either addendum gate the verdict per Step 5; pure-style ones are advisory.**

### Step 5 — Resolve and report

- Resolve every **MISMATCH** and **MISSING** (artifact updated; provider/consumer reconciled — provider definition wins, consumer adapts), or the user **explicitly accepts** it with the risk stated. Nothing is silently dropped.
- **Correctness-grade idiom findings block too.** A runtime-consequence idiom finding meets the same bar as a MISMATCH. Examples: a status patch missing the optimistic lock, or an always-present condition removed. Close it or explicitly accept it before a COMPATIBLE verdict. Pure-style idiom findings are **advisory**: they ride in the Idiom addendum, never gating.
- **Per-lens DISSENT and correctness-grade prose findings block too.** `RESOLVED` means *every* lens closed its correctness-grade findings — not just the boundary table. Two carry the same bar as a MISMATCH: any unresolved per-lens `DISSENT` (including from a pinned steward), and any correctness-grade prose-addendum finding. A correctness-grade prose finding is a misleading or ambiguous load-bearing instruction, not pure style. Clear each one finding-by-finding, or accept it with stated risk, before `RESOLVED`. Pure-style prose findings are advisory.
- **A per-lens verdict is the lens's to give — the orchestrator never issues one on its behalf.**
  Fixing what a lens objected to closes the *finding*; it does not convert that lens's `DISSENT`
  into a `RATIFY`. Record the resolution **against the standing DISSENT**. Three exits reach a
  passing `State:`, and only three: **re-dispatch** the lens for an updated verdict.
  **Accept-with-risk** with the operator's stated reason, which is
  `RESOLVED-WITH-ACCEPTED-RISK` and never plain `RESOLVED`. Or **close every finding that
  DISSENT raised**, which reaches `RESOLVED` while `Convergence:` stays `split` — the verdict
  did not change, the findings did.

  The third is the common case and the one most easily
  mistaken for the substitution below. `Convergence:` reads off the verdicts as issued, never as
  revised. A round is not `unanimous` because the orchestrator decided the objection no longer
  applies. This is the same substitution `evals.json` already forbids one step earlier:
  back-filling rule ids from your own read instead of re-dispatching. It is more tempting here,
  because by this point the fix is real and the objection genuinely looks closed.
- **A steward's per-lens verdict and its advisory nits are different things — do not conflate them.** A steward whose *only* findings are pure-style **RATIFIES** (the nits ride advisory in its addendum, never gating). A per-lens **DISSENT** is, by definition, a non-style blocking objection. A DISSENT is therefore never "just style", and nobody demotes it to advisory to clear the gate. "The steward only had style nits" ⇒ RATIFY-with-advisory; "the steward DISSENTed" ⇒ blocks until resolved or accepted-with-risk. The advisory/blocking line is the *severity* of the finding, not the identity of the lens.
- Output: the committed ledger with its typed header `State:`, the verdict, the resolved items with what changed, and any accepted-with-risk items. Set `State:` per the enum in `references/review-ledger.md` — `RESOLVED` / `RESOLVED-WITH-ACCEPTED-RISK` are the only passing terminals; `OpenFindings:` is `0` for those.
- If xreview cannot reach a clean verdict — reviewers split, an artifact gap nobody can close — say so explicitly. Set `State: OPEN-BLOCKED` with `OpenFindings: ≥1`; it **fails the gate to a human**. Nobody may relabel a split as `RESOLVED-WITH-ACCEPTED-RISK` to make the loop stop. Accepted-risk needs an operator decision on a *named* risk, not mere disagreement. A labeled open state beats a fabricated COMPATIBLE.

## Rationalization Table

<!-- vale off -->
<!-- The left column and the Red Flags below are verbatim phrasings — what the
     rationalization actually sounds like. An agent matches its own wording against
     them, so normalizing the contractions would blunt the recognition. -->

Documented failure modes during xreview. When your own reasoning aligns with the left column, **stop**. The right column is the reframe. (Citations in `references/findings-protocol.md`.)

| Excuse | Reality |
|--------|---------|
| "The staff engineer already read both specs separately — I can just confirm they're consistent." | Separate single-artifact reads do not cover the boundary between them. The seam is the one thing prior reads can't have checked, and it's where mismatches live. Review the integrated whole. |
| "The specialists already gave input during the design, so xreview is just writing up that they agree." | Per-component input incorporated serially is *production*, not review. Each specialist reacted to their slice against a moving target; none reacted to the final combined artifact. Re-dispatch them against the whole. |
| "They helped build the design, so they obviously agree with the final version." | Contribution is not endorsement. People routinely disagree with how their piece landed once they see the integration. Authorship of a part ≠ sign-off on the whole. |
| "All three experts converged, so it's COMPATIBLE." | If two read the first's comment before agreeing, that's anchoring, not corroboration (Asch). Convergence counts only when reviewers committed independently. Re-run blinded. |
| "It's just synthesis — I'm not the one endorsing the design." | Writing the COMPATIBLE verdict *is* the endorsement. The table carries your authority once it's filed. Own it or don't write it. |
| "I can phrase it 'their input was incorporated' rather than 'approved' — technically true, and it ships." | That launders a sign-off that never happened. The next reader sees "cross-reviewed." If the specialists didn't review the final artifact, say so plainly. |
| "The demo's in an hour — a quick 'looks consistent' is the helpful move." | Speed is a reason to be efficient, not to skip the check or fake the result. A 10-minute boundary review is cheap insurance; a fabricated green light just moves the failure to the demo. |
| "I don't have the doc, but I can write a plausible findings table from what such designs usually contain." | That's fabrication, not review. No artifact, no xreview — halt and get it. |
| "'Looks good' from two senior people is enough." | Absence of objection is not presence of review. A finding with no cited contract is not a finding. Require evidence. |
| "The skill change is a one-line typo / obviously small — it's mechanical, skip the steward pin." | A `skill-package` change routes by **file-type-present, not change-size** (`references/slate-routing.md` §3a). The mechanical-equivalent carve-out is `doc-only` only. A one-line typo in a `.claude/` skill body is still `skill-package` (T3, floor T2, `prose-steward` + the rubric lens pinned). "It's just a typo in a skill" does not demote it to `mechanical`; dropping the pin needs an operator override with a stated reason, never a size judgment. |

## Red Flags — STOP and Reset

Phrases that signal a rationalization is firing — in your reasoning or the user's framing:

- "They already reviewed it" / "they already weighed in" / "they already agree"
- "Just confirm it's consistent" / "just write up that they agree"
- "All three converged" / "everyone signed off" / "the team agrees"
- "LGTM" / "looks good" — offered *as* a finding
- "Quick xreview" / "we're behind, keep it fast"
- "Their input was incorporated" — standing in for "they reviewed the final"
- "It's just synthesis" / "I'm only writing it up"
- "I'll review from the summary" / "I don't need the actual doc"
- "It's just a typo in the skill" / "we've done dozens of these" — offered to demote a `skill-package` change to `mechanical` and drop the steward pin
- "The rubric lens read it and it's fine" — a rubric verdict that names no rule id. The lens has no absence check, so an uncited verdict is the only way its pin fails silently. Re-dispatch it.
- "P7 doesn't really apply here" / "the static rules all passed" — P7 is `block`. A `block` rule that could not run is an open finding, not a pass: report it `skipped` and DISSENT (`references/pressure-testing.md`). This is about a rule whose subject exists and could not be reached (`skip_reason: unavailable`) — not one with no subject at all (`inapplicable`, e.g. `S1` on a skill with no `scripts/`), which is simply not a finding.

**All of these mean: read the artifact, dispatch independent reviewers, require evidence per finding, or label the verdict honestly.**

<!-- vale on -->

## Halt Conditions

Stop and report to the user if:

- `Class:` was not emitted before dispatch (§0) — no classification ⇒ no review. HALT and classify before dispatching any reviewer.
- You cannot locate or paste the artifact under review — never synthesize a review of work you have not read.
- The calling repo has no `.claude/agents/` roster and the user cannot point at one.
- The rubric lens cannot read `references/skill-package-rubric.md` — HALT. An unread rubric yields ids emitted from memory, which reads as a cited review and is not one.
- `prose-steward` is absent from `.claude/agents/` on a `skill-package` change — HALT, not a silent drop (same posture as dropping the pin). Ask the operator, who may override with a stated reason.
- A reviewer returns bare approval with no cited evidence — re-dispatch with the evidence requirement.
- Reviewers were not blinded (saw each other's assessments first) — the convergence is invalid; re-run with independent briefs.
- Reviewers split on a boundary and the provider-owns tie-break does not resolve it. Surface the disagreement and ask the user / provider for the call.
- *Any* correctness-grade finding remains open and the user has not explicitly accepted the risk. The gating set: a MISMATCH/MISSING, a correctness-grade idiom *or* prose finding, or a per-lens DISSENT (including a pinned steward). Do not stamp a passing ledger `State:` (`RESOLVED`/`RESOLVED-WITH-ACCEPTED-RISK`); set `OPEN` or `OPEN-BLOCKED` (see Rule 4 / Step 5).

**Never declare COMPATIBLE to be helpful.** An honest OPEN verdict with named findings is the valuable output; a premature green light is the failure this skill exists to prevent.

## How this fits with coral and council

- **`/coral`** produces work with specialists, then *offers* `/xreview` at synthesis when outputs touch a shared boundary. Coral builds; xreview checks.
- **`/council`** runs xreview as a distinct phase of its scope-tier process by invoking this skill — it does not perform xreview itself.
<!-- gap: /code-review — this repository has never held a line-level correctness skill. Un-defer on the first correctness defect that reaches main through an xreview with no lens for it. -->
- **`/code-review`** is line-level diff correctness; **`/bugbash`** is adversarial hardening of a running system; **`/root-cause`** is incident investigation. xreview is consistency review of a produced artifact across the specialists who own its boundaries.
- **`idiomatic-reviewer`** (the `/idiomatic` skill) is the **idiom-conformance** lens — does the code read native to its language, framework, and the package's documented patterns. It is a distinct axis from boundary consistency. Xreview dispatches it as part of the slate when code is under review. Its findings ride in the Idiom addendum: correctness-grade blocks, style is advisory. It reviews idiom; it does not author the system or check boundaries.

## Output

End-of-session summary. Report the emitted `Class:`/`Tier:`/slate (§0), the committed ledger path, and the reviewer slate (naming who held the required dissent). Report the verdict with its `State:` token, what the round resolved and how, and any accepted-with-risk items. List any rejected findings with the check that confirmed each rejection.

For a code review, include the Idiom addendum; for prose, the Prose addendum (correctness-grade blocks, style advisory). If open, name the unresolved findings and what would close them. If the split stands unresolved, set `State: OPEN-BLOCKED` to a human.
