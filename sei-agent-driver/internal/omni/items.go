package omni

import (
	"maps"
	"slices"
	"strings"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// What may be read out of a conversation, and what may not.
//
// These are the guards behind the package doc's publishability rules. They read
// the server's own item shapes, which is why they live here rather than beside the
// [driver.Reply] they produce.

// textPartType is the only content-part type whose text is published.
//
// An allowlist, because this harness emits reasoning and reasoning-summary
// deltas, so a message can carry the model's private working alongside its
// answer. Admitting a part on the presence of a text key rather than its type
// publishes that working to a pull request.
const textPartType = "output_text"

// TurnReply returns the assistant message that answers the given turn.
//
// Attribution is positive: the item must carry that turn's own response id, which
// the server stamps on every item the turn commits. It has to be positive, because
// the negative form — newest assistant message absent from a pre-turn snapshot —
// fails open: anything the snapshot missed is accepted as this turn's reply.
//
// Newest-first, because a turn's last message is its answer.
func TurnReply(items []omnigent.ConversationItem, turnID string) (driver.Reply, bool) {
	if turnID == "" {
		return driver.Reply{}, false
	}
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.ResponseID != turnID {
			continue
		}
		msg, ok := assistantMessage(item)
		if !ok {
			continue
		}
		if body := messageText(msg); body != "" {
			return driver.Reply{Text: body, ItemID: item.ID}, true
		}
	}
	return driver.Reply{}, false
}

// ReplyGroupsSince lists the response ids that gained a publishable assistant
// message since prior, sorted so a log line is stable.
//
// One turn commits exactly one such group. More than one means something else
// committed a reply into this session while our turn ran, the likeliest cause
// being a superseded run whose stop lost the race against its own turn. This
// refuses on that rather than choosing the newest group, because choosing is
// precisely how another invocation's review gets published as this one.
func ReplyGroupsSince(items []omnigent.ConversationItem, prior map[string]bool) []string {
	groups := map[string]bool{}
	for _, item := range items {
		if item.ResponseID == "" || prior[item.ResponseID] {
			continue
		}
		if _, ok := assistantMessage(item); ok {
			groups[item.ResponseID] = true
		}
	}
	return slices.Sorted(maps.Keys(groups))
}

// GroupIsAfterAnchor reports whether every item carrying responseID sits after
// the anchor item in the session's order.
//
// Position, not recency: a stream opens by replaying earlier work, so an earlier
// invocation's completed reply looks newest too. An anchor the session does not
// carry answers false — position cannot be proven against an absent item.
func GroupIsAfterAnchor(items []omnigent.ConversationItem, anchorID, responseID string) bool {
	if anchorID == "" || responseID == "" {
		return false
	}
	anchor := -1
	for i, item := range items {
		if item.ID == anchorID {
			anchor = i
			break
		}
	}
	if anchor < 0 {
		return false
	}
	found := false
	for i, item := range items {
		if item.ResponseID != responseID {
			continue
		}
		if i <= anchor {
			return false
		}
		found = true
	}
	return found
}

// assistantMessage decodes an item that is a publishable assistant message, and
// reports false for anything else.
//
// The Type check comes before the Data decode, and it is the first of three things
// standing between a tool output and the pull request: AsMessageData is a bare
// json.Unmarshal with no discriminator consult, so it decodes a function_call_output
// into a zero-valued MessageData and reports no error. The role and empty-text checks
// behind it would reject that value too -- but resting on a zero-value coincidence,
// for a payload carrying whole diffs and gh responses, is not a guard.
func assistantMessage(item omnigent.ConversationItem) (omnigent.MessageData, bool) {
	switch {
	case item.Type != "message":
		return omnigent.MessageData{}, false
	case item.Status != "completed":
		return omnigent.MessageData{}, false
	case item.CreatedBy != nil:
		// Reject-only, deliberately. The server documents nil for agent, tool and
		// system items *and* for single-user mode, so nil does not attest that the
		// model wrote this. Non-nil does attest that a client wrote it, and a
		// client-authored item is never a turn's reply.
		return omnigent.MessageData{}, false
	}

	msg, err := item.Data.AsMessageData()
	if err != nil {
		return omnigent.MessageData{}, false
	}

	switch {
	case msg.Role != omnigent.MessageDataRoleAssistant:
		return omnigent.MessageData{}, false
	case msg.IsMeta != nil && *msg.IsMeta:
		// Durable context replayed to the agent but hidden from transcripts, such
		// as injected skill instructions. Null on every message this harness produces
		// today, so this rejects nothing; it is here because the contract permits it
		// and such a message is a message item.
		return omnigent.MessageData{}, false
	case msg.Interrupted != nil && *msg.Interrupted:
		// A durable partial response from an interrupted turn. Also null today, and
		// also permitted by the contract.
		return omnigent.MessageData{}, false
	}
	return msg, true
}

// messageText concatenates a message's published text parts.
//
// Parts are joined without a separator because the server already split one
// logical message across them.
func messageText(msg omnigent.MessageData) string {
	var b strings.Builder
	for _, part := range msg.Content {
		if partType, _ := part["type"].(string); partType != textPartType {
			continue
		}
		if text, ok := part["text"].(string); ok {
			b.WriteString(text)
		}
	}
	return b.String()
}
