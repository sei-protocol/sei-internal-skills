# Omnigent Meta-Harness

`internal/omni` — the Omnigent implementation of `driver.Host`.

A harness is what actually runs the agent: a real terminal driving Claude Code, or
an in-process SDK loop. Each reports the end of a turn differently, and neither
signal exists on the other. This package is the layer above them — it absorbs those
differences and presents one `Conversation` to the driver, which is why nothing
upstream of here has to know which harness answered.

Everything in it exists because of one absence.

## The absence

The server gives you an event stream and a session you can snapshot. It does not
give you a statement that a turn is finished, and it publishes **no link between
the prompt you sent and the message that answers it**.

So a turn's end has to be inferred from edges that mean different things on
different harnesses, and a reply has to be attributed by identity and position
rather than read off a field. Every mechanism below is a consequence. If the server
ever links an input item to its response, most of this collapses.

## Where things are written down

Three places, on purpose. Keep them apart or they drift.

| here | what belongs in it |
|---|---|
| `README.md` (this file) | how the package models the problem, what to read first, what is still unsettled |
| `doc.go` | the rules themselves — the id namespaces, how a turn is bounded, what may be published. Read with `go doc`, so it travels with the API |
| code comments | why one statement is where it is, at the statement |

This file does not restate the rules. If you need them, read `doc.go`.

## Reading order

Start with the two files that call nothing. Every rule they hold can be read and
tested without a server, and `turn_test.go` drives the state machine that way. Most
of the suite runs the other way round -- a fake server, a real driver, an exit code --
so a rule is checked where a caller would feel it.

| file | the question it answers | reaches the server |
|---|---|---|
| `turn.go` | is the exchange over? | no |
| `items.go` | is this text ours, and may we publish it? | no |
| `conversation.go` | drive the turn, and survive the transport | streams |
| `host.go` | get a session that can actually run a turn | requests |
| `client.go`, `mint.go`, `transport.go` | build a client that notices a dead connection | requests |
| `elicitation.go` | reduce a permission prompt to what a decision may rest on | no |

## How the package models it

Six pieces carry it. Each is small; together they are the shape of the package.

**The anchor — where our work begins.** Posting the prompt returns an id for it,
and the stream echoes it back as `session.input.consumed`. That echo is the
boundary: nothing at or before it can be ours, because a stream opens by replaying
earlier work. Two identifiers are involved, because a prompt reaching a live
runtime persists as an item immediately, while one parked before the sandbox is up
returns a pending id that names no item yet. `turn.anchor` holds whichever came
back, and `turn.anchorItem` holds the item the boundary resolved to — position
comparisons use that one.

**The prior set — identity is exclusive.** Before the turn, the run records every
response id already on the session. An id in that set cannot belong to the turn
answering our prompt, however well-timed its edge looks. This is what makes
overlapping runs safe: a superseded run whose stop lost the race ends its turn
inside our window, and its edge is otherwise identical to ours.

**Positive attribution — the publish gate.** Text reaches a caller only as the
content of an item that carries this turn's response id, is a completed assistant
message, was not authored by a client, and is neither injected context nor an
interrupted partial. Positive throughout, because the negative form — newest
message not seen before the turn — fails open and admits anything the pre-turn
snapshot missed. `items.go` also checks an item's type before decoding its payload,
since the decoder consults no discriminator and turns a tool output into an empty
message without complaint.

**The caller's predicate — what "finished" means is not ours.** `driver.Ask` carries
`Done`, and this package asks it rather than asking the server. A terminal-backed
session goes idle between tool calls, so both of the server's own signals read
"finished" mid-answer. The predicate travels down to the salvage paths for the same
reason.

**Snapshot reconciliation — the stream has no replay.** A connection lives roughly
three minutes and a turn runs longer, so several reconnects per turn are ordinary
success rather than failure, and `conversation.Turn` follows one turn across as many
streams as it takes. Nothing is replayed on rejoin, so a turn that ended while we
were disconnected sent its last edge to nobody — rejoining means asking the session
what happened, and what counts as *this run's* reply is this package's rule rather
than the protocol's. `turnSettled` records when waiting can no longer change the
outcome, so a reconnect does not watch for edges that will never come.

**Asymmetric liveness — one field, read two ways.** `runner_online` is the server's
only reachability signal. Adoption reads an unknown value as live, so it never
deletes a conversation on missing information; sending reads it as not-live, so it
never puts a prompt into a sandbox that does not exist. The defaults differ because
the costly mistake differs.

## Harness differences

The end of a turn is the one thing the server reports differently per harness, and
neither signal is available on the other.

**Terminal-backed** — the `*-native` harnesses, which drive a real terminal. The end
is a `session.status` edge reporting idle *and* carrying a response id, arriving
after the boundary. This is the server's intended contract rather than a workaround:
the forwarder derives that edge from Claude Code's `Stop` hook, which fires once per
finished turn, and attaches a response id for exactly this purpose. A bare idle edge
is terminal churn and ends nothing.

**In-process** — codex, claude-sdk, the agents SDKs. That edge never arrives:
`response_id` is documented as `None for ordinary in-process runtime edges`, and only
the route a native forwarder posts to attaches one. The end there is the response
lifecycle, which the executor yields only on a final answer, and the relay commits
the assistant message before publishing it.

`terminalBacked` picks between them from an enumerated list of in-process harnesses,
so a harness this package does not recognise takes the terminal-backed rule. That
direction is deliberate: waiting for an edge that never comes ends in a deadline,
which announces itself, while ending on a lifecycle event too early publishes a
half-written answer as a verdict.

**Response ids follow the same split**, which is why the end signal and the
attribution have to agree. A terminal-backed harness derives `resp_claude_<32 hex>`
from the Claude source key and stamps it on items and on the status edges describing
them; it emits no response lifecycle events at all. An in-process harness mints one
`resp_<24 hex>` per turn, carries it on every lifecycle event for that turn, and the
relay stamps the items it persists with the same id so that a turn's items and its
lifecycle share one identifier. So a lifecycle id is a valid thing to attribute
against — on the harness whose items are stamped from it, and only there.
