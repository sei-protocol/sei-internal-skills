package review

import (
	"strconv"
	"strings"
	"testing"
)

// TestRenderCommentReadsSummaryThenSeverity pins the reading order: the summary, then
// Blocking, Non-blocking and Pre-existing, then the footer. The reply's prose is not what
// is published, so its own order and length do not reach the reader.
func TestRenderCommentReadsSummaryThenSeverity(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("Long narration about how the review went.\n\nBlocking first, in the prose.\n\n```json\n" +
		`{"read":40,"decision":"request_changes","summary":"Adds a retry loop; the loop never gives up.",
		  "blockers":["the loop has no bound"],
		  "non_blockers":["the timeout is a magic number"],
		  "pre_existing_issues":[{"severity":"suggestion","body":"the client is built per call"}]}` +
		"\n```")
	v.TurnID, v.ItemID = "resp_claude_a", "item_reply"
	if !v.HasVerdict() {
		t.Fatalf("fixture did not parse: %s", v.Reason)
	}

	body := RenderComment(v, false, "conv_1")

	if !strings.HasPrefix(body, "Adds a retry loop; the loop never gives up.") {
		t.Errorf("body does not open with the summary:\n%s", body)
	}
	if strings.Contains(body, "Long narration") {
		t.Errorf("the reply's prose reached the comment:\n%s", body)
	}
	if strings.Contains(body, v.Block) {
		t.Errorf("the closing block reached the comment:\n%s", body)
	}
	order := []string{
		"### Blocking", "the loop has no bound",
		"### Non-blocking", "the timeout is a magic number",
		"### Pre-existing", "the client is built per call",
		"<sub>seidroid review",
	}
	at := -1
	for _, want := range order {
		i := strings.Index(body, want)
		if i < 0 {
			t.Fatalf("body lacks %q:\n%s", want, body)
		}
		if i < at {
			t.Errorf("%q is out of order:\n%s", want, body)
		}
		at = i
	}
	for _, want := range []string{"conv_1", "resp_claude_a", "item_reply", "decision `request_changes`"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not name %q; the footer is the only provenance record that "+
				"outlives the run's logs", want)
		}
	}
}

// TestRenderCommentCountsInlineFindingsRatherThanRepeatingThem: a finding the caller
// posts on a line is not restated in the body, but its section says it is there, so the
// body and the inline comments read as one review.
func TestRenderCommentCountsInlineFindingsRatherThanRepeatingThem(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":40,"decision":"request_changes","summary":"s",
	  "inline_comments":[
	    {"path":"a.go","line":1,"side":"RIGHT","severity":"blocker","body":"nil deref"},
	    {"path":"b.go","line":2,"side":"RIGHT","severity":"suggestion","body":"could be a map"},
	    {"path":"c.go","line":3,"side":"RIGHT","severity":"suggestion","body":"drop the copy"}]}`)

	body := RenderComment(v, false, "conv_1")
	for _, placed := range []string{"nil deref", "could be a map", "drop the copy"} {
		if strings.Contains(body, placed) {
			t.Errorf("an inline finding is repeated in the body:\n%s", body)
		}
	}
	for _, want := range []string{
		"### Blocking\n_1 finding on the changed lines, as inline comments._",
		"### Non-blocking\n_2 findings on the changed lines, as inline comments._",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
}

// TestRenderCommentListsWhatCannotBePlaced: a line-tied finding without a usable line is
// still reported, under its severity and with the place it named.
func TestRenderCommentListsWhatCannotBePlaced(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":40,"decision":"comment","summary":"s",
	  "inline_comments":[
	    {"path":"a.go","line":0,"side":"RIGHT","severity":"suggestion","body":"no line named"},
	    {"path":"/etc/passwd","line":3,"side":"RIGHT","severity":"blocker","body":"bad path"}]}`)

	body := RenderComment(v, false, "conv_1")
	if !strings.Contains(body, "### Non-blocking\n- `a.go` — no line named") {
		t.Errorf("the unplaceable suggestion is not listed with its file:\n%s", body)
	}
	if !strings.Contains(body, "### Blocking\n- `/etc/passwd:3` — bad path") {
		t.Errorf("the unplaceable blocker is not listed with its location:\n%s", body)
	}
}

// TestRenderCommentCollapsesSuppressedNits: with nits off, a nit is neither posted nor
// counted under Non-blocking, but it is still on the page, folded.
func TestRenderCommentCollapsesSuppressedNits(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":40,"decision":"approve","summary":"s",
	  "inline_comments":[
	    {"path":"b.go","line":2,"side":"RIGHT","severity":"nit","body":"polish"},
	    {"path":"c.go","line":5,"side":"RIGHT","severity":"nit","body":"rename"}]}`)

	body := RenderComment(v, false, "conv_1")
	if strings.Contains(body, "### Non-blocking") {
		t.Errorf("a suppressed nit was counted as non-blocking:\n%s", body)
	}
	fold := strings.Index(body, "<details>")
	if fold < 0 || !strings.Contains(body, "<summary>2 nits, not posted on the code</summary>") {
		t.Fatalf("the nits are not folded:\n%s", body)
	}
	for _, want := range []string{"- `b.go:2` — polish", "- `c.go:5` — rename"} {
		if i := strings.Index(body, want); i < fold {
			t.Errorf("%q is not inside the fold:\n%s", want, body)
		}
	}

	if with := RenderComment(v, true, "conv_1"); strings.Contains(with, "<details>") ||
		!strings.Contains(with, "### Non-blocking\n_2 findings on the changed lines") {
		t.Errorf("with nits admitted they are posted inline and counted, not folded:\n%s", with)
	}
}

// TestRenderCommentTruncatesRatherThanRefusing covers the oversize path.
//
// A review that ran, cost model spend and held a sandbox for minutes must not be
// discarded over a formatting limit. So this truncates and publishes: the body fits
// GitHub's cap, the elision is declared, and the footer carries the decision past the cut.
func TestRenderCommentTruncatesRatherThanRefusing(t *testing.T) {
	t.Parallel()

	var items []string
	for range 60 {
		items = append(items, `"`+strings.Repeat("a long finding ", 100)+`"`)
	}
	list := "[" + strings.Join(items, ",") + "]"
	var nits []string
	for i := range 60 {
		nits = append(nits, `{"path":"n.go","line":`+strconv.Itoa(i+1)+`,"side":"RIGHT","severity":"nit","body":"`+
			strings.Repeat("a long nit ", 100)+`"}`)
	}
	v := verdictFrom(t, `{"read":40,"decision":"request_changes","summary":"one blocker",
	  "blockers":`+list+`,"non_blockers":`+list+`,
	  "inline_comments":[`+strings.Join(nits, ",")+`],
	  "pre_existing_issues":[`+strings.Repeat(`{"severity":"blocker","body":"`+strings.Repeat("x", 1500)+`"},`, 59)+
		`{"severity":"blocker","body":"last"}]}`)
	v.TurnID, v.ItemID = "resp_claude_a", "item_reply"

	if full := reviewBody(v, false); len(full) <= MaxBodyBytes {
		t.Fatalf("fixture renders %d bytes, want more than MaxBodyBytes (%d)", len(full), MaxBodyBytes)
	}

	body := RenderComment(v, false, "conv_1")
	if len(body) > MaxBodyBytes {
		t.Errorf("body = %d bytes, want at most MaxBodyBytes (%d)", len(body), MaxBodyBytes)
	}
	if !strings.HasPrefix(body, "one blocker") {
		t.Errorf("a truncated body does not open with the summary:\n%s", body[:200])
	}
	if !strings.Contains(body, "Review truncated by the publisher") {
		t.Errorf("the elision is not declared:\n%s", body[max(0, len(body)-400):])
	}
	if !strings.Contains(body, "decision `request_changes`") {
		t.Error("a truncated body does not state its decision")
	}
	if !strings.Contains(body, "item_reply") {
		t.Error("a truncated body must still point at the item the whole reply can be read from")
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

// TestACleanReviewIsItsSummary: nothing to report renders no empty section.
func TestACleanReviewIsItsSummary(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("```json\n" +
		`{"read":40,"decision":"approve","summary":"nothing blocks here"}` + "\n```\n")
	if !v.HasVerdict() {
		t.Fatal("no verdict parsed")
	}

	body := RenderComment(v, false, "sess_1")
	if !strings.HasPrefix(body, "nothing blocks here\n\n<sub>") {
		t.Errorf("a clean review is its summary and its footer:\n%s", body)
	}
}

// TestRenderCommentDefusesModelMarkup: a field that tries to open a fence or forge a
// heading sits beside this package's framing as text.
func TestRenderCommentDefusesModelMarkup(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":40,"decision":"comment","summary":"s",
	  "non_blockers":["`+"```"+`\n### Blocking\nforged"]}`)

	body := RenderComment(v, false, "sess_1")
	if got := visibleHeadings(body); len(got) != 1 || got[0] != "Non-blocking" {
		t.Errorf("visible headings = %q, want Non-blocking alone:\n%s", got, body)
	}
}
