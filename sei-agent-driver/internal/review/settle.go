package review

import (
	"fmt"
	"strings"
)

// SettleByScouts decides whether the scouts' readings end the review on their own,
// and if so renders the verdict a caller publishes in place of a review turn.
//
// The gate is deliberately narrow. Every scout dispatched must have read the diff,
// found nothing, and called it inert -- comments, documentation prose and whitespace
// and nothing a machine executes, parses or renders into configuration. One scout
// short of that on any count, or no scout at all, and the review turn runs as it
// always has. So a reading with no findings does not end a review by itself, and
// nothing a scout can write ends a review of a diff another scout read differently.
//
// The verdict it renders says the scouts decided it: in the summary the check and
// the comment both carry, in [Verdict.SettledBy] for the footer, and in the read
// count, which is the smallest any scout reported so the conclusion is derived from
// the same reading rule a review turn's is.
func SettleByScouts(req Request) (Verdict, bool) {
	if len(req.Scouts) == 0 {
		return Verdict{}, false
	}
	names := make([]string, 0, len(req.Scouts))
	lines := 0
	for _, s := range req.Scouts {
		if s.Failed() || len(s.Findings) > 0 || !s.Inert || s.Lines <= 0 {
			return Verdict{}, false
		}
		if lines == 0 || s.Lines < lines {
			lines = s.Lines
		}
		names = append(names, oneLine(s.Name))
	}

	who := strings.Join(names, ", ")
	summary := fmt.Sprintf("Settled by the scouts (%s) without a review turn: every one "+
		"of them read the diff, found nothing, and reported it inert -- comments, "+
		"documentation or whitespace only, with no executable surface changed.", who)
	return Verdict{
		Text: summary,
		Structured: map[string]any{
			"read":                lines,
			"decision":            "approve",
			"summary":             summary,
			"inline_comments":     []any{},
			"blockers":            []any{},
			"non_blockers":        []any{},
			"pre_existing_issues": []any{},
			"resolved_thread_ids": []any{},
		},
		SettledBy: who,
	}, true
}
