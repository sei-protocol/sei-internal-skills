# linear-ticket

Files a specification's user stories as Linear issues in the house format.
Owns the ticket body and its lineage. Decides nothing.

## The format is not invented

It is the shape of `PLT-646`, generalised:

```
Problem · Impact · Relevant experts · Proposed approach ·
Acceptance criteria · Out of scope · References
```

Seven sections, all mandatory. An empty one says `None.` so a reader can tell an
empty section from a forgotten one.

## No open standard defines a ticket

Which is why the sections are stated in full rather than named. What goes
*inside* them is anchored: **INVEST** for whether the ticket is a real slice,
**Gherkin** for the acceptance criteria, **MoSCoW** for the priority band.
Underneath sits **ISO/IEC/IEEE 29148:2018** clause 6.4 — converting stakeholder
requirements into testable work items — which is paywalled, so it is cited and
never reproduced.

## Where it sits

| Skill | Owns |
|---|---|
| Spec Kit phases | The spec the stories come from |
| **`/linear-ticket`** | The ticket body, the project and team, the write-back |
| `/project-brief` | The project's description, not its issues |

`/tasks-to-linear` is a kernel that files one issue per story with no lineage
and no format. It does not ship in this repository. This skill supersedes it for
Sei work rather than wrapping it, because a project cannot be attached after
creation without an update, and that kernel forbids updates.

## What it refuses

- **Filing without confirmation.** Every issue is rendered in full first, then it
  waits for the word `confirm`.
- **Guessing the team or project.** Both come from `.specify/linear.json`. With
  no config it stops and names what is missing.
- **Inventing acceptance criteria.** They come from the story's Independent Test
  and Acceptance Scenarios. A story missing them is a spec defect, reported as
  one.
- **Filing twice.** It reads the write-back markers first and skips what is
  already filed.
- **Urgent priority.** P1 maps to High. A specification is not an interrupt.
- **Updating anything.** It creates. Editing, closing and re-prioritising stay
  with the caller.

## Files

| Path | What |
|---|---|
| `SKILL.md` | Guardrails, the priority mapping, the six-step procedure |
| `references/format.md` | The seven sections, what governs each, the field mapping |
| `evals/evals.json` | Happy path plus five halt and degrade conditions |
