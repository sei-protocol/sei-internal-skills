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
	// This reply was placed by the paged walk, which reads slice index as
	// transcript time: a descending listing would put every reply before its own
	// prompt and refuse the review this path exists to recover. The route does
	// default to ascending, which is why the bug does not show -- it is the only
	// listing on the API that does, so the direction is requested, and asserted
	// here rather than inherited.
	queries := fs.ItemsQueries()
	if len(queries) == 0 {
		t.Fatal("no items listing was requested; this case is supposed to walk")
	}
	for i, q := range queries {
		if !strings.Contains(q, "order=asc") {
			t.Errorf("items page %d query = %q, want order=asc", i, q)
		}
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

// TestSalvageRefusesAPartialTranscript covers the walk's error branch, which decides
// whether a salvaged reply is published when the listing behind the proof dies partway.
//
// An incomplete transcript cannot prove a position either way, so the driver refuses. The
// branch is one line and the guard is easy to lose: answering from the pages that did
// arrive keeps every other test green.
//
// [TestSalvageRefusesWhenThePagedReadFails] is the neighbour, and it cannot cover this.
// Its listing fails on the first page, so a fall-through reaches the decision having
// observed nothing and refuses anyway — the two behaviours are indistinguishable there.
// They only diverge when a decisive page arrives and a later one fails, which is this
// fixture. Neither test is redundant.
//
// The fixture has to reach that branch, and two earlier attempts did not — the pass came
// from somewhere else entirely. Three things are load-bearing, so the control sub-test
// runs the same fixture with the failure removed and asserts the walk both ran and
// decided. If the control stops publishing, this file is testing nothing again.
//
//  1. The snapshot must not carry the anchor. driverSessionWithItems prepends the prompt
//     item, which makes groupIsAfterAnchor succeed on the window and skip the walk, so
//     this uses driverSessionItemsInOrder with the reply alone — the truncated window the
//     walk exists for.
//  2. The first request on the items route is priorResponseIDs, not the walk. Feeding it
//     the decisive page records this turn's own response id as one that predates the turn,
//     and the reply is then refused for that reason instead of this one.
//  3. The walk's own first page must be decisive on its own — anchor, then the reply group
//     after it — so that answering from what arrived would publish. Otherwise the refusal
//     and a correct negative answer are indistinguishable.
func TestSalvageRefusesAPartialTranscript(t *testing.T) {
	t.Parallel()

	// Page one of the walk: decisive, and it claims more.
	const decisivePage = `{"data":[{"id":"item_1","response_id":"resp_prompt"},` +
		`{"id":"item_reply","response_id":"resp_claude_a"}],` +
		`"has_more":true,"last_id":"item_reply"}`
	const emptyPage = `{"data":[],"has_more":false}`

	run := func(t *testing.T, lastPage string) (driver.Result, *driverFakeServer) {
		t.Helper()
		fs := newDriverFakeServer(t, driverFakeServerConfig{
			AgentPages:   []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
			CreateResp:   driverSessionResp("conv_a", "ag_1"),
			StreamFrames: []string{driverAckFrame()},
			SessionResps: []string{
				driverSessionItemsInOrder("conv_a", "ag_1",
					driverReplyItem("item_reply", "resp_claude_a",
						driverVerdict("The review, finished server-side.", "approve"))),
			},
			ItemsResps: []string{emptyPage, decisivePage, lastPage},
		})
		result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
			Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 91})
		return result, fs
	}

	// The control. Everything the refusal depends on, with nothing failing.
	t.Run("the walk runs and decides", func(t *testing.T) {
		t.Parallel()
		result, fs := run(t, emptyPage)

		if result.ExitCode != driver.ExitOK {
			t.Fatalf("ExitCode = %d, want driver.ExitOK: the fixture never reaches the "+
				"walk's decision, so the refusal below would pass for another reason",
				result.ExitCode)
		}
		if result.Reply == nil || !carriesDecision(result.Reply, "approve") {
			t.Errorf("Reply = %+v, want the committed review", result.Reply)
		}
		// One for priorResponseIDs, then both pages of the walk.
		if got := len(fs.ItemsQueries()); got != 3 {
			t.Errorf("items listing requests = %d, want 3 (prior ids, then two walk "+
				"pages): %v", got, fs.ItemsQueries())
		}
	})

	t.Run("a page that dies refuses the reply", func(t *testing.T) {
		t.Parallel()
		result, fs := run(t, driverItemsFail)

		if result.ExitCode == driver.ExitOK {
			t.Errorf("ExitCode = driver.ExitOK: the position was answered from the pages "+
				"that arrived, so an unfinished transcript published a reply. Reply = %+v",
				result.Reply)
		}
		if result.Reply != nil {
			t.Errorf("Reply = %+v, want none: a reply whose position is unproven must "+
				"not reach the caller", result.Reply)
		}
		// At least the three of the first attempt. The salvage refusing sends the run
		// back to the stream, and every later attempt walks and fails the same way, so
		// the count is a floor rather than an equality.
		if got := len(fs.ItemsQueries()); got < 3 {
			t.Errorf("items listing requests = %d, want at least 3: the failing page was "+
				"not reached, so nothing here exercises the refusal", got)
		}
	})
}
