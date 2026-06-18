# In-source comment discipline

`idiomatic-reviewer` is the **champion-of-record** for the in-source comment axis — code comments and config-file (e.g. YAML) annotations. This file defines the standard, is the tie-break when it's in question, and the reviewer enforces it on every xreview cycle. The adjacent doc/prose axis — design docs, READMEs, and file/package **header docs** as narrative — is owned by `prose-steward` via `/lingua`; see "Boundary" below.

## The standard

Comments and docs serve BOTH the human who scans and the agent that ingests linearly. Four rules:

1. **Describe the HERE AND NOW — present state only.** A comment describes what the code IS, now. Never a change, history, or why-something-was-removed. "We dropped X because Y", "this used to…", "PLT-NNN: drop it so…" belong in the PR body / commit message (change control), NOT in source. To an agent reading linearly, "we removed the X path" reads as a *current instruction*, which is worse than stale.
2. **Sparingly.** Comments scattered through an implementation are themselves the smell — a sign there's no strategy for managing the code's documentation. Default to none.
3. **At the TOP, not in the body.** Go → package doc or the type/func doc comment. Other languages → top of the class/file. Config (YAML) → a file-header doc block, never per-line annotations.
4. **Comprehensive context → ONE centralized package-level doc** (dense, cohesive), not fragmented inline across the body.

Per-language idiomatic packs **specialize** this standard with language-specific lint anchors and examples — e.g. the Go pack's **D10** (`references/language-pack-go.md`) carries the `ST1000` / `ST1020`–`ST1022` / gocritic `commentedOutCode` anchors and the `doc.go` rule. This file is the **language-agnostic parent**; the packs are its specializations and must not diverge from it.

## Decision procedure — run before writing or keeping any comment

- **(a)** About a CHANGE, why-it-changed, or what-was-removed → PR/commit, not source.
- **(b)** Durable present-state context the reader needs → top of file / centralized package doc, sparingly.
- **(c)** A regression guard ("don't re-add X") → a CI lint or test, not a comment.
- **(d)** Default → no comment.

## What the reviewer flags

| Smell in the diff | Finding | Belongs instead |
|---|---|---|
| `// we dropped / used to / previously / PLT-NNN: removed` | history-in-source | PR/commit body |
| `// don't re-add this` / `// keep or X breaks` | regression-guard-as-comment | CI lint or test (c) |
| body comment restating the next line's name | narration | delete (d) |
| per-line YAML annotations | inline-config-narration | file-header doc block (rule 3) |
| same context repeated in 3+ inline spots | fragmented context | one package-level doc (rule 4) |
| any comment where none is needed | over-commenting | delete (rule 2/d) |

A clean diff with zero or one well-placed top-of-file comment reads native — say so, no finding. Do not manufacture a comment-discipline nit to look thorough.

## Boundary — in-source comments vs. doc artifacts

This axis owns **whether an in-source comment should exist and where it sits** — including the placement/existence of a top-of-file package/type doc and config-field annotations. The **narrative prose quality** of a header doc, README, guide, or design doc is `prose-steward`'s via `/lingua`. The **canonical boundary table** — full axis/owner/surface split, the header-doc aspect rule, and dispute resolution — lives in `/lingua` `references/audience-model.md`; this section points there rather than restating it.
