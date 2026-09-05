package review

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
// mechanically: an approving review event, a status check. A parser that accepts any
// string becomes a privilege defect on the day that lands. An unrecognised value is
// reported as no verdict, with what the agent actually said carried into the log.
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

	// Block is the closing block's bytes as the agent wrote them. Kept so a truncated
	// comment can carry the decision verbatim, rather than a re-rendering of it.
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

// CheckConclusion renders a GitHub check-run conclusion for this review.
//
// Derived rather than asked for separately. The two say the same thing about the same
// review, and one that could disagree with itself -- request_changes beside a passing
// check -- leaves a reader no way to tell which is meant.
//
// What it derives from is the findings, with the decision able to escalate but not to
// clear. A reply that says approve while it lists a blocker still fails: both readings
// came from the same reply, so the one backed by what it actually wrote wins.
//
// Only a BLOCKER decides it. Non-blocking notes -- suggestions, nits, pre-existing
// issues -- do not, because a review that has three suggestions and nothing blocking is
// a review that says the change is fine. Gating on "found anything at all" made approval
// unreachable for real work: the counts published beside the conclusion already tell the
// reader there were three things to say, so the conclusion does not also have to.
func (v Verdict) CheckConclusion() string {
	if !v.HasVerdict() {
		return ""
	}
	if v.Decision() == "request_changes" || len(Blockers(v)) > 0 || v.hasBlockingFinding() {
		return "failure"
	}
	// A comment decision with EMPTY buckets is the prompt's own read-failure signal: it
	// tells the reply to say comment when the diff OR THE TREE could not be read, and a
	// failed tree read arrives here with a truthful diff count and nothing written down.
	// readTheDiff below cannot see it, because the count it reads is the diff's.
	//
	// A notes-only review says comment too, by the same prompt, and that one must stay
	// clean -- it is the whole point of gating on blockers. The buckets separate the two
	// meanings the one word carries: something written down is a review, nothing written
	// down beside a soft decision is a review that did not happen.
	if v.Decision() == "comment" && !v.wroteAnythingDown() {
		return "neutral"
	}
	// Deliberately not failing on pre_existing_issues, whatever severity they carry. A
	// blocker the change did not introduce is already on the base branch, so failing
	// this check would fail every pull request that touches the file until someone
	// fixes it -- and the author who has to clear the check is the one person who did
	// not cause it. PreExisting keeps the severity so the reader can see it.
	//
	// A review that never got the diff has no findings either, so the blocker gate above
	// reads it as clean. A credential failure would then report a green check on every
	// pull request at once -- and, where the caller approves on success, approve every one
	// of them. This is the whole reason the reply carries a line count.
	//
	// Degraded to neutral rather than failed. A missing count means this tool cannot tell a
	// clean review from a review of nothing, which is not the same as knowing the change is
	// bad. And a reply from a session prompted before the field existed omits it without
	// having done anything wrong.
	if !v.readTheDiff() {
		return "neutral"
	}
	// A pre-existing BLOCKER stops short of success without failing. The check does not
	// go red, for the reason above -- the author did not cause it. But a caller that
	// approves on success would otherwise sign off a pull request while the review is
	// saying a blocker sits in the file it touched. Neutral records no position either
	// way, which is the honest one: nothing here blocks the change, and something in
	// the file blocks a reader.
	if v.hasPreExistingBlocker() {
		return "neutral"
	}
	return "success"
}

// hasPreExistingBlocker reports whether the review named a blocker that the change did
// not introduce.
func (v Verdict) hasPreExistingBlocker() bool {
	for _, issue := range PreExisting(v) {
		// Only an explicit suggestion clears. normalizeSeverity returns "" for a word it
		// does not recognise -- critical, P0, or an absent field -- and every other rule
		// reading a severity is rendering a COUNT, where treating the unknown as
		// non-blocking is right. This one withholds an approval, so it has to fail the
		// other way: an unrecognised severity beside "exploitable rce" must not read as
		// harmless.
		if issue.Severity != "suggestion" {
			return true
		}
	}
	return false
}

// wroteAnythingDown reports whether the review recorded anything at all, in any bucket.
//
// Every finding the reply reported, not only the ones that carry a usable line: a note
// dropped for naming no line is still something the review wrote down.
func (v Verdict) wroteAnythingDown() bool {
	return len(reportedFindings(v)) > 0 ||
		len(NonBlockers(v)) > 0 ||
		len(PreExisting(v)) > 0
}

// proseWithoutBlock returns the reply with its closing decision block removed.
//
// The block is the machine half of the reply; callers read it from check.json and the
// findings file, never from a published comment. The whole text when there is no block:
// a reply that never closed one still has a review in it.
//
// A reply that is ONLY its block falls back to the summary the block carries. Stripping
// to nothing would publish a comment that is a provenance footer and no review.
func (v Verdict) proseWithoutBlock() string {
	if v.Block == "" {
		return v.Text
	}
	prose := strings.TrimRight(strings.Replace(v.Text, v.Block, "", 1), " \t\n")
	if strings.TrimSpace(prose) == "" {
		return v.Summary()
	}
	return prose
}

// readTheDiff reports whether the reply affirms it read the change under review.
func (v Verdict) readTheDiff() bool {
	return intField(v.Structured, "read") > 0
}

// hasBlockingFinding reports whether any reported finding calls itself blocking.
//
// Read from every finding the reply offered, not only the ones that can be placed on
// a line. A finding that names no line is dropped from the inline comments, and a
// model omits a line routinely.
//
// Without this, a nil dereference reported as a blocker with no line passes the gate.
// The check then reads "0 findings", and the only trace left is prose in the comment
// body.
//
// The blockers array is checked separately by the caller. It holds what is blocking and
// tied to no line, which is a different bucket rather than a fallback.
func (v Verdict) hasBlockingFinding() bool {
	for _, entry := range reportedFindings(v) {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if findingFrom(fields).Severity == "blocker" {
			return true
		}
	}
	return false
}

// HasVerdict reports whether the turn produced a decision this driver can act
// on. Its absence is a no-verdict outcome: a review that cannot be decided
// mechanically is one a human has to read anyway.
func (v Verdict) HasVerdict() bool { return v.Structured != nil }

// ParseVerdict reads the closing block out of the agent's final message.
//
// A message must carry exactly one fenced block that decides, that block must be last,
// and nothing but whitespace may follow it. All three are required, and the first matters
// most. The agent reviews a pull request whose diff is written by someone else, so a file
// it quotes can itself contain a fenced block naming a decision.
//
// Position cannot tell the agent's own verdict from a verdict it is quoting: if the
// quoted block is physically last, it wins on position alone. Counting them can. Two
// decisions in one message means the message does not say what the agent decided, so this
// refuses rather than picking one.
//
// Refusing costs a review. Guessing costs the integrity of a comment a human reads as
// the reviewer's own words, and once anything acts on the decision it costs more than
// that. An outside contributor can therefore suppress a review by planting a decision
// block in their diff. That is a denial they can already achieve by other means, and it
// is the direction this should fail in.
//
// There is deliberately no fallback to the outermost braces in the message. On a recorded
// trace its only effect was to turn a garbled transport assembly into a publishable
// verdict. It would also make every JSON object the agent ever quotes a candidate.
func ParseVerdict(text string) Verdict {
	v := Verdict{Text: text}

	blocks := fencedJSON.FindAllStringSubmatchIndex(text, -1)
	if len(blocks) == 0 {
		v.Reason = "the message carries no fenced json block"
		return v
	}
	if n := countBlocks(text, blocks, decides); n > 1 {
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

	// The same predicate the count above used. Two definitions of "this block decides"
	// is how a block can stop counting toward the ambiguity rule while still being
	// accepted as the verdict, which is strictly worse than either rule alone.
	if !decides(out) {
		if !decisions[normalizeDecision(out["decision"])] {
			v.Reason = "unrecognised decision " + clip(fmt.Sprintf("%q", fmt.Sprint(out["decision"])), 72)
		} else {
			v.Reason = "the closing block decides without a summary, which the contract asks for"
		}
		return v
	}

	v.Structured = out
	v.Block = text[last[0]:last[1]]
	return v
}

// countBlocks counts how many of a message's fenced blocks parse to a JSON object that
// match accepts.
//
// Separate from picking the closing block, because the count is a precondition rather
// than a candidate search: more than one, and there is nothing to pick. Shared with
// [ParseScoutReport], which reads a reply drawn from the same attacker-written diff and
// owes it the same refusal.
func countBlocks(text string, blocks [][]int, match func(map[string]any) bool) int {
	n := 0
	for _, b := range blocks {
		var out map[string]any
		if json.Unmarshal([]byte(text[b[2]:b[3]]), &out) != nil {
			continue
		}
		if match(out) {
			n++
		}
	}
	return n
}

// decides reports whether a decoded block carries a decision this driver accepts, which
// is what makes it a candidate verdict.
//
// A summary is required alongside it, because the count rule only refuses two deciding
// blocks: a single one, quoted out of the reviewed diff by an agent that died before
// writing its own, satisfies every other rule -- it is the last block and nothing
// follows it -- and publishes that decision under this tool's identity. The prompt asks
// for read, decision and summary together, so a block carrying decision alone is not
// the shape this contract describes. [scoutFields] refuses the same way, on two keys.
//
// Blank does not count. A "summary" of "" or of spaces is a block carrying decision
// alone, which is the shape this refuses -- and [Verdict.Summary] already trims, so
// accepting it here would give one field two answers in one file.
//
// This raises the bar rather than closing the class: a planted block carrying both keys
// still passes. What no key check can supply is proof that the agent wrote the block,
// and the count rule is the only thing that speaks to authorship.
func decides(out map[string]any) bool {
	if !decisions[normalizeDecision(out["decision"])] {
		return false
	}
	summary, ok := out["summary"].(string)
	return ok && strings.TrimSpace(summary) != ""
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
