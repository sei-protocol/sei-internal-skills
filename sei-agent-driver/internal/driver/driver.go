package driver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
)

// RunKeyLabel carries a workload's run key on the session, so a later dispatch
// recognises a session an earlier one created. Namespaced because labels are a
// shared surface.
//
// The value still says xreview because it is written on live sessions: changing
// it orphans every session a running deployment would otherwise adopt. One
// workload shares this key today. A second one needs a workload discriminator
// beside it, or their run keys collide in one namespace.
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
		// Through classify like every other failure. Hardcoding ExitConfig here
		// mislabelled a token exchange that failed on the network as a
		// configuration fault, and the exit code is the caller's contract: it
		// decides whether to tell an operator to fix a secret or to retry.
		return d.classify(Result{ExitCode: ExitOK, TeardownOK: true}, err), err
	}

	result := d.review(ctx, client, w, runKey)
	d.log.Info("run finished",
		"run_key", runKey, "session_id", result.SessionID,
		"exit_code", result.ExitCode, "teardown_ok", result.TeardownOK)
	return result, nil
}

// newClient mints a token when configured to, then builds the SDK client.
//
// Origin is sent on every request because the server gates state-changing POSTs
// behind a trusted-origin check and this caller is not a browser. It rides
// WithAuthHeader, which is a general header setter despite the name — and the
// consequence of that naming is deliberate here: the SDK treats what it sets as
// credential-bearing and so refuses to carry it across an unsafe redirect, which
// is the behaviour this header wants anyway.
func (d *Driver) newClient(ctx context.Context) (*omnigent.Client, error) {
	token := d.cfg.Token
	if d.cfg.MintsOwnToken() {
		minted, err := MintToken(ctx, &http.Client{Timeout: d.cfg.RequestTimeout},
			d.cfg.BaseURL, d.cfg.MachineClientID, d.cfg.MachineClientSecret)
		if err != nil {
			return nil, err
		}
		d.log.Info("minted a machine token", "client_id", d.cfg.MachineClientID)
		token = minted
	}

	return omnigent.New(d.cfg.BaseURL,
		omnigent.WithBearerToken(token),
		omnigent.WithAuthHeader("Origin", d.cfg.Origin),
		omnigent.WithUserAgent("seidroid-xreview"),
		omnigent.WithStreamIdleTimeout(d.cfg.StreamIdleTimeout),
	)
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

	agentID, err := d.resolveAgent(ctx, client)
	if err != nil {
		return d.classify(result, err)
	}
	d.log.Info("resolved agent", "agent", d.cfg.Agent, "agent_id", agentID)

	session, adopted, err := d.createOrAdopt(ctx, client, agentID, runKey, w)
	if err != nil {
		return d.classify(result, err)
	}
	result.SessionID = session.ID
	d.log.Info("session ready", "session_id", session.ID,
		"continued", adopted.continued, "live", adopted.live)

	// The response ids already on the session, captured before the turn so its own
	// reply can be told apart from the history a reused session carries.
	prior, err := d.priorResponseIDs(ctx, client, session.ID)
	if err != nil {
		return d.classify(result, err)
	}

	reply, err := d.driveTurn(ctx, client, session.ID, w, adopted, prior)
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
		return d.classify(result, err)
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
func (d *Driver) classify(result Result, err error) Result {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
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

// resolveAgent turns the configured agent name into its id.
//
// There is no lookup-by-name route, so this pages the listing until the name
// matches. It pages rather than reading one page because the deployment's agent
// count is not this driver's to assume.
func (d *Driver) resolveAgent(ctx context.Context, client *omnigent.Client) (string, error) {
	var opts omnigent.ListAgentsOptions
	for {
		page, err := client.ListAgents(ctx, opts)
		if err != nil {
			return "", err
		}
		for _, agent := range page.Data {
			if agent.Name == d.cfg.Agent {
				return agent.ID, nil
			}
		}
		if !page.HasMore || len(page.Data) == 0 {
			return "", fmt.Errorf("%w: no agent named %q on this server",
				ErrConfig, d.cfg.Agent)
		}
		opts.After = page.LastID
	}
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
// rest of the run actually asks. They were one boolean until a session turned up
// that was continued but not live, and answering both from that one bit sent the
// prompt into a sandbox that did not exist.
type adoption struct {
	// continued reports that this session already holds a review of this pull
	// request, which decides which prompt it gets.
	continued bool

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
			return existing, adoption{continued: true, live: live}, nil
		}
		d.log.Warn("the session for this pull request cannot run a turn; replacing it",
			"run_key", runKey, "session_id", existing.ID)
		if _, err := client.DeleteSession(ctx, existing.ID, omnigent.DeleteSessionOptions{}); err != nil {
			return nil, adoption{}, fmt.Errorf(
				"the session for this pull request cannot run a turn and could not be "+
					"deleted, so a new one would collide with it: %w", err)
		}
	}

	session, err := client.CreateSession(ctx, omnigent.SessionCreateRequest{
		AgentID:  agentID,
		HostType: "managed",
		Title:    w.Title(),
		Labels:   map[string]string{RunKeyLabel: runKey},
	})
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
	return committed, adoption{continued: true, live: live}, nil
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

// findByRunKey walks the agent's sessions for one carrying this run key.
func (d *Driver) findByRunKey(
	ctx context.Context,
	client *omnigent.Client,
	agentID, runKey string,
) (*omnigent.SessionResponse, error) {
	opts := omnigent.ListSessionsOptions{AgentID: agentID}
	for {
		page, err := client.ListSessions(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Data {
			if item.Labels[RunKeyLabel] == runKey {
				return client.GetSession(ctx, item.ID, omnigent.GetSessionOptions{
					IncludeItems: omnigent.Ptr(true),
				})
			}
		}
		if !page.HasMore || len(page.Data) == 0 {
			return nil, nil
		}
		opts.After = page.LastID
	}
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
		return Result{ExitCode: ExitConfig}, err
	}
	agentID, err := d.resolveAgent(ctx, client)
	if err != nil {
		return d.classify(Result{ExitCode: ExitOK}, err), err
	}
	session, err := d.findByRunKey(ctx, client, agentID, runKey)
	if err != nil {
		return d.classify(Result{ExitCode: ExitOK}, err), err
	}
	if session == nil {
		d.log.Info("no session for this pull request; nothing to close", "run_key", runKey)
		return Result{ExitCode: ExitOK, TeardownOK: true}, nil
	}
	if _, err := client.DeleteSession(ctx, session.ID, omnigent.DeleteSessionOptions{}); err != nil {
		d.log.Error("could not delete the session; the sandbox will leak until reclaimed",
			"session_id", session.ID, "error", err)
		return Result{ExitCode: ExitTeardownLeak, SessionID: session.ID}, nil
	}
	d.log.Info("session deleted", "session_id", session.ID)
	return Result{ExitCode: ExitOK, SessionID: session.ID, TeardownOK: true}, nil
}

// turn is the state of one prompt-and-answer exchange.
//
// One value, constructed once and never field-reset. A run drives exactly one
// turn, so there is no reset path for an implementer to forget to advance.
type turn struct {
	// anchor is our own prompt's item id, as the server assigned it.
	//
	// Set inside the subscription hook, which the SDK documents as running on the
	// caller's goroutine before the first event reaches it. So it is in place
	// before anything can be attributed, and needs no synchronisation.
	anchor string

	// crossed reports that the server echoed the anchor back.
	//
	// Until it does, everything on the stream is history or another actor's work.
	// The stream opens with a prologue that replays earlier items, and on a
	// recorded trace that prologue carried a previous invocation's completed
	// assistant message 31 milliseconds before our own prompt was persisted. No
	// check on an item's content distinguishes that message from a real reply;
	// only its position does.
	crossed bool

	// id is the turn's response id, taken from the edge that ended the turn.
	//
	// It is deliberately not learned earlier or from anywhere else. In particular
	// it is not read off our own prompt item: that item carries whichever response
	// was last active, which is measurably a stale id from before the boundary.
	id string

	// bareIdles counts idle edges that carried no response id. Logged, because it
	// is the one number that shows the next reader why "the first idle edge ends
	// the turn" is wrong — a recorded trace has five, one of them squarely
	// mid-work.
	bareIdles int

	// deltaChars counts streamed text. Logged, never published: on a recorded
	// trace the chunks arrive out of index order and land one chunk short of the
	// committed message, so reassembling them cannot produce a reply.
	deltaChars int

	// prior is every response id already on the session when this run started. An
	// id in it cannot belong to the turn answering our prompt, however well-timed
	// its edge looks.
	prior map[string]bool

	// staleIdles counts id-bearing idle edges rejected on that basis. A non-zero
	// count means another run ended a turn inside our window.
	staleIdles int

	answered map[string]bool

	// complete is the workload's test for a finished answer. Held here because
	// the salvage paths need it and they are four frames from the caller that
	// knows the workload.
	complete func(text string) bool
	seen     map[string]int

	// turnSettled records that waiting longer cannot change this turn's outcome,
	// which is what stops a reconnect from watching for edges that will not come.
	//
	// Deliberately not "the session named no active response". A claude-native
	// session goes idle mid-turn, so an idle snapshot with only the agent's
	// opening sentence behind it is a turn still being written, not a finished
	// one -- reading it as finished publishes a review the agent never wrote. It
	// is set when the reply carries a reply, and when two replies make
	// attribution impossible; both are outcomes waiting cannot improve. It stays
	// false while the agent may still be working.
	turnSettled bool

	// failure is the first fatal signal. Written once, so the cause a run reports
	// is the one that actually stopped it.
	failure error

	// failedTurnID is the response id a failed edge named, when it named one.
	// Deliberately not id: a failed turn did not end, and only
	// [Driver.salvageFailedTurn] may read a reply against this.
	failedTurnID string
}

func newTurn(prior map[string]bool, complete func(string) bool) *turn {
	return &turn{
		prior:    prior,
		complete: complete,
		answered: map[string]bool{},
		seen:     map[string]int{},
	}
}

func (t *turn) fail(err error) {
	if t.failure == nil {
		t.failure = err
	}
}

// ended reports whether there is nothing further to read.
func (t *turn) ended() bool { return t.id != "" || t.failure != nil }

// crossBoundary marks the point after which events can be this turn's.
func (t *turn) crossBoundary(e omnigent.SessionInputConsumedEvent) {
	// Either identifier, because the anchor is whichever the send returned. A
	// prompt persisted straight away is echoed by its item id; one parked as a
	// pending input is echoed by the pending id it drains, on the same event. The
	// item id is checked first because it is always populated, so a run holding a
	// pending anchor is not matched by another message's item.
	if e.Data.ItemID == t.anchor {
		t.crossed = true
		return
	}
	if e.Data.ClearedPendingID != nil && *e.Data.ClearedPendingID == t.anchor {
		t.crossed = true
	}
}

// observeStatus reads a coarse session status edge.
//
// An idle edge carrying a response id, after the boundary, is the end of the turn
// and the only thing that is. A bare idle edge is pane churn, so a missing
// response id downgrades the edge to noise rather than making it a wildcard.
func (t *turn) observeStatus(e omnigent.SessionStatusEvent) {
	if e.Status == omnigent.SessionStatusEventStatusFailed {
		if t.crossed && e.ResponseID != nil {
			t.failedTurnID = *e.ResponseID
		}
		t.fail(fmt.Errorf("%w: %s", ErrTurnFailed, statusDetail(e)))
		return
	}
	if e.Status != omnigent.SessionStatusEventStatusIdle || !t.crossed {
		return
	}
	if e.ResponseID == nil || *e.ResponseID == "" {
		t.bareIdles++
		return
	}
	if t.prior[*e.ResponseID] {
		// Already on the session before we sent our prompt, so it cannot be the
		// turn that answers it. This is the reachable half of the overlapping-run
		// hazard: a superseded run whose stop lost the race ends its turn inside
		// our window, and its edge is otherwise indistinguishable from ours.
		t.staleIdles++
		return
	}
	t.id = *e.ResponseID
}

// observeSuperseded ends the turn on a Claude /clear.
//
// The live terminal moves to a new conversation, so a verdict read here would land
// somewhere nothing is watching. Worse, the run key still points at the retired
// conversation, so every later review of this pull request adopts a dead session
// until someone intervenes — which is why this fails loudly rather than following
// the redirect.
func (t *turn) observeSuperseded(e omnigent.SessionSupersededEvent) {
	t.fail(fmt.Errorf("%w: the session was superseded; its conversation is now %s",
		ErrTurnFailed, e.TargetConversationID))
}

// statusDetail renders a failed edge's error, which is the reason to watch this
// event rather than infer failure from silence.
func statusDetail(e omnigent.SessionStatusEvent) string {
	if e.Error == nil {
		return "the session reported failure, with no detail"
	}
	return fmt.Sprintf("the session reported failure: %s (%s)", e.Error.Message, e.Error.Code)
}

// logTurnObserved records what the turn machine saw.
//
// Deferred by its caller so it survives the early returns: the stream-error paths
// are the timeout and transport-drop cases, where the workflow log is all an
// operator has.
func (d *Driver) logTurnObserved(sessionID string, t *turn) {
	d.log.Info("turn observed", "session_id", sessionID, "turn_id", t.id,
		"crossed_boundary", t.crossed, "bare_idle_edges", t.bareIdles,
		"stale_idle_edges", t.staleIdles, "failed_turn_id", t.failedTurnID,
		"delta_chars", t.deltaChars, "answered", len(t.answered),
		"event_types", t.seen)
}

// eventKey names an event by its wire type where the SDK does not model it, so a
// shape this build does not know cannot hide behind one Go type in the census.
func eventKey(ev omnigent.Event) string {
	if unknown, ok := ev.(omnigent.UnknownEvent); ok {
		return "unknown:" + unknown.Type
	}
	return fmt.Sprintf("%T", ev)
}

// driveTurn sends the prompt and reads the stream until the turn that answers it
// ends.
//
// The turn's end is the single signal this driver trusts: a session status edge
// reporting idle and carrying a response id, arriving after the server has echoed
// our own prompt back. Every other candidate was tried and is wrong on this
// harness. The response lifecycle's terminal event in particular is an injection
// acknowledgement — it arrives before our prompt has even been persisted, in an id
// namespace that can never match a conversation item — so treating it as a turn
// end ended the turn a second or two in, before the agent had done anything.
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
) (Reply, error) {
	t := newTurn(prior, w.Complete)

	defer d.logTurnObserved(sessionID, t)

	// A continued session already holds a review of this pull request, so it gets
	// the what-changed-since prompt, and prompts parked before this stream existed
	// are read from its snapshot rather than replayed onto it.
	prompt := w.Prompt()
	if from.continued {
		prompt = w.AdoptedPrompt()
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
	for attempt := 1; ; attempt++ {
		reply, err := d.consumeTurn(ctx, client, sessionID, prompt, t, prior, opts)

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
		if attempt >= resubscribeLimit {
			d.log.Error("the stream would not stay up long enough to finish the turn",
				"session_id", sessionID, "attempts", attempt,
				"prompt_sent", t.anchor != "", "error", err)
			return reply, err
		}

		// Before the prompt is in, the sandbox may have come up while the stream was
		// down, in which case the ready edge has already passed and waiting for it
		// again would hang. After it is in, the send hook must not fire a second
		// time and queue the prompt twice.
		if t.anchor == "" && d.sessionIsLive(ctx, client, sessionID) {
			opts.OnSubscribed = d.sendOnSubscribe(client, prompt, t)
		}

		// A stream is a snapshot and a live tail with no replay, so a turn that
		// ended while this one was down sent its last edge to nobody. Watching for
		// that edge again would wait out the whole deadline for something already
		// past. The salvage above has just asked the session which happened, so a
		// turn it reports as finished is resolved from what it committed rather
		// than rejoined.
		if t.anchor != "" && t.turnSettled {
			return reply, err
		}
		d.log.Info("stream ended before the turn did; re-subscribing",
			"session_id", sessionID, "attempt", attempt, "prompt_sent", t.anchor != "")
	}
}

// resubscribeLimit bounds how many times the stream is re-established while the
// prompt waits. The run deadline is the real bound; this stops a server that
// refuses to stream at all from spinning against it.
const resubscribeLimit = 10

// replyPreviewChars bounds how much of a reply reaches a log line.
//
// Long enough to tell a truncated review from a refusal and to carry the first
// finding, short enough that a full review does not land in the log twice over.
const replyPreviewChars = 300

// sessionIsLive reports whether a runner is registered for this session right
// now, and demands an explicit yes.
//
// Stricter than the reachability adoption uses, deliberately, because the two
// decisions fail in opposite directions. Adoption reads an unknown liveness as
// live so it never deletes a session on missing information; sending reads it as
// not-live so it never puts a prompt into a sandbox that does not exist, which is
// the failure this whole path is here to prevent. A read that errors answers no
// for the same reason.
func (d *Driver) sessionIsLive(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
) bool {
	readCtx, cancel := context.WithTimeout(ctx, d.cfg.RequestTimeout)
	defer cancel()

	session, err := client.GetSession(readCtx, sessionID, omnigent.GetSessionOptions{})
	if err != nil {
		return false
	}
	return session.RunnerOnline != nil && *session.RunnerOnline
}

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
		// A refusal and a control input both land here. The server distinguishes
		// them with denied/reason, which this cannot read until the SDK release
		// carrying those fields is picked up.
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

// fetchReply reads the turn's reply off the session.
//
// One read, and no poll. The reply commits before the edge that ends the turn —
// 4.4 and 4.5 seconds ahead of it on the two recorded traces that completed — so
// by the time this runs the item is already stored. An absent reply is reported
// rather than retried against, because retrying would only be guessing at an
// ordering the traces contradict.
//
// Its own bounded context, because the run's may be the thing that expired and
// this is the last chance to recover a verdict the agent did produce.
func (d *Driver) fetchReply(
	ctx context.Context,
	client *omnigent.Client,
	sessionID, turnID string,
	prior map[string]bool,
) (Reply, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.cfg.RequestTimeout)
	defer cancel()

	session, err := client.GetSession(ctx, sessionID, omnigent.GetSessionOptions{
		IncludeItems: omnigent.Ptr(true),
	})
	if err != nil {
		return Reply{}, fmt.Errorf("reading the session for a verdict: %w", err)
	}

	if groups := ReplyGroupsSince(session.Items, prior); len(groups) > 1 {
		// Two turns replied into this session while ours ran. Nothing on the wire
		// says which is ours, so this refuses: choosing the newest is exactly how
		// another invocation's review once got posted as this one's.
		d.log.Error("more than one turn replied into this session",
			"session_id", sessionID, "turn_id", turnID, "reply_groups", groups)
		return Reply{Reason: "another turn replied into this session while ours ran"}, nil
	}

	reply, ok := TurnReply(session.Items, turnID)
	if !ok {
		d.log.Warn("no reply carries this turn's response id",
			"session_id", sessionID, "turn_id", turnID, "items", len(session.Items))
		return Reply{Reason: "no assistant message carries this turn's response id"}, nil
	}

	// A minted bearer is deliberately not among the literals. It lives in a local
	// in newClient rather than on the config, and holding it on the driver to scan
	// for it would buy nothing: the sandbox never sees this process's bearer, so
	// the agent cannot quote it. What the agent does hold is its own gh
	// credentials, which the patterns cover.
	if shape := ScanSecrets(reply.Text, d.cfg.Token, d.cfg.MachineClientSecret); shape != "" {
		// Fail closed here rather than at the publish step: this is the last point
		// where the text is still inside the driver, and the pull requests it posts
		// to are public.
		d.log.Error("the reply carries something shaped like a credential; refusing to publish",
			"session_id", sessionID, "turn_id", turnID, "item_id", reply.ItemID, "shape", shape)
		return Reply{Reason: "the reply carries something shaped like a credential (" + shape + ")"}, nil
	}

	// The preview rides on every run, not only the failing ones: a reply is the
	// one thing here the logs cannot otherwise reconstruct, and a short one is
	// the first symptom of a turn that ended early. Safe to log because the
	// credential scan above has already returned on anything shaped like a
	// secret.
	d.log.Info("reply attributed", "session_id", sessionID, "turn_id", turnID,
		"item_id", reply.ItemID, "chars", len(reply.Text), "part_types", reply.PartTypes,
		"preview", clip(reply.Text, replyPreviewChars))

	reply.TurnID = turnID
	return reply, nil
}

// priorResponseIDs is every response id already on the session before this run's
// turn.
//
// Paged rather than read off the create-or-adopt snapshot. That snapshot returns
// the newest 100 items with nothing on the response marking the truncation, and
// one session per pull request is exactly the shape that outgrows it — so on a
// well-reviewed pull request the oldest ids fall out of view. An id missing from
// this set is one the turn machine would accept as its own.
func (d *Driver) priorResponseIDs(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
) (map[string]bool, error) {
	ids := map[string]bool{}
	opts := omnigent.SessionItemsOptions{Limit: 1000}
	for {
		page, err := client.ListSessionItems(ctx, sessionID, opts)
		if err != nil {
			return nil, fmt.Errorf("listing this session's items: %w", err)
		}
		for _, item := range page.Data {
			// A flat map rather than a typed item, which is why this reads one
			// string and nothing else: the guards that decide publishability run
			// on the typed snapshot, and all this needs is identity.
			if id, _ := item["response_id"].(string); id != "" {
				ids[id] = true
			}
		}
		if !page.HasMore || len(page.Data) == 0 {
			return ids, nil
		}
		opts.After = page.LastID
	}
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

// answerPending decides the prompts already parked on a session.
//
// Fatal on failure, both here and in [Driver.answer], and that is the fix for the
// most expensive fault this driver has produced. The permission hook blocks the
// agent synchronously while it waits, so a prompt this driver fails to answer
// stalls the review for the rest of the run: one recorded trace sat on an
// unanswered prompt for 9 minutes 39 seconds while the transport stayed perfectly
// healthy. The previous version logged the failure and carried on.
func (d *Driver) answerPending(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
	answered map[string]bool,
) error {
	session, err := client.GetSession(ctx, sessionID, omnigent.GetSessionOptions{})
	if err != nil {
		return fmt.Errorf("reading this session's parked prompts: %w", err)
	}
	for _, raw := range session.PendingElicitations {
		if err := d.answer(ctx, client, sessionID, ElicitationFromSnapshot(raw), answered); err != nil {
			return err
		}
	}
	return nil
}

// answer decides one prompt and sends the reply, once.
//
// Every decision is logged with the attested fields it turned on and the rule that
// fired, so an operator can see which policy name to allow rather than only that
// something was declined. The message and preview are logged too, but they are
// never what the decision reads.
func (d *Driver) answer(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
	e Elicitation,
	answered map[string]bool,
) error {
	if e.ID == "" || answered[e.ID] {
		return nil
	}
	action, reason := d.policy.Decide(e)
	d.log.Info("deciding a permission prompt",
		"session_id", sessionID, "elicitation_id", e.ID,
		"policy_name", e.PolicyName, "phase", e.Phase, "tool_name", e.ToolName,
		"action", action, "reason", reason, "preview", clip(e.ContentPreview, 200))

	target := e.ResolveSession(sessionID)
	if _, err := client.ResolveElicitation(ctx, target, e.ID,
		omnigent.ElicitationResult{Action: omnigent.ElicitationAction(action)}); err != nil {
		// Deliberately not ErrTurnFailed. The turn did not fail; we failed to
		// answer it, and reporting the agent's outcome for our own transport
		// fault sends an operator looking in the wrong place.
		return fmt.Errorf("answering prompt %s left the agent blocked: %w", e.ID, err)
	}
	answered[e.ID] = true
	return nil
}
