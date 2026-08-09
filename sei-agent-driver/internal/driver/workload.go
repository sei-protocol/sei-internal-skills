package driver

// Workload is the half of a run this package does not decide.
//
// Everything else — resolving the agent, creating or adopting the session,
// following a turn across the streams it outlives, answering the permission
// prompts it raises — is the same whatever the agent is being asked to do, and
// belongs here. What the agent is asked, and how to tell that it has finished
// answering, does not.
type Workload interface {
	// RunKey identifies the unit of work, stably, across dispatches. It is the
	// label a later run matches to adopt the session an earlier one created
	// rather than opening a second, so two dispatches for the same work must
	// agree on it and dispatches for different work must not collide.
	RunKey() string

	// Title names the session for a human reading a session list.
	Title() string

	// Prompt is the first message a fresh session receives.
	Prompt() string

	// AdoptedPrompt is the message a session that already holds this work's
	// history receives instead. It can refer to what the agent did before,
	// because that conversation is still in front of it.
	AdoptedPrompt() string

	// Complete reports whether a reply is a finished answer rather than an agent
	// still working.
	//
	// The driver cannot answer this itself, and that is not an abstraction
	// preference. A terminal-backed session goes idle between tool calls and
	// reports no active response while it does, so both of the server's own
	// signals read "finished" mid-answer. The only reliable reading is whether
	// the reply satisfies the contract the prompt asked for, which only the
	// workload knows. Getting it wrong published an agent's opening sentence as
	// its review of a 1,575-line diff.
	Complete(text string) bool
}
