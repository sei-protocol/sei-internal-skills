package driver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// fakeHost is a [Host] that answers from a script.
//
// This is the whole reason [Host] is an interface. What this package decides — the
// run deadline, whether a reply is an answer, which exit code an error becomes, and
// that a close outlives its own cancellation — is four decisions and no I/O. Read
// through a real deployment they need a server, a stream, and an event vocabulary,
// none of which is under test here.
type fakeHost struct {
	// conv is handed back by Open when openErr is nil.
	conv *fakeConversation

	// openErr, when set, is what Open reports instead.
	openErr error

	// closeID and closeErr are what Close reports.
	closeID  string
	closeErr error

	mu sync.Mutex

	// opened and closed record the work each call was asked for, so a test can
	// assert on the identity this package derived from a workload.
	opened []Work
	closed []Work

	// closeCtxLive records whether Close's context was still good when it ran,
	// which is what proves teardown was detached from a cancelled run.
	closeCtxLive bool
}

func (h *fakeHost) Open(ctx context.Context, w Work) (Conversation, error) {
	h.mu.Lock()
	h.opened = append(h.opened, w)
	h.mu.Unlock()
	if h.openErr != nil {
		return nil, h.openErr
	}
	return h.conv, nil
}

func (h *fakeHost) Close(ctx context.Context, w Work) (string, error) {
	h.mu.Lock()
	h.closed = append(h.closed, w)
	h.closeCtxLive = ctx.Err() == nil
	h.mu.Unlock()
	return h.closeID, h.closeErr
}

// fakeConversation answers one turn from a script and records what it was asked.
type fakeConversation struct {
	sessionID string

	// reply and turnErr are what Turn reports. Both may be set at once: a turn that
	// answered and then lost its transport is the ordinary case.
	reply   Reply
	turnErr error

	// answered is what Turn passes to [Ask.Prompt], standing in for a session an
	// earlier dispatch already answered in.
	answered bool

	// prompt is what the workload asked for, captured so a test can assert the
	// second-pass prompt was the one sent.
	prompt string

	// turns counts the exchanges, because this package drives exactly one.
	turns int
}

func (c *fakeConversation) SessionID() string { return c.sessionID }

func (c *fakeConversation) Turn(ctx context.Context, ask Ask) (Reply, error) {
	c.turns++
	c.prompt = ask.Prompt(c.answered)
	return c.reply, c.turnErr
}

// quietLogger discards records. These tests assert on the returned Result, and a
// package whose failure path logs at Error would otherwise make a passing run look
// like a broken one.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// namedWork is a workload that names its own agent, for [AgentNamer].
type namedWork struct {
	testWork
	agent string
}

func (w namedWork) AgentName() string { return w.agent }

// finishedReply is a reply testWork.Complete accepts.
var finishedReply = Reply{
	Text:   "the answer\n```json\n{\"decision\": \"approve\"}\n```",
	ItemID: "item_reply",
	TurnID: "resp_claude_a",
}

// unfinishedReply is a reply testWork.Complete rejects, standing in for an agent
// that committed its opening sentence and stopped.
var unfinishedReply = Reply{
	Text:   "I will start by reading the diff.",
	ItemID: "item_partial",
	TurnID: "resp_claude_a",
	Reason: "",
}

func wrapped(sentinel error, detail string) error {
	return fmt.Errorf("%w: %s", sentinel, detail)
}
