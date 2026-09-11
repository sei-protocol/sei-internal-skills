package review

import (
	"strings"
)

// AcceptedHeading is the word a heading in the standards file opens with to introduce the
// accepted pre-existing conditions. Anything after it is the repository's own title.
const AcceptedHeading = "accepted"

// minAcceptanceWords is the fewest words an acceptance may carry and still match.
//
// An acceptance names one specific condition, and a one-word or two-word entry -- "pin",
// "the pin", "blocker" -- names a category. The floor is what keeps the list from
// becoming a blanket: an entry under it is kept out of the list, not widened.
const minAcceptanceWords = 3

// maxAcceptances bounds how many entries one file can contribute.
const maxAcceptances = 100

// ParseAccepted reads the accepted pre-existing conditions out of a repository's
// standards file, REVIEW.md by default.
//
// The caller reads that file from the pull request's BASE branch and never from its
// head. A pull request that could write its own acceptance would be handing itself the
// approval its blocker withholds. The driver cannot see the repository, so the caller
// carries that guarantee and this function parses what it is handed.
//
// The shape is one markdown section: a heading whose text opens with "Accepted", then
// bullets, one condition per bullet, until the next heading. A bullet may carry a
// rationale after an em dash or a double hyphen; the text before it is what a finding
// must contain to match. Bullets below [minAcceptanceWords] are dropped. Nothing else in
// the file is read.
//
//	## Accepted pre-existing conditions
//	- pinned to the uci feature branch — deliberate until PLT-1300 lands
func ParseAccepted(text string) []string {
	var out []string
	inSection := false
	for _, line := range strings.Split(normalizeLineEndings(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if title, ok := headingTitle(trimmed); ok {
			inSection = strings.HasPrefix(strings.ToLower(title), AcceptedHeading)
			continue
		}
		if !inSection || trimmed == "" || !bulletMarker(trimmed) {
			continue
		}
		entry := strings.TrimSpace(trimmed[1:])
		if len(strings.Fields(acceptancePhrase(entry))) < minAcceptanceWords {
			continue
		}
		out = append(out, entry)
		if len(out) >= maxAcceptances {
			break
		}
	}
	return out
}

// headingTitle returns the text of an ATX heading, and false when the line is not one.
func headingTitle(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	level := leadingRun(line)
	if level > 6 || len(line) > level && line[level] != ' ' && line[level] != '\t' {
		return "", false
	}
	return strings.TrimSpace(line[level:]), true
}

// acceptancePhrase is the part of an acceptance a finding must contain: the bullet up to
// its rationale, if it wrote one.
func acceptancePhrase(entry string) string {
	for _, sep := range []string{" — ", " -- ", " – "} {
		if before, _, found := strings.Cut(entry, sep); found {
			return before
		}
	}
	return entry
}

// acceptanceFor returns the acceptance a pre-existing issue matches, and "" for none.
//
// A match is the acceptance's phrase appearing inside the finding's body, compared
// loosely enough that backticks, case and run-on whitespace do not decide it. Loose in
// form and narrow in scope: the phrase has to be there, whole.
func acceptanceFor(body string, accepted []string) string {
	haystack := matchable(body)
	for _, entry := range accepted {
		if phrase := matchable(acceptancePhrase(entry)); phrase != "" &&
			strings.Contains(haystack, phrase) {
			return entry
		}
	}
	return ""
}

// matchable folds text for [acceptanceFor]: lowercased, backticks and quotes removed,
// whitespace collapsed to single spaces.
func matchable(s string) string {
	s = strings.NewReplacer("`", "", "\"", "", "'", "", "“", "", "”", "", "‘", "", "’", "").
		Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}
