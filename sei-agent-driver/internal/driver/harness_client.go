package driver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
	"golang.org/x/net/http2"
)

// Everything that talks to omnigent: building the client, finding an agent and a
// session, reading a reply back, and answering a permission prompt.
//
// Kept apart from the orchestration so the two read separately. What a run does is
// a sequence of named steps in driver.go; how each step reaches the server is here,
// including the parts that exist only because of how this server behaves — paging
// a session list to find a label, a stream that must be answered on the session it
// arrived from, a reply read detached from an expired run.
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
		minted, ttl, err := MintToken(ctx, &http.Client{Timeout: d.cfg.RequestTimeout},
			d.cfg.BaseURL, d.cfg.MachineClientID, d.cfg.MachineClientSecret)
		if err != nil {
			return nil, err
		}
		d.log.Info("minted a machine token",
			"client_id", d.cfg.MachineClientID, "token_ttl", ttl)

		// Minted once and never re-minted, so a deadline past the token's life
		// spends its tail on 401s: the review is still running, every call is
		// rejected, and nothing in that failure names the token as the cause.
		if ttl > 0 && ttl < d.cfg.RunDeadline {
			d.log.Warn("the run deadline outlives the minted token, so a long turn will start failing as unauthorised",
				"token_ttl", ttl, "run_deadline", d.cfg.RunDeadline)
		}
		token = minted
	}

	httpClient, err := healthCheckedClient(d.log)
	if err != nil {
		return nil, err
	}

	return omnigent.New(d.cfg.BaseURL,
		omnigent.WithHTTPClient(httpClient),
		omnigent.WithBearerToken(token),
		omnigent.WithAuthHeader("Origin", d.cfg.Origin),
		omnigent.WithUserAgent("seidroid-xreview"),
		omnigent.WithStreamIdleTimeout(d.cfg.StreamIdleTimeout),
		omnigent.WithUnaryTimeout(d.cfg.UnaryTimeout),
	)
}

// healthCheckedClient returns a client whose HTTP/2 connections answer for
// themselves.
//
// Without this, nothing tells the transport a connection has stopped carrying
// traffic. A middlebox that drops a flow without a reset leaves the socket
// ESTABLISHED, the connection stays a reuse candidate, and every request handed to
// it is written into a socket nothing is reading. Recovery then waits on the
// kernel's retransmit ceiling, which is minutes.
//
// ReadIdleTimeout makes the transport send a PING once a connection has been quiet;
// PingTimeout drops the connection when the PING is not answered, so the next
// request dials a new one. The idle bound sits above the server's 15-second stream
// heartbeat, so a healthy stream resets the timer and pings only when frames have
// genuinely stopped.
func healthCheckedClient(log *slog.Logger) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// The header bound is the only timeout a streaming request can carry, since a
	// stream's body is unbounded by design.
	transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout

	if _, err := configureHealthChecks(transport); err != nil {
		return nil, err
	}

	return &http.Client{Transport: &tracingTransport{base: transport, log: log}}, nil
}

// configureHealthChecks enables HTTP/2 keepalive pings on transport and returns the
// HTTP/2 transport it configured.
//
// Separate from its caller, and returning what it set, because http2's configure
// call refuses a transport it has already enabled — so a test cannot re-derive these
// settings from a finished client, only from doing the configuring itself.
func configureHealthChecks(transport *http.Transport) (*http2.Transport, error) {
	h2, err := http2.ConfigureTransports(transport)
	if err != nil {
		return nil, fmt.Errorf("%w: configuring http/2 health checks: %w", ErrConfig, err)
	}
	h2.ReadIdleTimeout = http2ReadIdleTimeout
	h2.PingTimeout = http2PingTimeout
	return h2, nil
}

// resolveAgent finds the agent the server knows by this name, paging the listing
// until it matches.
//
// It returns the whole agent rather than its id because the harness travels on
// it, and the harness decides which signal ends a turn.
func (d *Driver) resolveAgent(
	ctx context.Context, client *omnigent.Client, name string,
) (omnigent.AgentObject, error) {
	var opts omnigent.ListAgentsOptions
	for {
		page, err := client.ListAgents(ctx, opts)
		if err != nil {
			return omnigent.AgentObject{}, err
		}
		for _, agent := range page.Data {
			if agent.Name == name {
				return agent, nil
			}
		}
		if !page.HasMore || len(page.Data) == 0 {
			return omnigent.AgentObject{}, fmt.Errorf("%w: no agent named %q on this server",
				ErrConfig, name)
		}
		opts.After = page.LastID
	}
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
		// Logged rather than swallowed. Whether this read succeeds while the stream
		// will not open is what separates a stream that is wedged from a server that
		// is: one says the fault is scoped to the stream path, the other says nothing
		// is getting through. Returning a bare false discards the difference.
		d.log.Warn("could not read the session while deciding whether it is live",
			"session_id", sessionID, "error", err)
		return false
	}
	return session.RunnerOnline != nil && *session.RunnerOnline
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
		// says which is ours, so this refuses rather than choosing the newest —
		// which publishes another invocation's review as this one's.
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

// answerPending decides the prompts already parked on a session.
//
// Fatal on failure, both here and in [Driver.answer], and that is the fix for the
// most expensive fault this driver has produced. The permission hook blocks the
// agent synchronously while it waits, so a prompt this driver fails to answer
// stalls the review for the rest of the run: one recorded trace sat on an
// unanswered prompt for 9 minutes 39 seconds while the transport stayed perfectly
// healthy. So a prompt this driver cannot answer is fatal, here and in
// [Driver.answer], rather than logged and carried past.
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
