# Issue ↔ Design Lineage

How `/design` and `/issue` thread bidirectional lineage. The point is that someone reading EITHER artifact can navigate to the other — no detective work, no chat archaeology.

## The lineage primitives

**One issue can have zero or more designs.** A complex issue may need a system-tier design and a component-tier LLD. A simple issue may need none.

**One design typically has zero or one source issue.** Designs that started from explicit asks have a source issue. Pure exploratory designs (sketching an architecture for a doc-only deliverable) don't.

**No issue has a "design field" in GitHub's metadata.** Lineage lives in the bodies — design frontmatter has `Issue: #n`, issue body's References section has `Design: <path>` and/or `Design: <PR-URL>`.

## Forward link: design → issue

When `/design` is invoked with `--issue <n>` or from a coral session that referenced an issue:

- **Frontmatter** gets `**Issue:** #<n>` as a top-level field. This is the canonical primitive.
- **References** section automatically includes the issue: `Issue #<n> — <title>`.
- **Background** is seeded from the issue's Problem section (the user can edit further).
- **Non-goals** are seeded from the issue's Out of scope section.

The forward link is set at creation time. It's a static reference — if the issue is renamed or closed, the link still resolves via GitHub's issue redirect.

## Reverse link: issue → design

After `/design` writes the file, the skill offers to update the source issue:

> Design captured at `design/milestones/seinode-mid-life-signing-key-drift-lld.md`.
>
> Update issue #14 with the design link?
> 1. Add a comment: "Design captured: design/milestones/...lld.md"
> 2. Edit the issue body's References section to include the design path
> 3. Both
> 4. Skip — I'll update manually

**Default offer is option 1 (comment).** It's the lightest touch and doesn't require body-edit permissions or risk clobbering the issue's current state. The user can opt up to option 3 if they want the issue body to reflect the design as a permanent reference.

For option 2 (body edit), the skill:

1. Fetches the current issue body via `gh issue view <n> --json body -q .body`.
2. Locates the `## References` section. If it doesn't exist, appends it at the end.
3. Adds a `- Design: <relative-path>` line if not already present.
4. Re-edits via `gh issue edit <n> --body-file <tmp>`.

Idempotent: if the design path is already in References, no change is made.

## When the design lands as a PR

The most common path: a design doc isn't a final artifact on its own — it lives in a PR alongside the implementation. In that case:

- The design doc path is `design/milestones/<slug>-lld.md` (or wherever).
- The PR includes the design + the code that implements it.
- The PR description references the source issue (standard `Closes #<n>`).
- The issue, after merge, has the merged PR auto-linked by GitHub.

The design's reverse link to the issue then goes through the PR. The design itself only needs the forward link (`Issue: #n` in frontmatter) — the issue's reverse link to the design is implied by the PR linkage.

If the design is shipping ahead of implementation (design lands in its own PR), explicit reverse linking via comment matters more.

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
