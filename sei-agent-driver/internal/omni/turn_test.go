package omni

import (
	"errors"
	"testing"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// The turn machine decides when an exchange is over, and it decides it from edges
// that mean different things depending on the harness underneath. These exercise it
// directly rather than through a stream, because the cases that matter are the ones
// a fixture cannot easily stage: an edge naming a response that predates the run, an
// edge arriving before the boundary, an end signal that belongs to the other harness.

// anchored returns a turn that has already seen its prompt echoed back, which is the
// state every end rule is conditioned on.
func anchored(prior map[string]bool, terminal bool) *turn {
	t := newTurn(prior, func(string) bool { return true }, terminal)
	t.anchor = "item_1"
	t.crossBoundary(omnigent.SessionInputConsumedEvent{
		Data: omnigent.SessionInputConsumedPayload{ItemID: "item_1"},
	})
	return t
}

func idle(id string) omnigent.SessionStatusEvent {
	e := omnigent.SessionStatusEvent{Status: omnigent.SessionStatusEventStatusIdle}
	if id != "" {
		e.ResponseID = omnigent.Ptr(id)
	}
	return e
}

func TestCrossBoundaryMatchesEitherIdentifier(t *testing.T) {
	t.Parallel()

	t.Run("an item id the send returned", func(t *testing.T) {
		t.Parallel()
		tn := newTurn(nil, nil, true)
		tn.anchor = "item_1"
		tn.crossBoundary(omnigent.SessionInputConsumedEvent{
			Data: omnigent.SessionInputConsumedPayload{ItemID: "item_1"},
		})
		if !tn.crossed || tn.anchorItem != "item_1" {
			t.Errorf("crossed=%v anchorItem=%q", tn.crossed, tn.anchorItem)
		}
	})

	t.Run("a pending id resolves to the item it drained into", func(t *testing.T) {
		t.Parallel()
		tn := newTurn(nil, nil, true)
		tn.anchor = "pending_1"
		tn.crossBoundary(omnigent.SessionInputConsumedEvent{
			Data: omnigent.SessionInputConsumedPayload{
				ItemID: "item_9", ClearedPendingID: omnigent.Ptr("pending_1"),
			},
		})
		if !tn.crossed {
			t.Fatal("a pending anchor was not matched by the id it drained")
		}
		if tn.anchorItem != "item_9" {
			t.Errorf("anchorItem = %q, want the item, since positions compare against it",
				tn.anchorItem)
		}
	})

	t.Run("another message's echo is not ours", func(t *testing.T) {
		t.Parallel()
		tn := newTurn(nil, nil, true)
		tn.anchor = "item_1"
		tn.crossBoundary(omnigent.SessionInputConsumedEvent{
			Data: omnigent.SessionInputConsumedPayload{ItemID: "item_2"},
		})
		if tn.crossed {
			t.Error("crossed on an echo that named a different item")
		}
	})
}

func TestObserveStatusEndsOnlyOnAnIdBearingIdleAfterTheBoundary(t *testing.T) {
	t.Parallel()

	t.Run("an idle edge carrying a fresh id ends the turn", func(t *testing.T) {
		t.Parallel()
		tn := anchored(nil, true)
		tn.observeStatus(idle("resp_claude_a"))
		if tn.id != "resp_claude_a" || !tn.ended() {
			t.Errorf("id=%q ended=%v", tn.id, tn.ended())
		}
	})

	t.Run("a bare idle edge ends nothing", func(t *testing.T) {
		t.Parallel()
		tn := anchored(nil, true)
		tn.observeStatus(idle(""))
		if tn.ended() {
			t.Error("a bare idle edge ended the turn; a session emits several mid-work")
		}
	})

	t.Run("an idle edge before the boundary ends nothing", func(t *testing.T) {
		t.Parallel()
		tn := newTurn(nil, nil, true)
		tn.observeStatus(idle("resp_claude_a"))
		if tn.ended() {
			t.Error("ended on an edge from the prologue, before our prompt was echoed")
		}
	})

	t.Run("an id already on the session cannot be this turn", func(t *testing.T) {
		t.Parallel()
		tn := anchored(map[string]bool{"resp_claude_old": true}, true)
		tn.observeStatus(idle("resp_claude_old"))
		if tn.ended() {
			t.Error("ended on a response that predates this run: a superseded run's turn " +
				"finishing inside our window is not our turn finishing")
		}
	})
}

// TestObserveStatusIgnoresAFailureThatPredatesTheRun is the failed-edge half of the
// prior check. A previous dispatch's turn goes on running server-side, so its failure
// arrives inside our window -- taken as ours it ends the run, and salvage would then
// read that turn's reply and publish it as this one's.
func TestObserveStatusIgnoresAFailureThatPredatesTheRun(t *testing.T) {
	t.Parallel()

	failed := func(id string) omnigent.SessionStatusEvent {
		return omnigent.SessionStatusEvent{
			Status:     omnigent.SessionStatusEventStatusFailed,
			ResponseID: omnigent.Ptr(id),
		}
	}

	t.Run("an older response failing is not our failure", func(t *testing.T) {
		t.Parallel()
		tn := anchored(map[string]bool{"resp_claude_old": true}, true)
		tn.observeStatus(failed("resp_claude_old"))
		if tn.failure != nil || tn.failedTurnID != "" {
			t.Errorf("failure=%v failedTurnID=%q: an earlier turn's failure ended this run",
				tn.failure, tn.failedTurnID)
		}
	})

	t.Run("our own response failing is", func(t *testing.T) {
		t.Parallel()
		tn := anchored(nil, true)
		tn.observeStatus(failed("resp_claude_a"))
		if !errors.Is(tn.failure, driver.ErrTurnFailed) {
			t.Errorf("failure = %v, want ErrTurnFailed", tn.failure)
		}
		if tn.failedTurnID != "resp_claude_a" {
			t.Errorf("failedTurnID = %q, want the id salvage reads against", tn.failedTurnID)
		}
	})
}

func TestObserveResponseTerminalBelongsToTheInProcessHarnessOnly(t *testing.T) {
	t.Parallel()

	t.Run("ignored on a terminal-backed harness", func(t *testing.T) {
		t.Parallel()
		tn := anchored(nil, true)
		tn.observeResponseTerminal("resp_1", "completed", nil)
		if tn.ended() {
			t.Error("ended on a lifecycle event, which there acknowledges only that the " +
				"prompt reached the terminal")
		}
	})

	t.Run("ends the turn on an in-process harness", func(t *testing.T) {
		t.Parallel()
		tn := anchored(nil, false)
		tn.observeResponseTerminal("resp_1", "completed", nil)
		if tn.id != "resp_1" {
			t.Errorf("id = %q, want resp_1: the lifecycle is the end signal there", tn.id)
		}
	})

	t.Run("still requires the boundary", func(t *testing.T) {
		t.Parallel()
		tn := newTurn(nil, nil, false)
		tn.observeResponseTerminal("resp_1", "completed", nil)
		if tn.ended() {
			t.Error("ended before our prompt was echoed back")
		}
	})

	t.Run("still excludes a response that predates the run", func(t *testing.T) {
		t.Parallel()
		tn := anchored(map[string]bool{"resp_old": true}, false)
		tn.observeResponseTerminal("resp_old", "completed", nil)
		if tn.ended() {
			t.Error("ended on a response that was already live before our prompt")
		}
	})

	t.Run("a terminal that carries a fault fails rather than ends", func(t *testing.T) {
		t.Parallel()
		tn := anchored(nil, false)
		tn.observeResponseTerminal("resp_1", "failed", errors.New("boom"))
		if tn.id != "" {
			t.Error("a failed response ended the turn; it did not end, it failed")
		}
		if tn.failedTurnID != "resp_1" || tn.failure == nil {
			t.Errorf("failedTurnID=%q failure=%v", tn.failedTurnID, tn.failure)
		}
	})
}

// TestFailKeepsTheFirstCause pins that the reason a run reports is the one that
// stopped it, not whatever arrived last on the way out.
func TestFailKeepsTheFirstCause(t *testing.T) {
	t.Parallel()

	tn := newTurn(nil, nil, true)
	first := errors.New("the sandbox never launched")
	tn.fail(first)
	tn.fail(errors.New("stream closed"))
	if !errors.Is(tn.failure, first) {
		t.Errorf("failure = %v, want the first cause", tn.failure)
	}
}

// TestObserveSupersededFailsLoudly covers a Claude /clear. The live terminal moves
// to a new conversation while the run key still points at the retired one, so every
// later review of this pull request would adopt a dead session.
func TestObserveSupersededFailsLoudly(t *testing.T) {
	t.Parallel()

	tn := anchored(nil, true)
	tn.observeSuperseded(omnigent.SessionSupersededEvent{TargetConversationID: "conv_new"})
	if !errors.Is(tn.failure, driver.ErrTurnFailed) {
		t.Errorf("failure = %v, want ErrTurnFailed", tn.failure)
	}
	if !tn.ended() {
		t.Error("a superseded session left the turn running")
	}
}

func TestTerminalBackedTreatsAnUnknownHarnessStrictly(t *testing.T) {
	t.Parallel()

	for harness, want := range map[string]bool{
		"claude-native":  true,
		"":               true,
		"something-new":  true,
		"codex":          false,
		"CODEX":          false,
		"  claude-sdk  ": false,
		"openai-agents":  false,
	} {
		if got := terminalBacked(harness); got != want {
			t.Errorf("terminalBacked(%q) = %v, want %v: an unrecognised harness takes the "+
				"stricter rule, because a wait announces itself and a half-written review "+
				"does not", harness, got, want)
		}
	}
}
