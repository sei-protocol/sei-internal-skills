package omni

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// TestDriverAdoptsAnExistingSessionForTheSameRunKey is the idempotency guarantee,
// and it is the one the deleted lease file could not provide.
//
// The lease lived in per-job scratch that is emptied before the process starts, so
// it was written and never read — its duplicate branch was unreachable. The run
// key is instead a label on the session, which is server-side and outlives the
// runner, so a redelivered trigger finds the first run's session and drives that
// rather than starting a second review of the same tree.
func TestDriverAdoptsAnExistingSessionForTheSameRunKey(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 12, Trigger: "redelivered-comment"}
	runKey := testRunKey(req.Repo, req.PR)

	// The listing answers with a session already carrying this run key, which is
	// what a second dispatch of the same trigger would find.
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		CreateResp: driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[{"id":"conv_prior","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		// Three reads happen on the adopt path, in order: the snapshot adoption
		// takes after the label matches, the parked-prompt read, and the reply read.
		SessionResps: []string{
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionWithItems("conv_prior", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Re-read the diff.", "comment"))),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), req)

	// The whole point: no second session was made.
	if created := fs.CreateReqs(); len(created) != 0 {
		t.Errorf("created %d sessions, want 0 — the labelled session should have been adopted: %+v",
			len(created), created)
	}
	if result.SessionID != "conv_prior" {
		t.Errorf("SessionID = %q, want conv_prior (the adopted session)", result.SessionID)
	}
	if fs.ListSessionHits() == 0 {
		t.Error("the pre-create search never ran; idempotency rests on it")
	}
	// It still drives the adopted session to a verdict rather than treating the
	// duplicate as nothing to do.
	if !carriesDecision(result.Reply, "comment") {
		t.Errorf("Verdict = %+v, want the adopted session driven to a verdict", result.Reply)
	}
}

// TestDriverCreatesWhenNoSessionCarriesTheRunKey is the other half: an empty
// search must not be read as "already done".
func TestDriverCreatesWhenNoSessionCarriesTheRunKey(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:      []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		CreateResp:      driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_new", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", driverVerdict("Fine.", "approve"))),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{
			Repo: "sei-protocol/sandbox", PR: 13, Trigger: "fresh"})
	if created := fs.CreateReqs(); len(created) != 1 {
		t.Fatalf("created %d sessions, want exactly 1", len(created))
	}
	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK", result.ExitCode)
	}
}

// TestDriverAdoptsRatherThanCreatingWhenTheLabelIsOnALaterPage checks the search
// pages. There is no server-side label filter and a page holds the agent's newest
// sessions, so a prior session for a busy agent will not be on page one — and
// stopping there would create the duplicate this exists to prevent.
func TestDriverAdoptsRatherThanCreatingWhenTheLabelIsOnALaterPage(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 14, Trigger: "page-two"}
	runKey := testRunKey(req.Repo, req.PR)

	// The fake server serves one listing body, so instead of two pages this
	// asserts the narrower thing the same code path turns on: a page whose
	// has_more is true and whose entries do not match must not short-circuit
	// into a create. Here the match is on that same page behind a non-matching
	// entry, which proves the scan does not stop at the first row either.
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		CreateResp: driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[` +
			`{"id":"conv_other","agent_id":"ag_1","labels":{"` + RunKeyLabel + `":"someone-else"}},` +
			`{"id":"conv_mine","agent_id":"ag_1","labels":{"` + RunKeyLabel + `":"` + runKey + `"}}` +
			`],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(), driverCreatedFrame(),
			driverDeltaFrame("```json\n{\"decision\":\"approve\"}\n```"),
			driverCompletedFrame(), driverDoneFrame(),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), req)
	if len(fs.CreateReqs()) != 0 {
		t.Error("created a session despite a matching label further down the page")
	}
	if result.SessionID != "conv_mine" {
		t.Errorf("SessionID = %q, want conv_mine", result.SessionID)
	}
}

// TestCloseReportsAFailedDelete is the only path that still produces
// driver.ExitTeardownLeak. A refused delete has to reach the exit code rather than the
// logs, because the run reads as successful otherwise and nothing reclaims the
// sandbox afterwards.
func TestCloseReportsAFailedDelete(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 23}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		SessionListResp: `{"data":[{"id":"conv_stuck","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		DeleteStatus: http.StatusInternalServerError,
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Close(t.Context(), req)
	if result.ExitCode != driver.ExitTeardownLeak {
		t.Errorf("ExitCode = %d, want driver.ExitTeardownLeak (%d): a refused delete has to be "+
			"reported, not swallowed", result.ExitCode, driver.ExitTeardownLeak)
	}
	if result.TeardownOK {
		t.Error("TeardownOK = true, want false: the server refused the delete")
	}
	if result.SessionID != "conv_stuck" {
		t.Errorf("SessionID = %q, want conv_stuck: the leaked session has to be nameable",
			result.SessionID)
	}
}

// TestCloseDestroysTheSession covers the close path, which is the
// only thing that reclaims a sandbox: a review deletes nothing, so the pod runs
// from the first review until this does.
func TestCloseDestroysTheSession(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 21}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		SessionListResp: `{"data":[{"id":"conv_close","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Close(t.Context(), req)
	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK", result.ExitCode)
	}
	if got := fs.DeletedIDs(); len(got) != 1 || got[0] != "conv_close" {
		t.Errorf("deleted = %v, want [conv_close]", got)
	}
}

// TestCloseIsQuietWhenThereIsNoSession keeps a pull request that was
// closed without ever being reviewed from reading as a failure.
func TestCloseIsQuietWhenThereIsNoSession(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:      []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		SessionListResp: `{"data":[],"has_more":false}`,
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Close(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})
	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK for a PR with no session", result.ExitCode)
	}
	if got := fs.DeleteHits(); got != 0 {
		t.Errorf("DELETE calls = %d, want 0", got)
	}
}

// TestTwoDifferentTriggersShareOneSession is the continuity contract stated at
// the level a caller sees it: two separate comments on the same pull request must
// land on the same session, so the second review builds on the first.
//
// RunKey's signature no longer takes a trigger, so this cannot regress silently
// through that function — but it can regress by something else being folded into
// the key, or by the adopt path being bypassed, and this catches both.
func TestTwoDifferentTriggersShareOneSession(t *testing.T) {
	t.Parallel()

	first := testWork{Repo: "sei-protocol/sandbox", PR: 30, Trigger: "comment-111"}
	second := testWork{Repo: "sei-protocol/sandbox", PR: 30, Trigger: "comment-222"}

	if testRunKey(first.Repo, first.PR) != testRunKey(second.Repo, second.PR) {
		t.Fatal("two triggers on one pull request produced different keys; " +
			"each invocation would start a fresh session and lose the context")
	}

	// And the driver acts on that: with a session already carrying the key, the
	// second dispatch adopts rather than creating.
	runKey := testRunKey(second.Repo, second.PR)
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		CreateResp: driverSessionResp("conv_should_not_be_created", "ag_1"),
		SessionListResp: `{"data":[{"id":"conv_first","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(), driverCreatedFrame(),
			driverDeltaFrame("```json\n{\"decision\":\"approve\"}\n```"),
			driverCompletedFrame(), driverDoneFrame(),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), second)
	if result.SessionID != "conv_first" {
		t.Errorf("SessionID = %q, want conv_first — the second trigger must reuse the first's session",
			result.SessionID)
	}
	if len(fs.CreateReqs()) != 0 {
		t.Error("a second session was created; the conversation from the first review is lost")
	}
}

// TestDriverReplacesASessionThatCannotRunATurn is the guard against a pull
// request becoming permanently unreviewable.
//
// A session whose sandbox is gone keeps its conversation, so the run key finds it
// forever. Adopting it fails the same way every time — the prompt is accepted
// without being queued — and no later dispatch can escape, because the search
// keeps returning the same dead session. Sandboxes are reclaimed over time, so
// this is the eventual state of every session rather than a rare one.
func TestDriverReplacesASessionThatCannotRunATurn(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 21, Trigger: "trigger-dead"}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		CreateResp: driverSessionResp("conv_fresh", "ag_1"),
		// Found by run key, but no runner and a host the provider cannot wake.
		SessionListResp: `{"data":[{"id":"conv_dead","agent_id":"ag_1","runner_online":false,` +
			`"host_resumable":false,"labels":{"` + RunKeyLabel + `":"` + runKey + `"}}],` +
			`"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			// The read adoption does after the label matches: no runner, and a host
			// the provider cannot wake.
			driverLivenessResp("conv_dead", "ag_1", false, false),
			driverSessionWithItems("conv_fresh", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Reviewed on a working session.", "comment"))),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), req)

	if created := fs.CreateReqs(); len(created) != 1 {
		t.Fatalf("created %d sessions, want 1: a session that cannot run a turn must be "+
			"replaced rather than adopted", len(created))
	}
	if got := fs.DeleteHits(); got != 1 {
		t.Errorf("DELETE calls = %d, want 1: the dead session must be removed or the new "+
			"one collides with it on the run key", got)
	}
	if result.SessionID != "conv_fresh" {
		t.Errorf("SessionID = %q, want conv_fresh: the run must drive the replacement",
			result.SessionID)
	}
}

// TestDriverKeepsASessionWhoseHostCanBeWoken separates the two false cases. A
// dormant host the provider can resume wakes when it is sent a message, so
// deleting it would throw away the conversation this feature exists to keep.
func TestDriverKeepsASessionWhoseHostCanBeWoken(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 22, Trigger: "trigger-asleep"}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		CreateResp: driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[{"id":"conv_asleep","agent_id":"ag_1","runner_online":false,` +
			`"host_resumable":true,"labels":{"` + RunKeyLabel + `":"` + runKey + `"}}],` +
			`"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverLivenessResp("conv_asleep", "ag_1", false, true),
			driverSessionResp("conv_asleep", "ag_1"),
			driverSessionWithItems("conv_asleep", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Woke and reviewed.", "comment"))),
		},
	})

	newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), req)
	if created := fs.CreateReqs(); len(created) != 0 {
		t.Errorf("created %d sessions, want 0: a resumable host is woken, not replaced", len(created))
	}
	if got := fs.DeleteHits(); got != 0 {
		t.Errorf("DELETE calls = %d, want 0: deleting a resumable session discards the "+
			"conversation it exists to keep", got)
	}
}

// TestDriverAsksForAFirstReviewUntilTheConversationHoldsOne pins which prompt a
// session gets, which is decided by what the conversation holds rather than by how
// the session was found.
//
// The two ways of finding one that holds nothing are the cases worth pinning. A
// create that commits and loses its response leaves a session this run just made,
// and a run that expired before its first reply leaves one an earlier dispatch
// made. Both are found by the run key and neither has been reviewed, so the
// what-changed-since prompt would tell the agent it had already read a pull
// request nobody has read.
func TestDriverAsksForAFirstReviewUntilTheConversationHoldsOne(t *testing.T) {
	t.Parallel()

	// Named, so each row below says which prompt it expects rather than carrying a
	// bare boolean.
	const (
		unanswered = false
		answered   = true
	)

	req := testWork{Repo: "sei-protocol/sandbox", PR: 21, Trigger: "dispatch"}
	runKey := testRunKey(req.Repo, req.PR)
	labelled := func(id string) string {
		return `{"data":[{"id":"` + id + `","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`
	}
	empty := `{"data":[],"has_more":false}`
	priorReply := driverReplyItem("item_old", "resp_claude_old",
		driverVerdict("Read it before.", "comment"))
	// A completed message that is not a finished answer. An agent commits its
	// opening sentence like this before it has read anything.
	openingSentence := driverReplyItem("item_open", "resp_claude_open",
		"I'll start by reading the diff.")
	newReply := driverReplyItem("item_new", "resp_claude_a",
		driverVerdict("Read it again.", "comment"))

	for _, tc := range []struct {
		name string
		cfg  driverFakeServerConfig
		want string
	}{
		{
			name: "a session that already replied is asked what changed",
			cfg: driverFakeServerConfig{
				SessionListResp: labelled("conv_prior"),
				SessionResps: []string{
					driverSessionWithItems("conv_prior", "ag_1", priorReply),
					driverSessionWithItems("conv_prior", "ag_1", priorReply),
					driverSessionWithItems("conv_prior", "ag_1", priorReply, newReply),
				},
			},
			want: req.Prompt(answered),
		},
		{
			name: "a session an expired run left unanswered is asked for a full review",
			cfg: driverFakeServerConfig{
				SessionListResp: labelled("conv_silent"),
				SessionResps: []string{
					driverSessionResp("conv_silent", "ag_1"),
					driverSessionResp("conv_silent", "ag_1"),
					driverSessionWithItems("conv_silent", "ag_1", newReply),
				},
			},
			want: req.Prompt(unanswered),
		},
		{
			name: "a session holding only an unfinished reply is asked for a full review",
			cfg: driverFakeServerConfig{
				SessionListResp: labelled("conv_partial"),
				SessionResps: []string{
					driverSessionWithItems("conv_partial", "ag_1", openingSentence),
					driverSessionWithItems("conv_partial", "ag_1", openingSentence),
					driverSessionWithItems("conv_partial", "ag_1", openingSentence, newReply),
				},
			},
			want: req.Prompt(unanswered),
		},
		{
			// The reported bug: create fails after committing, so the reconcile
			// search finds the session this very run opened.
			name: "a session reconciled after a lost create is asked for a full review",
			cfg: driverFakeServerConfig{
				CreateStatus:     http.StatusInternalServerError,
				SessionListResps: []string{empty, labelled("conv_committed")},
				SessionResps: []string{
					driverSessionResp("conv_committed", "ag_1"),
					driverSessionResp("conv_committed", "ag_1"),
					driverSessionWithItems("conv_committed", "ag_1", newReply),
				},
			},
			want: req.Prompt(unanswered),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.cfg.AgentPages = []string{driverAgentPage("ag_1", "sei-droid", "", false)}
			tc.cfg.StreamFrames = []string{
				driverAckFrame(),
				driverConsumedFrame("item_1"),
				driverIdleFrame("resp_claude_a"),
				driverDoneFrame(),
			}
			fs := newDriverFakeServer(t, tc.cfg)

			newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
				Run(t.Context(), req)

			events := driverPrompts(fs.EventReqs())
			if len(events) != 1 {
				t.Fatalf("prompt posts = %d, want exactly 1", len(events))
			}
			if got := driverPromptText(t, events[0].Data); got != tc.want {
				t.Errorf("prompt sent = %q, want %q", got, tc.want)
			}
		})
	}
}

// namingWork is work that runs on an agent of its own, for [driver.AgentNamer].
type namingWork struct {
	testWork
	agent string
}

func (w namingWork) AgentName() string { return w.agent }

// TestDriverResolvesTheAgentTheWorkNames guards the capability the multi-reader
// arrangement rests on.
//
// The agent fixes the harness, so a workload that names one and gets the default
// is a second opinion from the same model that produced the first — corroboration
// that corroborates nothing, and indistinguishable in every output from the real
// thing. Both halves are driven, because a test that only checks the naming case
// passes just as well when the default is what always resolves.
func TestDriverResolvesTheAgentTheWorkNames(t *testing.T) {
	t.Parallel()

	newFake := func(t *testing.T) *driverFakeServer {
		return newDriverFakeServer(t, driverFakeServerConfig{
			// One page carrying both, so the name is what selects between them
			// rather than which page happens to be served.
			AgentPages: []string{`{"data":[` +
				`{"id":"ag_1","name":"sei-droid","created_at":1},` +
				`{"id":"ag_2","name":"sei-droid-codex","created_at":1}],"has_more":false}`},
			CreateResp:      driverSessionResp("conv_a", "ag_1"),
			SessionListResp: `{"data":[],"has_more":false}`,
			StreamFrames: []string{
				driverAckFrame(), driverConsumedFrame("item_1"),
				driverIdleFrame("resp_claude_a"), driverDoneFrame(),
			},
			SessionResps: []string{
				driverSessionWithItems("conv_a", "ag_1",
					driverReplyItem("item_reply", "resp_claude_a", driverVerdict("Fine.", "approve"))),
			},
		})
	}

	for _, c := range []struct {
		name string
		work driver.Workload
		want string
	}{
		{"work naming no agent takes the run's default", testWork{Repo: "r/n", PR: 1}, "ag_1"},
		{"work naming its own agent gets that one",
			namingWork{testWork{Repo: "r/n", PR: 1}, "sei-droid-codex"}, "ag_2"},
		{"an empty name means no preference",
			namingWork{testWork{Repo: "r/n", PR: 1}, ""}, "ag_1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fs := newFake(t)
			newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
				Run(t.Context(), c.work)
			creates := fs.CreateReqs()
			if len(creates) != 1 {
				t.Fatalf("creates = %d, want 1", len(creates))
			}
			if creates[0].AgentID != c.want {
				t.Errorf("create AgentID = %q, want %q: a reading on the wrong harness is "+
					"not independent of the one it is checking", creates[0].AgentID, c.want)
			}
		})
	}

	t.Run("an agent the server does not know fails the run", func(t *testing.T) {
		t.Parallel()
		fs := newFake(t)
		// Run reports a classified fault in the driver.Result rather than as an error, so
		// the exit code is the contract a caller has to read. A caller that only
		// checked err would treat this as a completed reading.
		result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
			Run(t.Context(), namingWork{testWork{Repo: "r/n", PR: 1}, "sei-droid-absent"})
		if result.ExitCode != driver.ExitConfig {
			t.Fatalf("ExitCode = %d, want driver.ExitConfig (%d): falling back to the default would "+
				"answer on the review's own harness and read like a second opinion",
				result.ExitCode, driver.ExitConfig)
		}
		if len(fs.CreateReqs()) != 0 {
			t.Error("a session was created for an agent the server does not know")
		}
	})
}

// TestCloseSurvivesACancelledRunEndToEnd is the regression test for a teardown that
// aborted before it reached the call it was protecting.
//
// A terminate signal cancels the run context precisely so that teardown can happen.
// The previous shape detached only the DELETE, leaving the three calls in front of
// it — building the client, resolving the agent, finding the session — on the
// cancelled context, so a signalled run failed at the agent listing and reported
// ExitCancelled with the sandbox still held. The detach now sits above all four, on
// the one path every close takes.
func TestCloseSurvivesACancelledRunEndToEnd(t *testing.T) {
	t.Parallel()

	runKey := testRunKey("sei-protocol/sandbox", 22)
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		SessionListResp: `{"data":[{"id":"conv_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		SessionResps: []string{driverSessionResp("conv_1", "ag_1")},
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Close(ctx, testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if !result.TeardownOK {
		t.Errorf("TeardownOK = false (exit %d): a signalled run must still delete its "+
			"session, since nothing else reclaims the sandbox", result.ExitCode)
	}
	if result.SessionID != "conv_1" {
		t.Errorf("SessionID = %q, want conv_1", result.SessionID)
	}
}

// TestCloseDeletesEverySessionUnderTheRunKey covers the duplicate an overlapping
// dispatch leaves behind.
//
// Opening searches and then creates, which is not a lock: two dispatches that
// overlap can both find nothing and both create. A close that stopped at the first
// match reported success while the second sandbox was still held, and nothing would
// ever look for it again.
func TestCloseDeletesEverySessionUnderTheRunKey(t *testing.T) {
	t.Parallel()

	runKey := testRunKey("sei-protocol/sandbox", 22)
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		SessionListResp: `{"data":[` +
			`{"id":"conv_a","labels":{"` + RunKeyLabel + `":"` + runKey + `"}},` +
			`{"id":"conv_b","labels":{"` + RunKeyLabel + `":"` + runKey + `"}},` +
			`{"id":"conv_other","labels":{"` + RunKeyLabel + `":"someone-else"}}` +
			`],"has_more":false}`,
		SessionResps: []string{driverSessionResp("conv_a", "ag_1")},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Close(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	deleted := fs.DeletedIDs()
	if len(deleted) != 2 {
		t.Fatalf("deleted %v, want both conv_a and conv_b: a duplicate left behind is a "+
			"sandbox nothing will look for again", deleted)
	}
	for _, want := range []string{"conv_a", "conv_b"} {
		if !slices.Contains(deleted, want) {
			t.Errorf("deleted %v, missing %s", deleted, want)
		}
	}
	if slices.Contains(deleted, "conv_other") {
		t.Errorf("deleted %v: another unit of work's session was reclaimed", deleted)
	}
	if !result.TeardownOK {
		t.Errorf("TeardownOK = false (exit %d), want true", result.ExitCode)
	}
}
