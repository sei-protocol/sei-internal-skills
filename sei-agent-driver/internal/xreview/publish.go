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
// be read from. The decision block rides with it, so a truncated review is still
// machine-readable. That is what matters the moment anything acts on the decision.
//
// On the truncated path the notice, the decision and the provenance all come before the
// cut text, and that order is the whole guard. A cut can leave a code fence, an HTML
// comment or a raw <pre> open, and each of those hides or garbles whatever follows it --
// including nested inside each other, where closing correctly needs a model of HTML
// nesting rather than of markdown. Putting them ahead of the cut needs no model at all.
// [closeDanglingMarkup] still runs on the tail, but only so the tail itself renders; the
// notice no longer depends on it.
func RenderComment(v Verdict, sessionID string) string {
	footer := v.footer(sessionID)
	prose := strings.TrimRight(v.Text, "\n")

	// Closed even when nothing was cut: a reply that ends inside its own fence would
	// otherwise render the footer as code and lose the provenance record.
	//
	// Measured, not reserved. The whole string is in hand here, so what the close
	// actually costs is knowable -- nothing at all for the balanced reply, which is
	// almost all of them. markupReserve answers a different question, the worst case
	// over an unknown cut, and using it here truncated replies that fitted.
	if closed := closeDanglingMarkup(prose); len(closed)+len(footer) <= MaxBodyBytes {
		return closed + footer
	}

	// The notice states the whole reply's size rather than how much was elided. The elided
	// count depends on where the cut lands, which depends on the notice's own length. That
	// circularity is not worth a second pass for a number a reader cannot act on anyway.
	// Where to find the rest is actionable.
	notice := fmt.Sprintf(
		"> **Review truncated by the publisher.** The whole reply is %d bytes and the "+
			"text below is cut. Read the rest at item `%s` of session `%s`.\n\n",
		len(prose), v.ItemID, sessionID)

	// Here the reserve is right: the cut point is not known until the budget is, so
	// the bound has to hold for whichever construct the cut leaves open.
	reserve := markupReserve(prose)

	lead := notice + v.Block + footer + proseSeparator
	if MaxBodyBytes-len(lead)-reserve < minProseBytes {
		// The closing block alone crowds out the review, which takes a findings array
		// of a few thousand entries. The decision still travels in the footer, and the
		// notice still says where to read the rest. Both beat refusing to publish a
		// review that ran.
		lead = notice + footer + proseSeparator
	}

	return lead + closeDanglingMarkup(truncateBytes(prose, MaxBodyBytes-len(lead)-reserve))
}

// proseSeparator divides the decision from the cut review text that follows it.
const proseSeparator = "\n\n---\n\n"

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
// The constructs are tracked together rather than in separate passes, because each hides
// the others: <!-- inside a code block is code, not a comment, and a fence inside a
// comment is comment text, not a fence. At most one is ever open, so at most one close
// is appended.
//
// The third is a raw <pre> or <textarea>. CommonMark runs those to their close tag past
// blank lines, so an unclosed one swallows what follows exactly as a comment does. The
// blank-line-terminated tags -- <details>, <table>, <div> -- are not tracked: the notice
// opens with a blank line, which ends them.
//
// The scan reads normalised line endings and the returned string does not: a \r\n reply
// keeps its own bytes and gains only the close.
func closeDanglingMarkup(s string) string {
	open := openMarkup(s)
	switch {
	case open.comment:
		return s + commentClose
	case open.tag != "":
		return s + "\n</" + open.tag + ">\n"
	case open.fenceLen > 0:
		return s + "\n" + strings.Repeat(string(open.fenceChar), open.fenceLen) + "\n"
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
func openMarkup(s string) (open markup) {
	for _, line := range strings.Split(normalizeLineEndings(s), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		switch {
		case open.comment:
			// Only --> closes a comment. Everything else inside one is comment text.
			at := strings.Index(line, "-->")
			if at < 0 {
				continue
			}
			open.comment = commentOpens(line[at+len("-->"):])
		case open.tag != "":
			// Only the matching end tag closes one of these, and it closes from
			// anywhere on the line.
			if strings.Contains(strings.ToLower(line), "</"+open.tag+">") {
				open.tag = ""
			}
		case open.fenceLen > 0:
			// Only a matching fence line closes a block. <!-- inside one is code.
			if run := fenceRun(trimmed); run > 0 && trimmed[0] == open.fenceChar &&
				run >= open.fenceLen && strings.TrimRight(trimmed[run:], " \t") == "" {
				open.fenceLen = 0
			}
		default:
			if run := fenceRun(trimmed); run > 0 {
				open.fenceChar, open.fenceLen = trimmed[0], run
				continue
			}
			if tag := rawBlockTag(trimmed); tag != "" {
				open.tag = tag
				continue
			}
			open.comment = commentOpens(line)
		}
	}
	return open
}

// markup is the one construct open at a point in a document: a fence, an HTML comment,
// or a raw block tag. At most one field is ever set.
type markup struct {
	fenceChar byte
	fenceLen  int
	comment   bool
	tag       string
}

// rawBlockTag is the tag a line opens a raw HTML block with, lowercased, or "" for none.
//
// These are the tags CommonMark runs to their end tag rather than to a blank line, which
// is what makes an unclosed one able to swallow the notice. The tag has to start the
// line, and what follows it has to be whitespace, a > or nothing -- <president> is not
// <pre>.
func rawBlockTag(trimmed string) string {
	if trimmed == "" || trimmed[0] != '<' {
		return ""
	}
	lower := strings.ToLower(trimmed)
	for _, tag := range rawBlockTags {
		rest, ok := strings.CutPrefix(lower, "<"+tag)
		if !ok {
			continue
		}
		if rest == "" || rest[0] == '>' || rest[0] == ' ' || rest[0] == '\t' {
			return tag
		}
	}
	return ""
}

// rawBlockTags are CommonMark's HTML block type 1 tags. GitHub strips script and style,
// so only pre and textarea can reach a reader -- the other two are tracked because the
// parser treats all four the same and a stripped tag still ends a block.
var rawBlockTags = []string{"pre", "script", "style", "textarea"}

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
// fixed, and a raw block's is its own end tag. At most one close is appended, so the
// largest covers all three.
func markupReserve(s string) int {
	longest := 0
	for _, line := range strings.Split(normalizeLineEndings(s), "\n") {
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
	tag := 0
	for _, line := range strings.Split(normalizeLineEndings(s), "\n") {
		if t := rawBlockTag(strings.TrimLeft(line, " \t")); t != "" {
			tag = max(tag, len("\n</"+t+">\n"))
		}
	}
	return max(fence, max(comment, tag))
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
