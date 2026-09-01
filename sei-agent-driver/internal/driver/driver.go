package driver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

// minTeardownBudget is the least time a close gets, whatever the configured
// request timeout.
//
// Sized for what a close actually does rather than for the delete alone: a close
// runs in its own process, so it mints a token first, and that mint may spend three
// attempts and its own backoff before a single session has been listed. A budget
// covering only the delete would be exhausted by the credential exchange in front
// of it and reclaim nothing.
const minTeardownBudget = 30 * time.Second

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
	// which deletes nothing and so leaks nothing, and on a close that deleted what it
	// found.
	//
	// False means only that this did not establish the sandbox was freed: a session
	// found and refused, a close that ran out of budget before it could look, and a
	// close that never authenticated all report it. [ExitTeardownLeak] covers the
	// first two, so the exit code narrows the reason without settling it -- the log
	// names which, and carries the run key to search on.
	TeardownOK bool
}

// Driver runs one [Workload] to an answer on a [Host].
type Driver struct {
	cfg  Config
	host Host
	log  *slog.Logger
}

// New returns a driver. The logger receives one structured record per decision this
// package makes — which session, which run key, whether the reply was usable, and
// what the run exited with — because those are the questions asked when a run
// misbehaves and nobody is watching. The prompts a turn parks on, and the turn's own
// end, are the [Host]'s to record.
func New(cfg Config, host Host, log *slog.Logger) *Driver {
	// A non-positive RunDeadline is not an unbounded run, it is a context that has
	// already expired -- so Run would log "run starting", fail its first request in
	// microseconds, and report ExitTimeout on a run that never left the ground.
	// [DefaultRunDeadline] is what [LoadConfig] would have supplied, and this is the
	// other constructor that doc names; omni.New does the same for the three fields
	// it owns.
	if cfg.RunDeadline <= 0 {
		cfg.RunDeadline = DefaultRunDeadline
	}
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
// It tears nothing down itself. The session outlives the run, for the reasons in
// the package doc, and [Driver.Close] is what ends it -- though opening will delete
// a session it finds unable to run a turn at all.
//
// What that leaves behind is a turn still running when a run ends early, on a
// cancelled context or an expired deadline. The next invocation's prompt queues
// behind it rather than racing it, so this is latency rather than corruption.
func (d *Driver) Run(ctx context.Context, w Workload) (result Result) {
	work := d.workFor(w)
	defer d.recoverPanic(work, &result)

	d.log.Info("run starting", "run_key", work.RunKey, "title", work.Title)

	// The whole run is bounded here, so every call below inherits it and no
	// individual step needs its own deadline arithmetic.
	ctx, cancel := context.WithTimeout(ctx, d.cfg.RunDeadline)
	defer cancel()

	result = d.answer(ctx, work, w)
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
// This is the end of the work, not the end of a run, and it is what frees a sandbox
// once the work is done: a close that never happens leaks one for good, because
// nothing reaps it on a schedule. Opening reclaims one too, but only a session it
// finds unable to run a turn. See the package doc.
//
// Absent is not an error. Work that ended without ever being started has no
// session, and saying so is not a failure.
func (d *Driver) Close(ctx context.Context, w Workload) (result Result) {
	work := d.workFor(w)
	defer d.recoverPanic(work, &result)

	d.log.Info("closing out the session", "run_key", work.RunKey, "title", work.Title)

	// Detached, with a budget of its own. A terminate signal cancels ctx so that
	// teardown can run, so inheriting that ctx here would abort the delete this
	// method exists to perform. One guard, on the path every close takes.
	//
	// The budget is a small multiple of one request rather than the run's, because
	// this is a handful of requests and it usually runs on a process that is already
	// terminating. A runner grants a bounded window before it sends SIGKILL, so a
	// twenty minute budget does not buy twenty minutes -- it only means the process
	// is killed part-way through with the delete never issued, and the sandbox held.
	// Floored, so a Config that never went through LoadConfig cannot hand teardown
	// no budget at all -- which would silently skip the reclaim this method exists
	// to perform.
	//
	// The same argument applies to this budget and is not resolved here. At the
	// default request timeout it is two minutes, which the runner's window may also
	// be shorter than, and nothing in this process reads that window to find out.
	// Minting is the largest share of it: three attempts at one request timeout each,
	// plus backoff, can spend most of the budget before a session has been listed --
	// and the deletes, the only step that frees anything, take what is left.
	//
	// Picking a smaller number without measuring the window would trade one guess
	// for another, so what is written down is the assumption: this is only as good
	// as the grace the runner actually grants. Measure a teardown that was killed
	// mid-flight before changing the multiplier.
	budget := max(4*d.cfg.RequestTimeout, minTeardownBudget)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
	defer cancel()

	sessionID, err := d.host.Close(ctx, work)
	switch {
	case errors.Is(err, ErrLeaked):
		d.log.Error("could not delete the session; the sandbox will leak until reclaimed",
			"session_id", sessionID, "error", err)
		return Result{ExitCode: ExitTeardownLeak, SessionID: sessionID}
	case errors.Is(err, ErrConfig), errors.Is(err, ErrMint):
		// Ahead of the unfinished-close arm, because an error can satisfy both and
		// only one of them can be acted on. http.Client wraps a body read that outran
		// its timeout in an error reporting itself as a deadline, so a stalled token
		// endpoint arrives here as a mint failure that is also a deadline. Told to fix
		// the variable and re-run, a close reclaims the sandbox on the retry; told to
		// go deleting by hand, an operator does the same work manually.
		//
		// A session may still be held either way -- this never reached the delete --
		// which is why TeardownOK stays false and the reclaim is only deferred.
		d.log.Error("the close could not authenticate or address the server; "+
			"re-run it once that is fixed",
			"run_key", work.RunKey, "agent", work.Agent, "error", err)
		return Result{ExitCode: ExitConfig, SessionID: sessionID}
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		// A close that did not finish cannot say whether the delete landed, and it is
		// reported as the leak rather than the timeout because that is the actionable
		// half of the ambiguity. A caller reading ExitTimeout goes looking for a slow
		// server; a caller reading this goes looking for a held sandbox. Only one of
		// those costs anything once the process is gone, and a needless check is
		// cheaper than a pod nothing reclaims.
		//
		// Not gated on ctx.Err() the way [Driver.classify] gates the run deadline. There
		// the distinction earns its keep, because a single slow request must not read as
		// the whole run expiring. Here either reading ends at the same place: nothing
		// established that the session is gone.
		//
		// The session id is usually empty here, because a close that timed out before
		// it listed anything has none to report. The run key is what an operator
		// searches on in that case.
		d.log.Error("the close did not finish; a sandbox may still be held",
			"run_key", work.RunKey, "agent", work.Agent,
			"session_id", sessionID, "budget", budget, "error", err)
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

// recoverPanic turns a panic into an exit code, and both entry points defer it.
//
// The contract this package states is that every outcome is a number the caller acts
// on. A panic breaks it in the worst way available: the runtime exits 2, which is
// [ExitConfig], so a crash arrives as "fix your configuration" and a caller that
// would have retried a transient fault stops instead.
//
// It cannot cover a panic on another goroutine -- recover reaches only the stack it
// is deferred on, and the runtime takes the process down at 2 whatever this does. So
// it narrows the collision rather than closing it, and [ExitConfig] carries what is
// left.
//
// The stack goes to the log because an exit code cannot be debugged, and this is the
// last point at which the panic is still in hand.
func (d *Driver) recoverPanic(work Work, result *Result) {
	v := recover()
	if v == nil {
		return
	}
	d.log.Error("the driver panicked",
		"run_key", work.RunKey, "session_id", result.SessionID,
		"panic", fmt.Sprint(v), "stack", string(debug.Stack()))
	// The session id survives the outcome it arrived with. A panic after a session
	// was opened still leaves one, and it is the only handle on it.
	*result = Result{ExitCode: ExitInternal, SessionID: result.SessionID}
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
// by, the title it is listed under, and the agent and model it must run on — those
// being the ones it names when it names them, and the run's configured defaults
// otherwise. See [AgentNamer].
func (d *Driver) workFor(w Workload) Work {
	agent := d.cfg.Agent
	model := d.cfg.Model
	if a, ok := w.(AgentNamer); ok {
		if named := a.AgentName(); named != "" {
			agent = named

			// And with it the model, because the configured one was chosen for the
			// default agent. A workload on its own agent is a second harness, and a
			// model name one provider answers to is rejected at turn start by another
			// -- which costs the reading, not just the model.
			model = ""
		}
	}
	return Work{RunKey: w.RunKey(), Title: w.Title(), Agent: agent, Model: model}
}
