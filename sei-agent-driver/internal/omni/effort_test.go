package omni

import (
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// runWithEffort drives one review against fs with the configured reasoning effort.
func runWithEffort(t *testing.T, fs *driverFakeServer, work testWork, effort string) driver.Result {
	t.Helper()

	cfg := driverTestConfig(t, fs.URL)
	cfg.Effort = effort
	return newTestDriver(cfg, driver.Policy{}, driverTestLogger()).Run(t.Context(), work)
}

func TestCreateCarriesTheConfiguredEffort(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 31, Trigger: "first"}
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

	runWithEffort(t, fs, work, "high")

	created := fs.CreateReqs()
	if len(created) != 1 {
		t.Fatalf("created %d sessions %+v, want 1", len(created), created)
	}
	if created[0].ReasoningEffort == nil || *created[0].ReasoningEffort != "high" {
		t.Errorf("create reasoning_effort = %v, want high", created[0].ReasoningEffort)
	}
}

func TestAdoptedSessionIsPointedAtThisRunsEffort(t *testing.T) {
	t.Parallel()

	work := testWork{Repo: "sei-protocol/sandbox", PR: 32, Trigger: "again"}
	fs := modelFakeServer(t, testRunKey(work.Repo, work.PR), "")

	if result := runWithEffort(t, fs, work, "high"); result.SessionID != "conv_prior" {
		t.Fatalf("SessionID = %q, want conv_prior — the adopt path did not run", result.SessionID)
	}

	patches := fs.PatchReqs()
	if len(patches) != 1 {
		t.Fatalf("sent %d session patches %+v, want 1", len(patches), patches)
	}
	if patches[0].ReasoningEffort == nil || *patches[0].ReasoningEffort != "high" {
		t.Errorf("reasoning_effort = %v, want high", patches[0].ReasoningEffort)
	}
	if patches[0].ModelOverride != nil {
		t.Errorf("model_override = %q, want it untouched: no model was configured to move",
			*patches[0].ModelOverride)
	}
}
