package review

import (
	"strings"
	"testing"
)

// TestDecisionIsDerivedWithTheConclusion is the contract behind the recorded decision:
// what the caller submits and what the check concludes come from one reading, and the
// word the reply wrote does not get to record an approval its own findings withhold.
func TestDecisionIsDerivedWithTheConclusion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		block      string
		said       string
		decision   string
		conclusion string
		notice     []string // in both the check summary and the comment footer
	}{
		{"clean approve",
			`{"read":40,"decision":"approve","summary":"s"}`,
			"approve", "approve", "success", nil},
		{"approve over suggestions",
			`{"read":40,"decision":"approve","summary":"s",
			  "non_blockers":["consider a table"]}`,
			"approve", "approve", "success", nil},
		{"request_changes with a blocker",
			`{"read":40,"decision":"request_changes","summary":"s",
			  "blockers":["nil deref in a.go"]}`,
			"request_changes", "request_changes", "failure", nil},
		{"approve beside a blocker",
			`{"read":40,"decision":"approve","summary":"s",
			  "blockers":["nil deref in a.go"]}`,
			"approve", "request_changes", "failure",
			[]string{"**Changes requested.**", "wrote `approve`", "nil deref in a.go"}},
		{"approve beside a line-tied blocker",
			`{"read":40,"decision":"approve","summary":"s",
			  "inline_comments":[{"path":"a.go","line":2,"side":"RIGHT",
			    "severity":"blocker","body":"unchecked error"}]}`,
			"approve", "request_changes", "failure",
			[]string{"**Changes requested.**", "unchecked error"}},
		{"approve beside an unaccepted pre-existing blocker",
			`{"read":40,"decision":"approve","summary":"Approving.",
			  "pre_existing_issues":[{"severity":"blocker","body":"b.go:4 leaks a handle"}]}`,
			"approve", "comment", "neutral",
			[]string{"**Approval withheld.**", "pre-existing blocker", "b.go:4 leaks a handle"}},
		{"approve that did not read the diff",
			`{"decision":"approve","summary":"s"}`,
			"approve", "comment", "neutral",
			[]string{"**Approval withheld.**", "did not show it read the diff"}},
		{"comment and nothing else",
			`{"read":40,"decision":"comment","summary":"s"}`,
			"comment", "comment", "neutral",
			[]string{"**No position recorded.**"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := verdictFrom(t, tc.block)
			if got := v.Said(); got != tc.said {
				t.Errorf("Said() = %q, want %q", got, tc.said)
			}
			if got := v.Decision(); got != tc.decision {
				t.Errorf("Decision() = %q, want %q", got, tc.decision)
			}
			if got := v.CheckConclusion(); got != tc.conclusion {
				t.Errorf("CheckConclusion() = %q, want %q", got, tc.conclusion)
			}
			check, ok := BuildCheckRun(v, false)
			if !ok {
				t.Fatal("BuildCheckRun() = !ok on a verdict")
			}
			if check.Decision != tc.decision {
				t.Errorf("check.Decision = %q, want %q", check.Decision, tc.decision)
			}
			comment := RenderComment(v, "conv_1")
			if !strings.Contains(comment, "decision `"+tc.decision+"`") {
				t.Errorf("the footer does not carry the recorded decision %q:\n%s",
					tc.decision, comment)
			}
			for _, want := range tc.notice {
				if !strings.Contains(check.Summary, want) {
					t.Errorf("check summary lacks %q:\n%s", want, check.Summary)
				}
				if !strings.Contains(comment, want) {
					t.Errorf("comment lacks %q:\n%s", want, comment)
				}
			}
			if tc.notice == nil {
				for _, stray := range []string{"withheld", "No position", "Changes requested."} {
					if strings.Contains(check.Summary, stray) {
						t.Errorf("check summary carries an unwarranted notice %q:\n%s",
							stray, check.Summary)
					}
				}
			}
		})
	}
}

// TestTheNoticeIsAheadOfTheCut: the withheld notice rides in the footer, which is what
// the publisher keeps when the prose is cut, so a long reply cannot push it off the page.
func TestTheNoticeIsAheadOfTheCut(t *testing.T) {
	t.Parallel()

	pad := strings.Repeat("Approving, this all looks fine.\n", 4000)
	v := ParseVerdict(pad + "\n```json\n" + `{"read":40,"decision":"approve","summary":"s",
	  "pre_existing_issues":[{"severity":"blocker","body":"b.go:4 leaks a handle"}]}` + "\n```")
	body := RenderComment(v, "conv_1")
	cut := strings.Index(body, proseSeparator)
	if cut < 0 {
		t.Fatalf("no separator, so the prose was not cut:\n%s", body[:300])
	}
	lead := body[:cut]
	for _, want := range []string{"**Approval withheld.**", "b.go:4 leaks a handle", "decision `comment`"} {
		if !strings.Contains(lead, want) {
			t.Errorf("%q is not ahead of the cut text", want)
		}
	}
}

// TestScoutSettlementRecordsApproveWithoutANotice: a verdict the scouts settled is an
// approve on its own terms and the position has nothing to add.
func TestScoutSettlementRecordsApproveWithoutANotice(t *testing.T) {
	t.Parallel()

	v, ok := SettleByScouts(Request{Scouts: []ScoutResult{
		{Name: "codex", Lines: 3, Inert: true},
	}})
	if !ok {
		t.Fatal("SettleByScouts() = !ok on an inert, clean reading")
	}
	if got := v.Decision(); got != "approve" {
		t.Errorf("Decision() = %q, want approve", got)
	}
	if got := v.CheckConclusion(); got != "success" {
		t.Errorf("CheckConclusion() = %q, want success", got)
	}
	if p := v.position(); p != "" {
		t.Errorf("position() = %q, want empty on a settled verdict", p)
	}
}

// TestAcceptedPreExistingBlockerDoesNotWithhold is the acknowledgement path, enforced
// in Go: the same finding vetoes without the base branch's acceptance and clears with
// it, and in both cases it stays on the page.
func TestAcceptedPreExistingBlockerDoesNotWithhold(t *testing.T) {
	t.Parallel()

	const block = `{"read":40,"decision":"approve","summary":"s",
	  "pre_existing_issues":[
	    {"severity":"blocker","body":"go.mod pins uci to the feature branch, not a tag"},
	    {"severity":"suggestion","body":"README drifts from the flags"}]}`
	accepted := ParseAccepted("# Review\n\n## Accepted pre-existing conditions\n\n" +
		"- pins `uci` to the feature branch — deliberate until PLT-1300 lands\n")
	if len(accepted) != 1 {
		t.Fatalf("ParseAccepted() = %q, want one entry", accepted)
	}

	unaccepted := verdictFrom(t, block)
	if got := unaccepted.Decision(); got != "comment" {
		t.Errorf("without an acceptance Decision() = %q, want comment", got)
	}

	v := verdictFrom(t, block)
	v.Accepted = accepted
	if got := v.Decision(); got != "approve" {
		t.Errorf("with the acceptance Decision() = %q, want approve", got)
	}
	if got := v.CheckConclusion(); got != "success" {
		t.Errorf("with the acceptance CheckConclusion() = %q, want success", got)
	}
	issues := PreExisting(v)
	if len(issues) != 2 {
		t.Fatalf("PreExisting() = %d issues, want both kept", len(issues))
	}
	if issues[0].Accepted != accepted[0] {
		t.Errorf("issues[0].Accepted = %q, want the acceptance verbatim", issues[0].Accepted)
	}
	if issues[1].Accepted != "" {
		t.Errorf("issues[1].Accepted = %q, want none: the suggestion matched nothing",
			issues[1].Accepted)
	}
	check, _ := BuildCheckRun(v, false)
	for _, want := range []string{
		"**blocker, accepted**", "pins uci to the feature branch", "PLT-1300",
		"**Accepted pre-existing blocker**",
	} {
		if !strings.Contains(check.Summary, want) {
			t.Errorf("check summary lacks %q:\n%s", want, check.Summary)
		}
	}
	if strings.Contains(check.Summary, "Approval withheld") {
		t.Errorf("check summary still withholds:\n%s", check.Summary)
	}
	if comment := RenderComment(v, "conv_1"); !strings.Contains(comment, "**Accepted pre-existing blocker**") {
		t.Errorf("comment does not name the acceptance:\n%s", comment)
	}
}

// TestAcceptanceIsNarrow: an entry accepts one condition, not a category, and a
// second blocker beside an accepted one still withholds.
func TestAcceptanceIsNarrow(t *testing.T) {
	t.Parallel()

	accepted := ParseAccepted("## Accepted\n- pins uci to the feature branch\n")
	v := verdictFrom(t, `{"read":40,"decision":"approve","summary":"s",
	  "pre_existing_issues":[
	    {"severity":"blocker","body":"go.mod pins uci to the feature branch"},
	    {"severity":"blocker","body":"c.go:9 writes the token to the log"}]}`)
	v.Accepted = accepted
	if got := v.Decision(); got != "comment" {
		t.Errorf("Decision() = %q, want comment: the second blocker is not accepted", got)
	}
	check, _ := BuildCheckRun(v, false)
	if !strings.Contains(check.Summary, "writes the token to the log") {
		t.Errorf("the notice does not name the blocker that withheld:\n%s", check.Summary)
	}
}

func TestParseAccepted(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		text string
		want []string
	}{
		{"no section", "# Review\n\n- pins uci to the feature branch\n", nil},
		{"the section, then another heading ends it",
			"## Accepted pre-existing conditions\n- pins uci to the feature branch\n" +
				"## Style\n- prefer short names here\n",
			[]string{"pins uci to the feature branch"}},
		{"star bullets and a rationale",
			"### Accepted\n* pins uci to the feature branch -- until PLT-1300\n",
			[]string{"pins uci to the feature branch -- until PLT-1300"}},
		{"too short to name one condition",
			"## Accepted\n- pin\n- the pin\n- blocker — anything\n",
			nil},
		{"a heading that merely contains the word",
			"## Not accepted\n- pins uci to the feature branch\n", nil},
		{"crlf", "## Accepted\r\n- pins uci to the feature branch\r\n",
			[]string{"pins uci to the feature branch"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ParseAccepted(tc.text)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("ParseAccepted() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAcceptanceMatchingFoldsForm(t *testing.T) {
	t.Parallel()

	accepted := []string{"pins `uci` to the feature branch — deliberate"}
	for body, want := range map[string]bool{
		"go.mod PINS uci   to the feature branch instead of a tag": true,
		"pins 'uci' to the feature branch":                         true,
		"pins uci to a tag":                                        false,
		"":                                                         false,
	} {
		if got := acceptanceFor(body, accepted) != ""; got != want {
			t.Errorf("acceptanceFor(%q) matched = %v, want %v", body, got, want)
		}
	}
}
