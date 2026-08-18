package driver

import "errors"

// Exit codes. The caller workflow branches on these, so the numbers are a
// contract with it and carry over unchanged from the Python driver — a workflow
// pinned to an older ref must keep reading them the same way.
//
// Exported, unlike the defaults in config.go, because that contract is the reason
// they exist: the command reads them to decide what to report, and a caller outside
// this module branches on the numbers themselves.
const (
	// ExitOK is a completed turn that produced a verdict. A repeat dispatch for the
	// same pull request is not a separate outcome: it adopts that pull request's
	// session and drives a fresh turn like any other run.
	ExitOK = 0

	// ExitConfig is a configuration or credential problem. Nothing was sent.
	ExitConfig = 2

	// ExitTimeout is the run deadline expiring. Nothing is interrupted: a turn that
	// had started goes on server-side and the next invocation's prompt queues behind
	// it, and a run whose prompt never went in leaves nothing running at all -- the
	// commonest cause, a sandbox that never reported ready. So a caller must not read
	// this as "still working".
	//
	// The conversation is kept, as on every exit. Only a close deletes a session that
	// can still run a turn; opening deletes one it finds unable to.
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
	// Surfaced rather than swallowed because nothing reclaims the sandbox on a
	// schedule. The Kubernetes launcher sets no lifetime cap and the
	// server runs no sweep, so nothing else will: the pod holds its reserved cpu
	// and memory until it is removed by hand. Only promoted over ExitOK, so a
	// leak beside another non-zero outcome shows up in the logs rather than in
	// the exit code.
	ExitTeardownLeak = 8
)

// Sentinels, matchable with [errors.Is] by a caller that wants to branch on the
// class rather than the exit code.
//
// They live here rather than beside the code that produces them because they are
// the input to [Driver.classify], which is what turns an error into one of the
// exit codes above. A [Host] reaching a server states its failures in these terms
// so the exit-code contract does not depend on that server's own error taxonomy.
var (
	// ErrConfig is a configuration or credential defect.
	ErrConfig = errors.New("configuration")

	// ErrTurnFailed is the session reporting a failed turn.
	ErrTurnFailed = errors.New("turn failed")

	// ErrMint is a failure to exchange machine credentials for an access token.
	ErrMint = errors.New("token exchange")

	// ErrLeaked is a session found and not deleted, so its sandbox is held. It is
	// what separates the two ways [Driver.Close] fails: this one held a session and
	// could not free it, and anything else could not find out whether there was one.
	ErrLeaked = errors.New("session not deleted")
)
