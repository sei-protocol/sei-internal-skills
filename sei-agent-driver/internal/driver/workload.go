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

// AgentNamer is an optional [Workload] capability: work that names the agent it
// must run on rather than taking the run's configured default.
//
// Optional because a run usually has one agent and every workload wants it. It
// exists for work composed of several asks that must not share a conversation —
// a review that gathers independent opinions before merging them needs each
// opinion from a different agent, since the agent is what fixes the harness, and
// an opinion from the same harness as the one merging them is not independent.
//
// The name is the bundle's, not the server-assigned id, and it is resolved the
// same way the configured default is. A name the server does not know fails the
// run: falling back would answer on the default harness, which looks like a
// working run and reads like corroboration the work never got. An empty string
// means no preference and takes the default.
type AgentNamer interface {
	AgentName() string
}
