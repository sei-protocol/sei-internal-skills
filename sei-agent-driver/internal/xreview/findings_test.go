package xreview

import "testing"

// TestPlaceableFindingsDropsWhatCannotBePlaced pins the rule that decides which
// observations become inline comments.
//
// A review comment needs a path and a line. An entry missing either has nowhere
// to go, and posting it somewhere arbitrary attributes a finding to code it is
// not about. Everything dropped here still travels in the summary comment's
// prose, so the review is not lost — only its placement.
func TestPlaceableFindingsDropsWhatCannotBePlaced(t *testing.T) {
	t.Parallel()

	v := ParseVerdict("prose\n\n```json\n" + `{"decision":"comment","summary":"s","findings":[
		{"file":"a.go","line":12,"severity":"high","detail":"placeable"},
		{"file":"","line":9,"severity":"low","detail":"no file"},
		{"file":"b.go","line":0,"severity":"low","detail":"no line"},
		{"file":"c.go","line":"34","severity":"low","detail":"line as a string"},
		{"file":"d.go","line":7,"severity":"low","detail":""},
		"not an object"
	]}` + "\n```")

	got := PlaceableFindings(v)
	if len(got) != 2 {
		t.Fatalf("placeable findings = %d, want 2: %+v", len(got), got)
	}
	if got[0].File != "a.go" || got[0].Line != 12 {
		t.Errorf("first = %+v, want a.go:12", got[0])
	}
	// A quoted number is still a number; an agent hand-writing JSON quotes them
	// often enough that refusing would drop real findings.
	if got[1].File != "c.go" || got[1].Line != 34 {
		t.Errorf("second = %+v, want c.go:34 from a quoted line", got[1])
	}
}

// TestPlaceableFindingsOnNoVerdict keeps a reply with no structured block from
// producing inline comments.
func TestPlaceableFindingsOnNoVerdict(t *testing.T) {
	t.Parallel()

	if got := PlaceableFindings(ParseVerdict("just prose, no block")); len(got) != 0 {
		t.Errorf("placeable findings = %d, want 0 when there is no verdict", len(got))
	}
}

// TestPlaceableFindingsReadsTheCurrentContract covers the buckets a review sorts
// its observations into, including side, which is what makes a finding about a
// removed line placeable at all.
func TestPlaceableFindingsReadsTheCurrentContract(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"decision":"request_changes","summary":"s",
	  "inline_comments":[
	    {"path":"a.go","line":10,"side":"RIGHT","severity":"blocker","body":"boom"},
	    {"path":"b.go","line":4,"side":"LEFT","severity":"nit","body":"gone"},
	    {"path":"c.go","line":0,"severity":"nit","body":"unplaceable"}],
	  "blockers":["needs a test"],
	  "non_blockers":["naming could be clearer"],
	  "pre_existing_issues":[{"severity":"nit","body":"old thing"}]}`)

	got := PlaceableFindings(v)
	if len(got) != 2 {
		t.Fatalf("placed %d findings, want 2 (the line-less one is dropped): %+v", len(got), got)
	}
	if got[0].Side != "RIGHT" || got[1].Side != "LEFT" {
		t.Errorf("sides = %q, %q; want RIGHT, LEFT", got[0].Side, got[1].Side)
	}
	if got[0].Severity != "blocker" || got[0].Detail != "boom" || got[0].File != "a.go" {
		t.Errorf("first finding = %+v", got[0])
	}
	if b := Blockers(v); len(b) != 1 || b[0] != "needs a test" {
		t.Errorf("Blockers = %v", b)
	}
	if n := NonBlockers(v); len(n) != 1 {
		t.Errorf("NonBlockers = %v", n)
	}
	pre := PreExisting(v)
	if len(pre) != 1 || pre[0].Body != "old thing" {
		t.Fatalf("PreExisting = %+v", pre)
	}
	if pre[0].Severity != "suggestion" {
		t.Errorf("pre-existing severity = %q, want suggestion: the bucket takes "+
			"blocker or suggestion, and a nit about untouched code is noise",
			pre[0].Severity)
	}
}

// TestPlaceableFindingsStillReadsTheOlderContract is the one that stops a schema
// change from going silent. A session that reviewed before this contract carries
// its first prompt in context and cannot be told otherwise, so it keeps answering
// in the vocabulary it learned. Reading only the new keys would place nothing on
// every pull request already reviewed, and report success while doing it.
func TestPlaceableFindingsStillReadsTheOlderContract(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"decision":"comment","summary":"s",
	  "findings":[{"file":"a.go","line":10,"severity":"high","detail":"boom"},
	              {"file":"b.go","line":2,"severity":"low","detail":"minor"}]}`)

	got := PlaceableFindings(v)
	if len(got) != 2 {
		t.Fatalf("placed %d, want 2: %+v", len(got), got)
	}
	if got[0].Severity != "blocker" || got[1].Severity != "nit" {
		t.Errorf("severities = %q, %q; want blocker, nit (high/low map on)",
			got[0].Severity, got[1].Severity)
	}
	if got[0].Side != "RIGHT" {
		t.Errorf("side = %q, want RIGHT: a finding written before side existed "+
			"meant the new file", got[0].Side)
	}
}

// TestCheckConclusion pins the mapping the check run is published under.
func TestCheckConclusion(t *testing.T) {
	t.Parallel()

	for decision, want := range map[string]string{
		"request_changes": "failure",
		"comment":         "neutral",
		"approve":         "success",
	} {
		v := verdictFrom(t, `{"decision":"`+decision+`","summary":"s"}`)
		if got := v.CheckConclusion(); got != want {
			t.Errorf("decision %q -> %q, want %q", decision, got, want)
		}
	}
	if got := (Verdict{}).CheckConclusion(); got != "" {
		t.Errorf("no verdict -> %q, want empty", got)
	}
}

// verdictFrom parses a closing block the way a real reply carries one.
func verdictFrom(t *testing.T, block string) Verdict {
	t.Helper()
	v := ParseVerdict("Review prose.\n\n```json\n" + block + "\n```\n")
	if !v.HasVerdict() {
		t.Fatalf("no verdict parsed from %s", block)
	}
	return v
}
