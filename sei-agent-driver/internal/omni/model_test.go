package omni

import (
	"net/http"
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// modelFakeServer stands up a server whose listing already holds this work's session,
// so every case here runs the adopt path rather than the create path.
//
// override is what that session currently carries: empty renders no field, which is
// the wire shape for "no override active".
func modelFakeServer(t *testing.T, runKey, override string) *driverFakeServer {
	t.Helper()

	// On the snapshot, not the list item: the label match happens on the listing, but
	// findByRunKey then fetches the session, and that fetch is what adoption reads.
	adopted := driverSessionResp("conv_prior", "ag_1")
	if override != "" {
		adopted = `{"id":"conv_prior","agent_id":"ag_1","created_at":1,"status":"idle",` +
			`"items":[],"model_override":"` + override + `"}`
	}
	return newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "", false)},
		CreateResp: driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[{"id":"conv_prior","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			adopted,
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionWithItems("conv_prior", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Re-read the diff.", "comment"))),
		},
	})
}

// runWithModel drives one review against fs with the configured model.
func runWithModel(t *testing.T, fs *driverFakeServer, work testWork, model string) driver.Result {
	t.Helper()

	cfg := driverTestConfig(t, fs.URL)
	cfg.Model = model
	return newTestDriver(cfg, driver.Policy{}, driverTestLogger()).Run(t.Context(), work)
}

// TestAdoptedSessionIsPointedAtThisRunsModel covers the case the create-time override
// cannot reach.
//
// The override lives on the session row, and a session opened by an earlier dispatch
// carries that dispatch's model. A pull request that gains a model label between
// reviews has to move the session it already has, or the label reads as applied while
// the old model answers.
func TestAdoptedSessionIsPointedAtThisRunsModel(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 21, Trigger: "labelled"}
	fs := modelFakeServer(t, testRunKey(work.Repo, work.PR), "")

	if result := runWithModel(t, fs, work, "claude-opus-4-7"); result.SessionID != "conv_prior" {
		t.Fatalf("SessionID = %q, want conv_prior — the adopt path did not run", result.SessionID)
	}

	patches := fs.PatchReqs()
	if len(patches) != 1 {
		t.Fatalf("sent %d session patches %+v, want 1", len(patches), patches)
	}
	if patches[0].ModelOverride == nil || *patches[0].ModelOverride != "claude-opus-4-7" {
		t.Errorf("model_override = %v, want claude-opus-4-7", patches[0].ModelOverride)
	}
}

// TestAdoptedSessionIsLeftAloneWhenItsModelAlreadyMatches keeps the reconcile from
// costing a request per review.
//
// Most reviews configure no model and adopt a session carrying no override, so the
// common path has to be free. This is the same rule with a value on both sides, which
// is the half a nil-versus-empty comparison gets wrong.
func TestAdoptedSessionIsLeftAloneWhenItsModelAlreadyMatches(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, configured, carried string }{
		{"neither side has one", "", ""},
		{"both sides agree", "claude-opus-4-7", "claude-opus-4-7"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			work := testWork{Repo: "sei-protocol/sandbox", PR: 22, Trigger: "comment"}
			fs := modelFakeServer(t, testRunKey(work.Repo, work.PR), c.carried)

			runWithModel(t, fs, work, c.configured)

			if patches := fs.PatchReqs(); len(patches) != 0 {
				t.Errorf("sent %d session patches %+v, want none", len(patches), patches)
			}
		})
	}
}

// TestAdoptedSessionsOverrideIsClearedWhenNoModelIsConfigured is the symmetric case,
// and the one a set-only reconcile leaves broken.
//
// Removing the label has to return the session to the model its agent spec names. A
// reconcile that only ever sets would leave the first labelled review's model in place
// for every later review of that pull request.
func TestAdoptedSessionsOverrideIsClearedWhenNoModelIsConfigured(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 23, Trigger: "unlabelled"}
	fs := modelFakeServer(t, testRunKey(work.Repo, work.PR), "claude-opus-4-7")

	runWithModel(t, fs, work, "")

	patches := fs.PatchReqs()
	if len(patches) != 1 {
		t.Fatalf("sent %d session patches %+v, want 1", len(patches), patches)
	}
	// The clear is an alias rather than an empty string, and the SDK owns which word.
	// What this pins is that a clear was sent at all, and that it was not a set.
	if patches[0].ModelOverride == nil || *patches[0].ModelOverride == "" {
		t.Errorf("model_override = %v, want a clear alias", patches[0].ModelOverride)
	}
}

// TestCreateCarriesTheConfiguredModel pins the other half of the pair.
//
// The override has to be on the session row before the harness launches, so a session
// this run opens sends it at create. Patching afterwards would leave the first turn --
// the one that writes the review -- on the agent spec's model.
func TestCreateCarriesTheConfiguredModel(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 24, Trigger: "first"}
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:      []string{driverAgentPage("ag_1", "seidroid", "", false)},
		CreateResp:      driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionResp("conv_new", "ag_1"),
			driverSessionWithItems("conv_new", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read the diff.", "comment"))),
		},
	})

	runWithModel(t, fs, work, "claude-opus-4-7")

	created := fs.CreateReqs()
	if len(created) != 1 {
		t.Fatalf("created %d sessions %+v, want 1", len(created), created)
	}
	if created[0].ModelOverride == nil || *created[0].ModelOverride != "claude-opus-4-7" {
		t.Errorf("create model_override = %v, want claude-opus-4-7", created[0].ModelOverride)
	}
	// Nothing to reconcile on a session this run opened.
	if patches := fs.PatchReqs(); len(patches) != 0 {
		t.Errorf("sent %d session patches %+v, want none", len(patches), patches)
	}
}

// TestCreateOmitsTheModelWhenNoneIsConfigured pins the path every current run takes.
//
// Nothing is configured in the ordinary case, and the wire shape of that case is not
// obvious from the Go: the SDK tags model_override omitempty, so the field should be
// absent rather than sent as an empty string. The difference is not cosmetic — a server
// that validates the value would reject every create, which is a review outage rather
// than a review on the wrong model. The fake accepts any body, so nothing else in this
// package would notice the tag changing.
func TestCreateOmitsTheModelWhenNoneIsConfigured(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 25, Trigger: "first"}
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:      []string{driverAgentPage("ag_1", "seidroid", "", false)},
		CreateResp:      driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionResp("conv_new", "ag_1"),
			driverSessionWithItems("conv_new", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read the diff.", "comment"))),
		},
	})

	runWithModel(t, fs, work, "")

	created := fs.CreateReqs()
	if len(created) != 1 {
		t.Fatalf("created %d sessions %+v, want 1", len(created), created)
	}
	if created[0].ModelOverride != nil {
		t.Errorf("create sent model_override = %q; want the field absent, because the "+
			"SDK tags it omitempty and nothing asked for a model",
			*created[0].ModelOverride)
	}
}

// TestAdoptedScoutSessionIsLeftAtItsOwnModel drives a workload on its own agent
// through the adopt path, which is where the three-valued model earns its shape.
//
// A scout carries a nil model: not "claude-opus-4-7", so the configured one does not
// reach another provider's harness, and not "" either, because "" means clear and
// clearing an override this run does not manage is the same overreach as setting one.
// The session here already carries one, so a reconcile that ran at all would be visible
// as a patch.
//
// It is also the only test that takes a nil model through the host. A nil deref here
// fails the review outright, and nothing else in this package exercises the path.
func TestAdoptedScoutSessionIsLeftAtItsOwnModel(t *testing.T) {
	t.Parallel()

	scout := namingWork{
		testWork: testWork{Repo: "sei-protocol/sandbox", PR: 26, Trigger: "scout"},
		agent:    "seidroid-codex",
	}
	runKey := testRunKey(scout.Repo, scout.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_codex", "seidroid-codex", "", false)},
		CreateResp: driverSessionResp("conv_new", "ag_codex"),
		SessionListResp: `{"data":[{"id":"conv_scout","agent_id":"ag_codex","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			// Already carrying an override, so a reconcile could not be a no-op.
			`{"id":"conv_scout","agent_id":"ag_codex","created_at":1,"status":"idle",` +
				`"items":[],"model_override":"gpt-5-codex"}`,
			driverSessionResp("conv_scout", "ag_codex"),
			driverSessionWithItems("conv_scout", "ag_codex",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read the diff.", "comment"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.Model = "claude-opus-4-7"
	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).Run(t.Context(), scout)

	if result.SessionID != "conv_scout" {
		t.Fatalf("SessionID = %q, want conv_scout — the scout adopt path did not run",
			result.SessionID)
	}
	if patches := fs.PatchReqs(); len(patches) != 0 {
		t.Errorf("sent %d session patches %+v to a scout, want none: the configured "+
			"model is not its to take, and not its to lose", len(patches), patches)
	}
}

// TestCreatedScoutSessionCarriesNoModel is the nil-at-create case, and it is the one
// the sibling create test cannot reach.
//
// A review with nothing configured still carries a non-nil model — a pointer to the
// empty string, meaning "no override" — so it exercises the value branch. Only a
// workload on its own agent carries nil, and only its first dispatch creates. Without
// this, [modelOrEmpty] could return anything for nil and the suite would stay green
// while every first scout dispatch sent it.
func TestCreatedScoutSessionCarriesNoModel(t *testing.T) {
	t.Parallel()

	scout := namingWork{
		testWork: testWork{Repo: "sei-protocol/sandbox", PR: 27, Trigger: "scout"},
		agent:    "seidroid-codex",
	}
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:      []string{driverAgentPage("ag_codex", "seidroid-codex", "", false)},
		CreateResp:      driverSessionResp("conv_new", "ag_codex"),
		SessionListResp: `{"data":[],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionResp("conv_new", "ag_codex"),
			driverSessionWithItems("conv_new", "ag_codex",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read the diff.", "comment"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.Model = "claude-opus-4-7"
	newTestDriver(cfg, driver.Policy{}, driverTestLogger()).Run(t.Context(), scout)

	created := fs.CreateReqs()
	if len(created) != 1 {
		t.Fatalf("created %d sessions %+v, want 1", len(created), created)
	}
	if created[0].ModelOverride != nil {
		t.Errorf("created a scout session with model_override = %q; want the field "+
			"absent — the configured model belongs to the review's agent",
			*created[0].ModelOverride)
	}
}

// TestReviewSurvivesAFailedModelMove is the fail-open promise, and it had no test.
//
// The model is a preference. A run that refused because the server would not move it
// would trade a review on the wrong model for no review at all, which is the worse of
// the two for the author waiting on it.
//
// The session here carries a different override, so the reconcile reaches the write and
// the server rejects it. What must still arrive is the verdict.
func TestReviewSurvivesAFailedModelMove(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 28, Trigger: "labelled"}
	runKey := testRunKey(work.Repo, work.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:  []string{driverAgentPage("ag_1", "seidroid", "", false)},
		CreateResp:  driverSessionResp("conv_new", "ag_1"),
		PatchStatus: http.StatusInternalServerError,
		SessionListResp: `{"data":[{"id":"conv_prior","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			`{"id":"conv_prior","agent_id":"ag_1","created_at":1,"status":"idle",` +
				`"items":[],"model_override":"an-older-model"}`,
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionWithItems("conv_prior", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Re-read the diff.", "comment"))),
		},
	})

	result := runWithModel(t, fs, work, "claude-opus-4-7")

	// The write was attempted, so this is the failure path and not a skipped reconcile.
	if patches := fs.PatchReqs(); len(patches) != 1 {
		t.Fatalf("sent %d patches %+v, want 1 — the reconcile never reached the server, "+
			"so this test is not exercising the failure", len(patches), patches)
	}
	if !carriesDecision(result.Reply, "comment") {
		t.Errorf("Reply = %+v; want the review to arrive anyway — a model that could not "+
			"be moved is a preference unmet, not a reason to leave the pull request "+
			"without a review", result.Reply)
	}
	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want %d — the run must not fail on a preference",
			result.ExitCode, driver.ExitOK)
	}
}
