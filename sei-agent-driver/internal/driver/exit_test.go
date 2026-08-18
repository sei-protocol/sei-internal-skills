package driver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

// panickingHost stands in for a defect anywhere below this package -- a nil map
// written, an index off the end, a type assertion on a shape the server changed.
type panickingHost struct{ on string }

func (h *panickingHost) Open(ctx context.Context, w Work) (Conversation, error) {
	if h.on == "open" {
		panic("a nil map write below Open")
	}
	return &fakeConversation{sessionID: "conv_1", reply: finishedReply}, nil
}

func (h *panickingHost) Close(ctx context.Context, w Work) (string, error) {
	if h.on == "close" {
		panic("a nil map write below Close")
	}
	return "conv_1", nil
}

// TestAPanicIsReportedApartFromAConfigurationFailure pins the reason
// [ExitInternal] exists. Unrecovered, the runtime exits 2, which is [ExitConfig]:
// the caller reads "fix your configuration", stops retrying, and an operator goes
// looking for a variable that was never wrong.
func TestAPanicIsReportedApartFromAConfigurationFailure(t *testing.T) {
	for _, on := range []string{"open", "close"} {
		t.Run("a panic below "+on, func(t *testing.T) {
			d := New(Config{RunDeadline: time.Minute, RequestTimeout: time.Second},
				&panickingHost{on: on}, quietLogger())

			var result Result
			if on == "open" {
				result = d.Run(context.Background(), testWork{})
			} else {
				result = d.Close(context.Background(), testWork{})
			}

			if result.ExitCode != ExitInternal {
				t.Errorf("ExitCode = %d, want ExitInternal (%d)", result.ExitCode, ExitInternal)
			}
		})
	}
}

// TestCloseReportsAnUnfinishedDeleteAsAPossibleLeak covers the ambiguity a close
// cannot resolve.
//
// When the budget runs out mid-close, nothing here knows whether the delete landed.
// The two readings cost differently once the process is gone: a needless check
// against a session that was already deleted, or a pod holding its reserved cpu and
// memory with nothing that will ever reclaim it. Only the second keeps accruing, so
// the ambiguity resolves toward the leak.
func TestCloseReportsAnUnfinishedDeleteAsAPossibleLeak(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sessionID string
		err       error
	}{
		// The first two are the production shapes: omni's host reports no session id on
		// any error path except a refused delete, so this arm nearly always runs blind.
		{"the budget expired before anything was listed", "", context.DeadlineExceeded},
		{"cancelled before anything was listed", "", fmt.Errorf("listing sessions: %w", context.Canceled)},
		{"the budget expired with a session in hand", "conv_1", context.DeadlineExceeded},
		{"expired mid-delete", "conv_1", fmt.Errorf("deleting conv_1: %w", context.DeadlineExceeded)},
		{"cancelled with a session in hand", "conv_1", context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &fakeHost{closeID: tc.sessionID, closeErr: tc.err}
			d := New(Config{RunDeadline: time.Minute, RequestTimeout: time.Second},
				host, quietLogger())

			result := d.Close(context.Background(), testWork{})

			if result.ExitCode != ExitTeardownLeak {
				t.Errorf("ExitCode = %d, want ExitTeardownLeak (%d): an unfinished close "+
					"has not established that the sandbox is free",
					result.ExitCode, ExitTeardownLeak)
			}
			if result.ExitCode == ExitTimeout {
				t.Error("a possibly held sandbox is reporting itself as a slow server")
			}
			if result.TeardownOK {
				t.Error("TeardownOK is true on a close that never confirmed the delete")
			}
			// Whatever the host knew travels, which on this path is usually nothing: a
			// close that timed out before it listed anything has no session to name. The
			// run key is what an operator searches on, and it is on the log record.
			if result.SessionID != tc.sessionID {
				t.Errorf("SessionID = %q, want %q", result.SessionID, tc.sessionID)
			}
		})
	}
}

// TestCloseStillTellsAConfigFailureFromALeak guards the arm above from swallowing
// the other outcomes. A close that could not find out whether a session existed is
// a different report from one that found a session and could not free it.
func TestCloseStillTellsAConfigFailureFromALeak(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"a bad configuration", wrapped(ErrConfig, "OMNIGENT_BASE_URL"), ExitConfig},
		{"an unreachable server", errors.New("dial tcp: connection refused"), ExitTransport},
		{"a session that would not delete", wrapped(ErrLeaked, "conv_1: 500"), ExitTeardownLeak},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &fakeHost{closeID: "conv_1", closeErr: tc.err}
			d := New(Config{RunDeadline: time.Minute, RequestTimeout: time.Second},
				host, quietLogger())

			if got := d.Close(context.Background(), testWork{}); got.ExitCode != tc.want {
				t.Errorf("ExitCode = %d, want %d", got.ExitCode, tc.want)
			}
		})
	}
}

// TestCloseKeepsACredentialFailureApartFromAPossibleLeak pins the ordering of the
// two arms above.
//
// An error can satisfy both. http.Client wraps a body read that outran its timeout
// in an error that reports itself as a deadline -- verified against a real server --
// so a stalled token endpoint reaches Close as a mint failure that is also a
// deadline. Read as the deadline it becomes ExitTeardownLeak, and the operator is
// sent to delete a sandbox by hand when the actionable fact is a credential: fix it,
// re-run the close, and the reclaim happens on its own.
func TestCloseKeepsACredentialFailureApartFromAPossibleLeak(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"a mint that stalled mid-body", fmt.Errorf("%w: reading response: %w", ErrMint, context.DeadlineExceeded)},
		{"a bad address that timed out", fmt.Errorf("%w: %w", ErrConfig, context.DeadlineExceeded)},
		{"a mint cancelled in flight", fmt.Errorf("%w: %w", ErrMint, context.Canceled)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &fakeHost{closeErr: tc.err}
			d := New(Config{RunDeadline: time.Minute, RequestTimeout: time.Second},
				host, quietLogger())

			result := d.Close(context.Background(), testWork{})

			if result.ExitCode != ExitConfig {
				t.Errorf("ExitCode = %d, want ExitConfig (%d): the credential is the "+
					"actionable half, and repairing it reclaims the sandbox on the retry",
					result.ExitCode, ExitConfig)
			}
			// Still not established as freed. This never reached the delete, so a
			// session may be held -- the reclaim is deferred, not done.
			if result.TeardownOK {
				t.Error("TeardownOK is true on a close that never reached the delete")
			}
		})
	}
}

// panickingHandler fails on the record whose message matches, standing in for a
// panic raised after the turn already produced its answer.
type panickingHandler struct{ on string }

func (h *panickingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *panickingHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *panickingHandler) WithGroup(string) slog.Handler            { return h }

func (h *panickingHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Message == h.on {
		panic("a handler that panicked on the last record")
	}
	return nil
}

// TestARecoveredPanicKeepsTheSessionItKnewAbout covers the one handle a crash can
// still offer. A panic raised after a session was opened leaves that session
// running, and the id is the only way to reach it.
func TestARecoveredPanicKeepsTheSessionItKnewAbout(t *testing.T) {
	host := &fakeHost{conv: &fakeConversation{sessionID: "conv_42", reply: finishedReply}}
	log := slog.New(&panickingHandler{on: "run finished"})
	d := New(Config{RunDeadline: time.Minute, RequestTimeout: time.Second}, host, log)

	result := d.Run(context.Background(), testWork{})

	if result.ExitCode != ExitInternal {
		t.Errorf("ExitCode = %d, want ExitInternal (%d)", result.ExitCode, ExitInternal)
	}
	if result.SessionID != "conv_42" {
		t.Errorf("SessionID = %q, want the session the run had already opened", result.SessionID)
	}
}
