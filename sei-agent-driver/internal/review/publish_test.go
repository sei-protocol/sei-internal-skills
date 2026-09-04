package review

import (
	"strings"
	"testing"
)

// TestRenderCommentPassesAShortReviewThrough checks the ordinary case: the agent's
// words verbatim, plus the provenance footer.
func TestRenderCommentPassesAShortReviewThrough(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("Two findings, both minor.\n\n```json\n{\"decision\": \"comment\"}\n```")
	v.TurnID = "resp_claude_a"
	v.ItemID = "item_reply"

	body := RenderComment(v, "conv_1")

	if !strings.HasPrefix(body, "Two findings, both minor.") {
		t.Errorf("body does not open with the agent's own words:\n%s", body)
	}
	if strings.Contains(body, "truncated") {
		t.Error("a short review must not be marked truncated")
	}
	for _, want := range []string{"conv_1", "resp_claude_a", "item_reply", "comment"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not name %q; the footer is the only provenance record that "+
				"outlives the run's logs", want)
		}
	}
}

// TestRenderCommentTruncatesRatherThanRefusing covers the oversize path.
//
// A review that ran, cost model spend and held a sandbox for minutes must not be
// discarded over a formatting limit. So this truncates and publishes.
//
// What it must still guarantee: the body fits GitHub's cap, the elision is declared, the
// closing block survives so the decision stays machine-readable, and the fences balance
// so the notice cannot be swallowed into a code block.
func TestRenderCommentTruncatesRatherThanRefusing(t *testing.T) {
	t.Parallel()

	// Opens a fence and never closes it, so the cut necessarily lands inside a
	// code block.
	prose := "```text\n" + strings.Repeat("a very long finding line\n", 4000)
	v := ParseVerdict(prose + "\n```json\n{\"decision\": \"request_changes\"}\n```")
	v.TurnID = "resp_claude_a"
	v.ItemID = "item_reply"

	if len(v.Text) <= MaxBodyBytes {
		t.Fatalf("fixture is only %d bytes, want more than MaxBodyBytes (%d)",
			len(v.Text), MaxBodyBytes)
	}

	body := RenderComment(v, "conv_1")

	if len(body) > MaxBodyBytes {
		t.Errorf("body = %d bytes, want at most MaxBodyBytes (%d)", len(body), MaxBodyBytes)
	}
	if !strings.Contains(body, "Review truncated by the publisher") {
		t.Errorf("the elision is not declared:\n%s", body[max(0, len(body)-400):])
	}
	if !strings.Contains(body, v.Block) {
		t.Error("the closing block did not survive truncation, so the decision is not " +
			"machine-readable")
	}
	if n := strings.Count(body, "```"); n%2 != 0 {
		t.Errorf("fence count = %d, want even: an unbalanced fence renders the notice and the "+
			"verdict as code, which changes what the comment appears to say", n)
	}
	if !strings.Contains(body, "item_reply") {
		t.Error("a truncated body must still point at the item the whole reply can be read from")
	}
}

// TestRenderCommentHandlesABlockLargerThanTheBudget is the pathological case: the
// closing block alone does not fit. It still publishes something bounded rather
// than refusing.
func TestRenderCommentHandlesABlockLargerThanTheBudget(t *testing.T) {
	t.Parallel()

	huge := strings.Repeat("x", MaxBodyBytes*2)
	v := ParseVerdict("prose\n```json\n{\"decision\": \"approve\", \"summary\": \"" + huge + "\"}\n```")
	if !v.HasVerdict() {
		t.Fatalf("fixture did not parse: %s", v.Reason)
	}
	v.TurnID, v.ItemID = "resp_claude_a", "item_reply"

	body := RenderComment(v, "conv_1")
	if len(body) > MaxBodyBytes {
		t.Errorf("body = %d bytes, want at most MaxBodyBytes (%d)", len(body), MaxBodyBytes)
	}
	if body == "" {
		t.Error("body is empty; an oversize verdict must still publish something")
	}
}

// TestTruncateBytesNeverSplitsARune pins the property that keeps a clipped body
// valid UTF-8.
func TestTruncateBytesNeverSplitsARune(t *testing.T) {
	t.Parallel()

	// Three bytes per rune, so most cut points land mid-rune.
	s := strings.Repeat("あ", 50)
	for max := 0; max <= len(s); max++ {
		got := truncateBytes(s, max)
		if len(got) > max {
			t.Fatalf("truncateBytes(_, %d) returned %d bytes", max, len(got))
		}
		if !strings.HasPrefix(s, got) {
			t.Fatalf("truncateBytes(_, %d) is not a prefix of the input", max)
		}
		for _, r := range got {
			if r == '�' {
				t.Fatalf("truncateBytes(_, %d) split a rune: %q", max, got)
			}
		}
	}
}

// TestPublishedCommentKeepsASuppressedNitInItsBlock records the residue the nit gate
// deliberately leaves, so the next reader does not take it for a bug.
//
// The gate governs placement and the check run's count. It does not reach the published
// summary comment, because [RenderComment] carries the reply's closing block whole and
// that block is model output. So the check can say one finding over a comment whose block
// lists two.
//
// Closing that would mean rewriting what the agent said before publishing it, which is a
// larger decision than a severity gate: the block is the record of the reply, and a
// publisher that edits it makes the transcript and the comment disagree instead.
func TestPublishedCommentKeepsASuppressedNitInItsBlock(t *testing.T) {
	v := verdictFrom(t, `{"read":40,"decision":"request_changes","summary":"s",
	  "inline_comments":[
	    {"path":"a.go","line":1,"side":"RIGHT","severity":"blocker","body":"real"},
	    {"path":"b.go","line":2,"side":"RIGHT","severity":"nit","body":"polish"}]}`)

	// The two surfaces the gate does reach.
	if got := PlaceableFindings(v, false); len(got) != 1 {
		t.Fatalf("PlaceableFindings = %+v; want the blocker alone", got)
	}
	check, ok := BuildCheckRun(v, false)
	if !ok {
		t.Fatal("no check run for a verdict that decided")
	}
	if !strings.HasPrefix(check.Title, "1 finding") {
		t.Fatalf("check Title = %q; want 1 finding", check.Title)
	}

	// And the one it does not.
	body := RenderComment(v, "sess_1")
	if !strings.Contains(body, "polish") {
		t.Errorf("the published comment dropped the suppressed nit from its block; the "+
			"gate is not supposed to edit model output, and the comment on "+
			"reportedBySeverity says so:\n%s", body)
	}
}
