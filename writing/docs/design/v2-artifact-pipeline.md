# The V2 artifact pipeline

**Status**: Draft · 2026-08-19 · Author: Brandon Chatham

## Background

V1 of the skill library reached one user in five months. Spec 001 argues the cause is
distribution rather than size, and it names four channels: contract, evidence, gate, and
expert.

This document decides the **artifact shape** those channels govern. It takes the shape
from Kiro rather than inventing one, because Kiro's spec-driven flow is the one this team
has used and liked.

**Provenance.** Kiro's public documentation states the three-file structure and does not
publish the section headings. The shape below therefore comes from reading current
Kiro-lineage output in public repositories, and from this machine's own older Kiro specs. One of those
repositories names `cc-sdd`, a Kiro-compatible community tool, in its brief. Treat the
shape as Kiro-lineage rather than as an official specification, and re-read it when Kiro
publishes one.

## Goals

1. One artifact hierarchy, from system design down to a task, with every link checkable.
2. Take Kiro's stronger internals while keeping Spec Kit's filenames, CLI, and phase
   skills.
3. Anchor each artifact to public standards, and state where no anchor exists.
4. Give the escalation path a mechanism rather than a convention.

## Non-goals

- Spec Kit stays. Its filenames and CLI keep upstream tracking intact.
- Kiro the tool is out of scope. This takes the shape, not the runtime.
- The organisation-specific skills stay as they are. Spec 001 excludes them.
- Diagram-as-code for every diagram. See the trade-off on Excalidraw below.

## Design

### The hierarchy

Directory-encoded, so a misfiled artifact is visible without reading it, and so a Vale
section glob can gate each level.

```
docs/design/<system>/
  hld.md                        arc42 structure · C4 levels · declares sub-domains
  <sub-domain>/
    specs/<NNN>-<slug>/
      spec.md                   what and why
      plan.md                   how
      tasks.md                  work units
      state.json               phase and approvals
```

A sub-domain is a DDD bounded context. The HLD declares its sub-domains; a spec sits
inside the sub-domain it implements. The path is the relationship.

### The artifacts and their anchors

<!-- vale AgenticWriting.STE-NounCluster = NO -->
| Artifact | Semantic Anchors | Stated in text, no anchor |
|---|---|---|
| `hld.md` | arc42 · C4 model · Domain-Driven Design | the sub-domain declaration |
| `spec.md` | EARS · RFC 2119 · Connextra · INVEST · Gherkin · DDD ubiquitous language | boundary commitments |
| `plan.md` | arc42 · Clean Architecture · ADR (Nygard) | revalidation triggers |
| `tasks.md` | TDD · Property-Based Testing · ISO/IEC/IEEE 29148 traceability | the task annotations |
| implementation | Effective Go · Go Code Review Comments · Google Go Style Guide · Code Smells | the step-structure rule, stated in the contract |
<!-- vale AgenticWriting.STE-NounCluster = YES -->

Effective Go carries one documented gap: it predates modules and generics, so it says
nothing about either.

### What we take from Kiro

Seven deltas on the vendored Spec Kit templates. Each one earns its place by fixing
something the upstream template leaves to the author.

| Delta | Goes in | Fixes |
|---|---|---|
| `## Glossary` — every term defined once | `spec.md` | An agent reads linearly and cannot ask what a term means. |
| `## Boundary Context` — what this spec is inside | `spec.md` | A spec with no stated boundary grows while it is open. |
| `**Objective:** As a <role>, I want <X>, so that <Y>` | each requirement | Names the beneficiary. Spec Kit separates stories from requirements and permits an orphan requirement. |
| Acceptance criteria in EARS **with a named actor** | each requirement | `THE Controller SHALL...` names who acts. `System MUST` does not. |
| `## Boundary Commitments` — Owns · Out of Boundary · Allowed Dependencies | `plan.md` | Makes the dependency rule of Clean Architecture explicit and reviewable. |
| `### Revalidation Triggers` | `plan.md` | The escalation mechanism. See below. |
| `### Existing Architecture Analysis` | `plan.md` | Forces the author to read the as-is before writing the to-be. This is the section that prevents a confident description of a system nobody looked at. |

### The task annotations

The strongest borrow. Every task carries five things:

```
- [ ] 2.1 (P) Route the scoped parser through the shared extractor
  - Write the failing unit tests first, covering precedence, guards, and casing
  - Observable: the integration suite passes including the new header test
  - _Requirements: 1.1, 1.2, 4.1_
  - _Boundary: parserForAccessToken_
  - _Depends: 1.1_
```

| Annotation | Purpose | Anchor |
|---|---|---|
| `Observable:` | The falsifiable check. An Independent Test at task level. | — |
| `_Requirements:_` | Traceability up to the acceptance criteria | ISO/IEC/IEEE 29148 |
| `_Boundary:_` | Which component the task touches | Clean Architecture · DDD |
| `_Depends:_` | Ordering constraint | — |
| `(P)` | Safe to run in parallel | — |

The task text states test-first, rather than leaving it to a policy elsewhere.

### Escalation

The pipeline holds the design and the spec constant. A blocker escalates, and
`### Revalidation Triggers` is the mechanism: the plan states, up front, the conditions
that force a design revisit. A blocker matching a trigger produces an ADR that supersedes
the earlier decision, because an ADR supersedes and never deletes.

`state.json` records the phase and the per-phase approval, so "approved" is a recorded
fact with a date rather than a recollection.

### The gates

Each level gets a Vale mode, scoped by its path. Three modes exist. Two are new.

| Mode | Path | Checks |
|---|---|---|
| Design (exists) | `docs/design/**` | Non-goals · Alternatives · Trade-offs · Open questions |
| Spec (exists) | `writing/specs/**` | Semantic Anchors · Success Criteria · Independent Test |
| HLD (new) | `docs/design/*/hld.md` | Sub-domains declared |
| Plan (new) | `**/plan.md` | Boundary Commitments · Revalidation Triggers · Existing Architecture Analysis |

Traceability needs a script rather than a lint rule. A Vale rule has no way to confirm that
every task's `_Requirements:_` resolves to a real acceptance criterion, because it cannot
compare two token sets.

## Alternatives

**Adopt Kiro wholesale.** Rejected. It abandons the vendored CLI, the nine phase skills,
and upstream tracking, in exchange for filenames. The internals carry the value and they
port without the runtime.

**Keep Spec Kit unchanged.** Rejected. Its template writes `System MUST [capability]`,
separates stories from requirements, so it allows an orphan requirement, and has no
glossary, no boundary statement, and no task traceability. Every one of those gaps is a
place an implementer invents something.

**Flat specs with a domain field.** Rejected as the primary encoding. A field needs a
script to validate; the glob that gates a path validates it. A later change may still add
the field as redundancy.

**Clean Architecture as the layering inside a sub-domain.** Not adopted as a default. Its
own anchor page carries the criticism: Bogard and Comartin argue the indirection does not
pay because most changes traverse every layer, and Bogard offers Vertical Slice
Architecture instead. It also collides with this team's own Go idiom rule that three
similar lines beat a premature helper. The design takes Clean Architecture for the dependency
direction between sub-domains, and it is not imposed within one.

## Trade-offs

**Seven deltas is a maintenance surface.** Every delta is a place where the vendored
template and our contract can diverge when Spec Kit ships a new version. The mitigation
is that one reference file states each delta once, and a rule that fails
when the section disappears.

**A directory hierarchy is rigid.** Moving a spec between sub-domains is a directory
move, and the path is load-bearing. That rigidity is the point: it is what makes the
relationship checkable without a script.

**Excalidraw is not verifiable.** C4's named tooling is Structurizr, and diagram-as-code
stays true because it is text. An Excalidraw scene drifts from the system silently, which
is the failure this repository exists to prevent. The C4 levels that must stay true are
therefore Mermaid, which is text, diffable, and renders in the places we read. Excalidraw stays
reserved for the one genuinely complex subsystem where expressiveness wins and the drift
we accept knowingly.

**Actor-named EARS is more work to write.** `THE Controller SHALL` requires the author to
decide who acts before writing the sentence. That is the cost, and it is also the benefit.

## Open questions

1. **Does `state.json` belong in this repository or in the harness?** A phase-and-approval
   file is state, and this repository has held no state until now.
2. **Who validates `_Requirements:_` resolution?** This needs a script. Whether it lives in
   CI here or in the consuming repository is undecided.
3. **Does the HLD need `## Sub-domains` as a heading, or is the directory listing the
   declaration?** A heading is checkable; a directory listing cannot drift.
4. **Glossary and Boundary Context are separate sections in different Kiro generations.**
   Whether the design needs both, or one subsumes the other, needs a worked example.

## References

- `writing/specs/001-anchored-agentic-tooling/spec.md` — the four channels this shape fills.
- `writing/docs/writing-modes.md` — the existing three gates and how to add a fourth.
- `writing/anchors/registry.yaml` — steward, licence and recognition status per anchor.
- Semantic Anchors — <https://llm-coding.github.io/Semantic-Anchors/>
- Kiro documentation — <https://kiro.dev/docs/cli/>
