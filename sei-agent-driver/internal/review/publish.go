package review

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxBodyBytes bounds the comment body this driver will write.
//
// GitHub rejects a review or issue comment over 65,536 characters. The margin below that
// covers the calling workflow's own envelope. What that envelope holds is not this
// driver's to know: today a marker line and a findings line, tomorrow whatever framing it
// grows. Guessing tight buys nothing and costs a red job after a good review.
//
// Bytes rather than characters, because len is bytes and bytes are never fewer than
// runes. So a body inside this bound is inside GitHub's limit, whichever unit that limit
// turns out to count.
const MaxBodyBytes = 60_000

// RenderComment renders the body to post for a verdict.
//
// The body is composed from the closing block, in the order a reader wants it: the
// summary, the recorded position where it departs from the word the reply wrote, then
// Blocking, Non-blocking and Pre-existing. The reply's prose is not published. A prose
// review is written in whatever order and at whatever length the model chose, and the
// two complaints a reader has about it -- too long, and the verdict at the bottom -- are
// both answered by rendering the block instead.
//
// The line-tied findings the caller posts inline are not repeated here; each severity
// section counts them, so the body and the inline comments read as one review. A
// line-tied finding the caller cannot post -- no usable line, or past the placement cap --
// is listed under its severity with its location. Each section is bounded as the check
// summary's are, and a section that overran says so and names the session item that
// holds the rest.
//
// A nit this run does not admit is listed collapsed, so the reader who wants it can open
// it and the one who does not is not made to read it. Every field is model text beside
// framing it must not be able to imitate, so it goes through [defuseMarkup].
//
// It truncates and publishes rather than refusing. A review that ran, cost model spend
// and held a sandbox for minutes must not be discarded over a formatting limit. On that
// path the notice and the footer come before the cut text, and that order is the whole
// guard: the one construct a cut can leave open is this package's own <details>, which
// GitHub closes at the end of the document, so whatever follows the cut would render
// folded. Nothing follows it.
func RenderComment(v Verdict, includeNits bool, sessionID string) string {
	footer := v.footer(sessionID)
	body := strings.TrimRight(reviewBody(v, includeNits), "\n")
	if len(body)+len(footer) <= MaxBodyBytes {
		return body + footer
	}
	notice := fmt.Sprintf(
		"> **Review truncated by the publisher.** The rendered review is %d bytes and the "+
			"text below is cut. Read the whole of it at item `%s` of session `%s`.",
		len(body), v.ItemID, sessionID)
	lead := notice + footer + "\n---\n\n"
	return lead + truncateBytes(body, MaxBodyBytes-len(lead))
}

// reviewBody renders the summary and every section a review has something to say under.
//
// Sections with nothing in them are omitted rather than rendered empty. A clean review
// is its summary and its footer, which is the shortest true thing to publish.
func reviewBody(v Verdict, includeNits bool) string {
	placed := placedBySeverity(v, includeNits)
	sections := []string{
		v.position(),
		bulletSection("Blocking", inlineLead(placed["blocker"]),
			append(Blockers(v), unplacedBullets(v, includeNits, true)...), inTheSession),
		bulletSection("Non-blocking", inlineLead(placed["suggestion"]+placed["nit"]+placed[""]),
			append(NonBlockers(v), unplacedBullets(v, includeNits, false)...), inTheSession),
		preExistingSection(PreExisting(v), v.acceptedSource(), inTheSession),
		nitSection(v, includeNits),
	}
	out := make([]string, 0, len(sections)+1)
	if prose := clipProse(defuseMarkup(v.Summary()), 0, inTheSession); prose != "" {
		out = append(out, prose)
	}
	for _, s := range sections {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, sectionSeparator)
}

// placedBySeverity counts the findings handed to the caller for inline placement, by the
// severity each will be posted under. It is what lets the body say "and N more on the
// changed lines" without repeating them.
func placedBySeverity(v Verdict, includeNits bool) map[string]int {
	out := make(map[string]int)
	for _, f := range placeableFindings(v, includeNits) {
		out[f.Severity]++
	}
	return out
}

// inlineLead is the line under a section heading that counts the findings of that
// severity posted on the code, or nothing when none were.
func inlineLead(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("_%s on the changed lines, as inline comments._", plural(n, "finding"))
}

// unplacedBullets renders the line-tied findings the caller will not post: the ones with
// no usable line, and the ones past the placement cap. blocking selects which severity
// they are rendered under. A suppressed nit is not one of these; [nitSection] has it.
func unplacedBullets(v Verdict, includeNits, blocking bool) []string {
	placed := make(map[string]bool)
	for _, f := range placeableFindings(v, includeNits) {
		placed[f.dedupeKey()] = true
	}
	seen := make(map[string]bool)
	var out []string
	for _, entry := range reportedFindings(v) {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		f := findingFrom(fields)
		if f.Detail == "" || (f.Severity == "nit" && !includeNits) {
			continue
		}
		if (f.Severity == "blocker") != blocking {
			continue
		}
		key := f.dedupeKey()
		if placed[key] || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, locatedBullet(f))
	}
	return out
}

// locatedBullet renders a line-tied finding as text, with the place it names ahead of
// what it says. The location is model text like the rest and is defused with it.
func locatedBullet(f Finding) string {
	where := f.File
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	if where == "" {
		return f.Detail
	}
	return fmt.Sprintf("`%s` — %s", strings.ReplaceAll(oneLine(where), "`", "'"), f.Detail)
}

// nitSection renders the nits this run does not post, collapsed.
//
// Collapsed rather than omitted, because a nit the review wrote is still a nit the author
// may want; and collapsed rather than open, because the reader who asked for no nits is
// the one who does not. Nothing when the run admits nits -- those are posted inline like
// any other finding -- and nothing when there are none.
func nitSection(v Verdict, includeNits bool) string {
	if includeNits {
		return ""
	}
	seen := make(map[string]bool)
	var items []string
	for _, entry := range reportedFindings(v) {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		f := findingFrom(fields)
		if f.Severity != "nit" || f.Detail == "" || seen[f.dedupeKey()] {
			continue
		}
		seen[f.dedupeKey()] = true
		items = append(items, locatedBullet(f))
	}
	if len(items) == 0 {
		return ""
	}
	list := bulletSection("Nits", "", items, inTheSession)
	list = strings.TrimPrefix(list, "### Nits\n")
	return fmt.Sprintf("<details>\n<summary>%s, not posted on the code</summary>\n\n%s\n\n</details>",
		plural(len(items), "nit"), list)
}

// footer names the comment's own provenance.
//
// Actions logs expire and a pull request comment does not. So this is the only record
// that makes a wrong-session or wrong-turn publish discoverable after the fact. The
// decision it carries is the recorded one.
func (v Verdict) footer(sessionID string) string {
	if v.SettledBy != "" {
		return fmt.Sprintf(
			"\n\n<sub>seidroid review · decision `%s` · settled by the scouts %s · no review turn ran</sub>\n",
			v.Decision(), v.SettledBy)
	}
	return fmt.Sprintf(
		"\n\n<sub>seidroid review · decision `%s` · session `%s` · turn `%s` · item `%s`</sub>\n",
		v.Decision(), sessionID, v.TurnID, v.ItemID)
}

// fenceRun is the length of the fence a line opens or closes, or zero if it is not a
// fence line: three or more backticks or tildes, at the start of the line.
func fenceRun(trimmed string) int {
	if trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0
	}
	if run := leadingRun(trimmed); run >= 3 {
		return run
	}
	return 0
}

// truncateBytes cuts to at most max bytes, preferring the last line break so the
// cut lands somewhere a reader expects, and never splitting a rune.
func truncateBytes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	cut := max
	if nl := strings.LastIndexByte(s[:cut], '\n'); nl > max/2 {
		cut = nl
	}
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
