# sei-internal-skills Constitution

This file is the contract for work on this repository. Every Spec Kit phase reads it.
**A convention absent from this file is not adopted here.**

It governs the artifacts in this repository: skills, agents, Omnigent bundles, and the
sync machinery. It does not govern the systems those artifacts operate on.

## Core Principles

### I. Anchor, contract, or delete

Every convention is one of three things. An **anchor** names a public standard the model
already holds: name it and spend no more words. A **contract** is a local rule, or a term
with a weak prior: state it in full. Anything that is neither gets deleted.

### II. Evidence before anchor

Naming a standard is a bet on its prior. A term becomes an anchor here only after a probe
records that a model resolves it, with the model identifier and the date. An unprobed term
is a contract. Fame is not evidence, and neither is presence in a public catalogue.

### III. The gate is the claim

A rule no command checks is a wish. Every convention states whether a gate checks it, and
says so plainly when none does. Never weaken a gate to make a check pass. Delete a rule you cannot express, and
record the constraint as uncheckable.

### IV. The surface that reaches an engineer wins

A convention reaches an engineer through an always-loaded file, through a role name, or
through a gate. It does not reach them through a trigger phrase they must remember.
**A skill exists only where a procedure has side effects outside this repository.**
Knowledge belongs to the agent that applies it or to the contract.

An author proposing a new skill MUST first state why the knowledge cannot live in an
agent or in this file.

### V. State the gap

A named standard fails three ways: the model substitutes openly, it substitutes silently,
or it invents. Only the first announces itself. Every anchor therefore carries what it
does **not** cover, and a partial verdict keeps its stated text alongside the anchor.

### VI. Two tiers, one move

`.claude/` is the shipped core. `experimental/` is everything else. The exclusion is
structural: the sync scripts read `.claude/` only. Promoting and parking are the same
`git mv`. No third list exists.

A resource belongs in the core on two conditions. An engineering team outside its author
reaches for it on ordinary work. Changing it is a considered act.

### VII. Name the widening

A change that widens what an agent can reach names three things: the widening, the gate
that bounds it, and the blast radius if that gate fails. `.claude/skills/` is not only a
menu an engineer picks from. It is the unconditional discovery scope of a headless agent
that approves its own tool calls, so a change there is a change to a trust boundary.

An unset `tools:` key on an agent is a grant of every tool. State the grant.

## The anchors

Name one from this table. **Naming an anchor absent from it is forbidden.** A confabulated
method name reads authoritative and costs more than plain prose.

**No anchor here carries a recorded probe verdict yet.** Until one does, treat a surprising
output as the anchor failing rather than the model disagreeing.

| Anchor | Governs | Does not cover |
|---|---|---|
| Google SRE | SLO, SLI, error budget, toil | whether the target is the right one |
| AWS Builder's Library | timeouts, retries, back-pressure, jitter | whether the failure mode applies here |
| OpenTelemetry semantic conventions | attribute and metric naming | whether the signal is worth emitting |
| Google AIP | API resource shape and versioning | whether the resource is the right one |
| Kubernetes API conventions | CRD field shape, status, conditions | whether the controller reconciles correctly |
| controller-runtime | reconcile loop, client cache, owner refs | idempotence of a specific reconcile |
| OpenGitOps | declarative, versioned, pulled, reconciled | whether the overlay is correct |
| Pod Security Standards | baseline and restricted profiles | whether the workload needs the exception |
| Effective Go | Go idiom | modules and generics; it predates both |
| Go Code Review Comments | review-time Go checklist | design-level structure |
| Code Smells | surface signs of design trouble | whether the fix is worth it |
| Popper falsifiability | a hypothesis states what would refute it | whether the refuting observation is reachable |
| Asch conformity | why unblinded agreement is not corroboration | whether the reviewers were in fact blinded |
| arc42 | section order of a design | section quality |
| ADR (Nygard) | a decision record that supersedes | whether consequences state the real cost |
| EARS | requirement syntax | whether the template fits the real class |
| RFC 2119 | normative keywords, uppercase | whether the obligation is the right one |
| INVEST | whether a story is a real slice | whether the slice delivers value |
| Gherkin | acceptance scenarios | whether the scenario is the important one |
| Conventional Commits | commit subject | whether the scope names the right component |
| Diátaxis | one page, one mode | whether the author chose the mode well |
| BLUF | conclusion first | whether that sentence is the bottom line |

## Stated in full

These have no reliable public prior. Naming them is not enough.

<!-- vale off -->
<!-- Verbatim contract text. It goes into a model's context unchanged, so the gate
     must not reshape it. Its own 25-word cap does not apply to the clause that
     states the cap. -->
**Writing.** Write in Simplified Technical English (ASD-STE100): approved words in one
meaning only, active voice, one instruction per sentence, procedural sentences under 20
words and descriptive under 25, noun clusters of at most 3 words. Keep code, commands,
identifiers, and quoted output verbatim.
<!-- vale on -->

**Provider owns the interface.** The component that produces a contract owns its shape.
Consumers adapt. This is the tie-break when two reviewers disagree about a boundary.

**Two-way doors only.** A one-way door needs explicit human approval before anyone
finalizes it. A one-way door is a persisted schema, a public API contract, or a wire
format. An Omnigent bundle name that a route resolves is one, and so is anything another
system depends on.

**Independent before synthesized.** A reviewer commits findings before seeing a peer's.
Convergence counts as corroboration only when each reviewer worked blind.

**Findings carry evidence.** A finding names the specific contract, field, signature, or
line it is about. Bare approval is not a finding.

**Errors are interface.** Every error condition is part of the public contract.

## Governance

Precedence, highest first: a direct instruction in the conversation; this file; the
repository's `AGENTS.md` doctrine block.

**This repository is internal.** An artifact here MAY carry Sei-specific operational
detail. An artifact for the public `agentic-writing` repository MUST NOT carry it.

An amendment states what changed and why, and bumps the version below. Deleting a
principle requires the same ceremony as adding one.

**Version**: 0.2.0 | **Ratified**: 2026-08-27 | **Last Amended**: 2026-08-27 (added Principle VII)
