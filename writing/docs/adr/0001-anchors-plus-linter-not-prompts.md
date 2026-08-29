# 1. Pair a semantic anchor with a linter, instead of a private prompt

## Status

Accepted, 2026-08-19.

## Context

A private output style or a personal skill improves model output. It has four defects:

1. It is not portable. It works only where the file is installed.
2. It is not explainable. A reader cannot tell which instruction did the work.
3. It is not verifiable. Nothing checks whether the model obeyed it.
4. It is not durable. A model upgrade can change the result, and nothing detects that.

An anchor to a public standard removes defects 1 and 2. It does not remove 3 or 4, because
an anchor is a hint in a context window. The model can ignore it, and often does under a
long context or a competing instruction.

Vale can check a pattern in finished text. It cannot check meaning, so it cannot replace
the anchor either.

## Decision

Use both, and declare the seam between them.

- Anchors go in `writing/anchors/registry.yaml`, with an `invoke_as` string for the model.
- Every anchor declares `verifier.coverage`, and lists what is `not_checkable`.
- Generated context files come from the registry, so the hint and the rule cannot drift.
- Correctness rests on the Vale run, not on the anchor.

## Consequences

Positive:

- A third party can audit the whole contract. Every constraint names a public clause.
- A model upgrade cannot silently degrade the output, because CI still checks the artifact.
- The anchor layer stays cheap: a few hundred tokens, no infrastructure.

Negative:

- Two artifacts describe one constraint, so they can disagree. Generation from the
  registry reduces the risk; it does not remove it, because a Vale rule is hand-written.
- Partial coverage invites a false sense of compliance. The `coverage` field is a
  countermeasure, not a fix.
- Vale adds a dependency and a CI job to every consumer repository.
- STE is copyrighted, so the vocabulary check is an approximation from OpenSTE. A passing
  run is not a certificate.

## Alternatives considered

**Prompt only.** Rejected: no verification, no regression signal.

**Linter only.** Rejected: the model produces a first draft that fails many rules, so the
loop is slow and the agent burns turns on rewrites.

**Fine-tune or a bespoke controlled language.** Rejected: an invented term carries no
weight as an anchor, and a private language cannot be audited by a reader who does not
have it.
