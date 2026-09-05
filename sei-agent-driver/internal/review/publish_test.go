package review

import (
	"fmt"
	"strings"
	"testing"
)

// TestRenderCommentPassesAShortReviewThrough checks the ordinary case: the agent's own
// prose, its closing block dropped, plus the provenance footer.
func TestRenderCommentPassesAShortReviewThrough(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("Two findings, both minor.\n\n```json\n" +
		`{"decision": "comment", "summary": "two minor findings"}` + "\n```")
	v.TurnID = "resp_claude_a"
	v.ItemID = "item_reply"
	if !v.HasVerdict() {
		t.Fatalf("fixture did not parse: %s", v.Reason)
	}

	body := RenderComment(v, "conv_1")

	if !strings.HasPrefix(body, "Two findings, both minor.") {
		t.Errorf("body does not open with the agent's own words:\n%s", body)
	}
	if strings.Contains(body, "truncated") {
		t.Error("a short review must not be marked truncated")
	}
	if strings.Contains(body, v.Block) {
		t.Errorf("the closing block reached the comment:\n%s", body)
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
// notice sizes the whole reply so the reader knows what the fetch costs, the footer
// carries the decision past the cut, and the fences balance so the notice cannot be
// swallowed into a code block.
func TestRenderCommentTruncatesRatherThanRefusing(t *testing.T) {
	t.Parallel()

	// Opens a fence and never closes it, so the cut necessarily lands inside a
	// code block.
	prose := "```text\n" + strings.Repeat("a very long finding line\n", 4000)
	v := ParseVerdict(prose + "\n```json\n" +
		`{"decision": "request_changes", "summary": "one blocker"}` + "\n```")
	v.TurnID = "resp_claude_a"
	v.ItemID = "item_reply"

	if !v.HasVerdict() {
		t.Fatalf("fixture did not parse: %s", v.Reason)
	}
	// Measured on what gets published, which is the prose. A fixture oversize only by
	// its block is cut nowhere.
	if published := len(v.proseWithoutBlock()); published <= MaxBodyBytes {
		t.Fatalf("fixture publishes %d bytes, want more than MaxBodyBytes (%d)",
			published, MaxBodyBytes)
	}

	body := RenderComment(v, "conv_1")

	if len(body) > MaxBodyBytes {
		t.Errorf("body = %d bytes, want at most MaxBodyBytes (%d)", len(body), MaxBodyBytes)
	}
	if !strings.Contains(body, "Review truncated by the publisher") {
		t.Errorf("the elision is not declared:\n%s", body[max(0, len(body)-400):])
	}
	if strings.Contains(body, v.Block) {
		t.Error("the closing block reached a truncated comment; the callers that read a " +
			"decision take it from check.json")
	}
	if !strings.Contains(body, "decision `request_changes`") {
		t.Error("a truncated body does not state its decision; the footer is what carries " +
			"it past the cut")
	}
	if !strings.Contains(body, fmt.Sprintf("%d bytes", len(v.Text))) {
		t.Errorf("the notice does not size the whole reply (%d bytes), so a reader cannot "+
			"tell what is waiting at the item", len(v.Text))
	}
	if n := strings.Count(body, "```"); n%2 != 0 {
		t.Errorf("fence count = %d, want even: an unbalanced fence renders the notice and the "+
			"verdict as code, which changes what the comment appears to say", n)
	}
	if !strings.Contains(body, "item_reply") {
		t.Error("a truncated body must still point at the item the whole reply can be read from")
	}
}

// TestABlockLargerThanTheBudgetCostsTheCommentNothing pins what the budget is spent on.
//
// The comment publishes the reply's prose, so the budget is the prose's. Measuring the
// whole reply instead would cut a five-byte review over a block nobody reads there, and
// a truncation notice on an untruncated review sends the reader after text that is
// already in front of them.
func TestABlockLargerThanTheBudgetCostsTheCommentNothing(t *testing.T) {
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
	if !strings.HasPrefix(body, "prose") {
		t.Errorf("body does not open with the agent's own words:\n%s", body[:min(len(body), 400)])
	}
	if strings.Contains(body, "truncated") {
		t.Error("a five-byte review was marked truncated; the block it drops is not part " +
			"of the budget")
	}
	if strings.Contains(body, huge) {
		t.Error("the oversize block reached the comment")
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

// TestABlockOnlyReplyStillSaysSomething covers a reply that is all block and no prose.
//
// Stripping it leaves nothing, and a comment that is a provenance footer alone tells the
// reader less than the block did. The summary the block carries is the fallback.
func TestABlockOnlyReplyStillSaysSomething(t *testing.T) {
	// Built without verdictFrom, which prepends prose and so cannot express this shape.
	v := ParseVerdict("```json\n" +
		`{"read":40,"decision":"approve","summary":"nothing blocks here"}` + "\n```\n")
	if !v.HasVerdict() {
		t.Fatal("no verdict parsed")
	}

	body := RenderComment(v, "sess_1")
	if !strings.Contains(body, "nothing blocks here") {
		t.Errorf("a block-only reply published no review at all:\n%s", body)
	}
	if strings.Contains(body, "```json") {
		t.Errorf("the fallback published the block:\n%s", body)
	}
}

// TestAQuotedCopyOfTheBlockDoesNotStandInForIt covers a reply whose prose already carries
// the bytes its closing block carries.
//
// The diff under review can hold a fenced json block, and a reply that quotes one and
// then closes with its own carries those bytes twice. Only the last copy is the verdict.
// Cutting an earlier one publishes the closing block, which is the whole point of cutting.
func TestAQuotedCopyOfTheBlockDoesNotStandInForIt(t *testing.T) {
	t.Parallel()

	block := "```json\n" +
		`{"read":40,"decision":"approve","summary":"nothing blocks here"}` + "\n```"
	quoted := "```json\n" + `{"quoted":"from the diff"}` + "\n"
	v := ParseVerdict(quoted + block + "\n\n" + block + "\n")
	if !v.HasVerdict() {
		t.Fatalf("fixture did not parse: %s", v.Reason)
	}
	if v.Block != block {
		t.Fatalf("Block = %q, want the closing block", v.Block)
	}
	if n := strings.Count(v.Text, block); n != 2 {
		t.Fatalf("the fixture carries the block %d times, want 2: the earlier copy is what "+
			"this test is about", n)
	}

	body := RenderComment(v, "conv_1")
	if !strings.HasPrefix(body, quoted+block) {
		t.Errorf("the comment cut the quoted copy and kept the closing block:\n%s", body)
	}
	if n := strings.Count(body, block); n != 1 {
		t.Errorf("the block appears %d times in the comment, want the quoted copy alone:\n%s",
			n, body)
	}
}

// TestPublishedCommentDropsASuppressedNit pins the nit gate across the surfaces it
// reaches: the placements, the check run's count, and the block.
//
// [Verdict.proseWithoutBlock] leaves the reply's closing block unpublished, so the comment
// restates no block entry the gate drops. The reply's own prose is published whole and no
// gate reads it: a reply that writes the nit into its Non-blocking section publishes it
// beside a check count that omits it.
func TestPublishedCommentDropsASuppressedNit(t *testing.T) {
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

	// And the comment. "polish" sits only inside the block here, so the assertion
	// catches a block entry re-rendered as prose as well as the block itself. It says
	// nothing about a reply that wrote the nit into its own prose.
	body := RenderComment(v, "sess_1")
	if strings.Contains(body, "```json") {
		t.Errorf("the published comment still carries the decision block:\n%s", body)
	}
	if strings.Contains(body, "polish") {
		t.Errorf("a suppressed nit reached the comment anyway:\n%s", body)
	}
}
