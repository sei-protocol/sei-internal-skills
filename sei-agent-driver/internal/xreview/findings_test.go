package xreview

import (
	"strings"
	"testing"
)

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

// TestPlaceableFindingsReadsBothKeysWhenTheNewOneIsEmpty is the case a fallback
// misses. A session shown the current schema on re-review knows the new key
// exists and writes it empty, while still reporting under the vocabulary its
// first prompt taught. Reading the old key only when the new one is ABSENT
// places nothing here, and reports success doing it.
func TestPlaceableFindingsReadsBothKeysWhenTheNewOneIsEmpty(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"decision":"comment","summary":"s",
	  "inline_comments":[],
	  "findings":[{"file":"a.go","line":9,"severity":"high","detail":"boom"}]}`)

	got := PlaceableFindings(v)
	if len(got) != 1 {
		t.Fatalf("placed %d, want 1: an empty inline_comments must not hide a "+
			"filled findings", len(got))
	}
	if got[0].Severity != "blocker" || got[0].File != "a.go" {
		t.Errorf("finding = %+v", got[0])
	}
}

// TestPlaceableFindingsDedupesAcrossKeys covers the other half of reading both:
// a reply that wrote one finding under each key means it once.
func TestPlaceableFindingsDedupesAcrossKeys(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"decision":"comment","summary":"s",
	  "inline_comments":[{"path":"a.go","line":9,"side":"RIGHT","severity":"blocker","body":"boom"}],
	  "findings":[{"file":"a.go","line":9,"severity":"high","detail":"boom"}]}`)

	if got := PlaceableFindings(v); len(got) != 1 {
		t.Fatalf("placed %d, want 1: the same finding under both keys is one "+
			"finding, and posting it twice is noise on the author's diff", len(got))
	}
}

// TestBothPromptsCarryTheBucketRules pins what a session can actually read.
// Sessions outlive runs, so every dispatch after the first takes the adopted
// path; rules that live only in the other prompt stop applying there.
func TestBothPromptsCarryTheBucketRules(t *testing.T) {
	t.Parallel()

	req := Request{Repo: "sei-protocol/sandbox", PR: 42}
	for name, prompt := range map[string]string{
		"BuildPrompt":   BuildPrompt(req),
		"AdoptedPrompt": AdoptedPrompt(req),
	} {
		for _, want := range []string{
			"inline_comments", "blockers", "non_blockers", "pre_existing_issues",
			"side is RIGHT for an added or changed line",
			"blocker, suggestion or nit",
		} {
			if !strings.Contains(prompt, want) {
				t.Errorf("%s does not carry %q", name, want)
			}
		}
	}
}

// TestBothPromptsCarryTheRepoContext pins the standards and intent a review
// reads. Adding a step to one prompt and not the other is how a rule reaches the
// first dispatch on a pull request and no dispatch after it — and the adopted
// path is the one almost every review takes.
func TestBothPromptsCarryTheRepoContext(t *testing.T) {
	t.Parallel()

	req := Request{Repo: "sei-protocol/sandbox", PR: 42}
	for name, prompt := range map[string]string{
		"BuildPrompt":   BuildPrompt(req),
		"AdoptedPrompt": AdoptedPrompt(req),
	} {
		for _, want := range []string{
			// From the BASE branch. Read from the working tree, a change that edits
			// the standards would be handing itself the ones it is judged against —
			// and they outrank this prompt's checklist, which makes that a way to
			// approve anything.
			"--json baseRefName",
			"REVIEW_GUIDELINES.md?ref=$base",
			"never from pr-42-tree",
			"outrank the checklist",
			// Each command on its own indented line. Run together with the prose, an
			// agent copying it literally asked gh for a field called "body." and got
			// no intent at all.
			"\n    gh pr view 42 --repo sei-protocol/sandbox --json title,body\n",
			"never justify one",
		} {
			if !strings.Contains(prompt, want) {
				t.Errorf("%s does not carry %q", name, want)
			}
		}
	}
}
