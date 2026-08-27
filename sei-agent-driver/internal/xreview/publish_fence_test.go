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

	for _, tc := range []struct {
		name  string
		block string
	}{
		{"the ordinary cut", "```json\n{\"decision\":\"approve\",\"summary\":\"s\"}\n```"},
		// A block larger than the budget takes the fallback, which drops the block.
		{"the oversized-block fallback", "```json\n{\"decision\":\"approve\",\"summary\":\"" +
			strings.Repeat("x", MaxBodyBytes) + "\"}\n```"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := ParseVerdict(inFence + "\n" + tc.block)
			body := RenderComment(v, "conv_1")

			if len(body) > MaxBodyBytes {
				t.Errorf("body = %d bytes, want at most %d", len(body), MaxBodyBytes)
			}
			// The parity a reader's renderer sees: only a line-leading fence opens one.
			open := false
			for _, line := range strings.Split(body, "\n") {
				if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
					open = !open
				}
			}
			if open {
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
