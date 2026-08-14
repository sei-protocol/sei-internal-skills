# Design Document Format

The canonical body produced by `/design`. Section order is fixed; empty optional sections are omitted entirely (no placeholder headers). The shape is ADR-flavored — opinionated about what gets captured, light on bureaucracy.

## Frontmatter — required

```markdown
# Design: <Title>

**Status:** Draft
**Date:** YYYY-MM-DD
**Issue:** <ref>   (`#14` for a GitHub source, or `ENG-123 — https://linear.app/...` for a Linear source; omit the line if no source issue)
**Authors:** <git user.name>, ...
```

- **Status** lifecycle: `Draft` → `Under review` → `Accepted` → `Superseded by <new design path>`. The skill writes `Draft`; updates are manual.
- **Date** is the date the design was first written. Not updated on revision (use git history for that).
- **Issue** is the lineage primitive. If the design started from `/issue` or a coral/council session that referenced an issue, capture the ref here — `#<n>` for a GitHub issue, or `<IDENTIFIER> — <url>` for a Linear issue (carry the URL; the bare identifier isn't clickable). The skill auto-fills when invoked with `--issue <ref>`.
- **Authors** defaults to the local git user; user can add collaborators (people who weighed in during the design session).

## Body sections (in order)

### `## Background` — required

Why does this design exist? What's in the world today that motivates it? Keep it factual, not aspirational — describe the system as it is, the constraint or gap that prompted the design, and any prior work the design builds on.

If the source is a `/issue` body, the issue's **Problem** section is the starting point. The design pass may add deeper context (architecture details, code references) the issue didn't have.

One to three paragraphs is right. If it's longer than that, the Background is doing the Design's job.

### `## Goals` — required

Bulleted list. Each item is a desired outcome — observable, ideally testable. Things the design is trying to *achieve*, not how it's going to do it.

```markdown
- Mid-life SigningKey patches on Running validators trigger a controlled pod re-roll
- Existing single-shot deployment flow continues to work unchanged
- Drift detection is reusable for future spec-mutation cases (mode switch, etc.)
```

If a goal can't be expressed as an outcome, it's probably a non-goal or a design choice — move it elsewhere.

### `## Acceptance criteria` — optional

Bulleted list of **falsifiable, observable** conditions the implementation must satisfy. Where Goals are *prose intent* (what the design tries to achieve), acceptance criteria are the *falsifiable checks* a reviewer or test can hold the implementation against — the formalized success contract. Two allowed forms (mix freely):

- **EARS-style** (encouraged for behavioral): `WHEN <trigger> THE SYSTEM SHALL <observable behavior>` (or `IF <condition>`/`WHILE <state>` variants). Use `SHALL` only for a criterion the design has actually **settled** — an unsettled "we should probably…" stays an Open question, never a hardened `SHALL` (a `/language` pass immediately *before* the `design-approval` checkpoint guards this: promoting an undecided modal into a normative criterion is drift born at authoring).
- **Falsifiable checklist** (for structural / non-behavioral): each item a check a reviewer or test can confirm true/false.

```markdown
- WHEN a SigningKey patch lands on a Running validator THE SYSTEM SHALL trigger exactly one controlled pod re-roll.
- The existing single-shot deployment flow passes its current test suite unchanged.
- Drift detection is invoked from the mode-switch path without modification (reuse, not fork).
```

**Optional, and earned — not boilerplate.** Fill this only when the design has implementation that will be *verified against it*; skip it for pure-decision / discovery designs (criteria that restate Goals are noise — cut them). At authoring, run the **traceability self-check**: *for each criterion, can you point to where it will be satisfied — a task, a test, a PR? List any gaps.* A criterion no work will satisfy is either premature or a gap to surface, not a line to leave dangling.

(Acceptance criteria are the artifact a `/workstream`'s pre-merge drift check reads — see the workstream lifecycle. The criteria are not a hidden one-way door: they are a checkable contract, editable as the design evolves.)

### `## Non-goals` — optional but strongly encouraged

Bulleted list. Each item is something the design **won't** address, with brief reason. This is high-value content — it prevents reviewers from asking about those things and protects the design from scope creep during implementation.

If the source is a `/issue` body, the issue's **Out of scope** section is the starting point.

```markdown
- **Mode switch (full-node → validator).** Same drift-detection mechanic, larger surface; out of scope for v1.
- **Demoting a validator to non-signing.** Workflow is unsupported per LLD §11.
```

### `## Design` — required

The meat. Prose + diagrams + code/file references. This is where mermaid lives.

Subsections are encouraged when the design has multiple parts. Keep them descriptive (`### Drift detection`, `### Re-apply plan`), not generic (`### Approach`, `### Implementation`).

**Mermaid usage** — see `mermaid-patterns.md` for snippets. Include diagrams when:
- An interaction across components matters → sequence diagram
- A decision tree or state-conditional path matters → flowchart
- An object goes through lifecycle states → state diagram

Reference code with `path/to/file.go:LineNumber` so reviewers can click through. Inline code blocks for short snippets; link out to PRs/branches for longer context.

### `## Alternatives considered` — optional but encouraged

Bulleted list. Each item: the alternative, why it was considered, why it wasn't chosen. The point is to capture *reasoning*, not just options — six months later, this is what tells a future engineer why the chosen path was right (or not).

```markdown
- **Alt A: Add a dedicated reconciler.** Considered because it cleanly separates drift from initial deploy. Not chosen because it duplicates the StatefulSet rollout watcher already in `buildNodeUpdatePlan`.
- **Alt B: Polling instead of plan-driven.** Simpler. Not chosen because it doesn't fit the existing reconciliation model and would race with image-drift logic.
```

If the design pass weighed exactly one option, this section can be omitted. But usually there were alternatives — capture them.

### `## Trade-offs` — optional but encouraged

Bulleted list. Each item: a known cost or risk of the chosen design, with what it buys you. Honest about what you're giving up.

```markdown
- **Re-apply triggers a full StatefulSet rolling update**, not a surgical container restart. Costs ~30s of unavailability per rolling-update pass. Buys: reuse of existing rollout machinery; no new failure modes.
- **Drift detection is per-field, not generic.** Adding a new mutable spec field requires a new drift-detection branch. Buys: explicit, audit-able set of supported drift cases.
```

### `## Open questions` — optional

Bulleted list. Each item: an unresolved question, with an owner if known. These are questions that would change the design if answered differently — not implementation details that can be figured out during build.

```markdown
- Should validation block the StatefulSet apply, or proceed and surface the error in status? — owner: kubernetes-specialist, decided during PR
- Do we need a `Status.LastDriftDetectedAt` for observability? — owner: opentelemetry-expert
```

### `## References` — optional

Bulleted list with descriptive labels.

```markdown
- Issue #14 — original drift-detection ask
- PR #135 — merged LLD for SigningKey
- PR #136 — single-shot deployment that motivated this drift work
- [LLD §11 — deferred entry](https://github.com/.../docs/foo.md#11-deferred)
- `.sei-internal-skills/validator-migration.md` — internal runbook
```

If the design has a source issue, the issue is automatically referenced here in addition to the frontmatter.

## Slug naming

Convert the title to kebab-case for the filename:

- `"Detect spec drift on Running nodes for mid-life SigningKey patch"` → `detect-spec-drift-running-nodes-mid-life-signing-key-patch.md`

Truncate to ~60 characters. Strip articles (a, the) and stop-words. The filename is for humans skimming a directory listing.

**LLD suffix:** component-tier LLDs get `-lld.md` (detected by genre/arc).

## Output path resolution

In priority order (mirrors `SKILL.md` step 1 — keep the two in sync):

1. `--output-dir <path>` if provided.
2. **The DRI's designs repo (per Design 05).** Resolve it, in order: (a) `--designs-repo <owner/name|path>` if given; (b) a sibling `<name>-designs` checkout (a sibling dir of CWD, or the same git org's `<gh-user>-designs`); (c) otherwise **ask the user** which designs repo. Land arc-foldered as `designs/<arc>/<slug>.md` (component-tier LLDs `-lld.md`, system-tier plain); ask which arc when ambiguous.
3. Fallback — **only after step 2c and only if the user confirms they have no designs repo**: `docs/designs/` in the current repo. Never silently fall through to this when a DRI repo is the intent.

Always show the user the resolved path before writing. Never overwrite without confirmation.

## Section selection cheat-sheet

| Design type | Required | Usually included | Often omitted |
|---|---|---|---|
| LLD (component implementation) | Title, Background, Goals, Design | Non-goals, Alternatives, Trade-offs, References, **Acceptance criteria** (when the LLD will be implemented + verified) | Open questions (if review pass already happened) |
| System-tier (multi-component) | Title, Background, Goals, Design | Non-goals, Alternatives, Trade-offs, Open questions, References, **Acceptance criteria** (when implemented under a /workstream) | None — system designs benefit from all sections |
| Decision record (one-way door) | Title, Background, Goals, Design, Alternatives | Trade-offs, References | Non-goals (the decision IS the scope), Open questions |
| Sketch / exploratory | Title, Background, Design | Goals, Open questions | Non-goals, Alternatives, Trade-offs (premature) |
