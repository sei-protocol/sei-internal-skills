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
		plural(distinctReported(v)+len(Blockers(v))+len(NonBlockers(v)), "finding"),
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
// maxCheckSummary bounds the check run's summary.
//
// GitHub rejects a check run whose summary exceeds 65,536 characters, and every
// input here is model output: the summary itself, and every blocker, non-blocker
// and pre-existing entry. A rejected check run is no check run, which reads as a
// review that did not run rather than one that passed -- so this truncates for the
// same reason [RenderComment] does, and says when it did.
const maxCheckSummary = 60_000

// maxCheckBullet bounds one entry, so a single long one cannot crowd out the rest.
const maxCheckBullet = 2_000

// checkTruncated says the summary was cut, so a reader does not take a truncated
// list for the whole one.
const checkTruncated = "\n\n_This summary was truncated. The published comment carries the full review._"

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
	// Bounded last, over the assembled whole. Bounding each part would still let
	// enough parts exceed what the API accepts.
	summary := strings.Join(out, "\n\n")
	if len(summary) > maxCheckSummary {
		summary = truncateBytes(summary, maxCheckSummary-len(checkTruncated)) + checkTruncated
	}
	return summary
}

func bulletSection(heading string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, "### "+heading)
	for _, item := range items {
		// One-lined as well as clipped: an entry carrying newlines can otherwise
		// render its own "### Blocking" heading, which is indistinguishable from
		// this package's framing to whoever reads the check.
		lines = append(lines, "- "+clip(oneLine(item), maxCheckBullet))
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

// BuildFailureCheck is the check run for a review that produced no verdict.
//
// [BuildCheckRun] reports false there, and a caller that publishes nothing leaves
// the checks list with no xreview entry -- which reads as a review that did not
// run, not one that could not be read. Those are different things, and only one of
// them means an operator should look.
//
// neutral rather than failure: this says the tool could not read a review, not that
// the change is bad, and a repository that wants the distinction to block can
// require the check. The reason is the one [Verdict] already computed on every
// refusal path and that nothing published.
func BuildFailureCheck(v Verdict) CheckRun {
	reason := v.Reason
	if reason == "" {
		reason = "the turn produced no reply this driver could read"
	}
	return CheckRun{
		Title:      "no verdict",
		Conclusion: "neutral",
		Summary: "This review produced no decision that could be read mechanically.\n\n" +
			clip(oneLine(reason), maxCheckBullet) +
			"\n\nThe agent's own words, if it wrote any, are in the published comment.",
	}
}
