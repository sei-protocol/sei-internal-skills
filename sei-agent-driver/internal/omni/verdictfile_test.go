package omni

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// TestVerdictFileIsWrittenOnlyForAStructuredVerdict guards the gate the calling
// workflow depends on.
//
// That workflow decides whether to post by whether the file exists and is
// non-empty, deliberately not by the exit code — a non-zero exit can still carry
// a good review, a teardown leak being the obvious case. So the file's presence
// is the whole contract, and it has to mean the same thing the driver means by a
// verdict.
//
// The failure this pins is not hypothetical. A turn that trails off ends with
// prose and no fenced block: the driver reports driver.ExitNoVerdict but keeps the text,
// on purpose, so a human reading stdout has something to look at. Gating the file
// on non-empty text instead of on a verdict routed that prose to the file, where
// the workflow would upsert it over a previous good structured review.
//
// It runs the real binary because the gate lives in main's report(), and a test
// of the driver alone would not touch it.
func TestVerdictFileIsWrittenOnlyForAStructuredVerdict(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	t.Parallel()

	bin := filepath.Join(t.TempDir(), "xreview")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/sei-agent-driver").
		CombinedOutput(); err != nil {
		t.Fatalf("building the binary: %v\n%s", err, out)
	}

	tests := []struct {
		name string
		// reply is the assistant text the fake turn commits.
		reply string
		// wantFile is whether --out should exist afterwards.
		wantFile bool
		wantExit int
	}{
		{
			name:     "a structured verdict is written",
			reply:    "here is my review\n```json\n{\"decision\":\"approve\"}\n```",
			wantFile: true,
			wantExit: driver.ExitOK,
		},
		{
			name: "prose with no fenced block is not written",
			reply: "I read the diff and it looks reasonable, but I am not going to " +
				"commit to a decision.",
			wantFile: false,
			wantExit: driver.ExitNoVerdict,
		},
		{
			name:     "a malformed fenced block is not written",
			reply:    "```json\n{\"decision\": \"approve\",,,}\n```",
			wantFile: false,
			wantExit: driver.ExitNoVerdict,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages:      []string{driverAgentPage("ag_1", "sei-droid", "", false)},
				CreateResp:      driverSessionResp("conv_1", "ag_1"),
				SessionListResp: `{"data":[],"has_more":false}`,
				StreamFrames: []string{
					driverAckFrame(),
					driverConsumedFrame("item_1"),
					driverIdleFrame("resp_claude_a"),
					driverDoneFrame(),
				},
				SessionResps: []string{
					driverSessionWithItems("conv_1", "ag_1",
						driverReplyItem("item_reply", "resp_claude_a", tc.reply)),
				},
			})

			out := filepath.Join(t.TempDir(), "verdict.md")
			cmd := exec.Command(bin, "xreview", "sei-protocol/sandbox", "70", "--out", out)
			cmd.Env = append(os.Environ(),
				"OMNIGENT_BASE_URL="+fs.URL,
				"OMNIGENT_API_TOKEN=test-token",
				"XREVIEW_RUN_DEADLINE_S=60",
				"GITHUB_RUN_ID=verdictfile",
				"GITHUB_RUN_ATTEMPT="+string(rune('a'+i)),
			)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			runErr := cmd.Run()

			exit := 0
			var ee *exec.ExitError
			if runErr != nil {
				if !errors.As(runErr, &ee) {
					t.Fatalf("running the binary: %v\n%s", runErr, stderr.String())
				}
				exit = ee.ExitCode()
			}
			if exit != tc.wantExit {
				t.Errorf("exit = %d, want %d\nstderr:\n%s", exit, tc.wantExit, stderr.String())
			}

			body, statErr := os.ReadFile(out)
			if tc.wantFile && statErr == nil {
				// The footer is the only provenance record that outlives the run's
				// logs, so a body without one is not publishable.
				for _, want := range []string{"resp_claude_a", "item_reply", "conv_1"} {
					if !strings.Contains(string(body), want) {
						t.Errorf("the written body does not name %s; the comment must carry "+
							"its own provenance:\n%s", want, body)
					}
				}
			}
			switch {
			case tc.wantFile && statErr != nil:
				t.Errorf("--out was not written for a structured verdict: %v", statErr)
			case !tc.wantFile && statErr == nil:
				t.Errorf("--out was written for an unstructured turn; the caller posts on "+
					"file presence, so this would upsert unparsed prose over a good review.\n"+
					"content: %q", string(body))
			}
		})
	}
}
