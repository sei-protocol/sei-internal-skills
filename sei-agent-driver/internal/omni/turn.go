package omni

import (
	"fmt"
	"strings"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// A turn is one prompt and the answer to it. This file holds when that exchange
// is over and how the driver knows: the state a turn carries, the events that
// move it, and the two rules for reading an end out of a stream that reports one
// differently depending on the harness underneath.
type turn struct {
	// anchor is our own prompt's item id, as the server assigned it.
	//
	// Set inside the subscription hook, which the SDK documents as running on the
	// caller's goroutine before the first event reaches it. So it is in place
	// before anything can be attributed, and needs no synchronisation.
	anchor string

	// anchorItem is the conversation item the boundary resolved to.
	//
	// [turn.anchor] is whichever identifier the send returned, and a prompt parked
	// before the runtime is up returns a pending id, which names no item. Recovery
	// compares positions, so it reads this one.
	anchorItem string

	// crossed reports that the server echoed the anchor back.
	//
	// Until it does, everything on the stream is history or another actor's work.
	// The stream opens with a prologue that replays earlier items, and that prologue
	// can carry a previous invocation's completed assistant message, arriving before
	// our own prompt is persisted. No check on an item's content distinguishes that
	// message from a real reply; only its position does.
	crossed bool

	// id is the turn's response id, taken from the edge that ended the turn.
	//
	// It is deliberately not learned earlier or from anywhere else. In particular
	// it is not read off our own prompt item: that item carries whichever response
	// was last active, which is a stale id from before the boundary.
	id string

	// prior is every response id already on the session when this run started. An
	// id in it cannot belong to the turn answering our prompt, however well-timed
	// its edge looks.
	prior map[string]bool

	answered map[string]bool

	// complete is the workload's test for a finished answer. Held here because
	// the salvage paths need it and they are four frames from the caller that
	// knows the workload.
	complete func(text string) bool

	// frames counts everything this turn has read off any stream, so a
	// reconnect can tell an open that carried something from one that did not.
	frames int

	// turnSettled records that waiting longer cannot change this turn's outcome,
	// which is what stops a reconnect from watching for edges that will not come.
	//
	// Deliberately not "the session named no active response". A claude-native
	// session goes idle mid-turn, so an idle snapshot with only the agent's
	// opening sentence behind it is a turn still being written, not a finished
	// one -- reading it as finished publishes a review the agent never wrote. It
	// is set when the reply carries a reply, and when two replies make
	// attribution impossible; both are outcomes waiting cannot improve. It stays
	// false while the agent may still be working.
	turnSettled bool

	// failure is the first fatal signal. Written once, so the cause a run reports
	// is the one that actually stopped it.
	failure error

	// terminalBacked reports that this turn runs on a harness whose status edges
	// carry a response id, which is what makes the id-bearing idle edge available
	// as a turn end. False for an in-process harness, where that edge never comes
	// and the response lifecycle is the end instead.
	//
	// Unknown harnesses are terminal-backed, deliberately: the failure that costs
	// more is publishing a half-written review, and that is what the lifecycle
	// event produces on the harnesses that ack their injection.
	terminalBacked bool

	// failedTurnID is the response id a failed edge named, when it named one.
	// Deliberately not id: a failed turn did not end, and only
	// [Driver.salvageFailedTurn] may read a reply against this.
	failedTurnID string
}

func newTurn(prior map[string]bool, complete func(string) bool, terminalBacked bool) *turn {
	return &turn{
		prior:          prior,
		complete:       complete,
		answered:       map[string]bool{},
		terminalBacked: terminalBacked,
	}
}

// inProcessHarnesses are the harnesses whose turns end on the response lifecycle,
// listed rather than pattern-matched so an unknown one cannot silently take the
// looser rule. These are what the deployed runner advertises on connect.
var inProcessHarnesses = map[string]bool{
	"codex":             true,
	"claude-sdk":        true,
	"claude_sdk":        true,
	"openai-agents":     true,
	"openai-agents-sdk": true,
	"agents_sdk":        true,
	"open-responses":    true,
	"pi":                true,
}

// terminalBacked reports whether a harness drives a real terminal, which is what
// decides where its status edges get a response id.
//
// An allowlist rather than a test for "native" in the name: a harness this does
// not recognise answers true and keeps the stricter rule. Getting that backwards
// costs a published half-written review, where getting it this way costs a turn
// that waits for its deadline — the same failure we already know how to see.
func terminalBacked(harness string) bool {
	return !inProcessHarnesses[strings.ToLower(strings.TrimSpace(harness))]
}

func (t *turn) fail(err error) {
	if t.failure == nil {
		t.failure = err
	}
}

// ended reports whether there is nothing further to read.
func (t *turn) ended() bool { return t.id != "" || t.failure != nil }

// crossBoundary marks the point after which events can be this turn's.
func (t *turn) crossBoundary(e omnigent.SessionInputConsumedEvent) {
	// Either identifier, because the anchor is whichever the send returned. A
	// prompt persisted straight away is echoed by its item id; one parked as a
	// pending input is echoed by the pending id it drains, on the same event. The
	// item id is checked first because it is always populated, so a run holding a
	// pending anchor is not matched by another message's item.
	if e.Data.ItemID == t.anchor {
		t.crossed = true
		t.anchorItem = e.Data.ItemID
		return
	}
	if e.Data.ClearedPendingID != nil && *e.Data.ClearedPendingID == t.anchor {
		t.crossed = true
		// The item the pending input drained into. Always populated, and the only
		// one of the two identifiers that names a conversation item.
		t.anchorItem = e.Data.ItemID
	}
}

// observeStatus reads a coarse session status edge.
//
// An idle edge carrying a response id, after the boundary, is the end of the turn
// and the only thing that is. A bare idle edge is pane churn, so a missing
// response id downgrades the edge to noise rather than making it a wildcard.
func (t *turn) observeStatus(e omnigent.SessionStatusEvent) {
	if e.Status == omnigent.SessionStatusEventStatusFailed {
		if t.crossed && e.ResponseID != nil {
			t.failedTurnID = *e.ResponseID
		}
		t.fail(fmt.Errorf("%w: %s", driver.ErrTurnFailed, statusDetail(e)))
		return
	}
	if e.Status != omnigent.SessionStatusEventStatusIdle || !t.crossed {
		return
	}
	if e.ResponseID == nil || *e.ResponseID == "" {
		return
	}
	if t.prior[*e.ResponseID] {
		// Already on the session before we sent our prompt, so it cannot be the
		// turn that answers it. This is the reachable half of the overlapping-run
		// hazard: a superseded run whose stop lost the race ends its turn inside
		// our window, and its edge is otherwise indistinguishable from ours.
		return
	}
	t.id = *e.ResponseID
}

// observeResponseTerminal reads a response-lifecycle terminal event, which is what
// ends a turn on an in-process harness.
//
// Ignored on a terminal-backed harness, where the same event means only that the
// prompt reached the terminal. The prior check is the one the idle path already
// makes: a response that was live before our prompt can complete inside our window
// and its event is otherwise indistinguishable from ours.
func (t *turn) observeResponseTerminal(id, kind string, detail error) {
	if t.terminalBacked || !t.crossed || id == "" || t.prior[id] {
		return
	}
	if detail != nil {
		t.failedTurnID = id
		t.fail(detail)
		return
	}
	t.id = id
}

// observeSuperseded ends the turn on a Claude /clear.
//
// The live terminal moves to a new conversation, so a verdict read here would land
// somewhere nothing is watching. Worse, the run key still points at the retired
// conversation, so every later review of this pull request adopts a dead session
// until someone intervenes — which is why this fails loudly rather than following
// the redirect.
func (t *turn) observeSuperseded(e omnigent.SessionSupersededEvent) {
	t.fail(fmt.Errorf("%w: the session was superseded; its conversation is now %s",
		driver.ErrTurnFailed, e.TargetConversationID))
}

// statusDetail renders a failed edge's error, which is the reason to watch this
// event rather than infer failure from silence.
func statusDetail(e omnigent.SessionStatusEvent) string {
	if e.Error == nil {
		return "the session reported failure, with no detail"
	}
	return fmt.Sprintf("the session reported failure: %s (%s)", e.Error.Message, e.Error.Code)
}
