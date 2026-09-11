package driver

import (
	"testing"
	"time"
)

// TestNewSubstitutesANonPositiveRunDeadline covers the field omni.New cannot: it is
// this package's, and this constructor is the one config.go's exported-defaults
// paragraph names.
//
// A zero is not an unbounded run. context.WithTimeout with it yields a context that
// has already expired, so the run logs that it started, fails its first request in
// microseconds, and reports ExitTimeout -- the code a caller reads as "the agent took
// too long" on a run that never reached the server.
//
// Every other construction in this suite sets RunDeadline explicitly, which is why
// nothing caught it.
func TestNewSubstitutesANonPositiveRunDeadline(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		set  time.Duration
		want time.Duration
	}{
		{"zero", 0, DefaultRunDeadline},
		{"negative", -time.Second, DefaultRunDeadline},
		{"a caller's own value is kept", 90 * time.Second, 90 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := New(Config{RunDeadline: tc.set}, &fakeHost{}, quietLogger())
			if d.cfg.RunDeadline != tc.want {
				t.Errorf("RunDeadline = %v, want %v", d.cfg.RunDeadline, tc.want)
			}
		})
	}
}

// TestDefaultRunDeadlineLeavesHeadroomOverAnOrdinaryReview pins the default against
// the longest review turn observed on a large diff. A deadline the turn grazes ends a
// run that was about to answer with exit 3 and no verdict, so the default has to
// clear that mark by a margin and not by minutes.
func TestDefaultRunDeadlineLeavesHeadroomOverAnOrdinaryReview(t *testing.T) {
	t.Parallel()

	const observedLongest = 1029 * time.Second
	if DefaultRunDeadline < observedLongest*3/2 {
		t.Errorf("DefaultRunDeadline = %v, want at least 1.5x the %v review turn observed",
			DefaultRunDeadline, observedLongest)
	}
}

// TestAZeroRunDeadlineDoesNotReportATimeout is the consequence, through Run rather
// than through the field: the exit code a caller branches on must not say the agent
// was slow when the driver never asked it anything.
func TestAZeroRunDeadlineDoesNotReportATimeout(t *testing.T) {
	t.Parallel()

	host := &fakeHost{conv: &fakeConversation{sessionID: "conv_1", reply: finishedReply}}
	result := New(Config{Agent: "reviewer"}, host, quietLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if result.ExitCode == ExitTimeout {
		t.Error("ExitCode = ExitTimeout on a run that was never given a deadline")
	}
	if len(host.opened) != 1 {
		t.Errorf("host opened %d times, want 1: the run has to reach the host at all",
			len(host.opened))
	}
}
