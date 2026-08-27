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

	// Reserved unconditionally, so a cut that lands inside a code block and one
	// that does not are bounded the same way.
	// The fence close a cut inside a code block may need; see closeDanglingFence.
	budget := MaxBodyBytes - len(footer) - len(notice) - len(v.Block) - len(danglingFenceClose)
	if budget < minProseBytes {
		// The closing block alone does not fit, which takes a findings array of a few
		// thousand entries. The decision still travels in the footer, and the notice still
		// says where to read the rest. Both beat refusing to publish a review that ran.
		// Room for the fence close too, since the cut may land inside a code block here
		// exactly as it can above.
		head := closeDanglingFence(truncateBytes(prose,
			MaxBodyBytes-len(footer)-len(notice)-len(danglingFenceClose)))
		return head + notice + footer
	}

	return closeDanglingFence(truncateBytes(prose, budget)) + notice + v.Block + footer
}

// closeDanglingFence appends a fence close when a cut landed inside a code block.
//
// Both truncation paths need it. An unclosed fence renders everything after it as code,
// so the notice would stop reading as a notice and the closing block would stop reading
// as a decision -- a truncated review that no longer says it was truncated.
//
// Counts only a fence that opens a line, which is the only place CommonMark opens one.
// Counting every occurrence lets a fence named in prose, or one inside a code span,
// flip the parity and either close a block that was never open or leave one open.
func closeDanglingFence(s string) string {
	open := false
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimLeft(line, " \t"); strings.HasPrefix(trimmed, "```") ||
			strings.HasPrefix(trimmed, "~~~") {
			open = !open
		}
	}
	if !open {
		return s
	}
	return s + danglingFenceClose
}

// danglingFenceClose closes a code block a truncation left open.
const danglingFenceClose = "\n```\n"

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
