package omni

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// TestSalvageRefusesWhatTheCompletenessCheckWouldAccept pins the two guards that
// stand between a dropped stream and publishing somebody else's review.
//
// recoverFromStreamLoss has six fail-closed guards. Wherever a fixture's rejected
// reply is also unfinished, the completeness guard rejects first and stands in for
// the rest, which is what left the active-response and anchor-position guards
// unpinned.
//
// Both cases here give the salvage a reply that is verdict-shaped and complete, so
// the completeness check cannot do the rejecting. What is left is the guard under
// test.
func TestSalvageRefusesWhatTheCompletenessCheckWouldAccept(t *testing.T) {
	t.Parallel()

	// Verdict-shaped and finished. The point is that completeness passes.
	const finished = "resp_claude_a"
	reply := driverReplyItem("item_reply", finished,
		driverVerdict("A complete review, from somebody else's turn.", "approve"))

	for _, tc := range []struct {
		name    string
		session string
		why     string
	}{
		{
			name: "the session still names an active response",
			// status running with an active response: the work continues, so the
			// stream ending says nothing about it.
			session: driverRunningSessionResp("conv_drop", "ag_1", "resp_claude_a",
				driverPromptItem(driverAnchorItemID), reply),
			why: "the turn was still running, so this is a half-written review",
		},
		{
			name: "the new reply sits before this turn's prompt",
			// The reply leads and the anchor follows, which is the stream prologue
			// replaying a previous invocation's completed work.
			session: fmt.Sprintf(
				`{"id":"conv_drop","agent_id":"ag_1","created_at":1,"status":"idle",`+
					`"items":[%s,%s]}`, reply, driverPromptItem(driverAnchorItemID)),
			why: "the reply predates this turn's prompt, so it is a previous run's",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
				CreateResp: driverSessionResp("conv_drop", "ag_1"),
				StreamFrames: []string{
					driverAckFrame(),
					driverConsumedFrame(driverAnchorItemID),
					// No done frame: the stream just stops.
				},
				SessionResps: []string{tc.session},
			})

			log, sink := driverCapturingLogger()
			result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, log).
				Run(t.Context(), testWork{
					Repo: "sei-protocol/sandbox", PR: 33, Trigger: "trigger-salvage"})

			if result.Reply != nil && result.Reply.Text != "" {
				t.Errorf("published a review anyway: %s\nreply: %q",
					tc.why, result.Reply.Text)
			}
			if result.ExitCode == driver.ExitOK {
				t.Errorf("ExitCode = driver.ExitOK, want the dropped stream to stand: %s",
					tc.why)
			}
			// The reply still reaches the log, which is the only record of what the
			// agent said on a run that published nothing.
			if !strings.Contains(sink.String(), "conv_drop") {
				t.Errorf("nothing named the session in the logs:\n%s", sink.String())
			}
		})
	}
}
