# Linear Integration

How `/issue` files to the Linear sink. Read alongside `format-spec.md` (the body shape) and the SKILL.md procedure. The rendered body is identical to the GitHub path — Linear is just a second sink for the same synthesized markdown.

## When this path runs

Step 6 of the procedure, when the user picks **Linear** at the sink prompt. The body is already rendered (steps 2–4); this path only handles destination.

## Preconditions (checked here, not upfront)

The Linear MCP tools must be connected and authenticated — at minimum `list_teams` and `save_issue` (others below are used when offered). They are interactively-authenticated and **may be absent in headless / cron runs**. If a Linear tool isn't available, say so plainly and fall back to the GitHub sink or print the body. **Never fabricate a Linear identifier or URL.**

## Field mapping

| Issue field (format-spec) | Linear `save_issue` field |
|---|---|
| Title | `title` (required) |
| The full rendered body (Problem / Impact / Relevant experts / …) | `description` (Markdown — pass **literal newlines**, not `\n` escapes) |
| — (Linear requires a team) | `team` (required — resolved interactively, below) |
| References URLs (optional) | left inline in the body; optionally also added as `links` attachments if the user asks |

`Relevant experts` stays in the Markdown body exactly as rendered (the `.claude/agents` persona names) — Linear has no native concept for it, and that's fine; it's routing prose, not metadata.

## Procedure

1. **Resolve the team — always ask, never guess.** Call the `list_teams` tool and present the names; the user picks one. Linear requires a team and there is no safe default, so do not infer it from the repo or pick silently — this holds **even on a coral/council handoff**: the team is always interactively resolved, so the orchestrator should not promise a pre-filled Linear destination. If the user already named a team in their invocation ("file this in Linear under Platform"), confirm that team appears in the `list_teams` result and use it; **if the named team isn't in the result** (typo, renamed, no access), say so and present the actual list to pick from — never fall back to a first/default team.

2. **Offer optional fields — offer, don't force.** After the team is chosen, offer (in one prompt, all optional):
   - **Project** (`project`) — call the `list_projects` tool (scoped to the team) if the user wants to attach one.
   - **Labels** (`labels`) — call the `list_issue_labels` tool (scoped to the team); pass label names or IDs.
   - **Priority** (`priority`) — `0` None, `1` Urgent, `2` High, `3` Medium, `4` Low.
   - **Assignee** (`assignee`) — user ID / name / email / "me".
   - **Impact bet** (optional) — if this work advances an Impact Hub bet, offer to apply its `impact:<slug>` label (see *Impact-bet decoration* below).

   These are MCP tool calls, not CLI commands — there is no Linear CLI. If a list comes back long, ask the user to name the project/label rather than enumerating dozens. Skip any the user doesn't want. Don't block on them; a title + team + description is a complete Linear issue.

   **Impact-bet decoration (Linear-sink only).** If the work advances an Impact Hub bet, offer to stamp the `impact:<slug>` label so `impact-weekly` rolls it up deterministically instead of falling back to name-matching. On opt-in:
   - **Resolve the bet — never guess.** Query the Impact Tracker data source via Notion (`notion-fetch` / `notion-search` the Impact Hub) filtered to the engineer's `Person`; present their bets and let the user pick (or say "none").
   - **Derive the label.** `slug` = kebab-case of the bet's Name; the label is `impact:<slug>`. The bet's Notion **page ID is the canonical identity** — the slug is a re-derivable display alias. The **label on the Linear issue is the durable record**; `/issue` does *not* write impact-weekly's state — impact-weekly resolves `impact:<slug>` → page ID and caches it on its own first read. Don't reach into another skill's cache.
   - **Ensure the label exists.** `list_issue_labels` (team-scoped); if `impact:<slug>` is absent, create it with `create_issue_label` (`create_issue_label` is part of the connected Linear MCP). Then add it to the issue's `labels`. `/issue` only ever *creates/applies* the label — it never deletes or renames one; stale/duplicate `impact:` labels from a bet rename are surfaced and reconciled on impact-weekly's read side (human relabel), an accepted MVP liability.
   - **Linear-sink only.** A GitHub-sink issue carries no Linear label — instead note the bet in the issue's **References** section as `Impact bet: <Name> — <url>` (distinct from the body's `## Impact` cost section) and skip the label. Offer, don't force. Convention: `docs/designs/impact-hub-pm-skill-suite.md` (decoration convention) — the same one `impact-weekly` consumes.

3. **Create.** Call `save_issue` **without `id`** (passing `id` updates an existing issue — only create here):
   - `title` — the issue title.
   - `team` — the resolved team name or ID (`save_issue` accepts either; the name the user picked from `list_teams` is fine).
   - `description` — the rendered body, as literal Markdown.
   - plus any optional fields the user chose.

4. **Echo the result.** Report the returned issue **identifier** (e.g. `ENG-123`) and its URL. This identifier is the "next workstream" pointer the orchestrator records on a coral/council handoff. **If `save_issue` returns without an identifier or URL**, report what it did return ("created, but couldn't read back the identifier") and surface the raw response — never synthesize a plausible `ENG-NNN` or URL to fill the gap.

## Guardrails specific to this path

- **Always create, never update.** `/issue` files *new* issues. Do not pass `id` to `save_issue` — that would mutate an existing ticket. Updating/triaging existing issues is out of scope *for this skill* (see SKILL.md "What this skill doesn't do"). (`/design` legitimately *does* update via `save_issue id` to thread its reverse link — that's a different skill's job, not this one's.)
- **One issue per invocation.** Same as the GitHub path — if a session produced multiple deferred slices, the user picks which to file; don't batch-create.
- **No fabricated identifiers.** If creation fails or the MCP is unavailable, surface the failure and offer GitHub / print. A made-up `ENG-NNN` is worse than an honest "couldn't reach Linear."
- **One sink, one issue per invocation.** No cross-posting the same issue to both GitHub and Linear, and no parent/sub-issue (`parentId`) linking — both are deferred. Un-defer when a user actually asks; until then, file one issue to one sink.
- **Impact-bet decoration is offered, never forced or guessed.** Never invent a bet or a slug; resolve the bet from Notion and let the user pick. The bet's **page ID is the identity**, the `impact:<slug>` label is the alias — don't treat the slug as the join key.

## Lineage with `/design`

GitHub issues thread to designs via `#<n>`. Linear issues use an identifier (e.g. `ENG-123`) instead.

`/design` consumes both: `/design --issue ENG-123` detects the Linear identifier, fetches the issue via the Linear `get_issue` MCP tool, seeds the design from its body, records `**Issue:** ENG-123 — <url>` in the design frontmatter, and reverse-links via a Linear comment (`save_comment`) or a description edit (`save_issue`). So the `--issue` lineage flow works for Linear-tracked work just as it does for GitHub. One difference to know: GitHub gets free PR back-linking (`Closes #<n>`), while Linear's PR linkage depends on its GitHub integration being configured — so for Linear the explicit reverse comment/description link is the primary thread, not an optional extra. See `.claude/skills/design/references/issue-integration.md` ("Two sources: GitHub and Linear").
