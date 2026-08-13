# Selection, Coverage & Rendering

What goes into the weekly report and how it's shaped. Read alongside `write-contract.md` (how it's written) and the SKILL.md procedure.

## Section selection

Candidates = Impact Tracker rows where `Quarter == <current>` (runner-confirmed) and `Person` is non-null (a section needs an owner). Query against `collection://35edb6ff-6057-8038-9d07-000b08363d40`:

```sql
SELECT pageId, Name, Person, "Overall Confidence"
FROM impact_tracker
WHERE Quarter = :currentQuarter
  AND Person IS NOT NULL;
```

A candidate becomes a **section** only if it had **activity this week** (next).

## Union detection (anti-FM#1)

A bet is active this week if **either** signal fires:

- **(a) Toggle-present** — its Weekly log contains a `> Week of <thisMonday>` toggle (the exact ISO Monday; not "latest", not "in range"). Toggle-present *is* proof of activity.
- **(b) Linear** — `list_issues(label: "impact:<slug>", updated OR completed in [thisMonday, until])`, **cross-engineer** (no assignee filter — unlike `impact-weekly`, which is Person-scoped). ≥1 issue ⇒ active. This scan is **read-only**: `list_issues` only. Never apply an `impact:` label, set a status, or comment on a Linear issue to "improve" detection — that is a write to work you don't own. (Bulk-labeling to drive the untagged-rate down is a separate, human-led lever, not this skill's action.)

**Why union, not label-only:** at cold start most work is untagged and the label scan under-counts; toggle-present catches bets whose owner ran their weekly without a label. **Why not toggle-only:** then a skipped weekly = a silently missing section.

**Partial Linear scan — unknown is not quiet.** If the `impact:<slug>` query errors or times out for a bet (Linear is connected but *that* slug's scan failed), its signal (b) is **unknown, not negative**. Never classify such a bet "quiet" on a failed Linear half. If it also has no toggle, record it in the partial-fetch manifest as a source that **couldn't be read** — never silently treat unread as inactive (that is exactly the FM#1 silent omission the report exists to prevent).

**The residual hole — state it, don't hide it.** Work that is *both* untagged *and* has no toggle is invisible to both signals. This is irreducible without a name-match scan (out of MVP scope). The report does **not** claim total coverage; the untagged-rate is the signal this risk is high (the shared lever with `impact-weekly` — bulk-labeling drives it down).

The detection window is the **same Monday-anchored calendar week** as the producer's toggle key. If the Linear window and the toggle key disagree, a bet can show Linear activity while its toggle sits under the adjacent Monday → a false gap. Use the identical ISO Monday for both.

## Content lift

For each active bet, read its `Week of <thisMonday>` toggle and lift outcomes:

- **≤3 visible bullets**, top by impact. >3 material outcomes → keep 3 + a `+N more in the weekly →` pointer (link to the bet's Weekly log). Never a silent drop; never a 4th visible bullet.
- Carry each bullet's **evidence link verbatim** — do not re-derive from Linear (avoids report/log divergence).
- One line per bullet, ≤1 context sentence, no inlined PR/issue bodies.

## Owner & confidence

- **Owner** = the bet's `Person` (user-ID → display name; degrade to raw ID, flagged; never fabricate). Render with the framing **"owner ≠ sole contributor"** — work is cross-engineer and the bet owner may not be the doer. Do not use this report for individual credit until a contributor-attribution model exists.
- **Overall Confidence** = the bet's property (On Track / At Risk / Need Help / Not Started / Delivered), shown next to the owner. It is the exec counter-signal to a rosy self-authored weekly — a glowing narrative under an At-Risk bet must be visible.

## Coverage gaps (rendered, not refused)

A bet detected active by Linear (b) but with **no toggle** (a) is a **coverage gap** — the owner hasn't written their weekly, so there is no content to lift. It renders in a **"Coverage gaps this week"** section (owner + the Linear evidence that proves activity). It is:

- **not** auto-filled from Linear (preserves the producer/consumer boundary),
- **not** a write-blocker (refusing on gaps makes the common cold-start week a refusal — the adoption-killer that shaped `impact-weekly`),
- **visible to the exec reader**, not just surfaced to the runner at confirm time (a gap the runner can silently drop is a lie of omission).

Quiet bets (no activity by either signal) are simply omitted — optionally one tail line "Quiet this week: …".

## Non-bet projects (Wave-class)

An active project that is **not yet a bet** (e.g. Wave) is a **runner-supplied manual injection**: name, owner, ≤3 bullets **each with ≥1 evidence link** (PR/Linear/doc). Rendered in a **"Not yet tracked"** section, flagged **"(not yet a bet — needs adding)"**, and listed as a tracking-gap action item. The flag prevents *bet-laundering*; the link requirement prevents *unsubstantiation* — the manual door is the one path that could smuggle in a fabricated claim, so it gets the same substantiate-or-cut rule as everything else.

## Substantiation (anti-FM#3)

Minimal evidence unit per section bullet = a Linear issue URL (PR secondary), inherited from the toggle. A bullet whose upstream had no link is **cut** ("unsubstantiated — cite or cut"). Exec-summary bullets carry no own link but must **trace to a substantiated section** — a new top-level claim with no section behind it is cut. The exec summary is the one place the skill writes synthesized prose, so the line is tight: **aggregate section facts only** — a trend, judgment, or comparative characterization ("Platform is accelerating", "we're ahead on X") that no source weekly actually made is fabrication of narrative even if it "traces" to a section. Restate what the sections substantiate; never fabricate.

In the **manager view** the evidence unit relocates from the inline bullet to the per-person **appendix** item, but the rule is unchanged in force: a one-liner with no appendix item is the manager-view equivalent of the cut "unsubstantiated" bullet, and the same anti-fabrication line applies to its narrative framing (see **Manager-view rendering → grounding discipline** above).

## Rendered shape

```markdown
# Impact Report - Week of June 8th      (display; identity = report_week: 2026-06-08)

## Executive summary
- <portfolio-altitude bullet — a cross-cutting theme / headline / milestone>   (3–5; trace to a section, no restatement)

## <Bet Name>
**Owner:** <name> (owner ≠ sole contributor) · **Confidence:** At Risk
- <delivery> — [SEI-123](url) ([PR #456](url))
- <delivery> — [SEI-789](url)
- <delivery> — [SEI-790](url) · +2 more in the weekly →

## Coverage gaps this week
- <Bet> (Owner X): active in Linear, no weekly written — chase the weekly

## Not yet tracked
- Wave — Owner Y, (not yet a bet — needs adding): <delivery> — [PR](url)

---
_Read 9 of 11 sources; could not load: <bet>, <bet>._
```

The shape above is the **durable per-bet report** (the Weekly Reports row). The `--read-only` **manager view** uses the different shape below.

## Manager-view rendering (one-liners + appendix)

The `--read-only` manager view answers *"what did my team do this week"* and is organized **by person, not by bet**. It reads as a leader-facing narrative — short, human one-liners — with the evidence relocated to a single **Work items** appendix at the end. This is the *only* render that moves links out of the body; the durable report keeps inline links.

**Body — per person:** a short theme label, then **2–4 one-liners**. A one-liner = a plain-language outcome, plus a *why-it-matters* clause whenever the work item supports one (include it for the readability this format exists to deliver; omit it rather than invent one — rule 2). No Linear/PR links inline.

**Appendix — one at the end, grouped by person:** the Linear-issue / PR links for that person's items, so every one-liner is traceable. A person whose data failed to load gets an explicit "no data this run" line here (never silent omission).

### The grounding discipline (what keeps a friendly one-liner honest)

The narrative latitude is the risk this format adds — so a one-liner's freedom is bounded by four rules. **This section is the canonical statement; SKILL.md Guardrails 3–4 restate it in brief and defer here on any conflict.**

1. **Trace to a *supporting* item — not merely an adjacent one.** Every one-liner must be backed by ≥1 specific appendix work-item that **actually substantiates its outcome**; existence of *some* item in the person's block is not enough (that lets an orphan claim lineage to the nearest ticket). The agent records, per one-liner, which item(s) back it; a line no single named item supports is **cut**. To keep that correspondence recoverable **without re-cluttering the body with IDs** (the whole point of moving links out), **order each person's appendix items to mirror the one-liner order**; an aggregate line that legitimately spans items must be backed by *all* of them. Links live in the appendix only.

2. **Entailed, not invented — the test.** A one-liner's why-it-matters clause passes one test: **could a reader confirm it from the work item alone — its title, status, and what the change does?** *Allowed:* a faithful plain-language description of what the change *is* or *does*, including the evident *kind* of benefit that directly follows ("fix hardcoded evmone dylibs + add to dockerfile" → "removed the hardcoded evmone libs by vendoring them in the Dockerfile"; "single-pane Loki federation" → "one place to query all logs"). *Forbidden — invented, cut it:* a quantified or comparative or business claim the item doesn't state — a metric, a superlative ("biggest release of the year"), a "win", a trend, a cross-week/cross-person comparison ("a strong week across the team"), a cost/efficiency outcome ("cost savings", "compounding leverage"), a risk-posture verb the item doesn't ("de-risked", "hardened the attack surface" from a bare "bounded-compute fix"), or an invented specific — an adjective, a quantifier, or a **program / project / initiative name the item doesn't carry** ("a set of broken/outdated links" from "update README links"; "the Immunefi backlog" from a bare "bounded-compute fix"). A secondhand "this relates to the cost effort" is **not** the item stating a cost result. The why-it-matters clause **carries no citation by construction** (it is the un-linked half of the line) — so it must be the **most conservative** part of the one-liner; when in doubt, drop it and state only the outcome. **Status is part of what a reader can confirm:** a found-but-unfixed item (a Todo / open finding) renders as a **found gap**, never a shipped win.

   **Operator-supplied context is not agent fabrication.** A human runner may layer in higher-altitude framing they know first-hand — "the biggest release of the year", "the team's priority now is landing it well, not new features" — at the draft/confirm step; that is human-authored truth, like a manual injection, and it is exactly the warmth this format wants. The line the test draws is only on what the **agent generates unaided**: the agent never manufactures such framing from a ticket title. (If in doubt whether a clause is operator-given or agent-invented, it's invented — cut it and let the human add it back. The read-only view writes nothing and is regenerated from source each run, so operator enrichment lives in the human's own edit, not in agent-persisted prose — the agent re-derives only the conservative baseline and never re-launders an enriched line back in as its own.)

3. **Thin reads thin; self gets no polish.** A one-item week is one honest line — never inflated to manufacture parity or because someone "shouldn't look idle" (raise visibility upstream, not in the digest — a padded line collapses the moment someone asks the engineer about the "hardening"). The runner's **own** section gets the identical discipline; self-flattering framing is the same fabrication — and on a page the runner authored it is the **least-trusted line**, so it discredits the honest sections around it.

4. **No cross-item synthesis in a one-liner.** Aggregating N small items into one larger-sounding theme ("a broad reliability sweep", "modernizing the tooling stack") is the same fabrication as a cross-person comparison — the *aggregate characterization* is stated by no single item. Give N honest lines (or fewer), each tracing to its item; thematic/portfolio altitude lives only in the durable report's exec summary (which itself bans invented trends — see Substantiation), never in a manager-view one-liner. And **thin data is not a license for richer prose**: when a partial fetch leaves the page sparse, render fewer plain lines + the coverage note — never lean on narrative to make a thin run read substantive (that is the partial-read polish the manifest exists to prevent).

**Block length is not a productivity score, and this is not a credit readout.** State once at the top that the digest reflects **tracked** work and is **not an individual-credit or performance readout** (it is organized by person for scannability; bet activity is cross-engineer). A short per-person block can mean a thin week, deep work that didn't decompose into many tickets, *or* untracked work invisible to the scan — so where `betGraph.rollup` shows in-window `untaggedNearby` activity for a person, add a neutral "+ untagged activity not itemized" line rather than letting block-length be misread. Where a person's tracked work rolls into an **At Risk / Need Help** bet, surface that flag — a friendly one-liner must not bury a struggling bet.

Coverage gaps and the partial-fetch manifest **still render** (a coverage note in the body + per-person "no data this run" appendix lines). A partial read is never polished into a clean-looking full-team digest — "make it read clean for the VP" is the tell. When a person has more material outcomes than ~4 one-liners can carry, the surplus stays as appendix work-items (already linked, traceable) — **never a silent drop**; the body picks top-by-impact, mirroring the durable report's top-3 rule.

### Worked shape

The **agent-generated baseline** (every clause confirmable from its item; appendix ordered to mirror the body):

```markdown
_Tracked work only — organized by person for scannability; not an individual-credit or performance readout._

## Masih — release & security
- v6.6 release checklist and coordination is underway — backports, RC builds, and testing in progress.
- An AI security scan of the v6.6 branch flagged a real gap: the EVM RPC deny-list isn't applied to the WebSocket path (open, not yet fixed).

## Brandon — observability & agentic tooling
- Single-pane Loki log federation — one place to query all logs — is in progress.
- Building the omni-trigger on-call receiver ("Wall-E") that auto-triages PagerDuty alerts; trigger pipeline in progress.

## Amir — RPC bounded-compute
- Landed bounded-compute fixes on the RPC path (tx/block-search caps, HTTP timeouts); two follow-on fixes are in review.

## Coverage note
Linear scan for **Monty** did not return this run (timeout) — not represented below.

---
## Appendix — work items
### Masih   (order mirrors the one-liners above)
- [PLT-439](url) v6.6 release checklist & coordination (In progress)
- [PLT-744](url) EVM RPC deny_list not applied to WebSocket (Todo — the scanner finding)
### Brandon
- [PLT-751](url) single-pane Loki federation (In progress) · [PLT-715](url) omni-trigger receiver (sei-internal-skills #214–218)
### Amir
- [PLT-700](url) / [PLT-440](url) bounded-compute fixes (Done) · [PLT-701](url) / [PLT-702](url) (In review)
### Monty
- _no data this run (Linear scan timed out)_

_Read 5 of 6 sources; could not load: Monty._
```

Every body one-liner restates its items in their own terms — no metric, trend, superlative, beneficiary, or business outcome the items don't state; the scanner finding renders as a *found* gap (a Todo), not a shipped win.

The **runner may then enrich in their own voice** at the confirm step with context they know first-hand — e.g. Masih's line becomes "6.6 branch is cut and clean — the biggest release of the year; the priority now is landing it well, not new features." That framing is legitimate *because a human supplied it* (rule 2's operator carve-out); the agent does not generate it from the ticket.

## Reuse with `impact-weekly`

**Planned shared reference — not yet extracted.** These concepts are currently authored in *both* skills and should converge into one suite-level reference both read, **once the Notion write mechanism is spiked and the contracts freeze** (tracked: sei-internal-skills #119 / PLT-437) — extracting now would lock interfaces the spike may still move: bet **page-ID identity**, `impact:<slug>` **label-first resolution**, the canonical **ISO-Monday week-key derivation**, **`Person` → display-name** resolution, the **brevity** and **substantiation** rules. Until then they are inlined in each skill and kept in sync by hand.

**`impact-portfolio`-local:** the cross-engineer/cross-quarter selection query, union detection, the report-row write contract, and the exec-summary roll-up.

**Stays `impact-weekly`-local:** the write-to-bet coverage gate and bet-write target re-verification — this skill never writes a bet.
