package omni

import (
	"testing"
	"time"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// TestAZeroTimeoutDoesNotDiscardAPaidForAnswer covers a Config that did not come
// through driver.LoadConfig.
//
// Config is exported with exported fields and New takes one, so nothing forces a
// caller through LoadConfig. A zero RequestTimeout is not a short timeout, it is an
// expired one: the read that collects the agent's answer fails before it is issued.
// The run still resolves the agent, launches the sandbox, sends the prompt and waits
// out the turn -- so the review is written and paid for, and then thrown away. It
// exits as a transport fault, which a caller retries, so each retry buys the same
// review and discards it again.
func TestAZeroTimeoutDoesNotDiscardAPaidForAnswer(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_zero", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(), driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"), driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_zero", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read the diff.", "approve"))),
		},
	})

	// Everything the driver needs, and not one timeout -- the shape a caller
	// assembling a Config by hand produces.
	cfg := driver.Config{
		BaseURL: fs.URL, Origin: "test-origin", Agent: "seidroid", Token: "test-token",
		RunDeadline: 10 * time.Second,
	}
	req := testWork{Repo: "sei-protocol/sandbox", PR: 32, Trigger: "trigger-zero"}

	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).Run(t.Context(), req)
	if result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK: the turn finished and its reply "+
			"was committed, so a timeout the caller never set must not discard it",
			result.ExitCode)
	}
	if result.Reply == nil || result.Reply.Text == "" {
		t.Error("Reply is empty: the answer was collected and then dropped")
	}
}

// TestNewSubstitutesEveryNonPositiveTimeout pins each field separately, since one
// field defaulting is not evidence the others do.
func TestNewSubstitutesEveryNonPositiveTimeout(t *testing.T) {
	t.Parallel()

	h := New(driver.Config{
		RequestTimeout: 0, UnaryTimeout: -1 * time.Second, StreamIdleTimeout: 0,
	}, driver.Policy{}, driverTestLogger())

	for _, tc := range []struct {
		name string
		got  interface{ Seconds() float64 }
		want interface{ Seconds() float64 }
	}{
		{"RequestTimeout", h.cfg.RequestTimeout, driver.DefaultRequestTimeout},
		{"UnaryTimeout", h.cfg.UnaryTimeout, driver.DefaultUnaryTimeout},
		{"StreamIdleTimeout", h.cfg.StreamIdleTimeout, driver.DefaultStreamIdleTimeout},
	} {
		if tc.got.Seconds() != tc.want.Seconds() {
			t.Errorf("%s = %vs, want the documented default %vs",
				tc.name, tc.got.Seconds(), tc.want.Seconds())
		}
	}
}
