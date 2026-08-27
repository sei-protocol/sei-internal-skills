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
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := ParseVerdict(tc.prose + "\n" + tc.block)
			body := RenderComment(v, "conv_1")

			if len(body) > MaxBodyBytes {
				t.Errorf("body = %d bytes, want at most %d", len(body), MaxBodyBytes)
			}
			if endsInsideACodeBlock(body) {
				t.Error("the body ends inside a code block, so the notice and the " +
					"decision render as code")
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
	if got := closeDanglingFence(s); got != s {
		t.Errorf("appended a close to balanced text; the inline mention was counted:\n%q", got)
	}
	// Genuinely open, with a mention after it.
	open := "```go\nx()\nUse ``` inline here.\n"
	if got := closeDanglingFence(open); got == open {
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := closeDanglingFence(tc.in)
			if got != tc.in+tc.want {
				t.Errorf("closeDanglingFence(%q)\n got %q\nwant %q", tc.in, got, tc.in+tc.want)
			}
			if endsInsideACodeBlock(got) {
				t.Errorf("still open after closing: %q", got)
			}
		})
	}
}

// endsInsideACodeBlock is the test's own reading of CommonMark's fence rules, spelled
// out rather than calling the package's openFence: an oracle that shares the code's
// model agrees with the code's bugs. The previous oracle was the same single boolean,
// counting only backticks, and passed on every case above.
//
// The rules: a fence opens on three or more backticks or tildes at the start of a line,
// and closes only on the same character, at least as long, with nothing but whitespace
// after it.
func endsInsideACodeBlock(s string) bool {
	var openChar byte
	openLen := 0
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimLeft(line, " \t")
		if line == "" || (line[0] != '`' && line[0] != '~') {
			continue
		}
		run := 0
		for run < len(line) && line[run] == line[0] {
			run++
		}
		if run < 3 {
			continue
		}
		if openLen == 0 {
			openChar, openLen = line[0], run
			continue
		}
		if line[0] == openChar && run >= openLen &&
			strings.TrimRight(line[run:], " \t") == "" {
			openLen = 0
		}
	}
	return openLen != 0
}
