package driver

// Workload is the half of a run this package does not decide: what the agent is
// asked, and how to tell it has finished answering. Everything else is the same
// whatever the ask, and belongs here.
type Workload interface {
	// RunKey identifies the unit of work across dispatches. A later run matches it
	// to adopt the session an earlier one opened, so two dispatches for the same
	// work must agree and different work must not collide.
	RunKey() string

	// Title names the session for a human reading a session list.
	Title() string

	// Prompt is the first message a fresh session receives.
	Prompt() string

	// AdoptedPrompt is what a session already holding this work's history receives
	// instead. It can refer to what the agent did before; that conversation is
	// still in front of it.
	AdoptedPrompt() string

	// Complete reports whether a reply is a finished answer rather than an agent
	// still working.
	//
	// The driver cannot answer this itself. A terminal-backed session goes idle
	// between tool calls and reports no active response while it does, so both of
	// the server's signals read "finished" mid-answer. Only the reply itself is
	// reliable, and only the workload knows what its prompt asked for. Reading the
	// server instead published an agent's opening sentence as a review.
	Complete(text string) bool
}
