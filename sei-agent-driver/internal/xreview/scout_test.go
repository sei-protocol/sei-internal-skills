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
		{"block without findings", "done\n```json\n{\"summary\": \"looks fine\"}\n```", false},
		{"malformed block", "done\n```json\n{\"findings\": [}\n```", false},
		{"empty findings is an answer", "nothing to report\n```json\n{\"findings\": []}\n```", true},
		{"populated findings", "one thing\n```json\n{\"findings\": [" +
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
	scoutReply := "found one\n```json\n{\"findings\": [" +
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
