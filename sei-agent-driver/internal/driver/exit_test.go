package driver

import (
	"context"
	"errors"
	"fmt"
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
			if result.ExitCode == ExitConfig {
				t.Error("a crash is reporting itself as a configuration failure")
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
		name string
		err  error
	}{
		{"the budget expired", context.DeadlineExceeded},
		{"the budget expired mid-request", fmt.Errorf("deleting conv_1: %w", context.DeadlineExceeded)},
		{"the close was cancelled", context.Canceled},
		{"cancelled mid-request", fmt.Errorf("listing sessions: %w", context.Canceled)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &fakeHost{closeID: "conv_1", closeErr: tc.err}
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
			// The session id still travels, because it is the only thing that makes the
			// exit code actionable by hand.
			if result.SessionID != "conv_1" {
				t.Errorf("SessionID = %q, want the session an operator has to chase",
					result.SessionID)
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
