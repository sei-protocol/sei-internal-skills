package driver

import "context"

// Work identifies the unit of work a session belongs to, and says nothing about
// what will be asked in it. A [Host] needs the identity to find or open the
// session; what to say in it is [Ask].
type Work struct {
	// RunKey identifies the unit of work across dispatches, and is what a host
	// searches on. See [Workload.RunKey].
	RunKey string

	// Title names the session for a human reading a session list.
	Title string

	// Agent names the agent the work must run on. Empty takes the host's default.
	Agent string
}

// Ask is one exchange: what to say, and how to know the answer is finished.
type Ask struct {
	// Prompt returns the message to send.
	//
	// answered reports that this conversation already holds a reply Done accepted.
	// It is a callback rather than a field because only the host can answer it —
	// the conversation may be one an earlier dispatch opened — and it cannot be
	// answered without Done. See [Workload.Prompt].
	Prompt func(answered bool) string

	// Done reports whether text is a finished answer.
	//
	// The caller supplies it because no signal from the server can: a
	// terminal-backed session goes idle between tool calls and reports no active
	// response while it does, so both of the server's own signals read "finished"
	// mid-answer. It is asked only of committed text, never of a streamed partial.
	Done func(text string) bool
}

// Host is the agent deployment a run drives. [Conversation] is one session on it.
//
// The two methods are a pair on the same key: Open returns the session for a unit
// of work, and Close destroys it when that work ends.
//
// This is the whole of what a run needs from a server, and the reason it is an
// interface is that far more than this is involved in providing it. Which of two
// id namespaces an id belongs to, whether a turn's end arrives as a status edge or
// a response lifecycle event, and how many times a stream must be re-established
// to outlast one turn are all facts about one deployment. None of them is a fact
// about running an agent to an answer, which is what this package does.
type Host interface {
	// Open returns the conversation for this work, opening a session only if none
	// carries the run key.
	Open(ctx context.Context, w Work) (Conversation, error)

	// Close destroys the session for this work and returns its id.
	//
	// An empty id means no session carried the run key, which is not an error: work
	// that ended without ever being started has nothing to close.
	//
	// An error wrapping [ErrLeaked] means a session was found and could not be
	// deleted, so its sandbox is held. That is a different outcome from a failure to
	// look, and the caller reports it differently, because nothing reclaims a sandbox
	// on a schedule.
	Close(ctx context.Context, w Work) (sessionID string, err error)
}

// Conversation is one session on a [Host], driven a turn at a time.
type Conversation interface {
	// SessionID names the session driven, for provenance in whatever the caller
	// publishes and for an operator reading a session list.
	SessionID() string

	// Turn sends the prompt and returns the reply to it.
	//
	// A returned reply and a returned error are not exclusive. A turn that answered
	// and then lost its transport is the ordinary case, and discarding the answer
	// costs the whole run.
	Turn(ctx context.Context, ask Ask) (Reply, error)
}
