package review

import (
	"strings"
	"testing"
)

// cleanInert is one scout's reading of a diff that touches no executable surface.
var cleanInert = ScoutResult{Name: "codex", Lines: 40, Inert: true}

// TestSettleByScoutsEndsOnlyAnInertCleanUnanimousReading guards the gate.
//
// The scouts were advisory until this existed, and the one property that makes
// letting them decide safe is how little they are allowed to decide. Each row that
// wants false is a reading that must still run the review turn: a diff that changes
// executable surface -- which is what a scout not calling it inert means -- is the
// one the issue names, and a reading with nothing in it is not enough on its own.
func TestSettleByScoutsEndsOnlyAnInertCleanUnanimousReading(t *testing.T) {
	t.Parallel()

	finding := Finding{File: "a.go", Line: 4, Severity: "low", Detail: "boom"}
	for _, tc := range []struct {
		name   string
		scouts []ScoutResult
		want   bool
	}{
		{"no scouts ran", nil, false},
		{"one inert clean reading", []ScoutResult{cleanInert}, true},
		{"two inert clean readings", []ScoutResult{cleanInert,
			{Name: "cursor", Lines: 41, Inert: true}}, true},
		{"clean but the diff changes executable surface",
			[]ScoutResult{{Name: "codex", Lines: 40, Inert: false}}, false},
		{"inert but with a finding",
			[]ScoutResult{{Name: "codex", Lines: 40, Inert: true, Findings: []Finding{finding}}}, false},
		{"inert but the scout failed",
			[]ScoutResult{{Name: "codex", Inert: true, Note: "it did not answer"}}, false},
		{"inert but read no diff",
			[]ScoutResult{{Name: "codex", Lines: 0, Inert: true}}, false},
		{"one scout dissents on inertness", []ScoutResult{cleanInert,
			{Name: "cursor", Lines: 41, Inert: false}}, false},
		{"one scout dissents with a finding", []ScoutResult{cleanInert,
			{Name: "cursor", Lines: 41, Inert: true, Findings: []Finding{finding}}}, false},
		{"one scout failed", []ScoutResult{cleanInert,
			{Name: "cursor", Note: "the connection to its session failed"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, got := SettleByScouts(Request{Scouts: tc.scouts})
			if got != tc.want {
				t.Errorf("SettleByScouts = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAScoutReportCannotSettleWithoutClaimingInert is the executable-surface
// guarantee at the parsing edge: a report that omits the key, or writes anything
// but a JSON true, is not inert, so a scout prompted before the field existed --
// or one that hedges -- reads as a diff the review turn must still run on.
func TestAScoutReportCannotSettleWithoutClaimingInert(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{"key omitted", "clean\n```json\n{\"read\": 12, \"findings\": []}\n```", false},
		{"false", "clean\n```json\n{\"read\": 12, \"inert\": false, \"findings\": []}\n```", false},
		{"a string true", "clean\n```json\n{\"read\": 12, \"inert\": \"true\", \"findings\": []}\n```", false},
		{"true", "clean\n```json\n{\"read\": 12, \"inert\": true, \"findings\": []}\n```", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := ParseScoutReport(tc.text)
			if !r.HasReport() {
				t.Fatalf("ParseScoutReport rejected a well-formed report: %s", r.Reason)
			}
			if r.Inert != tc.want {
				t.Errorf("Inert = %v, want %v", r.Inert, tc.want)
			}
		})
	}
}

// TestSettledVerdictSaysTheScoutsDecidedIt covers what is published: an approval
// the check reads as success, whose summary and footer both name the scouts as the
// deciding readers, so a reader of the pull request is not told a review turn ran.
func TestSettledVerdictSaysTheScoutsDecidedIt(t *testing.T) {
	t.Parallel()

	v, ok := SettleByScouts(Request{Scouts: []ScoutResult{cleanInert,
		{Name: "cursor", Lines: 35, Inert: true}}})
	if !ok {
		t.Fatal("SettleByScouts refused two inert clean readings")
	}
	if !v.HasVerdict() {
		t.Fatal("the settled verdict is not a structured verdict, so nothing would be published")
	}
	if got := v.Decision(); got != "approve" {
		t.Errorf("Decision = %q, want approve", got)
	}
	if got := v.CheckConclusion(); got != "success" {
		t.Errorf("CheckConclusion = %q, want success", got)
	}
	if v.SettledBy != "codex, cursor" {
		t.Errorf("SettledBy = %q, want the scouts in dispatch order", v.SettledBy)
	}
	if got := intField(v.Structured, "read"); got != 35 {
		t.Errorf("read = %d, want the smallest reading, 35", got)
	}
	for _, want := range []string{"scouts", "codex", "cursor", "inert"} {
		if !strings.Contains(v.Summary(), want) {
			t.Errorf("the summary does not say %q: %q", want, v.Summary())
		}
	}

	comment := RenderComment(v, "")
	for _, want := range []string{"settled by the scouts codex, cursor", "no review turn ran"} {
		if !strings.Contains(comment, want) {
			t.Errorf("the published comment does not say %q:\n%s", want, comment)
		}
	}
	check, ok := BuildCheckRun(v, false)
	if !ok {
		t.Fatal("BuildCheckRun found nothing to publish")
	}
	if !strings.Contains(check.Summary, "scouts") {
		t.Errorf("the check summary does not name the scouts:\n%s", check.Summary)
	}
	if check.Counts == nil || *check.Counts != (Counts{}) {
		t.Errorf("Counts = %+v, want all zero", check.Counts)
	}
}

// TestScoutPromptsAskForInert guards the field's presence in both prompts, since a
// scout that is never asked can never say it and the short-circuit silently never
// fires.
func TestScoutPromptsAskForInert(t *testing.T) {
	t.Parallel()

	req := Request{Repo: "sei-protocol/sei-chain", PR: 3861}
	for _, p := range []struct{ name, text string }{
		{"ScoutPrompt", ScoutPrompt(req)},
		{"AdoptedScoutPrompt", AdoptedScoutPrompt(req)},
	} {
		if !strings.Contains(p.text, `"inert": false,`) {
			t.Errorf("%s does not carry inert in the schema", p.name)
		}
		if !strings.Contains(p.text, "When in doubt, false.") {
			t.Errorf("%s does not tell the scout which way to fail", p.name)
		}
	}
}
