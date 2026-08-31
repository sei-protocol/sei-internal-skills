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
// It keeps the Python driver's contract: the same environment variables, the same exit
// codes, and the same fail-closed permission policy keyed on server-attested tool
// identity. It does not keep that driver's per-trigger idempotency -- the run key is the
// unit of work, so nothing is skipped as a duplicate and every dispatch adopts the
// session and reviews the current tree. See [xreview.RunKey]. The exit codes are that set and one more, [ExitInternal], which a
// caller pinned to an older ref reads as an unknown failure.
//
// # One session per unit of work
//
// The unit of work is not a run. A session is opened on the first dispatch and adopted by
// every later one, so a second-pass prompt can ask what changed rather than briefing the
// agent again. The run key is a label on the session, so it outlives the runner.
//
// What survives is the conversation: the checklist, the output rules, and the findings the
// agent wrote with the reasoning behind them. The tree does not, and is not meant to --
// the adopted prompt re-fetches the diff and re-clones the tree on every dispatch, and
// deletes the tree if it cannot bring it current, because memory of a moved pull request
// is wrong rather than merely stale. Reusing the session is worth its cost only if the
// prompt then declines to re-send what the session holds; see [xreview.AdoptedPrompt].
//
// For the first workload that unit is a pull request. The vocabulary here stays
// neutral because nothing in this package depends on which it is — only that two
// dispatches for the same work agree on the key, and different work does not
// collide.
//
// A run tears nothing down, because stopping buys nothing: stopping a session ends
// the agent process and the runner, never the sandbox, so the pod bills the same
// either way — and a stopped runner costs the next invocation a fresh one and a
// rebuilt transcript. The runner's idle timeout ends that process unasked. It frees
// no sandbox.
//
// So [Driver.Close], when the work ends, is what reclaims a sandbox. Nothing does it
// on a schedule: the Kubernetes launcher sets no lifetime cap and the server runs no
// sweep, so a session nothing deletes holds its pod's cpu and memory indefinitely.
// Opening reclaims one too, but only a session it finds unable to run a turn at all,
// so no working session is ever freed except by a close.
//
// That is why teardown runs on a context detached from the run's: a terminate signal
// cancels the run so that teardown can happen, so inheriting that cancellation would
// abort the reclaim it exists to allow.
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
// pinned to an older ref, so their meanings only ever widen. Nothing in this module
// reads them: the command that does lives outside it, and the operator-facing
// description of every environment variable and exit code travels with that command.
package driver
