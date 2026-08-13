package xreview

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
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

// Summary returns the review's one- or two-sentence summary, empty when it wrote
// none.
func (v Verdict) Summary() string {
	if v.Structured == nil {
		return ""
	}
	return strings.TrimSpace(stringField(v.Structured, "summary"))
}

// CheckConclusion renders the decision as a GitHub check-run conclusion.
//
// Derived rather than asked for separately. The two say the same thing about the
// same review, and a review that could disagree with itself — request_changes
// beside a passing check — would leave a reader no way to tell which is meant.
func (v Verdict) CheckConclusion() string {
	switch v.Decision() {
	case "request_changes":
		return "failure"
	case "comment":
		return "neutral"
	case "approve":
		return "success"
	}
	return ""
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
