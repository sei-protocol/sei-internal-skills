package omni

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// restartWaitLog is what answerPending writes before each wait on a server that
// is being replaced.
const restartWaitLog = "the server is unreachable; waiting for it to come back"

// restartSessionConfig is a turn that spans a server replacement: the prompt lands,
// the stream drops mid-turn, and the turn ends on the stream after the server
// returns. unavailable is how many session reads reach only the gateway.
func restartSessionConfig(unavailable int) driverFakeServerConfig {
	return driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_restart", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			// No done sentinel: the server went away mid-turn.
		},
		LaterStreamFrames: []string{
			driverAckFrame(),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionGatewayUnavailable: unavailable,
		SessionResps: []string{
			driverRunningSessionResp("conv_restart", "ag_1", "resp_claude_a",
				driverReplyItem("item_narration", "resp_claude_a", "I'll read the diff.")),
			driverSessionWithItems("conv_restart", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("The review, written across a restart.", "comment"))),
		},
	}
}

// TestATurnSurvivesTheServerBeingReplaced is platform#1831's failure. The server
// was replaced under a live review: the stream dropped, the next read of the
// session reached only the gateway, and its 503 ended the run with no verdict.
// The sandbox and the turn survived, so the answer was there to collect once the
// new server came up, and a re-run collected it.
func TestATurnSurvivesTheServerBeingReplaced(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, restartSessionConfig(3))
	cfg := driverTestConfig(t, fs.URL)
	cfg.ServerRestartBudget = 2 * time.Second

	log, sink := driverCapturingLogger()
	result := newTestDriver(cfg, driver.Policy{}, log).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 61, Trigger: "t-restart"})

	if result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK: the server came back inside the "+
			"budget, so the turn's answer was there to collect", result.ExitCode)
	}
	if got := fs.GatewayUnavailableHits(); got != 3 {
		t.Errorf("gateway 503s served = %d, want 3: the run has to read through all of them", got)
	}
	if !strings.Contains(sink.String(), restartWaitLog) {
		t.Errorf("no %q line: an operator reading the run cannot tell it waited on a restart",
			restartWaitLog)
	}
	if result.Reply == nil || !strings.Contains(result.Reply.Text, "written across a restart") {
		t.Errorf("Reply = %+v, want the review the turn finished after the restart", result.Reply)
	}
	if got := len(driverPrompts(fs.EventReqs())); got != 1 {
		t.Errorf("prompt posts = %d, want 1: waiting out a restart must not re-send the prompt", got)
	}
}

// TestARestartLongerThanTheBudgetStillEndsTheRun pins the other edge. A server
// that stays gone past the budget is an outage, not a restart, and the run has to
// end on it with the transport exit, which a caller re-runs, rather than hold the
// sandbox for the whole run deadline.
func TestARestartLongerThanTheBudgetStillEndsTheRun(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, restartSessionConfig(1000))
	cfg := driverTestConfig(t, fs.URL)
	cfg.ServerRestartBudget = 400 * time.Millisecond

	log, sink := driverCapturingLogger()
	started := time.Now()
	result := newTestDriver(cfg, driver.Policy{}, log).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 62, Trigger: "t-outage"})
	elapsed := time.Since(started)

	if result.ExitCode != driver.ExitTransport {
		t.Errorf("ExitCode = %d, want driver.ExitTransport: a server gone past the "+
			"budget must end the run the way an unreachable server always has", result.ExitCode)
	}
	if elapsed >= cfg.RunDeadline {
		t.Errorf("run took %v, want well under the %v run deadline: the budget, not the "+
			"deadline, has to bound the wait", elapsed, cfg.RunDeadline)
	}
	if n := strings.Count(sink.String(), restartWaitLog); n > restartPolls {
		t.Errorf("waited %d times, want at most %d: the cadence divides the budget",
			n, restartPolls)
	}
}

// TestOnlyAGatewaySpeakingForTheServerIsRetried pins the classifier. The line it
// draws is the whole safety argument: a response the server wrote is final, and
// only a gateway's word that the server is gone is not.
func TestOnlyAGatewaySpeakingForTheServerIsRetried(t *testing.T) {
	t.Parallel()

	transport := &url.Error{Op: "Get", URL: "https://example.invalid", Err: errors.New("connection refused")}
	for _, tc := range []struct {
		name        string
		err         error
		gateway     bool
		unreachable bool
	}{
		{"gateway 503, no envelope", &omnigent.APIError{StatusCode: 503, Body: []byte("no healthy upstream")}, true, true},
		{"gateway 502", &omnigent.APIError{StatusCode: 502}, true, true},
		{"gateway 504", &omnigent.APIError{StatusCode: 504}, true, true},
		{"wrapped gateway 503", fmt.Errorf("reading: %w", &omnigent.APIError{StatusCode: 503}), true, true},
		{"503 the server wrote", &omnigent.APIError{StatusCode: 503, RequestID: "req_1"}, false, false},
		{"runner_unavailable", &omnigent.APIError{StatusCode: 503, Code: "runner_unavailable", RequestID: "req_1"}, false, false},
		{"503 with a titled failure", &omnigent.APIError{StatusCode: 503, Title: "Host offline"}, false, false},
		{"503 with a detail envelope", &omnigent.APIError{StatusCode: 503, Detail: json.RawMessage(`"draining"`)}, false, false},
		{"500 with no envelope", &omnigent.APIError{StatusCode: 500}, false, false},
		{"404 with no envelope", &omnigent.APIError{StatusCode: 404}, false, false},
		{"transport failure", transport, false, true},
		{"spent deadline", context.DeadlineExceeded, false, false},
		{"nil", nil, false, false},
	} {
		if got := gatewayUnavailable(tc.err); got != tc.gateway {
			t.Errorf("%s: gatewayUnavailable = %t, want %t", tc.name, got, tc.gateway)
		}
		if got := serverUnreachable(tc.err); got != tc.unreachable {
			t.Errorf("%s: serverUnreachable = %t, want %t", tc.name, got, tc.unreachable)
		}
	}
}
