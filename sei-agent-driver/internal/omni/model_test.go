package omni

import (
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
	if created[0].ModelOverride != "claude-opus-4-7" {
		t.Errorf("create model_override = %q, want claude-opus-4-7", created[0].ModelOverride)
	}
	// Nothing to reconcile on a session this run opened.
	if patches := fs.PatchReqs(); len(patches) != 0 {
		t.Errorf("sent %d session patches %+v, want none", len(patches), patches)
	}
}
