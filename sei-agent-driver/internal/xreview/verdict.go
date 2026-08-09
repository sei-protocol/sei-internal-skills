package xreview

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
)

// fencedJSON finds a ```json fenced object. Non-greedy between the fences and
// anchored on a brace, so a message that explains itself in prose and then emits
// one block yields the block.
var fencedJSON = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// decisions is the closed set of decisions this driver accepts.
//
// Closed rather than open because the decision is on its way to being acted on
// mechanically — an approving review event, a status check — and a parser that
// accepts any string becomes a privilege defect on the day that lands. An
// unrecognised value is reported as no verdict, with what the agent actually said
// carried into the log.
var decisions = map[string]bool{
	"approve":         true,
	"comment":         true,
	"request_changes": true,
}

// textPartType is the only content-part type whose text is published.
//
// An allowlist, because this harness emits reasoning and reasoning-summary
// deltas, so a message can carry the model's private working alongside its
// answer. Admitting a part on the presence of a text key rather than its type
// publishes that working to a pull request.
const textPartType = "output_text"

// Verdict is what the review turn produced.
type Verdict struct {
	// Text is the agent's final message verbatim. This is what gets posted, so
	// it is the agent's words and not this package's paraphrase.
	Text string

	// Structured is the decoded closing block, set only when that block carried
	// a recognised decision. Nil otherwise, which is not an error: a turn can end
	// in prose, and the caller decides whether that counts.
	Structured map[string]any

	// Block is the closing block's bytes as the agent wrote them, kept so a
	// truncated comment can carry the decision verbatim rather than a
	// re-rendering of it.
	Block string

	// TurnID and ItemID are where this text came from, carried so a published
	// comment can name its own provenance.
	TurnID string
	ItemID string

	// Reason renders why Structured is nil, for the operator who has to act on
	// it. Empty when there is a verdict.
	Reason string
}

// Decision returns the verdict's decision, normalised. Empty when there is no
// verdict, which by construction is the only way it can be empty.
func (v Verdict) Decision() string {
	if v.Structured == nil {
		return ""
	}
	return normalizeDecision(v.Structured["decision"])
}

// HasVerdict reports whether the turn produced a decision this driver can act
// on. Its absence is a no-verdict outcome: a review that cannot be decided
// mechanically is one a human has to read anyway.
func (v Verdict) HasVerdict() bool { return v.Structured != nil }

// ParseVerdict reads the closing block out of the agent's final message.
//
// A message must carry exactly one fenced block that decides, that block must be
// last, and nothing but whitespace may follow it. All three are required, and the
// first is the one that matters most: the agent reviews a pull request whose diff
// is written by someone else, so a file it quotes can itself contain a fenced
// block naming a decision. Position cannot tell the agent's own verdict from a
// verdict it is quoting — if the quoted block is physically last it wins on
// position alone. Counting them can: two decisions in one message means the
// message does not say what the agent decided, so this refuses rather than
// picking one.
//
// Refusing costs a review. Guessing costs the integrity of a comment a human
// reads as the reviewer's own words, and once anything acts on the decision it
// costs more than that. An outside contributor can therefore suppress a review by
// planting a decision block in their diff, which is a denial they can already
// achieve by other means and is the direction this should fail in.
//
// There is deliberately no fallback to the outermost braces in the message. On a
// recorded trace its only effect was to turn a garbled transport assembly into a
// publishable verdict, and it would make every JSON object the agent ever quotes
// a candidate.
func ParseVerdict(text string) Verdict {
	v := Verdict{Text: text}

	blocks := fencedJSON.FindAllStringSubmatchIndex(text, -1)
	if len(blocks) == 0 {
		v.Reason = "the message carries no fenced json block"
		return v
	}
	if n := decidingBlocks(text, blocks); n > 1 {
		v.Reason = fmt.Sprintf(
			"%d fenced blocks carry a decision, so the message does not say which is the verdict", n)
		return v
	}

	last := blocks[len(blocks)-1]
	if trailing := strings.TrimSpace(text[last[1]:]); trailing != "" {
		v.Reason = fmt.Sprintf("%d bytes follow the closing block, which must be last",
			len(trailing))
		return v
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text[last[2]:last[3]]), &out); err != nil {
		v.Reason = "the closing block is not a json object"
		return v
	}

	if !decisions[normalizeDecision(out["decision"])] {
		v.Reason = "unrecognised decision " + clip(fmt.Sprintf("%q", fmt.Sprint(out["decision"])), 72)
		return v
	}

	v.Structured = out
	v.Block = text[last[0]:last[1]]
	return v
}

// decidingBlocks counts how many of a message's fenced blocks parse to a JSON
// object carrying a recognised decision.
//
// Separate from picking the verdict because the count is a precondition, not a
// candidate search: more than one and there is no verdict to pick.
func decidingBlocks(text string, blocks [][]int) int {
	n := 0
	for _, b := range blocks {
		var out map[string]any
		if json.Unmarshal([]byte(text[b[2]:b[3]]), &out) != nil {
			continue
		}
		if decisions[normalizeDecision(out["decision"])] {
			n++
		}
	}
	return n
}

// normalizeDecision lowercases and trims a decision value, yielding "" for
// anything that is not a JSON string.
//
// The normalisation is for a model that capitalises its own answer. It is not a
// vocabulary: "changes_requested" stays unrecognised.
func normalizeDecision(raw any) string {
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(s))
}

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
