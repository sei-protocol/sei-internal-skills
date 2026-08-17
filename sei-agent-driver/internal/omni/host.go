package omni

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// managed is the host type every session is created with: a server-provisioned
// sandbox, which is the only kind this driver launches.
const managed = "managed"

// RunKeyLabel carries a unit of work's run key on the session, so a later dispatch
// recognises a session an earlier one created. Namespaced because labels are a
// shared surface.
//
// The value still says xreview because it is written on live sessions: changing
// it orphans every session a running deployment would otherwise adopt.
const RunKeyLabel = "xreview.seinetwork.io/run-key"

// Host is an Omnigent deployment, and the [driver.Host] this driver runs against.
type Host struct {
	cfg    driver.Config
	policy driver.Policy
	log    *slog.Logger
}

// New returns the host for a configuration. The logger receives one structured
// record per decision point — which session, which run key, which prompt was
// answered how, and how the turn ended — because those are the questions asked
// when a run misbehaves and nobody is watching.
func New(cfg driver.Config, policy driver.Policy, log *slog.Logger) *Host {
	return &Host{cfg: cfg, policy: policy, log: log}
}

// Open implements [driver.Host].
func (h *Host) Open(ctx context.Context, w driver.Work) (driver.Conversation, error) {
	client, err := h.newClient(ctx)
	if err != nil {
		return nil, err
	}

	agent, err := h.resolveAgent(ctx, client, w.Agent)
	if err != nil {
		return nil, err
	}
	harness := ""
	if agent.Harness != nil {
		harness = *agent.Harness
	}
	h.log.Info("resolved agent", "agent", w.Agent, "agent_id", agent.ID, "harness", harness)

	session, from, err := h.createOrAdopt(ctx, client, agent.ID, w)
	if err != nil {
		return nil, err
	}
	h.log.Info("session ready", "session_id", session.ID,
		"continued", from.continued, "live", from.live, "items", len(session.Items))

	// The response ids already on the session, captured before the turn so its own
	// reply can be told apart from the history a reused session carries.
	prior, err := h.priorResponseIDs(ctx, client, session.ID)
	if err != nil {
		return nil, err
	}

	return &conversation{
		host:      h,
		client:    client,
		sessionID: session.ID,
		harness:   harness,
		from:      from,
		items:     session.Items,
		prior:     prior,
	}, nil
}

// Close implements [driver.Host].
//
// Every session carrying the run key is deleted, not the first one found. Opening
// searches and then creates, which is not a lock: two dispatches that overlap can
// both find nothing and both create, and a close that stops at the first match
// leaves the other holding a sandbox that nothing will ever look for again.
func (h *Host) Close(ctx context.Context, w driver.Work) (string, error) {
	client, err := h.newClient(ctx)
	if err != nil {
		return "", err
	}
	agent, err := h.resolveAgent(ctx, client, w.Agent)
	if err != nil {
		return "", err
	}
	ids, err := h.findAllByRunKey(ctx, client, agent.ID, w.RunKey)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", nil
	}
	if len(ids) > 1 {
		h.log.Warn("more than one session carries this run key; deleting all of them",
			"run_key", w.RunKey, "session_ids", ids)
	}

	var held []string
	var cause error
	for _, id := range ids {
		_, err := client.DeleteSession(ctx, id, omnigent.DeleteSessionOptions{})
		switch {
		case err == nil, alreadyGone(err):
			// Already gone counts as reclaimed. Two closes racing, or a reopened and
			// re-closed unit of work, would otherwise report a leak that is not one --
			// and a false leak trains an operator to ignore the only signal there is.
		default:
			held = append(held, id)
			cause = err
		}
	}
	if len(held) > 0 {
		return held[0], fmt.Errorf("%w: %s: %w", driver.ErrLeaked, strings.Join(held, ", "), cause)
	}
	return ids[0], nil
}

// alreadyGone reports a delete that failed because there was nothing to delete.
func alreadyGone(err error) bool {
	var apiErr *omnigent.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// findAllByRunKey collects every session carrying this run key.
//
// [Host.findByRunKey] stops at the first, which is what opening wants: it needs the
// session's items, and one is enough to adopt. Closing wants all of them, because
// what it is reclaiming is compute rather than a conversation.
func (h *Host) findAllByRunKey(
	ctx context.Context,
	client *omnigent.Client,
	agentID, runKey string,
) ([]string, error) {
	var ids []string
	opts := omnigent.ListSessionsOptions{AgentID: agentID, Limit: 1000}
	for {
		page, err := client.ListSessions(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Data {
			if item.Labels[RunKeyLabel] == runKey {
				ids = append(ids, item.ID)
			}
		}
		if !page.HasMore || len(page.Data) == 0 {
			return ids, nil
		}
		opts.After = page.LastID
	}
}

// resolveAgent finds the agent the server knows by this name, paging the listing
// until it matches.
//
// It returns the whole agent rather than its id because the harness travels on
// it, and the harness decides which signal ends a turn.
func (h *Host) resolveAgent(
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
				driver.ErrConfig, name)
		}
		opts.After = page.LastID
	}
}

// findByRunKey walks the agent's sessions for one carrying this run key.
//
// Paged at the server's maximum rather than its default of 20. The server has no
// label filter, so this walk is linear in the agent's session count, and it runs on
// every open and every close -- including the close on a runner that is already
// being terminated. Sessions accumulate for as long as anything fails to reclaim
// one, so the cheap default is the expensive one over time.
func (h *Host) findByRunKey(
	ctx context.Context,
	client *omnigent.Client,
	agentID, runKey string,
) (*omnigent.SessionResponse, error) {
	opts := omnigent.ListSessionsOptions{AgentID: agentID, Limit: 1000}
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

// adoption is where a conversation's session came from, split into the two
// questions the rest of the run asks. They are separate because a session can be
// continued and not live, and answering both from one bit sends the prompt into a
// sandbox that does not exist.
type adoption struct {
	// continued reports that this session was found rather than opened here, so it
	// may be holding prompts parked before this stream existed.
	continued bool

	// live reports that a runner is registered right now, which decides whether
	// the prompt goes in on subscribe or waits for the launch pipeline.
	live bool
}

// createOrAdopt finds this work's session or opens one, and refuses to hand back a
// session that can never run a turn.
//
// Searching first is the idempotency guarantee. The run key is a label on the
// session, so it outlives the runner and a redelivered trigger finds the first
// run's session rather than doing the work twice. It walks every page because the
// server has no label filter and a page holds the agent's 20 newest. It is not a
// lock: two simultaneous runs can both find nothing and both create, which the
// caller's concurrency group prevents. This rules out the sequential duplicate.
//
// The refusal is the other half. A session whose sandbox never launched is stopped
// with its conversation intact, so the run key still finds it forever, and
// whether it can be revived is the provider's call: a resumable host wakes when
// sent a message, a non-resumable one is a dead end. Adopting the dead end makes
// every later run for that work fail the same way, so it is deleted and replaced
// instead.
func (h *Host) createOrAdopt(
	ctx context.Context,
	client *omnigent.Client,
	agentID string,
	w driver.Work,
) (*omnigent.SessionResponse, adoption, error) {
	existing, err := h.findByRunKey(ctx, client, agentID, w.RunKey)
	if err != nil {
		return nil, adoption{}, fmt.Errorf("looking for an existing session: %w", err)
	}
	if existing != nil {
		if live, revivable := reachability(existing); live || revivable {
			h.log.Info("adopting the session an earlier dispatch created",
				"run_key", w.RunKey, "session_id", existing.ID, "live", live)
			return existing, adoption{continued: true, live: live}, nil
		}
		h.log.Warn("the session for this work cannot run a turn; replacing it",
			"run_key", w.RunKey, "session_id", existing.ID)
		if _, err := client.DeleteSession(ctx, existing.ID, omnigent.DeleteSessionOptions{}); err != nil {
			return nil, adoption{}, fmt.Errorf(
				"the session for this work cannot run a turn and could not be "+
					"deleted, so a new one would collide with it: %w", err)
		}
	}

	create := omnigent.SessionCreateRequest{
		AgentID:  agentID,
		HostType: managed,
		Title:    w.Title,
		Labels:   map[string]string{RunKeyLabel: w.RunKey},
	}

	session, err := client.CreateSession(ctx, create)
	if err == nil {
		return session, adoption{}, nil
	}

	// A rejected argument means nothing was sent, so there is no session to
	// reconcile against and searching would only hide the real fault. Wrapped into
	// the driver's taxonomy so its exit code does not depend on the SDK's.
	if errors.Is(err, omnigent.ErrInvalidArgument) {
		return nil, adoption{}, fmt.Errorf("%w: %w", driver.ErrConfig, err)
	}

	// A second search, for a different case than the one above: create may have
	// committed server-side and lost its response, and this run must not retry
	// it. The SDK deliberately never retries a create for exactly that reason.
	h.log.Warn("create failed; looking for a session it may have committed",
		"run_key", w.RunKey, "error", err)
	committed, findErr := h.findByRunKey(ctx, client, agentID, w.RunKey)
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

// sessionState reports whether a runner is registered for this session right now,
// and whether the session is already working on a response.
//
// live demands an explicit yes.
//
// Stricter than the reachability adoption uses, deliberately, because the two
// decisions fail in opposite directions. Adoption reads an unknown liveness as
// live so it never deletes a session on missing information; sending reads it as
// not-live so it never puts a prompt into a sandbox that does not exist, which is
// the failure this whole path is here to prevent. A read that errors answers no
// for the same reason.
// busy is the other half, and it is what a caller whose send failed ambiguously
// needs: an active response means the server took a prompt, so sending again would
// put a second one to a runtime already answering the first.
func (h *Host) sessionState(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
) (live, busy bool) {
	readCtx, cancel := context.WithTimeout(ctx, h.cfg.RequestTimeout)
	defer cancel()

	session, err := client.GetSession(readCtx, sessionID, omnigent.GetSessionOptions{})
	if err != nil {
		// Logged rather than swallowed. Whether this read succeeds while the stream
		// will not open is what separates a stream that is wedged from a server that
		// is: one says the fault is scoped to the stream path, the other says nothing
		// is getting through. Returning a bare false discards the difference.
		h.log.Warn("could not read the session while deciding whether it is live",
			"session_id", sessionID, "error", err)
		return false, false
	}
	return session.RunnerOnline != nil && *session.RunnerOnline,
		session.ActiveResponseID != nil
}

// priorResponseIDs is every response id already on the session before this run's
// turn.
//
// Paged rather than read off the create-or-adopt snapshot. That snapshot returns
// the newest 100 items with nothing on the response marking the truncation, and
// one session per unit of work is exactly the shape that outgrows it — so on a
// well-reviewed pull request the oldest ids fall out of view. An id missing from
// this set is one the turn machine would accept as its own.
func (h *Host) priorResponseIDs(
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
