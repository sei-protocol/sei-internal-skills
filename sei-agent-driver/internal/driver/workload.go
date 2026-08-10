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

	// Prompt is the message to send.
	//
	// answered reports that this conversation already holds a reply [Workload.Complete]
	// accepted. It is the reason a session is worth reusing: that reply is still in
	// front of the agent, so a later dispatch has the option of asking what changed
	// instead of putting the whole question again. Taking that option is the
	// workload's call, and one with nothing different to say may ignore the flag and
	// return the same message either way.
	Prompt(answered bool) string

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
