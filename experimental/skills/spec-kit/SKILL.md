---
name: spec-kit
category: workstream-bootstrap
description: "Use when authoring a feature specification with GitHub Spec Kit — 'write the spec', 'spec this feature', 'run specify', 'draft spec.md', 'write plan.md', 'write tasks.md', '/spec-kit'. Produces condensed engineering specs: Spec Kit's own template shape and CLI, semantic anchors declared once instead of method explained inline, an Independent Test on every user story, and a named verifier on every success criterion, so the stories seed Linear tickets without a rewrite. Anti-triggers: NOT for filing the tickets themselves (use /tasks-to-linear); NOT for a design spike that argues a position (that is a design doc, not a spec); NOT for work whose diff you could describe in one sentence — skip the spec and do it; NOT for a project description (use /project-brief); NOT for prose review (use prose-steward). It authors the artifacts; it does not decide scope, priority, or which milestone comes first."
user-invocable: true
---

# Spec Kit

Author a feature specification that a different engineer, or a different agent
session with none of your context, can implement from.

The skill adds three things to Spec Kit and changes nothing else:

1. **An anchor block.** Name the methods once at the top. Never explain a named
   method in the body. This is what makes the spec condensed.
2. **A verifier per success criterion.** A criterion no command can check is a
   wish. Say which command checks it, or mark it as judgement.
3. **A ticket-ready user story.** Spec Kit already requires a priority and an
   Independent Test. Hold that bar, because `/tasks-to-linear` files one issue
   per story and cannot invent what the story left out.

## Guardrails

This skill writes files in `specs/<NNN>-<slug>/`. Before any write:

1. **Never invent a requirement.** An unstated detail becomes
   `[NEEDS CLARIFICATION: <the question>]`. A plausible default silently written
   into a spec is the failure this whole artifact exists to prevent.
2. **`spec.md` holds WHAT and WHY only.** The moment you name a library, a
   schema, a signature, or a file path, it belongs in `plan.md`.
3. **Never name an unvetted anchor.** Name an anchor only if it appears in
   `references/anchors.md`. A confabulated method name reads authoritative and
   costs more than a paragraph of plain prose.
4. **Never file a ticket.** Hand off to `/tasks-to-linear`.
5. **Never overwrite.** If `spec.md`, `plan.md`, or `tasks.md` exists, stop and
   show the diff you propose. Do not write over prior intent.
6. **Refuse without `.specify/`.** Run `specify init` in the repository. Do not
   hand-roll the directory or copy a template by hand — the vendored templates
   are the convention.

Refuse to run when the work is one sentence of diff. Say so and stop.

## Preconditions

| Need | Check |
|---|---|
| Spec Kit CLI | `specify --version` |
| Vendored templates | `.specify/templates/spec-template.md` exists |
| Writing verifier | `vale --version` |
| Linear config, only if tickets follow | `.specify/linear.json` holds the team key |

## Procedure

### 1. Decide whether a spec is warranted

A spec earns its cost when the decomposition is the hard part: many obligations,
real ordering constraints, and a credible risk of declaring victory early. Below
that, write the code.

State the decision in one line before continuing.

### 2. Run the CLI, do not hand-roll

```sh
specify init          # only if .specify/ is absent
```

Create `specs/<NNN>-<slug>/` and start from
`.specify/templates/spec-template.md`. Numbering is sequential and permanent.

### 3. Write the anchor block

Directly under the metadata, before User Scenarios:

```markdown
## Anchors

Named once. Not restated below.

| Anchor | Governs | Does not cover |
|---|---|---|
| EARS | requirement syntax | whether the template matches the real class |
| RFC 2119 | normative keywords | whether the obligation is the right one |
| INVEST | user story quality | whether the slice delivers value |
| Gherkin | acceptance scenarios | whether the scenario is the important one |
```

Pick from `references/anchors.md`. Three to six anchors. More than six means the
spec is doing two jobs.

The **Does not cover** column is mandatory. An anchor is a hint, not a
guarantee, and the gap is the honest part.

### 4. Write the body against the anchors

Follow the vendored template's section order exactly. Apply the deltas in
`references/deltas.md` — they are what turns a product-flavoured template into
an engineering one.

Per user story, all four are mandatory, because a ticket is generated from them:

- `(Priority: P1)` — P1 is the slice that, alone, still delivers something usable.
- **Why this priority** — the value, in one or two sentences.
- **Independent Test** — the concrete action a person takes to confirm it works.
- **Acceptance Scenarios** — Given / When / Then.

### 5. Condense

Cut in this order:

1. Any sentence that explains a named anchor. The anchor is the explanation.
2. Any placeholder comment left from the template.
3. Any requirement that restates a success criterion, or the reverse.
4. Any adjective carrying no measurement.

A condensed spec is not a short spec. It is a spec with no line that a reader
could have derived from the anchor block.

### 6. Verify

```sh
vale specs/<NNN>-<slug>/spec.md
```

Then check by hand:

- Every `SC-nnn` names a command, or says `judgement`.
- Every user story has all four required parts.
- Every `[NEEDS CLARIFICATION]` is a real question, not a placeholder.
- No implementation detail sits in `spec.md`.

Report what Vale said, including a failure.

### 7. Hand off

`plan.md` is HOW. `tasks.md` is work units. Then `/tasks-to-linear` files one
issue per user story.

Do not file tickets from here.

## References

- `references/anchors.md` — the vetted anchor set, and what each does not cover.
- `references/deltas.md` — the deltas from the vendored template, and why.
