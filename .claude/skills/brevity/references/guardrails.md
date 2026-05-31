# Brevity Skill — Detailed Safety Model

Extended version of the Guardrails section in SKILL.md. This file is the canonical reference for what the skill will and won't do, and why.

## Authorship boundary

This skill operates on **agent-authored** natural-language output. The user owns human-authored words.

When a user invokes the skill on content they wrote themselves, the skill:
1. Surfaces the authorship distinction.
2. Asks: "Editorial suggestions, or a full rewrite?"
3. Defaults to suggestions (advisory diff) — the user has to explicitly opt in to rewrite.

## Surface boundary

Today's MVP surfaces: PR descriptions, in-code comments.

**Deferred surfaces and their un-defer triggers:**

| Surface | Why deferred | Un-defer trigger |
|---|---|---|
| Doc / design docs | They're already rare; review can catch verbosity there | First design doc that ships with measurable filler that survived review |
| Runbooks | Ops hasn't complained about verbose runbooks yet | First on-call complaint about a runbook being too long to scan |
| Memory writes | Current entries are already terse (~5 lines each) | First memory entry over 20 lines, or first time memory grows past 2k words total |
| Chat output mid-conversation | `feedback_authoritative_voice` covers this softly | First user complaint about post-fact narration in chat |
| Commit messages | Conventional Commits convention handles | First Conventional Commit body that runs over 5 lines |

## Safety-critical content

Sections that compression NEVER strips without explicit user consent:

- `## Safety`
- `## Rollback`
- `## Blast radius`
- `## Migration`
- `## Breaking change`
- Anything tagged `<!-- brevity: keep -->` or surrounded by `<!-- safety -->...<!-- /safety -->`

The skill applies brevity rules within these sections (cutting filler) but never removes load-bearing operational content. When in doubt, halt and ask.

## Mechanical-constraint refusal

Brevity is judgment-laden. If the user asks for:
- "Ban these words"
- "Auto-replace all instances of X with Y"
- "Run a regex over the PR body"

...the skill refuses and surfaces lint tools as the right home:
- [write-good](https://github.com/btford/write-good) — Node CLI for weasel-word patterns
- [proselint](https://github.com/amperser/proselint) — Python style linter
- Custom CI step with regex — embed in `.github/workflows/` if you want hard enforcement

## Cross-author boundary

The skill will NOT compress someone else's PR body without explicit user invocation that:
1. Names the target PR (URL or owner/repo#N).
2. States the relationship (reviewer suggestion vs. author handoff).
3. Routes the output as a suggestion, not a direct edit.

This is a courtesy + signal-quality boundary, not a permission check — humans own their words even when those humans are teammates.

## Floor and ceiling

**Floor (refuse to compress below)**:
- PR bodies under 50 words.
- Single-line comments.
- Sections explicitly tagged as safety-critical.

**Ceiling (refuse to expand above)**: not applicable — this skill never adds words, only removes.

## Halt-and-litigate

If the user pushes back on three or more rule applications in a single session, halt and surface:

> "We're litigating the rules rather than applying them. Want to redirect to a specific surface or skip the skill on this one?"

This prevents the skill from becoming a back-and-forth disagreement-machine and gives the user a clean exit.

## Garbled input

If the load-bearing claim of a PR body is unclear after one careful read, the skill suggests:

> "I can't identify the load-bearing claim. Rewriting from scratch is likely faster than compressing this. Want a fresh draft based on the diff?"

Don't attempt compression on garbled input — output will be both shorter AND lossier.
