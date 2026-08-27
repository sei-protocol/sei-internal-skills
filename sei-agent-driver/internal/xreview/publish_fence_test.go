package xreview

import (
	"strings"
	"testing"
)

// TestATruncatedCommentStillReadsAsTruncated covers both cut paths.
//
// An unclosed fence renders everything after it as code, so the notice stops reading as
// a notice and the closing block stops reading as a decision. The oversized-block
// fallback had no rebalancing at all, and the ordinary path counted every "```" -- so a
// fence named in prose flipped the parity and either closed a block that was never open
// or left one open.
func TestATruncatedCommentStillReadsAsTruncated(t *testing.T) {
	t.Parallel()

	// The fence opens early and never closes, so a cut anywhere after it lands inside
	// the code block. A fixture that opens the fence late is cut before reaching it,
	// leaves a balanced head, and passes whether or not the rebalancing runs.
	inFence := "```go\nfunc x() {\n" + strings.Repeat("\t// padding\n", 12000)
	// A tilde block holding a ``` line, and a long backtick fence a shorter one
	// cannot close. Both are open at the cut, and neither is closed by the three
	// backticks the publisher used to append unconditionally.
	inTildes := "~~~go\nfunc x() {\n```\n" + strings.Repeat("\t// padding\n", 12000)
	inLongFence := "`````go\nfunc x() {\n```\n" + strings.Repeat("\t// padding\n", 12000)
	// An HTML comment the cut lands inside. A review that quotes another bot's
	// comment carries these: <!-- BUGBOT_BUG_ID: ... --> is one. GitHub hides
	// everything after an unclosed one, so the notice and footer vanish outright
	// rather than merely rendering as code.
	// The comment must still be open at the cut, so its --> sits past the budget.
	inComment := "Quoting the report:\n<!-- DESCRIPTION START -->\n<!-- BUGBOT_BUG_ID: " +
		strings.Repeat("0123456789abcdef\n", 5000) + " -->\n"
	// A reply that arrives with CRLF endings throughout, cut inside the block.
	inCRLF := strings.ReplaceAll(inFence, "\n", "\r\n")

	for _, tc := range []struct {
		name  string
		prose string
		block string
	}{
		{"the ordinary cut", inFence,
			"```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```"},
		// A block larger than the budget takes the fallback, which drops the block.
		{"the oversized-block fallback", inFence,
			"```json\n{\"decision\":\"approve\",\"summary\":\"" +
				strings.Repeat("x", MaxBodyBytes) + "\"}\n```"},
		{"a tilde block the cut lands in", inTildes,
			"```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```"},
		{"a fence longer than its closer", inLongFence,
			"```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```"},
		{"an HTML comment the cut lands in", inComment,
			"```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```"},
		{"a CRLF reply the cut lands in", inCRLF,
			"```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := ParseVerdict(tc.prose + "\n" + tc.block)
			body := RenderComment(v, "conv_1")

			if len(body) > MaxBodyBytes {
				t.Errorf("body = %d bytes, want at most %d", len(body), MaxBodyBytes)
			}
			if fence, comment := endsInsideMarkup(body); fence || comment {
				what := "a code block, so the notice and the decision render as code"
				if comment {
					what = "an HTML comment, so the notice and the decision are hidden"
				}
				t.Errorf("the body ends inside %s", what)
			}
		})
	}
}

// TestAFenceNamedInProseDoesNotFlipTheParity pins the line-leading rule. Counting every
// occurrence let prose about fences decide whether the comment closed one.
func TestAFenceNamedInProseDoesNotFlipTheParity(t *testing.T) {
	t.Parallel()

	// Balanced: one real block, plus a mention inside a sentence.
	s := "```go\nx()\n```\nUse ``` to open a fence.\n"
	if got := closeDanglingMarkup(s); got != s {
		t.Errorf("appended a close to balanced text; the inline mention was counted:\n%q", got)
	}
	// Genuinely open, with a mention after it.
	open := "```go\nx()\nUse ``` inline here.\n"
	if got := closeDanglingMarkup(open); got == open {
		t.Error("left a code block open; the inline mention was read as its close")
	}
}

// TestTheFenceModelFollowsCommonMark pins the three rules one boolean cannot hold.
//
// Each case here rendered wrong before: the publisher toggled a single flag on any
// fence line and always appended three backticks.
func TestTheFenceModelFollowsCommonMark(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   string
		want string // the closer expected, "" for none
	}{
		{"a tilde line inside a backtick block is content",
			"```go\nx()\n~~~\n", "\n```\n"},
		{"a backtick line inside a tilde block is content",
			"~~~go\nx()\n```\n", "\n~~~\n"},
		{"a tilde block closes with tildes",
			"~~~go\nx()\n", "\n~~~\n"},
		{"a shorter fence does not close a longer one",
			"````go\nx()\n```\n", "\n````\n"},
		{"a longer fence does close a shorter one",
			"```go\nx()\n````\n", ""},
		{"an info string marks an opener, never a close",
			"```go\nx()\n```go\n", "\n```\n"},
		{"an indented close still closes",
			"```go\nx()\n   ```\n", ""},
		{"trailing whitespace does not stop a close",
			"```go\nx()\n```  \n", ""},
		{"balanced text is left alone", "```go\nx()\n```\n", ""},
		{"text with no fence at all is left alone", "just prose\n", ""},
		{"an open HTML comment closes with -->",
			"<!-- DESCRIPTION START -->\n<!-- open\n", "\n-->\n"},
		{"a closed HTML comment is left alone",
			"<!-- a note -->\nprose\n", ""},
		{"two comments on one line, the second open",
			"<!-- a --> text <!-- b\n", "\n-->\n"},
		{"comments do not nest, so the first --> closes",
			"<!-- a <!-- b -->\n", ""},
		{"a comment opened and closed across lines",
			"<!-- a\nb\n-->\n", ""},
		// Each construct hides the other, so neither may be scanned on its own.
		{"a fence inside a comment is comment text, not a fence",
			"<!-- a\n```go\n", "\n-->\n"},
		{"an open comment inside a fence is code, not a comment",
			"```go\n<!-- a\n", "\n```\n"},
		{"a comment closed inside a fence does not close the fence",
			"```go\n<!-- a -->\n", "\n```\n"},
		// CommonMark recognises three line endings and Go splits on one. A trailing
		// \r read as an info string, so a balanced fence never closed and a second
		// closer was appended -- which a renderer reads as a new opener, the very
		// failure this function exists to prevent.
		{"a balanced CRLF fence needs no close",
			"```go\r\nx()\r\n```\r\n", ""},
		{"an open CRLF fence closes",
			"```go\r\nx()\r\n", "\n```\n"},
		{"a balanced lone-CR fence needs no close",
			"```go\rx()\r```\r", ""},
		{"a CRLF tilde fence closes with tildes",
			"~~~go\r\nx()\r\n", "\n~~~\n"},
		{"a CRLF comment is seen as closed",
			"<!-- a -->\r\nprose\r\n", ""},
		// A raw <pre> runs to its end tag past blank lines, so an unclosed one hides
		// the notice the way a comment does. <details> and friends stop at a blank
		// line, and the notice opens with one, so they are not tracked.
		{"an open pre block closes with its end tag",
			"<pre>\nx()\n", "\n</pre>\n"},
		{"a closed pre block is left alone",
			"<pre>\nx()\n</pre>\nprose\n", ""},
		{"the end tag closes from anywhere on the line",
			"<pre>\nx()</pre> and more prose\n", ""},
		{"an open textarea closes with its own tag",
			"<textarea rows=2>\nx\n", "\n</textarea>\n"},
		{"the tag match is case-insensitive",
			"<PRE>\nx()\n", "\n</pre>\n"},
		{"a tag that merely starts the same is not one",
			"<president>\nx()\n", ""},
		{"a blank-line-terminated tag is not tracked",
			"<details>\nx()\n", ""},
		{"a pre inside a fence is code, not a raw block",
			"```html\n<pre>\n", "\n```\n"},
		{"a fence inside a pre is literal, not a fence",
			"<pre>\n```go\n", "\n</pre>\n"},
		{"a comment inside a pre is literal",
			"<pre>\n<!-- a\n", "\n</pre>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := closeDanglingMarkup(tc.in)
			if got != tc.in+tc.want {
				t.Errorf("closeDanglingMarkup(%q)\n got %q\nwant %q", tc.in, got, tc.in+tc.want)
			}
			if fence, comment := endsInsideMarkup(got); fence || comment {
				t.Errorf("still open after closing (fence=%v comment=%v): %q",
					fence, comment, got)
			}
		})
	}
}

// endsInsideMarkup is the test's own reading of the rules, spelled out rather than
// calling the package's openMarkup: an oracle that shares the code's model agrees with
// the code's bugs. The oracle here started as the same single boolean the code had,
// counting only backticks, and passed on every fence case above.
//
// The rules. A fence opens on three or more backticks or tildes at the start of a line,
// and closes only on the same character, at least as long, with nothing but whitespace
// after it. An HTML comment opens on <!-- and closes on the next -->, and does not nest.
// A raw block opens on <pre, <script, <style or <textarea at the start of a line and
// closes only on its own end tag, from anywhere on a line. Each construct hides the
// others: whichever opens first holds until it closes.
func endsInsideMarkup(s string) (fence, comment bool) {
	// The three line endings CommonMark recognises, folded to the one Go splits on.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var openChar byte
	openLen := 0
	openTag := ""
	for _, raw := range strings.Split(s, "\n") {
		if comment {
			at := strings.Index(raw, "-->")
			if at < 0 {
				continue
			}
			comment = lastOpenerIsBare(raw[at+3:])
			continue
		}
		if openTag != "" {
			if strings.Contains(strings.ToLower(raw), "</"+openTag+">") {
				openTag = ""
			}
			continue
		}
		line := strings.TrimLeft(raw, " \t")
		run := 0
		if line != "" && (line[0] == '`' || line[0] == '~') {
			for run < len(line) && line[run] == line[0] {
				run++
			}
			if run < 3 {
				run = 0
			}
		}
		switch {
		case openLen > 0:
			if run > 0 && line[0] == openChar && run >= openLen &&
				strings.TrimRight(line[run:], " \t") == "" {
				openLen = 0
			}
		case run > 0:
			openChar, openLen = line[0], run
		default:
			lower := strings.ToLower(line)
			for _, t := range []string{"pre", "script", "style", "textarea"} {
				rest, ok := strings.CutPrefix(lower, "<"+t)
				if ok && (rest == "" || rest[0] == '>' || rest[0] == ' ' || rest[0] == '\t') {
					openTag = t
					break
				}
			}
			if openTag == "" {
				comment = lastOpenerIsBare(raw)
			}
		}
	}
	return openLen != 0, comment || openTag != ""
}

// lastOpenerIsBare reports whether the final <!-- on a line has no --> after it.
func lastOpenerIsBare(line string) bool {
	at := strings.LastIndex(line, "<!--")
	return at >= 0 && !strings.Contains(line[at:], "-->")
}

// TestMarkupReserveHoldsBackEnoughForItsOwnCloser pins the budget side of the same rule.
//
// Tested directly rather than through RenderComment. A verdict's text always carries the
// closing block, so its fence alone reserves five bytes and hides whether the comment
// term is there at all -- an end-to-end fixture cannot tell the two apart.
func TestMarkupReserveHoldsBackEnoughForItsOwnCloser(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   string
		want int
	}{
		{"nothing that can be left open", "just prose\n", 0},
		{"an HTML comment", "<!-- open\n", len(commentClose)},
		{"a three-backtick fence", "```go\nx()\n", len("\n```\n")},
		{"a long fence reserves its own length", "``````go\nx()\n", len("\n``````\n")},
		{"a tilde fence", "~~~go\nx()\n", len("\n~~~\n")},
		{"the longer of the two wins", "``````go\nx()\n<!-- open\n", len("\n``````\n")},
		// Without normalising, a lone \r leaves this as one line that does not begin
		// with a fence, so the longest run reads as zero while the scan still finds a
		// fence open -- and the body overruns by the closer it did not reserve.
		{"a lone-CR ending before the fence", "text\r``````go\r", len("\n``````\n")},
		{"a CRLF fence reserves its own length", "``````go\r\nx()\r\n", len("\n``````\n")},
		{"a raw block reserves its end tag", "<textarea>\nx\n", len("\n</textarea>\n")},
		{"the longest close of the three wins",
			"<pre>\n```\n<!-- a\n", len("\n</pre>\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := markupReserve(tc.in); got != tc.want {
				t.Errorf("markupReserve(%q) = %d, want %d", tc.in, got, tc.want)
			}
			// The reserve has to cover the closer the same input actually produces.
			if closer := len(closeDanglingMarkup(tc.in)) - len(tc.in); closer > tc.want {
				t.Errorf("closer is %d bytes but only %d were reserved", closer, tc.want)
			}
		})
	}
}

// TestTheNoticeSurvivesMarkupNoCloserCanFix is why the notice precedes the cut text.
//
// Each fixture leaves two constructs open at once, nested. Appending one close cannot fix
// that: CommonMark ends a <pre> block at </pre>, but the HTML that reaches a reader is
// parsed by HTML rules, where an unterminated comment inside the <pre> runs on and takes
// the close, the notice and the decision with it. Getting that right needs a tag stack.
// Ordering needs nothing -- the notice is ahead of the cut, so no construct in the cut
// text can reach it.
func TestTheNoticeSurvivesMarkupNoCloserCanFix(t *testing.T) {
	t.Parallel()

	pad := strings.Repeat("a line of the review\n", 4000)
	for _, tc := range []struct {
		name  string
		prose string
	}{
		{"an HTML comment nested in a raw block", "<pre>\n<!-- still open\n" + pad},
		{"a textarea nested in a raw block", "<pre>\n<textarea>\n" + pad},
		{"a raw block nested in an HTML comment", "<!-- open\n<pre>\n" + pad},
		{"a fence nested in a raw block", "<pre>\n```go\n" + pad},
		{"three deep", "<pre>\n<!-- open\n```go\n" + pad},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := ParseVerdict(tc.prose +
				"\n```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```")
			v.TurnID, v.ItemID = "resp_claude_a", "item_reply"
			body := RenderComment(v, "conv_1")

			if len(body) > MaxBodyBytes {
				t.Errorf("body = %d bytes, want at most %d", len(body), MaxBodyBytes)
			}
			// The notice has to be the first thing in the body. Merely being present
			// is what the old order gave, and the old order is the bug.
			if !strings.HasPrefix(body, "> **Review truncated by the publisher.**") {
				t.Errorf("the body does not open with the notice, so the cut text is "+
					"ahead of it:\n%s", body[:min(300, len(body))])
			}
			cut := strings.Index(body, proseSeparator)
			if cut < 0 {
				t.Fatalf("no separator, so the cut text is not fenced off:\n%s", body[:300])
			}
			lead := body[:cut]
			for _, want := range []string{
				"Review truncated by the publisher",
				v.Block,
				// From the footer only. "conv_1" and "item_reply" also appear in the
				// notice, so neither shows whether the footer survived.
				"turn `resp_claude_a`",
				"decision `approve`",
			} {
				if !strings.Contains(lead, want) {
					t.Errorf("%q is not ahead of the cut text, so the reply can hide it",
						want)
				}
			}
			// And the lead itself must open nothing, or it would hide its own tail.
			if fence, comment := endsInsideMarkup(lead); fence || comment {
				t.Errorf("the lead leaves markup open (fence=%v comment=%v)", fence, comment)
			}
		})
	}
}

// TestAnUncutReplyThatEndsOpenStillShowsItsFooter covers the path with no truncation.
//
// The driver did not cut this one -- the agent's own reply ends inside its fence -- but
// the footer is still the only provenance record that outlives the run's logs, and an
// open fence renders it as code. Nothing here is about the size limit, so the bound is
// asserted too: closing costs bytes the untruncated path has to have counted.
func TestAnUncutReplyThatEndsOpenStillShowsItsFooter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, prose string }{
		{"ends inside a fence", "Here is what I found:\n```go\nx()"},
		{"ends inside an HTML comment", "Quoting:\n<!-- BUGBOT_BUG_ID: abc"},
		{"ends inside a raw block", "Output:\n<pre>\nx()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := ParseVerdict(tc.prose +
				"\n```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```")
			v.TurnID, v.ItemID = "resp_claude_a", "item_reply"
			body := RenderComment(v, "conv_1")

			if len(body) > MaxBodyBytes {
				t.Errorf("body = %d bytes, want at most %d", len(body), MaxBodyBytes)
			}
			if !strings.Contains(body, "turn `resp_claude_a`") {
				t.Fatalf("no footer in the body:\n%s", body[max(0, len(body)-200):])
			}
			// The footer must not be inside anything the reply left open.
			at := strings.Index(body, "<sub>seidroid xreview")
			if fence, comment := endsInsideMarkup(body[:at]); fence || comment {
				t.Errorf("the footer renders inside open markup (fence=%v comment=%v), "+
					"so the provenance record is lost", fence, comment)
			}
		})
	}
}

// TestTheUncutBoundCountsTheCloseItWillAppend sits the reply exactly on the limit.
//
// The uncut path appends a close, so its own bound has to count it or the body overruns
// by those bytes. The window is the width of one close, so the size is computed rather
// than guessed.
//
// The fence is four backticks on purpose. A three-backtick fence is closed by the verdict
// block's own closing fence, which leaves nothing open and nothing to append; four is
// longer than the block's three, so the block cannot close it.
func TestTheUncutBoundCountsTheCloseItWillAppend(t *testing.T) {
	t.Parallel()

	mk := func(pad int) Verdict {
		v := ParseVerdict("````go\n" + strings.Repeat("x", pad) +
			"\n```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```")
		v.TurnID, v.ItemID = "resp_claude_a", "item_reply"
		return v
	}

	empty := mk(0)
	base := len(strings.TrimRight(empty.Text, "\n")) + len(empty.footer("conv_1"))
	v := mk(MaxBodyBytes - base)

	prose := strings.TrimRight(v.Text, "\n")
	if got := len(prose) + len(v.footer("conv_1")); got != MaxBodyBytes {
		t.Fatalf("fixture is %d bytes with its footer, want exactly %d", got, MaxBodyBytes)
	}
	if _, length, _ := func() (byte, int, bool) {
		o := openMarkup(prose)
		return o.fenceChar, o.fenceLen, o.comment
	}(); length == 0 {
		t.Fatal("fixture leaves no fence open, so no close is appended and the bound " +
			"is not under test")
	}

	if body := RenderComment(v, "conv_1"); len(body) > MaxBodyBytes {
		t.Errorf("body = %d bytes, want at most %d: the uncut bound did not count the "+
			"close it appends", len(body), MaxBodyBytes)
	}
}

// TestABalancedReplyThatFitsIsNotTruncated pins the difference between the close a reply
// needs and the close a cut might need.
//
// Every verdict carries a balanced ```json block, so a worst-case reserve charges every
// reply for a close none of them uses. Bounding the uncut path that way truncated replies
// that fitted -- and a truncation notice on an untruncated review tells a reader to go
// find text that is already in front of them.
func TestABalancedReplyThatFitsIsNotTruncated(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, prose string }{
		{"a short balanced reply", "It looks fine to me."},
		{"a balanced fence in the prose", "Here:\n```go\nx()\n```\nThat is the issue."},
		{"a closed HTML comment", "Quoting:\n<!-- BUGBOT_BUG_ID: abc -->\nSame finding."},
		{"a closed raw block", "Output:\n<pre>\nx()\n</pre>\nSo it passes."},
		// Sized to sit just inside the limit with its footer, where a worst-case
		// reserve is the difference between publishing whole and being cut.
		{"balanced and just inside the limit", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mk := func(pad int) Verdict {
				body := tc.prose
				if tc.name == "balanced and just inside the limit" {
					body = strings.Repeat("x", pad)
				}
				v := ParseVerdict(body +
					"\n```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```")
				v.TurnID, v.ItemID = "resp_claude_a", "item_reply"
				return v
			}
			v := mk(0)
			if tc.name == "balanced and just inside the limit" {
				base := len(strings.TrimRight(v.Text, "\n")) + len(v.footer("conv_1"))
				v = mk(MaxBodyBytes - base)
			}

			body := RenderComment(v, "conv_1")
			if len(body) > MaxBodyBytes {
				t.Fatalf("body = %d bytes, want at most %d", len(body), MaxBodyBytes)
			}
			if strings.Contains(body, "Review truncated by the publisher") {
				t.Errorf("a reply of %d bytes that fits in %d was truncated; the uncut "+
					"bound charged it for a close it does not need",
					len(strings.TrimRight(v.Text, "\n")), MaxBodyBytes)
			}
			// Nothing was cut, so the reply's own text is published whole.
			if !strings.HasPrefix(body, strings.TrimRight(v.Text, "\n")) {
				t.Error("the reply's text was altered on a path that cut nothing")
			}
		})
	}
}
