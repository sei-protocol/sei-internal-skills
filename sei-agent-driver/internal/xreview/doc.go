// Package xreview drives one sei-droid review of a pull request inside an
// Omnigent managed sandbox: resolve the agent, create or adopt the session for
// that pull request, send the review prompt, answer the permission prompts the
// turn parks on, read a verdict off the session, and render it for a caller to
// post.
//
// It is the Go successor to the Python driver this repository's history carries,
// and it keeps that driver's contract: the same environment variables, the same
// exit codes, the same run-key idempotency, and the same fail-closed permission
// policy keyed on server-attested tool identity.
//
// The build carries no runtime dependency install, so the credential-holding
// runner installs nothing while it holds a live token.
//
// # One session per pull request
//
// The unit of work is a pull request, not a run. A session is created on the
// first review and adopted by every later one, so the agent's memory of having
// reviewed this tree survives between invocations and the adopted prompt can ask
// what changed. The run key is a label on the session, which is server-side state
// and therefore outlives the runner.
//
// A review therefore ends without tearing anything down, and the reason it does
// not bother is that it cannot. stop_session ends the agent process and the
// runner, never the sandbox, so the pod bills the same either way; all a stop
// buys is that the next invocation has to spawn a fresh runner and rebuild its
// transcript before it can work. The runner's own idle timeout reclaims it
// unasked.
//
// So [Driver.DeleteSessionForPR], driven by the pull request closing, is the only
// thing that reclaims a sandbox. Nothing else will: the Kubernetes launcher sets
// no lifetime cap and the server runs no sweep, so a session nothing deletes
// holds its pod's reserved cpu and memory indefinitely.
//
// A run drives exactly one turn. A run that ends without a verdict is re-run, and
// because the next invocation adopts the same session with its context intact,
// re-running is the retry.
//
// # Two response-id namespaces
//
// The server uses two disjoint id namespaces and only one of them can be
// compared to a conversation item:
//
//   - resp_claude_<32 hex> is stamped on every item a turn commits, and on the
//     session status edges that describe that turn. This is the one to attribute
//     on.
//   - resp_<24 hex> appears only on response lifecycle events. It is synthesised
//     per event and can never equal an item's response id.
//
// Comparing across them silently discards every reply, so this package converts
// nothing between the two and reads the turn id only from a status edge.
//
// # How a turn is bounded
//
// Two server-attested facts bound a turn, and everything else is inference:
//
// The boundary is the item id SendInput returns, echoed back on the stream as
// session.input.consumed. Nothing at or before it is ever publishable. The stream
// opens with a prologue that replays earlier work, so without this line a previous
// invocation's completed reply is indistinguishable from a fresh one.
//
// The end is a session.status edge reporting idle and carrying a response id,
// arriving after the boundary. That edge's response id is the turn id. An idle
// edge with no response id is terminal churn and ends nothing. No response
// lifecycle event ends a turn on this harness: the completed one is an
// acknowledgement that the prompt was injected into the terminal, and it arrives
// before the prompt has even been persisted.
//
// This is the server's intended contract rather than a workaround for a missing
// one. The forwarder derives that edge from the agent's Stop hook, which fires
// once per finished turn, and it attaches a response id for exactly this purpose:
// to distinguish a turn-end edge from the runner's own idle badge, which is a
// quiescence heuristic that oscillates on every mid-turn lull. Two producers emit
// idle and only one of them means a turn ended.
//
// The server publishes no link between the input item and the turn that answers
// it, so pairing them by that ordering is this package's one inference. The
// prompt item's own response id is not that link — it carries whichever response
// was last active, which is a stale id from before the boundary.
//
// # What may be published
//
// Text reaches a pull request only as the content of a committed item that carries
// this turn's response id, is a completed assistant message, was not authored by a
// client, and is neither injected context nor an interrupted partial. Its type is
// checked before its payload is decoded, because the payload accessor consults no
// discriminator and will decode a tool output into an empty message without
// complaint. Only output_text content parts contribute, since a message may carry
// the model's private reasoning beside its answer.
//
// Attribution is positive throughout. A negative filter — newest message not seen
// before the turn — fails open, and this package has published another
// invocation's verdict that way.
//
// The streamed text deltas are a progress signal and never a source. They arrive
// out of index order and, on the one complete review recorded, one chunk short of
// the committed message, so reassembling them cannot even produce a verdict.
//
// # Where the contract is written down
//
// The exit codes in errors.go are a contract with a calling workflow that may be
// pinned to an older ref, so their meanings only ever widen. The operator-facing
// description of every environment variable and exit code is cmd/sei-agent-driver/README.md.
package xreview
