package xreview

import (
	"fmt"
	"strings"
)

// CheckRun is what a caller needs to publish a review as a GitHub check run.
//
// A check run is the half of a review a reader sees without opening it: the
// conclusion lands in the pull request's checks list, so it is what says at a
// glance whether the review objected. The buckets a review sorts its
// observations into are what fills it — the ones tied to no line especially,
// since those reach a reader nowhere else.
type CheckRun struct {
	// Conclusion is failure, neutral or success. Empty when the turn produced no
	// verdict, which is not a check run at all: a review that could not be
	// decided has nothing to conclude.
	Conclusion string `json:"conclusion"`

	// Title is the one-line reading in the checks list.
	Title string `json:"title"`

	// Summary is the body, in markdown.
	Summary string `json:"summary"`
}

// BuildCheckRun renders a verdict as a check run, and reports whether there is
// one to publish.
func BuildCheckRun(v Verdict) (CheckRun, bool) {
	if !v.HasVerdict() {
		return CheckRun{}, false
	}
	return CheckRun{
		Conclusion: v.CheckConclusion(),
		Title:      checkTitle(v),
		Summary:    checkSummary(v),
	}, true
}

// checkTitle counts what the review found, because the count is what a reader
// scanning the checks list is deciding on.
func checkTitle(v Verdict) string {
	// Every finding the reply reported, not only the placeable ones. A finding
	// dropped for naming no line is still something the review said, and a title
	// that counts only what could be pinned to a line reads as "found nothing" over
	// a review that found something.
	counts := []string{
		plural(len(reportedFindings(v))+len(Blockers(v))+len(NonBlockers(v)), "finding"),
	}
	if n := len(PreExisting(v)); n > 0 {
		counts = append(counts, plural(n, "pre-existing issue"))
	}
	return strings.Join(counts, ", ")
}

// checkSummary renders the review's own summary and every observation that
// names no line.
//
// The line-tied ones are deliberately absent: they are posted against the code
// they are about, and repeating them here would have an author read each twice.
// What would otherwise be lost is exactly what this carries.
func checkSummary(v Verdict) string {
	sections := []string{v.Summary()}
	sections = append(sections, bulletSection("Blocking", Blockers(v)))
	sections = append(sections, bulletSection("Non-blocking", NonBlockers(v)))
	sections = append(sections, preExistingSection(PreExisting(v)))

	out := make([]string, 0, len(sections))
	for _, s := range sections {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "\n\n")
}

func bulletSection(heading string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, "### "+heading)
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func preExistingSection(issues []PreExistingIssue) string {
	if len(issues) == 0 {
		return ""
	}
	lines := make([]string, 0, len(issues)+2)
	lines = append(lines, "### Pre-existing",
		"Already true on the base branch, not introduced here.")
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("- **%s** — %s", issue.Severity, issue.Body))
	}
	return strings.Join(lines, "\n")
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
