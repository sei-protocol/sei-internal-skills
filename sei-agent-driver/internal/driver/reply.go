package driver

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
)

// textPartType is the only content-part type whose text is published.
//
// An allowlist, because this harness emits reasoning and reasoning-summary
// deltas, so a message can carry the model's private working alongside its
// answer. Admitting a part on the presence of a text key rather than its type
// publishes that working to a pull request.
const textPartType = "output_text"

// Reply is the attributed answer to one turn.
type Reply struct {
	// Text is the message's published text.
	Text string

	// ItemID is the conversation item it came from, recorded so a published
	// comment can name its own provenance.
	ItemID string

	// PartTypes counts the content-part types the message carried, so a run
	// records what it declined to publish and not only what it published. This is
	// the instrument for whether a reasoning part can reach an assistant message
	// on this harness, which no recorded trace settles.
	PartTypes map[string]int

	// TurnID is the response this reply answers, carried so a caller publishing
	// it can name its own provenance.
	TurnID string

	// Reason renders why there is no usable reply, for the operator who has to
	// act on it. Empty when Text carries one.
	Reason string
}

// TurnReply returns the assistant message that answers the given turn.
//
// Attribution is positive: the item must carry that turn's own response id, which
// the server stamps on every item the turn commits. It has to be positive, because
// the negative form — newest assistant message absent from a pre-turn snapshot —
// fails open: anything the snapshot missed is accepted as this turn's reply.
//
// Newest-first, because a turn's last message is its answer.
func TurnReply(items []omnigent.ConversationItem, turnID string) (Reply, bool) {
	if turnID == "" {
		return Reply{}, false
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
			return Reply{Text: body, ItemID: item.ID, PartTypes: partTypes(msg)}, true
		}
	}
	return Reply{}, false
}

// ResponseIDs is the set of response ids present in a snapshot, captured before
// the turn so [ReplyGroupsSince] can tell this turn's work from the session's
// history.
func ResponseIDs(items []omnigent.ConversationItem) map[string]bool {
	ids := map[string]bool{}
	for _, item := range items {
		if item.ResponseID != "" {
			ids[item.ResponseID] = true
		}
	}
	return ids
}

// ReplyGroupsSince lists the response ids that gained a publishable assistant
// message since prior, sorted so a log line is stable.
//
// One turn commits exactly one such group. More than one means something else
// committed a reply into this session while our turn ran, the likeliest cause
// being a superseded run whose stop lost the race against its own turn. The
// driver refuses on that rather than choosing the newest group, because choosing
// is precisely how another invocation's review gets published as this one.
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

// assistantMessage decodes an item that is a publishable assistant message, and
// reports false for anything else.
//
// The Type check comes before the Data decode, and that ordering is the only
// thing standing between a tool output and the pull request: AsMessageData is a
// bare json.Unmarshal with no discriminator consult, so it decodes a
// function_call_output into a zero-valued MessageData and reports no error. Tool
// output on this path carries whole diffs and gh responses.
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
		// as injected skill instructions. Null on every message claude-native has
		// produced in the recorded traces, so this does nothing today; it is here
		// because the contract permits it and such a message is a message item.
		return omnigent.MessageData{}, false
	case msg.Interrupted != nil && *msg.Interrupted:
		// A durable partial response from an interrupted turn. Also null
		// throughout the traces, and also permitted by the contract.
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

// partTypes counts a message's content-part types. An absent or non-string type
// is counted under "" rather than dropped, so a shape this driver does not
// understand still shows up in the log.
func partTypes(msg omnigent.MessageData) map[string]int {
	counts := map[string]int{}
	for _, part := range msg.Content {
		partType, _ := part["type"].(string)
		counts[partType]++
	}
	return counts
}

// clip bounds a value taken from model output before it reaches a log line,
// cutting on a rune boundary so the line stays valid UTF-8.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max] + "…"
}

// secretPatterns match shapes that must never reach a public pull request. The
// agent holds gh credentials inside its sandbox and can quote anything it reads,
// and the repositories this posts to are public.
var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"github token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`)},
	{"github pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	{"aws access key id", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"pem private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"json web token", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}`)},
}

// ScanSecrets names the first credential shape found in text, or "" when there is
// none. The literals are values this process holds and must never republish.
//
// Only the pattern's name is returned, never what matched: a diagnostic that
// quoted the match would leak the thing it exists to protect.
func ScanSecrets(text string, literals ...string) string {
	for _, literal := range literals {
		// A short or empty literal would match everything. An unset credential is
		// not a reason to refuse every review.
		if len(literal) >= 8 && strings.Contains(text, literal) {
			return "a credential this process holds"
		}
	}
	for _, pattern := range secretPatterns {
		if pattern.re.MatchString(text) {
			return pattern.name
		}
	}
	return ""
}
