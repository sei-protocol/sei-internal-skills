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
// must contain to match. Bullets below [minAcceptanceWords] are dropped, and a fenced
// code block is skipped, so an example of the format is not a live entry. Nothing
// else in the file is read.
//
//	## Accepted pre-existing conditions
//	- pinned to the uci feature branch — deliberate until PLT-1300 lands
func ParseAccepted(text string) []string {
	var out []string
	inSection, inFence := false, false
	for _, line := range strings.Split(normalizeLineEndings(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if title, ok := headingTitle(trimmed); ok {
			inSection = opensAccepted(title)
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

// opensAccepted reports whether a heading's first word is [AcceptedHeading]: "Accepted
// pre-existing conditions" opens the section, "Acceptedness" does not.
func opensAccepted(title string) bool {
	fields := strings.Fields(strings.ToLower(title))
	return len(fields) > 0 && strings.TrimRight(fields[0], ":.,") == AcceptedHeading
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
// loosely enough that backticks, case and run-on whitespace do not decide it, and
// carrying the finding rather than appearing in it: a phrase the body negates, or one
// the body goes on from with a second claim, is not a match. Both fail closed: the
// blocker withholds, the notice names it, and the maintainer rewrites the entry or the
// review's wording settles down. See [negated] and [compounded].
func acceptanceFor(body string, accepted []string) string {
	haystack := matchable(body)
	for _, entry := range accepted {
		phrase := matchable(acceptancePhrase(entry))
		if phrase == "" {
			continue
		}
		before, after, found := strings.Cut(haystack, phrase)
		if found && !negated(before) && !compounded(after) {
			return entry
		}
	}
	return ""
}

// negations are the words that, within [negationReach] words ahead of the phrase, turn
// it into a claim about its absence.
var negations = map[string]bool{
	"no": true, "not": true, "never": true, "longer": true, "without": true,
	"stopped": true, "removed": true, "dropped": true, "neither": true, "nor": true,
}

// negationReach is how many words ahead of the phrase a negation still governs it.
const negationReach = 3

// negated reports whether the text ahead of a matched phrase negates it.
func negated(before string) bool {
	words := strings.Fields(before)
	if len(words) > negationReach {
		words = words[len(words)-negationReach:]
	}
	for _, w := range words {
		if negations[strings.Trim(w, ".,;:()")] {
			return true
		}
	}
	return false
}

// conjunctions open a second claim after the phrase: "pins uci to the feature branch
// and leaks the token" accepts the pin and says nothing about the token.
var conjunctions = []string{" and ", " but ", " also ", " as well as ", " plus ", "; "}

// compounded reports whether the text after a matched phrase carries another claim.
func compounded(after string) bool {
	for _, c := range conjunctions {
		if strings.Contains(" "+after, c) {
			return true
		}
	}
	return false
}

// matchable folds text for [acceptanceFor]: lowercased, backticks and quotes removed,
// whitespace collapsed to single spaces.
func matchable(s string) string {
	s = strings.NewReplacer("`", "", "\"", "", "'", "", "“", "", "”", "", "‘", "", "’", "").
		Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}
