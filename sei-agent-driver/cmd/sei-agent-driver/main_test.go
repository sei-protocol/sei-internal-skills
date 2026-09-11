package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	if err := report("", "", check, result, verdictOf(result), review.Request{}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if _, err := os.Stat(check); err != nil {
		t.Errorf("the check run was not written without --out: %v", err)
	}
}

// TestReportPublishesAScoutSettledVerdict covers the path that runs no review turn:
// the settled verdict has to travel through the same three files a turn's verdict
// does, or the caller sees a run that exited 0 with nothing to post and reads it as
// a review that never happened.
func TestReportPublishesAScoutSettledVerdict(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "verdict.md")
	findings := filepath.Join(dir, "findings.json")
	check := filepath.Join(dir, "check.json")

	req := review.Request{Scouts: []review.ScoutResult{{Name: "codex", Lines: 12, Inert: true}}}
	verdict, ok := review.SettleByScouts(req)
	if !ok {
		t.Fatal("SettleByScouts refused an inert clean reading")
	}
	result := driver.Result{ExitCode: driver.ExitOK, TeardownOK: true}
	if err := report(out, findings, check, result, verdict, req); err != nil {
		t.Fatalf("report: %v", err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the verdict was not written: %v", err)
	}
	if !strings.Contains(string(body), "settled by the scouts codex") {
		t.Errorf("the published comment does not say the scouts decided it:\n%s", body)
	}
	blob, err := os.ReadFile(check)
	if err != nil {
		t.Fatalf("the check run was not written: %v", err)
	}
	var run review.CheckRun
	if err := json.Unmarshal(blob, &run); err != nil {
		t.Fatalf("check.json: %v", err)
	}
	if run.Conclusion != "success" {
		t.Errorf("Conclusion = %q, want success", run.Conclusion)
	}
	if _, err := os.Stat(findings); !os.IsNotExist(err) {
		t.Errorf("a findings file exists for a verdict that placed nothing: %v", err)
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
	if err := report(out, "", "", driver.Result{SessionID: "s2"}, verdictOf(driver.Result{SessionID: "s2"}), review.Request{}); err != nil {
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
		err := report(out, "", "", driver.Result{SessionID: "s1"}, verdictOf(driver.Result{SessionID: "s1"}), review.Request{})
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
	if err := report("", findings, check, result, verdictOf(result), review.Request{IncludeNits: true}); err != nil {
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

// TestANoVerdictRunStillHandsTheCallerACheckRun covers the run that publishes nothing.
//
// A review that ran and could not be read is indistinguishable, on the pull request,
// from a review that never ran — unless something reaches the checks list. So the
// assertion is on the three files together: the check run is there, and the two files
// that mean "post this review" are not, because a caller decides on their absence.
func TestANoVerdictRunStillHandsTheCallerACheckRun(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "verdict.md")
	findings := filepath.Join(dir, "findings.json")
	check := filepath.Join(dir, "check.json")

	result := driver.Result{
		SessionID: "s1",
		ExitCode:  driver.ExitNoVerdict,
		Reply:     &driver.Reply{Text: "I read the diff and it looks fine to me.", TurnID: "t1"},
	}
	if err := report(out, findings, check, result, verdictOf(result), review.Request{IncludeNits: true}); err != nil {
		t.Fatalf("report: %v", err)
	}

	for _, path := range []string{out, findings} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			blob, _ := os.ReadFile(path)
			t.Errorf("%s exists on a no-verdict run; a caller posts on presence, so "+
				"this is unparsed prose published as a review:\n%s", path, blob)
		}
	}

	blob, err := os.ReadFile(check)
	if err != nil {
		t.Fatalf("reading the check run: %v", err)
	}
	var got review.CheckRun
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("decoding %s: %v", check, err)
	}
	want := review.BuildFailureCheck(review.ParseVerdict(result.Reply.Text))
	if got.Conclusion != want.Conclusion || got.Title != want.Title {
		t.Errorf("check.json = %q/%q, want %q/%q",
			got.Conclusion, got.Title, want.Conclusion, want.Title)
	}
	if got.Summary != want.Summary {
		t.Errorf("summary = %q, want %q", got.Summary, want.Summary)
	}
	// The reason is the actionable half, and the parser's own words for this reply are
	// what name it. A summary that reached the reader without one says only that
	// something went wrong.
	if !strings.Contains(got.Summary, "fenced json block") {
		t.Errorf("the check run does not say why there is no verdict:\n%s", blob)
	}
	// Absent, not zero. A caller reading .counts.blocking off this file must get null:
	// nothing blocking, over a review nobody could read, is a green gate on an unread
	// change.
	if strings.Contains(string(blob), "counts") {
		t.Errorf("check.json carries a counts key on a no-verdict run:\n%s", blob)
	}
}

// TestTheFailureCheckQuotesTheDriversOwnReason pins which of two reasons is published.
//
// A reply refused for carrying a credential parses as ordinary prose, so the parser's
// reason for it — no fenced block — describes the text and not the refusal. The driver's
// reason names the refusal, and it is the one an operator has to act on. stdout and the
// check run have to agree on it, because the same person reads both.
func TestTheFailureCheckQuotesTheDriversOwnReason(t *testing.T) {
	dir := t.TempDir()
	check := filepath.Join(dir, "check.json")

	const refusal = "the reply looks like it carries a github token"
	result := driver.Result{
		SessionID: "s1",
		ExitCode:  driver.ExitNoVerdict,
		Reply:     &driver.Reply{Text: "here is the diff", Reason: refusal},
	}
	if err := report("", "", check, result, verdictOf(result), review.Request{IncludeNits: true}); err != nil {
		t.Fatalf("report: %v", err)
	}

	blob, err := os.ReadFile(check)
	if err != nil {
		t.Fatalf("reading the check run: %v", err)
	}
	if !strings.Contains(string(blob), refusal) {
		t.Errorf("the check run quotes the parser's reason over the driver's:\n%s", blob)
	}
}

// TestARunWithNoReplyNamesWhyRatherThanBlamingTheReply covers the paths where the turn
// produced nothing at all: a deadline, a transport fault, a failed turn, a bad config.
//
// The check concludes failure on all of them, so under branch protection they hold a
// merge. The summary is the only place an operator can tell an infrastructure fault
// from a review that answered in prose.
func TestARunWithNoReplyNamesWhyRatherThanBlamingTheReply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		exitCode int
		want     string
	}{
		{"timeout", driver.ExitTimeout, "run deadline"},
		{"transport", driver.ExitTransport, "transport"},
		{"turn failed", driver.ExitTurnFailed, "turn as failed"},
		{"config", driver.ExitConfig, "configuration or credential"},
		{"internal", driver.ExitInternal, "this driver failed"},
		{"cancelled", driver.ExitCancelled, "cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			check := filepath.Join(t.TempDir(), "check.json")
			result := driver.Result{SessionID: "s1", ExitCode: tc.exitCode}
			if err := report("", "", check, result, verdictOf(result), review.Request{IncludeNits: true}); err != nil {
				t.Fatalf("report: %v", err)
			}
			blob, err := os.ReadFile(check)
			if err != nil {
				t.Fatalf("reading the check run: %v", err)
			}
			if !strings.Contains(string(blob), tc.want) {
				t.Errorf("summary does not name %q:\n%s", tc.want, blob)
			}
			if strings.Contains(string(blob), "no reply this driver could read") {
				t.Errorf("summary blames a reply that never arrived:\n%s", blob)
			}
		})
	}
}

// TestCheckJSONCarriesTheThreadPlan covers the other half of the file the workflow reads.
//
// The assertion is on the bytes for the reason the counts test gives: nothing between
// this driver and the step that resolves a thread is Go, so a field the marshaller
// dropped would pass a check on the struct and resolve nothing on the pull request.
//
// The refusal is the part that matters. The ids come out of a reply the agent wrote
// after reading a diff whose author is not this tool, and the caller resolves what it is
// handed — so an id that matches no thread this run was told about must reach that caller
// in the refused list and nowhere else.
func TestCheckJSONCarriesTheThreadPlan(t *testing.T) {
	dir := t.TempDir()
	check := filepath.Join(dir, "check.json")

	const ours = "PRRT_kwDOABCDEF4Ax1y2"
	const restated = "PRRT_kwDOABCDEF4Bz3w4"
	block := `{"read":120,"decision":"request_changes","summary":"One left.",` +
		`"resolved_thread_ids":["` + ours + `","PRRT_neverOurs"],` +
		`"inline_comments":[{"path":"a.go","line":9,"side":"RIGHT","severity":"blocker",` +
		`"body":"still a nil deref","supersedes_thread_ids":["` + restated + `"]}]}`
	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text: "A review.\n\n```json\n" + block + "\n```", TurnID: "t1", ItemID: "i1",
	}}
	req := review.Request{IncludeNits: true, PriorThreads: []review.PriorThread{
		{ID: ours, File: "a.go", Line: 4, Body: "a leak"},
		{ID: restated, File: "a.go", Line: 9, Body: "a nil deref"},
	}}

	if err := report("", "", check, result, verdictOf(result), req); err != nil {
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
	if got.Threads == nil {
		t.Fatalf("check.json carries no thread plan:\n%s", blob)
	}
	if len(got.Threads.Addressed) != 1 || got.Threads.Addressed[0] != ours {
		t.Errorf("addressed = %v, want [%s]", got.Threads.Addressed, ours)
	}
	if len(got.Threads.Superseded) != 1 || got.Threads.Superseded[0] != restated {
		t.Errorf("superseded = %v, want [%s]", got.Threads.Superseded, restated)
	}
	if len(got.Threads.Refused) != 1 || got.Threads.Refused[0] != "PRRT_neverOurs" {
		t.Errorf("refused = %v, want [PRRT_neverOurs]", got.Threads.Refused)
	}
	if strings.Contains(string(blob), `"PRRT_neverOurs"`) &&
		!strings.Contains(string(blob), `"refused"`) {
		t.Errorf("an unmatched id reaches the caller outside the refused list:\n%s", blob)
	}
}

// TestFindingsJSONCarriesTheSupersededLinkage covers the file the resolve step reads.
//
// The gate on a superseded thread is per thread, and this is the only thing that makes it
// so: the caller closes a thread on finding its id beside a comment that posted. Asserted
// on the bytes for the reason the plan test gives -- nothing between this driver and that
// step is Go, so a field the marshaller dropped would pass a check on the struct and read
// as a driver that publishes no linkage at all.
//
// Two findings, one replacing a thread and one replacing nothing, because both keys
// matter. The first has to carry the id. The second has to carry no key: whether ANY
// finding in the file carries one is how the caller tells this driver from an older one,
// and an empty array on every finding would answer that question wrongly.
func TestFindingsJSONCarriesTheSupersededLinkage(t *testing.T) {
	dir := t.TempDir()
	findings := filepath.Join(dir, "findings.json")
	check := filepath.Join(dir, "check.json")

	const restated = "PRRT_kwDOABCDEF4Bz3w4"
	block := `{"read":120,"decision":"request_changes","summary":"Two.",` +
		`"inline_comments":[` +
		`{"path":"a.go","line":9,"side":"RIGHT","severity":"blocker",` +
		`"body":"still a nil deref","supersedes_thread_ids":["` + restated + `",` +
		`"PRRT_neverOurs"]},` +
		`{"path":"b.go","line":3,"side":"RIGHT","severity":"suggestion",` +
		`"body":"a finding this review makes for the first time"}]}`
	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text: "A review.\n\n```json\n" + block + "\n```", TurnID: "t1", ItemID: "i1",
	}}
	req := review.Request{IncludeNits: true, PriorThreads: []review.PriorThread{
		{ID: restated, File: "a.go", Line: 9, Body: "a nil deref"},
	}}

	if err := report("", findings, check, result, verdictOf(result), req); err != nil {
		t.Fatalf("report: %v", err)
	}

	blob, err := os.ReadFile(findings)
	if err != nil {
		t.Fatalf("reading the findings: %v", err)
	}
	// Decoded as the raw objects rather than as []review.Finding, because the question
	// is which keys the file carries and a struct field answers that for both.
	var entries []map[string]any
	if err := json.Unmarshal(blob, &entries); err != nil {
		t.Fatalf("decoding %s: %v", findings, err)
	}
	if len(entries) != 2 {
		t.Fatalf("findings.json holds %d entries, want 2:\n%s", len(entries), blob)
	}
	if got := entries[0]["supersedes"]; !reflect.DeepEqual(got, []any{restated}) {
		t.Errorf("the replacing comment carries supersedes = %v, want [%s]; without it "+
			"the caller cannot tell which thread this comment replaced and closes every "+
			"superseded thread together", got, restated)
	}
	if got, named := entries[1]["supersedes"]; named {
		t.Errorf("a finding replacing no thread wrote supersedes = %v; the key's "+
			"presence anywhere in this file is what tells a caller the linkage is "+
			"published, so writing it empty makes an older driver of a newer one", got)
	}
	if strings.Contains(string(blob), "PRRT_neverOurs") {
		t.Errorf("an id matching no thread this run was told about reached the file the "+
			"caller resolves from:\n%s", blob)
	}

	// The plan is the caller's warrant and the linkage is which comment spends it, so the
	// two files have to name the same thread.
	var run review.CheckRun
	checkBlob, err := os.ReadFile(check)
	if err != nil {
		t.Fatalf("reading the check run: %v", err)
	}
	if err := json.Unmarshal(checkBlob, &run); err != nil {
		t.Fatalf("decoding %s: %v", check, err)
	}
	if run.Threads == nil || len(run.Threads.Superseded) != 1 ||
		run.Threads.Superseded[0] != restated {
		t.Errorf("check.json superseded = %+v, want [%s] beside the linkage in "+
			"findings.json", run.Threads, restated)
	}
}

// TestAnOlderCallersCheckJSONPlansNothing covers the workflow that supplies no thread
// ids, which is what every caller does before it learns to.
//
// The review still runs and still publishes. What it cannot do is close a thread, and
// the empty allowlist is what turns every id the reply names into a refusal rather than
// a mutation.
func TestAnOlderCallersCheckJSONPlansNothing(t *testing.T) {
	dir := t.TempDir()
	check := filepath.Join(dir, "check.json")

	block := `{"read":120,"decision":"approve","summary":"Clean.",` +
		`"resolved_thread_ids":["PRRT_kwDOABCDEF4Ax1y2"]}`
	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text: "A review.\n\n```json\n" + block + "\n```", TurnID: "t1",
	}}
	req := review.Request{PriorThreads: []review.PriorThread{
		{File: "a.go", Line: 4, Body: "a leak"},
	}}

	if err := report("", "", check, result, verdictOf(result), req); err != nil {
		t.Fatalf("report: %v", err)
	}

	var got review.CheckRun
	blob, err := os.ReadFile(check)
	if err != nil {
		t.Fatalf("reading the check run: %v", err)
	}
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("decoding %s: %v", check, err)
	}
	if got.Threads == nil {
		t.Fatalf("check.json carries no thread plan:\n%s", blob)
	}
	if len(got.Threads.Addressed) != 0 || len(got.Threads.Superseded) != 0 {
		t.Errorf("plan = %+v; a caller that supplied no ids gave this run nothing to "+
			"match against, so there is no thread it may close", *got.Threads)
	}
	if len(got.Threads.Refused) != 1 {
		t.Errorf("refused = %v, want the one id the reply named", got.Threads.Refused)
	}
	if got.Conclusion != "success" {
		t.Errorf("conclusion = %q, want success; a refused id is not a bad review",
			got.Conclusion)
	}
}

// TestTheReportedRefusalsCarryTheirTotal covers the number an operator reads.
//
// The refusal list is capped, so its length says nothing about how many there were. A
// model that slipped once and a model inventing ids by the thousand both print the same
// handful, and only the total separates them — which is the whole reason the field
// exists. Asserted on the printed bytes, because stdout is what an operator reads and a
// field left out of the payload is invisible from the struct.
func TestTheReportedRefusalsCarryTheirTotal(t *testing.T) {
	invented := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		invented = append(invented, fmt.Sprintf(`"invented%d"`, i))
	}
	block := `{"read":120,"decision":"approve","summary":"Clean.",` +
		`"resolved_thread_ids":[` + strings.Join(invented, ",") + `]}`
	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text: "A review.\n\n```json\n" + block + "\n```", TurnID: "t1",
	}}

	printed := captureStdout(t, func() {
		if err := report("", "", "", result, verdictOf(result), review.Request{}); err != nil {
			t.Fatalf("report: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(printed), &payload); err != nil {
		t.Fatalf("decoding the printed report: %v\n%s", err, printed)
	}
	ids, ok := payload["refused_thread_ids"].([]any)
	if !ok {
		t.Fatalf("the report names no refused ids:\n%s", printed)
	}
	total, ok := payload["refused_thread_ids_total"].(float64)
	if !ok {
		t.Fatalf("the report carries no refusal total, so the capped list reads as the "+
			"whole of what the reply named:\n%s", printed)
	}
	if int(total) != 40 {
		t.Errorf("refused_thread_ids_total = %v, want 40", total)
	}
	if len(ids) >= int(total) {
		t.Errorf("the printed list holds %d of %v ids; this test is meant to exercise "+
			"the capped case, where the length and the total differ", len(ids), total)
	}
}

// TestACleanReviewReportsNoRefusals keeps the two keys out of the common report, so their
// presence means something happened rather than being noise on every run.
func TestACleanReviewReportsNoRefusals(t *testing.T) {
	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text: "A review.\n\n```json\n" +
			`{"read":120,"decision":"approve","summary":"Clean."}` + "\n```",
		TurnID: "t1",
	}}

	printed := captureStdout(t, func() {
		if err := report("", "", "", result, verdictOf(result), review.Request{}); err != nil {
			t.Fatalf("report: %v", err)
		}
	})

	for _, key := range []string{"refused_thread_ids", "refused_thread_ids_total"} {
		if strings.Contains(printed, key) {
			t.Errorf("a review that refused nothing reports %q:\n%s", key, printed)
		}
	}
}

// captureStdout runs fn with os.Stdout replaced by a pipe and returns what it wrote.
//
// The report prints rather than returning, so this is the only way to assert on the
// bytes an operator sees. Restored on the way out whatever fn did.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = write
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, read)
		done <- b.String()
	}()
	func() {
		defer func() {
			os.Stdout = saved
			_ = write.Close()
		}()
		fn()
	}()
	out := <-done
	_ = read.Close()
	return out
}
