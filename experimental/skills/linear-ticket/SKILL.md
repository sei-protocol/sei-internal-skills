---
name: linear-ticket
category: project-management
description: "Use when turning a Spec Kit feature's user stories into Linear issues in the house format — 'file the tickets for this spec', 'create the Linear issues for these stories', 'convert these milestones to tickets', 'file this as a Linear ticket', '/linear-ticket'. Writes the seven-section body (Problem, Impact, Relevant experts, Proposed approach, Acceptance criteria, Out of scope, References), attaches the project and team, and writes identifiers back so a re-run never duplicates. Anti-triggers: NOT for authoring the spec itself; NOT for the project description or its update (use /project-brief); NOT for GitHub issues; NOT for updating, closing, or re-prioritising an existing issue; NOT for deciding scope or which story is P1 — the spec decided that. It files what the spec already settled."
user-invocable: true
---

# Linear ticket

File a spec's user stories as Linear issues that an engineer, or another agent
session, can pick up without reading the spec first.

The format is not invented. It is the shape of `PLT-646`, generalised.

## Guardrails

This skill creates issues in a real tracker. Before any write:

1. **Confirm before filing.** Render every issue in full, then require the word
   `confirm`. Never file on the first call.
2. **Never guess the team or project.** Both come from `.specify/linear.json`.
   With no config, stop and say what is missing. Never file into whichever team
   sorts first.
3. **Create, never update.** This skill opens new issues. It does not edit,
   close, re-prioritise, or move an existing one. That is the caller's job.
4. **Never file twice.** Read the write-back markers in `tasks.md` first and
   skip every story that already carries one. A re-run after a partial failure
   files only what is missing.
5. **Bounded.** Refuse to create more than `max_issues` (default 10) in one run.
   An unexpectedly large tasks file stops the run; it does not populate a board.
6. **Never invent acceptance criteria.** They come from the story's Independent
   Test and Acceptance Scenarios. A story missing them is a spec defect — say
   so and stop. Do not write a plausible bar.
7. **Degrade, do not fail silently.** With no Linear tools available, render the
   issues into the conversation and say plainly what was missing.

## Relationship to `/tasks-to-linear`

`/tasks-to-linear` is a kernel that files one issue per user story with no
lineage and no format. **It does not ship in this repository** — it reaches a
machine by another route, so do not assume it is installed.

This skill is the layer that kernel's design anticipated. **It supersedes the
kernel for Sei work** — it does not call it, because a project cannot be
attached after creation without an update, and the kernel forbids updates. Use
the kernel in a repository that wants issues with no lineage.

## Preconditions

| Need | Check |
|---|---|
| A finished spec | `specs/<NNN>-<slug>/spec.md` with priorities and Independent Tests |
| Work units | `specs/<NNN>-<slug>/tasks.md` |
| Config | `.specify/linear.json` |
| Linear tools | `list_projects`, `save_issue` reachable |

```json
{
  "team": "PLT",
  "project": "Sei Agentic Mesh",
  "max_issues": 10
}
```

## Procedure

### 1. Read, do not re-plan

Read `spec.md` and `tasks.md`. The spec set the priorities and the acceptance
bar. Carry them across unchanged. If you find yourself deciding something, stop —
that decision belongs in the spec.

### 2. Resolve the lineage

Look up the team and project by the config's names. Confirm both resolve to
exactly one record. Report what you resolved before filing.

Map the story priority to Linear's scale:

| Spec | Linear |
|---|---|
| P1 | 2 High |
| P2 | 3 Medium |
| P3 and below | 4 Low |

Urgent (1) is never set from a spec. An urgent ticket is an interrupt, and a
specification is not an interrupt.

### 3. Build each body

One issue per user story, plus Setup and Foundational when the tasks file has
them. Use the seven sections in `references/format.md`. Every section is
mandatory; write `None.` rather than dropping a heading.

### 4. Render and confirm

Show every issue in full: title, priority, project, team, and the whole body.
Then wait for `confirm`.

### 5. File, then write back

Create the issues. Immediately append the identifier to the story's heading in
`tasks.md`, so a re-run skips it:

```markdown
### User Story 1 - Session item operations (Priority: P1) <!-- linear: PLT-1042 -->
```

Write back after each issue, not once at the end. A run that dies halfway must
leave an accurate record.

### 6. Report

State every identifier created, and every story skipped and why. If any issue
failed, say which and leave the rest filed.

## References

- `references/format.md` — the seven-section body and the field mapping.
