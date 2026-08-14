# code-structure

Restructures code so it reads as a legible sequence of named steps. Owns step
decomposition and where the *why* lives. Owns nothing else.

## The one principle

> Code should read as a sequence of named steps a new engineer can follow
> top-to-bottom without an expert narrating it.

The method body is the table of contents. The step names carry the *what*. You drill
into a step only for its detail. If following a method requires someone to walk you
through it, the structure failed.

## Where it sits

| Skill | Owns | Precedence |
|---|---|---|
| `/idiomatic` | Language, framework and package idiom | **Outranks this skill wherever they meet** |
| **`/code-structure`** | Step decomposition, comment placement | Applies to already-idiomatic code |
| `/systems` | Reliability, performance, observability, API durability | Separate axis |
| `/code-review` | Correctness | Separate axis |

Run `/idiomatic` after a structure refactor. Never introduce an abstraction the
codebase does not already have — a novel pattern that looks clean is an idiom
regression, and `/idiomatic` owns that call.

## What it refuses

- **Behavior changes.** The proof is the existing tests passing *unchanged*. If a test
  would need editing, this is not a structure refactor and the skill says so and stops.
- **Zero-comment aesthetics.** The target is a step sequence legible without comments,
  with the remaining comments explaining only the non-obvious why. Deleting a
  load-bearing invariant to tidy up is the canonical violation — relocation is the
  move, never deletion.
- **Extraction for its own sake.** A one-line helper called once, with a name no
  clearer than the expression, is worse than the expression. When an extraction is
  deferred, the skill states the condition that would un-defer it.
- **Applying anything.** It proposes a diff. The author approves.

## The anti-caricature rules are the point

Applied without judgment this over-corrects, and both failure modes *feel* like doing
the job well: extract everything, and delete every comment. The skill carries five
named over-corrections and the reason each is wrong.

## Origin

Generalized from a personal code-style profile built on an authentic corpus of real
refactor deltas and review feedback in this codebase. `references/worked-examples.md`
carries those deltas, including the cases where the rule says *don't* extract — which
is where the judgment actually lives.

The personal version stays personal. This is the portable standard underneath it: the
principles are about readability discipline, not about one engineer's taste.

## Files

| Path | What |
|---|---|
| `SKILL.md` | The principle, guardrails, twelve principles, anti-caricature rules, procedure |
| `references/worked-examples.md` | Nine real before → after deltas, each naming its principle |
| `evals/evals.json` | Pressure evals for the two over-corrections, plus a happy path |
