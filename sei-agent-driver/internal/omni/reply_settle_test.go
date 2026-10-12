package omni

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// newReplyConversation builds a conversation on conv_1 against a fake server that
// answers session reads with sessions, in order, repeating the last.
func newReplyConversation(t *testing.T, budget time.Duration, sessions ...string) (*conversation, *driverFakeServer) {
	t.Helper()
	return newReplyConversationOn(t, budget, driverFakeServerConfig{SessionResps: sessions})
}

// newReplyConversationOn is [newReplyConversation] against a fake server built from
// fcfg.
func newReplyConversationOn(t *testing.T, budget time.Duration, fcfg driverFakeServerConfig) (*conversation, *driverFakeServer) {
	t.Helper()
	fs := newDriverFakeServer(t, fcfg)
	cfg := driverTestConfig(t, fs.URL)
	cfg.ReplySettleBudget = budget
	h := New(cfg, driver.Policy{}, driverTestLogger())
	client, err := h.newClient(t.Context())
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	return &conversation{host: h, client: client, sessionID: "conv_1"}, fs
}

// TestReplyForWaitsForAReplyThatLandsAfterTheEdge pins the ordering a terminal-backed
// harness produces: the edge that ends the turn reaches the server before the turn's
// final message, so the first read finds the turn without its answer.
func TestReplyForWaitsForAReplyThatLandsAfterTheEdge(t *testing.T) {
	t.Parallel()

	c, fs := newReplyConversation(t, 2*time.Second,
		driverSessionWithItems("conv_1", "ag_1"),
		driverSessionWithItems("conv_1", "ag_1",
			driverReplyItem("item_reply", "resp_claude_a",
				driverVerdict("Landed after the edge.", "approve"))),
	)

	verdict, err := c.replyFor(t.Context(),
		&turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}})
	if err != nil {
		t.Fatalf("replyFor returned %v, want the verdict that landed on the second read", err)
	}
	if !carriesDecision(&verdict, "approve") {
		t.Errorf("reply = %q (reason %q), want the answer that landed after the edge",
			verdict.Text, verdict.Reason)
	}
	if got := fs.getSessHits.Load(); got != 2 {
		t.Errorf("session reads = %d, want 2: one at the edge and one that finds the reply", got)
	}
}

// TestReplyForReportsAMissingReplyOnceTheBudgetIsSpent pins the bound: a reply that
// never comes is reported as missing after re-reading for the whole ReplySettleBudget,
// and not much longer.
func TestReplyForReportsAMissingReplyOnceTheBudgetIsSpent(t *testing.T) {
	t.Parallel()

	const budget = 300 * time.Millisecond
	c, fs := newReplyConversation(t, budget, driverSessionWithItems("conv_1", "ag_1"))

	start := time.Now()
	verdict, err := c.replyFor(t.Context(),
		&turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("replyFor returned %v, want a reported missing reply", err)
	}
	if want := "no assistant message carries this turn's response id"; verdict.Reason != want {
		t.Errorf("reason = %q, want %q", verdict.Reason, want)
	}
	if got := fs.getSessHits.Load(); got < 2 {
		t.Errorf("session reads = %d, want the session re-read before giving up", got)
	}
	// At least the whole budget: the last read lands at the deadline, not one
	// interval short of it.
	if elapsed < budget || elapsed > budget+2*time.Second {
		t.Errorf("gave up after %v, want the whole %v budget and not much more", elapsed, budget)
	}
}

// TestReplyForDoesNotWaitOnARefusal pins that a second turn's reply is refused at
// once: waiting cannot make another invocation's answer ours.
func TestReplyForDoesNotWaitOnARefusal(t *testing.T) {
	t.Parallel()

	c, fs := newReplyConversation(t, 2*time.Second,
		driverSessionWithItems("conv_1", "ag_1",
			driverReplyItem("item_ours", "resp_claude_a", driverVerdict("Ours.", "approve")),
			driverReplyItem("item_theirs", "resp_claude_b", driverVerdict("Theirs.", "approve"))),
	)

	verdict, err := c.replyFor(t.Context(),
		&turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}})
	if err != nil {
		t.Fatalf("replyFor returned %v, want a refusal", err)
	}
	if want := "another turn replied into this session while ours ran"; verdict.Reason != want {
		t.Errorf("reason = %q, want %q", verdict.Reason, want)
	}
	if got := fs.getSessHits.Load(); got != 1 {
		t.Errorf("session reads = %d, want 1: a refusal does not wait", got)
	}
}

// hasClosingBlock stands in for the review workload's test for a finished answer: a
// reply that ends in the fenced verdict the prompts ask for.
func hasClosingBlock(text string) bool { return strings.Contains(text, "```json") }

// TestReplyForWaitsPastAProgressNote pins the second shape of the same ordering: at the
// edge, only an earlier message of the turn has reached the server, so the turn's
// latest reply is a progress note rather than the answer.
func TestReplyForWaitsPastAProgressNote(t *testing.T) {
	t.Parallel()

	note := driverReplyItem("item_note", "resp_claude_a",
		"Go isn't installed in the sandbox, so I'll review statically. Next I'm checking the main path.")
	c, fs := newReplyConversation(t, 2*time.Second,
		driverSessionWithItems("conv_1", "ag_1", note),
		driverSessionWithItems("conv_1", "ag_1", note,
			driverReplyItem("item_reply", "resp_claude_a", driverVerdict("The finished review.", "approve"))),
	)

	verdict, err := c.replyFor(t.Context(), &turn{
		id: "resp_claude_a", crossed: true, prior: map[string]bool{}, complete: hasClosingBlock,
	})
	if err != nil {
		t.Fatalf("replyFor returned %v, want the verdict that followed the progress note", err)
	}
	if !carriesDecision(&verdict, "approve") {
		t.Errorf("reply = %q, want the finished answer, not the progress note", verdict.Text)
	}
	if got := fs.getSessHits.Load(); got != 2 {
		t.Errorf("session reads = %d, want 2: one at the edge and one that finds the answer", got)
	}
}

// TestReplyForReturnsAnUnfinishedReplyOnceTheBudgetIsSpent pins what happens when the
// answer never comes: the latest reply goes back to the caller, which judges it as it
// would any other, rather than being turned into a missing reply.
func TestReplyForReturnsAnUnfinishedReplyOnceTheBudgetIsSpent(t *testing.T) {
	t.Parallel()

	const budget = 300 * time.Millisecond
	const text = "Next I'm checking the main path."
	c, fs := newReplyConversation(t, budget,
		driverSessionWithItems("conv_1", "ag_1", driverReplyItem("item_note", "resp_claude_a", text)))

	start := time.Now()
	verdict, err := c.replyFor(t.Context(), &turn{
		id: "resp_claude_a", crossed: true, prior: map[string]bool{}, complete: hasClosingBlock,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("replyFor returned %v, want the unfinished reply", err)
	}
	if verdict.Text != text || verdict.Reason != "" {
		t.Errorf("reply = %q (reason %q), want the latest reply %q with no reason", verdict.Text, verdict.Reason, text)
	}
	if got := fs.getSessHits.Load(); got < 2 {
		t.Errorf("session reads = %d, want the session re-read before giving up", got)
	}
	if elapsed < budget || elapsed > budget+2*time.Second {
		t.Errorf("gave up after %v, want the whole %v budget and not much more", elapsed, budget)
	}
}

// TestReplyForStopsWaitingWhenTheRunEnds pins that the wait between reads gives way to
// the run's context, and that no read starts after it ends. A cancelled job has only a
// few seconds before it is killed, and a wait or a read that outlasts them skips the
// session teardown and leaks the sandbox. The read itself stays detached, so a run
// that has already ended still reads once, as
// [TestReplyForReadsAFinishedTurnEvenAfterTheClockExpires] requires.
func TestReplyForStopsWaitingWhenTheRunEnds(t *testing.T) {
	t.Parallel()

	const budget = 5 * time.Second
	const missing = "no assistant message carries this turn's response id"

	t.Run("a run that ends during the wait reads no more", func(t *testing.T) {
		t.Parallel()
		c, fs := newReplyConversation(t, budget, driverSessionWithItems("conv_1", "ag_1"))
		ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
		defer cancel()

		start := time.Now()
		verdict, err := c.replyFor(ctx, &turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}})
		elapsed := time.Since(start)
		if err != nil || verdict.Reason != missing {
			t.Fatalf("replyFor = (%q, %v), want reason %q", verdict.Reason, err, missing)
		}
		if elapsed > 2*time.Second {
			t.Errorf("waited %v after the run ended, want it to stop well inside the %v budget", elapsed, budget)
		}
		if got := fs.getSessHits.Load(); got != 1 {
			t.Errorf("session reads = %d, want exactly 1: no read starts once the run ended", got)
		}
	})

	t.Run("a run that already ended reads once", func(t *testing.T) {
		t.Parallel()
		c, fs := newReplyConversation(t, budget, driverSessionWithItems("conv_1", "ag_1"))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		verdict, err := c.replyFor(ctx, &turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}})
		if err != nil || verdict.Reason != missing {
			t.Fatalf("replyFor = (%q, %v), want reason %q", verdict.Reason, err, missing)
		}
		if got := fs.getSessHits.Load(); got != 1 {
			t.Errorf("session reads = %d, want exactly 1: nothing to wait for once the run ended", got)
		}
	})
}

// TestReplyForRidesOutAFailedReRead pins that only the first read's error is fatal.
// A later read that fails counts as one that found nothing new, so one transient
// error neither ends the wait early nor drops a reply an earlier read found.
func TestReplyForRidesOutAFailedReRead(t *testing.T) {
	t.Parallel()

	empty := driverSessionWithItems("conv_1", "ag_1")

	t.Run("a later read still finds the reply", func(t *testing.T) {
		t.Parallel()
		c, fs := newReplyConversationOn(t, 2*time.Second, driverFakeServerConfig{
			SessionResps: []string{empty, empty, driverSessionWithItems("conv_1", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", driverVerdict("Landed after a failed read.", "approve")))},
			SessionStatuses: []int{0, http.StatusInternalServerError},
		})

		verdict, err := c.replyFor(t.Context(),
			&turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}})
		if err != nil {
			t.Fatalf("replyFor returned %v, want the verdict a read after the failed one found", err)
		}
		if !carriesDecision(&verdict, "approve") {
			t.Errorf("reply = %q (reason %q), want the answer that landed", verdict.Text, verdict.Reason)
		}
		if got := fs.getSessHits.Load(); got != 3 {
			t.Errorf("session reads = %d, want 3: the edge, the failed read, the one that finds it", got)
		}
	})

	t.Run("the earlier reply stands when every re-read fails", func(t *testing.T) {
		t.Parallel()
		const text = "Next I'm checking the main path."
		failures := make([]int, 100)
		for i := 1; i < len(failures); i++ {
			failures[i] = http.StatusInternalServerError
		}
		c, fs := newReplyConversationOn(t, 300*time.Millisecond, driverFakeServerConfig{
			SessionResps: []string{driverSessionWithItems("conv_1", "ag_1",
				driverReplyItem("item_note", "resp_claude_a", text))},
			SessionStatuses: failures,
		})

		verdict, err := c.replyFor(t.Context(), &turn{
			id: "resp_claude_a", crossed: true, prior: map[string]bool{}, complete: hasClosingBlock,
		})
		if err != nil {
			t.Fatalf("replyFor returned %v, want the reply the first read found", err)
		}
		if verdict.Text != text || verdict.Reason != "" {
			t.Errorf("reply = %q (reason %q), want the first read's reply %q", verdict.Text, verdict.Reason, text)
		}
		if got := fs.getSessHits.Load(); got < 2 {
			t.Errorf("session reads = %d, want re-reads that failed", got)
		}
	})

	t.Run("a failed first read is still an error", func(t *testing.T) {
		t.Parallel()
		c, fs := newReplyConversationOn(t, 2*time.Second, driverFakeServerConfig{
			SessionResps:    []string{empty},
			SessionStatuses: []int{http.StatusInternalServerError},
		})

		if _, err := c.replyFor(t.Context(),
			&turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}}); err == nil {
			t.Fatal("replyFor returned no error, want the first read's error")
		}
		if got := fs.getSessHits.Load(); got != 1 {
			t.Errorf("session reads = %d, want 1: nothing to wait on without a first read", got)
		}
	})
}
