# project-brief

Authors the two text fields on a Linear Project — the one-line `summary` and the
`description` — so a person who opens the project understands it without opening
anything else.

Replaces the aggregation skills that were retired when Linear Views and Pulse took
over their job. Those assembled a second surface from many projects; this one makes a
single project good enough that no second surface is needed.

## The bar it enforces

> "It has got to be self contained, polished with link to external resources as
> needed, written for human consumption first and foremost. Everything else: specs,
> issues etc. stems from that."

## The principle

**Self-containment is a property of dependencies, not of volume.** A project is
self-contained when the reader can understand and decide without opening anything
else. Adding words does not make it self-contained; removing the reader's
dependencies does.

That resolves the apparent conflict between "write it verbose" and Linear's own "aim
for brevity — short specs are more likely to be read." Both forbid padding. Verbose
means complete, not long.

## What it refuses

- **Status.** Blockers, health, percent-done and progress belong to project updates,
  a separate Linear surface with its own cadence.
- **The project's own issues.** Linear renders them already. You may explain the
  shape of the work; you may not transcribe its identifiers — and relabelling the
  list "Phases" does not change what it is.
- **Anything the page already renders.** Lead, dates, milestones, counts, Resources.
- **Writing for a machine.** One human reader. A parsable line typed into prose
  competes with Linear's live fields and loses.
- **Placeholders.** No `[TODO]` in a live field. A field with holes looks finished
  and is not.
- **Fabrication.** Identifiers get verified before use, and verification happens
  before drafting — a draft in hand turns a verification failure into a negotiation.

## Testing

Built with the since-cut `/author-skill`. Three max-pressure scenarios were run against subagents
without the skill (deadline + authority; sunk-cost spec + social proof; dual-audience
parser pressure), and every rationalization in the skill's table is quoted from what
those agents actually said. The same three scenarios were re-run with the skill
loaded; all three halted rather than fabricate.

Two REFACTOR cycles followed, both from defects the testers found in the skill itself:
its examples were drawn from the same domains as the test scenarios and read as
pasteable facts, and its override clause covered only one guardrail. See
`evals/evals.json`.

## Files

| Path | What |
|---|---|
| `SKILL.md` | Guardrails, procedure, rationalization table, halt conditions |
| `references/structure.md` | Section order for both fields, with the reasoning and the length target |
| `evals/evals.json` | Three pressure evals + one happy path |
