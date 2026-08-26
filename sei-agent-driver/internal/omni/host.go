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
		_, err := client.Sessions().Delete(ctx, id, omnigent.DeleteSessionOptions{})
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

// boundWalk bounds a whole paginated listing, not just the requests inside it.
//
// The SDK's iterator stops on the last page or on its own 10,000-page backstop. It
// does not stop on an empty page that still claims more, which the hand-written
// loops this replaced did. A server answering that shape turns one request into
// thousands, and the caller with no budget of its own is Close: it would spend the
// whole teardown window listing, then exit by deadline rather than by the leak path,
// so nothing names the sandbox it failed to reclaim.
func (h *Host) boundWalk(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, listingWalkBudget*h.cfg.RequestTimeout)
}

// listingWalkBudget multiplies [driver.Config.RequestTimeout] into a whole-listing
// budget. A listing this driver expects to fit in one or two pages gets room for a
// handful, and no more.
const listingWalkBudget = 4

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
	walkCtx, cancel := h.boundWalk(ctx)
	defer cancel()

	var ids []string
	opts := omnigent.ListSessionsOptions{AgentID: agentID, Limit: 1000}
	for session, err := range client.Sessions().List(walkCtx, opts) {
		if err != nil {
			return nil, err
		}
		if session.Labels[RunKeyLabel] == runKey {
			ids = append(ids, session.ID)
		}
	}
	return ids, nil
}

// resolveAgent finds the agent the server knows by this name.
//
// It returns the whole agent rather than its id because the harness travels on
// it, and the harness decides which signal ends a turn.
//
// A miss crosses as [driver.ErrConfig], because a name no deployment registers is
// a configuration fault rather than a transport one, and the exit code a run
// reports turns on that. The server's own miss message is not wrapped: it
// enumerates the agent names it did see, and those are an operator's to look up
// rather than a run's to print.
func (h *Host) resolveAgent(
	ctx context.Context, client *omnigent.Client, name string,
) (omnigent.AgentObject, error) {
	agent, err := client.Sessions().ResolveAgent(ctx, name)
	switch {
	case errors.Is(err, omnigent.ErrNotFound):
		return omnigent.AgentObject{}, fmt.Errorf("%w: no agent named %q on this server",
			driver.ErrConfig, name)
	case err != nil:
		return omnigent.AgentObject{}, err
	}
	return *agent, nil
}

// findByRunKey walks the agent's sessions for one carrying this run key.
//
// Paged at the server's maximum rather than its default of 20. The server has no
// label filter, so this walk is linear in the agent's session count, and it runs on
// every open, and [Host.findAllByRunKey] walks the same listing on every close --
// including the close on a runner that is already
// being terminated. Sessions accumulate for as long as anything fails to reclaim
// one, so the cheap default is the expensive one over time.
func (h *Host) findByRunKey(
	ctx context.Context,
	client *omnigent.Client,
	agentID, runKey string,
) (*omnigent.SessionResponse, error) {
	walkCtx, cancel := h.boundWalk(ctx)
	defer cancel()

	opts := omnigent.ListSessionsOptions{AgentID: agentID, Limit: 1000}
	for session, err := range client.Sessions().List(walkCtx, opts) {
		if err != nil {
			return nil, err
		}
		if session.Labels[RunKeyLabel] == runKey {
			// Deliberately ctx, not walkCtx: this is the fetch the walk existed to
			// reach, and it should not inherit what the search already spent.
			return client.Sessions().Get(ctx, session.ID, omnigent.GetSessionOptions{
				IncludeItems: omnigent.Ptr(true),
			})
		}
	}
	return nil, nil
}

// adoption is where a conversation's session came from, split into the two
// questions the rest of the run asks. They are separate because a session can be
// continued and not live, and answering both from one bit sends the prompt into a
// sandbox that does not exist.
type adoption struct {
	// continued reports that this session was found rather than opened here, so it
	// may be holding prompts parked before this stream existed.
	continued bool

	// live reports that a runner is registered right now.
	live bool

	// revivable reports a session whose host is dormant but can be woken. It takes
	// the prompt on subscribe like a live one, because sending is what wakes it --
	// waiting for a launch that only a send would trigger is a run that never asks
	// its question and spends its whole budget not asking.
	revivable bool
}

// createOrAdopt finds this work's session or opens one, and refuses to hand back a
// session that can never run a turn.
//
// Searching first is the idempotency guarantee. The run key is a label on the
// session, so it outlives the runner and a redelivered trigger finds the first
// run's session rather than doing the work twice. It walks every page because the
// server has no label filter. It is not a lock: two simultaneous runs can both find
// nothing and both create, which the caller's concurrency group prevents. This rules
// out the sequential duplicate.
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
				"run_key", w.RunKey, "session_id", existing.ID,
				"live", live, "revivable", revivable)
			return existing, adoption{continued: true, live: live, revivable: revivable}, nil
		}
		h.log.Warn("the session for this work cannot run a turn; replacing it",
			"run_key", w.RunKey, "session_id", existing.ID)
		if _, err := client.Sessions().Delete(ctx, existing.ID, omnigent.DeleteSessionOptions{}); err != nil {
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

	session, err := client.Sessions().Create(ctx, create)
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
	// Both halves, like the adopt path above. Dropping revivable here left a session
	// reconciled after a lost create unable to be woken: neither send arm fires, so
	// the run re-subscribes until its deadline without ever asking its question.
	live, revivable := reachability(committed)
	return committed, adoption{continued: true, live: live, revivable: revivable}, nil
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

// sessionIsLive reports whether a runner is registered for this session right now,
// and demands an explicit yes.
//
// Stricter than the reachability adoption uses, deliberately, because the two
// decisions fail in opposite directions. Adoption reads an unknown liveness as
// live so it never deletes a session on missing information; sending reads it as
// not-live so it never puts a prompt into a sandbox that does not exist, which is
// the failure this whole path is here to prevent. A read that errors answers no
// for the same reason.
func (h *Host) sessionIsLive(
	ctx context.Context,
	client *omnigent.Client,
	sessionID string,
) bool {
	readCtx, cancel := context.WithTimeout(ctx, h.cfg.RequestTimeout)
	defer cancel()

	session, err := client.Sessions().Get(readCtx, sessionID, omnigent.GetSessionOptions{})
	if err != nil {
		// Logged rather than swallowed. Whether this read succeeds while the stream
		// will not open is what separates a stream that is wedged from a server that
		// is: one says the fault is scoped to the stream path, the other says nothing
		// is getting through. Returning a bare false discards the difference.
		h.log.Warn("could not read the session while deciding whether it is live",
			"session_id", sessionID, "error", err)
		return false
	}
	return session.RunnerOnline != nil && *session.RunnerOnline
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
	walkCtx, cancel := h.boundWalk(ctx)
	defer cancel()

	ids := map[string]bool{}
	opts := omnigent.SessionItemsOptions{Limit: 1000}
	for item, err := range client.Sessions().ListItems(walkCtx, sessionID, opts) {
		if err != nil {
			return nil, fmt.Errorf("listing this session's items: %w", err)
		}
		// One field and nothing else: the guards that decide publishability run
		// on the typed snapshot, and all this needs is identity.
		if item.ResponseID != "" {
			ids[item.ResponseID] = true
		}
	}
	return ids, nil
}
