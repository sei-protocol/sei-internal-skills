package driver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRunReportsAFinishedAnswer(t *testing.T) {
	t.Parallel()

	conv := &fakeConversation{sessionID: "conv_1", reply: finishedReply}
	host := &fakeHost{conv: conv}
	d := New(Config{Agent: "reviewer", RunDeadline: time.Minute}, host, quietLogger())

	result := d.Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if result.ExitCode != ExitOK {
		t.Errorf("ExitCode = %d, want ExitOK (%d)", result.ExitCode, ExitOK)
	}
	if result.SessionID != "conv_1" {
		t.Errorf("SessionID = %q, want the conversation's own id", result.SessionID)
	}
	if !carriesDecision(result.Reply, "approve") {
		t.Errorf("Reply = %+v, want the finished answer", result.Reply)
	}
	if conv.turns != 1 {
		t.Errorf("turns = %d, want 1: a run drives exactly one", conv.turns)
	}
}

// TestRunReportsNoVerdictWhenTheReplyIsUnfinished pins the distinction the workload
// owns and this package cannot make for itself.
//
// A terminal-backed session goes idle between tool calls, so the server's signals
// read "finished" while the agent is still working. Only the workload's rule
// separates an answer from an opening sentence, and reading the server instead
// published one as a review.
func TestRunReportsNoVerdictWhenTheReplyIsUnfinished(t *testing.T) {
	t.Parallel()

	host := &fakeHost{conv: &fakeConversation{sessionID: "conv_1", reply: unfinishedReply}}
	d := New(Config{RunDeadline: time.Minute}, host, quietLogger())

	result := d.Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if result.ExitCode != ExitNoVerdict {
		t.Errorf("ExitCode = %d, want ExitNoVerdict (%d)", result.ExitCode, ExitNoVerdict)
	}
	if result.Reply == nil {
		t.Fatal("Reply = nil, want the unusable reply carried so its reason reaches the caller")
	}
	if result.Reply.Text != unfinishedReply.Text {
		t.Errorf("Reply.Text = %q, want the text the turn did produce", result.Reply.Text)
	}
}

// TestRunCarriesAReplyReadBeforeAFailure pins that a turn can answer and still
// fail. A stream expiring after the agent replied is the ordinary case, and
// discarding the text leaves no way to tell a truncated answer from a refusal.
func TestRunCarriesAReplyReadBeforeAFailure(t *testing.T) {
	t.Parallel()

	host := &fakeHost{conv: &fakeConversation{
		sessionID: "conv_1",
		reply:     finishedReply,
		turnErr:   errors.New("the stream was interrupted"),
	}}
	d := New(Config{RunDeadline: time.Minute}, host, quietLogger())

	result := d.Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if result.ExitCode != ExitTransport {
		t.Errorf("ExitCode = %d, want ExitTransport (%d)", result.ExitCode, ExitTransport)
	}
	if result.SessionID != "conv_1" {
		t.Errorf("SessionID = %q, want the session named even on a failure: it is what "+
			"an operator reads the run back from", result.SessionID)
	}
}

// TestRunMapsEveryFailureOntoItsExitCode pins the contract a calling workflow
// branches on. The numbers only ever widen, so each of these is load-bearing.
func TestRunMapsEveryFailureOntoItsExitCode(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		err  error
		want int
	}{
		{"a configuration fault", wrapped(ErrConfig, "no agent named x"), ExitConfig},
		{"a credential that will not mint", wrapped(ErrMint, "invalid_client"), ExitConfig},
		{"the session reporting failure", wrapped(ErrTurnFailed, "the response failed"), ExitTurnFailed},
		{"a cancelled run", context.Canceled, ExitCancelled},
		{"anything else", errors.New("connection reset by peer"), ExitTransport},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			host := &fakeHost{openErr: c.err}
			d := New(Config{RunDeadline: time.Minute}, host, quietLogger())

			result := d.Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

			if result.ExitCode != c.want {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, c.want)
			}
			if result.Reply != nil {
				t.Errorf("Reply = %+v, want nil: nothing was answered", result.Reply)
			}
		})
	}
}

// TestClassifyTellsARequestTimeoutFromTheRunDeadline pins which deadline an
// expired one was.
//
// A create that never returns headers, and the reconcile behind it, exhaust the
// SDK's unary timeout rather than the run's budget. That error satisfies
// errors.Is(err, context.DeadlineExceeded) either way, so the run context is the
// only thing that says which deadline expired — and reading the error alone
// reported "run deadline exceeded" on a run with most of its twenty minutes left.
func TestClassifyTellsARequestTimeoutFromTheRunDeadline(t *testing.T) {
	t.Parallel()

	d := New(Config{RunDeadline: 20 * time.Minute}, &fakeHost{}, quietLogger())

	t.Run("a request timeout on a live run is transport", func(t *testing.T) {
		t.Parallel()
		// The shape net/http produces for Client.Timeout: it wraps
		// context.DeadlineExceeded while the caller's own context is still good.
		err := fmt.Errorf("Post %q: %w (Client.Timeout exceeded while awaiting headers)",
			"https://example.invalid/v1/sessions", context.DeadlineExceeded)
		got := d.classify(t.Context(), Result{ExitCode: ExitOK}, err)
		if got.ExitCode != ExitTransport {
			t.Errorf("ExitCode = %d, want ExitTransport (%d): the run had budget left, one call did not",
				got.ExitCode, ExitTransport)
		}
	})

	t.Run("the run deadline expiring is a timeout", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
		defer cancel()
		<-ctx.Done()
		got := d.classify(ctx, Result{ExitCode: ExitOK}, context.DeadlineExceeded)
		if got.ExitCode != ExitTimeout {
			t.Errorf("ExitCode = %d, want ExitTimeout (%d)", got.ExitCode, ExitTimeout)
		}
	})
}

// TestRunBoundsTheWholeRunOnItsDeadline pins that the budget is applied once, at
// the top, so no step below needs its own deadline arithmetic.
func TestRunBoundsTheWholeRunOnItsDeadline(t *testing.T) {
	t.Parallel()

	host := &fakeHost{openErr: context.DeadlineExceeded}
	d := New(Config{RunDeadline: time.Nanosecond}, host, quietLogger())

	result := d.Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if result.ExitCode != ExitTimeout {
		t.Errorf("ExitCode = %d, want ExitTimeout (%d): the run's own budget expired",
			result.ExitCode, ExitTimeout)
	}
}

// TestRunAsksTheSecondPassPromptWhenTheConversationHoldsAnAnswer pins why a session
// is worth reusing: the earlier reply is still in front of the agent, so the prompt
// can ask what changed.
func TestRunAsksTheSecondPassPromptWhenTheConversationHoldsAnAnswer(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 22}
	for _, c := range []struct {
		name     string
		answered bool
		want     string
	}{
		{"a fresh conversation gets the first-pass prompt", false, work.Prompt(false)},
		{"one that already answered gets the second", true, work.Prompt(true)},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			conv := &fakeConversation{sessionID: "conv_1", reply: finishedReply, answered: c.answered}
			d := New(Config{RunDeadline: time.Minute}, &fakeHost{conv: conv}, quietLogger())

			d.Run(t.Context(), work)

			if conv.prompt != c.want {
				t.Errorf("prompt = %q, want %q", conv.prompt, c.want)
			}
		})
	}
}

// TestWorkForNamesTheAgentTheWorkloadAsks pins [AgentNamer]. Work composed of
// several asks that must not share a harness needs each on a named agent, and
// falling back to the default would answer on the same harness while looking like a
// working run.
func TestWorkForNamesTheAgentTheWorkloadAsks(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 22}
	for _, c := range []struct {
		name string
		work Workload
		want string
	}{
		{"no preference takes the configured default", work, "reviewer"},
		{"a named agent wins", namedWork{testWork: work, agent: "codex-scout"}, "codex-scout"},
		{"an empty name is no preference", namedWork{testWork: work, agent: ""}, "reviewer"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			host := &fakeHost{conv: &fakeConversation{sessionID: "conv_1", reply: finishedReply}}
			d := New(Config{Agent: "reviewer", RunDeadline: time.Minute}, host, quietLogger())

			d.Run(t.Context(), c.work)

			if len(host.opened) != 1 {
				t.Fatalf("opened %d conversations, want 1", len(host.opened))
			}
			if got := host.opened[0]; got.Agent != c.want {
				t.Errorf("Work.Agent = %q, want %q", got.Agent, c.want)
			}
			if got := host.opened[0]; got.RunKey != work.RunKey() || got.Title != work.Title() {
				t.Errorf("Work = %+v, want the workload's own run key and title", got)
			}
		})
	}
}

func TestCloseReportsEachOutcomeDistinctly(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name       string
		id         string
		err        error
		wantCode   int
		wantTeardn bool
	}{
		{"a deleted session", "conv_1", nil, ExitOK, true},
		{"no session at all", "", nil, ExitOK, true},
		{"a session that would not delete", "conv_1", wrapped(ErrLeaked, "conv_1: 500"), ExitTeardownLeak, false},
		{"a server that could not be reached", "", errors.New("connection refused"), ExitTransport, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			host := &fakeHost{closeID: c.id, closeErr: c.err}
			d := New(Config{RunDeadline: time.Minute}, host, quietLogger())

			result := d.Close(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

			if result.ExitCode != c.wantCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, c.wantCode)
			}
			if result.TeardownOK != c.wantTeardn {
				t.Errorf("TeardownOK = %t, want %t", result.TeardownOK, c.wantTeardn)
			}
			if result.SessionID != c.id {
				t.Errorf("SessionID = %q, want %q: a leak has to name what leaked",
					result.SessionID, c.id)
			}
		})
	}
}

// TestCloseOutlivesACancelledRun pins the one guard that cannot be tested through a
// real deployment without racing it.
//
// A terminate signal cancels the run context so that teardown can run, so a close
// that inherited it would abort the delete it exists to perform. Close is the only
// thing that frees a sandbox: the launcher sets no lifetime cap and the server runs
// no sweep, so a session this misses holds its pod's cpu and memory for good.
func TestCloseOutlivesACancelledRun(t *testing.T) {
	t.Parallel()

	host := &fakeHost{closeID: "conv_1"}
	d := New(Config{RunDeadline: time.Minute}, host, quietLogger())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := d.Close(ctx, testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if !host.closeCtxLive {
		t.Error("the host's context was already cancelled: teardown must not inherit " +
			"the context whose cancellation is what asked for it")
	}
	if result.ExitCode != ExitOK || !result.TeardownOK {
		t.Errorf("ExitCode = %d, TeardownOK = %t, want ExitOK and true",
			result.ExitCode, result.TeardownOK)
	}
}
