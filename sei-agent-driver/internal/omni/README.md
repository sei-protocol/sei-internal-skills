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
tested without a server, which is why most of this package's tests exercise them
directly.

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

**The anchor — where our work begins.** `SendInput` returns an id for the prompt,
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

## Unsettled

**Whether the in-process end reconciles across id namespaces.** `doc.go` states that
a lifecycle event's `resp_<24 hex>` can never equal an item's response id. The
in-process path does exactly that comparison: `observeResponseTerminal` takes the
lifecycle id as the turn id, and `fetchReply` then looks up items by it. If the claim
holds universally that lookup always misses, and it misses *quietly* — the run
reports no verdict rather than failing.

The test cannot adjudicate it, because we author both sides of the fixture and it
uses one id for the lifecycle event and the item. The likely truth is that `doc.go`'s
claim was derived from claude-native traces and is over-general. Settling it takes
one live run on an in-process harness that produces an attributed reply, or reading
the forwarder. Narrow the wording in `doc.go` either way.

## Changing this safely

The driver must not gain a dependency on the client library. That is the boundary
this package exists to hold, and it is checkable:

```sh
go list -deps ./internal/driver | grep omnigent-go-sdk   # must print nothing
```

Failures here cross into `driver`'s sentinels — `ErrConfig`, `ErrTurnFailed`,
`ErrMint`, `ErrLeaked` — because those are what the exit codes are derived from.
Returning this server's own error taxonomy would make the contract with a calling
workflow depend on a library's choices.
