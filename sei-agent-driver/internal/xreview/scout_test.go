package xreview

import (
	"strings"
	"testing"
)

// TestScoutRunKeysNeverCollide guards session identity.
//
// The run key IS the session: a later dispatch adopts whatever it matches. Two
// roles sharing a key means a scout wakes up inside the review's conversation,
// reads the verdict it was supposed to be independent of, and agrees with it. The
// arrangement's only purpose is independence, and this is what enforces it.
func TestScoutRunKeysNeverCollide(t *testing.T) {
	const repo, pr = "sei-protocol/sei-chain", 3861
	review := RunKey(repo, pr)
	codex := ScoutRunKey(repo, pr, "codex")
	cursor := ScoutRunKey(repo, pr, "cursor")

	for _, c := range []struct{ a, b, why string }{
		{review, codex, "a scout would adopt the review's session and read the verdict it must not see"},
		{review, cursor, "a scout would adopt the review's session and read the verdict it must not see"},
		{codex, cursor, "two scouts would share a conversation and converge on one opinion"},
	} {
		if c.a == c.b {
			t.Errorf("run keys collide (%s): %s", c.a, c.why)
		}
	}

	// A scout's key must also not move between dispatches, or every invocation
	// opens a new session and the continuity the key exists for is lost.
	if ScoutRunKey(repo, pr, "codex") != codex {
		t.Error("ScoutRunKey is not stable for the same scout, so no dispatch can adopt an earlier session")
	}
	// Different pull requests are different work.
	if ScoutRunKey(repo, pr+1, "codex") == codex {
		t.Error("ScoutRunKey ignores the pull request, so one scout session would serve every review")
	}
}

// TestScoutCompleteRequiresAFindingsBlock guards the turn-end contract.
//
// A session reports itself idle between tool calls, so this is the only reliable
// signal that a scout has finished. Accepting prose ends the turn on an opening
// sentence; refusing an empty findings list hangs a scout that correctly found
// nothing, and the run burns to its deadline.
func TestScoutCompleteRequiresAFindingsBlock(t *testing.T) {
	s := NewScout(Request{Repo: "sei-protocol/sei-chain", PR: 3861}, "codex", "sei-droid-codex")

	for _, c := range []struct {
		name string
		text string
		want bool
	}{
		{"prose only", "I am starting to look at the diff now.", false},
		{"block without findings", "done\n```json\n{\"read\": 9, \"summary\": \"fine\"}\n```", false},
		{"malformed block", "done\n```json\n{\"findings\": [}\n```", false},
		// Without the read count the orchestrator cannot tell a scout that read
		// the diff from one that never got it, so the report is not an answer.
		{"findings without a read count", "done\n```json\n{\"findings\": []}\n```", false},
		{"read nothing is an answer", "the fetch failed\n```json\n{\"read\": 0, \"findings\": []}\n```", true},
		{"read and found nothing", "clean\n```json\n{\"read\": 812, \"findings\": []}\n```", true},
		{"populated findings", "one thing\n```json\n{\"read\": 812, \"findings\": [" +
			"{\"file\": \"a.go\", \"line\": 4, \"severity\": \"high\", \"detail\": \"boom\"}]}\n```", true},
	} {
		if got := s.Complete(c.text); got != c.want {
			t.Errorf("%s: Complete = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestScoutAndVerdictContractsStayApart guards the separation the two schemas
// rest on.
//
// A scout reports findings and no decision; the review decides. If either parser
// accepted the other's block, a scout could end its turn on a quoted verdict, or a
// review could publish a scout's reading as its own decision.
func TestScoutAndVerdictContractsStayApart(t *testing.T) {
	scoutReply := "found one\n```json\n{\"read\": 812, \"findings\": [" +
		"{\"file\": \"a.go\", \"line\": 4, \"severity\": \"high\", \"detail\": \"boom\"}]}\n```"
	verdictReply := "reviewed\n```json\n{\"decision\": \"comment\", \"summary\": \"ok\", \"findings\": []}\n```"

	if ParseVerdict(scoutReply).HasVerdict() {
		t.Error("ParseVerdict accepts a scout report, so a scout's reading could be published as a decision")
	}
	if !ParseScoutReport(scoutReply).HasReport() {
		t.Error("ParseScoutReport rejects a well-formed scout report")
	}
	if !ParseVerdict(verdictReply).HasVerdict() {
		t.Error("ParseVerdict rejects a well-formed verdict")
	}
}

// TestScoutPromptsNameTheDiffCommand guards the property that makes a reading a
// reading, for the same reason the review's prompts are held to it: an abstract
// instruction to inspect the diff is satisfiable by `gh pr view`.
func TestScoutPromptsNameTheDiffCommand(t *testing.T) {
	req := Request{Repo: "sei-protocol/sei-chain", PR: 3861}
	// Written out rather than built from the helper the prompts use, which would
	// assert only that they agree with each other.
	wantCommand := "gh pr diff 3861 --repo sei-protocol/sei-chain > " +
		"pr-3861.diff && wc -l pr-3861.diff"

	for _, p := range []struct {
		name string
		text string
	}{
		{"ScoutPrompt", ScoutPrompt(req)},
		{"AdoptedScoutPrompt", AdoptedScoutPrompt(req)},
	} {
		if !strings.Contains(p.text, wantCommand) {
			t.Errorf("%s does not name %q, so the scout can report from the title alone",
				p.name, wantCommand)
		}
		if !strings.Contains(p.text, "untrusted") {
			t.Errorf("%s drops the untrusted-content rule, which is the only thing "+
				"telling the scout the diff is not addressed to it", p.name)
		}
	}

	// The scout must not be asked for a decision: that is the review's, and two
	// deciding blocks in one conversation is how a verdict becomes ambiguous.
	if strings.Contains(ScoutPrompt(req), `"decision"`) {
		t.Error("ScoutPrompt asks the scout for a decision, which belongs to the review")
	}
}

// TestScoutNamesItsOwnAgent guards independence at the harness level: a scout on
// the review's agent is the same harness reading the same diff, which corroborates
// nothing.
func TestScoutNamesItsOwnAgent(t *testing.T) {
	s := NewScout(Request{Repo: "r/n", PR: 1}, "codex", "sei-droid-codex")
	if got := s.AgentName(); got != "sei-droid-codex" {
		t.Errorf("AgentName = %q, want the bundle the scout was dispatched on", got)
	}
	if s.Name() != "codex" {
		t.Errorf("Name = %q, want the identity the synthesis attributes findings with", s.Name())
	}
}

// TestScoutReportSeparatesAFailedReadFromACleanOne guards the distinction three
// doc comments claim the design rests on.
//
// A scout that never got the diff and a scout that read it and found nothing both
// close with an empty findings list. Only the read count separates them, and
// without it the orchestrator renders the first as the second — positive evidence
// of a clean review, generated indefinitely, with no error anywhere.
func TestScoutReportSeparatesAFailedReadFromACleanOne(t *testing.T) {
	t.Parallel()

	failed := ParseScoutReport("could not fetch\n```json\n{\"read\": 0, \"findings\": []}\n```")
	clean := ParseScoutReport("nothing found\n```json\n{\"read\": 812, \"findings\": []}\n```")

	if !failed.HasReport() || !clean.HasReport() {
		t.Fatal("both are answers; neither should hang the turn")
	}
	if failed.Read() {
		t.Error("a scout reporting read=0 is recorded as having read the diff")
	}
	if !clean.Read() {
		t.Error("a scout reporting a line count is not recorded as having read the diff")
	}
	if failed.Read() == clean.Read() {
		t.Error("a failed read is indistinguishable from a clean one, which is the " +
			"failure the design says it exists to prevent")
	}
}

// TestParseScoutReportSaysWhyItRefused guards the operator's diagnostic. The
// sibling ParseVerdict sets a distinct reason on every refusal path; a scout that
// burns its whole budget and returns one generic sentence tells nobody anything.
func TestParseScoutReportSaysWhyItRefused(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, text := range []string{
		"just prose",
		"bad\n```json\n{not json}\n```",
		"no list\n```json\n{\"read\": 9}\n```",
		"no count\n```json\n{\"findings\": []}\n```",
	} {
		r := ParseScoutReport(text)
		if r.HasReport() {
			t.Errorf("%q was accepted as a report", text)
			continue
		}
		if r.Reason == "" {
			t.Errorf("%q was refused with no reason for the operator", text)
			continue
		}
		if seen[r.Reason] {
			t.Errorf("%q reuses the reason %q, so two faults read alike", text, r.Reason)
		}
		seen[r.Reason] = true
	}
}

// TestReconcileStepWillNotPointTheReviewOutOfTheTree guards what a scout can send
// the review to open.
//
// The review is told to check each claim against the diff, which means reading
// what the claim names, in a sandbox holding a live credential. A file field is
// model output about attacker-influenceable input, so a path leaving the tree is
// rendered as text rather than as somewhere to go.
func TestReconcileStepWillNotPointTheReviewOutOfTheTree(t *testing.T) {
	t.Parallel()

	for _, escape := range []string{
		"/etc/passwd", "~/.config/gh/hosts.yml", "../../.git/config", "a/../../../etc/shadow",
	} {
		req := Request{Repo: "r/n", PR: 1, Scouts: []ScoutResult{{
			Name:     "codex",
			Findings: []Finding{{File: escape, Line: 1, Severity: "high", Detail: "look here"}},
		}}}
		rendered := strings.Join(reconcileStep(req), "\n")
		if strings.Contains(rendered, escape) {
			t.Errorf("%q is rendered as a location the review is told to open", escape)
		}
		if !strings.Contains(rendered, "no place in this tree") {
			t.Errorf("%q is dropped silently rather than shown as unplaceable", escape)
		}
	}

	// A real path still renders as one.
	ok := Request{Repo: "r/n", PR: 1, Scouts: []ScoutResult{{
		Name: "codex", Findings: []Finding{{File: "p2p/router.go", Line: 174, Severity: "low", Detail: "x"}},
	}}}
	if !strings.Contains(strings.Join(reconcileStep(ok), "\n"), "p2p/router.go:174") {
		t.Error("an in-tree path is not rendered as a location")
	}
}

// TestBothScoutPromptsCarryTheSchema pins the reason the review's own rules are
// restated on the adopted path: a session replays only its first prompt, so a
// contract the scout is told to recall is one it may not be able to re-read — and
// a back-reference points at the old schema if this contract ever changes.
func TestBothScoutPromptsCarryTheSchema(t *testing.T) {
	t.Parallel()

	req := Request{Repo: "o/r", PR: 1}
	for name, got := range map[string]string{
		"ScoutPrompt":        ScoutPrompt(req),
		"AdoptedScoutPrompt": AdoptedScoutPrompt(req),
	} {
		for _, want := range []string{`"read": 0`, `"severity": "high|medium|low"`, "```json"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s does not carry %q", name, want)
			}
		}
		if strings.Contains(got, "same schema as before") {
			t.Errorf("%s points back at a schema instead of restating it", name)
		}
	}
}
