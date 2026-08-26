// Package omni drives an Omnigent deployment, and is the [driver.Host] the driver
// runs against: resolve the agent, open or adopt the session for the unit of work,
// send the prompt, answer the permission prompts the turn parks on, and read the
// reply back off the session.
//
// Everything here is a fact about one server. That is the reason it is a package
// rather than part of the driver: which of two id namespaces an id belongs to,
// whether a turn's end arrives as a status edge or a response lifecycle event, and
// how many times a stream must be re-established to outlast one turn are answers
// this deployment gives, and none of them is a fact about running an agent to an
// answer. The driver states what it needs in [driver.Host] and [driver.Conversation]
// and depends on no client library; this package is what makes those true.
//
// # Response ids belong to a harness, not to the server
//
// Which id an item carries depends on the harness that produced it, so a turn id
// is only ever compared against items from the same harness:
//
//   - A terminal-backed harness stamps an id of its own -- resp_claude_<32 hex> on
//     claude-native, derived from the Claude source key; another prefix on another
//     runtime -- and its items and the status edges describing them share it. It
//     emits no response lifecycle events at all, which is why a turn cannot end on
//     one there.
//   - An in-process harness mints one resp_<24 hex> per turn, carries it on every
//     response lifecycle event for that turn, and the relay stamps the items it
//     persists with that same id -- deliberately, so a turn's items and its
//     lifecycle share one identifier.
//
// So the rule is not "a lifecycle id never matches an item" but "read the turn id
// from the signal this harness's items are stamped from". Reading the wrong one
// discards every reply silently: attribution finds no item, and the run reports no
// answer rather than failing.
//
// # How a turn is bounded
//
// Two server-attested facts bound a turn, and everything else is inference:
//
// The boundary is the item id posting the prompt returns, echoed back on the stream as
// session.input.consumed. Nothing at or before it is ever publishable. The stream
// opens with a prologue that replays earlier work, so without this line a previous
// invocation's completed reply is indistinguishable from a fresh one.
//
// The end depends on the harness, because the server emits a different signal for
// each and neither is available on the other. [terminalBacked] picks.
//
// On a terminal-backed harness — the *-native ones, which drive a real terminal —
// the end is a session.status edge reporting idle and carrying a response id,
// arriving after the boundary. That edge's response id is the turn id. A bare idle
// edge is terminal churn and ends nothing: two producers emit idle and only one of
// them means a turn ended, so several arrive per session and one of them lands
// mid-work. A turn cannot end on a response lifecycle event there because none is
// emitted -- and an unrecognised runtime that did emit one early would publish a
// review the agent had not finished writing, which is why the strict rule is the
// default.
//
// That edge is the server's intended contract rather than a workaround. The
// forwarder derives it from Claude Code's Stop hook, which fires once per finished
// turn, and attaches a response id for exactly this purpose.
//
// On an in-process harness that edge never arrives, and waiting for it is waiting
// for something the server does not send. The field is documented as "None for
// ordinary in-process runtime edges", and only the route a native forwarder posts
// to attaches one. So the end there is the response lifecycle: the executor yields
// it only on a final answer, and the relay commits the assistant message before
// publishing it. Requiring the id-bearing edge on that harness made the predicate
// unsatisfiable by specification: a scout on that harness produces a complete
// report, and the run waits out its whole budget and discards it.
//
// An unrecognised harness takes the terminal-backed rule. Waiting too long is a
// failure that announces itself; publishing half a review is not.
//
// The server publishes no link between the input item and the turn that answers
// it, so pairing them by that ordering is this package's one inference. The
// prompt item's own response id is not that link — it carries whichever response
// was last active, which is a stale id from before the boundary.
//
// # What may be published
//
// Text reaches a caller only as the content of a committed item that carries this
// turn's response id, is a completed assistant message, was not authored by a
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
// out of index order and can land short of the committed message, so reassembling
// them cannot even produce a verdict.
//
// # Errors cross the boundary in the driver's terms
//
// A failure this package can classify is wrapped into one of the driver's sentinels
// — [driver.ErrConfig], [driver.ErrTurnFailed], [driver.ErrMint], [driver.ErrLeaked]
// — because those are what the exit codes are derived from, and returning this
// server's own taxonomy instead would tie a workflow's contract to a client
// library's choices.
//
// Most failures are not classified. A transport error, a read that would not
// complete, a create the server rejected for its own reasons: those travel as they
// arrived and land on the caller's default, which is a transport exit. That is the
// right default -- retryable -- and it is why only the cases that are *not*
// retryable are wrapped.
package omni
