# Closing the spec loop

**Status**: Draft · 2026-08-24 · Author: Brandon Chatham

## Background

The V2 artifact pipeline decided the artifact shape. This document covers what
happens to those artifacts once people use them. It reports one full cycle rather
than stating an intent.

That cycle ran in sei-load between 2026-08-22 and 2026-08-24. It produced a
specification of 23 requirements, 10 acceptance scenarios and 10 success
criteria. It also produced five Linear tickets, one pull request of 3000 lines,
and two rounds of independent review. The feature shipped. The process also
failed in four ways that no gate caught, and those failures share one shape.

**Every artifact agreed with every other artifact, and four of them were wrong
together.**

- The specification's stated problem was a metrics problem: *"a metric series
  spanning two restarts describes two different contracts"*. Not one of its 23
  requirements mentioned metrics. A run satisfying every requirement leaves the
  motivating problem open, and that is what shipped.
- An acceptance scenario said a failed run has deployed nothing. The
  specification's own design section could not satisfy it for a profile of more
  than one scenario. The contradiction survived specify, plan, tasks,
  implementation, and the author's tests.
- The verification plan said *"assert both builders complete"*. Both complete in
  exactly the case the requirement forbids.
- Every requirement had a test naming its ID. Three of those tests passed by
  sharing the code's own wrong assumption.

Independent review found all four, and found them **after** implementation. Four
blinded reviewers dissented. One reproduced seven defects with working examples. That
is the process working, late.

Three of the author's own defects also passed the whole mechanical verifier
chain: `gofmt`, `go vet`, `staticcheck`, `golangci-lint`, and a full test suite.
One was an orphaned method that compiled because nothing required it. One was a
frozen invariant a regex deleted from a comment. One was a test that agreed with
the bug. Clean linters are evidence about syntax. They say nothing about
comments, and nothing about whether a test exercises the real path.

**The second gap is where feedback lives.** Review comments on a specification
reached three surfaces, and each one loses something. A GitHub pull request takes
no comment after it merges. A Linear ticket description is not a conversation. A
published artifact takes comments and sits outside the repository.

The cycle published its specifications as artifacts for that reason. It then
closed the pull request that would have committed them, because the team had not
decided whether Spec Kit is the standard. The specification is now correct in two
places and merged in neither.

## Goals

1. Move falsification earlier than review, without adding a phase.
2. Give feedback on an artifact a surface that survives the artifact's merge.
3. Keep the doctrine small enough that a reader holds all of it.

## Non-goals

- Replacing Spec Kit. Its phase order and filenames stay.
- Replacing independent review. Review found what the gates could not, and this
  design assumes it continues.
- A rule for every failure. Three of the four failures above need judgement, and
  a linter that pretends otherwise adds noise and false confidence.

## Design

### Three planes, one registry

The framework already has two planes and one source of truth. This adds a third.

```
                    anchors/registry.yaml
                             |
        +--------------------+--------------------+
        |                    |                    |
   AUTHORING           VERIFICATION          COLLABORATION
   Spec Kit            Vale + tests          session host
        |                    |                    |
   spec, plan,         style rules,         shared session,
   tasks, tickets      cross-refs,          human + agent,
        |              mutation score       durable comment
        |                    |                    |
        +--------------------+--------------------+
                             |
                          artifact
```

The registry stays the only place a human edits an anchor. A generator derives
both the authoring hint and the verification rule from it, so the two cannot
drift. The collaboration plane consumes both and adds no vocabulary of its own.

### What each plane owns

**Authoring — Spec Kit and Linear.** Produces the artifacts and files the work.
`/linear-ticket` already sources ticket acceptance criteria from `AS-` and `SC-`
IDs in the specification, so a ticket cannot invent its own bar. That worked and
stays.

**Verification — Vale and tests.** Gates what a machine can check, and states
plainly what it cannot. The registry's `verifier.coverage` field already carries
`none` as a legitimate value. Two of eight anchors declare it.

**Collaboration — a session host.** Holds a session that people and agents share.
A reader leaves feedback as a message in that session rather than as a comment on
a pull request that will close. Agentic orchestration runs from the same chat
surface, and that surface is where someone dispatches independent review by hand
today.

Any host that holds a durable shared session satisfies this plane. The one an
organisation picks is a decision for that organisation's own repository, so this
document names the role rather than the product.

### The change that matters most

**Falsification moves from review into authoring.** Someone trying to break the
artifact found all four failures, and nothing before review was trying. Two
anchors from the Semantic Anchors catalogue name this discipline. Both sit at the
catalogue's highest confidence tier:

- **Red/Green TDD** carries the rule as a clause: *"Watch it fail: verifying that
  the test actually fails before implementation catches tautological tests and
  typos in the test itself."* In the cycle above, the assertions proven failing
  first all held. The ones not proven were the ones that were false.
- **Mutation Testing** reframes traceability from coverage to evidence: *"Are
  tests good enough, not is coverage high enough."* It is the rare anchor that is
  also a verifier, so it satisfies the rule that a claim of done names a command.

Neither needs a new gate. Both change what done means in a task.

### The change that needs no anchor

Nothing in a 195-entry catalogue names requirements-closure against the problem
statement. The catalogue's own doctrine covers that case. A concept with a thin
prior belongs in a contract rather than an anchor. This one is a single sentence:

> A requirement set MUST close its own problem statement. Name the requirement ID
> that addresses each claim in the problem section, or record the claim as out of
> scope.

That is checkable by a reader in one pass and by no linter, and the contract
should say so.

## What this breaks into

Three specifications, in dependency order. Each is independently valuable, and
the first two do not depend on a session host existing.

**Spec A — Spec Kit and Vale.** The authoring and verification planes. It adopts
the two anchors, adds the closure clause, and decides which of the four failures
earns a rule and which earns a prompt. Its hardest question is where a check
belongs. Vale is a prose linter, so cross-reference integrity may be a repository
test rather than a style rule. Its output is a smaller doctrine, not a larger
one.

**Spec B — Session host integration.** The collaboration plane. It decides three things. How a session
carries durable feedback for an artifact that later merges. How a chat interface
dispatches and records a review round. What the session stores rather than git. Its hardest question is the boundary between a session and a
repository. Getting that wrong makes the repository optional, and the repository
is what makes the work auditable.

**Spec C — Tool integrations.** How the tools already in use bind to both planes.
Four of them matter here: one runs independent multi-specialist review, one files
tickets from a specification, and two review code, for step structure and for
language idiom. Its hardest question is precedence. All four state overlapping
rules today, and a doctrine file resolves the overlap by hand.

Naming those four adds nothing a reader outside one organisation can act on. What
transfers is the shape. Once more than one tool carries rules, something has to
say which rule wins, and a hand-maintained file is the weakest answer available.

## Alternatives

**Add a phase to Spec Kit.** A "falsify" phase between plan and tasks would catch
the acceptance-versus-design contradiction. Rejected for now. A phase costs
process on every feature, and one question in the plan phase catches the same
thing. Revisit if that question proves too easy to skip.

**Lint the four failures.** Tempting, because it needs no human. Rejected for
three of the four. Requirements closing over a problem, a scenario contradicting
a design, and a verification entry naming a real failure are all judgements. A
rule for any of them fires on a good specification and passes a bad one, which is
worse than no rule.

**Keep feedback in GitHub.** Cheapest, and the cycle tried it. It failed in one
specific way. A specification's review comments matter most after that
specification merges, and GitHub closes the surface at merge.

## Trade-offs

**Two more anchors is two more terms.** The registry's rule is that an anchor
needing a paragraph is not an anchor. Both proposed anchors state in one
sentence, and both sit at Tier 3. The doctrine still grows. The offset is that
they replace house phrasing the team already uses.

**Mutation testing is a real cost.** A mutation run is slower than a test run by
an order of magnitude. This design proposes it as a claim a task can make, not as
a gate on every commit.

**A session surface can weaken the repository.** A decision that lives only in a
session stops the repository being the record. Spec B draws that line, and the
failure mode is quiet.

## Open questions

1. ~~Which of the four failures is machine-checkable, and in which tool.~~
   **Settled by ADR 0002.** Vale validates prose and enforces the template. It
   checks nothing that needs an open set, so none of the four failures gets a
   Vale rule. ISO/IEC/IEEE 29148:2018 clause 5.2.7 is the one lintable clause
   with authority behind it, and it is a prose rule. Spec A can start.
2. Whether `YAGNI` and `KISS` stay in the doctrine. The catalogue rates YAGNI at
   its lowest confidence tier, and KISS is absent from the catalogue entirely
   under any spelling. Both currently cost words in Lane 3.
3. Whether a specification belongs in the repository it describes. This cycle
   published to an artifact and closed the committing pull request. That decision
   is still open, and Spec B changes its answer.
4. What the session stores. Named here because it is the boundary question, and
   answered in Spec B.

## References

- `docs/design/v2-artifact-pipeline.md` — the artifact shape this extends
- `docs/adr/0001-anchors-plus-linter-not-prompts.md` — why both, and the seam
- `docs/architecture.md` — goals, constraints, and the registry as source of truth
- `writing/anchors/registry.yaml` — the schema any new anchor must fit
- Semantic Anchors catalogue: https://llm-coding.github.io/Semantic-Anchors/ —
  Mutation Testing, Red/Green TDD, and the contracts-not-anchors rule for thin
  priors
- sei-load PR #66 and Linear PLT-1060 — the cycle this document reports
