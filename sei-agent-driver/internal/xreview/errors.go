package xreview

import "errors"

// Exit codes. The caller workflow branches on these, so the numbers are a
// contract with it and carry over unchanged from the Python driver — a workflow
// pinned to an older ref must keep reading them the same way.
const (
	// ExitOK is a completed turn that produced a verdict. A repeat dispatch for the
	// same pull request is not a separate outcome: it adopts that pull request's
	// session and drives a fresh turn like any other run.
	ExitOK = 0

	// ExitConfig is a configuration or credential problem. Nothing was sent.
	ExitConfig = 2

	// ExitTimeout is the run deadline expiring. The turn is stopped and the
	// conversation kept, like every other exit — only DeleteSessionForPR destroys a
	// session, and it runs when the pull request closes.
	ExitTimeout = 3

	// ExitTurnFailed is the session reporting failure, which is the agent's
	// outcome rather than the driver's.
	ExitTurnFailed = 4

	// ExitNoVerdict is a turn that ended without a verdict this driver could
	// read. Distinct from ExitTurnFailed: the turn may have succeeded and simply
	// not produced the final message shape the caller needs.
	ExitNoVerdict = 5

	// ExitTransport is the stream or a request failing in a way retrying inside
	// one run cannot fix.
	ExitTransport = 6

	// ExitCancelled is a terminate signal unwinding the run. Teardown is
	// attempted on the way out.
	ExitCancelled = 7

	// ExitTeardownLeak is a --close whose session could not be deleted. A review
	// never reports it: it deletes nothing, because the session is meant to
	// outlive the run.
	//
	// Surfaced rather than swallowed because the delete is the only thing that
	// reclaims the sandbox. The Kubernetes launcher sets no lifetime cap and the
	// server runs no sweep, so nothing else will: the pod holds its reserved cpu
	// and memory until it is removed by hand. Only promoted over ExitOK, so a
	// leak beside another non-zero outcome shows up in the logs rather than in
	// the exit code.
	ExitTeardownLeak = 8
)

// Sentinels, matchable with [errors.Is] by a caller that wants to branch on the
// class rather than the exit code.
var (
	// ErrConfig is a configuration or credential defect.
	ErrConfig = errors.New("configuration")

	// ErrTurnFailed is the session reporting a failed turn.
	ErrTurnFailed = errors.New("turn failed")
)
