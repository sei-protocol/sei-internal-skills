package driver

import (
	"fmt"
	"strings"
)

// testWork is the workload the driver's tests drive.
//
// It carries the same fields the review workload does, because what the driver
// does with a workload is the same either way and a test reads better naming
// real work than an abstraction. Its completeness test is the interesting part:
// a test that needs an unfinished reply supplies its own.
type testWork struct {
	Repo     string
	PR       int
	Trigger  string
	complete func(string) bool
}

func (w testWork) RunKey() string { return testRunKey(w.Repo, w.PR) }

func (w testWork) Title() string { return fmt.Sprintf("test %s#%d", w.Repo, w.PR) }

func (w testWork) Prompt() string { return fmt.Sprintf("review %s#%d", w.Repo, w.PR) }

func (w testWork) AdoptedPrompt() string {
	return fmt.Sprintf("review %s#%d again", w.Repo, w.PR)
}

// Complete defaults to the review workload's rule — a closing fenced block —
// because that is what the fixtures produce.
func (w testWork) Complete(text string) bool {
	if w.complete != nil {
		return w.complete(text)
	}
	return strings.Contains(text, "```json")
}

// testRunKey is a deterministic key for a unit of work. Only stability matters
// here; the real digest and its properties are the workload package's to test.
func testRunKey(repo string, pr int) string { return fmt.Sprintf("%s#%d", repo, pr) }

// carriesDecision reports whether a reply closes with the given decision.
//
// The driver does not parse a verdict — that is the workload's, and is tested
// there. This is only enough to tell one fixture's reply from another's, which
// is what the attribution tests are actually asserting.
func carriesDecision(r *Reply, decision string) bool {
	return r != nil && strings.Contains(r.Text, `"decision": "`+decision+`"`)
}
