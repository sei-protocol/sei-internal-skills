package omni

import (
	"reflect"
	"strings"
	"testing"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
)

// TestTurnReply covers positive attribution: the reply must carry the turn's own
// response id, and nothing else qualifies however new it is.
func TestTurnReply(t *testing.T) {
	t.Parallel()

	t.Run("the newest message stamped for this turn wins", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictItem(t, "msg_a1", "resp_a", "assistant", "an earlier message this turn"),
			verdictItem(t, "msg_a2", "resp_a", "assistant", "the final answer"),
		}
		reply, ok := TurnReply(items, "resp_a")
		if !ok || reply.Text != "the final answer" || reply.ItemID != "msg_a2" {
			t.Errorf("TurnReply = %+v (ok=%v), want the final answer from msg_a2", reply, ok)
		}
	})

	t.Run("a newer message from another turn is not the reply", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictItem(t, "msg_ours", "resp_a", "assistant", "ours"),
			verdictItem(t, "msg_theirs", "resp_b", "assistant", "newer, and not ours"),
		}
		reply, ok := TurnReply(items, "resp_a")
		if !ok || reply.Text != "ours" {
			t.Errorf("TurnReply = %+v (ok=%v), want ours: recency must not outrank the turn id",
				reply, ok)
		}
	})

	t.Run("an unstamped message is never the reply", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictItem(t, "msg_a1", "", "assistant", "carries no response id"),
		}
		if _, ok := TurnReply(items, "resp_a"); ok {
			t.Error("TurnReply matched an item carrying no response id")
		}
	})

	t.Run("an empty turn id matches nothing", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictItem(t, "msg_a1", "", "assistant", "would match a naive comparison"),
		}
		if _, ok := TurnReply(items, ""); ok {
			t.Error("TurnReply matched on an empty turn id, which would publish unattributed text")
		}
	})

	t.Run("a user message stamped for this turn is not the reply", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictItem(t, "msg_u1", "resp_a", "user", "the prompt we sent"),
		}
		if _, ok := TurnReply(items, "resp_a"); ok {
			t.Error("TurnReply matched a user message, which would publish our own prompt")
		}
	})

	t.Run("no items yields no reply", func(t *testing.T) {
		t.Parallel()
		if _, ok := TurnReply(nil, "resp_a"); ok {
			t.Error("TurnReply matched on an empty snapshot")
		}
	})
}

// TestAssistantMessageRejectsToolOutput pins the one ordering that keeps a tool
// output out of a pull request comment.
//
// AsMessageData is a bare json.Unmarshal with no discriminator consult, so it
// decodes a payload that happens to look like a message and reports no error
// whatever the item's declared type. The Type check therefore has to come first.
// Revert it and this test publishes the item.
func TestAssistantMessageRejectsToolOutput(t *testing.T) {
	t.Parallel()

	item := verdictItem(t, "call_out", "resp_a", "assistant", "a whole diff, or a gh auth dump")
	item.Type = "function_call_output"

	if _, ok := assistantMessage(item); ok {
		t.Error("assistantMessage accepted a function_call_output whose payload decoded as a " +
			"message: the type discriminator must be checked before the union decode")
	}
	if _, ok := TurnReply([]omnigent.ConversationItem{item}, "resp_a"); ok {
		t.Error("TurnReply attributed a tool output as the turn's reply")
	}
}

// TestAssistantMessageRejectsUnpublishableShapes covers the remaining rejects: an
// incomplete item, a human-authored one, injected context, and an interrupted
// partial response.
func TestAssistantMessageRejectsUnpublishableShapes(t *testing.T) {
	t.Parallel()

	author := "someone@example.com"
	yes := true

	tests := []struct {
		name   string
		mutate func(*omnigent.ConversationItem, *omnigent.MessageData)
	}{
		{"an incomplete item", func(i *omnigent.ConversationItem, _ *omnigent.MessageData) {
			i.Status = "in_progress"
		}},
		{"a human-authored item", func(i *omnigent.ConversationItem, _ *omnigent.MessageData) {
			i.CreatedBy = &author
		}},
		{"injected context", func(_ *omnigent.ConversationItem, m *omnigent.MessageData) {
			m.IsMeta = &yes
		}},
		{"an interrupted partial response", func(_ *omnigent.ConversationItem, m *omnigent.MessageData) {
			m.Interrupted = &yes
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			msg := omnigent.MessageData{
				Role:    omnigent.MessageDataRoleAssistant,
				Content: []map[string]any{{"type": "output_text", "text": "text"}},
			}
			item := omnigent.ConversationItem{
				ID: "msg_1", ResponseID: "resp_a", Type: "message", Status: "completed",
			}
			tc.mutate(&item, &msg)

			var data omnigent.ConversationItem_Data
			if err := data.FromMessageData(msg); err != nil {
				t.Fatalf("FromMessageData: %v", err)
			}
			item.Data = data

			if _, ok := assistantMessage(item); ok {
				t.Errorf("assistantMessage accepted %s", tc.name)
			}
		})
	}
}

// TestMessageTextPublishesOnlyOutputText pins the content-part allowlist.
//
// This harness emits reasoning and reasoning-summary deltas, so a message can
// carry the model's private working next to its answer. Admitting a part on the
// presence of a text key rather than on its type posts that working to the pull
// request as review prose.
func TestMessageTextPublishesOnlyOutputText(t *testing.T) {
	t.Parallel()

	msg := omnigent.MessageData{
		Role: omnigent.MessageDataRoleAssistant,
		Content: []map[string]any{
			{"type": "reasoning", "text": "PRIVATE_REASONING"},
			{"type": "output_text", "text": "the review, "},
			{"type": "reasoning_summary", "summary_text": "PRIVATE_SUMMARY"},
			{"type": "output_text", "text": "in two parts"},
			{"type": "input_image", "image_url": "https://example.invalid/x.png"},
		},
	}

	got := messageText(msg)
	if got != "the review, in two parts" {
		t.Errorf("messageText = %q, want only the output_text parts joined", got)
	}
	for _, secret := range []string{"PRIVATE_REASONING", "PRIVATE_SUMMARY"} {
		if strings.Contains(got, secret) {
			t.Errorf("messageText published %s", secret)
		}
	}
}

// TestReplyGroupsSince covers the ambiguity cross-check: it counts response groups
// that gained a publishable assistant message, and nothing else.
func TestReplyGroupsSince(t *testing.T) {
	t.Parallel()

	t.Run("one new group is the normal case", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictItem(t, "msg_old", "resp_old", "assistant", "history"),
			verdictItem(t, "msg_new", "resp_a", "assistant", "this turn"),
		}
		got := ReplyGroupsSince(items, map[string]bool{"resp_old": true})
		if !reflect.DeepEqual(got, []string{"resp_a"}) {
			t.Errorf("ReplyGroupsSince = %v, want [resp_a]", got)
		}
	})

	t.Run("two new groups are the signature of a shared session", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictItem(t, "msg_a", "resp_a", "assistant", "ours"),
			verdictItem(t, "msg_b", "resp_b", "assistant", "theirs"),
		}
		got := ReplyGroupsSince(items, map[string]bool{})
		if !reflect.DeepEqual(got, []string{"resp_a", "resp_b"}) {
			t.Errorf("ReplyGroupsSince = %v, want both groups, sorted", got)
		}
	})

	t.Run("a group with no publishable message does not count", func(t *testing.T) {
		t.Parallel()
		items := []omnigent.ConversationItem{
			verdictFunctionCallItem(t, "call_1", "resp_tools"),
			verdictItem(t, "msg_u", "resp_users", "user", "a prompt"),
		}
		if got := ReplyGroupsSince(items, map[string]bool{}); len(got) != 0 {
			t.Errorf("ReplyGroupsSince = %v, want none: only a reply can be mistaken for a reply", got)
		}
	})
}

// TestGroupIsAfterAnchorRefusesAnEarlierReply pins the invariant doc.go states
// and this package once broke: a stream opens by replaying earlier work, so a
// completed reply from a previous invocation looks exactly like this turn's under
// a newest-not-seen-before filter.
func TestGroupIsAfterAnchorRefusesAnEarlierReply(t *testing.T) {
	t.Parallel()

	items := []omnigent.ConversationItem{
		{ID: "i1", ResponseID: "resp_earlier", Type: "message"},
		{ID: "i2", ResponseID: "", Type: "message"}, // this turn's prompt
		{ID: "i3", ResponseID: "resp_mine", Type: "message"},
	}
	for _, tc := range []struct {
		name             string
		anchor, response string
		want             bool
	}{
		{"this turn's reply, after the prompt", "i2", "resp_mine", true},
		{"an earlier invocation's reply", "i2", "resp_earlier", false},
		{"the anchor is not in the session", "gone", "resp_mine", false},
		{"no anchor at all", "", "resp_mine", false},
		{"a response the session does not carry", "i2", "resp_absent", false},
	} {
		if got := GroupIsAfterAnchor(items, tc.anchor, tc.response); got != tc.want {
			t.Errorf("%s: GroupIsAfterAnchor = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCrossBoundaryResolvesAPendingAnchorToItsItem covers the cold-start path:
// a prompt parked before the runtime is up is echoed by the pending id it drains,
// and that id names no conversation item. Comparing a reply's position against it
// would refuse every recovery on that path — discarding a finished review whose
// stream happened to drop.
func TestCrossBoundaryResolvesAPendingAnchorToItsItem(t *testing.T) {
	t.Parallel()

	pending := "pending_7"
	tn := &turn{anchor: pending}
	tn.crossBoundary(omnigent.SessionInputConsumedEvent{
		Data: omnigent.SessionInputConsumedPayload{
			ItemID:           "item_9",
			ClearedPendingID: &pending,
		},
	})
	if !tn.crossed {
		t.Fatal("the boundary was not recognised for a pending anchor")
	}
	if tn.anchorItem != "item_9" {
		t.Errorf("anchorItem = %q, want the item the pending input drained into", tn.anchorItem)
	}

	// And the ordinary path still resolves to itself.
	direct := &turn{anchor: "item_1"}
	direct.crossBoundary(omnigent.SessionInputConsumedEvent{
		Data: omnigent.SessionInputConsumedPayload{ItemID: "item_1"},
	})
	if direct.anchorItem != "item_1" {
		t.Errorf("anchorItem = %q, want item_1", direct.anchorItem)
	}
}

// verdictItem builds a real omnigent.ConversationItem carrying a MessageData
// payload, via the union type's own From accessor rather than a hand-rolled JSON
// shape.
func verdictItem(t *testing.T, id, responseID, role, text string) omnigent.ConversationItem {
	t.Helper()

	var data omnigent.ConversationItem_Data
	if err := data.FromMessageData(omnigent.MessageData{
		Role:    omnigent.MessageDataRole(role),
		Content: []map[string]any{{"type": "output_text", "text": text}},
	}); err != nil {
		t.Fatalf("FromMessageData: %v", err)
	}
	return omnigent.ConversationItem{
		ID: id, ResponseID: responseID, Type: "message", Status: "completed", Data: data,
	}
}

// verdictFunctionCallItem builds a tool call the turn made along the way, which
// attribution must skip over.
func verdictFunctionCallItem(t *testing.T, id, responseID string) omnigent.ConversationItem {
	t.Helper()

	var data omnigent.ConversationItem_Data
	if err := data.FromFunctionCallData(omnigent.FunctionCallData{
		CallID: "call_1", Name: "search.web", Arguments: "{}",
	}); err != nil {
		t.Fatalf("FromFunctionCallData: %v", err)
	}
	return omnigent.ConversationItem{
		ID: id, ResponseID: responseID, Type: "function_call", Status: "completed", Data: data,
	}
}
