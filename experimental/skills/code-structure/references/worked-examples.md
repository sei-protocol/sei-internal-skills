# Worked examples

Real before → after deltas from this codebase. Each names the principle it
demonstrates. They exist because the principles are easy to nod at and hard to apply —
the judgment lives in the cases where the rule says *don't*.

---

## A. Inline predicate behind a long rationale → named method
*(Principles 1, 2, 3)*

**Before**, in `SeiNodeReconciler.Reconcile`:

```go
// Hold initial StatefulSet creation while the state-sync gate suppresses
// the init plan: the plan's EnsureDataPVC task is the only creator of the
// data PVC the pod mounts by claimName, so an STS created now would strand
// a Pending pod until the gate opens. Only initial creation is held
// (Status.StatefulSet == nil) — an existing STS is never touched, and
// StateSyncBlocksPlan is pre-Running-only …          [8 lines total]
holdInitialSTS := planner.StateSyncBlocksPlan(node) && node.Status.StatefulSet == nil
```

**After** — the body reads as a step:

```go
holdInitialSTS := shouldHoldInitialStatefulSet(node)
```

…and the *what* is the name, the *why* is the doc comment, in the file that owns the
concern:

```go
// shouldHoldInitialStatefulSet reports whether initial STS creation must wait
// for the state-sync gate. The plan's EnsureDataPVC task is the only creator of
// the data PVC the pod mounts by claimName, so an STS created before the gate
// opens would strand a Pending pod. …
func shouldHoldInitialStatefulSet(node *seiv1alpha1.SeiNode) bool {
    return planner.StateSyncBlocksPlan(node) && node.Status.StatefulSet == nil
}
```

**The placement rule this establishes:** the extracted predicate lands in the file that
owns the concern — an STS predicate goes to `statefulset.go`, next to its siblings —
not in the file you extracted it from.

---

## B. A load-bearing invariant, kept in place and trimmed
*(Principle 4 — the case where the rule says don't extract and don't delete)*

A state-sync gate was resolved *before* the Failed/Paused early-returns so its status
mutation rides whatever flush those exits already do. That is a sequencing dependency,
and it is invisible from call order.

The comment was **kept at the call site** — not extracted, not deleted — and trimmed
from seven lines to four.

The test that decided it: *a competent engineer could reorder this and silently break
the piggybacked flush.* So it stays.

---

## C. Deliberately not extracted, with the un-defer condition stated
*(Principle 5, guardrail 4)*

Two things were left inline on purpose:

- **A snapshot bundle** (`before` / `statusBase` / `observedPhase` / `prev*`).
  Extracting it into a struct would thread that struct through an existing
  `(before, statusBase)` signature — churn and a signature touch for zero readability
  gain. **Un-defer when:** a third consumer of the same bundle appears.
- **Two flush-and-return exits.** They carry distinct error-wrap strings — `"flushing
  status on Failed"` versus `"flushing paused status"` — that a shared helper would
  blur. Two paths, not one.

A deferred extraction with a stated trigger is a decision. Without one it is an
oversight.

---

## D. A byte-identical anchor that proves equivalence
*(Principle 6)*

The `holdInitialSTS` / `holdForWorkflow` **locals were kept** rather than inlined into
the `if`. That leaves the guard line textually unchanged:

```go
if !holdInitialSTS && !holdForWorkflow {
```

so the diff proves only the right-hand sides became named calls. Inlining to
`if !shouldHoldInitialStatefulSet(node) && !adoptedWorkflowIsExecuting(node)` is a fine
follow-up — **once equivalence is established, not in the diff that has to prove it.**

### The result shape

```
snapshot base + prev conditions
setNodePausedCondition
flushStatus := closure
resolveStateSyncGate                    (+ trimmed ordering note)
if Failed        { flush; event; return }
holdInitialSTS  := shouldHoldInitialStatefulSet(node)
holdForWorkflow := adoptedWorkflowIsExecuting(node)
if !hold…        { reconcileStatefulSet }
if Paused        { flush; return }
reconcilePeers
reconcileWorkflow  (handled? return)
resolveDriftPlan   (unless workflow-suppressed)
flush; return execErr; emitPhaseTransition; return steadyStateRequeue
```

A reader can follow that top-to-bottom without a guide. That is the bar.

---

## E. A rationale that overstated what the code defends
*(Principle 7)*

**Before**, on a handshake call:

```go
// handshakeCtx, not ctx: the accept slot is held until release()
// below, so an unbounded read here lets a peer hold it forever.
```

The owner's correction: the node-info exchange is still part of the handshake, and
after it, liveness is monitored by pings. What the deadline defends against is a
**network connectivity problem, not a malicious peer** — a malicious peer can open a
connection and keep it alive without sending anything useful anyway.

**After:**

```go
// The node info exchange is part of the handshake, so it runs under
// the same deadline. Without it a peer that loses connectivity
// mid-exchange holds an accept slot for as long as its socket lives.
```

**The code did not change.** The comment had claimed a defense the change does not
provide, and the honest reason — the exchange belongs to the handshake — is both
simpler and correct. A rationale is part of the code's contract with its next editor.

---

## F. A cross-file "keep in sync" comment, removed rather than enforced
*(Principle 8)*

Two packages each held the same 10ms default, and each doc comment named the other as
the thing to stay in sync with.

The resolution was **neither a drift test nor a shared constant.** The owner accepted
the duplication temporarily, intending a structural fix — making the field required so
the default has exactly one home.

The reasoning worth carrying: *a comment asserting an invariant that nothing enforces
is worse than duplication that is honestly two values.* The comment promises the next
editor something the codebase cannot keep.

---

## G. Design history in doc comments
*(Principle 9)*

**Removed on review:**

```go
// p2pRouterOptions derives … Split out of createRouter so the derivation is
// testable on its own: any RouterOptions field left unset here silently falls
// back to a package default rather than failing, which is how the accept rate
// stayed pinned at its 1/s default while max-connections appeared to govern it.

// This PR's own diagnosis is that a 1/s production accept rate survived because …
```

**After:**

```go
// p2pRouterOptions returns the router's connection budget and pacing,
// derived from the p2p config.
```

plus, where a real invariant needed stating:

```go
// Every other RouterOptions construction site substitutes rate.Inf, so this
// derivation is the only place the production accept rate is exercised.
```

"This PR" stops meaning anything the moment it merges. The second comment survived
because it states a *current* property of the code, not a fact about the change.

---

## H. The same guard placed three times before it landed
*(Principle 10)*

One config validation, three review rounds:

1. Wired the field at the single call site in node setup — **fixed one caller.**
2. Raised the package default the accessor falls back to — **fixed every embedder.**
3. Routed the whole config section through the method every production path calls —
   **made the guards run at all.**

Each earlier fix sat one level below the path everything actually took. The repo's own
`AGENTS.md` states the rule that was being violated:

> "Guard at the choke point, never at each caller. A guard repeated at every call site
> is a convention the next caller can forget, where a guard at the single function
> every path passes through is an invariant they cannot."

---

## I. Staleness created by reversing a decision
*(Principle 11)*

A config key was changed from unrendered to rendered in a generated template. Left
behind **in the same branch**: a test still named
`TestHiddenP2PKnobsStillParseFromExistingConfig`, a comment describing both keys as
hidden, and a PR description claiming the key was "not rendered."

Three review rounds went to staleness the reversal should have swept in its own commit.
When you flip a decision, the names and comments written under the old one are part of
the change.
