# spec-kit

Authors a feature specification with GitHub Spec Kit's own CLI and vendored
templates. Owns the spec's shape and its evidence bar. Owns nothing downstream.

## The one principle

> A spec is condensed when no line in it could have been derived from the
> anchor block at the top.

Naming a method costs a few tokens. Explaining it costs paragraphs, and the
paragraphs are what make a spec unreadable. The methods are therefore named
once, and the body carries only what is specific to this feature.

## What it adds to Spec Kit

Three things. Everything else is upstream's, unchanged.

| Addition | Why |
|---|---|
| **Anchor block** | Methods named once, each with a mandatory *Does not cover* column. An anchor is a hint, and the gap is the honest part. |
| **A verifier per success criterion** | `SC-002 … Verifier: gorelease in CI`. A criterion nothing checks says `judgement`, which is a verdict rather than a wish. |
| **Four mandatory story fields** | Priority, Why this priority, Independent Test, Acceptance Scenarios. A ticket is generated from them and cannot invent what the story omitted. |

## Where it sits

| Skill | Owns |
|---|---|
| **`/spec-kit`** | `spec.md`, `plan.md`, `tasks.md` — what and why, then how |
| `/linear-ticket` | Turning the finished stories into tracker issues |
| `/project-brief` | The project's own description, not a feature's spec |
| `/design` | A spike that argues a position — a design, not a spec |

## What it refuses

- **Inventing a requirement.** An unstated detail becomes
  `[NEEDS CLARIFICATION: <question>]`. A plausible default written silently into
  a spec is the exact failure the artifact exists to prevent.
- **Implementation detail in `spec.md`.** Naming a library, schema, or signature
  moves the line to `plan.md`.
- **An unvetted anchor.** Only names a method listed in
  `references/anchors.md`. A confabulated method name reads authoritative and
  costs more than plain prose.
- **Running at all** when the diff fits in one sentence. It says so and stops.
- **Filing tickets.** That is `/linear-ticket`.

## On anchors and honesty

None of the anchors carries a recorded probe verdict yet. The reference file
says so at the top. A named method can fail three ways — the model substitutes
openly, substitutes silently, or invents — and only the first announces itself.
Until a probe exists, a surprising output is the anchor failing, not the model
disagreeing.

## Files

| Path | What |
|---|---|
| `SKILL.md` | Guardrails, preconditions, the seven-step procedure |
| `references/anchors.md` | The vetted anchor set, each with what it does not cover |
| `references/deltas.md` | The seven deltas from the vendored template, with reasons |
| `evals/evals.json` | Happy path plus four halt conditions |
