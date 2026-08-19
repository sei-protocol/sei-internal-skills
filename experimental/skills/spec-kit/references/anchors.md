# Vetted anchors for a specification

Name an anchor from this file. Do not name one that is not here.

**Probe status: none of these carry a recorded verdict yet.** They are drawn from
a public catalogue, not from a measured probe against the model writing the spec.
Until a verdict exists, treat a surprising output as the anchor failing, not as
the model disagreeing. The three failure modes are transparent substitution,
silent substitution, and confabulation; only the first announces itself.

Catalogue: <https://llm-coding.github.io/Semantic-Anchors/>

## Requirements — for `spec.md`

| Anchor | Governs | Does not cover |
|---|---|---|
| **EARS** | Requirement syntax: ubiquitous, `WHEN`, `IF..THEN`, `WHILE`, `WHERE` | Whether the chosen template matches the real requirement class |
| **RFC 2119** | Normative keywords in uppercase: `MUST`, `SHOULD`, `MAY` | Whether the obligation is the right one |
| **INVEST** | Story quality: independent, negotiable, valuable, estimable, small, testable | Whether the slice delivers value to anyone |
| **MoSCoW** | Priority bands | Which band a given item belongs in |
| **Gherkin** | Acceptance scenarios as Given / When / Then | Whether the scenario is the one that matters |
| **Cockburn fully-dressed use cases** | A flow with triggers, preconditions, main flow, extensions | Whether the extensions are complete |

## Architecture — for `plan.md`

| Anchor | Governs | Does not cover |
|---|---|---|
| **arc42** | Section order of a design document | Section quality |
| **C4 diagrams** | Diagram levels: context, container, component, code | Whether the boundary is drawn in the right place |
| **ADR (Nygard)** | Decision record: Status, Context, Decision, Consequences | Whether Consequences states the real cost |
| **Clean Architecture** | Dependency direction | Whether the layering earns its cost here |
| **Hexagonal Architecture** | Ports and adapters | Which side of the port a concern belongs on |
| **Domain-Driven Design** | Bounded context, ubiquitous language | Where the context boundary actually falls |

## Work units — for `tasks.md`

| Anchor | Governs | Does not cover |
|---|---|---|
| **INVEST** | Whether a task is a real slice | Ordering under real constraints |
| **Conventional Commits** | Commit subject: `type(scope)!: description` | Whether the scope is the right component |
| **SemVer** | Version increment meaning | Whether a change is genuinely breaking |

## Communication — applies to all three artifacts

| Anchor | Governs | Does not cover |
|---|---|---|
| **BLUF** | Conclusion in the first sentence | Whether that sentence is the real bottom line |
| **MECE** | Decomposition that does not overlap and leaves no gap | Whether the categories are the useful ones |
| **Pyramid Principle (Minto)** | Claim first, support beneath | Whether the support is sufficient |
| **Progressive Disclosure** | Detail revealed as the reader needs it | Which detail is needed first |

## Verification — cite when the spec makes a testing claim

| Anchor | Governs | Does not cover |
|---|---|---|
| **Testing Pyramid** | Distribution of test sizes | Whether the tests assert the right thing |
| **TDD (London / Chicago)** | Test-first, with mocks or with real collaborators | Which school suits this codebase |
| **Property-Based Testing** | Invariants over generated inputs | Finding the invariant |
| **OWASP Top 10** | Web risk categories | Whether the threat model applies here |
| **STRIDE** | Threat categories | Severity in this system |

## Cite, but do not treat as an anchor

**ISO/IEC/IEEE 29148:2018** — the requirements engineering standard. Clause 6.4
covers converting stakeholder requirements into testable work items and user
stories. It recommends a traceability matrix from each requirement to its
origin, design, and tests.

Paywalled, so cite-and-link only. Being a real standard is not evidence of a
dense prior. Until a probe says otherwise, state the rule you want rather than
naming the clause and hoping.

## Not anchors — state these in text

These have no reliable public prior, or they are ours. Writing the name alone
is not enough; the rule has to appear in the document.

- **ASD-STE100** — absent from the public catalogue. The writing contract states
  it in full. `vale` checks the part of it that is checkable.
- **Any Sei convention** — the platform profile, the controller conventions, the
  ticket format. A convention that overrides a public standard is precisely the
  knowledge no model holds.
- **A repository's own pattern** — it outranks generic idiom and must be shown.
