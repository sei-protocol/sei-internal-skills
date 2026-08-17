---
name: code-structure
description: "Use when restructuring code for readability — 'make this method easier to follow', 'this is hard to read', 'refactor this for readability', 'the comments are doing too much work here', 'clean up this method', 'make this digestible for a new engineer', '/code-structure'. Especially for a long orchestration method — a controller reconcile, a request handler, a plan builder, a migration runner — whose sequence is buried under inline prose. Anti-triggers: NOT language, framework or package idiom (use /idiomatic — this skill sits on top of it and never overrides an idiom call); NOT correctness or logic bugs (use /code-review); NOT reliability, performance or observability (use /systems); NOT cross-component boundary review (use /xreview); NOT prose or documentation (use prose-steward). It changes structure and where the 'why' lives; it never changes behavior, and it proposes rather than rewrites."
category: code-quality
---

# Code Structure

Restructures code so it reads as a legible sequence of named steps. Owns step
decomposition and where the *why* lives. Owns nothing else.

**Shape:** technique with a discipline spine. The method is short; the refusals carry
the weight, because the two ways this goes wrong — extracting everything, and deleting
every comment — both feel like doing the job well.

## The one principle

**Code should read as a sequence of named steps a new engineer can follow
top-to-bottom without an expert narrating it.**

The method body is the table of contents. The step names carry the *what*. You drill
into a step only for its detail. If following a method requires someone to walk you
through it, the structure failed — restructure until the step sequence carries the
meaning.

When any two rules below conflict, this one wins.

## Guardrails

1. **Structure changes, behavior never.** The refactor must be diffable as "nothing
   changed but structure and where comments live." **The existing tests passing
   unchanged is the proof** — if a test needs editing, this is not a structure
   refactor and you must say so and stop. Readability is never worth a behavior
   change.

2. **This sits on top of idiom; it does not replace it.** `/idiomatic` owns language,
   framework and package idiom. This skill owns step decomposition and comment
   placement *of already-idiomatic code*. Extract into the shape the package already
   uses. **Never introduce an abstraction the codebase does not already have** — a
   novel pattern that looks clean is an idiom regression. Run `/idiomatic` after.

3. **The target is not zero comments.** It is a step sequence legible *without*
   comments, with the remaining comments explaining only the non-obvious *why*.
   Deleting a load-bearing invariant to tidy up is the canonical violation.
   Relocation is the move, not deletion.

4. **Do not over-abstract.** Extraction serves the read. Refuse it when it would
   thread a new parameter through an existing signature, blur two distinct paths into
   one helper (two exits with different error strings are two paths), or add
   indirection for a single caller with a name no clearer than the expression.
   **When you defer an extraction, state the condition that would un-defer it.**

5. **Propose; the author approves.** Output a diff or the restructured method. Never
   silently rewrite someone's production code. For a safety-critical or
   consensus-critical file, the refactor still goes through the normal review gate.

## The principles

Each is stated with the test that decides it.

**1. A method reads as a list of named steps.** The orchestration is a sequence of
calls whose names are the outline — `snapshotBase() → resolveGate() → if failed
{ flushAndExit() } → reconcile()` — not a wall of expressions and inline prose.

**2. The name carries the *what*; the doc comment carries the *why*.** A compute-heavy
predicate sitting inline behind an eight-line rationale becomes a named method, and
the rationale moves to that method's doc comment. The call site reads clean; the why
is visible when you edit *that* decision and invisible when you scan the flow.

> **Where the repo codifies doc-comment scope, the repo wins.** Some codebases rule
> that a doc comment states only what a thing *is*, and rationale belongs inline at
> the line that needs it. Read `AGENTS.md` / `CONTRIBUTING` before placing the why.
> This skill decides *whether* the rationale survives and that it lands where a
> future editor will find it. The repo decides *which* comment holds it.

**3. A long inline comment means the step was never named.** Prose explaining what a
block does is the signal to extract the block — name is the *what*, doc is the *why*.
The explanation is not the problem. Its placement inline in the orchestration is.
**Fix by extracting, never by deleting.**

**4. Keep the load-bearing why.** Some comments survive because the invariant they
state is not visible from the call order: a sequencing dependency, correct code that
looks wrong, a safety invariant. *The test: could a competent engineer get this wrong
without the comment?* If yes, keep it — trimmed, and on the thing it governs.

**5. Extraction has a cost.** Not everything should be a method. When you skip one,
name what would change your mind: "extract the snapshot bundle when a third consumer
appears." A deferred extraction with a trigger is a decision; without one it is an
oversight.

**6. Prove equivalence with an anchor.** Where it helps a reviewer verify nothing
shifted, keep a byte-identical line. Keeping a local rather than inlining it means the
guard line is textually unchanged and the diff proves only the right-hand side became
a named call. Inlining is a fine follow-up once equivalence is established.

**7. The why must be the true reason, not the most consequential-sounding one.** A
rationale that overstates what the code defends against is a defect even when the code
is correct, because the next person calibrates their changes to the reason you wrote.
*The test: would the engineer who owns this code recognise your reason as the real
one?*

**8. A comment cannot carry a cross-file invariant.** "Kept in sync with X" is a
convention the next editor can forget, not an invariant they cannot break. Either make
drift structurally impossible, or drop the claim and let each site stand alone.
**Being unable to do either is itself the finding** — it means the coupling sits in
the wrong place.

**9. Write in the present tense of the code, not the history of the change.** "Split
out of X", "previously Y", "this PR", "the old behavior" read as noise once merged and
mislead a reader who never saw the change. Provenance lives in the commit and the PR.

**10. Put the guard on the path every caller takes.** Prefer the one function all
paths pass through over the call site in front of you. A guard repeated per caller is
a convention the next caller forgets; a guard at the choke point is an invariant they
cannot. **When no such function exists, that absence is the finding** — not a reason
to distribute the guard.

**11. A reversed decision invalidates everything written under it.** When a change
flips a decision, the names, comments and doc text written for the old one go stale in
the same commit and read as contradictions. Sweep them with the reversal rather than
waiting for review to find them.

**12. Digestibility for a new engineer is the north star.** Success is a new engineer
reading the method top-to-bottom and following it without a guide. Extraction, naming
and comment density all serve that one metric. If onboarding still needs an expert
narrating the method, the structure is not done.

## Anti-caricature rules

Applied without judgment, this skill over-corrects. Each of these *feels* like doing
the job well.

| Over-correction | Why it is wrong |
|---|---|
| **Extraction sprawl** | The goal is a readable body, not a high method count. A one-line helper called once, with a name no clearer than the expression, is worse than the expression. Extract predicates that carry a rationale or a name that adds meaning; leave self-evident expressions inline. |
| **Comment scorched-earth** | "Fewer comments" is not the rule. "The step sequence is legible without them" is. Never strip a load-bearing invariant to hit an aesthetic. |
| **Novel abstractions** | Match the package's existing shape — receiver methods, free functions, whatever it uses. Importing a pattern the codebase lacks to look clean is an idiom regression. |
| **Rationale inflation** | Reaching for the most serious-sounding justification because it makes the change feel important. If the honest reason is mundane, the comment is mundane. |
| **Behavior drift disguised as cleanup** | Reordering, collapsing branches or "simplifying" a condition can change semantics. If the tests would need editing, it is not a structure refactor. |

## Procedure

1. **Read the package first.** Note the shapes it already uses and whether
   `AGENTS.md` / `CONTRIBUTING` codifies doc-comment scope. You are extracting *into*
   an existing vocabulary, not inventing one.

2. **Find the steps.** Read the target top-to-bottom and mark where one thing ends and
   the next begins. Long inline comments mark these boundaries reliably — principle 3.

3. **Name each step.** If a step resists naming, that is information: it probably does
   two things, or it is not a step.

4. **Place each why.** For every comment, apply principle 4's test. Load-bearing ones
   move to the thing they govern, trimmed. Ones the step name now carries are cut,
   because the name says it. Ones that explain *what* a block does get extracted with
   the block, not deleted.

5. **Decide what not to extract**, and state the un-defer condition for each — 
   guardrail 4.

6. **Check equivalence.** Would the existing tests pass unchanged? If not, stop and
   say what would change. Add an anchor line where it helps a reviewer — principle 6.

7. **Output the proposal.** The diff or restructured method, what you deliberately did
   not extract and why, and the un-defer conditions. Point at `/idiomatic` and the
   review gate as the next steps.

## Halt conditions

- **Behavior would change.** Report what and stop. This is never a judgment call.
- **The tests would need editing.** Same thing, discovered later.
- **The idiomatic shape is unclear** — the package is inconsistent, or you cannot tell
  what pattern it uses. Ask rather than picking one; guardrail 2 depends on knowing.
- **The file is safety- or consensus-critical and the user wants it applied
  directly.** Propose, and say the review gate still applies.
- **The method is not actually hard to follow.** Say so. A refactor with no reader
  benefit is churn in someone's blame history.

## What this skill does not do

- **Idiom.** `/idiomatic` owns it, and outranks this skill wherever they meet.
- **Correctness.** `/code-review`.
- **Reliability, performance, observability, API durability.** `/systems`.
- **Cross-component boundaries.** `/xreview`.
- **Prose and documentation.** `prose-steward`.
- **Apply anything.** It proposes. The author approves and merges.

## Output

A proposed refactor — diff or restructured method — plus:
- what was deliberately not extracted, each with its un-defer condition;
- which comments were relocated, and which were kept and why;
- the equivalence claim, backed by "the existing tests pass unchanged" or an explicit
  statement that they would not.

Never assert behavior-equivalence without that proof.
