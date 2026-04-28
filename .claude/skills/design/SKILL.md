---
name: design
description: "Capture a design as a structured markdown document under docs/designs/ (or the repo's design folder). Companion to /issue — when an issue gets picked up and a coral/council session produces a design, /design captures the artifact and threads bidirectional lineage back to the source issue. Trigger on 'design this', 'capture this design', 'write up the design', 'design doc for issue #X', 'turn this into a design', '/design'. Coral and council should offer this skill when the deliverable IS a design (LLD, architecture sketch, system design) — distinct from /issue which captures NEXT work. Produces an ADR-flavored body (Background / Goals / Non-goals / Design / Alternatives / Trade-offs / Open questions / References) with mermaid diagrams encouraged. Anti-triggers: NOT for filing a new issue (use /issue); NOT for code review write-ups; NOT for postmortems."
---

# Design

Captures a design — LLD, architecture sketch, system design pass — as a structured markdown document. The point is to make design context durable: the doc is the artifact someone reviews, references in PRs, and rediscovers six months later.

The shape is fixed: Background, Goals, Non-goals, Design (with mermaid diagrams), Alternatives, Trade-offs, Open questions, References. See `references/format-spec.md`.

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

### 2. From an issue (`/design --issue <n>`)

When an issue (typically filed via `/issue`) gets picked up and a design pass runs, invoke `/design --issue 14` to seed the doc from the issue's body:

- Issue **Problem** → design **Background**
- Issue **Out of scope** → design **Non-goals** (starting point; the design pass may refine)
- Issue **Proposed approach** → design **Design** (starting point; specialists likely expanded it)
- Issue **References** → design **References** (plus the issue itself)
- Issue **#n** → design frontmatter `Issue: #n` (the lineage primitive)

The orchestrator (or the user) fills in what the design session produced — Goals, refined Design, Alternatives, Trade-offs, Open questions.

After the design lands, offer to comment on the issue with the design path. See `references/issue-integration.md`.

### 3. Standalone

Direct user invocation when there's no active workstream or upstream issue — e.g., a designer is sketching an architecture for a doc-only deliverable. The procedure prompts for each section.

## Preconditions

- A markdown-friendly editor or pager (the skill writes a file; review happens in your editor of choice).
- For issue-mode and lineage features: `gh` CLI installed and authenticated.
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
   - `--issue <n>` → fetch the issue body via `gh issue view <n> --json body,title,number,url -q '...'` and pre-fill from it (mode 2).
   - Coral/council handoff with synthesized context → use that context (mode 1).
   - Otherwise → prompt for each section (mode 3).

3. **Gather inputs.** Required: **Title**, **Background**, **Goals**, **Design**. Optional: **Non-goals**, **Alternatives**, **Trade-offs**, **Open questions**, **References**, **Status** (defaults to `Draft`), **Authors** (defaults to git user.name).

   - **Coral handoff path:** show the pre-fill, take adjustments. Don't re-prompt fields the session answered.
   - **From-issue path:** show what was inherited from the issue, take adjustments, prompt for the design-specific sections (Goals, Alternatives, Trade-offs, Open questions).
   - **Standalone path:** prompt for each. Push back on framings like "design X" without context — Background should answer "why does this design exist?"

4. **Mermaid diagrams.** During the **Design** section, identify candidate diagrams from the synthesized context:
   - **Sequence** — when an interaction across components matters (request flow, handoff order).
   - **Flowchart** — when a decision tree or state-conditional path matters.
   - **State** — when an object goes through lifecycle states.
   - **ERD-like flowchart** — when storage layout or data shape matters.

   Generate plausible mermaid based on session context. Mark each diagram with a comment: `<!-- verify this matches your intent -->`. The user reviews and adjusts before finalizing.

   See `references/mermaid-patterns.md` for snippets.

5. **Resolve slug.** Convert title to kebab-case for the filename. Repo-specific suffix conventions: Tide LLDs use `<slug>-lld.md`. Default: `<slug>.md`. Show the user the resolved path before writing.

6. **Render the body.** Use the section order in `references/format-spec.md`. Skip empty optional sections rather than emitting placeholder headers. Frontmatter (Status / Date / Issue / Authors) is required.

7. **Show the rendered body.** Ask: "Write to `<path>`?" Don't auto-write.

8. **Write the file.** Confirm the path; create parent directories if needed.

9. **Issue lineage (if --issue mode or coral session referenced an issue).** Offer to comment on the issue:
   ```
   Design captured: <relative-path-from-repo-root>
   ```
   See `references/issue-integration.md` for the full lineage flow including bidirectional updates.

## What this skill doesn't do

- **Cross-review the design.** That's coral/council's job *before* the design is captured. `/design` is the recording step.
- **Maintain status across the doc's lifetime.** It writes Draft initially. Updates to "Under review", "Accepted", "Superseded" are manual edits to the file.
- **Cross-repo federation.** A design that spans repos lives in the load-bearing repo (where the work happens); link out from there.
- **Replace existing repo conventions.** Tide already has `design/milestones/` for LLDs and `design/high-level/` for higher-level designs. The skill respects existing conventions when detected.

## Output

End-of-turn summary: one line — design path written, plus the title and (if applicable) the issue # the design is threaded to. If invoked from a coral/council session, the orchestrator should record the design path in its session summary.
