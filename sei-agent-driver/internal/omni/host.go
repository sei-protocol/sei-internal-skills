package omni

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
// The value keeps the xreview spelling deliberately, and a rename sweep must not take
// it. This label is written on live sessions, it is the only thing adopt matches on, and
// it is the only label create writes. Changing it orphans every session a running
// deployment would otherwise adopt: a second session per pull request, and a --close
// that walks the listing, matches nothing and reclaims nothing, against a server with no
// lifetime cap and no sweep of its own.
//
// [TestRunKeyLabelIsNotSwept] is the guard rather than this paragraph, because this
// paragraph already failed to stop one sweep -- the rename that dropped the x rewrote
// the warning along with the value, leaving a comment that argued against its own line.
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
	return &Host{cfg: withUsableTimeouts(cfg), policy: policy, log: log}
}

// withUsableTimeouts replaces a non-positive timeout with the default
// [driver.LoadConfig] would have supplied.
//
// [driver.Config] is exported with exported fields and this constructor takes one,
// so nothing makes a caller build it through LoadConfig. A zero RequestTimeout is
// the expensive one: context.WithTimeout with it yields a context that is already
// expired, so the read that collects the agent's answer fails instantly. The run
// resolves the agent, launches a sandbox, sends the prompt, waits out the turn, and
// then throws the finished reply away -- and it exits as a transport fault, which a
// caller retries, so each retry pays for the review again and discards it again.
//
// Two callers already defend themselves this way and say so where they do it:
// [driver.Driver.Close] floors its teardown budget, and [Host.boundWalk] floors a listing.
// Doing it here instead puts the guard on the path every one of them takes, rather
// than asking each new read to remember.
func withUsableTimeouts(cfg driver.Config) driver.Config {
	for _, field := range []struct {
		value    *time.Duration
		fallback time.Duration
	}{
		{&cfg.RequestTimeout, driver.DefaultRequestTimeout},
		{&cfg.UnaryTimeout, driver.DefaultUnaryTimeout},
		{&cfg.StreamIdleTimeout, driver.DefaultStreamIdleTimeout},
	} {
		if *field.value <= 0 {
			*field.value = field.fallback
		}
	}
	return cfg
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
	// A search that ran out of budget still reports what it found, so reclaim runs on
	// the partial set rather than being abandoned. searchErr is carried to the end: it
	// means the set may be short, so even a clean sweep of it cannot attest that
	// nothing is held.
	ids, searchErr := h.findAllByRunKey(ctx, client, agent.ID, w.RunKey)
	if len(ids) == 0 {
		return "", searchErr
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
	if searchErr != nil {
		// Everything named was deleted, but the naming was incomplete. Reported as a
		// leak because that is what it is: a session this run created may still hold a
		// sandbox and nothing has looked at it. The id returned is one that was
		// reclaimed, which is the run key an operator searches on.
		return ids[0], fmt.Errorf("%w: reclaimed %s before the search finished: %w",
			driver.ErrLeaked, strings.Join(ids, ", "), searchErr)
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
// The SDK's iterator ends when the server says there is no more, and refuses — with
// [omnigent.ErrListingUnbounded] — on an empty page that claims more, on a page
// claiming more with no cursor, on a repeated cursor, and at its 10,000-page
// backstop. So a walk terminates. What it does not do is terminate soon: ten
// thousand pages of one request each is minutes, and a proxy minting a fresh cursor
// per page reaches that cap rather than the repeat guard.
//
// The caller with no budget of its own is Close. It would spend the whole teardown
// window listing, then exit by deadline rather than by the leak path, so nothing
// names the sandbox it failed to reclaim.
//
// The budget is a share of what the caller still has, never all of it. Close runs on
// 4*RequestTimeout, so a walk allowed the same would leave nothing for the deletes
// the walk exists to find -- the search would consume the reclaim it was serving.
// Halving whatever remains keeps that true however the caller was budgeted.
//
// Floored, because the multiplier can round to nothing: a zero RequestTimeout would
// otherwise hand the walk a context that has already expired and turn every reclaim
// into a failure before one request goes out.
//
// The share is applied after the floor, not before, so the floor cannot undo it. A
// caller with under two seconds left is the case where the two rules disagree, and the
// share is the one that has to win: a walk handed the whole window is a search that
// consumed the reclaim it was serving, which is the failure this exists to prevent.
// Half of very little is still a request or two, and it leaves the caller the same.
func (h *Host) boundWalk(ctx context.Context) (context.Context, context.CancelFunc) {
	budget := max(listingWalkBudget*h.cfg.RequestTimeout, minListingWalkBudget)
	if deadline, ok := ctx.Deadline(); ok {
		budget = min(budget, time.Until(deadline)/2)
	}
	return context.WithTimeout(ctx, budget)
}

// itemWalkOptions is how this driver pages a session's items.
//
// Order is sent rather than defaulted. One caller reads position out of the
// listing's own sequence, and this route is the only listing on the API that
// defaults to ascending -- sessions and agents default to descending -- so the
// direction it needs is asked for instead of inherited. The caller that only
// collects ids does not care, and shares this so there is one answer to what a
// page of items looks like.
func itemWalkOptions() omnigent.SessionItemsOptions {
	return omnigent.SessionItemsOptions{Limit: 1000, Order: omnigent.SortAscending}
}

const (
	// listingWalkBudget multiplies [driver.Config.RequestTimeout] into a whole-listing
	// budget. A listing this driver expects to fit in one or two pages gets room for a
	// handful, and no more. Deliberately below the multiplier Close budgets itself on,
	// so a walk cannot spend the window it is searching on behalf of.
	listingWalkBudget = 2

	// minListingWalkBudget is the least a walk gets, so a misconfigured or nearly
	// spent caller still issues its requests instead of failing on arithmetic. Small
	// on purpose: it exists to avoid a zero-length context, not to grant a budget, so
	// the multiplier above stays the thing that normally decides.
	minListingWalkBudget = time.Second
)

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
			// The matches already found are returned with the error, not dropped.
			// Reclaiming three sandboxes of four beats reclaiming none, and the error
			// still tells Close the search was incomplete, so it cannot report a
			// teardown it did not establish.
			return ids, err
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

// modelOrEmpty reads a work's model override, treating "leave it alone" as "no
// override at create" -- a session being opened here carries nothing to leave alone.
func modelOrEmpty(model *string) string {
	if model == nil {
		return ""
	}
	return *model
}

// reconcileModel moves an adopted session's model override to what this work asks for.
//
// The override lives on the session row, not on the turn, so a session opened by an
// earlier dispatch answers on that dispatch's model until something changes it. A pull
// request that gained a model label between reviews would otherwise be answered by the
// old model with nothing in the output saying so.
//
// The current value is read from the session rather than assumed, so the common case --
// nothing asked for and no override present -- costs no request.
//
// live is reported rather than acted on, and it is the limit of what this can promise.
// The server places the override on the row so that it is there *before the harness
// launches*, which is the SDK's stated reason for its create-time field. A session whose
// harness is already up has therefore already read its model, and this run's turn goes to
// that process: the row is correct for the next launch, not for the turn about to be
// sent. Saying so is the point -- an unqualified success here would have the log claim a
// change this run did not get.
//
// A failure is logged, not returned. The model is a preference, and the review still runs
// on the model the session already carries; refusing here would turn a preference into a
// pull request with no review on it. Not the agent spec's model -- this runs only when
// current and want already differ, so a failed write leaves the session on the override
// it had, which in the clear case is the very one the run was removing.
func (h *Host) reconcileModel(
	ctx context.Context,
	client *omnigent.Client,
	session *omnigent.SessionResponse,
	want string,
	live bool,
) {
	current := ""
	if session.ModelOverride != nil {
		current = *session.ModelOverride
	}
	if current == want {
		return
	}

	var err error
	if want == "" {
		_, err = client.Sessions().ClearModelOverride(ctx, session.ID)
	} else {
		_, err = client.Sessions().SetModelOverride(ctx, session.ID, want)
	}
	if err != nil {
		h.log.Warn("could not move the adopted session's model, so it answers on the "+
			"one it already had",
			"session_id", session.ID, "want", want, "have", current, "error", err)
		return
	}
	if live {
		h.log.Warn("moved the adopted session's model, but its harness is already up "+
			"and read the old one at launch, so this run still answers on that",
			"session_id", session.ID, "want", want, "have", current)
		return
	}
	h.log.Info("moved the adopted session's model before its harness launched",
		"session_id", session.ID, "want", want, "have", current)
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
			if w.Model != nil {
				h.reconcileModel(ctx, client, existing, *w.Model, live)
			}
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

		// At create as well as on adopt, because the override has to be on the session
		// row before the harness launches. Setting it afterwards would leave the first
		// turn -- the one that writes the review -- on the spec's model.
		ModelOverride: modelOrEmpty(w.Model),
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
	for item, err := range client.Sessions().ListItems(walkCtx, sessionID, itemWalkOptions()) {
		if err != nil {
			return nil, fmt.Errorf("listing this session's items: %w", err)
		}
		// One field and nothing else: the guards that decide publishability run
		// on the typed snapshot, and all this needs is identity.
		//
		// A method rather than a field, because this route sends the flatten-for-API
		// shape and the SDK yields it untyped. The typed item the snapshot route
		// carries is a different shape, and reading its payload off one of these
		// would have found nothing.
		if id := item.ResponseID(); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}
