package omni

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// driverSessionItemsInOrder is a snapshot whose item order the caller decides,
// for the position checks [groupIsAfterAnchor] makes. driverSessionWithItems
// always leads with the prompt, which is the order that must be accepted; these
// tests need the orders that must not be.
func driverSessionItemsInOrder(id, agentID string, items ...string) string {
	return fmt.Sprintf(`{"id":%q,"agent_id":%q,"created_at":1,"status":"idle","items":[%s]}`,
		id, agentID, strings.Join(items, ","))
}

// TestSalvageAnchorsOnTheItemTheSendNamed covers the stream that dies between the
// send's response and its consume echo.
//
// The server answered the send with an item id, so the prompt is persisted and its
// position is known. Nothing replays the consume event, so a run that waits for it
// re-subscribes until the open limit and discards a review that is committed and
// paid for -- while the log reports the prompt as never persisted.
func TestSalvageAnchorsOnTheItemTheSendNamed(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("The review, finished server-side.", "approve")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_a", "ag_1"),
		// The prompt goes in and is acked with item_1, then the connection dies
		// with no consume echo and no terminal sentinel.
		StreamFrames: []string{driverAckFrame()},
		LaterStreamFrames: []string{
			driverAckFrame(), driverIdleFrame("resp_claude_a"), driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_a", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", reply)),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 90})

	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK (%d): the send named an item, so the "+
			"reply's position is provable without the consume echo",
			result.ExitCode, driver.ExitOK)
	}
	if result.Reply == nil || !carriesDecision(result.Reply, "approve") {
		t.Fatalf("Reply = %+v, want the committed review", result.Reply)
	}
	// One open. Waiting for an echo that is never replayed is what spent forty.
	if got := fs.StreamHits(); got != 1 {
		t.Errorf("stream subscriptions = %d, want 1: the answer was already readable", got)
	}
}

// TestSalvageStillRefusesWithoutAProvablePosition pins the guards the change above
// must not relax. Each of these has an item anchor, so the gate is passed and the
// refusal has to come from the guard named.
func TestSalvageStillRefusesWithoutAProvablePosition(t *testing.T) {
	t.Parallel()

	verdict := driverVerdict("A complete-looking review.", "approve")
	other := driverVerdict("Another turn's review.", "request_changes")

	for _, tc := range []struct {
		name     string
		eventAck string
		session  string
		// wantLog is the refusal this case is about. Asserted because the guards
		// overlap: several of these snapshots are refused by more than one of them,
		// so a test that only checks "nothing was published" passes when the guard
		// it names is gone and a neighbour catches it instead.
		wantLog string
		why     string
	}{
		{
			name: "a reply that sits before this turn's prompt",
			// The only new reply group predates the anchor, which is how a previous
			// invocation's completed message looks on a reused session.
			session: driverSessionItemsInOrder("conv_b", "ag_1",
				driverReplyItem("item_old", "resp_previous", verdict),
				driverPromptItem(driverAnchorItemID)),
			wantLog: "the new reply does not sit after this turn's prompt",
			why:     "position is what separates our reply from a replayed one",
		},
		{
			name: "two new reply groups",
			session: driverSessionItemsInOrder("conv_b", "ag_1",
				driverPromptItem(driverAnchorItemID),
				driverReplyItem("item_r1", "resp_one", verdict),
				driverReplyItem("item_r2", "resp_two", other)),
			wantLog: "does not name one new reply",
			why:     "nothing on the wire says which of two replies is ours",
		},
		{
			name: "a turn the session still reports as running",
			session: driverRunningSessionResp("conv_b", "ag_1", "resp_claude_a",
				driverPromptItem(driverAnchorItemID),
				driverReplyItem("item_r1", "resp_claude_a", verdict)),
			// The active-response arm returns without logging, so there is no line
			// to name here. It is pinned by the outcome alone.
			why: "an active response says the agent is still writing",
		},
		{
			name: "a prompt still parked as a pending input",
			// A pending id names no item, so no position can be proven and the
			// anchor gate itself must refuse.
			eventAck: `{"queued":true,"pending_id":"pending_1"}`,
			session: driverSessionItemsInOrder("conv_b", "ag_1",
				driverPromptItem("item_drained"),
				driverReplyItem("item_r1", "resp_claude_a", verdict)),
			wantLog: "still a pending input",
			why:     "a pending id names no item to compare against",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages:   []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
				CreateResp:   driverSessionResp("conv_b", "ag_1"),
				EventResp:    tc.eventAck,
				StreamFrames: []string{driverAckFrame()},
				// Every later open behaves the same way, so the run exhausts its
				// opens rather than being rescued by a frame a fixture happened
				// to schedule.
				LaterStreamFrames: []string{driverAckFrame()},
				SessionResps:      []string{tc.session},
			})

			cfg := driverTestConfig(t, fs.URL)
			cfg.RunDeadline = 5 * time.Second
			sink := &driverLogSink{}
			result := newTestDriver(cfg, driver.Policy{},
				slog.New(slog.NewTextHandler(sink, nil))).
				Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 91})

			if result.Reply != nil && result.Reply.Text != "" {
				t.Errorf("published %q: %s", result.Reply.Text, tc.why)
			}
			if result.ExitCode == driver.ExitOK {
				t.Errorf("ExitCode = driver.ExitOK: %s", tc.why)
			}
			if tc.wantLog != "" && !strings.Contains(sink.String(), tc.wantLog) {
				t.Errorf("no record of %q, so something other than this guard refused it:\n%s",
					tc.wantLog, sink.String())
			}
		})
	}
}
