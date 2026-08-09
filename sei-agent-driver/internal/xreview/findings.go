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

	// Severity is high, medium or low as the prompt defines them. Carried
	// verbatim rather than normalised: it is the agent's word, and rewriting it
	// would misreport what the review said.
	Severity string `json:"severity"`

	// Detail is what is wrong and why it matters.
	Detail string `json:"detail"`
}

// PlaceableFindings returns the findings a caller can post against a line.
//
// Only the placeable ones, because a review comment needs a path and a line and
// there is nowhere sensible to put one that has neither. What is dropped here is
// still in the prose the summary comment carries, so nothing is lost from the
// review — only from the inline placement.
func PlaceableFindings(v Verdict) []Finding {
	raw, _ := v.Structured["findings"].([]any)
	out := make([]Finding, 0, len(raw))
	for _, entry := range raw {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		f := Finding{
			File:     strings.TrimSpace(stringField(fields, "file")),
			Line:     intField(fields, "line"),
			Severity: strings.TrimSpace(stringField(fields, "severity")),
			Detail:   strings.TrimSpace(stringField(fields, "detail")),
		}
		if f.File == "" || f.Line <= 0 || f.Detail == "" {
			continue
		}
		out = append(out, f)
	}
	return out
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
