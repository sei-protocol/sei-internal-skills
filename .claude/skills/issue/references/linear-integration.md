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

1. **Resolve the team — always ask, never guess.** Call `list_teams` and present the names; the user picks one. Linear requires a team and there is no safe default, so do not infer it from the repo or pick silently. If the user already named a team in their invocation ("file this in Linear under Platform"), confirm that team exists in the `list_teams` result and use it.

2. **Offer optional fields — offer, don't force.** After the team is chosen, offer (in one prompt, all optional):
   - **Project** (`project`) — `list_projects --team <team>` if the user wants to attach one.
   - **Labels** (`labels`) — `list_issue_labels --team <team>`; pass label names or IDs.
   - **Priority** (`priority`) — `0` None, `1` Urgent, `2` High, `3` Medium, `4` Low.
   - **Assignee** (`assignee`) — user ID / name / email / "me".

   Skip any the user doesn't want. Don't block on them; a title + team + description is a complete Linear issue.

3. **Create.** Call `save_issue` **without `id`** (passing `id` updates an existing issue — only create here):
   - `title` — the issue title.
   - `team` — the resolved team name or ID.
   - `description` — the rendered body, as literal Markdown.
   - plus any optional fields the user chose.

4. **Echo the result.** Report the returned issue **identifier** (e.g. `ENG-123`) and its URL. This identifier is the "next workstream" pointer the orchestrator records on a coral/council handoff.

## Guardrails specific to this path

- **Always create, never update.** `/issue` files *new* issues. Do not pass `id` to `save_issue` — that would mutate an existing ticket. Updating/triaging existing issues is out of scope (see SKILL.md "What this skill doesn't do").
- **One issue per invocation.** Same as the GitHub path — if a session produced multiple deferred slices, the user picks which to file; don't batch-create.
- **No fabricated identifiers.** If creation fails or the MCP is unavailable, surface the failure and offer GitHub / print. A made-up `ENG-NNN` is worse than an honest "couldn't reach Linear."

## Lineage with `/design`

GitHub issues thread to designs via `#<n>`; Linear issues thread via their identifier (e.g. `ENG-123`). When a later session captures a design for a Linear-tracked workstream, the `/design` doc's `Issue:` reference should carry the Linear identifier + URL rather than a `#n`. The References section of the *issue* can also carry a `Design: <path>` line once the design lands — same bidirectional pattern as `format-spec.md` describes for GitHub, with the Linear identifier standing in for the issue number.
