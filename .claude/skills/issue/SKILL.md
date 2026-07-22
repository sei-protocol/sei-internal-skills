---
name: issue
category: workstream-bootstrap
model: claude-opus-4-8
description: "Use when synthesizing the current Claude Code session — typically a /coral or /council workstream — into a standard-format issue that bootstraps the next pickup, filed to GitHub or Linear. Trigger on 'file this as an issue', 'capture this in an issue', 'turn this into an issue', 'open an issue for the follow-up', 'file this in Linear', 'create a Linear ticket', 'track this as a Linear issue', 'bootstrap this as a workstream', '/issue'. Coral and council offer this at handoff moments (deferred slice, end of session, scope cut); the sink (GitHub or Linear, or print) is chosen at the create step. Anti-triggers: NOT for pull requests (use `gh pr create`); NOT for triaging or commenting on existing issues; NOT for capturing this work's design (use /design); NOT for in-conversation TODOs that won't be tracked across sessions (use TaskCreate)."
---

# Issue

Captures the current session's context — design notes, deferred slices, follow-up work — as a standard-format issue, filed to **GitHub or Linear**. The point is to make the *next* pickup trivial: whoever opens the issue gets the same picture the original session had, in the same shape every time.

The shape is fixed: Problem, Impact, Relevant experts, Proposed approach (optional), Acceptance criteria (optional), Out of scope (optional), References (optional). See `references/format-spec.md`. The rendered body is **sink-agnostic** — the same markdown becomes a GitHub issue body or a Linear issue description. The skill asks which sink (or print) at the create step; it does not assume one.

## Guardrails

`/issue` files *new* issues from synthesized session context. Before any create action:

1. **Never auto-create; never silently infer the sink.** Always render the body before creating. The sink is the user's *explicit* choice: if the invocation already named one ("file this in Linear", "open a GitHub issue"), honor it without re-asking; otherwise ask GitHub / Linear / print. What's forbidden is *inferring* a sink from context (the repo, a prior issue) or creating before the body is shown — not honoring a sink the user stated.
2. **Never fabricate.** Don't invent issue URLs or Linear identifiers, and don't fill required fields with `_TBD_` placeholders. If a field has no signal, omit it (when optional) or ask (when required).
3. **Problem is behavior, not fix.** Refuse to write the proposed solution into the Problem field — push back and capture observable behavior. The fix, if any, belongs in Proposed approach.
4. **Create, never update.** This skill opens new issues. Don't pass `id` to Linear `save_issue` (that mutates an existing ticket); triaging or editing existing issues is out of scope.
5. **Degrade, don't block.** If the chosen sink is unavailable (`gh` not authenticated; Linear MCP absent in a headless run), say so and offer another sink or print — never silently switch sinks or fail opaquely.

## Two invocation modes

### 1. Coral / council handoff (primary)

The orchestrator already has rich context: the original ask, what specialists said, what got cut for YAGNI, what's deferred. At a handoff moment, the orchestrator offers to file the deferred slice (or the next workstream) as an issue.

**When the orchestrator should offer this skill:**

- A specialist's output included a "deferred — when X" line that maps to follow-up work.
- The user explicitly cut scope ("not now, but file it") during synthesis.
- End of a coral session and there's clearly a "phase 2" the user will pick up later.
- A one-way door surfaces in coral (the door isn't for now, but the *next* pickup needs the design captured).
- Council closes a workstream and there's a sibling workstream identified but not started.

**The orchestrator passes synthesized context.** Don't re-prompt the user for fields the session already established. Pre-fill: Problem (from the deferred slice's framing), Impact (from why it was cut, not just deferred), Relevant experts (from who was on the workstream + who'd own the next slice), Proposed approach (from the specialist sketches that produced the cut), Out of scope (from what *this* skill explicitly won't touch).

The user reviews and confirms; they don't re-author from scratch.

See `references/coral-integration.md` for the full handoff shape and example.

### 2. Standalone (secondary)

Direct user invocation when there's no active workstream — e.g., a teammate spotted a bug while reading code and wants to file it, or a runbook flagged something that should be tracked. Procedure follows below; the orchestrator-handoff path uses the same procedure but skips the prompts that the session already answered.

## Preconditions

Preconditions are checked per sink, at the create step — not upfront. The body is always drafted first; if the chosen sink isn't available, the skill prints the body for paste rather than blocking.

- **GitHub sink:** `gh` CLI installed and authenticated for the target repo's org; CWD is a git repo OR the user passed `--repo owner/name`. Default target is the current repo (i.e. sei-internal-skills for teammates working in this checkout).
- **Linear sink:** the Linear MCP tools (`mcp__…__list_teams`, `…__save_issue`, `…__list_issue_labels`, `…__create_issue_label`, etc.) are connected and authenticated. These are interactively-authenticated — they may be absent in headless / cron runs. If unavailable, say so and fall back to GitHub or print; never fabricate a Linear URL. The optional Impact-bet decoration additionally uses the Notion MCP to resolve the bet; skip the decoration (don't fail the filing) if Notion is unavailable.

## Procedure

1. **Resolve the GitHub target repo** (used only if the GitHub sink is chosen in step 5 — the Linear team is resolved later, in step 6's Linear path).
   - If invoked from coral/council, default to the repo of the current workstream (almost always CWD).
   - Else: `gh repo view --json nameWithOwner -q .nameWithOwner` from CWD.
   - Override with `--repo owner/name` if the issue belongs to a sibling repo (e.g. filing a downstream task in `sei-protocol/platform` from a sei-internal-skills session).

2. **Gather inputs.** Required fields: **Title**, **Problem**, **Impact**, **Relevant experts**.
   - **Coral handoff path:** the orchestrator pre-fills these from session context. Show the user the pre-fill and ask for adjustments — don't re-prompt fields the session already answered.
   - **Standalone path:** prompt the user for each. Push back on framings like "do X" — the Problem field is observable behavior, not the proposed fix.

   Optional: **Proposed approach**, **Acceptance criteria**, **Out of scope**, **References**. Offer them; don't force them.

3. **Suggest experts.** Read `.claude/agents/*.md` if accessible (CWD or via `gh api`). Suggest 1–3 personas based on what the issue touches. Confirm with the user. See `references/expert-routing.md`.

4. **Render the body.** Use the section order in `references/format-spec.md`. Skip empty optional sections rather than emitting placeholder headers.

5. **Show the rendered body and resolve the sink.** Always show the body before creating. Then:

   - **If the user's invocation already named a sink** ("file this in Linear", "open a GitHub issue", "just print it"), honor it — don't re-ask. State which sink you're using so it's visible.
   - **Otherwise ask:** "File as a **GitHub** issue, a **Linear** ticket, or **print** for paste?"

   Never infer the sink silently from context (the repo, a prior issue) — an unstated sink is always asked, never guessed. A **weak cue is not a named sink**: "the last few went to Linear", "this is a Platform thing", or the repo's history do NOT count — only an explicit instruction in *this* request ("file this in Linear") does. When in doubt, ask. The same rendered body is used whichever sink is chosen.

6. **File or print, per sink.**
   - **GitHub:** if the work advances an Impact Hub bet, offer (before creating) to add an `Impact bet: <Name> — <url>` line to the body's **References** — the GitHub analog of the Linear label, since GitHub carries no Linear label. Notion-resolve the bet (Person-scoped, user picks, never guess); skip if Notion is unavailable. This is a human / `/design`-inheritance breadcrumb, **not** part of the deterministic label spine (only Linear-filed work joins that). Then `gh issue create --repo <target> --title "<title>" --body-file <tmp>`. Echo the resulting issue URL.
   - **Linear:** resolve the team, then create. See `references/linear-integration.md` for the full path. In short: list teams (`list_teams`) and have the user pick one — never guess the team; optionally offer project / labels / priority (offer, don't force); create with `save_issue` (`title`, `team`, `description` = the rendered body as Markdown — pass literal newlines, not escaped sequences). **If the work advances an Impact Hub bet, decorate the new issue via `/execution-plan`** — don't re-implement label logic here: call its `ensurePlan` (resolve the bet, ensure the `impact:<slug>` label — first label creation is confirm-gated) then `stamp` (apply the label + the bet's design-URL link, idempotent). `/execution-plan` owns identity (the bet's Notion page-ID), the label cache, and the slug rule, so the spine stays single-homed. Echo the returned issue **identifier** (e.g. `ENG-123`) and URL.
   - **Print:** emit the body in a fenced markdown block.

## What this skill doesn't do

- **Triage or prioritize.** It files; humans (and `coral` / `council`) decide what to do with it.
- **Cross-repo federation.** When a problem spans repos, file separate linked issues — one invocation per repo.
- **Redact sensitive content.** The filer is responsible for keeping secrets, internal URLs, and customer data out of the body.
- **xreview the proposed approach.** If the filer wants design feedback before filing, they should iterate in coral first; the skill captures whatever sketch is on the table.

## Halt Conditions

Stop and surface to the user if:

- The required fields (Title, Problem, Impact, Relevant experts) can't be established from session context or user input — ask; don't fabricate them.
- The chosen sink is unavailable (`gh` unauthenticated, or the Linear MCP not connected) — report it and offer another sink or print. Never invent a URL or Linear identifier.
- A user-named Linear team isn't in `list_teams` — present the actual list and let the user pick; never file into a fallback/first team.
- The Problem can only be stated as the proposed fix — push back for observable behavior before filing.
- The session produced multiple deferred slices — ask which to file; don't batch-create reflexively.

## Output

End-of-turn summary: one line — the GitHub issue URL, the Linear identifier + URL (e.g. `ENG-123 — https://linear.app/...`), or "drafted, ready for paste" — plus the title. If invoked from a coral/council handoff, the orchestrator should record that pointer (URL or Linear identifier) in its session summary as the "next workstream" pointer.
