package xreview

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxBodyBytes bounds the comment body this driver will write.
//
// GitHub rejects an issue comment over 65,536 characters. The margin below that covers
// the calling workflow's own envelope. What that envelope holds is not this driver's to
// know: today a marker line, tomorrow whatever framing it grows. Guessing tight buys
// nothing and costs a red job after a good review.
//
// Bytes rather than characters, because len is bytes and bytes are never fewer than
// runes. So a body inside this bound is inside GitHub's limit, whichever unit that limit
// turns out to count.
const MaxBodyBytes = 60_000

// RenderComment renders the body to post for a verdict.
//
// It truncates and publishes rather than refusing. A review that ran, cost model spend
// and held a sandbox for minutes must not be discarded over a formatting limit. A red
// job after a good review is a second failure, not a safety measure.
//
// Every published byte is still the agent's own. What truncation costs is contiguity.
// The notice says so, names how much went missing, and points at the item the rest can
// be read from. The closing block is re-appended after the cut, so a truncated review is
// still machine-readable. That is what matters the moment anything acts on the decision.
func RenderComment(v Verdict, sessionID string) string {
	footer := v.footer(sessionID)
	prose := strings.TrimRight(v.Text, "\n")

	if len(prose)+len(footer) <= MaxBodyBytes {
		return prose + footer
	}

	// The notice states the whole reply's size rather than how much was elided. The elided
	// count depends on where the cut lands, which depends on the notice's own length. That
	// circularity is not worth a second pass for a number a reader cannot act on anyway.
	// Where to find the rest is actionable.
	notice := fmt.Sprintf(
		"\n\n> **Review truncated by the publisher.** The whole reply is %d bytes: "+
			"item `%s` of session `%s`.\n\n",
		len(prose), v.ItemID, sessionID)

	// Room for the close a cut inside a code block or an HTML comment needs, reserved
	// unconditionally so every kind of cut is bounded the same way. Measured from the
	// whole reply: whatever a prefix leaves open was opened by something in the whole,
	// so the whole bounds the close any cut of it can need.
	reserve := markupReserve(prose)
	budget := MaxBodyBytes - len(footer) - len(notice) - len(v.Block) - reserve
	if budget < minProseBytes {
		// The closing block alone does not fit, which takes a findings array of a few
		// thousand entries. The decision still travels in the footer, and the notice still
		// says where to read the rest. Both beat refusing to publish a review that ran.
		// Room for the close too, since the cut may land inside a code block here
		// exactly as it can above.
		head := closeDanglingMarkup(truncateBytes(prose,
			MaxBodyBytes-len(footer)-len(notice)-reserve))
		return head + notice + footer
	}

	return closeDanglingMarkup(truncateBytes(prose, budget)) + notice + v.Block + footer
}

// closeDanglingMarkup closes the construct a cut landed inside.
//
// Both truncation paths need it, and for the same reason. An unclosed fence renders
// everything after it as code; an unclosed HTML comment hides everything after it
// outright. Either way the notice stops reading as a notice and the closing block stops
// reading as a decision -- a truncated review that no longer says it was truncated, or
// no longer appears at all.
//
// Counts only a fence that opens a line, which is the only place CommonMark opens one.
// Counting every occurrence lets a fence named in prose, or one inside a code span,
// flip the parity and either close a block that was never open or leave one open.
//
// The two constructs are tracked together rather than in two passes, because each hides
// the other: <!-- inside a code block is code, not a comment, and a fence inside a
// comment is comment text, not a fence. At most one is ever open, so at most one close
// is appended.
func closeDanglingMarkup(s string) string {
	char, length, inComment := openMarkup(s)
	switch {
	case inComment:
		return s + commentClose
	case length > 0:
		return s + "\n" + strings.Repeat(string(char), length) + "\n"
	}
	return s
}

// commentClose closes an HTML comment a truncation left open.
const commentClose = "\n-->\n"

// openMarkup reports what is still open at the end of s: a code fence, given by its
// character and how long it is, or an HTML comment.
//
// CommonMark closes a fence only with the same character, at least as long as the
// opener, and with nothing but whitespace after it -- an info string marks an opener,
// so a line carrying one never closes. One boolean cannot hold that: a ~~~ line
// inside an open ``` block is content, not a close, and backticks do not close a
// tilde block however many of them are appended.
func openMarkup(s string) (char byte, length int, inComment bool) {
	for _, line := range strings.Split(s, "\n") {
		switch {
		case inComment:
			// Only --> closes a comment. A fence line inside one is comment text.
			at := strings.Index(line, "-->")
			if at < 0 {
				continue
			}
			inComment = commentOpens(line[at+len("-->"):])
		case length > 0:
			// Only a matching fence line closes a block. <!-- inside one is code.
			trimmed := strings.TrimLeft(line, " \t")
			if run := fenceRun(trimmed); run > 0 && trimmed[0] == char &&
				run >= length && strings.TrimRight(trimmed[run:], " \t") == "" {
				length = 0
			}
		default:
			trimmed := strings.TrimLeft(line, " \t")
			if run := fenceRun(trimmed); run > 0 {
				char, length = trimmed[0], run
				continue
			}
			inComment = commentOpens(line)
		}
	}
	return char, length, inComment
}

// commentOpens reports whether a line leaves an HTML comment open, which is true when
// the last <!-- on it has no --> after it. Comments do not nest, so the last opener is
// the only one that can still be open.
func commentOpens(line string) bool {
	at := strings.LastIndex(line, "<!--")
	return at >= 0 && !strings.Contains(line[at:], "-->")
}

// markupReserve is the bytes to hold back for whichever close a cut in s may need, or
// zero when s carries nothing that can be left open.
//
// The longest fence line bounds the fence close: whichever fence a cut leaves open was
// opened by a line in s, and a close is never longer than its opener. A comment close is
// fixed. At most one close is appended, so the larger of the two covers both.
func markupReserve(s string) int {
	longest := 0
	for _, line := range strings.Split(s, "\n") {
		if run := fenceRun(strings.TrimLeft(line, " \t")); run > longest {
			longest = run
		}
	}
	fence := 0
	if longest > 0 {
		fence = longest + len("\n\n")
	}
	comment := 0
	if strings.Contains(s, "<!--") {
		comment = len(commentClose)
	}
	return max(fence, comment)
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

// minProseBytes is the least review text worth publishing alongside a closing
// block. Below it, the block is so large that carrying it would crowd out the
// review itself, and [RenderComment] drops the block instead.
const minProseBytes = 4096

// footer names the comment's own provenance.
//
// Actions logs expire and a pull request comment does not. So this is the only record
// that makes a wrong-session or wrong-turn publish discoverable after the fact.
func (v Verdict) footer(sessionID string) string {
	return fmt.Sprintf(
		"\n\n<sub>seidroid xreview · decision `%s` · session `%s` · turn `%s` · item `%s`</sub>\n",
		v.Decision(), sessionID, v.TurnID, v.ItemID)
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
