package xreview

import "strings"

// Finding is one observation a review made about a specific line.
//
// It is the structured half of a review, and the only half that can be posted
// against the code it is about. The prose is what a person reads; this is what a
// machine can place.
type Finding struct {
	// File is the path as the diff names it. A finding naming no file cannot be
	// placed and is dropped rather than posted somewhere arbitrary.
	File string `json:"file"`

	// Line is the line within that file. Zero means the agent gave none, which is
	// placeable only at the top of the file, so such a finding is dropped too.
	Line int `json:"line"`

	// Side is which side of the diff Line counts against: RIGHT for the new file,
	// LEFT for the old one. It is what makes a finding about a REMOVED line
	// placeable at all — without it such a finding either lands on an unrelated
	// new line or is dropped.
	//
	// Empty means RIGHT, which is what the overwhelming majority of findings are
	// and what every finding written before this field existed meant.
	Side string `json:"side"`

	// Severity is blocker, suggestion or nit. Carried verbatim rather than
	// normalised once recognised: it is the agent's word, and rewriting it would
	// misreport what the review said. The older high/medium/low vocabulary maps
	// onto it, because a session that has reviewed before still speaks it.
	Severity string `json:"severity"`

	// Detail is what is wrong and why it matters.
	Detail string `json:"detail"`
}

// PreExistingIssue is a problem the change did not introduce.
//
// Kept apart from [Finding] because the two answer different questions. A
// finding is about this pull request; a pre-existing issue is about the code it
// landed in, and presenting one as the other tells an author they broke
// something they did not touch.
type PreExistingIssue struct {
	// Severity is blocker or suggestion. Blocker is reserved for something
	// critical on its own — an exploitable hole, data loss, a likely outage —
	// rather than for anything the change happened to sit near.
	Severity string `json:"severity"`

	// Body identifies the location and the impact in one line.
	Body string `json:"body"`
}

// severityAliases maps the vocabulary a session learned before this contract
// onto the current one. An adopted session carries its first prompt in context,
// so it keeps answering in the words that prompt used.
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

// PlaceableFindings returns the findings a caller can post against a line.
//
// Only the placeable ones, because a review comment needs a path and a line and
// there is nowhere sensible to put one that has neither. What is dropped here is
// still in the prose the summary comment carries, so nothing is lost from the
// review — only from the inline placement.
func PlaceableFindings(v Verdict) []Finding {
	raw := listField(v.Structured, "inline_comments")
	if raw == nil {
		// A session that reviewed under the previous contract answers in its
		// vocabulary, and it cannot be told otherwise once its first prompt is in
		// context. Reading both is what stops a schema change from silently
		// placing nothing on every pull request already reviewed.
		raw = listField(v.Structured, "findings")
	}
	out := make([]Finding, 0, len(raw))
	for _, entry := range raw {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		file := strings.TrimSpace(stringField(fields, "path"))
		if file == "" {
			file = strings.TrimSpace(stringField(fields, "file"))
		}
		body := strings.TrimSpace(stringField(fields, "body"))
		if body == "" {
			body = strings.TrimSpace(stringField(fields, "detail"))
		}
		f := Finding{
			File:     file,
			Line:     intField(fields, "line"),
			Side:     normalizeSide(stringField(fields, "side")),
			Severity: normalizeSeverity(stringField(fields, "severity")),
			Detail:   body,
		}
		if f.File == "" || f.Line <= 0 || f.Detail == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Blockers returns the must-fix findings that name no single line.
//
// They exist because the alternative is losing them. A review that says the
// change needs a test, or that two functions now disagree, has nowhere to put
// that against a line, and a contract with only line-tied findings drops it
// silently — the author reads a clean set of inline comments and never learns
// the review's most important objection.
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

// intField reads a number that JSON decoding may have made a float64, and takes
// a string too because an agent writing JSON by hand quotes numbers often enough
// to matter.
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
