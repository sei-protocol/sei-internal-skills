package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/review"
)

// TestParseScouts guards the two ways a scout list goes wrong quietly.
//
// A malformed entry must not be skipped: the review would publish having weighed
// fewer opinions than it was configured to hear, and say nothing about it. Two
// scouts under one name must not be accepted: they derive the same run key, so the
// second adopts the first's session and reports its findings back as a second
// reading of the same pull request.
func TestParseScouts(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name    string
		raw     string
		want    []scoutSpec
		wantErr bool
	}{
		{name: "none configured runs the review alone", raw: "  "},
		{
			name: "dispatch order is preserved",
			raw:  "codex=seidroid-codex, cursor=seidroid-cursor",
			want: []scoutSpec{
				{name: "codex", agent: "seidroid-codex"},
				{name: "cursor", agent: "seidroid-cursor"},
			},
		},
		{name: "missing agent", raw: "codex=", wantErr: true},
		{name: "missing name", raw: "=seidroid-codex", wantErr: true},
		{name: "no separator", raw: "codex", wantErr: true},
		{name: "duplicate name collides on the run key", raw: "codex=a,codex=b", wantErr: true},
		// The bundle fixes the harness, so both of these produce a reading that is
		// not independent of the thing it is meant to check.
		{name: "scout on the review's own agent", raw: "codex=seidroid", wantErr: true},
		{name: "two scouts on one agent", raw: "codex=a,cursor=a", wantErr: true},
	} {
		got, err := parseScouts(c.raw, "seidroid")
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: parseScouts(%q) succeeded; a silently dropped scout makes a "+
					"thinner review look like a full one", c.name, c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: parseScouts(%q) = %v", c.name, c.raw, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d scouts, want %d", c.name, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: scout %d = %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// TestParseTargetRefusesAnythingShellShaped pins the entry point against the
// same class the guidelines filename is already checked for. The repository name
// is written into the diff fetch, the clone, the standards read and the intent
// read — four commands the prompt tells the agent to run — so a name carrying a
// substitution would end the argument and start something else.
func TestParseTargetRefusesAnythingShellShaped(t *testing.T) {
	t.Parallel()

	for _, repo := range []string{
		"o/r$(env|base64)",
		"o/r;id",
		"o/r`id`",
		"o/r&&whoami",
		"o/r|tee /tmp/x",
		"o/r\nname",
		"o/r ",
		`o/r"`,
	} {
		if _, _, err := parseTarget([]string{repo, "1"}); err == nil {
			t.Errorf("parseTarget accepted %q", repo)
		}
	}
}

// TestParseTargetKeepsEveryNameGitHubAllows guards the other direction: the
// check must not refuse a repository someone actually has.
func TestParseTargetKeepsEveryNameGitHubAllows(t *testing.T) {
	t.Parallel()

	for _, repo := range []string{
		"sei-protocol/sei-chain",
		"sei-protocol/sei-internal-skills",
		"o/r.with.dots",
		"o/r_with_underscores",
		"Owner123/Repo456",
	} {
		if _, _, err := parseTarget([]string{repo, "1"}); err != nil {
			t.Errorf("parseTarget refused %q: %v", repo, err)
		}
	}
}

// TestReportWritesEachOutputOnItsOwnFlag pins that the check run and the findings
// no longer depend on --out. A checks list with no review entry reads as a
// review that did not run rather than one that passed, so gating the fail-closed
// signal on an unrelated flag made it fail open.
func TestReportWritesEachOutputOnItsOwnFlag(t *testing.T) {
	dir := t.TempDir()
	check := filepath.Join(dir, "check.json")

	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text:   "A review.\n\n```json\n{\"decision\":\"comment\",\"summary\":\"s\"}\n```",
		TurnID: "t1", ItemID: "i1",
	}}
	if err := report("", "", check, result, false); err != nil {
		t.Fatalf("report: %v", err)
	}
	if _, err := os.Stat(check); err != nil {
		t.Errorf("the check run was not written without --out: %v", err)
	}
}

// TestReportClearsAnEarlierRunsOutputs covers a reused workspace. The caller
// publishes on the file being present, so a previous run's verdict left on disk
// is published as this one's.
func TestReportClearsAnEarlierRunsOutputs(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "verdict.md")
	if err := os.WriteFile(out, []byte("a previous review"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A run that reached no verdict: nothing to publish.
	if err := report(out, "", "", driver.Result{SessionID: "s2"}, false); err != nil {
		t.Fatalf("report: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		body, _ := os.ReadFile(out)
		t.Errorf("the earlier run's verdict survived and would be published: %q", body)
	}
}

// TestOutputsAreClearedBeforeAnEarlyExit covers a stale verdict outliving its run.
//
// A caller publishes on the file being present, so an output an earlier run left on
// a reused workspace is that run's verdict posted under this one's name. Clearing
// inside report is too late: every path that exits before it -- a missing
// credential, a malformed scout list -- leaves the file where it was.
func TestOutputsAreClearedBeforeAnEarlyExit(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "verdict.md")
	if err := os.WriteFile(out, []byte("a previous run's verdict"), 0o600); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(dir, "sei-agent-driver")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "review", "--out", out, "sei-protocol/sandbox", "22")
	// No credential, so this exits before it reaches the review.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir}
	_ = cmd.Run()

	if _, err := os.Stat(out); !os.IsNotExist(err) {
		body, _ := os.ReadFile(out)
		t.Errorf("the earlier run's output survived a failed run (%q); a caller that "+
			"publishes on file presence would post it as this run's verdict", body)
	}
}

// TestARefusedClearIsFatal covers the other half of the same invariant.
//
// Clearing is what makes "a file is present" mean "this run produced it". A
// removal that fails leaves the earlier run's verdict in place, so the run has to
// refuse rather than continue and let the caller publish it. A non-empty directory
// is the portable way to make os.Remove fail without a read-only mount.
func TestARefusedClearIsFatal(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "verdict.md")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "occupant"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Two more paths after the undeletable one, each holding an earlier run's bytes.
	// The obligation is per file, so a return on the first failure leaves these two
	// where they are and the caller publishes both.
	findings := filepath.Join(dir, "findings.json")
	check := filepath.Join(dir, "check.json")
	for _, p := range []string{findings, check} {
		if err := os.WriteFile(p, []byte("an earlier run's output"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := clearOutputs(out, findings, check)
	if err == nil {
		t.Fatal("clearOutputs succeeded on a path it could not remove; the caller " +
			"would publish whatever was left there")
	}
	if !strings.Contains(err.Error(), out) {
		t.Errorf("error = %v, want it to name the path that could not be cleared", err)
	}
	for _, p := range []string{findings, check} {
		if _, statErr := os.Stat(p); statErr == nil {
			body, _ := os.ReadFile(p)
			t.Errorf("%s survived (%q): a failure on an earlier path stopped this one "+
				"from being attempted, so the caller publishes it as this run's",
				filepath.Base(p), body)
		}
	}

	// An absent path stays not-an-error: most runs have nothing to clear.
	if err := clearOutputs(filepath.Join(dir, "never-written.md")); err != nil {
		t.Errorf("clearOutputs on an absent path = %v, want nil", err)
	}
}

// TestBothCallersActOnARefusedClear covers what clearOutputs returning an error is for.
//
// Testing the function alone leaves the whole point untested: a caller that discards the
// error clears nothing and continues, and the workflow publishes on file presence. There
// are two callers — run, before anything can exit, and report, before anything is
// written — so both are pinned. A non-empty directory is the portable way to make
// os.Remove fail without a read-only mount.
func TestBothCallersActOnARefusedClear(t *testing.T) {
	dir := t.TempDir()

	undeletable := func(t *testing.T, name string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "occupant"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("report refuses", func(t *testing.T) {
		out := undeletable(t, "report-out")
		err := report(out, "", "", driver.Result{SessionID: "s1"}, false)
		if err == nil {
			t.Fatal("report returned nil on an output it could not clear; the caller " +
				"publishes on presence, so an earlier verdict posts as this run's")
		}
		if !strings.Contains(err.Error(), out) {
			t.Errorf("error = %v, want it to name the path", err)
		}
	})

	t.Run("the binary refuses before it reviews", func(t *testing.T) {
		out := undeletable(t, "run-out")
		bin := filepath.Join(dir, "sei-agent-driver")
		if b, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, b)
		}
		// A working credential, so a non-zero exit is about the clear and not about
		// configuration. The clear runs first either way.
		cmd := exec.Command(bin, "review", "--out", out, "sei-protocol/sandbox", "22")
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"), "HOME=" + dir,
			"OMNIGENT_API_TOKEN=test-token",
			"OMNIGENT_BASE_URL=http://127.0.0.1:1",
		}
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()

		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run: %v\n%s", err, stderr.String())
		}
		if ee.ExitCode() != driver.ExitConfig {
			t.Errorf("exit = %d, want driver.ExitConfig (%d): a run that cannot clear "+
				"its outputs has to refuse, not review\nstderr:\n%s",
				ee.ExitCode(), driver.ExitConfig, stderr.String())
		}
		if !strings.Contains(stderr.String(), "clearing an earlier run's output") {
			t.Errorf("stderr does not say the clear failed:\n%s", stderr.String())
		}
	})
}

// TestCheckJSONCarriesTheCounts covers the file the workflow actually reads.
//
// The counts are derived in the driver and consumed by whatever composes the pull
// request comment, and nothing between the two is Go. So the assertion that matters is
// on the bytes: the key is present, it is nested under counts, and its four integers are
// the ones [review.BuildCheckRun] derived. Testing the struct alone would pass on a field
// the marshaller drops.
//
// The findings file is checked against counts.placeable in the same pass, because that is
// the number's whole definition: what the caller was handed to attempt.
func TestCheckJSONCarriesTheCounts(t *testing.T) {
	dir := t.TempDir()
	check := filepath.Join(dir, "check.json")
	findings := filepath.Join(dir, "findings.json")

	block := `{"read":120,"decision":"request_changes","summary":"Two problems.",` +
		`"inline_comments":[` +
		`{"path":"a.go","line":9,"side":"RIGHT","severity":"blocker","body":"nil deref"},` +
		`{"path":"b.go","line":0,"severity":"suggestion","body":"nowhere to put this"}],` +
		`"blockers":["the new path has no test"],` +
		`"non_blockers":["naming could be clearer"],` +
		`"pre_existing_issues":[{"severity":"blocker","body":"b.go:4 leaks a handle"}]}`
	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text:   "A review.\n\n```json\n" + block + "\n```",
		TurnID: "t1", ItemID: "i1",
	}}
	if err := report("", findings, check, result, true); err != nil {
		t.Fatalf("report: %v", err)
	}

	blob, err := os.ReadFile(check)
	if err != nil {
		t.Fatalf("reading the check run: %v", err)
	}
	var got review.CheckRun
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("decoding %s: %v", check, err)
	}
	if got.Counts == nil {
		t.Fatalf("check.json carries no counts:\n%s", blob)
	}
	// Two blocking: the one on a line and the one tied to none. The pre-existing
	// blocker is neither, which is the whole point of the fourth integer.
	want := review.Counts{Blocking: 2, NonBlocking: 2, Placeable: 1, PreExisting: 1}
	if *got.Counts != want {
		t.Errorf("counts = %+v, want %+v", *got.Counts, want)
	}
	if !strings.Contains(string(blob), `"counts"`) {
		t.Errorf("counts is not nested under its own key, so a caller reading "+
			"'.counts | ...' finds nothing:\n%s", blob)
	}

	// The three fields the workflow already reads keep their names and their values, so
	// nothing about this is a schema break for an existing consumer.
	built, ok := review.BuildCheckRun(review.ParseVerdict(result.Reply.Text), true)
	if !ok {
		t.Fatal("no check run for a verdict that decided")
	}
	if got.Conclusion != built.Conclusion || got.Title != built.Title {
		t.Errorf("check.json = %q/%q, want %q/%q",
			got.Conclusion, got.Title, built.Conclusion, built.Title)
	}

	placed, err := os.ReadFile(findings)
	if err != nil {
		t.Fatalf("reading the findings: %v", err)
	}
	var entries []review.Finding
	if err := json.Unmarshal(placed, &entries); err != nil {
		t.Fatalf("decoding %s: %v", findings, err)
	}
	if len(entries) != got.Counts.Placeable {
		t.Errorf("findings.json holds %d entries beside counts.placeable = %d; the "+
			"number is defined as the length of that file", len(entries),
			got.Counts.Placeable)
	}
}
