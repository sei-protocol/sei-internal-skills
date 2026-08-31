<!--
MODIFIED FROM UPSTREAM. GitHub Spec Kit ships this file; we carry deltas.
Keeping the fork visible matters: `specify` overwrites this on re-init, and a
silent overwrite would take the deltas with it. The pristine copy sits beside
this one as spec-template.upstream.md, so the fork is diffable.

The baseline is github/spec-kit, templates/spec-template.md, at commit
756d63212987152564ed0a52ddfd7f8e9b504e09 (2026-05-12).
https://github.com/github/spec-kit/blob/756d632/templates/spec-template.md
check-template-deltas.sh diffs this body against that copy, so the copy is
load-bearing and the revision it came from belongs on the record. Refreshing it
means replacing spec-template.upstream.md, updating this line, and re-reading
the delta table: upstream may have adopted a delta, which retires the row.

Deltas, and the reason for each:

  ## Semantic Anchors      Methods named once. Without it the body restates
                           them, and the spec grows without saying more.
  ## Glossary              An agent reads linearly and cannot ask what a term
                           means. Ubiquitous language, in one place.
  ## Boundary Context      A spec with no stated boundary grows while it is open.
  ### Requirement N        Requirements carry their own acceptance criteria
                           rather than sitting in a flat list, so no requirement
                           is an orphan.
  **Objective:**           Names the beneficiary, not only the behaviour.
  **Traces to:**           Every requirement points back at the story it serves.
  #### Acceptance Criteria EARS, with a named actor. THE <system> SHALL is the
                           closed grammar that keeps criteria readable.
  *Verifier:*              A success criterion names the command that checks it,
                           or says judgement. An unmarked criterion is not honest.

Write SHALL in uppercase. It satisfies EARS and RFC 2119 at once, and both run
at error on this path.

Delete every bracketed placeholder and this comment before the spec ships.
-->

# Feature Specification: [FEATURE NAME]

**Feature Branch**: `[###-feature-name]`

**Created**: [DATE]

**Status**: Draft

**Input**: User description: "$ARGUMENTS"

## Semantic Anchors

Named once. Not restated below. Three to six. Each needs a *does not cover*
entry, because an anchor is a hint and the gap is the honest part.

| Anchor | Governs | Does not cover |
|---|---|---|
| EARS | acceptance criteria syntax | whether the template fits the real class |
| RFC 2119 | normative keywords | whether the obligation is the right one |
| INVEST | whether a story is a real slice | whether the slice delivers value |

## Glossary

Every term this spec uses in a specific sense, defined once.

- **[Term]**: [What it means here, precisely enough that two readers agree.]

## Boundary Context

What this specification is inside, and what it is not responsible for.

- **Sits within**: [the system or sub-domain]
- **Owns**: [the behaviour this spec decides]
- **Does not own**: [the adjacent behaviour, and who decides it instead]

## User Scenarios & Testing *(mandatory)*

Order stories by priority. Each story stands as an independent test: implementing
only that one still leaves something usable.

### User Story 1 - [Brief Title] (Priority: P1)

[The journey in plain language.]

**Why this priority**: [The value, and why it ranks here.]

**Independent Test**: [The concrete action someone takes to confirm it works.]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

---

### User Story 2 - [Brief Title] (Priority: P2)

[The journey in plain language.]

**Why this priority**: [The value, and why it ranks here.]

**Independent Test**: [The concrete action someone takes to confirm it works.]

**Acceptance Scenarios**:

1. **Given** [initial state], **When** [action], **Then** [expected outcome]

### Edge Cases

- What happens when [boundary condition]?
- How does the system handle [error scenario]?

## Requirements *(mandatory)*

Each requirement carries its own acceptance criteria, so no requirement is an
orphan and no criterion floats free of a requirement.

### Requirement 1: [Name]

**Objective:** As a [role], I want [capability], so that [benefit].

**Traces to:** User Story 1

#### Acceptance Criteria

Write each one in an EARS template. All five end in THE [system] SHALL
[response]. Uppercase SHALL.

1. WHEN [trigger], THE [system] SHALL [response].
2. WHILE [state], THE [system] SHALL [response].
3. IF [unwanted condition], THEN THE [system] SHALL [response].
4. WHERE [feature is included], THE [system] SHALL [response].
5. THE [system] SHALL [response].

### Requirement 2: [Name]

**Objective:** As a [role], I want [capability], so that [benefit].

**Traces to:** User Story 2

#### Acceptance Criteria

1. WHEN [trigger], THE [system] SHALL [response].

Mark an unstated detail. Never guess one:

2. WHEN [trigger], THE [system] SHALL [NEEDS CLARIFICATION: the question].

### Key Entities *(include if the feature involves data)*

- **[Entity]**: [What it represents, and its relationships. No implementation.]

## Success Criteria *(mandatory)*

Every criterion names the command that checks it, or says `judgement`. A
criterion nothing checks is a wish.

- **SC-001**: [Measurable outcome.]
  *Verifier:* `[the command]`
- **SC-002**: [Measurable outcome.]
  *Verifier:* judgement — [who decides, and against what]

## Assumptions

- [The default chosen where the input was silent, and why it is reasonable.]

## Out of scope

- [What this specification does not do, and where that work lives instead.]
