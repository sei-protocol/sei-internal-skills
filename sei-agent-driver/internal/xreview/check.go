package xreview

import (
	"fmt"
	"strings"
)

// CheckRun is what a caller needs to publish a review as a GitHub check run.
//
// A check run is the half of a review a reader sees without opening it. The conclusion
// lands in the pull request's checks list, so it says at a glance whether the review
// objected. What fills it is the buckets a review sorts its observations into, and above
// all the ones tied to no line: those reach a reader nowhere else.
type CheckRun struct {
	// Conclusion is failure, neutral or success. Empty when the turn produced no verdict,
	// which is not a check run at all: a review that could not be decided has nothing to
	// conclude.
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
	// Every finding the reply reported, not only the placeable ones. A finding dropped for
	// naming no line is still something the review said. A title that counts only what could
	// be pinned to a line reads as "found nothing" over a review that found something.
	counts := []string{
		plural(distinctReported(v)+len(Blockers(v))+len(NonBlockers(v)), "finding"),
	}
	if n := len(PreExisting(v)); n > 0 {
		counts = append(counts, plural(n, "pre-existing issue"))
	}
	return strings.Join(counts, ", ")
}

// maxCheckSummary bounds the check run's summary.
//
// GitHub rejects a check run whose summary exceeds 65,536 characters. Every input here
// is model output: the summary itself, and every blocker, non-blocker and pre-existing
// entry. A rejected check run is no check run, which reads as a review that did not run
// rather than one that passed. So this truncates for the same reason [RenderComment]
// does, and says when it did.
const maxCheckSummary = 60_000

// maxCheckBullet bounds one entry, so a single long one cannot crowd out the rest.
const maxCheckBullet = 2_000

// checkTruncated says the summary was cut, so a reader does not take a truncated
// list for the whole one.
const checkTruncated = "\n\n_This summary was truncated. The published comment carries the full review._"

// checkSummary renders the review's own summary and every observation that names no
// line.
//
// The line-tied ones are deliberately absent. They are posted against the code they are
// about, and repeating them here would have an author read each twice. What would
// otherwise be lost is exactly what this carries.
//
// Every part of it is model text, assembled under headings of this package's. That is
// what makes the sanitising load-bearing here and not in the published comment. There
// the agent's prose stands alone as the agent's prose. Here it sits beside framing it
// must not be able to imitate.
func checkSummary(v Verdict) string {
	sections := []string{defuseHeadings(v.Summary())}
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
		lines = append(lines, checkBullet(item))
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
		// Through checkBullet like every other entry. Both fields are model text, and
		// a severity or a body carrying a newline forges a section here as readily as
		// a blocker does.
		lines = append(lines, checkBullet(fmt.Sprintf("**%s** — %s", issue.Severity, issue.Body)))
	}
	return strings.Join(lines, "\n")
}

// checkBullet renders one piece of model text as a list item.
//
// One-lined as well as clipped. An entry carrying newlines can otherwise render its own
// "### Blocking" heading, which is indistinguishable from this package's framing to
// whoever reads the check.
func checkBullet(s string) string { return "- " + clip(oneLine(s), maxCheckBullet) }

// defuseHeadings makes a line-leading markdown heading marker inert.
//
// For the summary, the one piece of model text published here as prose. It keeps its
// paragraphs, so it cannot be one-lined the way an entry is. What it must not keep is
// the ability to open a section: a "### Blocking" of its own reads as this package's
// framing.
//
// Both spellings, since either makes a heading: a leading # and a run of = or - alone on
// a line under a paragraph. A backslash before the first character is what markdown
// honours. So a legitimate --- separator renders as literal --- text. That is the cost,
// and it falls on a rule nobody writing a summary needs.
//
// Line endings are normalised first, because markdown recognises three and Go splits
// Line endings are normalised first, because markdown recognises three and Go splits on
// one. With \r\n, a trailing \r stays on the underline line. The run of - stops matching
// here, and a parser still reads it as a heading. A lone \r is worse. It ends a line for
// the parser, and Split does not break on it at all, so every heading after the first one
// arrives live.
//
// Those three are the whole set. CommonMark defines a line ending as \n, \r, or \r\n,
// and nothing else.
func defuseHeadings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		rest := line[indent:]
		if strings.HasPrefix(rest, "#") || setextUnderline(rest) {
			lines[i] = line[:indent] + `\` + rest
		}
	}
	return strings.Join(lines, "\n")
}

// setextUnderline reports whether a line would turn the paragraph above it into a
// heading. That is a run of = or of -, with nothing else on the line.
func setextUnderline(s string) bool {
	t := strings.TrimRight(s, " \t")
	if t == "" {
		return false
	}
	return strings.Count(t, "=") == len(t) || strings.Count(t, "-") == len(t)
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// BuildFailureCheck is the check run for a review that produced no verdict.
//
// [BuildCheckRun] reports false there. A caller that publishes nothing leaves the checks
// list with no xreview entry, which reads as a review that did not run rather than one
// that could not be read. Those are different things, and only one of them means an
// operator should look.
//
// The conclusion is neutral rather than failure. It says this tool could not read a
// review, not that the change is bad. A repository that wants that distinction to
// block can require the check. The reason it carries is the one [Verdict] already
// computed on every refusal path and nothing published.
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
