package driver

import (
	"context"
	"errors"
	"log/slog"
)

// Result is the outcome of a run.
type Result struct {
	// ExitCode is what the process should exit with. See the Exit constants.
	ExitCode int

	// Reply is the agent's answer, when the turn produced one. Reading it is
	// the workload's; this package only attributes it.
	Reply *Reply

	// SessionID is the session driven, when one was opened or adopted.
	SessionID string

	// TeardownOK reports that no session was left holding a sandbox. True on a run,
	// which deletes nothing and so leaks nothing, and on a close that deleted what
	// it found. A false value is what [ExitTeardownLeak] reports.
	TeardownOK bool
}

// Driver runs one [Workload] to an answer on a [Host].
type Driver struct {
	cfg  Config
	host Host
	log  *slog.Logger
}

// New returns a driver. The logger receives one structured record per decision
// point — which session, which run key, which prompt was answered how, and how the
// turn ended — because those are the questions asked when a run misbehaves and
// nobody is watching.
func New(cfg Config, host Host, log *slog.Logger) *Driver {
	return &Driver{cfg: cfg, host: host, log: log}
}

// Run drives one turn to an answer.
//
// It returns no error, because every outcome a caller acts on is an exit code:
// this runs unattended, and the caller's contract is the number it exits with plus
// the log behind it. A failure that stopped the run before the agent said anything
// is [ExitConfig] or [ExitTransport] with a nil Reply, which the caller reports the
// same way it reports any other outcome.
//
// It tears nothing down. The session outlives the run, for the reasons in the
// package doc, and [Driver.Close] is what ends it.
//
// What that leaves behind is a turn still running when a run ends early, on a
// cancelled context or an expired deadline. The next invocation's prompt queues
// behind it rather than racing it, so this is latency rather than corruption.
func (d *Driver) Run(ctx context.Context, w Workload) Result {
	work := d.workFor(w)
	d.log.Info("run starting", "run_key", work.RunKey, "title", work.Title)

	// The whole run is bounded here, so every call below inherits it and no
	// individual step needs its own deadline arithmetic.
	ctx, cancel := context.WithTimeout(ctx, d.cfg.RunDeadline)
	defer cancel()

	result := d.answer(ctx, work, w)
	d.log.Info("run finished",
		"run_key", work.RunKey, "session_id", result.SessionID,
		"exit_code", result.ExitCode, "teardown_ok", result.TeardownOK)
	return result
}

// answer is the body of a run: open the conversation, take one turn, and decide
// whether what came back is an answer.
func (d *Driver) answer(ctx context.Context, work Work, w Workload) Result {
	result := Result{ExitCode: ExitOK, TeardownOK: true}

	conv, err := d.host.Open(ctx, work)
	if err != nil {
		return d.classify(ctx, result, err)
	}
	result.SessionID = conv.SessionID()

	reply, err := conv.Turn(ctx, Ask{Prompt: w.Prompt, Done: w.Complete})
	if err != nil {
		// A turn can produce a reply and still fail: a stream that expires after
		// the agent answered is the ordinary case. classify returns on the error
		// and the text goes with it, which otherwise leaves a failed run with no
		// record of what the agent said and no way to tell a truncated answer
		// from a refusal.
		if reply.Text != "" || reply.Reason != "" {
			d.log.Warn("a reply was read but the run failed before publishing it",
				"session_id", result.SessionID, "turn_id", reply.TurnID,
				"chars", len(reply.Text), "reason", reply.Reason)
		}
		return d.classify(ctx, result, err)
	}

	if !w.Complete(reply.Text) {
		d.log.Warn("turn produced no usable reply", "session_id", result.SessionID,
			"reason", reply.Reason, "chars", len(reply.Text))
		result.ExitCode = ExitNoVerdict
		// Carried even with no text, so the reason reaches the caller's payload
		// rather than only the logs.
		result.Reply = &reply
		return result
	}

	result.Reply = &reply
	d.log.Info("turn complete", "session_id", result.SessionID,
		"turn_id", reply.TurnID, "chars", len(reply.Text))
	return result
}

// Close ends the unit of work and reclaims what it held.
//
// This is the end of the work, not the end of a run, and it is the only thing that
// frees a sandbox: a close that never happens leaks one for good, because nothing
// reaps it later. See the package doc.
//
// Absent is not an error. Work that ended without ever being started has no
// session, and saying so is not a failure.
func (d *Driver) Close(ctx context.Context, w Workload) Result {
	work := d.workFor(w)
	d.log.Info("closing out the session", "run_key", work.RunKey, "title", work.Title)

	// Detached, with its own budget. A terminate signal cancels ctx so that
	// teardown can run, so inheriting that ctx here would abort the delete this
	// method exists to perform. One guard, on the path every close takes.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.cfg.RunDeadline)
	defer cancel()

	sessionID, err := d.host.Close(ctx, work)
	switch {
	case errors.Is(err, ErrLeaked):
		d.log.Error("could not delete the session; the sandbox will leak until reclaimed",
			"session_id", sessionID, "error", err)
		return Result{ExitCode: ExitTeardownLeak, SessionID: sessionID}
	case err != nil:
		return d.classify(ctx, Result{ExitCode: ExitOK}, err)
	case sessionID == "":
		// Warn, not Info. "There was nothing to reclaim" and "I looked in the wrong
		// place" are the same observation from here: the search is scoped to one
		// agent, so a close dispatched with a different agent name than the run used
		// finds nothing and reports success while the sandbox is still held.
		d.log.Warn("no session carried this run key; nothing was reclaimed",
			"run_key", work.RunKey, "agent", work.Agent)
		return Result{ExitCode: ExitOK, TeardownOK: true}
	default:
		d.log.Info("session deleted", "session_id", sessionID)
		return Result{ExitCode: ExitOK, SessionID: sessionID, TeardownOK: true}
	}
}

// classify maps an error onto the exit code the caller reports.
//
// The run context decides what a deadline error means. An expired unary timeout
// also satisfies errors.Is(err, context.DeadlineExceeded), so without consulting
// the run context a single slow request is reported as the whole run running out of
// budget, on a run with most of its budget left.
func (d *Driver) classify(ctx context.Context, result Result, err error) Result {
	switch {
	case errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil:
		d.log.Error("run deadline exceeded", "budget", d.cfg.RunDeadline)
		result.ExitCode = ExitTimeout
	case errors.Is(err, context.Canceled):
		d.log.Error("run cancelled")
		result.ExitCode = ExitCancelled
	case errors.Is(err, ErrTurnFailed):
		d.log.Error("turn failed", "error", err)
		result.ExitCode = ExitTurnFailed
	case errors.Is(err, ErrConfig), errors.Is(err, ErrMint):
		d.log.Error("configuration or request rejected before sending", "error", err)
		result.ExitCode = ExitConfig
	default:
		d.log.Error("transport or server error", "error", err)
		result.ExitCode = ExitTransport
	}
	return result
}

// workFor reduces a workload to the identity a host needs: the run key it is found
// by, the title it is listed under, and the agent it must run on — that being the
// one it names when it names one, and the run's configured default otherwise. See
// [AgentNamer].
func (d *Driver) workFor(w Workload) Work {
	agent := d.cfg.Agent
	if a, ok := w.(AgentNamer); ok {
		if named := a.AgentName(); named != "" {
			agent = named
		}
	}
	return Work{RunKey: w.RunKey(), Title: w.Title(), Agent: agent}
}
