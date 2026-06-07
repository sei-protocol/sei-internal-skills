---
name: impact-portfolio
model: claude-opus-4-8
description: "Use when generating the weekly cross-project executive report for the Impact Hub — 'impact portfolio', 'weekly impact report', 'portfolio report', 'cross-project impact report', 'generate the weekly Impact report', '/impact-portfolio'. Reads the week's per-bet Weekly-log entries (+ a Linear impact:<slug> activity scan) across current-quarter bets and writes ONE human-confirmed report page per week under the Impact Hub's Weekly Reports section: exec summary + per-project sections (owner, Overall Confidence, ≤3 substantiated bullets). Anti-triggers: NOT for one engineer's per-bet weekly update (use /impact-weekly); NOT for the end-of-quarter per-engineer rollup (deferred impact-eoq); NOT for filing issues (use /issue) or capturing a design (use /design); NOT for editing a bet's definition fields or Weekly log — it is read-only on bets and writes only its own report page."
---

# Impact Portfolio

The leader-facing **synthesis tail** of the Impact Hub loop. `impact-weekly` is the producer — each engineer turns their week into a per-bet Weekly-log toggle. `impact-portfolio` is the **reader-facing aggregator**: once a week it lifts those entries into a single executive page so a leader opens *one* page and sees what got done across every project, each claim clickable, in under three minutes.

It does **not** generate truth — it indexes truth that already exists in the bets' Weekly logs. That framing is the whole safety model: it is **read-only on every bet page** and writes exactly one artifact — its own report page.

The runtime Notion mechanics (creating a page under the "Weekly Reports" toggle, a custom page property for identity) are **spike-gated preconditions** — verify them before first live run (see `references/write-contract.md`). The design is captured at `docs/designs/impact-portfolio-weekly-report.md`.

## Guardrails

This skill writes **one** artifact: the week's report page under the Impact Hub's "Weekly Reports" section. Before any write:

1. **Report page only — read-only on bets.** Never write, edit, or set anything on a bet's page (not its Weekly log, not Overall Confidence, not the definition fields). Immediately before any write, assert (a) the parent is the "Weekly Reports" section on the Impact Hub page and (b) the target page carries this skill's own provenance marker — **refuse on either mismatch**. This skill consumes the bets; it never mutates them.
2. **Cover or surface — never a silent subset.** Detect activity by **union** (Linear `impact:<slug>` scan ∪ a bet having a Weekly-log toggle this week). Render coverage gaps and the partial-fetch manifest **on the page itself** — incomplete coverage is *shown to the reader*, never silently dropped and never used to refuse the report. Only fabrication is refused. A project with activity but no bet (e.g. Wave) is included **flagged** "(not yet a bet)", never laundered as tracked.
3. **Substantiate or refuse — links inherited, never invented.** Every delivery bullet carries ≥1 evidence link, lifted from its source Weekly-log entry (a manually-injected Wave bullet must carry its own ≥1 link or it is cut). A bullet with no upstream link is refused: "unsubstantiated — cite or cut." Exec-summary bullets need no own link but must trace to a substantiated section. Never fabricate a link, owner, confidence, or section.
4. **Index of indexes — ≤3 bullets/section, no inlining.** Per section: at most 3 visible bullets, one line each, ≤1 context sentence, no inlined PR/issue bodies. When a project shipped more than 3 material outcomes, show the top 3 and append a "+N more in the weekly →" pointer — **never silently drop, never emit a 4th visible bullet.** Exec summary: 3–5 portfolio-altitude bullets, no verbatim restatement of sections.
5. **Draft → confirm → write; idempotent; degrade, don't fabricate.** Always render the full page, the exact parent, **and the destructive-action summary** (resolved target page id + last-editor + create-vs-replace) before writing; require explicit confirmation. One page per week, keyed on the **ISO Monday** (a page property, never the display title) — re-run updates in place and pulls in newly-added toggles; a duplicate or a provenance-marker mismatch **halts**. If Notion or any source is unavailable, say what failed and stop.

See `references/write-contract.md` (the Notion create/update mechanism, identity, provenance, idempotency) and `references/selection-and-coverage.md` (the selection query, union detection, owner/confidence, gaps, brevity).

## Who runs this

A **single named runner** per week (an EM / chief-of-staff). Single-runner is a convention, not a lock: MVP has no concurrency control, and two simultaneous runs can create duplicate pages (the >1-match halt then blocks until a human deletes one). Don't open the run to a rotating pool until a lock exists.

## Preconditions (checked at run, not assumed)

- **Notion MCP** connected, with the capabilities the write contract depends on (page-create under a toggle + a custom page property, or the `update_content` fallback — **spike before first live run**, see `references/write-contract.md`). The Impact Hub page (`35edb6ff605780b6b023d95456209168`) and its "Weekly Reports" section are reachable.
- **Linear MCP** connected for the activity scan (`list_issues` by label, cross-engineer). Interactively-authenticated — may be absent in headless runs; if so, detection degrades to toggle-present only and the report says so.
- **Impact Tracker** data source readable (`collection://35edb6ff-6057-8038-9d07-000b08363d40`).

## Procedure

1. **Resolve the week and quarter.** Compute the week's **ISO Monday** (anchored to a declared TZ — see write-contract) — this is the identity and the lookup key everywhere. Confirm the current quarter (e.g. `Q2 2026`) with the runner; don't infer silently.

2. **Select candidate bets.** Query the Impact Tracker for current-quarter bets with a non-null `Person` (cross-engineer). These are the *candidates*; activity (step 3) decides which become sections. See `references/selection-and-coverage.md`.

3. **Union-detect activity.** A bet is active this week if **either**: it has a `> Week of <thisMonday>` toggle in its Weekly log, **or** the Linear `impact:<slug>` scan returns ≥1 issue updated/completed in the *same* Monday-anchored week. Record which signal fired — it drives step 5.

4. **Lift content.** For each active bet, read its `Week of <thisMonday>` toggle and lift its outcomes, condensing to **≤3** (top by impact; if >3, keep 3 + a "+N more in the weekly →" pointer). Carry each bullet's evidence link through **verbatim** — never re-derive. Resolve the owner (`Person` → display name; degrade to raw ID, never fabricate) and read the bet's **Overall Confidence**.

5. **Compute coverage gaps + the fetch manifest.** A bet detected active by Linear but with **no toggle** is a **coverage gap** (the owner hasn't written their weekly) — it renders in a "Coverage gaps this week" section (owner + the Linear evidence), it is **not** auto-filled from Linear and it does **not** block the report. Track every source that failed to fetch for the "read N of M sources" manifest.

6. **Take manual injections (optional).** For an active project that is **not yet a bet** (e.g. Wave), accept a runner-supplied section: name, owner, ≤3 bullets **each with ≥1 link**. Flag it "(not yet a bet — needs adding)". No link → no bullet.

7. **Render the page.** Exec summary (3–5 portfolio bullets, each tracing to a section) → per-project sections (owner + confidence + ≤3 linked bullets) → "Coverage gaps this week" → "Not yet tracked" (injections) → the fetch manifest footer. Apply the brevity rules as a refusal, not a softening.

8. **Confirm the destructive action.** Show the rendered page, the exact parent ("Weekly Reports"), and the **resolved target**: absent → create; exactly 1 (marker asserts) → update in place; >1 or marker mismatch → halt. Display the target page id + last-editor so a clobber is visible. Require explicit confirmation.

9. **Write + report.** On confirmation, create or update per the write contract (writing the provenance marker + ISO `report_week` property on create). Echo the report page URL, the section count, the coverage gaps still open, and any tracking-gap action items (e.g. "add Wave as a bet").

## Rationalization Table

When your reasoning matches the left column, **stop**. The right column is the reframe.

| Excuse | Reality |
|--------|---------|
| "An engineer skipped their weekly — I'll just pull their bet's week from Linear so the report looks complete." | That re-implements `impact-weekly`'s mapping engine and inverts the producer/consumer boundary. The skill indexes content; it does not author it. The gap is **rendered**, not papered over. |
| "Only 3 of 9 bets have weeklies — I'll write what I have and ship." | A report that silently omits 6 active projects is the *current* broken state with extra steps. Render the coverage gaps so the reader sees the holes. |
| "The week's Monday is obvious — I'll match the page by its title." | The title is a display string ("June 8th"): ordinal/locale/TZ drift makes it a non-deterministic key → duplicate pages. Match on the ISO `report_week` property. |
| "This is clearly our report page — full-body replace is safe." | "Ours" was resolved by a fuzzy title match on a human-writable board. Assert the provenance marker first, or you'll clobber a human's colliding page or their mid-week edit. |
| "Half the toggles failed to fetch but the rest look fine — I'll publish." | A partial report that reads complete is a lie of omission on a leadership page. Render the "read N of M" manifest or halt. |
| "The owner wrote a great weekly — I'll lift it; the bet's confidence doesn't matter." | A rosy narrative under an At-Risk bet misleads the exec. Surface Overall Confidence next to the owner; it's the counter-signal. |
| "This project shipped 5 things — I'll list all 5 so credit is fair." | The exec page is ≤3 by design. Keep 3 + a "+N more →" pointer; the weekly is the fuller index. |
| "Wave isn't a bet yet but I'll just add a clean section for it." | Include it **flagged** "(not yet a bet)" with real links — never unflagged (laundering) and never link-free (fabrication). |
| "It's just a report — the confirm step is a formality, I'll auto-write." | The write lands on a shared exec board and can replace an existing page. The confirm — especially the destructive-action summary — is the safety. Never auto-write. |

## Red Flags — STOP

- "I'll fill the gap from Linear so it's complete" / "just pull their week"
- "Write what we have" / "the rest were probably quiet"
- "Match the page by title" / "the title is deterministic enough"
- "We own the page, replace is fine" (without asserting the marker)
- "Most sources came back" (publishing a partial as complete)
- "The weekly says it's going great" (ignoring Overall Confidence)
- "Just this once, auto-write it — it's only a report"

**All of these mean: union-detect, render gaps + manifest, key on the ISO property, assert provenance before replace, surface confidence, and confirm the destructive action.**

## Halt Conditions

Stop and surface to the user if:

- More than one report page resolves for the week, or the resolved target lacks this skill's provenance marker — never guess or clobber; surface for human resolution.
- The parent is not the "Weekly Reports" section on the Impact Hub page — refuse the write.
- A manual injection (or any bullet) has no evidence link — cut it or get the link; never fabricate one.
- Notion is unavailable, or sources fetched only partially and the runner won't accept a manifest-flagged partial — stop rather than publish a report that reads complete.
- The current quarter can't be established — ask; don't infer.

**Never declare the report complete to be helpful.** A page that visibly names its gaps is the valuable output; a clean-looking page that hides them is the failure this skill exists to prevent.

## What this skill doesn't do

- **Write or edit bets.** Read-only on every bet page. Per-bet updates are `impact-weekly`'s job.
- **Author content.** It lifts and condenses what the weeklies already say; it doesn't draft new progress prose.
- **Auto-fill skipped weeklies from Linear.** Gaps are rendered, not synthesized. (Deferred — thin-Linear fallback; un-defer only if surfacing proves too lossy at high adoption.)
- **Attribute contributors.** Owner = the bet's `Person`, rendered "owner ≠ sole contributor." Real contributor attribution is deferred — don't use this page for individual credit until it exists.
- **Auto-discover non-bet projects.** Wave-class projects are manual injections until they're real bets (or a deterministic PR→project filter exists).
- **The end-of-quarter rollup.** That's the deferred `impact-eoq`.

## Output

End-of-turn: the report page URL, the section count, open coverage gaps, the "read N of M sources" result, and any tracking-gap action items (e.g. "Wave still needs a bet"). If nothing was written (halt), say why and what would unblock it.
