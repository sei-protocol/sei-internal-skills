package review

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

	// Counts are this review's totals, for the pull request comment the caller composes
	// beside the check run. They are not part of the check run GitHub publishes. They
	// ride here because this is the file the caller already reads, and because they and
	// Conclusion are two readings of one reply that must not disagree.
	//
	// Nil when there is nothing to count; see [BuildFailureCheck]. A pointer so that
	// absent stays distinguishable from counted-and-zero. A caller that read a zero off
	// a review this driver could not read would report nothing blocking over it, and an
	// older driver binary writes the same absent key, so one branch covers both.
	Counts *Counts `json:"counts,omitempty"`
}

// Counts are the integers a published comment's findings line is composed from.
//
// Every observation the reply wrote lands in exactly one of Blocking, NonBlocking and
// PreExisting. The exception is a nit this run does not admit, which lands in none: it
// reaches the reader through no inline comment and appears in no section, so counting it
// would put a number on the comment that nothing underneath accounts for. [checkTitle]
// has counted it that way since the nit gate landed, and these must not disagree with it.
//
// They are not ai-review's numbers. That workflow counts a pre-existing blocker as
// blocking, beside a gate that deliberately does not fail on one, and counts its
// non-blocking inline comments as the anchored ones alone, so a suggestion whose line
// fell outside the diff is dropped from the total while still being printed below it.
// Both are corrected here rather than reproduced.
type Counts struct {
	// Blocking is the blockers bucket plus every line-tied finding that calls itself a
	// blocker, whether or not it named a line.
	//
	// Greater than zero implies Conclusion is failure, termwise:
	// [Verdict.CheckConclusion] fails on a non-empty blockers bucket, and on
	// [Verdict.hasBlockingFinding], which reads the same entries this does under the
	// same guard.
	//
	// The converse holds for every reply that sorted its observations as the prompt
	// asks, and fails for one shape: decision request_changes naming nothing blocking.
	// CheckConclusion honours that word, because the decision escalates and cannot
	// clear, and there is no entry here to count. Nothing is invented for it -- a number
	// with no bullet under it is the defect these counts exist to remove, seen from the
	// other side -- so a reader who finds zero blocking beside a red check is reading a
	// true statement about a malformed reply. Closing it needs a prompt clause, which
	// this change withholds.
	//
	// Pre-existing blockers are not in it, for the reason CheckConclusion gives: a
	// blocker already on the base branch would fail every pull request that touches the
	// file, and the author who has to clear it is the one person who did not cause it.
	Blocking int `json:"blocking"`

	// NonBlocking is everything else the review wrote down about this change: the
	// non_blockers bucket whole, and every other line-tied finding, placed or not.
	//
	// A finding whose severity the reply did not state is in it. Every other rule here
	// already treats an unrecognised severity as non-blocking -- hasBlockingFinding,
	// [PlaceableFindings], [Verdict.hasNotes] -- and a count that broke with them would
	// be a fourth answer to one question.
	NonBlocking int `json:"non_blocking"`

	// Placeable is how many findings were handed to the caller to post against a line,
	// which is the length of the findings file.
	//
	// What the caller can attempt, not what it managed. A line outside the diff's hunks
	// lands on the file instead, and one that lands nowhere goes in the summary. The
	// caller knows which happened and this does not, so a comment that says how many
	// reached a line says it from the caller's own tally and uses this as the
	// denominator.
	//
	// Bounded by maxPlaceableFindings where Blocking and NonBlocking are not, so it is
	// not a subset of their sum on a reply that ran away: a reply reporting sixty
	// findings yields fifty here and sixty across the two above.
	Placeable int `json:"placeable"`

	// PreExisting is how many problems the review said the change did not introduce.
	//
	// Its own number rather than a share of the two above. It answers a different
	// question -- it is about the code the change landed in, not about the change -- and
	// the gate excludes it deliberately. A caller rendering it beside the other two says
	// so, or a reader sums the sections and gets a total no number here reports.
	PreExisting int `json:"pre_existing"`
}

// BuildCheckRun renders a verdict as a check run, and reports whether there is
// one to publish.
func BuildCheckRun(v Verdict, includeNits bool) (CheckRun, bool) {
	if !v.HasVerdict() {
		return CheckRun{}, false
	}
	counts := countFindings(v, includeNits)
	return CheckRun{
		Conclusion: v.CheckConclusion(),
		Title:      checkTitle(counts),
		Summary:    checkSummary(v),
		Counts:     &counts,
	}, true
}

// countFindings derives every total from one reading of the reply.
//
// One derivation, so the check's title and the comment's findings line cannot drift.
// They are read side by side on the same pull request, and two numbers about one review
// that disagree leave a reader no way to tell which is meant.
func countFindings(v Verdict, includeNits bool) Counts {
	blocking, other := reportedBySeverity(v, includeNits)
	return Counts{
		Blocking:    blocking + len(Blockers(v)),
		NonBlocking: other + len(NonBlockers(v)),
		Placeable:   len(PlaceableFindings(v, includeNits)),
		PreExisting: len(PreExisting(v)),
	}
}

// checkTitle counts what the review found, because the count is what a reader
// scanning the checks list is deciding on.
//
// Rendered from [Counts] rather than derived again. Every finding the reply reported, not
// only the placeable ones: a finding dropped for naming no line is still something the
// review said, and a title that counts only what could be pinned to a line reads as
// "found nothing" over a review that found something.
func checkTitle(c Counts) string {
	parts := []string{plural(c.Blocking+c.NonBlocking, "finding")}
	if c.PreExisting > 0 {
		parts = append(parts, plural(c.PreExisting, "pre-existing issue"))
	}
	return strings.Join(parts, ", ")
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

// maxSummaryProse bounds the review's own summary before anything is appended to it.
//
// The schema asks for one or two sentences. A reply that writes tens of thousands of
// characters there is a runaway, and the assembled body is cut from the end, so that
// reply pushed every section this package writes past the cut. A blocker that names no
// line reaches a reader nowhere else, so what a runaway summary evicted was the review's
// actual objections.
const maxSummaryProse = 4_000

// maxCheckSection bounds what one bucket contributes, so a bucket that ran away cannot
// evict the next one. Three of these and one maxSummaryProse fit inside maxCheckSummary
// with room to spare, which is what makes every section reach the reader.
const maxCheckSection = 16_000

// maxCheckEntries bounds how many entries one bucket renders, so a reply that looped
// does not fill a section with a thousand near-identical lines.
const maxCheckEntries = 50

// sectionSeparator joins the check body's parts. Named because the prose budget has to
// account for it, and a blank line between two sections is not obviously two bytes.
const sectionSeparator = "\n\n"

// checkTruncated says the summary was cut, so a reader does not take a truncated
// list for the whole one.
const checkTruncated = "\n\n_This summary was truncated. The published comment carries the full review._"

// checkProseTruncated says the review's own summary was cut, so a reader does not take
// what is left of it for the whole of it. Separate from [checkTruncated] because it
// names a different loss: the sections below it are intact.
const checkProseTruncated = "\n\n_The review's summary was truncated here. " +
	"The published comment carries it in full._"

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
// must not be able to imitate, so every field goes through [defuseMarkup].
func checkSummary(v Verdict) string {
	sections := []string{
		bulletSection("Blocking", "", Blockers(v)),
		bulletSection("Non-blocking", "", NonBlockers(v)),
		preExistingSection(PreExisting(v)),
	}
	out := make([]string, 0, len(sections))
	for _, s := range sections {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	body := strings.Join(out, sectionSeparator)

	// The prose is bounded here, against its own budget and against what the sections
	// already spent. Bounding only the assembled whole cuts from the end, which is where
	// the sections are: a summary large enough evicted all three of them.
	if prose := clipProse(defuseMarkup(v.Summary()), len(body)); prose != "" {
		body = strings.TrimRight(prose+"\n\n"+body, "\n")
	}
	// Bounded over the whole as well. Every part is bounded now, so this is the backstop
	// rather than the control: it fires only on a combination the parts allow and the
	// API does not.
	if len(body) > maxCheckSummary {
		body = truncateBytes(body, maxCheckSummary-len(checkTruncated)) + checkTruncated
	}
	return body
}

// clipProse bounds the review's summary against maxSummaryProse and against the room the
// sections left, whichever is smaller.
//
// Zero room yields nothing. A bucket large enough to spend the whole budget is a review
// whose objections are the body, and prose that pushed them out would cost the reader
// the thing the check exists to carry.
func clipProse(s string, spent int) string {
	budget := maxSummaryProse
	// What the prose has to leave room for besides itself: the separator joining it to
	// the first section, and the notice a cut appends.
	if room := maxCheckSummary - spent - len(checkProseTruncated) - len(sectionSeparator); room < budget {
		budget = room
	}
	if budget <= 0 {
		return ""
	}
	if len(s) <= budget {
		return s
	}
	return truncateBytes(s, budget) + checkProseTruncated
}

// bulletSection renders one bucket under its heading, bounded in bytes and in count.
//
// A bucket is model output and nothing upstream limits how much of it there is. Rendered
// whole, one bucket spends the budget the next one needs, and the section after it never
// reaches the reader. What is left out is counted rather than dropped, so a shortened
// list reads as shortened rather than as all the review had.
func bulletSection(heading, lead string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	lines := []string{"### " + heading}
	if lead != "" {
		lines = append(lines, lead)
	}
	size := 0
	for _, line := range lines {
		size += len(line) + 1
	}
	shown := 0
	for _, item := range items {
		bullet := checkBullet(item)
		if shown >= maxCheckEntries || size+len(bullet)+1 > maxCheckSection {
			break
		}
		lines = append(lines, bullet)
		size += len(bullet) + 1
		shown++
	}
	if n := len(items) - shown; n > 0 {
		lines = append(lines, fmt.Sprintf(
			"- _and %d more, not shown here; the published comment carries the full review._", n))
	}
	return strings.Join(lines, "\n")
}

// preExistingSection renders the bucket for problems the change did not introduce.
//
// Every entry goes through the same bullet as the others. Both fields are model text,
// and a severity or a body carrying a newline forges a section here as readily as a
// blocker does.
func preExistingSection(issues []PreExistingIssue) string {
	items := make([]string, 0, len(issues))
	for _, issue := range issues {
		items = append(items, fmt.Sprintf("**%s** — %s", issue.Severity, issue.Body))
	}
	return bulletSection("Pre-existing",
		"Already true on the base branch, not introduced here.", items)
}

// checkBullet renders one piece of model text as a list item.
//
// One-lined and clipped before it is defused. A list item is a container: everything
// after the "- " is block content, so "### Blocking" there is a heading exactly as it is
// at the margin, and so is an <h3> tag.
func checkBullet(s string) string {
	return "- " + defuseMarkup(clip(oneLine(s), maxCheckBullet))
}

// defuseMarkup makes model text unable to open a markdown block.
//
// It is the single door every model-written field passes through on its way into the
// check summary, prose and list item alike. This package assembles that summary under
// headings of its own, so text that can open a section can attribute a finding — or a
// clean bill of health — to the review.
//
// Two rules, which together are the closure. Markdown opens a block only at the start of
// a line, so [defuseLine] escapes the character that would let a line do it. Raw HTML is
// the exception that also works mid-line, so [defuseTags] escapes every bracket that
// could begin a tag.
//
// Line endings are normalised first; see [normalizeLineEndings].
//
// It over-escapes rather than parses. A "---" separator, a line-leading "1." and a
// four-space-indented code sample all render with a visible backslash. That cost falls
// on shapes a review summary does not need, and the alternative is a markdown parser in
// a sanitiser.
func defuseMarkup(s string) string {
	lines := strings.Split(normalizeLineEndings(s), "\n")
	for i, line := range lines {
		lines[i] = defuseLine(line)
	}
	return defuseTags(strings.Join(lines, "\n"))
}

// normalizeLineEndings rewrites every line ending to \n, so a split on \n sees the lines
// a markdown parser sees.
//
// CommonMark recognises three -- \n, \r and \r\n, and nothing else -- and Go splits on
// one of them. Each of the other two breaks a line-level rule while a parser still
// applies it. A trailing \r stops a run of - matching, and stops a fence close matching,
// while a parser still reads the heading and still closes the fence. A lone \r is a line
// ending a split does not break on at all, so everything after one arrives as a single
// line.
//
// Callers that inspect rather than rewrite normalise for the scan only. The bytes this
// driver publishes stay the agent's own.
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// defuseLine escapes the character that would let one line open a markdown block.
//
// Indentation is kept and the escape goes in front of the first character after it,
// which is where a backslash escape is honoured.
func defuseLine(line string) string {
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	rest := line[indent:]
	at := blockStart(rest)
	if at < 0 {
		return line
	}
	return line[:indent+at] + `\` + rest[at:]
}

// blockStart returns where in an unindented line the escape belongs, and -1 when the
// line opens no block.
//
// The cases are markdown's block starts rather than the forgeries seen so far: heading,
// blockquote, fence, bullet, thematic break, setext underline, ordered list. A list that
// grew by attack would be short of the next one.
func blockStart(s string) int {
	if s == "" {
		return -1
	}
	c := s[0]
	switch {
	case c == '#', c == '>':
		return 0
	case (c == '`' || c == '~') && leadingRun(s) >= 3:
		return 0
	case bulletMarker(s), thematicBreak(s), setextUnderline(s):
		return 0
	}
	return orderedMarker(s)
}

// bulletMarker reports whether s opens a bullet list: -, + or * followed by a space or
// nothing. A list item is a container, so what follows the marker can be a heading.
func bulletMarker(s string) bool {
	if s[0] != '-' && s[0] != '+' && s[0] != '*' {
		return false
	}
	return len(s) == 1 || s[1] == ' ' || s[1] == '\t'
}

// thematicBreak reports whether s is a horizontal rule: three or more of -, _ or *, with
// only spaces and tabs between them. A rule divides the summary where this package did
// not, which is the visual half of forging a section.
func thematicBreak(s string) bool {
	if s[0] != '-' && s[0] != '_' && s[0] != '*' {
		return false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case s[0]:
			n++
		case ' ', '\t':
		default:
			return false
		}
	}
	return n >= 3
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

// orderedMarker returns the index of the . or ) that opens an ordered list, and -1 when
// s opens none.
//
// The delimiter is what gets escaped, not the digits. A backslash is honoured before
// ASCII punctuation only, so one before a digit renders as a stray backslash and leaves
// the marker live.
func orderedMarker(s string) int {
	i := 0
	for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) || (s[i] != '.' && s[i] != ')') {
		return -1
	}
	if i+1 < len(s) && s[i+1] != ' ' && s[i+1] != '\t' {
		return -1
	}
	return i
}

// leadingRun counts how many times a line's first byte repeats at its front.
func leadingRun(s string) int {
	n := 0
	for n < len(s) && s[n] == s[0] {
		n++
	}
	return n
}

// defuseTags escapes every angle bracket that could begin an HTML tag, comment or
// declaration.
//
// Anywhere in the text, not only at the start of a line. Raw HTML is the one markup that
// works mid-paragraph: GitHub renders an <h3> there as a heading, and an unclosed <!--
// hides every section written after it. A bracket that cannot begin a tag — "ch <- x",
// "a < b" — is left as it was written.
func defuseTags(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + len(s)/8)
	backslashes := 0
	for i := 0; i < len(s); i++ {
		// Only escape a tag this text has not escaped already. A backslash escapes
		// the next character only when the run before it is even -- "\\<" is a
		// literal backslash and a live tag -- so adding one unconditionally revives
		// what the model had already made inert.
		//
		// Block starts need no such counting: the renderer decides them from the raw
		// line before it processes escapes, so a leading backslash keeps a heading
		// from opening whatever the parity.
		if s[i] == '<' && i+1 < len(s) && tagStart(s[i+1]) && backslashes%2 == 0 {
			b.WriteByte('\\')
		}
		if s[i] == '\\' {
			backslashes++
		} else {
			backslashes = 0
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// tagStart reports whether a byte after < can begin a tag name, a closing tag, a comment
// or declaration, or a processing instruction.
func tagStart(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c == '/', c == '!', c == '?':
		return true
	}
	return false
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
// list with no review entry, which reads as a review that did not run rather than one
// that could not be read. Those are different things, and only one of them means an
// operator should look.
//
// The conclusion is neutral rather than failure. It says this tool could not read a
// review, not that the change is bad. A repository that wants that distinction to
// block can require the check. The reason it carries is the one [Verdict] already
// computed on every refusal path and nothing published.
//
// The reason quotes model text on one path — the decision word a reply wrote that this
// driver does not accept — so it is defused like every other field. This is the check a
// planted block produces, which makes it the one an attacker can aim at.
func BuildFailureCheck(v Verdict) CheckRun {
	reason := v.Reason
	if reason == "" {
		reason = "the turn produced no reply this driver could read"
	}
	return CheckRun{
		Title:      "no verdict",
		Conclusion: "neutral",
		Summary: "This review produced no decision that could be read mechanically.\n\n" +
			defuseMarkup(clip(oneLine(reason), maxCheckBullet)) +
			"\n\nThe agent's own words, if it wrote any, are in the published comment.",
	}
}
