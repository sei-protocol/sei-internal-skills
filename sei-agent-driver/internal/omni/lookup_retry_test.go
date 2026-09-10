package omni

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// TestTheRunKeyLookupSurvivesAConnectionThatDies is the failure the run key exists
// to absorb, on the path that had no retry: the listing the open searches on dies
// below HTTP, and the run ends with neither an adopted session nor a created one.
// A dispatch that hits it reports no verdict and a person retypes the request.
func TestTheRunKeyLookupSurvivesAConnectionThatDies(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 31, Trigger: "dropped-listing"}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:       []string{driverAgentPage("ag_1", "seidroid", "", false)},
		CreateResp:       driverSessionResp("conv_new", "ag_1"),
		SessionListDrops: 1,
		SessionListResp: `{"data":[{"id":"conv_prior","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionWithItems("conv_prior", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read the diff again.", "approve"))),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), req)

	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK — the second attempt reached the server",
			result.ExitCode)
	}
	if result.SessionID != "conv_prior" {
		t.Errorf("SessionID = %q, want conv_prior — the retry has to find what the first attempt could not",
			result.SessionID)
	}
	if len(fs.CreateReqs()) != 0 {
		t.Error("created a session; a lookup that failed must not read as an empty search")
	}
}

// TestTheRunKeyLookupSurvivesAListingThatHangs is the same failure arriving
// slowly. A blackholed flow answers nothing rather than resetting, so the
// attempt ends on the walk's own budget -- a deadline, which is the one shape a
// transport classifier has to refuse on its face. The walk budget is a share of
// what the caller still has, so spending one leaves the caller another.
func TestTheRunKeyLookupSurvivesAListingThatHangs(t *testing.T) {
	t.Parallel()

	req := testWork{Repo: "sei-protocol/sandbox", PR: 33, Trigger: "hung-listing"}
	runKey := testRunKey(req.Repo, req.PR)

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:        []string{driverAgentPage("ag_1", "seidroid", "", false)},
		CreateResp:        driverSessionResp("conv_new", "ag_1"),
		SessionListStalls: 1,
		SessionListResp: `{"data":[{"id":"conv_prior","agent_id":"ag_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionResp("conv_prior", "ag_1"),
			driverSessionWithItems("conv_prior", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read the diff again.", "approve"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	// Prices the hang: the walk budget it buys is what the first attempt spends
	// before the second gets to run.
	cfg.RequestTimeout = 200 * time.Millisecond

	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).Run(t.Context(), req)

	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK — the hang costs one walk, not the run",
			result.ExitCode)
	}
	if result.SessionID != "conv_prior" {
		t.Errorf("SessionID = %q, want conv_prior", result.SessionID)
	}
	if len(fs.CreateReqs()) != 0 {
		t.Error("created a session; a lookup that hung must not read as an empty search")
	}
}

// TestTheRunKeyLookupGivesUp bounds the retry. A server that never answers costs
// the attempts and no more, so a run reports the transport failure rather than
// spending its deadline on it.
func TestTheRunKeyLookupGivesUp(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:       []string{driverAgentPage("ag_1", "seidroid", "", false)},
		CreateResp:       driverSessionResp("conv_new", "ag_1"),
		SessionListDrops: transportAttempts + 1,
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{
			Repo: "sei-protocol/sandbox", PR: 32, Trigger: "dead-listing"})

	if result.ExitCode != driver.ExitTransport {
		t.Errorf("ExitCode = %d, want driver.ExitTransport", result.ExitCode)
	}
	// The attempts the lookup is allowed, plus the one the close sweep spends
	// reclaiming whatever the failed open may have left behind.
	if want, hits := transportAttempts+1, fs.ListSessionHits(); hits != want {
		t.Errorf("the listing was asked %d times, want %d", hits, want)
	}
}

// TestOnlyAnUnreachedRequestIsRetried pins what transportFailed admits. A status
// the server chose cannot be improved by asking again, and a spent context has
// nothing left to ask with.
func TestOnlyAnUnreachedRequestIsRetried(t *testing.T) {
	t.Parallel()

	dialFailed := &url.Error{Op: "Get", URL: "https://omni.invalid/v1/sessions",
		Err: errors.New("connection reset by peer")}
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"a connection that died", dialFailed, true},
		{"a wrapped one", errors.Join(errors.New("looking for a session"), dialFailed), true},
		{"the server refusing", &omnigent.APIError{StatusCode: 503}, false},
		{"a cancelled run", context.Canceled, false},
		{"a spent budget", &url.Error{Op: "Get", Err: context.DeadlineExceeded}, false},
		{"no error", nil, false},
	} {
		if got := transportFailed(tc.err); got != tc.want {
			t.Errorf("transportFailed(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
