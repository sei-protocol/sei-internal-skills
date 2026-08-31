# agentic-writing Constitution

This file is the contract. It loads in every session with no invocation, and that is
why a convention here reaches an engineer who is not looking for it. **A convention
absent from this file is not adopted.**

## Core Principles

### I. Anchor, contract, or delete

Every convention is one of three things. An **anchor** names a public standard the
model already holds: name it and spend no more words. A **contract** is a local rule,
or a term with a weak prior: state it in full. Anything that is neither gets deleted.

### II. Evidence before anchor

Naming a standard is a bet on its prior. A term becomes an anchor only after a
recognition test records that a model resolves it, with the model identifier and the
date. An untested term is a stated rule. Fame is not evidence.

### III. The gate is the claim

A rule no command checks is a wish. Every convention states whether a gate checks it,
and says so plainly when none does. Never weaken a gate to make a check pass. If Vale
cannot express a rule, delete it and record the constraint as uncheckable.

### IV. Push, not pull

A convention reaches an engineer through this file or through a gate. It does not
reach them through a command they have to remember. **A skill exists only where a
procedure has side effects outside the repository.** Knowledge lives here. An author
proposing a skill states first why the convention cannot live in this file.

### V. State the gap

A named standard fails three ways: the model substitutes openly, it substitutes
silently, or it invents. Only the first announces itself. Every anchor therefore
carries what it does **not** cover, and a partial verdict keeps its stated text alongside the anchor.

## The anchors

Name one from this table. **Naming an anchor absent from it is forbidden** — a
confabulated method name reads authoritative and costs more than plain prose.

**No anchor here carries a recorded verdict yet, and the suite that would produce one
is not built.** No suite exists to produce one, and none of this repository's gates
claims otherwise.
Until a verdict exists, treat a surprising output as the anchor failing rather than the
model disagreeing.

| Anchor | Governs | Does not cover |
|---|---|---|
| EARS | requirement syntax | whether the template fits the real class |
| RFC 2119 | normative keywords, uppercase | whether the obligation is the right one |
| INVEST | whether a story is a real slice | whether the slice delivers value |
| Gherkin | acceptance scenarios | whether the scenario is the important one |
| Cockburn use cases | a flow with triggers and extensions | whether the extensions are complete |
| arc42 | section order of a design | section quality |
| C4 model | diagram levels | where the boundary belongs |
| Domain-Driven Design | bounded context, ubiquitous language | where the boundary actually falls |
| Clean Architecture | dependency direction between contexts | layering inside one — see below |
| ADR (Nygard) | a decision record that supersedes | whether consequences state the real cost |
| TDD | test-first, red then green | which school suits this codebase |
| Property-Based Testing | invariants over generated inputs | finding the invariant |
| Conventional Commits | commit subject | whether the scope names the right component |
| BLUF | conclusion first | whether that sentence is the bottom line |
| Diátaxis | one page, one mode | whether the author chose the mode well |
| Effective Go | Go idiom | modules and generics — it predates both |
| Go Code Review Comments | review-time Go checklist | design-level structure |
| Google Go Style Guide | normative Go rulings | this repository's own patterns |
| Code Smells | surface signs of design trouble | whether the fix is worth it |

**Clean Architecture carries a documented criticism.** Bogard and Comartin argue the
indirection does not pay, because most changes traverse every layer anyway. It also
collides with Go idiom, where three similar lines beat a premature helper. Use it for
dependency direction between bounded contexts. Do not impose it inside one.

## Stated in full

These have no reliable public prior. Naming them is not enough.

**Writing.** Write in Simplified Technical English (ASD-STE100). Use approved words in
one meaning only, the active voice, and one instruction per sentence. Keep a procedural
sentence under 20 words and a descriptive one under 25. Keep a noun cluster to at most
three words. Keep code, commands,
identifiers, and quoted output verbatim.

**Code structure.** Code reads as a legible sequence of named steps a new engineer
follows top to bottom with no narrator. The method body is the table of contents; step
names carry the *what*; you drill into a step only for its detail. A refactor for
readability changes structure only, and the unchanged tests still passing is the proof.

**Comments.** A comment states the present. Never history, never why-removed — that
belongs in the commit. Put it at the top, as package, file, or type documentation.

**Errors are interface.** Every error condition is part of the public contract.

**Two-way doors only.** A one-way door needs explicit human approval before you finalize
it. A one-way door is a persisted schema, a public API contract, a wire format, or
anything another system comes to depend on.

## Writing modes

Four artifacts carry a structure contract. Ordinary prose carries the prose rules only.

| Artifact | Path | Gate checks |
|---|---|---|
| Design | `docs/design/**` | Non-goals, Alternatives, Trade-offs, Open questions |
| Spec | `specs/**` | Semantic Anchors, Success Criteria, Independent Test |
| Ticket | `tickets/**` | the seven sections of the body |
| Procedure | `docs/procedures/**` | 20-word sentences, imperative steps |

Run `vale <path>`. Exit code 0 means "no finding at or above the gate". It does not mean
compliant.

## The spec contract

A specification uses Spec Kit's filenames and its spec template. Nine deltas apply to
that template, each fixing something upstream leaves to the author.

`check-template-deltas.sh` asserts all nine. The count matters: the third principle
above says a convention absent from this file is not adopted, and four of these were
missing from the table while CI failed on them.

| Delta | In | Fixes |
|---|---|---|
| `## Semantic Anchors`, each with a *does not cover* column | `spec.md` | The body restates the method without it |
| `## Glossary` | `spec.md` | An agent reads linearly and cannot ask what a term means |
| `## Boundary Context` | `spec.md` | A spec with no stated boundary grows while open |
| `**Objective:** As a <role>, I want <X>, so that <Y>` | each requirement | Names the beneficiary; prevents an orphan requirement |
| EARS with a named actor — `THE Controller SHALL` | each requirement | `System MUST` names no actor |
| `### Requirement N:` | `spec.md` | upstream carries a flat list, so criteria have no owner |
| `**Traces to:**` | each requirement | a requirement that serves no story is an orphan |
| `#### Acceptance Criteria` | each requirement | the heading EARS-CriterionShall keys on |
| `*Verifier:*` | each success criterion | a criterion nothing checks is a wish |

`writing/scripts/check-template-deltas.sh` asserts all five, and CI runs it.

**Three deltas for `plan.md` used to sit in this table** — Boundary Commitments,
Revalidation Triggers, Existing Architecture Analysis. No plan template ever carried
them. The gate reads `spec-template.md` only, so nothing caught the claim. This
table drops the three rather than making them true after the fact. To restore one,
write the section into a plan template and add the row back in the same change.

**Every success criterion names its verifier.** `SC-002 … Verifier: gorelease in CI`.
A criterion nothing checks says `judgement`. An unmarked criterion is not honest.

**Every user story carries four things** — priority, why this priority, an Independent
Test, and acceptance scenarios. The generator builds a ticket from them, and it cannot
invent what the story omitted.

**Every task carries five** — a test-first instruction, an `Observable:` check,
`_Requirements:_` upward, `_Boundary:_`, and `_Depends:_`.

**Never invent a requirement.** An unstated detail becomes
`[NEEDS CLARIFICATION: <the question>]`. A plausible default written silently into a
spec is the failure the artifact exists to prevent.

**`spec.md` holds what and why only.** Naming a library, a schema, a signature, or a
file path moves the line to `plan.md`.

## Governance

Precedence, highest first: a direct instruction in the conversation; the repository's
own instruction file; this file.

This repository is public. Every artifact in it is publishable: no proprietary standard
text, no controlled dictionary, no organisation-specific operational detail. A
convention specific to one organisation's systems stays in that organisation's
repository and cites the public anchor from there.

**No anchor here cites a skill in this repository as its authority.** An anchor earns
its place by being a standard somebody else publishes, so a reader can follow the name
outside this repository. A gate for this arrives later in the series; today the rule
stands on review.

**A success criterion names a verifier that runs, or says that none does.** Write the
path in backticks, or write `not built — <what is missing>` or `judgement — <who
decides and how>`. A criterion citing a check nobody built reads exactly like one that
passes, which is the failure this repository exists to stop. The gate that asserts it
arrives later in the series; today the rule stands on review.

### Admitting an anchor

An anchor arrives with four artifacts or it does not arrive. Naming a standard is cheap,
and a catalogue that grows by naming becomes a list of things nobody checks.

1. **A registry entry** — steward, licence, the recognition test, and what the anchor
   does not cover.
2. **A fixture per rule** — its own configuration, its own input, and a non-empty
   golden file, so the rule cannot die silently. A directory missing a piece fails;
   it is never skipped.
3. **A coverage row** — which topics of the standard the rules reach, and which they do
   not. `false` is an expected answer.
4. **A false-positive count** — measured over a corpus before any rule reaches `error`.
   A rule that fires often starts at `warning`, and the number says which.

The registry marks every anchor that predates this rule `grandfathered`, and
`writing/anchors/grandfathered.txt` lists them. They are exempt, and the list only shrinks.

The gate that counts them, compares the list against `main`, and holds an `admitted`
anchor to all four artifacts is `check-admission.sh`. It arrives later in this series,
and that file says the same at its head. Until it lands the rule stands on review, and a
line added here rather than removed goes unnoticed.

This is what keeps the slope from being a slope. The same four artifacts bound the next
anchor, or it does not go in.

An amendment states what changed and why, and bumps the version below. Deleting a
principle requires the same ceremony as adding one.

**Version**: 1.3.0 | **Ratified**: 2026-08-19 | **Last Amended**: 2026-08-28
