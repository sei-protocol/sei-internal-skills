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
// Each construct hides the other: whichever opens first holds until it closes.
func endsInsideMarkup(s string) (fence, comment bool) {
	var openChar byte
	openLen := 0
	for _, raw := range strings.Split(s, "\n") {
		if comment {
			at := strings.Index(raw, "-->")
			if at < 0 {
				continue
			}
			comment = lastOpenerIsBare(raw[at+3:])
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
			comment = lastOpenerIsBare(raw)
		}
	}
	return openLen != 0, comment
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
