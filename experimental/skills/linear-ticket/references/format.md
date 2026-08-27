# The ticket body

Seven sections, in this order. Every one is mandatory. Write `None.` rather than
dropping a heading, so a reader can tell an empty section from a forgotten one.

The shape is `PLT-646` generalised. It was chosen because it already works in
this project, not because it is a standard.

```markdown
## Problem

What is wrong or missing today, in the present tense. Name the mechanism, not
the feeling. A reader who knows the system should recognise it without the spec.

## Impact

What it costs to leave this alone. This is the story's **Why this priority**,
stated as consequence rather than as ranking.

## Relevant experts

The specialist agents whose review this work needs, one line each, saying what
each one owns here. Omit an agent whose scope this does not touch.

## Proposed approach

The shape of the work in a short paragraph. Enough that an engineer can start;
not so much that it replaces the plan. Name files and interfaces where they are
already decided; do not invent them here.

## Acceptance criteria

A checklist. Every line is a statement someone can confirm true or false.

Sourced directly from the spec, never written fresh:
- the story's **Independent Test**, as the first line
- each **Acceptance Scenario**, as Given / When / Then
- every `SC-nnn` the story satisfies, with its verifier command

## Out of scope

What this ticket deliberately does not do, and where that work lives instead.
This is the section that stops a ticket growing while it is open.

## References

The spec path, related issues, and the files a reader should open first.
```

## What governs each section

No open standard defines a ticket's sections. The seven below are ours, and they
have to be stated, not named. What goes *inside* them is anchored:

| Section | Anchor that governs it |
|---|---|
| Problem | none — local |
| Impact | MoSCoW, for the priority band it justifies |
| Relevant experts | none — the roster is ours |
| Proposed approach | none — local |
| Acceptance criteria | Gherkin, for Given / When / Then |
| Out of scope | none — local |
| References | none — local |
| The ticket as a whole | INVEST, for whether it is a real slice |

Underneath all of it: **ISO/IEC/IEEE 29148:2018**, clause 6.4, which covers
converting stakeholder requirements into technical, testable work items and user
stories, and recommends a traceability matrix linking each requirement to its
origin, design, and tests. Paywalled — cite it, never reproduce it, and do not
assume a model resolves it without a probe.
<https://www.iso.org/standard/72089.html>

## Field mapping

Everything comes from the spec. Nothing is authored here.

| Ticket section | Spec source |
|---|---|
| Title | User story brief title, prefixed with the feature when it is not obvious |
| Problem | The story's plain-language journey, restated as the current gap |
| Impact | **Why this priority** |
| Relevant experts | Chosen from the agent roster by the work's domain |
| Proposed approach | The story's tasks in `tasks.md` |
| Acceptance criteria | **Independent Test** + **Acceptance Scenarios** + the `SC-nnn` verifiers |
| Out of scope | The spec's Out of scope, narrowed to this story |
| References | `specs/<NNN>-<slug>/spec.md`, related issues, first files to open |
| Priority | `(Priority: Pn)` mapped through the table in `SKILL.md` |
| Project, Team | `.specify/linear.json` |

## Titles

Descriptive, and readable on a board with no other context.

- Good: `Add a --check drift-guard for the doctrine block + CI`
- Good: `Pin all GitHub Action steps to commit SHAs`
- Weak: `User Story 3` — carries nothing outside the spec.
- Weak: `Fix the SDK` — names no mechanism.

A bracketed prefix is allowed where it classifies real follow-on work:
`[follow-up]`, `[hardening]`. Do not invent new prefixes.

## What a missing input means

| Missing | Do |
|---|---|
| Independent Test | Stop. Report the story as a spec defect. |
| Acceptance Scenarios | Stop. Same. |
| Priority | Stop. The spec decides priority, not this skill. |
| Out of scope | Write `None.` and continue. |
| A verifier on an `SC-nnn` | Carry the criterion across with `judgement` marked. |
