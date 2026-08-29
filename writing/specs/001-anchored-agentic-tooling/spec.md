# 001 — Anchored agentic tooling

**Status**: Draft

**Created**: 2026-08-19

**Input**: the request this specification answers, in the words of the person who
made it. The fence marks it as another person's text. The writing rules govern
this document, not its source.

```text
Iterate on the agentic-writing package as a V2 of the sei-internal-skills
and agents. Right now we have virtually no adoption of those tools, so rethink them in
simpler terms — keep the mechanisms that work well, like the agent experts, and refine
the skills to strategically replace custom engineering rigor, writing or thinking
patterns with semantic anchors instead. Frame it from the beginning around principled
agentic tooling best practices, to leverage existing research that tells us why
something works and that it can be expected to keep working as models develop. Anchor
it around spec-driven development with a clean semantic anchoring on technical design
doc writing style.
```

<!-- vale off -->
## Semantic Anchors

Named once. Not restated below.

| Anchor | Governs | Does not cover |
|---|---|---|
| EARS | requirement syntax | whether the template matches the real class |
| RFC 2119 | normative keywords | whether the obligation is the right one |
| INVEST | user story quality | whether the slice delivers value |
| Gherkin | acceptance scenarios | whether the scenario is the important one |
| Conventional Commits | commit subject | whether the scope names the right component |
<!-- vale on -->

## Glossary

Every term this specification uses in a specific sense.

- **Anchor**: a public standard the model already holds, named rather than restated,
  and only after a recognition test records that it resolves.
- **Stated rule**: a local rule, or a term with a weak prior, written out in full
  because naming it would not carry it. The counterpart of an anchor.
- **The contract**: the single always-loaded file holding the anchors and the stated
  rules, at `writing/CONTRACT.md`. Where this specification says "the
  contract" it means that file, never the category above.
- **Gate**: a command that measures an artifact and can block a merge.
- **Expert**: a persona addressable by role name, applying judgement a gate cannot.
- **Recognition test**: an open question asking what a model associates with an anchor
  name, scored on recognition, accuracy, depth and specificity. The method is in
  `writing/evals/recognition/README.md`. No suite runs it yet.
- **Verdict**: a recognition-test result, one of strong, partial or absent, recorded
  with the model identifier, the date, and the answer behind it.
- **Admitted**: an anchor carrying all four admission artifacts.
- **Grandfathered**: an anchor predating the admission rule, exempt and counted.
- **Writing mode**: a document type with a structure contract, selected by path.

## Boundary Context

- **Sits within**: this repository, which is public.
- **Owns**: the contract, the evidence behind each anchor, the gates, and the rule
  that admits a new anchor.
- **Does not own**: organisation-specific profiles, operational procedures, and the
  skills built on them. Those stay in the organisation's own repository and cite the
  public anchors from there.

## Why this exists

V1 is a single-author library. Measured on 2026-08-19:

| Measure | Value |
|---|---|
| Repository created | 2026-03-21 |
| Commits | 152 by one author, plus 1 |
| Merged pull requests | 100, all by the same author |
| Forks | 0 |
| Issues opened by anyone else | 0 |
| Skills | 16 core, 11 experimental |
| Skill payload | 13,778 lines |

Five months, no second user.

## The thesis

**Leanness is a maintainability win, not an adoption win.** Sixteen lean skills nobody
installs is the same outcome as sixteen heavy ones. The distinguishing variable in V1 is
not size. It is whether a mechanism reaches an engineer who is not looking for it.

| Mechanism | Reaches an engineer | Adopted in V1 |
|---|---|---|
| Doctrine block in the context file | Pushed into the repository, read every session | Yes |
| Agent expert | Addressable by role name | Yes |
| CI gate | Blocks a merge whether or not anyone invoked it | Yes |
| Skill | Needs an install, then a remembered trigger phrase | No |
| Experimental skill | Needs a second, separate opt-in | Invisible |

So V2 inverts the ratio. The convention moves into the channel that already works, and
a skill becomes the rare case.

**This is why semantic anchoring is load-bearing rather than cosmetic.** An anchor is
small enough to live in an always-loaded context file. A 350-line knowledge kit is not.
Anchors do not make skills better. They make skills unnecessary for the common case.

### Why it stays reliable

An anchor named after a densely published standard becomes **more** reliable as models
train on more text about that standard. A hand-tuned kit becomes **less** reliable as
models drift, and it fails silently.

The claim carries a bound, and the bound is part of the design: an anchor is only as strong
as its prior. A new or internal convention has no prior, cannot become an anchor, and stays
stated in full and verified.

## Architecture

Four channels. Each convention belongs to exactly one.

| Channel | How it reaches an engineer | Holds |
|---|---|---|
| **Contract** | Always loaded, no invocation | The anchors, and the local rules that have no prior |
| **Evidence** | Read when a reader doubts a claim | Per anchor: steward, licence, the question, verdict, date |
| **Gate** | Blocks a merge | The checkable subset, as lint rules in CI |
| **Expert** | Named by role | The persona that applies judgement a gate cannot |

A **skill** exists only where a procedure has real side effects. Everything that is
knowledge belongs in the contract.

Spec Kit supplies the contract slot already. Its constitution carries a version,
amendable, read by every phase, and its governance clause requires reviews to verify
compliance. V2 uses that slot rather than inventing one. Path and file decisions beyond
the channel belong in `plan.md`.

## User Scenarios & Testing

### User Story 1 - An engineer gets the convention without asking for it (Priority: P1)

An engineer who has never read this repository opens a feature in a consuming repository
and writes a design document. The anchors are already in their context, so the document
comes out in the house style. They installed nothing and invoked nothing.

**Why this priority**: This is the whole product. Every other story improves it or
assures it, and none has value if this one does not work. It is also the story V1 never
completed once in five months.

**Independent Test**: In a repository with the contract present and no skills installed,
ask for a design document. Confirm the output follows the anchored style, and that no
anyone invoked a slash command.

**Acceptance Scenarios**:

1. **Given** a repository carrying the contract and no installed skills,
   **When** an engineer asks for a technical design document,
   **Then** the output follows the
   anchored style without any invocation.
2. **Given** the same repository, **When** the engineer runs the gate, **Then** the gate
   reports findings against the same anchors the contract named.

### User Story 2 - A reviewer can tell why a rule exists (Priority: P2)

A reviewer disagrees with a finding. They follow the rule to the anchor. The anchor leads to
its steward and licence, and the anchor to its verdict and the date behind that verdict.
They can then argue about the standard rather than about taste.

**Why this priority**: An unexplained convention is the failure V1's README already
names — a hand-tuned prompt cannot explain itself. Without this, V2 is a different set
of unexplainable preferences.

**Independent Test**: Pick any finding the gate reports. Confirm the chain from rule to
anchor to steward to verdict resolves with no missing link.

**Acceptance Scenarios**:

1. **Given** a reported finding,
   **When** a reviewer looks up its rule,
   **Then** the rule names the anchor and the anchor resolves to a registry entry with a steward, a
   licence, and a verdict.
2. **Given** an anchor with no recorded verdict, **When** a reviewer reads the registry,
   **Then** the registry states the absence rather than implying it.

### User Story 3 - A design document reads correctly for a human and an agent (Priority: P2)

An author writes a technical design document. A human reviewer scans it and finds the
decision. An agent reads it linearly and acts on it. Neither has to guess which
sentences are normative.

**Why this priority**: The design document is the artifact where the organisation's
thinking lives, and the one both audiences consume. It is the natural first
subject, and it is the subject this repository already has machinery for.

**Independent Test**: Take one existing design document. Run the gate. Confirm every
finding names a clause of a public standard, and that fixing the findings does not
change the document's meaning.

**Acceptance Scenarios**:

1. **Given** a design document, **When** the gate runs, **Then** each finding cites a
   rule that cites a clause.
2. **Given** a normative statement, **When** an agent reads it, **Then** the obligation
   is unambiguous because the keyword is uppercase.

### User Story 4 - The contract survives a model upgrade (Priority: P3)

A new model becomes the default. The recognition suite runs against it. An anchor whose
recognition drops falls back to stated text before it silently degrades a review.

**Why this priority**: Silent degradation is the failure mode that makes a prompt-only
approach untrustworthy. It is also the failure a gate cannot catch, because the gate
checks the artifact and not the model.

**Independent Test**: Run the recognition suite against two different models. Confirm the
registry records a distinct verdict per model, and that a below-threshold verdict blocks.

**Acceptance Scenarios**:

1. **Given** a new default model, **When** the recognition suite runs, **Then** every anchor
   receives a scored verdict recorded with the model identifier and the date.
2. **Given** an anchor scoring below the threshold, **When** the gate runs, **Then** it
   fails until the registry records the lower verdict.

### Edge Cases

- **An anchor that is famous but sparse in training data.** Fame is not evidence. The
  the recognition test decides.
- **A local rule that resembles a public standard.** It stays local, because a later
  upstream change would silently move the rule.
- **A convention with no verifier.** The contract states it and listed as
  uncheckable. Nothing implies that a gate checks it.
- **Two anchors a model confuses.** The contract names the pair in full, never by short form.
- **A model that names an anchor and applies it wrongly.** Recognition passes,
  application fails, the verdict is partial, and the stated text stays.

## Requirements

Each requirement carries its own acceptance criteria, so no requirement is an orphan
and no criterion floats free of a requirement. The `FR-nnn` labels are stable
identifiers; the numbering now runs in order.

### Requirement 1: One always-loaded contract

**Objective:** As an engineer new to this repository, I want its conventions already
in my context, so that I follow them without setup.

**Traces to:** User Story 1

#### Acceptance Criteria

1. **FR-001** THE repository SHALL hold exactly one always-loaded contract, and a
   convention absent from it SHALL NOT count as adopted.
2. **FR-002** WHERE a recognition test records that a model resolves a public standard, THE
   author SHALL express that convention as an anchor.
3. **FR-003** WHERE a convention has no public standard, or no passing verdict, THE
   contract SHALL carry it as a stated rule, not an anchor.
4. **FR-004** THE contract SHALL state, for each convention, whether a gate checks it.
5. **FR-005** IF no gate checks a convention, THEN THE contract SHALL say so rather
   than leave the reader to assume a check exists.

### Requirement 2: Evidence behind every anchor

**Objective:** As a reviewer who disagrees, I want the chain from rule to anchor to
verdict, so that I argue the standard, not taste.

**Traces to:** User Story 2

#### Acceptance Criteria

1. **FR-006** THE registry SHALL record, per anchor, the steward, the licence, the
   redistribution terms, the question, and every verdict with its model and date.
2. **FR-007** WHEN an author adds an anchor to the contract, THE author SHALL record a
   verdict first.
3. **FR-008** THE recognition test SHALL score recognition, accuracy, depth and
   specificity.
4. **FR-009** THE registry SHALL store the answer beside the verdict, so that a reader
   who disagrees can re-score it.
5. **FR-010** IF an anchor scores below the partial threshold, THEN THE contract SHALL
   keep the stated text alongside the anchor.
6. **FR-011** IF an anchor scores below the failing threshold, THEN THE author SHALL
   remove it from the contract and state the rule instead.

### Requirement 3: Gates that cannot be quietly weakened

**Objective:** As an author of a blocked document, I want each finding to name its
rule and each rule its clause, so gates stay arguable.

**Traces to:** User Story 3

#### Acceptance Criteria

1. **FR-012** Every gate finding SHALL name a rule, and every rule SHALL name the
   clause it enforces.
2. **FR-013** THE author SHALL NOT weaken a gate to make a check pass. IF Vale cannot
   express a rule, THEN THE author SHALL delete it and record the constraint as
   uncheckable.
3. **FR-014** THE gate SHALL run in continuous integration on every change to a
   governed artifact.

### Requirement 4: Knowledge lives in the contract, not in a skill

**Objective:** As a teammate who installed nothing, I want conventions to reach me
through the contract and gates, so one I never invoke still applies.

**Traces to:** User Story 1

#### Acceptance Criteria

1. **FR-015** An expert SHALL answer to a role name and SHALL NOT need a remembered
   trigger phrase.
2. **FR-016** A skill SHALL exist only where a procedure has side effects outside the
   repository, and knowledge SHALL live in the contract.
3. **FR-017** WHEN an author proposes a new skill, THE author SHALL first state why the
   convention cannot live in the contract.

### Requirement 5: The licensing boundary

**Objective:** As a reader of this public repository, I want every artifact
publishable, so that nobody has to withdraw one later.

**Traces to:** User Story 2

#### Acceptance Criteria

1. **FR-018** Every artifact in this repository SHALL be publishable: no proprietary
   standard text, no controlled dictionary, and no organisation-specific operational
   detail.
2. **FR-019** IF a convention is specific to one organisation's systems, THEN it SHALL
   stay in that organisation's repository and cite the anchor.

### Requirement 6: The tracker

**Objective:** As an engineer picking up filed work, I want issues in one tracker, so
that the work has a single home.

**Traces to:** User Story 1

#### Acceptance Criteria

1. **FR-020** Linear SHALL be the tracker. WHERE a phase turns work units into tracker
   issues, that path SHALL target Linear.
2. **FR-021** THE repository SHALL NOT ship a GitHub-issue path. Spec Kit's own
   `speckit-taskstoissues` targets GitHub, so this repository drops it from the
   vendored set rather than leaving it installed and unused.

## Success Criteria

Each criterion names the command that checks it, or the word `judgement`.

**SC-001** One engineer who is not the author uses the contract on work the author did
not assign, within 60 days of release.
*Verifier:* judgement — a reader checks the commit and pull-request history of a
consuming repository for an author other than this one. No gate can see adoption. This
is the criterion V2 exists to satisfy; the others are subordinate to it.

**SC-002** Every anchor named in the contract resolves to a registry entry, or appears
in `writing/anchors/unregistered.txt` as a recorded debt. A name in neither fails the build.
*Verifier:* `writing/scripts/check-contract-anchors.sh`

The criterion is not yet met. The contract names 19 anchors and the registry holds 7 of
them. `writing/anchors/unregistered.txt` holds the other 12 as debt. That list only shrinks,
and a name leaves it by earning a registry entry with the four admission artifacts.

**SC-003** Every registry entry carries a verdict for the current default model.
*Verifier:* not built — no recognition suite exists. `writing/evals/recognition/README.md`
holds the method and nothing runs it.

Zero of the 8 entries carry a verdict. Until one does, every claim in this repository
about how well a model resolves an anchor is untested.

**SC-004** The gate reports zero errors on every governed artifact in this repository.
*Verifier:* `vale --no-global writing/`

**SC-005** The contract fits in a single file a person reads in under five minutes.
*Verifier:* `writing/scripts/check-artifact-length.sh`

**SC-006** No anchor page exceeds 150 lines. A longer one is evidence that it restates
the standard instead of citing it, which NOTICE.md forbids.
*Verifier:* `writing/scripts/check-artifact-length.sh`

The limit covers `anchors/*.md`. It used to read "no knowledge artifact", which swept in
the design documents at 231 and 194 lines. A design document argues a position; holding
it to a citation-length limit measures the wrong thing.

**SC-007** A reader can trace any finding to a clause without asking the author.
*Verifier:* judgement — a reviewer attempts it on three findings and reports.

## Migration

<!-- cites-private: the migration map names V1 skills because naming them is its job -->

V1 has 16 core skills. Eight rest on organisation-specific profiles and cannot move
to a public repository under FR-018 and FR-019.

| Disposition | Skills |
|---|---|
| **Convert to anchors and experts** in V2 | `idiomatic`, `systems`, `root-cause`, `xreview` |
| **Delete** — process rigor that a standard already covers | `audit-skill`, `author-skill`, `brevity`, `pr-quality` |
| **Stay in the organisation's repository** — Sei-local | `evm`, `kubernetes`, `platform`, `harbor-dev`, `gov-ops`, `validate-release`, `validator-platform`, `chaos-suite` |

Deletion is the point, not a cost. A skill that encodes rigor a public standard already
carries is the custom pattern this iteration exists to remove.

### FR-016 applied to this iteration's own work

The first artifacts built for V2 were two skills, written before this spec existed.
Checked against FR-016 afterwards:

- `/linear-ticket` passes. It creates issues in a tracker, a real external side effect.
- `/spec-kit` fails. It writes files inside the repository and has no side effect
  outside it, so its content is knowledge rather than procedure. It dissolves into
  the contract, and the
  skill goes.

Four further carry-overs came from V1 by habit rather than by decision. A guardrails
stanza and `SKILL.md` anatomy came from V1's template. `references/` directories
holding 280 lines of corpus, V1's category taxonomy, and a citation of a private skill
as an authority in a public document.

Nobody copied one deliberately. V1 sat open while this specification took shape. A strong
prior fires without announcing itself — the same silent substitution the anchor research
documents, applied to conventions instead of concepts. The private-citation gate exists
because of the fourth one.

## Assumptions

- Spec Kit's constitution is a stable slot. Its templates ship with the CLI, so the
  shape tracks upstream.
- The recognition suite runs against the models the team actually uses, not a fixed list.
- The four generic skills carry judgement that no lint rule can express, which is why
  they become experts rather than gates.
- `NOTICE.md` continues to govern what enters this repository.

## Out of scope

- The organisation-specific skills stay where they are. No generic rewrite here.
- V1 is not retired. It keeps its history and its Sei-local content.
- The recognition questions are not written here. This spec requires them; `plan.md`
  designs them.
- The file layout beyond the four channels is a `plan.md` decision.

## Open questions

1. **Where does the contract physically live** so that every harness loads it and
   not only one? The Spec Kit phases read its constitution. A root
   context file reaches the session. These are perhaps the same file.
2. **What is the failing threshold for a recognition test?** Published practice uses 80% and 50%
   bands. Adopting them without measuring our own anchors would be borrowing a number.
3. **Do the four generic skills become experts, or does one expert absorb several?**
   Four narrow experts have the same recall problem as four skills.
