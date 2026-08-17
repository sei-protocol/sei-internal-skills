package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
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
			raw:  "codex=sei-droid-codex, cursor=sei-droid-cursor",
			want: []scoutSpec{
				{name: "codex", agent: "sei-droid-codex"},
				{name: "cursor", agent: "sei-droid-cursor"},
			},
		},
		{name: "missing agent", raw: "codex=", wantErr: true},
		{name: "missing name", raw: "=sei-droid-codex", wantErr: true},
		{name: "no separator", raw: "codex", wantErr: true},
		{name: "duplicate name collides on the run key", raw: "codex=a,codex=b", wantErr: true},
		// The bundle fixes the harness, so both of these produce a reading that is
		// not independent of the thing it is meant to check.
		{name: "scout on the review's own agent", raw: "codex=sei-droid", wantErr: true},
		{name: "two scouts on one agent", raw: "codex=a,cursor=a", wantErr: true},
	} {
		got, err := parseScouts(c.raw, "sei-droid")
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
// no longer depend on --out. A checks list with no xreview entry reads as a
// review that did not run rather than one that passed, so gating the fail-closed
// signal on an unrelated flag made it fail open.
func TestReportWritesEachOutputOnItsOwnFlag(t *testing.T) {
	dir := t.TempDir()
	check := filepath.Join(dir, "check.json")

	result := driver.Result{SessionID: "s1", Reply: &driver.Reply{
		Text:   "A review.\n\n```json\n{\"decision\":\"comment\",\"summary\":\"s\"}\n```",
		TurnID: "t1", ItemID: "i1",
	}}
	if err := report("", "", check, result); err != nil {
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
	if err := report(out, "", "", driver.Result{SessionID: "s2"}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		body, _ := os.ReadFile(out)
		t.Errorf("the earlier run's verdict survived and would be published: %q", body)
	}
}
