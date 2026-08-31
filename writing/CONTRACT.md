# agentic-writing Constitution

This file is the contract. **A convention absent from this file is not adopted.**

Nothing loads it automatically yet. The design is that a session picks it up with no
invocation. That is the only way a convention reaches an engineer who is not looking
for it. `AGENTS.md` gains that pointer later in this series.

Until it does, this file reaches a reader who goes looking. That is the pull model
Principle IV argues against, so read Principle IV as the target rather than as a
description of today.

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

**No anchor here carries a recorded verdict, and the suite that would produce one is not
built.** Until a verdict exists, treat a surprising output as the anchor failing rather
than the model disagreeing.

This table therefore predates Principle II and stands on review, not on evidence. `ears`
is the one registry entry marked `admitted`, and its `verified` list is empty, so it does
not meet the precondition Principle II states. The registry grandfathers the other seven
entries, which exempts them. The next anchor admitted needs the recognition verdict.

<!-- Every cell in the first column is a citation title. STE-NounCluster reads a
     title as a noun cluster and cannot judge one, and the rule file names this
     directive as the escape hatch for that case. It covers the table alone. -->
<!-- vale AgenticWriting.STE-NounCluster = NO -->

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

<!-- vale AgenticWriting.STE-NounCluster = YES -->

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

Vale gates the writing convention above. No gate reads code, so the four below it —
code structure, comments, errors as interface, and two-way doors — stand on review alone.
Principle III asks each to say so, and this sentence says it for all four.

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

Five artifacts carry a structure contract. Ordinary prose carries the prose rules only.
Every rule named below runs at `error`, and `.vale.ini` holds the paths.

| Artifact | Path | Gate checks |
|---|---|---|
| Spec | `specs/**/spec.md` | Semantic Anchors, Success Criteria, Independent Test, Acceptance Criteria, an EARS criterion, uppercase RFC 2119 keywords |
| Design | `{docs/design/**/*.md,designs/**/*.md}` | Non-goals, Alternatives, Trade-offs, Open questions, arc42 section order |
| ADR | `docs/adr/*.md` | Status, Context, Decision, Consequences |
| Ticket | `tickets/**/*.md` | the seven sections of the body |
| Procedure | `docs/procedures/**/*.md` | 20-word sentences, imperative steps |

Run `vale <path>`. Exit code 0 means "no finding at or above the gate". It does not mean
compliant.

## The spec contract

A specification uses Spec Kit's filenames and its spec template. Nine deltas apply to
that template, each fixing something upstream leaves to the author.

`check-template-deltas.sh` asserts all nine, in ten checks: the anchor row takes
two, one for the heading and one for the *does not cover* column, because the
column is the half Principle V makes load-bearing. The count matters. The opening line of this
file says a convention absent from this file is not adopted. A delta the gate enforces and
this table omits is therefore a rule nobody agreed to.

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


**This table covers `spec.md` alone.** `check-template-deltas.sh` reads
`spec-template.md` and no other template, so a row for a plan template would state a
delta no gate checks. To add one, write the section into a plan template and extend the
gate to read that template, in the change that adds the row.

The five conventions below hold for a specification and the files beside it. `.vale.ini`
scopes the structure rules to `specs/**/spec.md`; a plan or a tasks file gets the prose
rules and nothing more. Each convention therefore says what checks it.

**Every success criterion names its verifier.** `SC-002 … Verifier: gorelease in CI`.
A criterion nothing checks says `judgement`. An unmarked criterion is not honest. The
gate that asserts it arrives later in the series; today it stands on review.

**Every user story carries four things** — priority, why this priority, an Independent
Test, and acceptance scenarios. The generator builds a ticket from them, and it cannot
invent what the story omitted. `Spec-IndependentTest` checks that the words
`**Independent Test**` appear once in `spec.md`. The other three stand on review.

**Every task carries five** — a test-first instruction, an `Observable:` check,
`_Requirements:_` upward, `_Boundary:_`, and `_Depends:_`. No structure rule reads a
tasks file. This one stands on review in full.

**Never invent a requirement.** An unstated detail becomes
`[NEEDS CLARIFICATION: <the question>]`. A plausible default written silently into a
spec is the failure the artifact exists to prevent. No gate reads the marker, because
Vale cannot tell a needed clarification from an absent one.

**`spec.md` holds what and why only.** Naming a library, a schema, a signature, or a
file path moves the line to `plan.md`. No gate checks it. The line between what and how
is a judgement, and Principle III says to record such a rule as uncheckable.

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

The registry holds eight entries. Seven carry `admission: grandfathered` and
`writing/anchors/grandfathered.txt` lists them. They are exempt, and the list only shrinks.
`ears` is the eighth and the only one marked `admitted`.

The anchor table above names nineteen. Seven of them hold a registry entry. The other
twelve hold none, and `writing/anchors/unregistered.txt` names all twelve. That file
records the debt rather than implying it. A name leaves it by earning an entry with the
four artifacts above, in the change that deletes its line. `asd-ste100` holds an entry
and the table does not name it, because this contract states ASD-STE100 directly.

The gate that counts them, compares the list against `main`, and holds an `admitted`
anchor to all four artifacts is `check-admission.sh`. It arrives later in this series,
and that file says the same at its head. Until it lands the rule stands on review, and a
line added here rather than removed goes unnoticed.

This is what keeps the slope from being a slope. The same four artifacts bound the next
anchor, or it does not go in.

An amendment states what changed and why, and bumps the version below. Deleting a
principle requires the same ceremony as adding one.

**Version**: 1.3.0 | **Ratified**: 2026-08-19 | **Last Amended**: 2026-08-28
