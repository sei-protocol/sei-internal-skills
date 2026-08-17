// Package driver runs one [Workload] to an answer: open the session for the unit
// of work, take one turn, and report whether what came back is an answer.
//
// Three things are split, and the split is the design. What the agent is asked, and
// how to tell it has finished answering, is the [Workload]'s. Reaching a server —
// opening a session, driving a turn, recognising its end — is the [Host]'s. What is
// left is this package: the run's budget, the one question only the workload's rule
// can settle, and the exit code the caller acts on.
//
// This package names no server. It depends on no client library, which is a
// compile-time fact rather than a convention, so a fault in how one deployment
// reports a turn cannot reach the exit-code contract except through [Host].
//
// It keeps the Python driver's contract: the same environment variables, exit
// codes, run-key idempotency, and fail-closed permission policy keyed on
// server-attested tool identity.
//
// # One session per unit of work
//
// The unit of work is not a run. A session is opened on the first dispatch and
// adopted by every later one, so the agent's memory of the tree survives between
// invocations and a second-pass prompt can ask what changed rather than asking
// again. The run key is a label on the session, so it outlives the runner.
//
// For the first workload that unit is a pull request. The vocabulary here stays
// neutral because nothing in this package depends on which it is — only that two
// dispatches for the same work agree on the key, and different work does not
// collide.
//
// A run tears nothing down, because stopping buys nothing: stopping a session ends
// the agent process and the runner, never the sandbox, so the pod bills the same
// either way — and a stopped runner costs the next invocation a fresh one and a
// rebuilt transcript. The runner's idle timeout reclaims it unasked.
//
// So [Driver.Close], when the work ends, is the only thing that reclaims a sandbox.
// Nothing else will: the Kubernetes launcher sets no lifetime cap and the server
// runs no sweep, so a session nothing deletes holds its pod's cpu and memory
// indefinitely. That is also why teardown runs on a context detached from the run's
// — a terminate signal cancels the run so that teardown can happen, so inheriting
// that cancellation would abort the one thing that frees anything.
//
// A run drives exactly one turn, and re-running is the retry: the next invocation
// adopts the same session with its context intact.
//
// # Why the workload decides what finished means
//
// [Workload.Complete] cannot be answered from the server. A terminal-backed session
// goes idle between tool calls and reports no active response while it does, so both
// of the server's signals read "finished" mid-answer. Only the reply itself is
// reliable, and only the workload knows what its prompt asked for. Reading the
// server instead published an agent's opening sentence as a review.
//
// # Where the contract is written down
//
// The exit codes in errors.go are a contract with a calling workflow that may be
// pinned to an older ref, so their meanings only ever widen. The operator-facing
// description of every environment variable and exit code is
// cmd/sei-agent-driver/README.md.
package driver
