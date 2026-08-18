package xreview

import (
	"strings"
	"testing"
)

// TestBuildCheckRunCarriesWhatTheInlineCommentsCannot is the point of the check
// run: a blocker tied to no line reaches a reader nowhere else, and a review
// whose most important objection is "this needs a test" would otherwise land as
// a clean set of inline comments.
func TestBuildCheckRunCarriesWhatTheInlineCommentsCannot(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read": 120, "decision": "request_changes","summary":"Two problems.",
	  "inline_comments":[{"path":"a.go","line":9,"side":"RIGHT","severity":"blocker","body":"nil deref"}],
	  "blockers":["the new path has no test"],
	  "non_blockers":["naming could be clearer"],
	  "pre_existing_issues":[{"severity":"suggestion","body":"b.go:4 leaks a handle"}]}`)

	check, ok := BuildCheckRun(v)
	if !ok {
		t.Fatal("no check run built from a verdict")
	}
	if check.Conclusion != "failure" {
		t.Errorf("Conclusion = %q, want failure", check.Conclusion)
	}
	if !strings.Contains(check.Title, "3 findings") {
		t.Errorf("Title = %q, want it to count all three", check.Title)
	}
	if !strings.Contains(check.Title, "1 pre-existing issue") {
		t.Errorf("Title = %q, want the pre-existing count", check.Title)
	}
	for _, want := range []string{
		"Two problems.", "the new path has no test", "naming could be clearer",
		"b.go:4 leaks a handle",
	} {
		if !strings.Contains(check.Summary, want) {
			t.Errorf("Summary missing %q:\n%s", want, check.Summary)
		}
	}
	if strings.Contains(check.Summary, "nil deref") {
		t.Errorf("Summary repeats a line-tied finding, which the author reads "+
			"twice:\n%s", check.Summary)
	}
}

// TestBuildCheckRunOnACleanReview covers the case that must still publish. A
// checks list with no xreview entry reads as a review that did not run, not one
// that passed.
func TestBuildCheckRunOnACleanReview(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read": 120, "decision": "approve","summary":"Clean.",
	  "inline_comments":[],"blockers":[],"non_blockers":[],"pre_existing_issues":[]}`)

	check, ok := BuildCheckRun(v)
	if !ok {
		t.Fatal("a clean review still concludes")
	}
	if check.Conclusion != "success" {
		t.Errorf("Conclusion = %q, want success", check.Conclusion)
	}
	if !strings.Contains(check.Title, "0 findings") {
		t.Errorf("Title = %q", check.Title)
	}
	if !strings.Contains(check.Summary, "Clean.") {
		t.Errorf("Summary = %q", check.Summary)
	}
}

// TestBuildCheckRunWithoutAVerdict pins that a review that could not be decided
// publishes no check at all, rather than a green one.
func TestBuildCheckRunWithoutAVerdict(t *testing.T) {
	t.Parallel()

	if _, ok := BuildCheckRun(Verdict{}); ok {
		t.Fatal("built a check run from no verdict")
	}
}

// TestCheckConclusionFollowsTheFindings covers a reply that contradicts itself.
// Observed on sei-chain#3899: decision approve beside three non_blockers, which
// published a green check over a review that had three things to say.
func TestCheckConclusionFollowsTheFindings(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		block string
		want  string
	}{
		"approve with non-blocking notes is not clean": {
			`{"read": 120, "decision": "approve","summary":"s","non_blockers":["a","b","c"]}`, "neutral",
		},
		"approve with a placeable finding is not clean": {
			`{"read": 120, "decision": "approve","summary":"s",
			  "inline_comments":[{"path":"a.go","line":3,"severity":"nit","body":"x"}]}`, "neutral",
		},
		"approve over a pre-existing issue is not clean": {
			`{"read": 120, "decision": "approve","summary":"s",
			  "pre_existing_issues":[{"severity":"suggestion","body":"old"}]}`, "neutral",
		},
		"blockers outrank a soft decision": {
			`{"read": 120, "decision": "comment","summary":"s","blockers":["needs a test"]}`, "failure",
		},
		"request_changes stands on its own": {
			`{"read": 120, "decision": "request_changes","summary":"s"}`, "failure",
		},
		"nothing found is clean": {
			`{"read": 120, "decision": "approve","summary":"s","inline_comments":[],"blockers":[],
			  "non_blockers":[],"pre_existing_issues":[]}`, "success",
		},
	} {
		v := verdictFrom(t, tc.block)
		if got := v.CheckConclusion(); got != tc.want {
			t.Errorf("%s: conclusion = %q, want %q", name, got, tc.want)
		}
	}
}

// TestABlockerWithNoLineStillFailsTheCheck covers the gap between what a review
// reported and what can be posted on a line.
//
// A finding with no line is dropped from the inline comments, which is the right
// call — the prompt tells the agent not to guess one. Counting only what survived
// that drop let a reply naming a blocker publish a green check titled "0 findings",
// with the finding surviving in prose alone. The check gates a merge, so it follows
// everything the review reported.
func TestABlockerWithNoLineStillFailsTheCheck(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, reply, want string
	}{
		{"blocker with no line", "```json\n" +
			`{"read": 120, "decision": "approve","summary":"s","inline_comments":[` +
			`{"path":"a.go","severity":"blocker","body":"nil deref"}]}` + "\n```", "failure"},
		{"blocker on a line, decision comment", "```json\n" +
			`{"read": 120, "decision": "comment","summary":"s","inline_comments":[` +
			`{"path":"a.go","line":4,"side":"RIGHT","severity":"blocker","body":"nil deref"}]}` +
			"\n```", "failure"},
		{"a nit is still not blocking", "```json\n" +
			`{"read": 120, "decision": "approve","summary":"s","inline_comments":[` +
			`{"path":"a.go","line":4,"side":"RIGHT","severity":"nit","body":"naming"}]}` +
			"\n```", "neutral"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			v := ParseVerdict(c.reply)
			if got := v.CheckConclusion(); got != c.want {
				t.Errorf("CheckConclusion = %q, want %q", got, c.want)
			}
			if run, _ := BuildCheckRun(v); strings.HasPrefix(run.Title, "0 finding") {
				t.Errorf("Title = %q, want it to count what the review reported", run.Title)
			}
		})
	}
}

// TestBuildFailureCheckNamesWhyThereIsNoVerdict covers the difference between a
// review that could not be read and a job that never ran.
//
// Publishing nothing makes those identical in the checks list, and only one of them
// is a reason to look. The reason is computed on every refusal path already.
func TestBuildFailureCheckNamesWhyThereIsNoVerdict(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("I could not read the diff.")
	if _, ok := BuildCheckRun(v); ok {
		t.Fatal("BuildCheckRun accepted a reply with no verdict")
	}
	run := BuildFailureCheck(v)
	if run.Conclusion != "neutral" {
		t.Errorf("Conclusion = %q, want neutral", run.Conclusion)
	}
	if !strings.Contains(run.Summary, "fenced json block") {
		t.Errorf("Summary = %q, want it to carry the parser's own reason", run.Summary)
	}
}

// TestAReviewThatNeverReadTheDiffIsNotClean covers the failure that would report
// green across the whole fleet at once.
//
// A review whose diff fetch failed has no findings either, so deriving the
// conclusion from the findings alone reads it as clean. The scout contract carries
// a line count for exactly this reason. Degraded rather than failed: a missing count
// means this tool cannot tell a clean review from a review of nothing.
func TestAReviewThatNeverReadTheDiffIsNotClean(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, reply, want string }{
		{"read nothing", "```json\n" + `{"read": 0, "decision": "approve", "summary": "s"}` + "\n```", "neutral"},
		{"omits the count", "```json\n" + `{"decision": "approve", "summary": "s"}` + "\n```", "neutral"},
		{"read the diff", "```json\n" + `{"read": 412, "decision": "approve", "summary": "s"}` + "\n```", "success"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ParseVerdict(c.reply).CheckConclusion(); got != c.want {
				t.Errorf("CheckConclusion = %q, want %q", got, c.want)
			}
		})
	}
}

// TestUnplaceableNotesStillMarkTheCheck covers the general case of the blocker gap.
//
// A note dropped for naming no line is still something the review wrote down.
// Counting only what could be placed let an approve over unplaceable notes publish a
// clean check, which is the reading the derivation exists to prevent.
func TestUnplaceableNotesStillMarkTheCheck(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("```json\n" +
		`{"read": 9, "decision": "approve", "summary": "s", "inline_comments":[` +
		`{"path":"a.go","severity":"suggestion","body":"worth a look"}]}` + "\n```")
	if got := v.CheckConclusion(); got != "neutral" {
		t.Errorf("CheckConclusion = %q, want neutral: the review wrote a note down", got)
	}
	if run, _ := BuildCheckRun(v); strings.HasPrefix(run.Title, "0 finding") {
		t.Errorf("Title = %q, want it to count the note", run.Title)
	}
}

// TestTitleCountsOneObservationOnce pins the dedupe across both schema keys. An
// adopted session can write the same observation under either vocabulary.
func TestTitleCountsOneObservationOnce(t *testing.T) {
	t.Parallel()

	one := `{"path":"a.go","line":4,"side":"RIGHT","severity":"nit","body":"x"}`
	v := ParseVerdict("```json\n" +
		`{"read": 9, "decision": "comment", "summary": "s",` +
		`"inline_comments":[` + one + `], "findings":[` + one + `]}` + "\n```")
	run, _ := BuildCheckRun(v)
	if !strings.HasPrefix(run.Title, "1 finding") {
		t.Errorf("Title = %q, want 1 finding: one observation under both keys is one finding", run.Title)
	}
}
