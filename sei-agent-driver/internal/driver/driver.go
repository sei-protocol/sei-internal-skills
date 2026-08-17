package driver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
)

// How an HTTP/2 connection proves it is still there.
//
// The idle bound is above the server's 15-second stream heartbeat so a healthy
// stream keeps resetting it, and a ping is only sent once frames have actually
// stopped. The ping bound is short because a live peer answers immediately; the
// cost of waiting is another request written into a dead socket.
const (
	http2ReadIdleTimeout = 20 * time.Second
	http2PingTimeout     = 5 * time.Second

	// defaultResponseHeaderTimeout bounds the wait for response headers. Long
	// enough for a create that provisions a sandbox, short enough that a stream
	// open which will never answer is retried rather than waited out.
	defaultResponseHeaderTimeout = 60 * time.Second
)

// RunKeyLabel carries a workload's run key on the session, so a later dispatch
// recognises a session an earlier one created. Namespaced because labels are a
// shared surface.
//
// The value still says xreview because it is written on live sessions: changing
// it orphans every session a running deployment would otherwise adopt. Two
// workloads write it — a review and each scout — and they do not collide because
// [xreview.ScoutRunKey] carries the scout's name into the key.
const RunKeyLabel = "xreview.seinetwork.io/run-key"

// Result is the outcome of a run.
type Result struct {
	// ExitCode is what the process should exit with. See the Exit constants.
	ExitCode int

	// Reply is the agent's answer, when the turn produced one. Reading it is
	// the workload's; this package only attributes it.
	Reply *Reply

	// SessionID is the session driven, when one was created or adopted.
	SessionID string

	// TeardownOK reports whether the close deleted the session. True on a review,
	// which deletes nothing. A false value is what [ExitTeardownLeak] reports.
	TeardownOK bool
}

// Driver runs one review against an Omnigent deployment.
type Driver struct {
	cfg    Config
	policy Policy
	log    *slog.Logger
}

// NewDriver returns a driver. The logger receives one structured record per
// decision point — which session, which run key, which prompt was answered how,
// and how the turn ended — because those are the questions asked when a review
// misbehaves and nobody is watching.
func NewDriver(cfg Config, policy Policy, log *slog.Logger) *Driver {
	return &Driver{cfg: cfg, policy: policy, log: log}
}

// Run performs the review.
//
// It returns a Result rather than an error for the outcomes the caller reports
// through an exit code, and an error only for the ones that mean the run never
// started: a bad configuration, or a credential that will not mint.
func (d *Driver) Run(ctx context.Context, w Workload) (Result, error) {
	runKey := w.RunKey()
	d.log.Info("run starting", "run_key", runKey, "title", w.Title())

	// The whole run is bounded here, so every call below inherits it and no
	// individual step needs its own deadline arithmetic.
	ctx, cancel := context.WithTimeout(ctx, d.cfg.RunDeadline)
	defer cancel()

	client, err := d.newClient(ctx)
	if err != nil {
		// Through classify like every other failure. The exit code is the caller's
		// contract — it decides whether to tell an operator to fix a secret or to
		// retry — and only classify can tell a token exchange that failed on the
		// network from a configuration fault.
		return d.classify(ctx, Result{ExitCode: ExitOK, TeardownOK: true}, err), err
	}

	result := d.review(ctx, client, w, runKey)
	d.log.Info("run finished",
		"run_key", runKey, "session_id", result.SessionID,
		"exit_code", result.ExitCode, "teardown_ok", result.TeardownOK)
	return result, nil
}

// review is the body of a run, after the client is built. It tears nothing down;
// the session outlives the run, for the reasons in the package doc.
//
// What that leaves behind is a turn still running when a run ends early, on a
// cancelled context or an expired deadline. The next invocation's prompt queues
// behind it rather than racing it, so this is latency rather than corruption.
func (d *Driver) review(
	ctx context.Context,
	client *omnigent.Client,
	w Workload,
	runKey string,
) Result {
	result := Result{ExitCode: ExitOK, TeardownOK: true}

	agentName := d.agentNameFor(w)
	agent, err := d.resolveAgent(ctx, client, agentName)
	if err != nil {
		return d.classify(ctx, result, err)
	}
	harness := ""
	if agent.Harness != nil {
		harness = *agent.Harness
	}
	d.log.Info("resolved agent", "agent", agentName, "agent_id", agent.ID,
		"harness", harness)

	session, adopted, err := d.createOrAdopt(ctx, client, agent.ID, runKey, w)
	if err != nil {
		return d.classify(ctx, result, err)
	}
	result.SessionID = session.ID
	d.log.Info("session ready", "session_id", session.ID,
		"continued", adopted.continued, "answered", adopted.answered, "live", adopted.live)

	// The response ids already on the session, captured before the turn so its own
	// reply can be told apart from the history a reused session carries.
	prior, err := d.priorResponseIDs(ctx, client, session.ID)
	if err != nil {
		return d.classify(ctx, result, err)
	}

	reply, err := d.driveTurn(ctx, client, session.ID, w, adopted, prior, harness)
	if err != nil {
		// A turn can produce a reply and still fail: a stream that expires after
		// the agent answered is the ordinary case. classify returns on the error
		// and the text goes with it, which otherwise leaves a failed run with no
		// record of what the agent said and no way to tell a truncated review
		// from a refusal.
		if reply.Text != "" || reply.Reason != "" {
			d.log.Warn("a reply was read but the run failed before publishing it",
				"session_id", session.ID, "turn_id", reply.TurnID,
				"chars", len(reply.Text), "reason", reply.Reason,
				"preview", clip(reply.Text, replyPreviewChars))
		}
		return d.classify(ctx, result, err)
	}

	if !w.Complete(reply.Text) {
		d.log.Warn("turn produced no usable reply", "session_id", session.ID,
			"reason", reply.Reason, "chars", len(reply.Text),
			"preview", clip(reply.Text, replyPreviewChars))
		result.ExitCode = ExitNoVerdict
		// Carried even with no text, so the reason reaches the caller's payload
		// rather than only the logs.
		result.Reply = &reply
		return result
	}

	result.Reply = &reply
	d.log.Info("turn complete", "session_id", session.ID,
		"turn_id", reply.TurnID, "chars", len(reply.Text))
	return result
}

// classify maps an error onto the exit code the caller reports.
//
// The run context decides what a deadline error means. An expired unary timeout
// also satisfies errors.Is(err, context.DeadlineExceeded), so without consulting
// the run context a single slow request is reported as the whole run running out
// of budget -- which it was, for a run that had spent three of twenty minutes.
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
	case errors.Is(err, omnigent.ErrInvalidArgument), errors.Is(err, ErrConfig), errors.Is(err, ErrMint):
		d.log.Error("configuration or request rejected before sending", "error", err)
		result.ExitCode = ExitConfig
	default:
		d.log.Error("transport or server error", "error", err)
		result.ExitCode = ExitTransport
	}
	return result
}

// agentNameFor returns the agent this workload runs on: the one it names when it
// names one, and the run's configured default otherwise. See [AgentNamer].
func (d *Driver) agentNameFor(w Workload) string {
	if a, ok := w.(AgentNamer); ok {
		if name := a.AgentName(); name != "" {
			return name
		}
	}
	return d.cfg.Agent
}

// createOrAdopt returns the session for this run key, creating one only if none
// exists yet.
//
// Searching first is the idempotency guarantee. The run key is a label on the
// session, so it outlives the runner and a redelivered trigger finds the first
// run's session rather than reviewing the same tree twice. It walks every page
// because the server has no label filter and a page holds the agent's 20 newest.
//
// It is not a lock: two simultaneous runs can both find nothing and both create,
// which the caller's concurrency group prevents. This rules out the sequential
// duplicate.

// adoption is where a run's session came from, split into the two questions the
// rest of the run actually asks. They are separate because a session can be
// continued and not live, and answering both from one bit sends the prompt into a
// sandbox that does not exist.
type adoption struct {
	// continued reports that this session was found rather than opened here, so it
	// may be holding prompts parked before this stream existed.
	continued bool

	// answered reports that its conversation already holds an answer to this work,
	// which decides which prompt it gets. Distinct from continued because a found
	// session need not have been answered in: see [holdsAnswer].
	answered bool

	// live reports that a runner is registered right now, which decides whether
	// the prompt goes in on subscribe or waits for the launch pipeline.
	live bool
}

// createOrAdopt finds this pull request's session or opens one, and refuses to
// hand back a session that can never run a turn.
//
// The refusal is the point. A session whose sandbox never launched is stopped
// with its conversation intact, so the run key still finds it forever, and
// whether it can be revived is the provider's call: a resumable host wakes when
// sent a message, a non-resumable one is a dead end. Adopting the dead end makes
// every later review of that pull request fail the same way, so it is deleted and
// replaced instead.
func (d *Driver) createOrAdopt(
	ctx context.Context,
	client *omnigent.Client,
	agentID, runKey string,
	w Workload,
) (*omnigent.SessionResponse, adoption, error) {
	existing, err := d.findByRunKey(ctx, client, agentID, runKey)
	if err != nil {
		return nil, adoption{}, fmt.Errorf("looking for an existing session: %w", err)
	}
	if existing != nil {
		if live, revivable := reachability(existing); live || revivable {
			d.log.Info("adopting the session an earlier dispatch created",
				"run_key", runKey, "session_id", existing.ID, "live", live)
			return existing, adoption{continued: true, answered: holdsAnswer(existing, w.Complete), live: live}, nil
		}
		d.log.Warn("the session for this pull request cannot run a turn; replacing it",
			"run_key", runKey, "session_id", existing.ID)
		if _, err := client.DeleteSession(ctx, existing.ID, omnigent.DeleteSessionOptions{}); err != nil {
			return nil, adoption{}, fmt.Errorf(
				"the session for this pull request cannot run a turn and could not be "+
					"deleted, so a new one would collide with it: %w", err)
		}
	}

	create := omnigent.SessionCreateRequest{
		AgentID:  agentID,
		HostType: "managed",
		Title:    w.Title(),
		Labels:   map[string]string{RunKeyLabel: runKey},
	}

	session, err := client.CreateSession(ctx, create)
	if err == nil {
		return session, adoption{}, nil
	}

	// A rejected argument means nothing was sent, so there is no session to
	// reconcile against and searching would only hide the real fault.
	if errors.Is(err, omnigent.ErrInvalidArgument) {
		return nil, adoption{}, err
	}

	// A second search, for a different case than the one above: create may have
	// committed server-side and lost its response, and this run must not retry
	// it. The SDK deliberately never retries a create for exactly that reason.
	d.log.Warn("create failed; looking for a session it may have committed",
		"run_key", runKey, "error", err)
	committed, findErr := d.findByRunKey(ctx, client, agentID, runKey)
	if findErr != nil {
		return nil, adoption{}, fmt.Errorf("create failed (%w) and reconcile failed: %w", err, findErr)
	}
	if committed == nil {
		return nil, adoption{}, err
	}
	live, _ := reachability(committed)
	return committed, adoption{continued: true, answered: holdsAnswer(committed, w.Complete), live: live}, nil
}

// holdsAnswer reports whether this session's conversation already carries a
// reply the workload counts as finished.
//
// This is what [adoption.answered] means, and it cannot be read off how the
// session was found. The session reconciled above is one this run just created,
// so the run key finds it while the agent has never answered in it; a run that
// expired before its first reply leaves the same thing for the next one to adopt.
// Both look continued and are not. The distinction is load-bearing because a
// workload's second-pass prompt tells the agent it has already answered this
// work, so sending it into an empty conversation puts a follow-up question to an
// agent that was never asked the first one.
//
// The workload's own rule decides, rather than the presence of a message. An
// agent commits its opening sentence as a completed message before it has done
// anything, so "a reply exists" would count a run that died mid-answer as having
// answered. Both that and a turn still in flight read as no answer, which is the
// safe direction: the worst case is answering in full twice, against putting a
// follow-up question to an agent that never answered the first one.
func holdsAnswer(s *omnigent.SessionResponse, complete func(string) bool) bool {
	for _, id := range ReplyGroupsSince(s.Items, nil) {
		if reply, ok := TurnReply(s.Items, id); ok && complete(reply.Text) {
			return true
		}
	}
	return false
}

// reachability reads whether a session can take a prompt now, and whether it
// could after being woken.
//
// runner_online is the server's sole reachability signal: true means a runner
// tunnel is registered and the session can be chatted to. When it is false,
// host_resumable splits a dormant managed host the provider can wake in place
// from a terminal one that it cannot. A nil runner_online means the server has no
// liveness lookup wired, which is not evidence the session is dead, so it is read
// as live rather than deleting a session on missing information.
func reachability(s *omnigent.SessionResponse) (live, revivable bool) {
	if s.RunnerOnline == nil || *s.RunnerOnline {
		return true, false
	}
	return false, s.HostResumable != nil && *s.HostResumable
}

// DeleteSession destroys the session for a pull request, and with it the
// conversation.
//
// This is the end of the unit of work, not the end of a run — it belongs to the
// pull request closing or merging, and it is the only thing that reclaims a
// sandbox. A close event that never arrives leaks one for good; nothing reaps it
// later. See the package doc.
//
// Absent is not an error. A pull request closed without ever being reviewed has
// no session, and saying so is not a failure.
func (d *Driver) DeleteSession(ctx context.Context, w Workload) (Result, error) {
	runKey := w.RunKey()
	d.log.Info("closing out the session", "run_key", runKey, "title", w.Title())

	client, err := d.newClient(ctx)
	if err != nil {
		return d.classify(ctx, Result{ExitCode: ExitOK}, err), err
	}
	agentName := d.agentNameFor(w)
	agent, err := d.resolveAgent(ctx, client, agentName)
	if err != nil {
		return d.classify(ctx, Result{ExitCode: ExitOK}, err), err
	}
	session, err := d.findByRunKey(ctx, client, agent.ID, runKey)
	if err != nil {
		return d.classify(ctx, Result{ExitCode: ExitOK}, err), err
	}
	if session == nil {
		d.log.Info("no session for this pull request; nothing to close", "run_key", runKey)
		return Result{ExitCode: ExitOK, TeardownOK: true}, nil
	}
	// Detached, with its own budget: a terminate signal cancels ctx so teardown can
	// run, so passing that ctx here would abort the delete it exists to allow.
	del, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.cfg.RequestTimeout)
	defer cancel()
	if _, err := client.DeleteSession(del, session.ID, omnigent.DeleteSessionOptions{}); err != nil {
		d.log.Error("could not delete the session; the sandbox will leak until reclaimed",
			"session_id", session.ID, "error", err)
		return Result{ExitCode: ExitTeardownLeak, SessionID: session.ID}, nil
	}
	d.log.Info("session deleted", "session_id", session.ID)
	return Result{ExitCode: ExitOK, SessionID: session.ID, TeardownOK: true}, nil
}

// driveTurn sends the prompt and reads the stream until the turn that answers it
// ends.
//
// Which signal ends the turn depends on the harness, because the server emits a
// different one for each and neither is available on the other.
//
// On a terminal-backed harness it is a session status edge reporting idle and
// carrying a response id, arriving after the server has echoed our prompt back.
// The response lifecycle's terminal event is not usable there: it is an
// acknowledgement that the prompt reached the terminal, so it arrives before the
// answer exists, and treating it as the end finished the turn a second or two in.
// A bare idle is not usable either — one recorded trace carries five, one of them
// squarely mid-work.
//
// On an in-process harness that id-bearing edge never comes. The server documents
// response_id as "None for ordinary in-process runtime edges", and only the route
// a native forwarder posts to attaches one, from Claude Code's Stop hook. Waiting
// for it there is waiting for something the server will never send: a codex scout
// produced a complete report and the wait ran its full budget and discarded it.
// The response lifecycle IS the end there — the executor yields it only on a final
// answer, and the relay commits the assistant message before publishing it.
//
// The prompt is sent from the subscription hook rather than before the stream
// opens. The server buffers nothing, so a turn started before the subscription is
// live publishes its first events to nobody.
func (d *Driver) driveTurn(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
	w Workload,
	from adoption,
	prior map[string]bool,
	harness string,
) (Reply, error) {
	t := newTurn(prior, w.Complete, terminalBacked(harness))

	defer d.logTurnObserved(sessionID, t)

	prompt := w.Prompt(from.answered)

	// Asked of any session this run did not open, reviewed or not: a prompt parked
	// before this stream existed is never replayed onto it, so it is read from the
	// snapshot instead. A session an earlier run left blocked on an elicitation has
	// no reply yet and is exactly the one that needs this.
	if from.continued {
		if err := d.answerPending(ctx, client, sessionID, t.answered); err != nil {
			return Reply{}, err
		}
	}

	// Liveness, not continuation, decides when the prompt goes in. A live session
	// takes it as soon as the stream is up; one whose sandbox is still launching
	// would accept it without queueing it, leaving no anchor, so it waits. See
	// [Driver.sendOnSubscribe] and [Driver.sendWhenLaunched], the two arms of this.
	opts := omnigent.StreamOptions{}
	if from.live {
		opts.OnSubscribed = d.sendOnSubscribe(client, prompt, t)
	}

	// A launching sandbox emits almost nothing and a quiet connection is dropped in
	// transit, so on a cold start the first stream usually dies before the sandbox
	// is ready. Waiting longer cannot help — the connection does not survive the
	// wait — so the stream is re-established while the prompt is still unsent.
	//
	// Once it is in, a lost stream is a different problem with a reply already
	// committed, which [Driver.recoverFromStreamLoss] reads back.
	// The budget is generous because a healthy long turn spends it: the connection
	// cap is roughly three minutes and a review runs longer, so several reconnects
	// are success rather than failure. Sized too tightly, a transient open failure
	// lands on a budget already spent that way and ends a turn that was fine.
	for opens := 1; ; opens++ {
		framesBefore := t.frames
		reply, err := d.consumeTurn(ctx, client, sessionID, prompt, t, prior, opts)
		carriedNothing := t.frames == framesBefore

		// A connection lives about three minutes and a review runs longer, so a
		// stream ending is not evidence the work stopped. The turn is followed
		// across as many streams as it takes.
		//
		// The loop is here rather than in the SDK because the server replays
		// nothing: rejoining means reconciling against a snapshot, and what counts
		// as this run's reply is this driver's rule, not the protocol's.
		if t.ended() || ctx.Err() != nil || t.failure != nil {
			return reply, err
		}
		if opens >= openLimit {
			d.log.Error("the stream would not stay up long enough to finish the turn",
				"session_id", sessionID, "opens", opens,
				"prompt_sent", t.anchor != "", "error", err)
			return reply, err
		}

		// Before the prompt is in, the sandbox may have come up while the stream was
		// down, in which case the ready edge has already passed and waiting for it
		// again would hang.
		if t.anchor == "" {
			if d.sessionIsLive(ctx, client, sessionID) {
				opts.OnSubscribed = d.sendOnSubscribe(client, prompt, t)
			}
			continue
		}

		// The hook fires on every subscribe, so once the prompt is in it is disarmed.
		// A second injection reaches a terminal still running the first one's work,
		// which the harness refuses outright and the turn fails with it. Disarmed
		// here rather than guarded inside the send: the anchor is written from the
		// stream's goroutine, and this runs with the stream closed.
		opts.OnSubscribed = nil

		// A stream is a snapshot and a live tail with no replay, so a turn that
		// ended while this one was down sent its last edge to nobody. Watching for
		// that edge again would wait out the whole deadline for something already
		// past. The salvage above has just asked the session which happened, so a
		// turn it reports as finished is resolved from what it committed rather
		// than rejoined.
		if t.anchor != "" && t.turnSettled {
			return reply, err
		}
		// The error class separates a transport that ended from one that went quiet,
		// which is what tells an operator whether an intermediary is capping the
		// connection or the far end stopped talking. Logged per drop because the two
		// have different fixes and neither is visible from a timeout alone.
		d.log.Info("stream ended before the turn did; re-subscribing",
			"session_id", sessionID, "opens", opens,
			"prompt_sent", t.anchor != "", "frames", t.frames,
			"idle_timeout", errors.Is(err, omnigent.ErrStreamIdle),
			"interrupted", errors.Is(err, omnigent.ErrStreamInterrupted), "error", err)

		// Only an open that carried nothing waits. One that carried frames and then
		// died is the ordinary connection cap, and pausing there spends the deadline.
		if carriedNothing {
			d.backoff(ctx, opens)
		}
	}
}

// openLimit bounds how many times the stream is re-established. Set well above
// what a long turn needs, since a review outlives the roughly three-minute
// connection cap a handful of times; it catches only a server that will not
// stream at all. The run deadline is the real bound.
const openLimit = 40

// backoff waits before re-opening a stream that carried nothing, so a server
// coming back from a restart is not hammered while it does. Capped, because the
// run deadline is the real bound and a long sleep spends it without trying.
func (d *Driver) backoff(ctx context.Context, failedOpens int) {
	wait := min(time.Duration(failedOpens)*250*time.Millisecond, 5*time.Second)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// replyPreviewChars bounds how much of a reply reaches a log line.
//
// Long enough to tell a truncated review from a refusal and to carry the first
// finding, short enough that a full review does not land in the log twice over.
const replyPreviewChars = 300

// consumeTurn watches one subscription until the turn ends or the stream does.
func (d *Driver) consumeTurn(
	ctx context.Context,
	client *omnigent.Client,
	sessionID, prompt string,
	t *turn,
	prior map[string]bool,
	opts omnigent.StreamOptions,
) (Reply, error) {
	for ev, err := range client.Stream(ctx, sessionID, opts) {
		if err != nil {
			return d.recoverFromStreamLoss(ctx, client, sessionID, t, err)
		}
		t.seen[eventKey(ev)]++
		t.frames++

		switch e := ev.(type) {
		case omnigent.SessionSandboxStatusEvent:
			if err := d.sendWhenLaunched(ctx, client, sessionID, prompt, t, e); err != nil {
				t.fail(err)
			}

		case omnigent.SessionInputConsumedEvent:
			t.crossBoundary(e)

		case omnigent.ElicitationRequestEvent:
			if err := d.answer(ctx, client, sessionID, ElicitationFromEvent(e), t.answered); err != nil {
				t.fail(err)
			}

		case omnigent.OutputTextDeltaEvent:
			t.deltaChars += len(e.Delta)

		case omnigent.SessionStatusEvent:
			t.observeStatus(e)

		case omnigent.ResponseCompletedEvent:
			t.observeResponseTerminal(e.Response.ID, "completed", nil)

		case omnigent.ResponseFailedEvent:
			t.observeResponseTerminal(e.Response.ID, "failed",
				fmt.Errorf("%w: the response failed", ErrTurnFailed))

		case omnigent.ResponseCancelledEvent:
			t.observeResponseTerminal(e.Response.ID, "cancelled",
				fmt.Errorf("%w: the response was cancelled", ErrTurnFailed))

		case omnigent.IncompleteEvent:
			// Terminal but truncated, so it is failed rather than ended: the text
			// behind it is a review that stops mid-sentence, and publishing one is
			// worse than reporting that nothing came back.
			t.observeResponseTerminal(e.Response.ID, "incomplete",
				fmt.Errorf("%w: the response ended incomplete", ErrTurnFailed))

		case omnigent.SessionSupersededEvent:
			t.observeSuperseded(e)
		}

		if t.ended() {
			break
		}
	}

	return d.replyFor(ctx, client, sessionID, t, prior)
}

// sendPrompt posts the review instruction and records the item id the server gave
// it.
//
// That id is the anchor, and it is the defence against a reused session's
// history: it correlates a send with its echo, and the echo marks where this
// invocation's work begins. An input accepted without queueing produces no
// anchor, so this refuses rather than reviewing blind — streaming on would burn
// the run deadline before saying so.
func (d *Driver) sendPrompt(
	ctx context.Context,
	client *omnigent.Client,
	sessionID, prompt string,
	t *turn,
) error {
	accepted, err := client.SendInput(ctx, sessionID, omnigent.UserMessage(prompt))
	if err != nil {
		return fmt.Errorf("sending the review prompt: %w", err)
	}
	if !accepted.Queued {
		// A refusal and a control input both land here, and the server says which
		// with denied/reason. Reporting the server's own reason beats reporting
		// that something unspecified went wrong.
		if accepted.Denied {
			reason := accepted.Reason
			if reason == "" {
				reason = "no reason given"
			}
			return fmt.Errorf("%w: the server refused the prompt: %s",
				ErrTurnFailed, reason)
		}
		return fmt.Errorf("%w: the server did not queue the prompt, so this run has "+
			"no turn to wait for", ErrTurnFailed)
	}

	// Either identifier anchors the turn, and which one arrives says how far the
	// prompt got rather than whether it landed. A native terminal that is already
	// up persists an item immediately and returns its id; one still starting parks
	// the prompt as a pending input and returns that id instead, and the item is
	// created when the terminal drains it. Both are queued, and the consume event
	// carries whichever this run holds.
	t.anchor = accepted.ItemID
	if t.anchor == "" {
		t.anchor = accepted.PendingID
	}
	if t.anchor == "" {
		return fmt.Errorf("%w: the server queued the prompt but named neither an item "+
			"nor a pending input, so there is nothing to attribute a reply against",
			ErrTurnFailed)
	}
	d.log.Info("prompt sent", "session_id", sessionID, "anchor", t.anchor,
		"pending", accepted.ItemID == "")
	return nil
}

// sendOnSubscribe is the send arm for a session whose host is already running:
// the prompt goes in as soon as the stream is live. [Driver.sendWhenLaunched] is
// the other arm.
func (d *Driver) sendOnSubscribe(
	client *omnigent.Client,
	prompt string,
	t *turn,
) func(context.Context, omnigent.Subscription) error {
	return func(ctx context.Context, sub omnigent.Subscription) error {
		return d.sendPrompt(ctx, client, sub.SessionID, prompt, t)
	}
}

// sendWhenLaunched is the send arm for a session this run created: the prompt
// waits until its sandbox reports ready, and is abandoned when the launch fails.
// [Driver.sendOnSubscribe] is the other arm.
//
// Only a created session reaches here with an unsent prompt, so an adopted
// session's late stage event finds the anchor already set and does nothing.
// Idempotent on the anchor for that reason, and because nothing promises the
// pipeline reports ready exactly once.
//
// A failed launch is reported rather than waited out. The stage carries the
// reason — a spend limit, a clone that could not authenticate — where an expiring
// run deadline carries none.
func (d *Driver) sendWhenLaunched(
	ctx context.Context,
	client *omnigent.Client,
	sessionID, prompt string,
	t *turn,
	e omnigent.SessionSandboxStatusEvent,
) error {
	if t.anchor != "" {
		return nil
	}
	switch e.Stage {
	case omnigent.SessionSandboxStatusEventStageReady:
		d.log.Info("sandbox ready; sending the prompt", "session_id", sessionID)
		return d.sendPrompt(ctx, client, sessionID, prompt, t)

	case omnigent.SessionSandboxStatusEventStageFailed:
		reason := "no reason given"
		if e.Error != nil && *e.Error != "" {
			reason = *e.Error
		}
		return fmt.Errorf("%w: the sandbox never launched: %s", ErrTurnFailed, reason)

	default:
		// provisioning, cloning, starting, connecting: progress, not an outcome.
		d.log.Info("sandbox launching", "session_id", sessionID, "stage", string(e.Stage))
		return nil
	}
}

// replyFor resolves what the observed turn produced.
//
// The order of these arms is their precedence, and every one of them is
// deliberate. A turn that ended outranks everything, including a clock that has
// since expired: its reply is already committed, [Driver.fetchReply] reads on a
// detached context, and discarding a finished review because the deadline landed
// in the window between the turn ending and this read throws away a whole paid-for
// review for nothing. A recorded fault outranks the clock for the same kind of
// reason in reverse — an expired clock is usually the consequence of the fault, so
// a run that stalls on an unanswered permission prompt and then hits its deadline
// should report the prompt.
func (d *Driver) replyFor(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
	t *turn,
	prior map[string]bool,
) (Reply, error) {
	switch {
	case t.id != "":
		return d.fetchReply(ctx, client, sessionID, t.id, prior)
	case t.failure != nil:
		return d.salvageFailedTurn(ctx, client, sessionID, t, prior)
	case ctx.Err() != nil:
		return Reply{}, ctx.Err()
	default:
		return Reply{}, fmt.Errorf(
			"the stream ended before the turn did (boundary crossed: %t)", t.crossed)
	}
}

// salvageFailedTurn recovers a review from a turn the server reported as failed.
//
// Worth attempting because the server publishes a failed edge on any lost
// transport, whatever the turn was actually doing, so a review that finished and
// then met a network blip arrives here. The reply is already committed by then, and
// throwing it away costs a whole re-review.
//
// Fails closed in all three directions: the failed edge must have named a response
// id, a reply must be attributable to that id, and that reply must carry a full
// reply. Anything short of all three reports the failure the server sent, which
// is why a partial review cannot be published as a complete one — the closing
// block is the agent's own statement that it finished.
func (d *Driver) salvageFailedTurn(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
	t *turn,
	prior map[string]bool,
) (Reply, error) {
	if t.failedTurnID == "" {
		return Reply{}, t.failure
	}
	reply, err := d.fetchReply(ctx, client, sessionID, t.failedTurnID, prior)
	if err != nil || !t.complete(reply.Text) {
		return Reply{}, t.failure
	}
	d.log.Warn("recovered a complete verdict from a turn the server reported as failed",
		"session_id", sessionID, "turn_id", t.failedTurnID, "error", t.failure)
	return reply, nil
}

// recoverFromStreamLoss tries to rescue a review whose stream died under it.
//
// A dropped stream is routine and recoverable by snapshot: the server persists an
// item before publishing it, so anything the stream would have carried is already
// readable. Without this a run discards a review that may be complete and paid
// for, and [Driver.salvageFailedTurn] cannot help — it keys on a failure edge a
// transport drop never produces.
//
// Fails closed three ways: only a genuine stream fault is recovered; the prompt
// must have been echoed, or the run has nothing of its own to find; and exactly
// one new reply group must exist and be complete, so an ambiguous or unfinished
// session reports the transport error instead.
func (d *Driver) recoverFromStreamLoss(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
	t *turn,
	cause error,
) (Reply, error) {
	if !errors.Is(cause, omnigent.ErrStreamInterrupted) && !errors.Is(cause, omnigent.ErrStreamIdle) {
		return Reply{}, cause
	}
	if !t.crossed {
		// Nothing to salvage, but the two ways of getting here have different
		// causes and the operator should not have to tell them apart from a stream
		// error. An unsent prompt means the sandbox never reported itself ready, so
		// the wait ran out; a sent one means the turn was lost before its prompt
		// was persisted.
		if t.anchor == "" {
			d.log.Error("the sandbox never reported ready, so the prompt was never sent",
				"session_id", sessionID, "error", cause)
		} else {
			d.log.Error("the stream died before the prompt was persisted",
				"session_id", sessionID, "anchor_item_id", t.anchor, "error", cause)
		}
		return Reply{}, cause
	}

	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.cfg.RequestTimeout)
	defer cancel()

	session, err := client.GetSession(readCtx, sessionID, omnigent.GetSessionOptions{
		IncludeItems: omnigent.Ptr(true),
	})
	if err != nil {
		// The stream error is returned, since it is what the caller acts on, but this
		// one is logged rather than dropped: a snapshot that reads cleanly while the
		// stream will not open says the fault is scoped to the stream path, and one
		// that fails the same way says nothing is getting through. Returning only the
		// stream's error leaves both looking identical.
		d.log.Warn("could not read the session to salvage a reply from it",
			"session_id", sessionID, "error", err, "stream_error", cause)
		return Reply{}, cause
	}

	// A turn still in flight is not something to salvage from. The stream is a
	// snapshot and a live tail, so its ending says nothing about the work, and a
	// session naming an active response says the work continues.
	if session.ActiveResponseID != nil {
		return Reply{}, cause
	}

	groups := ReplyGroupsSince(session.Items, t.prior)
	if len(groups) != 1 {
		// Two replies cannot be told apart, and waiting will not separate them.
		// None means the turn has committed nothing yet, which waiting still can.
		t.turnSettled = len(groups) > 1
		d.log.Warn("stream died and the session does not name one new reply",
			"session_id", sessionID, "reply_groups", groups, "error", cause)
		return Reply{}, cause
	}

	// The group above was found by asking which ids are new, which is the negative
	// filter doc.go forbids. Requiring the reply to sit after this turn's prompt is
	// the positive half that filter cannot carry.
	if !GroupIsAfterAnchor(session.Items, t.anchorItem, groups[0]) {
		d.log.Warn("stream died and the new reply does not sit after this turn's prompt",
			"session_id", sessionID, "anchor_item_id", t.anchorItem,
			"response_id", groups[0], "error", cause)
		return Reply{}, cause
	}

	reply, err := d.fetchReply(ctx, client, sessionID, groups[0], t.prior)
	if err != nil {
		return Reply{}, cause
	}

	// The prompt requires a closing verdict block, so a reply without one is the
	// agent mid-answer rather than an answer. That is the only reliable reading
	// here: the session reports itself idle between tool calls, so neither its
	// status nor its absent active response distinguishes the two.
	if !t.complete(reply.Text) {
		d.log.Warn("the session reads idle but its reply is not a review; rejoining",
			"session_id", sessionID, "turn_id", groups[0],
			"chars", len(reply.Text), "reason", reply.Reason)
		return Reply{}, cause
	}

	t.turnSettled = true
	d.log.Warn("recovered a complete verdict from a session whose stream died",
		"session_id", sessionID, "turn_id", groups[0], "error", cause)
	return reply, nil
}
