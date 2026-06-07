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
- **(b) Linear** — `list_issues(label: "impact:<slug>", updated OR completed in [thisMonday, until])`, **cross-engineer** (no assignee filter — unlike `impact-weekly`, which is Person-scoped). ≥1 issue ⇒ active.

**Why union, not label-only:** at cold start most work is untagged and the label scan under-counts; toggle-present catches bets whose owner ran their weekly without a label. **Why not toggle-only:** then a skipped weekly = a silently missing section.

**The residual hole — state it, don't hide it.** Work that is *both* untagged *and* has no toggle is invisible to both signals. This is irreducible without a name-match scan (out of MVP scope). The report does **not** claim total coverage; the untagged-rate is the signal this risk is high (the shared lever with `impact-weekly` — bulk-labeling drives it down).

The detection window is the **same Monday-anchored calendar week** as the producer's toggle key. If the Linear window and the toggle key disagree, a bet can show Linear activity while its toggle sits under the adjacent Monday → a false gap. Use the identical ISO Monday for both.

## Content lift

For each active bet, read its `Week of <thisMonday>` toggle and lift outcomes:

- **≤3 visible bullets**, top by impact. >3 material outcomes → keep 3 + a `+N more in the weekly →` pointer (link to the bet's Weekly log). Never a silent drop; never a 4th visible bullet.
- Carry each bullet's **evidence link verbatim** — do not re-derive from Linear (avoids report/log divergence).
- One line per bullet, ≤1 context sentence, no inlined PR/issue bodies.

## Owner & confidence

- **Owner** = the bet's `Person` (user-ID → display name; degrade to raw ID, flagged; never fabricate). Render with the framing **"owner ≠ sole contributor"** — work is cross-engineer and the bet owner may not be the doer. Do not use this page for individual credit until a contributor-attribution model exists.
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

Minimal evidence unit per section bullet = a Linear issue URL (PR secondary), inherited from the toggle. A bullet whose upstream had no link is **cut** ("unsubstantiated — cite or cut"). Exec-summary bullets carry no own link but must **trace to a substantiated section** — a new top-level claim with no section behind it is cut. Never fabricate.

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

## Reuse with `impact-weekly`

**Shared** (a suite-level reference both skills read — do not fork): bet **page-ID identity**, `impact:<slug>` **label-first resolution**, the canonical **ISO-Monday week-key derivation**, **`Person` → display-name** resolution, the **brevity** and **substantiation** rules.

**`impact-portfolio`-local:** the cross-engineer/cross-quarter selection query, union detection, the report-page write contract, and the exec-summary roll-up.

**Stays `impact-weekly`-local:** the write-to-bet coverage gate and bet-write target re-verification — this skill never writes a bet.
