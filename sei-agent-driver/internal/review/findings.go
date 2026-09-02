package review

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Finding is one observation a review made about a specific line.
//
// It is the structured half of a review, and the only half that can be posted
// against the code it is about. The prose is what a person reads; this is what a
// machine can place.
type Finding struct {
	// File is the path as the diff names it, repository-relative. A finding naming no
	// file cannot be placed and is dropped rather than posted somewhere arbitrary.
	//
	// It is the path field of a GitHub review comment, which travels in a JSON request
	// body. So it may hold a space, a parenthesis or a quote — those are ordinary in a
	// path git tracks — and a caller that puts it in a shell command has to quote it.
	// postablePath states what this package refuses.
	File string `json:"file"`

	// Line is the line within that file. Zero means the agent gave none, which is
	// placeable only at the top of the file, so such a finding is dropped too.
	Line int `json:"line"`

	// Side is which side of the diff Line counts against: RIGHT for the new file, LEFT for
	// the old one. It is what makes a finding about a removed line placeable at all. Without
	// it, such a finding lands on an unrelated new line or is dropped.
	//
	// Empty means RIGHT, which is what a finding naming no side means and what
	// nearly every finding is.
	Side string `json:"side"`

	// Severity is blocker, suggestion or nit, carried verbatim once recognised:
	// it is the agent's word, and rewriting it would misreport what the review
	// said. A review speaking the high/medium/low vocabulary maps onto it.
	Severity string `json:"severity"`

	// Detail is what is wrong and why it matters.
	Detail string `json:"detail"`
}

// PreExistingIssue is a problem the change did not introduce.
//
// Kept apart from [Finding] because the two answer different questions. A finding is
// about this pull request. A pre-existing issue is about the code it landed in, and
// presenting one as the other tells an author they broke something they did not touch.
type PreExistingIssue struct {
	// Severity is blocker or suggestion. Blocker is reserved for something critical on its
	// own: an exploitable hole, data loss, a likely outage. It is not for anything the
	// change happened to sit near.
	Severity string `json:"severity"`

	// Body identifies the location and the impact in one line.
	Body string `json:"body"`
}

// severityAliases maps the older vocabulary onto the current one. A session
// carries its first prompt in context and answers in the words that prompt
// used, so both are spoken here.
var severityAliases = map[string]string{
	"high":   "blocker",
	"medium": "suggestion",
	"low":    "nit",
}

// normalizeSeverity returns the current word for a severity, and "" for one this
// does not recognise. An unrecognised severity is not guessed at: the caller
// decides what to do with a finding whose weight the agent did not state.
func normalizeSeverity(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if mapped, ok := severityAliases[s]; ok {
		return mapped
	}
	switch s {
	case "blocker", "suggestion", "nit":
		return s
	}
	return ""
}

// normalizeSide returns RIGHT or LEFT, defaulting to RIGHT.
func normalizeSide(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "LEFT") {
		return "LEFT"
	}
	return "RIGHT"
}

// maxPlaceableFindings bounds how many inline comments one reply can produce.
//
// A caller posts one comment per entry under the bot's identity. The scout path is
// bounded for the same reason. This one was not, so a reply that looped could have the
// bot write thousands of comments on one pull request.
const maxPlaceableFindings = 50

// PlaceableFindings returns the findings a caller can post against a line.
//
// Only the placeable ones, because a review comment needs a path and a line, and there
// is nowhere sensible to put one that has neither. What is dropped here is still in the
// prose the summary comment carries. Nothing is lost from the review, only from the
// inline placement.
//
// includeNits carries [Request.IncludeNits]. It is enforced here, on the one path that
// produces every inline comment, rather than left to the prompt the model may ignore or
// to each caller to remember.
func PlaceableFindings(v Verdict, includeNits bool) []Finding {
	seen := make(map[string]bool)
	out := make([]Finding, 0)
	for _, entry := range reportedFindings(v) {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		f := findingFrom(fields)
		// Before the dedupe, not after: [Finding.dedupeKey] does not carry severity, so
		// a dropped nit that claimed the key would suppress a blocker reported about the
		// same line in the same words.
		if f.Severity == "nit" && !includeNits {
			continue
		}
		if !f.placeable() || seen[f.dedupeKey()] {
			continue
		}
		seen[f.dedupeKey()] = true
		out = append(out, f)
		if len(out) >= maxPlaceableFindings {
			break
		}
	}
	return out
}

// reportedFindings returns the raw entries a reply offered as line-tied
// findings, under either key it may have used.
//
// Both keys are read, always, rather than one falling back to the other. A session
// answers in the vocabulary its first prompt taught, and cannot be told otherwise. So a
// re-review shown the current schema can write an empty inline_comments beside a filled
// findings. A fallback that fires only when the key is absent does not fire there, and
// the run then places nothing and reports success.
func reportedFindings(v Verdict) []any {
	return append(listField(v.Structured, "inline_comments"),
		listField(v.Structured, "findings")...)
}

// findingFrom decodes one entry, taking either vocabulary's field names.
func findingFrom(fields map[string]any) Finding {
	return Finding{
		File:     firstNonEmpty(fields, "path", "file"),
		Line:     intField(fields, "line"),
		Side:     normalizeSide(stringField(fields, "side")),
		Severity: normalizeSeverity(stringField(fields, "severity")),
		Detail:   firstNonEmpty(fields, "body", "detail"),
	}
}

// placeable reports whether this finding can be posted against a line.
func (f Finding) placeable() bool {
	return postablePath(f.File) && f.Line > 0 && f.Detail != ""
}

// Bounds on the path a review comment may name. A path is model output and so unbounded,
// and these are the limits a filesystem git checks a tree out onto imposes anyway: 255
// bytes to a component, and a whole path a POSIX open can take.
const (
	maxCommentPath    = 4096
	maxCommentPathSeg = 255
)

// postablePath reports whether p is fit for the path field of a GitHub review comment.
//
// The rule is the shape of a repository-relative file path, not a set of permitted
// characters. This value goes into a JSON request body, where a quote, a dollar or a
// parenthesis is inert, so a character allowlist buys nothing at this sink and costs real
// trees: c++/, @types/, app/(marketing)/ and any path with a space are files a review has
// to be able to comment on. [isPlainRepoPath] is the check for the values this package
// interpolates into commands, which is a different sink under a different rule.
//
// What it refuses is what is not a path into this tree: absolute, a home reference, an
// empty or dot or parent segment, a control character, a leading dash an argv would read
// as an option, invalid UTF-8, or anything past the length a checkout could hold. Git
// tracks none of those, so nothing is lost by refusing them, and each is a way a path
// could name something other than a file in the pull request.
//
// It is not the only thing between a crafted path and the API. GitHub refuses a review
// comment whose path is not in the pull request's diff, and the repository and pull
// request number come from the orchestrator rather than from the reply. So a path that
// passes here still cannot place a comment outside the change under review.
func postablePath(p string) bool {
	if p == "" || len(p) > maxCommentPath || !utf8.ValidString(p) {
		return false
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || strings.HasPrefix(p, "-") {
		return false
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".", "..":
			return false
		}
		if len(seg) > maxCommentPathSeg {
			return false
		}
	}
	return true
}

// dedupeKey identifies a finding by what it says and where. A reply that wrote
// the same finding under both keys means it once, and posting it twice is a
// duplicate the author reads as noise.
func (f Finding) dedupeKey() string {
	return strings.Join([]string{f.File, f.Side, strconv.Itoa(f.Line), f.Detail}, "\x00")
}

// firstNonEmpty returns the first of keys that carries a non-empty string.
func firstNonEmpty(fields map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := strings.TrimSpace(stringField(fields, k)); s != "" {
			return s
		}
	}
	return ""
}

// Blockers returns the must-fix findings that name no single line.
//
// They exist because the alternative is losing them. A review that says the change needs
// a test, or that two functions now disagree, has no line to put that against. A contract
// with only line-tied findings drops it silently. The author reads a clean set of inline
// comments and never learns the review's most important objection.
func Blockers(v Verdict) []string { return bulletList(v.Structured, "blockers") }

// NonBlockers returns the same for observations that do not block.
func NonBlockers(v Verdict) []string { return bulletList(v.Structured, "non_blockers") }

// PreExisting returns problems the change did not introduce.
func PreExisting(v Verdict) []PreExistingIssue {
	raw := listField(v.Structured, "pre_existing_issues")
	out := make([]PreExistingIssue, 0, len(raw))
	for _, entry := range raw {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		body := strings.TrimSpace(stringField(fields, "body"))
		if body == "" {
			continue
		}
		severity := normalizeSeverity(stringField(fields, "severity"))
		if severity == "nit" {
			// The bucket admits blocker or suggestion only; a nit about code the
			// change did not touch is noise on someone else's work.
			severity = "suggestion"
		}
		out = append(out, PreExistingIssue{Severity: severity, Body: body})
	}
	return out
}

// bulletList reads a list of one-line strings, dropping empties.
func bulletList(m map[string]any, key string) []string {
	raw := listField(m, key)
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		if s := strings.TrimSpace(toString(entry)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// listField reads a JSON array, returning nil when the key is absent so a caller
// can tell "not written" from "written empty".
func listField(m map[string]any, key string) []any {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	return raw
}

// toString renders a bullet an agent may have written as a bare string or as an
// object with a body.
func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return stringField(t, "body")
	}
	return ""
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// intField reads a number that JSON decoding may have made a float64. It takes a string
// too, because an agent writing JSON by hand quotes numbers often enough to matter.
func intField(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n := 0
		for _, r := range v {
			if r < '0' || r > '9' {
				return 0
			}
			n = n*10 + int(r-'0')
		}
		return n
	default:
		return 0
	}
}

// distinctReported counts the findings a reply reported, once each.
//
// reportedFindings concatenates both schema keys without deduping, because its
// callers filter afterwards. An adopted session that writes one observation under
// both vocabularies would otherwise have it counted twice in the check's title.
func distinctReported(v Verdict, includeNits bool) int {
	seen := make(map[string]bool)
	for _, entry := range reportedFindings(v) {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		f := findingFrom(fields)
		// The same gate [PlaceableFindings] applies, and before the key for the same
		// reason. A nit this run will not post appears nowhere in this check: no inline
		// comment is written for it, and [checkSummary] leaves out every line-tied
		// finding by design. Counting it puts a number on the title that nothing
		// underneath accounts for.
		//
		// Only this check. [RenderComment] publishes the reply's closing block whole, so
		// the published comment still carries the entry -- the count and that block
		// disagree by the nits taken out here, and closing that would mean editing model
		// output rather than counting it.
		if f.Severity == "nit" && !includeNits {
			continue
		}
		seen[f.dedupeKey()] = true
	}
	return len(seen)
}
