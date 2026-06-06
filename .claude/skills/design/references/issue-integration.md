# Issue ↔ Design Lineage

How `/design` and `/issue` thread bidirectional lineage. The point is that someone reading EITHER artifact can navigate to the other — no detective work, no chat archaeology.

**Contents:** [Lineage primitives](#the-lineage-primitives) · [Two sources: GitHub and Linear](#two-sources-github-and-linear) · [Forward link](#forward-link-design--issue) · [Reverse link](#reverse-link-issue--design) · [When the design lands as a PR](#when-the-design-lands-as-a-pr) · [Multiple designs for one issue](#multiple-designs-for-one-issue) · [When a design is superseded](#when-a-design-is-superseded) · [Design without a source issue](#design-without-a-source-issue) · [Anti-patterns](#anti-patterns)

## The lineage primitives

**One issue can have zero or more designs.** A complex issue may need a system-tier design and a component-tier LLD. A simple issue may need none.

**One design typically has zero or one source issue.** Designs that started from explicit asks have a source issue. Pure exploratory designs (sketching an architecture for a doc-only deliverable) don't.

**No issue has a "design field" in GitHub's or Linear's metadata.** Lineage lives in the bodies — design frontmatter has the issue ref, the issue's References section has `Design: <path>` and/or `Design: <PR-URL>`.

## Two sources: GitHub and Linear

A design's source issue lives on **GitHub** or **Linear** (`/issue` files to either). The lineage shape is identical; only the ref format and the API differ:

| | GitHub | Linear |
|---|---|---|
| Ref format | `#<n>` (e.g. `#14`) | `<IDENTIFIER>` (e.g. `ENG-123`) |
| Frontmatter `Issue:` | `#<n>` | `<IDENTIFIER> — <url>` (carry the URL; the bare identifier isn't a clickable link) |
| Fetch issue body | `gh issue view <n> --json body,title,number,url` | `get_issue` MCP tool (`id: <ref>`) → read `title`, `description`, `identifier`, `url` |
| Reverse comment | `gh issue comment <n>` | `save_comment` MCP tool (`issueId: <identifier>`, `body`) |
| Reverse body edit | `gh issue edit <n> --body-file` | `save_issue` MCP tool (`id: <identifier>`, `description`) |
| Idempotency check | `gh issue view <n> --json comments` | `list_comments` MCP tool (`issueId: <identifier>`) |

Everything below applies to both; sink-specific commands are called out where they differ.

## Forward link: design → issue

When `/design` is invoked with `--issue <ref>` or from a coral session that referenced an issue:

- **Frontmatter** gets `**Issue:**` as a top-level field — `#<n>` for GitHub, `<IDENTIFIER> — <url>` for Linear (carry the URL so it's navigable). This is the canonical primitive.
- **References** section automatically includes the issue: `Issue #<n> — <title>` (GitHub) or `Issue <IDENTIFIER> — <title> (<url>)` (Linear).
- **Background** is seeded from the issue's Problem section (the user can edit further).
- **Non-goals** are seeded from the issue's Out of scope section.

The forward link is set at creation time. It's a static reference — if the issue is renamed or moved, GitHub resolves via its issue redirect; for Linear, the stored URL plus the immutable identifier keep it navigable.

## Reverse link: issue → design

After `/design` writes the file, the skill offers to update the source issue:

> Design captured at `design/milestones/seinode-mid-life-signing-key-drift-lld.md`.
>
> Update issue <ref> with the design link?
> 1. Add a comment: "Design captured: design/milestones/...lld.md"
> 2. Edit the issue body's References section to include the design path
> 3. Both
> 4. Skip — I'll update manually

**Default offer is option 1 (comment).** It's the lightest touch and doesn't require body-edit permissions or risk clobbering the issue's current state. The user can opt up to option 3 if they want the issue body to reflect the design as a permanent reference.

**Option 1 (comment):**
- **GitHub** — `gh issue comment <n> --body "Design captured: <relative-path>"`.
- **Linear** — `save_comment` (`issueId: <identifier>`, `body: "Design captured: <relative-path>"`; literal-newline Markdown).

**Option 2 (body edit):**
- **GitHub** — (1) fetch the body via `gh issue view <n> --json body -q .body`; (2) locate `## References` (append it if absent); (3) add a `- Design: <relative-path>` line if not present; (4) re-edit via `gh issue edit <n> --body-file <tmp>`.
- **Linear** — (1) fetch the `description` via `get_issue` (`id: <identifier>`); (2) locate `## References` (append if absent); (3) add the `- Design: <relative-path>` line if not present; (4) save via `save_issue` (`id: <identifier>`, `description: <updated>` — literal-newline Markdown). Passing `id` to `save_issue` is correct *here* (this is an intentional update of an existing issue, unlike `/issue`'s create path which must omit `id`).

**Idempotency:** before linking, check the right place and make no change if the design path is already present:
- *Comment path* — existing comments (GitHub: `gh issue view <n> --json comments`; Linear: `list_comments`).
- *Body/description edit path* — the body/description you fetched in step (1) above (GitHub: the `body`; Linear: the `description` from `get_issue`), not the comment list.

**Never fabricate the link.** If the comment/edit call fails or the backend (Linear MCP) is unavailable, report it and leave the lineage unthreaded — don't claim a link that wasn't written.

## When the design lands as a PR

The most common path: a design doc isn't a final artifact on its own — it lives in a PR alongside the implementation. In that case:

- The design doc path is `design/milestones/<slug>-lld.md` (or wherever).
- The PR includes the design + the code that implements it.
- The PR description references the source issue (standard `Closes #<n>`).
- The issue, after merge, has the merged PR auto-linked by GitHub.

The design's reverse link to the issue then goes through the PR. The design itself only needs the forward link (`Issue: #n` in frontmatter) — the issue's reverse link to the design is implied by the PR linkage.

If the design is shipping ahead of implementation (design lands in its own PR), explicit reverse linking via comment matters more.

**Linear caveat.** The auto-linking above is GitHub-specific: GitHub resolves `Closes #<n>` and back-links the merged PR on the issue for free. Linear has no `Closes #<n>` equivalent from a GitHub PR unless Linear's GitHub integration is configured (magic words / branch naming wired to the workspace). So for a Linear-tracked design, **don't assume the PR threads the lineage** — the explicit reverse link (comment via `save_comment`, or the description edit) is the primary thread, not an optional nicety. Offer it even when the design lands in a PR.

## Multiple designs for one issue

A complex issue may produce a system-tier design and one or more component-tier LLDs. The lineage handles this naturally:

- Each design's frontmatter has `Issue: #<n>`.
- Each design's References section can also link to the system-tier design (the parent), if one exists.
- The issue's References section accumulates `- Design: <path>` lines.

When an issue accumulates multiple design references, they should appear in design-tier order: system tier first, then component tier. The `/design` skill doesn't enforce order; the user can edit the References section to reflect the right hierarchy.

## When a design is superseded

If a design gets replaced by a new design (e.g. v1 LLD → v2 LLD because a constraint changed), the old design's status changes to:

```
**Status:** Superseded by [<new design>](path/to/new-design-lld.md)
```

This is a manual edit; `/design` doesn't manage status transitions. But a new design that supersedes an old one should:

- Add the old design path to its own References section as `Superseded design: <old-path>`.
- Optionally edit the old design's status header.

For issue lineage: the new design inherits the same `Issue: #n` reference. Both designs remain linked to the source issue; the chain is reconstructible by anyone reading either.

## Design without a source issue

Standalone designs (no `--issue` flag, no coral session that referenced an issue) skip the frontmatter `Issue:` field entirely. The skill should NOT prompt for an issue or fabricate one.

If the user later decides to file an issue retroactively, they can:
1. Run `/issue` to create the issue, referencing the design in its References section.
2. Manually edit the design's frontmatter to add `Issue: #n`.

`/design` doesn't auto-file the issue — that's the user's call.

## Anti-patterns

- **Don't comment on the issue without offering.** Auto-comments are noisy if the user already commented or doesn't want the design linked yet.
- **Don't deep-edit the issue body.** Only the References section is safe to update programmatically. Anything else risks clobbering the user's state.
- **Don't fabricate the issue ref if it's missing.** If a coral session didn't reference an issue, the design has no source issue. Don't ask the user to assign one — that's a separate decision.
- **Don't update issue status / labels / milestone.** The design landing doesn't change the issue's state. Triagers (and the user) decide that.
