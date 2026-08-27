package omni

import (
	"testing"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// driverEmptyItemsPage is the prior-id read every one of these makes on open. It
// has to be empty, or the reply under test would already be on the session and so
// would not count as new.
const driverEmptyItemsPage = `{"data":[],"has_more":false}`

// driverWholeTranscript is the paged route's answer: the anchor, this turn's tool
// traffic, and its reply, in transcript order. The listing carries no payload --
// that is the shape the route sends -- so it can prove position and nothing else.
func driverWholeTranscript() string {
	return `{"data":[{"id":"item_1","type":"message"},` +
		`{"id":"item_tool","response_id":"resp_claude_a","type":"function_call"},` +
		`{"id":"item_reply","response_id":"resp_claude_a","type":"message"}],` +
		`"has_more":false}`
}

// TestListItemsShapeCannotCarryTheContentGuards is why the truncation is not fixed
// by simply paging the salvage read.
//
// The paged route spreads each payload's fields onto the item instead of nesting
// them under data, so [omnigent.ConversationItem.Data] is nil on everything it
// yields. The guards that decide publishability all decode that field, so they
// reject every item from that route -- which fails closed, and would make salvage
// never recover anything at all. Position is the one thing the flat shape can
// prove, and that is all [conversation.anchorPrecedes] asks it for.
func TestListItemsShapeCannotCarryTheContentGuards(t *testing.T) {
	t.Parallel()

	flat := omnigent.ConversationItem{
		ID:         "item_reply",
		ResponseID: "resp_claude_a",
		Type:       "message",
		Status:     "completed",
	}

	if _, ok := assistantMessage(flat); ok {
		t.Error("assistantMessage accepted an item with no payload; the content guards " +
			"would have to be reimplemented against the flat shape, and there must be " +
			"only one answer to whether a message may be published")
	}
	if got := replyGroupsSince([]omnigent.ConversationItem{flat}, nil); len(got) != 0 {
		t.Errorf("replyGroupsSince = %v, want none from the flat shape", got)
	}
	if _, ok := turnReply([]omnigent.ConversationItem{flat}, "resp_claude_a"); ok {
		t.Error("turnReply read text off an item that carries none")
	}
	// The exception, and the reason the fallback is possible at all.
	items := []omnigent.ConversationItem{{ID: "item_1"}, flat}
	if !groupIsAfterAnchor(items, "item_1", "resp_claude_a") {
		t.Error("position needs only id and response_id, so it must survive the flat shape")
	}
}

// TestSalvageReachesAnAnchorThePageWindowTruncated covers the turn long enough to
// push its own prompt out of the snapshot.
//
// [omnigent.GetSessionOptions.IncludeItems] returns the newest hundred items and
// marks no truncation. A turn whose tool traffic overruns that window is also the
// turn that spans enough streams to need salvaging, so the two conditions arrive
// together rather than independently.
func TestSalvageReachesAnAnchorThePageWindowTruncated(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("A long review, finished during the reconnect gap.", "approve")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_trunc", "ag_1"),
		// The prompt is echoed, then the connection dies. The turn ended while it
		// was down, so no end edge will ever arrive and salvage is the only route.
		StreamFrames:      []string{driverAckFrame(), driverConsumedFrame("item_1")},
		LaterStreamFrames: []string{driverAckFrame()},
		SessionResps: []string{
			// item_1 has fallen off the window. The reply has not: it is the newest
			// thing on the session.
			driverSessionItemsInOrder("conv_trunc", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", reply)),
		},
		ItemsResps: []string{driverEmptyItemsPage, driverWholeTranscript()},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.RunDeadline = 5 * time.Second
	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 96})

	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK (%d): the reply is committed and its "+
			"position is provable from the paged transcript",
			result.ExitCode, driver.ExitOK)
	}
	if result.Reply == nil || !carriesDecision(result.Reply, "approve") {
		t.Fatalf("Reply = %+v, want the committed review", result.Reply)
	}
}

// TestSalvagePagesOnlyWhenTheWindowIsShort pins the cost of the fallback.
//
// The paged walk runs on the salvage path, which a long turn takes once per dropped
// stream, so asking for it when the snapshot already answers would add a listing to
// every reconnect. An anchor inside the window is decisive on its own.
func TestSalvagePagesOnlyWhenTheWindowIsShort(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("A short review.", "approve")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:        []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp:        driverSessionResp("conv_short", "ag_1"),
		StreamFrames:      []string{driverAckFrame(), driverConsumedFrame("item_1")},
		LaterStreamFrames: []string{driverAckFrame()},
		SessionResps: []string{
			// The anchor is in the window, so position is already provable.
			driverSessionWithItems("conv_short", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", reply)),
		},
		ItemsResps: []string{driverEmptyItemsPage},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.RunDeadline = 5 * time.Second
	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 97})

	if result.Reply == nil {
		t.Fatal("Reply = nil, want the recovered review")
	}
	// One listing: the prior-id read on open. The salvage never paged.
	if got := len(fs.ItemsQueries()); got != 1 {
		t.Errorf("item listings = %d, want 1 (the prior-id read alone): a snapshot that "+
			"already carries the anchor must not trigger a walk", got)
	}
}

// TestSalvageStillRefusesAReplyThePagedReadPlacesEarlier keeps the wider read from
// becoming a way around the position rule.
//
// A previous invocation's reply is absent from the anchor's window too, so it
// reaches the fallback. The paged transcript then has to refuse it on the same
// grounds the snapshot would: it sits before this turn's prompt.
func TestSalvageStillRefusesAReplyThePagedReadPlacesEarlier(t *testing.T) {
	t.Parallel()

	stale := driverVerdict("An earlier dispatch's review.", "request_changes")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:        []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp:        driverSessionResp("conv_stale", "ag_1"),
		StreamFrames:      []string{driverAckFrame(), driverConsumedFrame("item_1")},
		LaterStreamFrames: []string{driverAckFrame()},
		SessionResps: []string{
			// Neither the anchor nor any ordering: only the stale reply.
			driverSessionItemsInOrder("conv_stale", "ag_1",
				driverReplyItem("item_old", "resp_previous", stale)),
		},
		ItemsResps: []string{
			driverEmptyItemsPage,
			// The whole transcript puts that reply before this turn's prompt.
			`{"data":[{"id":"item_old","response_id":"resp_previous","type":"message"},` +
				`{"id":"item_1","type":"message"}],"has_more":false}`,
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.RunDeadline = 5 * time.Second
	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 98})

	if result.Reply != nil && result.Reply.Text != "" {
		t.Errorf("published %q: a reply the transcript places before this turn's prompt "+
			"belongs to an earlier dispatch", result.Reply.Text)
	}
	if result.ExitCode == driver.ExitOK {
		t.Error("ExitCode = driver.ExitOK on a reply that is not this turn's")
	}
}

// TestSalvageDoesNotPageWhenTheWindowAlreadyDecides is the other half of the cost
// rule. An anchor inside the window with no reply behind it is a refusal the
// snapshot has already settled, and paging cannot overturn it -- so asking would
// add a listing to a reconnect that gains nothing.
func TestSalvageDoesNotPageWhenTheWindowAlreadyDecides(t *testing.T) {
	t.Parallel()

	stale := driverVerdict("An earlier dispatch's review.", "request_changes")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:        []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp:        driverSessionResp("conv_decided", "ag_1"),
		StreamFrames:      []string{driverAckFrame(), driverConsumedFrame("item_1")},
		LaterStreamFrames: []string{driverAckFrame()},
		SessionResps: []string{
			// The anchor is present, and the only new reply sits before it.
			driverSessionItemsInOrder("conv_decided", "ag_1",
				driverReplyItem("item_old", "resp_previous", stale),
				driverPromptItem(driverAnchorItemID)),
		},
		ItemsResps: []string{driverEmptyItemsPage},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.RunDeadline = 5 * time.Second
	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 99})

	if result.Reply != nil && result.Reply.Text != "" {
		t.Errorf("published %q, which sits before this turn's prompt", result.Reply.Text)
	}
	if got := len(fs.ItemsQueries()); got != 1 {
		t.Errorf("item listings = %d, want 1 (the prior-id read alone): the window held "+
			"the anchor, so the answer was already settled", got)
	}
}

// TestSalvageRefusesWhenThePagedReadFails keeps a failed widening from reading as
// permission. An incomplete transcript proves no position in either direction, and
// the reply under it may belong to an earlier dispatch.
func TestSalvageRefusesWhenThePagedReadFails(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("A review whose position cannot be established.", "approve")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages:        []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp:        driverSessionResp("conv_pagefail", "ag_1"),
		StreamFrames:      []string{driverAckFrame(), driverConsumedFrame("item_1")},
		LaterStreamFrames: []string{driverAckFrame()},
		SessionResps: []string{
			driverSessionItemsInOrder("conv_pagefail", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", reply)),
		},
		// The widening read answers with something that will not decode.
		ItemsResps: []string{driverEmptyItemsPage, `{"data":[ NOT JSON`},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.RunDeadline = 5 * time.Second
	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 100})

	if result.Reply != nil && result.Reply.Text != "" {
		t.Errorf("published %q on a transcript that could not be read", result.Reply.Text)
	}
	if result.ExitCode == driver.ExitOK {
		t.Error("ExitCode = driver.ExitOK though no position was ever established")
	}
}
