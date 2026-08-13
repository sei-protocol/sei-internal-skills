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

	v := verdictFrom(t, `{"decision":"request_changes","summary":"Two problems.",
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

	v := verdictFrom(t, `{"decision":"approve","summary":"Clean.",
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
