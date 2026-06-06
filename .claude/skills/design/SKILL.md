---
name: design
description: "Use when a coral/council session, an issue pickup, or a standalone authoring pass produces a design (LLD, architecture sketch, system-tier decision) that should land as a durable markdown doc — 'design this', 'capture this design', 'write up the design', 'design doc for issue #X', 'design doc for ENG-123', 'turn this into a design', '/design'. Seeds from a source issue on GitHub (`--issue 14`) or Linear (`--issue ENG-123`) and threads bidirectional lineage. Companion to /issue — captures the design while /issue captures the next workstream. Anti-triggers: NOT for filing a new issue (use /issue); NOT for code review write-ups; NOT for postmortems; NOT for cross-reviewing a design (coral/council do that BEFORE /design captures); NOT for maintaining the doc's status over time (status transitions are manual edits)."
---

# Design

Captures a design — LLD, architecture sketch, system design pass — as a structured markdown document. The point is to make design context durable: the doc is the artifact someone reviews, references in PRs, and rediscovers six months later.

The shape is fixed: Background, Goals, Non-goals, Design (with mermaid diagrams), Alternatives, Trade-offs, Open questions, References. See `references/format-spec.md`.

## Guardrails

This skill writes one markdown file (the design doc) and optionally posts one issue comment — a GitHub issue comment (`gh`) or a Linear issue comment (MCP) — as the lineage thread. Before any write:

1. **Show-before-write.** Render the full body and the resolved path; ask "Write to `<path>`?" — never auto-write, even from a coral handoff with rich pre-fill.
2. **Don't overwrite without confirmation.** If a file already exists at the resolved path, halt and ask: overwrite, suffix with a run number (`<slug>-2.md`), or abort.
3. **Refuse on incoherent inputs.** If Title, Background, Goals, or Design are empty after gathering, halt and surface what's missing — `/design` is the recording step, not a generator that fills in blanks from training data.
4. **No content cross-review.** `/design` records what the session decided; it does not push back on alternatives, propose improvements, or critique the design content. That work belongs to coral/council *before* `/design` captures.
5. **No status maintenance.** `/design` writes Status: `Draft` on first capture. Updates to "Under review", "Accepted", "Superseded" are manual edits or a separate workflow — `/design` does not schedule, remind, or auto-update.

## Three invocation modes

### 1. Coral / council handoff (primary)

The orchestrator already has rich context: the original ask, what specialists argued for, what was decided, what got cut. At a handoff moment where the deliverable IS a design (vs. a code patch or a quick answer), coral/council offers `/design` to capture it.

**When the orchestrator should offer this skill:**

- A coral session produced a design pass — multiple specialists weighed in on architecture, the synthesis is the deliverable.
- Council closed a workstream with a final LLD or system-tier design.
- The user explicitly says "let's write this up" / "we should document this design."
- A one-way door was discussed and a decision was made — that decision is worth a design doc on the record.

**The orchestrator passes synthesized context.** Don't re-prompt the user for fields the session already established. Pre-fill: Background (from the original ask), Goals (from what the session set out to achieve), Non-goals (from explicit scope cuts), Design (from the synthesized output), Alternatives (from specialist suggestions that were weighed and not taken), Trade-offs (from the YAGNI pass), Open questions (from anything the session left unresolved).

The user reviews, adjusts, confirms; they don't re-author from scratch.

See `references/coral-integration.md` for the full handoff shape.

**`/design` and `/issue` are siblings, not competitors.** Both can fire from the same session:

- `/design` captures **this** work — the synthesized design pass.
- `/issue` captures **next** work — a deferred slice, a phase 2.

Coral can offer both at the same handoff moment.

### 2. From an issue (`/design --issue <ref>`)

When an issue (typically filed via `/issue`) gets picked up and a design pass runs, invoke `/design --issue <ref>` to seed the doc from the issue's body. The `<ref>` is either a **GitHub number** (`14` or `#14`) or a **Linear identifier** (`ENG-123`) — the skill detects which (see Procedure step 2) and fetches accordingly (`gh issue view` for GitHub, the Linear `get_issue` MCP tool for Linear). The field mapping is the same for both sinks:

- Issue **Problem** → design **Background**
- Issue **Out of scope** → design **Non-goals** (starting point; the design pass may refine)
- Issue **Proposed approach** → design **Design** (starting point; specialists likely expanded it)
- Issue **References** → design **References** (plus the issue itself)
- Issue ref → design frontmatter `Issue:` (the lineage primitive — `#n` for GitHub, `<IDENTIFIER> — <url>` for Linear)

The orchestrator (or the user) fills in what the design session produced — Goals, refined Design, Alternatives, Trade-offs, Open questions.

After the design lands, offer to thread the lineage back to the issue (a GitHub or Linear comment, or a body/description edit). See `references/issue-integration.md`.

### 3. Standalone

Direct user invocation when there's no active workstream or upstream issue — e.g., a designer is sketching an architecture for a doc-only deliverable. The procedure prompts for each section.

## Preconditions

- A markdown-friendly editor or pager (the skill writes a file; review happens in your editor of choice).
- For issue-mode and lineage features: `gh` CLI installed and authenticated for a **GitHub** ref; the Linear MCP tools (`get_issue`, `save_comment`, `save_issue`, `list_comments`) connected and authenticated for a **Linear** ref. The Linear MCP is interactively-authenticated and may be absent in headless / cron runs — when a Linear ref is given and the MCP is unavailable, halt and surface it rather than fabricating issue content or a lineage link.
- CWD is the target repo (where the design will land), unless `--repo owner/name` is specified.

## Procedure

1. **Resolve target repo and output directory.**
   - Repo: CWD's git repo (or `--repo owner/name`).
   - Output dir: in priority order:
     1. `--output-dir <path>` if provided.
     2. Repo-specific convention if detected (Tide → `design/milestones/` for LLDs, `design/high-level/` for higher-level designs — ask the user which when ambiguous).
     3. Default: `docs/designs/`.
   - Create the directory if it doesn't exist; show the path to the user before writing.

2. **Resolve invocation mode.**
   - `--issue <ref>` → normalize, detect the ref type, and fetch (mode 2):
     - **Normalize first:** strip a leading `#`; if `<ref>` is a `linear.app/.../issue/<IDENTIFIER>/...` URL, extract the `<IDENTIFIER>` token. Linear identifiers are matched case-insensitively (`get_issue` resolves them either case).
     - **GitHub** — normalized `<ref>` is purely numeric (`14`). Fetch via `gh issue view <n> --json body,title,number,url -q '...'`.
     - **Linear** — normalized `<ref>` matches `^[A-Za-z][A-Za-z0-9]*-\d+$` (a team key that starts with a letter and may contain digits, then `-`, then the number — e.g. `ENG-123`, `PLA4-16916`). Fetch via the Linear `get_issue` MCP tool (`id: <ref>`); read its `title`, `description` (the issue body), `identifier`, and `url`.
     - Ambiguous or unrecognized (e.g. a cross-repo `owner/repo#n`, or a string matching neither shape) → ask the user which sink rather than guessing.
     - **Seeding from the body:** when the issue body carries the standard `/issue` sections (`## Problem`, `## Out of scope`, `## Proposed approach`, `## References`), map them per the table in mode 2. When it doesn't — a Linear-native issue written in the UI, or any issue not filed via `/issue` — **don't force the mapping**: seed **Background** from the whole body and leave Goals / Non-goals / Design for the design pass to fill. Never invent section content that isn't there.
   - Coral/council handoff with synthesized context → use that context (mode 1).
   - Otherwise → prompt for each section (mode 3).

3. **Gather inputs.** Required: **Title**, **Background**, **Goals**, **Design**. Optional: **Non-goals**, **Alternatives**, **Trade-offs**, **Open questions**, **References**, **Status** (defaults to `Draft`), **Authors** (defaults to git user.name).

   Mode-specific:
   - **Coral handoff path:** show the pre-fill, take adjustments. Don't re-prompt fields the session answered.
   - **From-issue path:** show what was inherited from the issue, take adjustments, and prompt for the design-specific sections (Goals, Alternatives, Trade-offs, Open questions). **If the issue body was free-form** (step 2's fallback seeded only Background), also prompt for **Design** — the required fields must all be filled before this step's completeness check, or capture halts per Guardrail #3. Don't leave a required field empty just because the source issue didn't provide it.
   - **Standalone path:** prompt for each. Push back on framings like "design X" without context — Background should answer "why does this design exist?"

   **Output of this step:** a complete input set with all required fields filled. If any required field is empty after gathering, halt and surface what's missing per Guardrail #3.

4. **Mermaid diagrams.** During the **Design** section, identify candidate diagrams from the synthesized context:
   - **Sequence** — when an interaction across components matters (request flow, handoff order).
   - **Flowchart** — when a decision tree or state-conditional path matters.
   - **State** — when an object goes through lifecycle states.
   - **ERD-like flowchart** — when storage layout or data shape matters.

   Generate plausible mermaid based on session context. Mark each diagram with a comment: `<!-- verify this matches your intent -->`. The user reviews and adjusts; this step is **done** when the user confirms each generated diagram (or explicitly drops it) before proceeding to the slug resolution. See `references/mermaid-patterns.md` for snippets.

5. **Resolve slug and path.** Convert title to kebab-case for the filename. Repo-specific suffix conventions: Tide LLDs use `<slug>-lld.md`. Default: `<slug>.md`. Combine with the output dir from step 1 to produce the resolved path. Show the user the resolved path before continuing.

6. **Render and show the body.** Use the section order in `references/format-spec.md`. Skip empty optional sections rather than emitting placeholder headers. Frontmatter (Status / Date / Issue / Authors) is required. **Output of this step:** the full rendered body displayed inline plus the prompt "Write to `<path>`?" — never auto-write per Guardrail #1.

7. **Write the file on confirmation.** On the user's "yes": create parent directories if needed; check for existing file at the resolved path and halt per Guardrail #2 if found; write the body.

8. **Issue lineage (if --issue mode or coral session referenced an issue).** Offer to thread the lineage back to the source issue:
   ```
   Design captured: <relative-path-from-repo-root>
   ```
   - **GitHub source** → comment via `gh issue comment` (default) or edit the References section via `gh issue edit`.
   - **Linear source** → comment via the `save_comment` MCP tool (`issueId: <identifier>`, default) or append to the References of the issue `description` via `save_issue` (`id: <identifier>`).
   Default to a comment (lightest touch); offer the body/description edit as an opt-up. See `references/issue-integration.md` for the full lineage flow including the Linear branch and idempotency.

## Halt Conditions

Stop and report rather than auto-recovering when:

- Required inputs are missing after step 3 — surface what's empty and ask the user to fill them. `/design` is the recording step; it doesn't synthesize content from training data.
- The resolved output path already exists (step 7) — show the existing file's date and ask: overwrite, suffix-with-run-number, or abort.
- `--issue <ref>` mode but the needed backend is unavailable — `gh` not installed/authenticated for a GitHub ref, or the Linear MCP not connected for a Linear ref. Halt and surface the setup needed; never fabricate the issue body or a lineage link.
- The output dir can't be created (permissions, missing parent in a non-git path) — halt and surface.
- User says "no" to the "Write to `<path>`?" prompt in step 6 — that's a valid halt. Stop, don't retry.

## What this skill doesn't do

- **Cross-review the design.** That's coral/council's job *before* the design is captured. `/design` is the recording step.
- **Maintain status across the doc's lifetime.** It writes Draft initially. Updates to "Under review", "Accepted", "Superseded" are manual edits to the file.
- **Cross-repo federation.** A design that spans repos lives in the load-bearing repo (where the work happens); link out from there.
- **Replace existing repo conventions.** Tide already has `design/milestones/` for LLDs and `design/high-level/` for higher-level designs. The skill respects existing conventions when detected.

## Output

End-of-turn summary: one line — design path written, plus the title and (if applicable) the issue # the design is threaded to. If invoked from a coral/council session, the orchestrator should record the design path in its session summary.
