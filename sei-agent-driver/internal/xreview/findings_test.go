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
