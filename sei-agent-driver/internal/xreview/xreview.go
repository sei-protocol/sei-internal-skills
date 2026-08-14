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

	// Scouts is what the independent readings returned, in dispatch order. Empty
	// runs the review alone, which is the only mode before any scout is
	// configured.
	Scouts []ScoutResult

	// PriorThreads is what this tool already said on this pull request, with the
	// replies it drew. Empty on a first review.
	//
	// Supplied rather than remembered. A session recalls its own findings, and
	// nothing in it records that the author pushed back, fixed the code, or
	// resolved the thread — which is the half that decides whether a finding is
	// still worth making.
	PriorThreads []PriorThread
}

// maxScoutDetail bounds one rendered finding. A scout's detail is unbounded model
// output, and several verbose scouts would otherwise crowd the diff out of the
// review's context — the one thing it must not lose.
const (
	maxScoutDetail = 500

	// maxScoutField bounds the short fields. They are model output too, and a
	// bound on the detail alone leaves the same door open one column to the left.
	maxScoutField = 120

	// maxScoutFindings bounds how many of one scout's findings are rendered. The
	// count is reported in full, so a truncated list reads as truncated rather
	// than as all a scout had.
	maxScoutFindings = 25
)

// ScoutResult is one scout's contribution, as the orchestrator observed it.
//
// The orchestrator builds this, not the scout: Name is the identity the scout was
// dispatched under, so attribution cannot be forged by anything the scout — or
// anything the scout read — put in its reply. A reading that arrives claiming to
// be from somewhere else is still recorded here as what it actually is.
type ScoutResult struct {
	// Name identifies the scout, e.g. "codex".
	Name string

	// Findings is what it reported. Empty with Note unset means it read the diff
	// and found nothing, which is an answer.
	Findings []Finding

	// Note records why this scout contributed nothing, and is empty when it
	// contributed normally.
	//
	// It exists so the review can tell a scout that found nothing from one that
	// never ran. Collapsing those is how a fleet-wide credential failure reads as
	// a clean bill of health on every pull request at once.
	Note string
}

// Failed reports whether this scout contributed nothing because something went
// wrong, as opposed to finding nothing.
func (s ScoutResult) Failed() bool { return s.Note != "" }

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

// Prompt is the review instruction. A session that has already reviewed this
// pull request is asked what changed rather than handed the contract a second
// time; the two texts are [BuildPrompt] and [AdoptedPrompt].
func (r Review) Prompt(answered bool) string {
	if answered {
		return AdoptedPrompt(r.req)
	}
	return BuildPrompt(r.req)
}

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
