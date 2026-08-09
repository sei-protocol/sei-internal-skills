package driver

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
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

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

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

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), testWork{
			Repo: "sei-protocol/sandbox", PR: 13, Trigger: "fresh"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if created := fs.CreateReqs(); len(created) != 1 {
		t.Fatalf("created %d sessions, want exactly 1", len(created))
	}
	if result.ExitCode != ExitOK {
		t.Errorf("ExitCode = %d, want ExitOK", result.ExitCode)
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

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs.CreateReqs()) != 0 {
		t.Error("created a session despite a matching label further down the page")
	}
	if result.SessionID != "conv_mine" {
		t.Errorf("SessionID = %q, want conv_mine", result.SessionID)
	}
}

// TestSIGTERMEndsTheRunWithoutTouchingTheSession runs the real binary and
// signals it, because the property belongs to the process rather than to a
// function: main routes the signal into context cancellation, and what a
// preempted run must not do is take the session with it.
//
// This path runs whenever two dispatches overlap, since a newer `seidroid
// xreview` cancels the one in flight. The conversation belongs to the pull
// request rather than to the run, so the next invocation has to find it intact.
//
// The fake server holds the stream open so the process is genuinely mid-run when
// the signal lands.
func TestSIGTERMEndsTheRunWithoutTouchingTheSession(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:      []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		CreateResp:      driverSessionResp("conv_new", "ag_1"),
		SessionListResp: `{"data":[],"has_more":false}`,
		// Ack and open the turn, then nothing: the stream stays open, so the
		// driver is waiting when the signal arrives.
		StreamFrames: []string{driverAckFrame(), driverCreatedFrame()},
	})

	bin := t.TempDir() + "/xreview"
	build := exec.Command("go", "build", "-o", bin, "../../cmd/sei-agent-driver")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the binary: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "xreview", "sei-protocol/sandbox", "15")
	cmd.Env = append(os.Environ(),
		"OMNIGENT_BASE_URL="+fs.URL,
		"OMNIGENT_API_TOKEN=test-token",
		"XREVIEW_RUN_DEADLINE_S=120",
		"GITHUB_RUN_ID=sigterm-test",
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the binary: %v", err)
	}

	// Wait until the session exists, so the signal lands after there is
	// something to tear down rather than during startup.
	deadline := time.Now().Add(20 * time.Second)
	for len(fs.CreateReqs()) == 0 {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("the driver never created a session; stderr:\n%s", stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signalling: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("the process ignored SIGTERM; stderr:\n%s", stderr.String())
	}

	// Neither verb reaches the session. A stop would cost the next invocation a
	// fresh runner and rebuilt transcript for no saving, since the sandbox bills
	// either way; a delete would discard the conversation that invocation exists
	// to continue.
	if got := driverStops(fs.EventReqs()); got != 0 {
		t.Errorf("SIGTERM stopped the session (%d posts); it must be left running "+
			"for the next invocation; stderr:\n%s", got, stderr.String())
	}
	if fs.DeleteHits() != 0 {
		t.Errorf("SIGTERM deleted the session; the conversation must survive for the "+
			"next invocation (deletes = %d)", fs.DeleteHits())
	}
}

// TestDeleteSessionReportsAFailedDelete is the only path that still produces
// ExitTeardownLeak. A refused delete has to reach the exit code rather than the
// logs, because the run reads as successful otherwise and nothing reclaims the
// sandbox afterwards.
func TestDeleteSessionReportsAFailedDelete(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 23}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		SessionListResp: `{"data":[{"id":"conv_stuck","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		DeleteStatus: http.StatusInternalServerError,
	})

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		DeleteSession(context.Background(), req)
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if result.ExitCode != ExitTeardownLeak {
		t.Errorf("ExitCode = %d, want ExitTeardownLeak (%d): a refused delete has to be "+
			"reported, not swallowed", result.ExitCode, ExitTeardownLeak)
	}
	if result.TeardownOK {
		t.Error("TeardownOK = true, want false: the server refused the delete")
	}
	if result.SessionID != "conv_stuck" {
		t.Errorf("SessionID = %q, want conv_stuck: the leaked session has to be nameable",
			result.SessionID)
	}
}

// TestDeleteSessionDestroysTheSession covers the close path, which is the
// only thing that reclaims a sandbox: a review deletes nothing, so the pod runs
// from the first review until this does.
func TestDeleteSessionDestroysTheSession(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 21}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		SessionListResp: `{"data":[{"id":"conv_close","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
	})

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		DeleteSession(context.Background(), req)
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if result.ExitCode != ExitOK {
		t.Errorf("ExitCode = %d, want ExitOK", result.ExitCode)
	}
	if got := fs.DeletedIDs(); len(got) != 1 || got[0] != "conv_close" {
		t.Errorf("deleted = %v, want [conv_close]", got)
	}
}

// TestDeleteSessionIsQuietWhenThereIsNoSession keeps a pull request that was
// closed without ever being reviewed from reading as a failure.
func TestDeleteSessionIsQuietWhenThereIsNoSession(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:      []string{driverAgentPage("ag_1", "sei-droid", "", false)},
		SessionListResp: `{"data":[],"has_more":false}`,
	})

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		DeleteSession(context.Background(), testWork{Repo: "sei-protocol/sandbox", PR: 22})
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if result.ExitCode != ExitOK {
		t.Errorf("ExitCode = %d, want ExitOK for a PR with no session", result.ExitCode)
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

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
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

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

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

	if _, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if created := fs.CreateReqs(); len(created) != 0 {
		t.Errorf("created %d sessions, want 0: a resumable host is woken, not replaced", len(created))
	}
	if got := fs.DeleteHits(); got != 0 {
		t.Errorf("DELETE calls = %d, want 0: deleting a resumable session discards the "+
			"conversation it exists to keep", got)
	}
}
