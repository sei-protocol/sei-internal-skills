---
name: issue
description: "Synthesize the current Claude Code session — typically a /coral or /council workstream — into a standard-format GitHub issue that bootstraps the next pickup. Trigger on 'file this as an issue', 'capture this in an issue', 'turn this into an issue', 'open an issue for the follow-up', 'bootstrap this as a workstream', '/issue'. Coral and council should offer this skill at handoff moments (deferred slice, end of session, scope cut). Produces a body with Problem / Impact / Relevant experts / Proposed approach / Acceptance criteria / Out of scope / References, then offers to call `gh issue create`. Anti-triggers: NOT for pull requests (use `gh pr create`); NOT for triaging or commenting on existing issues; NOT for in-conversation TODOs that won't be tracked across sessions (use TaskCreate)."
---

# Issue

Captures the current session's context — design notes, deferred slices, follow-up work — as a standard-format GitHub issue. The point is to make the *next* pickup trivial: whoever opens the issue gets the same picture the original session had, in the same shape every time.

The shape is fixed: Problem, Impact, Relevant experts, Proposed approach (optional), Acceptance criteria (optional), Out of scope (optional), References (optional). See `references/format-spec.md`.

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

- `gh` CLI installed and authenticated for the target repo's org. If not, the skill drafts the body and prints it for paste — it does not block.
- CWD is a git repo OR the user passed `--repo owner/name`. Default behavior is to file against the current repo (i.e. Tide for teammates working in this checkout).

## Procedure

1. **Resolve target repo.**
   - If invoked from coral/council, default to the repo of the current workstream (almost always CWD).
   - Else: `gh repo view --json nameWithOwner -q .nameWithOwner` from CWD.
   - Override with `--repo owner/name` if the issue belongs to a sibling repo (e.g. filing a downstream task in `sei-protocol/platform` from a Tide session).

2. **Gather inputs.** Required fields: **Title**, **Problem**, **Impact**, **Relevant experts**.
   - **Coral handoff path:** the orchestrator pre-fills these from session context. Show the user the pre-fill and ask for adjustments — don't re-prompt fields the session already answered.
   - **Standalone path:** prompt the user for each. Push back on framings like "do X" — the Problem field is observable behavior, not the proposed fix.

   Optional: **Proposed approach**, **Acceptance criteria**, **Out of scope**, **References**. Offer them; don't force them.

3. **Suggest experts.** Read `.claude/agents/*.md` if accessible (CWD or via `gh api`). Suggest 1–3 personas based on what the issue touches. Confirm with the user. See `references/expert-routing.md`.

4. **Render the body.** Use the section order in `references/format-spec.md`. Skip empty optional sections rather than emitting placeholder headers.

5. **Show the rendered body.** Ask: "File via `gh issue create`, or print for paste?" Don't auto-create.

6. **File or print.**
   - On confirm: `gh issue create --repo <target> --title "<title>" --body-file <tmp>`. Echo the resulting issue URL.
   - On print: emit the body in a fenced markdown block.

## What this skill doesn't do

- **Triage or prioritize.** It files; humans (and `coral` / `council`) decide what to do with it.
- **Cross-repo federation.** When a problem spans repos, file separate linked issues — one invocation per repo.
- **Redact sensitive content.** The filer is responsible for keeping secrets, internal URLs, and customer data out of the body.
- **Cross-review the proposed approach.** If the filer wants design feedback before filing, they should iterate in coral first; the skill captures whatever sketch is on the table.

## Output

End-of-turn summary: one line — issue URL (if filed) or "drafted, ready for paste" — plus the title. If invoked from a coral/council handoff, the orchestrator should record the issue URL in its session summary as the "next workstream" pointer.
