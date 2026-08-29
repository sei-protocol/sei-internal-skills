# Feature Specification: Recognition Suite for Semantic Anchors

**Feature Branch**: `002-recognition-suite`

**Created**: 2026-08-28

**Status**: Draft

**Input**: User description: "Ask a model each anchor's own question in a clean
context. Score the answer by hand. Record one verdict per anchor, with the model
identifier and the date. Make a fall visible on the next model."

## Semantic Anchors

Named once. The body below does not restate them.

| Anchor | Governs | Does not cover |
|---|---|---|
| EARS | acceptance criteria syntax | whether the template fits the real class |
| RFC 2119 | normative keywords, uppercase | whether the obligation is the right one |
| INVEST | whether a story is a real slice | whether the slice delivers value |
| Gherkin | acceptance scenario shape | whether the scenario is the important one |
| BLUF | conclusion first in each section | whether that sentence is the bottom line |

All five carry no recorded verdict. This specification describes the suite that
would produce one. Until it runs, treat a surprising reading of these five rows
as the anchor failing rather than the model disagreeing.

Two of the five go further: INVEST and Gherkin have no registry entry at all, so
they hold no question to ask. `writing/anchors/unregistered.txt` records them, with ten
others the contract names. The suite cannot test an anchor the registry does not
hold, so that list bounds what a first run can cover.

## Glossary

- **Anchor**: a public standard the catalogue names, on the bet that a model
  already resolves the term.
- **Recognition**: a model naming the standard's own concepts from the term
  alone, with no help from the asker.
- **Registry**: the one file holding an entry per anchor. It is the source of
  truth for this suite.
- **Question**: the open prompt an entry carries, asked of a model with no other
  instruction.
- **Expected concept**: one concept a passing answer names. An entry lists
  more than one per anchor.
- **Clean context**: a model session with no repository instruction, no earlier
  turn, and no sight of the expected concepts.
- **Rating**: a value a person gives an answer on one of four dimensions:
  recognition, accuracy, depth, and specificity.
- **Verdict**: one value for one anchor from one run: strong, partial, or
  absent.
- **Run**: one pass over the registry against one model identifier on one date.
- **Record**: one verdict with its model identifier, its date, its four ratings,
  and the raw answer.
- **Fall**: a new verdict that ranks below the most recent earlier verdict for
  the same anchor.

## Boundary Context

- **Sits within**: the evidence rule of the constitution, which turns a named
  standard into an anchor only after a recorded test.
- **Owns**: how the suite asks, how a person scores, what a run records, and how
  a later run exposes a fall.
- **Does not own**: which terms the catalogue names, the wording of any
  convention, and the writing rules. The constitution decides all three.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A first verdict for every anchor (Priority: P1)

A maintainer runs the suite against the model in use. The suite reads the
registry and asks each question in a clean context. It collects the raw answers
and shows each one beside its expected concepts. The maintainer rates the answer
on four dimensions and settles one verdict. Each entry gains a record.

**Why this priority**: the catalogue rests on an untested bet today. Every later
story reads the records this story writes, so nothing else works without it.

**Independent Test**: run the suite over a registry holding one anchor. Confirm
that the entry gains one record carrying a verdict, a model identifier, a date,
and the raw answer.

**Acceptance Scenarios**:

1. **Given** a registry of five anchors with no records, **When** a maintainer
   runs the suite and rates each answer, **Then** every entry holds one record.
2. **Given** an entry whose question is blank, **When** the run starts, **Then**
   the suite stops and names that entry.
3. **Given** an entry with one older record, **When** a new run writes a second,
   **Then** the older record stays unchanged beside it.

---

### User Story 2 - A fall the upgrade cannot hide (Priority: P2)

A model upgrade lands. The maintainer re-runs the suite against the new model
identifier and rates the answers. The run report names every anchor whose
verdict dropped, before the maintainer reads a single answer. An anchor that
fell from strong to partial appears at the top of that report.

**Why this priority**: the catalogue is a standing bet, and a model upgrade can
lose it. A drop nobody sees is the failure this suite exists to stop.

**Independent Test**: record a strong verdict, then run again against a second
model identifier and rate one answer as partial. Confirm the run report names
that anchor as a fall.

**Acceptance Scenarios**:

1. **Given** an anchor holding a strong verdict, **When** a later run rates it
   partial, **Then** the run report names the anchor as a fall.
2. **Given** an anchor holding a partial verdict, **When** a later run rates it
   strong, **Then** the run report names a rise, not a fall.
3. **Given** an anchor with no earlier record, **When** the first run rates it,
   **Then** the run report calls the anchor new.

---

### User Story 3 - Evidence a reader can see (Priority: P3)

A reader opens the catalogue and wants to know which anchors carry evidence. For
each anchor the catalogue states the verdict, the model identifier, and the date.
An anchor with no record reads plainly as a stated rule, not as an anchor. A
partial verdict keeps its stated text beside the anchor name.

**Why this priority**: the records are worth little while they sit unread in a
file. This story turns a record into something a reader acts on.

**Independent Test**: read the catalogue for two anchors, one with a record and
one without. Confirm that a reader tells them apart without opening the registry.

**Acceptance Scenarios**:

1. **Given** an anchor with a strong verdict, **When** a reader opens the
   catalogue, **Then** the entry states the verdict, the model, and the date.
2. **Given** an anchor with no record, **When** a reader opens the catalogue,
   **Then** the entry reads as untested and as a stated rule.
3. **Given** an anchor with a partial verdict, **When** a reader opens the
   catalogue, **Then** the stated text sits beside the anchor name.

---

### User Story 4 - A new anchor with no code change (Priority: P4)

A maintainer admits a new anchor. They write one entry: the term, its question,
and its expected concepts. The next run covers the new anchor and produces a
record for it. Nobody edits the suite.

**Why this priority**: the value is real but small while the catalogue is short.
It matters on the day a second author adds an anchor.

**Independent Test**: add one entry to the registry and change nothing else. Run
the suite and confirm that the new anchor gains a record.

**Acceptance Scenarios**:

1. **Given** a registry of five entries, **When** a maintainer adds a sixth and
   runs the suite, **Then** the run asks six questions.
2. **Given** a new entry with no expected concepts, **When** the run starts,
   **Then** the suite stops and names the entry.
3. **Given** an entry a maintainer deletes, **When** the next run starts,
   **Then** the run asks nothing for that term.

### Edge Cases

- What happens when the model returns an empty answer, or declines to answer?
- What happens when a person rates part of a run and leaves the rest unrated?
- What happens when two entries carry the same question?
- What happens when a provider withdraws the model identifier of an earlier
  record, and no later run reaches it?
- What happens when an anchor leaves the catalogue while its records remain?
- What happens when a raw answer quotes text the repository cannot publish?

## Requirements *(mandatory)*

### Requirement 1: The registry is the source of truth

**Objective:** As a maintainer, I want one file to hold every question, expected
concept, and record, so that no second list can disagree with it.

**Traces to:** User Story 1, User Story 4

#### Acceptance Criteria

1. THE recognition suite SHALL read every anchor, question, and expected concept
   from the registry and from nowhere else.
2. WHEN a maintainer adds an entry, THE recognition suite SHALL cover that anchor
   on the next run with no change to the suite.
3. IF an entry carries no question or no expected concept, THEN THE recognition
   suite SHALL stop and name the entry.
4. WHERE an entry already holds records, THE recognition suite SHALL append the
   new record and leave the earlier ones unchanged.
5. WHILE a run is open, THE recognition suite SHALL write nothing to the registry
   until a person confirms a verdict.

### Requirement 2: One clean context per question

**Objective:** As a maintainer, I want each question asked alone, so that the
answer measures the model's own prior, not our prompt.

**Traces to:** User Story 1

#### Acceptance Criteria

1. WHEN the suite asks a question, THE recognition suite SHALL send that question
   alone, with no repository instruction and no earlier turn.
2. THE recognition suite SHALL keep the expected concepts out of the prompt.
3. WHERE one run covers many anchors, THE recognition suite SHALL hide every
   other question and answer from each context.
4. IF the model returns an empty answer or declines, THEN THE recognition suite
   SHALL store the empty result and mark the anchor unrated.
5. WHEN a run starts, THE recognition suite SHALL collect [NEEDS CLARIFICATION:
   how many answers per question, one or a fixed sample of many?].

### Requirement 3: A person scores the answer

**Objective:** As a maintainer, I want to rate each answer myself, so that the
verdict carries a judgement no program has to fake.

**Traces to:** User Story 1

#### Acceptance Criteria

1. THE recognition suite SHALL show the raw answer beside the expected concepts
   of that anchor.
2. WHEN a person rates an answer, THE recognition suite SHALL accept one value
   each for recognition, accuracy, depth, and specificity.
3. THE recognition suite SHALL turn the four ratings into one verdict, by
   [NEEDS CLARIFICATION: which rating combination gives each of the three
   verdicts?].
4. IF a person leaves any of the four ratings unset, THEN THE recognition suite
   SHALL refuse the verdict and keep the anchor unrated.
5. THE recognition suite SHALL never write a verdict that no person confirmed.

### Requirement 4: The record carries its evidence

**Objective:** As a reviewer, I want each verdict stored with its model
identifier, date, and raw answer, so that I can re-read it.

**Traces to:** User Story 1, User Story 3

#### Acceptance Criteria

1. WHEN a person confirms a verdict, THE recognition suite SHALL write the
   verdict, the model identifier, the date, the ratings, and the raw answer.
2. THE recognition suite SHALL store the raw answer as the model wrote it.
3. THE recognition suite SHALL add each record and replace none.
4. WHERE an anchor holds no confirmed verdict, THE recognition suite SHALL report
   that anchor as untested.
5. IF two records share an anchor and a model identifier, THEN THE recognition
   suite SHALL keep both and order them by date.

### Requirement 5: A fall is visible

**Objective:** As a maintainer, I want a re-run to name every anchor that
dropped, so that a model upgrade cannot quietly erode the catalogue.

**Traces to:** User Story 2

#### Acceptance Criteria

1. THE recognition suite SHALL rank strong above partial, and partial above
   absent.
2. WHEN a run ends, THE recognition suite SHALL compare each new verdict against
   the most recent earlier verdict for the same anchor.
3. IF a new verdict ranks below the earlier one, THEN THE recognition suite SHALL
   name that anchor as a fall in the run report.
4. WHERE an anchor holds no earlier verdict, THE recognition suite SHALL call the
   anchor new rather than a fall.
5. WHEN the run report names a fall, THE recognition suite SHALL [NEEDS
   CLARIFICATION: end the run in failure, or report the fall and exit clean?].

### Requirement 6: The catalogue states the evidence

**Objective:** As a reader, I want the catalogue itself to state each verdict, so
that I can weigh an anchor without opening the registry.

**Traces to:** User Story 3

#### Acceptance Criteria

1. THE recognition suite SHALL state, per anchor, the latest verdict, the model
   identifier, and the date.
2. WHERE an anchor holds no record, THE recognition suite SHALL present the term
   as a stated rule rather than as an anchor.
3. WHERE a verdict is partial, THE recognition suite SHALL keep the stated text
   beside the anchor name.
4. WHEN a run writes a new record, THE recognition suite SHALL update the stated
   verdict in the catalogue within the same run.
5. IF the catalogue and the registry disagree, THEN THE recognition suite SHALL
   treat the registry as correct and report the difference.

### Requirement 7: Everything here is publishable

**Objective:** As the owner of a public repository, I want no proprietary
standard text inside the suite, so that the whole record stays open.

**Traces to:** User Story 3

#### Acceptance Criteria

1. THE recognition suite SHALL hold no body text from a standard that forbids
   redistribution.
2. THE recognition suite SHALL cite a standard by name and reference only.
3. WHERE a question names a standard, THE recognition suite SHALL limit the quote
   to the term under test.
4. IF a reviewer finds unpublishable text in a raw answer, THEN THE recognition
   suite SHALL hold that record back until a reviewer clears it.
5. WHEN a maintainer adds an entry, THE recognition suite SHALL mark the entry
   for a publication review before the entry merges.

### Key Entities *(include if the feature involves data)*

- **Anchor entry**: one unit of the registry. It holds the term, the question,
  the expected concepts, and the records for that term.
- **Question**: the open prompt for one anchor. One meaning, and no hint of the
  answer.
- **Expected concept**: one concept a passing answer names.
- **Record**: one verdict with its model identifier, its date, its four ratings,
  and the raw answer.
- **Run**: one pass over the registry against one model identifier on one date.
- **Run report**: the verdicts one run produced, and the anchors that fell.

## Success Criteria *(mandatory)*

**SC-001**: Every anchor **the registry holds** carries at least one record with a
verdict, a model identifier, a date, and the raw answer.
*Verifier:* not built — the recognition suite and its registry check do not exist.

The criterion reads on the registry, not on the catalogue. The contract names 19
anchors and the registry holds 7. A criterion written against the catalogue could
not pass until twelve unrelated entries existed. Closing that gap is
`writing/anchors/unregistered.txt`, not this suite.

**SC-002**: A run against a new model identifier names every anchor that dropped,
before a person reads any answer.
*Verifier:* not built — no run report and no verdict comparison exist yet.

**SC-003**: A maintainer covers one added anchor by editing the registry alone,
and changes no other file.
*Verifier:* not built — the registry-driven runner does not exist yet.

**SC-004**: No verdict reaches the registry without four ratings a person set.
*Verifier:* not built — the scoring step and its refusal path do not exist yet.

**SC-005**: This specification passes the writing gate with no finding at error
level.
*Verifier:* `vale --no-global writing/specs/002-recognition-suite/spec.md`

**SC-006**: Every criterion above names a verifier that runs, or states plainly
that none does.
*Verifier:* `writing/scripts/check-verifiers.sh`

**SC-007**: A reader separates an evidenced anchor from an untested term without
opening the registry.
*Verifier:* judgement — a maintainer reads the catalogue and lists the untested
terms from it alone.

**SC-008**: No proprietary standard text enters the repository through an entry,
a question, or a raw answer.
*Verifier:* judgement — a reviewer reads each new entry and each answer before
the change merges.

## Assumptions

- The plan decides the registry format. This specification treats that format
  as given, and names no file.
- A model identifier is stable enough to name a run, and two runs with different
  identifiers are worth comparing.
- One person rates every answer in a run. Two raters and their disagreement are
  a later problem.
- An anchor's question stays fixed between runs. A changed question breaks the
  comparison, and the plan states what a maintainer does then.
- The catalogue and the registry live in the same repository, and one commit can
  update both.

## Out of scope

- An automatic scorer. A person rates every answer, and the brief rules the
  automatic path out.
- Deciding which terms the catalogue names as anchors. The constitution owns
  that table.
- Testing whether a model applies an anchor correctly. This suite tests
  recognition of the term and nothing more.
- The three other admission artifacts: the fixture per rule, the coverage row,
  and the false-positive count. Each has its own specification.
- Choosing the model provider, the transport, or the storage layout. All three
  belong to the plan.
- Rewriting a convention after a weak verdict. The verdict is evidence, and an
  author decides what to do with it.
