# internal/omni

The Omnigent implementation of `driver.Host`. Everything in this package exists
because of one absence.

## The absence

The server gives you an event stream and a session you can snapshot. It does not
give you a statement that a turn is finished, and it publishes **no link between
the prompt you sent and the message that answers it**.

So a turn's end has to be inferred from edges that mean different things on
different harnesses, and a reply has to be attributed by position and identity
rather than read off a field. Every rule in this package is a consequence. If the
server ever links an input item to its response, most of this collapses.

## Where things are written down

Three places, on purpose. Keep them apart or they drift.

| here | what belongs in it |
|---|---|
| `README.md` (this file) | why the code is shaped this way, what to read first, which decisions read as wrong and are not, what is still unsettled |
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

## Decisions that read as wrong and are not

Each of these has an obvious alternative that is worse. The pattern throughout is
that the two mistakes cost different amounts, so the code takes the direction whose
failure is louder.

| the code | the obvious alternative | why not |
|---|---|---|
| A bare `idle` edge ends nothing; only one carrying a response id does | treat the first `idle` as the end | Two producers emit `idle` and a session goes idle *between tool calls*. Ending there publishes the agent's opening sentence as a finished answer. |
| `inProcessHarnesses` is an enumerated list, so an unknown harness is treated as terminal-backed | test for `"native"` in the name | An unknown harness then ends on the response lifecycle, which on harnesses that ack their injection arrives *before the answer exists*. Guessing strict costs a turn that waits out its deadline — a failure that announces itself. |
| Recovery compares against `turn.anchorItem`, not `turn.anchor` | use the id the send returned | A prompt parked before the runtime is up returns a **pending id**, which names no conversation item. A position check against it fails on every cold start. |
| `turn.prior` excludes response ids already on the session | accept any id-bearing edge after the boundary | A superseded run whose stop lost the race ends its turn inside our window, and its edge is otherwise identical to ours. This is how another invocation's verdict gets published as this one's. |
| `turnSettled` is set from what the turn committed | set it when the session names no active response | A session reads idle mid-turn. An idle snapshot with only an opening sentence behind it is a turn still being written. |
| Attribution requires the item to carry *this turn's* response id | take the newest assistant message not seen before the turn | That fails **open** — anything the pre-turn snapshot missed is accepted as our reply. This package has published another invocation's verdict that way. |
| `assistantMessage` checks `item.Type` **before** decoding `Data` | decode first, then inspect | `AsMessageData` is a bare `json.Unmarshal` with no discriminator consult. It decodes a `function_call_output` into a zero-valued message and reports no error. Tool output on that path carries whole diffs and `gh` responses. |
| `CreatedBy` is used only to reject | treat `nil` as "the model wrote this" | The server documents `nil` for agent, tool and system items *and* for single-user mode. Only non-`nil` attests anything, and what it attests is that a client wrote it. |
| `reachability` reads unknown liveness as **live**; `sessionIsLive` reads it as **not live** | one helper, one default | Same field, opposite costly mistakes: one would delete a live conversation on missing information, the other would send a prompt into a sandbox that does not exist. |
| `priorResponseIDs` pages the item list | read the ids off the create-or-adopt snapshot | That snapshot returns the newest 100 items with nothing marking the truncation, and one session per unit of work is the shape that outgrows it. An id that falls out of view is one the turn machine accepts as its own. |
| `replyFor`'s arms are ordered: ended turn, then recorded fault, then the clock | check the clock first | An ended turn's reply is already committed and `fetchReply` reads on a detached context, so a deadline landing between the end and the read would discard a paid-for answer. A recorded fault outranks the clock for the mirror reason: the expired clock is usually the fault's consequence. |
| `holdsAnswer` asks the caller's own predicate, not whether a message exists | treat a found session as answered | A session this run created and then reconciled after a lost create response is found by the run key with the agent having never spoken in it. So is one from a run that died mid-answer. The second-pass prompt tells the agent it already answered, so sending it into an empty conversation asks a follow-up to an agent that was never asked the first question. |

## A dropped stream is not a failed turn

A connection lives roughly three minutes and a turn runs longer, so several
reconnects per turn are ordinary success. `conversation.Turn` follows one turn
across as many streams as it takes, and the reopen budget is sized for that rather
than as an error bound — see `connectionOpenLimit`.

The consequence is that a turn can be resolved from three different starting
points: an end edge, a recorded fault, or a stream that simply died. The third is
the awkward one, because nothing is replayed on reconnect — rejoining means asking
a snapshot what happened, and what counts as *this run's* reply is this package's
rule, not the protocol's.

## Unsettled

**Whether the in-process end reconciles across id namespaces.** `doc.go` states
that a lifecycle event's `resp_<24 hex>` can never equal an item's response id. The
in-process path does exactly that comparison: `observeResponseTerminal` takes the
lifecycle id as the turn id, and `fetchReply` then looks up items by it. If the
claim holds universally that lookup always misses, and it misses *quietly* — the
run reports no verdict rather than failing.

The test cannot settle it, because we author both sides of the fixture and it uses
one id for the lifecycle event and the item. The likely truth is that `doc.go`'s
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
