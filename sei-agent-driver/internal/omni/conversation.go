package omni

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// conversation is one session on a [Host], and the [driver.Conversation] a run
// drives.
type conversation struct {
	host   *Host
	client *omnigent.Client

	sessionID string

	// harness is the agent's runtime, which decides which signal ends a turn. See
	// [terminalBacked].
	harness string

	// from is where this session came from, which decides how the prompt goes in.
	from adoption

	// items is the snapshot Open read, kept because whether this conversation has
	// already been answered in is a question only the caller's predicate can settle,
	// and that arrives with the [driver.Ask].
	items []omnigent.ConversationItem

	// prior is every response id on the session before this run's turn, so the
	// turn's own reply can be told from the history a reused session carries.
	prior map[string]bool
}

// SessionID implements [driver.Conversation].
func (c *conversation) SessionID() string { return c.sessionID }

// connectionOpenLimit bounds how many times the stream is re-established. Set well
// above what a long turn needs, since a turn outlives the roughly three-minute
// connection cap a handful of times; it catches only a server that will not stream
// at all. The run deadline is the real bound.
const connectionOpenLimit = 40

// Turn implements [driver.Conversation].
//
// Two server-attested facts bound a turn and everything else is inference. The
// boundary is the item id posting the prompt returns, echoed back as
// session.input.consumed: nothing at or before it is publishable, and without that
// line a previous invocation's completed reply is indistinguishable from a fresh
// one, because the stream opens by replaying earlier work.
//
// The end depends on the harness. On a terminal-backed one it is a session.status
// edge reporting idle and carrying a response id, which the forwarder derives from
// Claude Code's Stop hook. A bare idle is not usable: a session emits several, and
// one of them lands mid-work. On an in-process harness that id-bearing
// edge never comes: the server documents response_id as "None for ordinary
// in-process runtime edges", so the response lifecycle is the end there. Waiting
// for the edge on that harness made the predicate unsatisfiable by specification —
// a codex scout produced a complete report and the wait ran its full budget and
// discarded it.
//
// The prompt goes in from the subscription hook on a session that can take one now,
// and from the sandbox-ready edge on one this run created. Either way it goes in
// after the stream is up: the server buffers nothing, so a turn started before the
// subscription is live publishes its first events to nobody.
func (c *conversation) Turn(ctx context.Context, ask driver.Ask) (driver.Reply, error) {
	answered := holdsAnswer(c.items, ask.Done)
	t := newTurn(c.prior, ask.Done, terminalBacked(c.harness))
	prompt := ask.Prompt(answered)
	c.host.log.Info("turn starting", "session_id", c.sessionID,
		"answered", answered, "harness", c.harness, "prompt_chars", len(prompt))

	// Whether the session can take a prompt now decides when it goes in, and only a
	// session this run created cannot. One still launching would accept the prompt
	// without queueing it, leaving no anchor, so it waits for the ready edge. A live
	// session takes it as soon as the stream is up, and so does a dormant one that
	// can be woken -- sending is what wakes it, and its launch pipeline does not
	// re-fire for a host that is already provisioned, so waiting there waits for
	// something only a send would cause. See [conversation.sendOnSubscribe] and
	// [conversation.sendWhenLaunched], the two arms of this.
	//
	// The revivable half is reasoned from the documented wake behaviour and is not
	// covered by a test: the fake server answers liveness from a queue, so a fixture
	// cannot hold a host dormant across the reads this path makes, and every attempt
	// passed with the condition removed. Treat it as unproven until a dormant
	// adoption is observed against a real deployment.
	opts := omnigent.StreamOptions{}
	if c.from.live || c.from.revivable {
		opts.OnSubscribed = c.sendOnSubscribe(prompt, t)
	}

	// A launching sandbox emits almost nothing and a quiet connection is dropped in
	// transit, so on a cold start the first stream usually dies before the sandbox
	// is ready. Waiting longer cannot help — the connection does not survive the
	// wait — so the stream is re-established while the prompt is still unsent.
	//
	// The budget is generous because a healthy long turn spends it: the connection
	// cap is roughly three minutes and a turn runs longer, so several reconnects are
	// success rather than failure.
	for opens := 1; ; opens++ {
		// Every iteration, not once before the loop. The stream replays nothing, so a
		// prompt raised while no stream was attached is never delivered -- and the
		// permission hook blocks the agent synchronously while it waits, so one this
		// run never answers stalls the turn for the rest of its budget with the
		// transport looking perfectly healthy. A reconnect is exactly when such a
		// prompt is sitting there unanswered.
		//
		// On the first pass only for a session this run did not open, which is the
		// one that may already be holding a prompt. After that on every pass, because
		// a reconnect is precisely when one has been raised with nobody listening.
		if c.from.continued || opens > 1 {
			if err := c.answerPending(ctx, t.answered); err != nil {
				return driver.Reply{}, err
			}
		}

		framesBefore := t.frames
		reply, err := c.consumeTurn(ctx, prompt, t, opts)
		carriedNothing := t.frames == framesBefore

		if t.ended() || ctx.Err() != nil || t.failure != nil {
			return reply, err
		}
		if opens >= connectionOpenLimit {
			c.host.log.Error("the stream would not stay up long enough to finish the turn",
				"session_id", c.sessionID, "opens", opens,
				"prompt_sent", t.anchor != "", "error", err)
			return reply, err
		}

		// Before the prompt is in, the sandbox may have come up while the stream was
		// down, in which case the ready edge has already passed and waiting for it
		// again would hang.
		if t.anchor == "" {
			if t.attempted {
				// A send went out and its acknowledgement never arrived, so whether the
				// server took the prompt is unknowable from here. Sending again would
				// put a second prompt to a runtime that may already be answering the
				// first, and this run has no anchor, so it could not attribute either
				// reply. Reporting it costs the run and nothing else.
				t.fail(fmt.Errorf(
					"%w: the prompt was sent without a usable acknowledgement, so it can "+
						"neither be sent again nor attributed", driver.ErrTurnFailed))
				return reply, t.failure
			}
			if c.host.sessionIsLive(ctx, c.client, c.sessionID) {
				opts.OnSubscribed = c.sendOnSubscribe(prompt, t)
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
		if t.turnSettled {
			return reply, err
		}
		// The error class separates a transport that ended from one that went quiet,
		// which is what tells an operator whether an intermediary is capping the
		// connection or the far end stopped talking. Logged per drop because the two
		// have different fixes and neither is visible from a timeout alone.
		c.host.log.Info("stream ended before the turn did; re-subscribing",
			"session_id", c.sessionID, "opens", opens,
			"prompt_sent", t.anchor != "", "frames", t.frames,
			"idle_timeout", errors.Is(err, omnigent.ErrStreamIdle),
			"interrupted", errors.Is(err, omnigent.ErrStreamInterrupted), "error", err)

		// Only an open that carried nothing waits. One that carried frames and then
		// died is the ordinary connection cap, and pausing there spends the deadline.
		//
		// This does not pace a server that accepts the subscription and immediately
		// ends it: its opening frame counts, so carriedNothing is false on exactly
		// the failure a pause would help with. Measured at forty opens in ten
		// milliseconds. Fixing it means sizing the pause against the open limit and
		// the run deadline together, which is a change to how long a run persists
		// against a wedged server rather than a local correction.
		if carriedNothing {
			c.backoff(ctx, opens)
		}
	}
}

// holdsAnswer reports whether a conversation already carries a reply done counts
// as finished.
//
// This is what [driver.Ask.Prompt]'s answered argument means, and it cannot be read
// off how the session was found. A session this run just created and then
// reconciled is one the run key finds while the agent has never answered in it; a
// run that expired before its first reply leaves the same thing for the next one to
// adopt. Both look continued and are not. The distinction is load-bearing because a
// second-pass prompt tells the agent it has already answered this work, so sending
// it into an empty conversation puts a follow-up question to an agent that was
// never asked the first one.
//
// The caller's own rule decides, rather than the presence of a message. An agent
// commits its opening sentence as a completed message before it has done anything,
// so "a reply exists" would count a run that died mid-answer as having answered.
// Both that and a turn still in flight read as no answer, which is the safe
// direction: the worst case is answering in full twice, against putting a follow-up
// question to an agent that never answered the first one.
func holdsAnswer(items []omnigent.ConversationItem, done func(string) bool) bool {
	for _, id := range replyGroupsSince(items, nil) {
		if reply, ok := turnReply(items, id); ok && done(reply.Text) {
			return true
		}
	}
	return false
}

// backoff waits before re-opening a stream that carried nothing, so a server
// coming back from a restart is not hammered while it does. Capped, because the
// run deadline is the real bound and a long sleep spends it without trying.
func (c *conversation) backoff(ctx context.Context, failedOpens int) {
	wait := min(time.Duration(failedOpens)*250*time.Millisecond, 5*time.Second)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// consumeTurn watches one subscription until the turn ends or the stream does.
func (c *conversation) consumeTurn(
	ctx context.Context,
	prompt string,
	t *turn,
	opts omnigent.StreamOptions,
) (driver.Reply, error) {
	for ev, err := range c.client.Stream(ctx, c.sessionID, opts) {
		if err != nil {
			return c.recoverFromStreamLoss(ctx, t, err)
		}
		t.frames++

		switch e := ev.(type) {
		case omnigent.SessionSandboxStatusEvent:
			if err := c.sendWhenLaunched(ctx, prompt, t, e); err != nil {
				t.fail(err)
			}

		case omnigent.SessionInputConsumedEvent:
			t.crossBoundary(e)

		case omnigent.ElicitationRequestEvent:
			if err := c.answer(ctx, elicitationFromEvent(e), t.answered); err != nil {
				t.fail(err)
			}

		case omnigent.SessionStatusEvent:
			t.observeStatus(e)

		case omnigent.ResponseCompletedEvent:
			t.observeResponseTerminal(e.Response.ID, "completed", nil)

		case omnigent.ResponseFailedEvent:
			t.observeResponseTerminal(e.Response.ID, "failed",
				fmt.Errorf("%w: the response failed", driver.ErrTurnFailed))

		case omnigent.ResponseCancelledEvent:
			t.observeResponseTerminal(e.Response.ID, "cancelled",
				fmt.Errorf("%w: the response was cancelled", driver.ErrTurnFailed))

		case omnigent.IncompleteEvent:
			// Terminal but truncated, so it is failed rather than ended: the text
			// behind it stops mid-sentence, and publishing one is worse than
			// reporting that nothing came back.
			t.observeResponseTerminal(e.Response.ID, "incomplete",
				fmt.Errorf("%w: the response ended incomplete", driver.ErrTurnFailed))

		case omnigent.SessionSupersededEvent:
			t.observeSuperseded(e)
		}

		if t.ended() {
			break
		}
	}

	return c.replyFor(ctx, t)
}

// sendPrompt posts the instruction and records the item id the server gave it.
//
// That id is the anchor, and it is the defence against a reused session's
// history: it correlates a send with its echo, and the echo marks where this
// invocation's work begins. An input accepted without queueing produces no
// anchor, so this refuses rather than working blind — streaming on would burn the
// run deadline before saying so.
func (c *conversation) sendPrompt(ctx context.Context, sessionID, prompt string, t *turn) error {
	// Before the call, so a send whose answer never arrives is still remembered as
	// attempted. See [turn.attempted].
	t.attempted = true

	accepted, err := c.client.Sessions().SendMessage(ctx, sessionID, prompt)
	if err != nil {
		return fmt.Errorf("sending the prompt: %w", err)
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
				driver.ErrTurnFailed, reason)
		}
		return fmt.Errorf("%w: the server did not queue the prompt, so this run has "+
			"no turn to wait for", driver.ErrTurnFailed)
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
			driver.ErrTurnFailed)
	}
	c.host.log.Info("prompt sent", "session_id", sessionID, "anchor", t.anchor,
		"pending", accepted.ItemID == "")
	return nil
}

// sendOnSubscribe is the send arm for a session whose host is already running:
// the prompt goes in as soon as the stream is live.
// [conversation.sendWhenLaunched] is the other arm.
func (c *conversation) sendOnSubscribe(
	prompt string,
	t *turn,
) func(context.Context, omnigent.Subscription) error {
	return func(ctx context.Context, sub omnigent.Subscription) error {
		return c.sendPrompt(ctx, sub.SessionID, prompt, t)
	}
}

// sendWhenLaunched is the send arm for a session this run created: the prompt
// waits until its sandbox reports ready, and is abandoned when the launch fails.
// [conversation.sendOnSubscribe] is the other arm.
//
// Only a created session reaches here with an unsent prompt, so an adopted
// session's late stage event finds the anchor already set and does nothing.
// Idempotent on the anchor for that reason, and because nothing promises the
// pipeline reports ready exactly once.
//
// A failed launch is reported rather than waited out. The stage carries the
// reason — a spend limit, a clone that could not authenticate — where an expiring
// run deadline carries none.
func (c *conversation) sendWhenLaunched(
	ctx context.Context,
	prompt string,
	t *turn,
	e omnigent.SessionSandboxStatusEvent,
) error {
	if t.anchor != "" {
		return nil
	}
	switch e.Stage {
	case omnigent.SessionSandboxStatusEventStageReady:
		c.host.log.Info("sandbox ready; sending the prompt", "session_id", c.sessionID)
		return c.sendPrompt(ctx, c.sessionID, prompt, t)

	case omnigent.SessionSandboxStatusEventStageFailed:
		reason := "no reason given"
		if e.Error != nil && *e.Error != "" {
			reason = *e.Error
		}
		return fmt.Errorf("%w: the sandbox never launched: %s", driver.ErrTurnFailed, reason)

	default:
		// provisioning, cloning, starting, connecting: progress, not an outcome.
		c.host.log.Info("sandbox launching", "session_id", c.sessionID, "stage", string(e.Stage))
		return nil
	}
}

// replyFor resolves what the observed turn produced.
//
// The order of these arms is their precedence, and every one of them is
// deliberate. A turn that ended outranks everything, including a clock that has
// since expired: its reply is already committed, [conversation.fetchReply] reads on
// a detached context, and discarding a finished answer because the deadline landed
// in the window between the turn ending and this read throws away a whole paid-for
// run for nothing. A recorded fault outranks the clock for the same kind of reason
// in reverse — an expired clock is usually the consequence of the fault, so a run
// that stalls on an unanswered permission prompt and then hits its deadline should
// report the prompt.
func (c *conversation) replyFor(ctx context.Context, t *turn) (driver.Reply, error) {
	switch {
	case t.id != "":
		return c.fetchReply(ctx, t.id, t.prior)
	case t.failure != nil:
		return c.salvageFailedTurn(ctx, t)
	case ctx.Err() != nil:
		return driver.Reply{}, ctx.Err()
	default:
		return driver.Reply{}, fmt.Errorf(
			"the stream ended before the turn did (boundary crossed: %t)", t.crossed)
	}
}

// salvageFailedTurn recovers an answer from a turn the server reported as failed.
//
// Worth attempting because the server publishes a failed edge on any lost
// transport, whatever the turn was actually doing, so a turn that finished and
// then met a network blip arrives here. The reply is already committed by then, and
// throwing it away costs a whole re-run.
//
// Fails closed in all three directions: the failed edge must have named a response
// id, a reply must be attributable to that id, and that reply must be a finished
// answer. Anything short of all three reports the failure the server sent, which is
// why a partial answer cannot be published as a complete one — the closing block is
// the agent's own statement that it finished.
func (c *conversation) salvageFailedTurn(ctx context.Context, t *turn) (driver.Reply, error) {
	if t.failedTurnID == "" {
		return driver.Reply{}, t.failure
	}
	reply, err := c.fetchReply(ctx, t.failedTurnID, t.prior)
	if err != nil || !t.complete(reply.Text) {
		return driver.Reply{}, t.failure
	}
	c.host.log.Warn("recovered a complete answer from a turn the server reported as failed",
		"session_id", c.sessionID, "turn_id", t.failedTurnID, "error", t.failure)
	return reply, nil
}

// recoverFromStreamLoss tries to rescue an answer whose stream died under it.
//
// A dropped stream is routine and recoverable by snapshot: the server persists an
// item before publishing it, so anything the stream would have carried is already
// readable. Without this a run discards an answer that may be complete and paid
// for, and [conversation.salvageFailedTurn] cannot help — it keys on a failure edge
// a transport drop never produces.
//
// Fails closed at every step: only a genuine stream fault is recovered; the prompt
// must have been echoed, or the run has nothing of its own to find; the session must
// name no active response; exactly one reply group must be new; that group must sit
// after this turn's prompt; and its reply must be a finished answer. Anything short of
// all six reports the transport error instead.
func (c *conversation) recoverFromStreamLoss(
	ctx context.Context,
	t *turn,
	cause error,
) (driver.Reply, error) {
	if !errors.Is(cause, omnigent.ErrStreamInterrupted) && !errors.Is(cause, omnigent.ErrStreamIdle) {
		return driver.Reply{}, cause
	}
	if !t.crossed {
		// Nothing to salvage, but the two ways of getting here have different
		// causes and the operator should not have to tell them apart from a stream
		// error. An unsent prompt means the sandbox never reported itself ready, so
		// the wait ran out; a sent one means the turn was lost before its prompt
		// was persisted.
		if t.anchor == "" {
			c.host.log.Error("the sandbox never reported ready, so the prompt was never sent",
				"session_id", c.sessionID, "error", cause)
		} else {
			c.host.log.Error("the stream died before the prompt was persisted",
				"session_id", c.sessionID, "anchor_item_id", t.anchor, "error", cause)
		}
		return driver.Reply{}, cause
	}

	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.host.cfg.RequestTimeout)
	defer cancel()

	session, err := c.client.Sessions().Get(readCtx, c.sessionID, omnigent.GetSessionOptions{
		IncludeItems: omnigent.Ptr(true),
	})
	if err != nil {
		// The stream error is returned, since it is what the caller acts on, but this
		// one is logged rather than dropped: a snapshot that reads cleanly while the
		// stream will not open says the fault is scoped to the stream path, and one
		// that fails the same way says nothing is getting through. Returning only the
		// stream's error leaves both looking identical.
		c.host.log.Warn("could not read the session to salvage a reply from it",
			"session_id", c.sessionID, "error", err, "stream_error", cause)
		return driver.Reply{}, cause
	}

	// A turn still in flight is not something to salvage from. The stream is a
	// snapshot and a live tail, so its ending says nothing about the work, and a
	// session naming an active response says the work continues.
	if session.ActiveResponseID != nil {
		return driver.Reply{}, cause
	}

	groups := replyGroupsSince(session.Items, t.prior)
	if len(groups) != 1 {
		// Two replies cannot be told apart, and waiting will not separate them.
		// None means the turn has committed nothing yet, which waiting still can.
		t.turnSettled = len(groups) > 1
		c.host.log.Warn("stream died and the session does not name one new reply",
			"session_id", c.sessionID, "reply_groups", groups, "error", cause)
		return driver.Reply{}, cause
	}

	// The group above was found by asking which ids are new, which is the negative
	// filter the package doc forbids. Requiring the reply to sit after this turn's
	// prompt is the positive half that filter cannot carry.
	if !groupIsAfterAnchor(session.Items, t.anchorItem, groups[0]) {
		c.host.log.Warn("stream died and the new reply does not sit after this turn's prompt",
			"session_id", c.sessionID, "anchor_item_id", t.anchorItem,
			"response_id", groups[0], "error", cause)
		return driver.Reply{}, cause
	}

	reply, err := c.fetchReply(ctx, groups[0], t.prior)
	if err != nil {
		return driver.Reply{}, cause
	}

	// The caller's rule requires a closing block, so a reply without one is the
	// agent mid-answer rather than an answer. That is the only reliable reading
	// here: the session reports itself idle between tool calls, so neither its
	// status nor its absent active response distinguishes the two.
	if !t.complete(reply.Text) {
		c.host.log.Warn("the session reads idle but its reply is unfinished; rejoining",
			"session_id", c.sessionID, "turn_id", groups[0],
			"chars", len(reply.Text), "reason", reply.Reason)
		return driver.Reply{}, cause
	}

	t.turnSettled = true
	c.host.log.Warn("recovered a complete answer from a session whose stream died",
		"session_id", c.sessionID, "turn_id", groups[0], "error", cause)
	return reply, nil
}

// fetchReply reads the turn's reply off the session.
//
// One read, and no poll. The reply commits well before the edge that ends the
// turn, so by the time this runs the item is already stored. An absent reply is
// reported rather than retried against, because retrying would be guessing at an
// ordering that does not hold.
//
// Its own bounded context, because the run's may be the thing that expired and
// this is the last chance to recover an answer the agent did produce.
func (c *conversation) fetchReply(
	ctx context.Context,
	turnID string,
	prior map[string]bool,
) (driver.Reply, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.host.cfg.RequestTimeout)
	defer cancel()

	session, err := c.client.Sessions().Get(ctx, c.sessionID, omnigent.GetSessionOptions{
		IncludeItems: omnigent.Ptr(true),
	})
	if err != nil {
		return driver.Reply{}, fmt.Errorf("reading the session for a reply: %w", err)
	}

	if groups := replyGroupsSince(session.Items, prior); len(groups) > 1 {
		// Two turns replied into this session while ours ran. Nothing on the wire
		// says which is ours, so this refuses rather than choosing the newest —
		// which publishes another invocation's answer as this one's.
		c.host.log.Error("more than one turn replied into this session",
			"session_id", c.sessionID, "turn_id", turnID, "reply_groups", groups)
		return driver.Reply{Reason: "another turn replied into this session while ours ran"}, nil
	}

	reply, ok := turnReply(session.Items, turnID)
	if !ok {
		c.host.log.Warn("no reply carries this turn's response id",
			"session_id", c.sessionID, "turn_id", turnID, "items", len(session.Items))
		return driver.Reply{Reason: "no assistant message carries this turn's response id"}, nil
	}

	// A minted bearer is deliberately not among the literals. It lives in a local
	// in newClient rather than on the config, and holding it here to scan for it
	// would buy nothing: the sandbox never sees this process's bearer, so the agent
	// cannot quote it. What the agent does hold is its own gh credentials, whose
	// literal form the patterns recognise.
	if shape := driver.ScanSecrets(reply.Text, c.host.cfg.Token, c.host.cfg.MachineClientSecret); shape != "" {
		// Fail closed here rather than at the publish step: this is the last point
		// where the text is still inside the driver, and the pull requests it posts
		// to are public.
		c.host.log.Error("the reply carries something shaped like a credential; refusing to publish",
			"session_id", c.sessionID, "turn_id", turnID, "item_id", reply.ItemID, "shape", shape)
		return driver.Reply{Reason: "the reply carries something shaped like a credential (" + shape + ")"}, nil
	}

	// The preview rides on every run, not only the failing ones. A reply is the one
	// thing here the logs cannot otherwise reconstruct, and a short one is the first
	// symptom of a turn that ended early, so a dropped stream and an agent that gave
	// up stay tellable apart afterwards.
	//
	// The scan above has returned on every shape it recognises, which is the most that
	// can be said for it: [driver.ScanSecrets] names the classes it cannot see, so this
	// is a residual risk accepted for the diagnostic rather than one the scan closes.
	c.host.log.Info("reply attributed", "session_id", c.sessionID, "turn_id", turnID,
		"item_id", reply.ItemID, "chars", len(reply.Text),
		"preview", clip(reply.Text, replyPreviewChars))

	reply.TurnID = turnID
	return reply, nil
}

// answerPending decides the prompts already parked on a session.
//
// Fatal on failure, both here and in [conversation.answer]. The permission hook
// blocks the agent synchronously while it waits, so a prompt this driver fails to
// answer stalls the run for the rest of its budget while the transport stays
// perfectly healthy — which is why it must not be logged and carried past.
func (c *conversation) answerPending(ctx context.Context, answered map[string]bool) error {
	session, err := c.client.Sessions().Get(ctx, c.sessionID, omnigent.GetSessionOptions{})
	if err != nil {
		return fmt.Errorf("reading this session's parked prompts: %w", err)
	}
	for _, raw := range session.PendingElicitations {
		if err := c.answer(ctx, elicitationFromSnapshot(raw), answered); err != nil {
			return err
		}
	}
	return nil
}

// answer decides one prompt and sends the reply, once.
//
// Every decision is logged with the attested fields it turned on and the rule that
// fired, so an operator can see which policy name to allow rather than only that
// something was declined. The preview goes with it, after a credential scan; the
// message does not, because it is neither clipped nor scanned. Neither is ever what
// the decision reads.
func (c *conversation) answer(
	ctx context.Context,
	e driver.Elicitation,
	answered map[string]bool,
) error {
	if e.ID == "" || answered[e.ID] {
		return nil
	}
	action, reason := c.host.policy.Decide(e)

	// The preview is the gated request's own payload -- for a shell prompt, the
	// command line, which is exactly where a credential appears. Scanned before it
	// is logged, on the same rule the reply path uses, because a workflow log on a
	// public repository is public.
	// Scanned whole and clipped after. Clipping first truncates a credential that
	// starts near the boundary to below the length its pattern requires, so the scan
	// misses it and the fragment is logged anyway.
	preview := clip(e.ContentPreview, elicitationPreviewChars)
	if shape := driver.ScanSecrets(e.ContentPreview, c.host.cfg.Token, c.host.cfg.MachineClientSecret); shape != "" {
		preview = "withheld: the preview carries something shaped like a credential (" + shape + ")"
	}
	c.host.log.Info("deciding a permission prompt",
		"session_id", c.sessionID, "elicitation_id", e.ID,
		"policy_name", e.PolicyName, "phase", e.Phase, "tool_name", e.ToolName,
		"target_session_id", e.ResolveSession(c.sessionID),
		"action", action, "reason", reason, "preview", preview)

	// Answered through the dedicated resolve URL, which is what upstream's own client
	// does for every verdict -- it does not branch on the elicitation's mode. The
	// route is absent from the vendored spec because the server registers it
	// include_in_schema=False, not because it is unsupported; both it and a
	// type:"approval" input reach the same server-side resolver.
	//
	// There is no answer to read. The route acks {"queued": false} on success and
	// carries no denial field, and neither does the approval branch of the events
	// route: the server's denied/reason shape belongs to a message input refused by
	// the input policy, which a verdict never passes through. So a non-2xx is the only
	// signal that the agent is still blocked.
	target := e.ResolveSession(c.sessionID)
	if err := c.client.Sessions().ResolveElicitation(ctx, target, e.ID,
		omnigent.ElicitationResult{Action: omnigent.ElicitationAction(action)}); err != nil {
		// Deliberately not ErrTurnFailed. The turn did not fail; we failed to
		// answer it, and reporting the agent's outcome for our own transport
		// fault sends an operator looking in the wrong place.
		return fmt.Errorf("answering prompt %s left the agent blocked: %w", e.ID, err)
	}
	answered[e.ID] = true
	return nil
}

// elicitationPreviewChars bounds how much of a gated request reaches a log line.
// Short, because the decision never reads it -- it is there so an operator can see
// what was approved, not so they can audit its contents.
const elicitationPreviewChars = 200

// replyPreviewChars bounds how much of a reply reaches a log line. Long enough to
// tell a truncated answer from a refusal and to carry the first finding, short
// enough that a full answer does not land in the log twice over.
const replyPreviewChars = 300

// clip bounds a value taken from model output before it reaches a log line,
// cutting on a rune boundary so the line stays valid UTF-8.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max] + "…"
}
