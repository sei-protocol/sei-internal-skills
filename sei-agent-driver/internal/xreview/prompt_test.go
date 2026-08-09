package xreview

import (
	"strings"
	"testing"
)

// TestPromptsNameTheDiffCommand guards the property that makes a review a review.
//
// A prompt that grants the capability to "inspect the diff" is satisfied by
// `gh pr view`, which returns a title and a description — so the agent can write
// a fluent review of the pull request's summary without reading a line of code,
// and nothing in the reply distinguishes that from a real review. The exact
// command has to be in the text, with this pull request's number and repository
// interpolated into it, or the reader is back to choosing its own.
func TestPromptsNameTheDiffCommand(t *testing.T) {
	req := Request{Repo: "sei-protocol/sei-chain", PR: 3861}
	// Written out rather than built from the helper the prompts use, which would
	// assert only that they agree with each other.
	// Relative on purpose: only a read inside the working directory goes
	// unprompted.
	wantCommand := "gh pr diff 3861 --repo sei-protocol/sei-chain > " +
		"pr-3861.diff && wc -l pr-3861.diff"

	for _, p := range []struct {
		name string
		text string
	}{
		{"BuildPrompt", BuildPrompt(req)},
		{"AdoptedPrompt", AdoptedPrompt(req)},
	} {
		if !strings.Contains(p.text, wantCommand) {
			t.Errorf("%s does not name %q; an abstract instruction to inspect the diff "+
				"is satisfiable by `gh pr view`", p.name, wantCommand)
		}
	}

	// The report headings are the other half: a findings array may be empty and a
	// summary can be written from the title, so the sections are what cannot be
	// filled honestly without having read the changed lines.
	// Matched in their numbered form. The bare words also appear in the checklist
	// and the verification gate, so a check for those would pass with the report
	// contract deleted.
	for _, heading := range []string{
		"1. Blocking", "2. Security", "3. Non-blocking", "4. Summary",
	} {
		if !strings.Contains(BuildPrompt(req), heading) {
			t.Errorf("BuildPrompt does not require a %q section", heading)
		}
	}
}
