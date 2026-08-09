// Package xreview is the pull-request review workload: what the agent is asked,
// how to tell it has answered, and what to publish.
//
// Everything about running the session — resolving the agent, adopting one an
// earlier dispatch created, following a turn across the streams it outlives,
// answering permission prompts — belongs to the driver package and is the same
// whatever the agent is asked for.
package xreview

import (
	"fmt"
	"unicode/utf8"
)

// Request is one review to perform.
type Request struct {
	// Repo is "owner/name".
	Repo string

	// PR is the pull request number.
	PR int

	// Trigger distinguishes this dispatch from another for the same pull
	// request. See [TriggerID].
	Trigger string
}

// Review is the review workload for one pull request.
type Review struct {
	req Request
}

// New returns the workload for a review of req.
func New(req Request) Review { return Review{req: req} }

// RunKey identifies the pull request, so a later dispatch adopts the session an
// earlier one opened rather than reviewing the same tree twice.
func (r Review) RunKey() string { return RunKey(r.req.Repo, r.req.PR) }

// Title names the session for a human reading a session list.
func (r Review) Title() string { return fmt.Sprintf("xreview %s#%d", r.req.Repo, r.req.PR) }

// Prompt is the review instruction a fresh session receives.
func (r Review) Prompt() string { return BuildPrompt(r.req) }

// AdoptedPrompt is what a session that has already reviewed this pull request
// receives instead.
func (r Review) AdoptedPrompt() string { return AdoptedPrompt(r.req) }

// Complete reports whether a reply is a finished review.
//
// The closing verdict block is the whole test, and it is load-bearing rather
// than cosmetic: a session reports itself idle between tool calls, so the
// driver's own signals cannot tell an agent mid-answer from one that finished.
// A reply without the block is an agent still working, and treating it as an
// answer publishes an opening sentence as a review.
func (r Review) Complete(text string) bool { return ParseVerdict(text).HasVerdict() }

// clip bounds a value taken from model output before it reaches a log line,
// cutting on a rune boundary so the line stays valid UTF-8.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max] + "…"
}
