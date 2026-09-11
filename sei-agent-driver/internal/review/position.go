package review

import (
	"fmt"
	"strings"
)

// position states what this driver recorded when the word the reply wrote would mislead.
//
// The recorded decision is derived from the findings, so it can part from the word: an
// approve beside a pre-existing blocker is recorded as comment and concludes neutral; an
// approve beside a blocker is recorded as request_changes. Prose that ends "Approving"
// over a check that recorded nothing says the opposite of what happened, and this is
// the one place on the pull request that explains the gap: what was recorded, why, and
// the finding that decided it. An accepted pre-existing blocker is named too, with the
// acceptance that cleared it, so the reader sees why it did not withhold.
//
// Empty when the recorded position is the word the reply wrote and nothing was accepted,
// which is almost every review, and on a verdict the scouts settled. Rendered into the
// comment's footer, where it survives truncation, and into the check summary, so the two
// readings of one review say the same thing.
//
// Every finding named is model text beside framing it must not be able to imitate, so it
// goes through the same door the check summary's bullets do.
func (v Verdict) position() string {
	if !v.HasVerdict() || v.SettledBy != "" {
		return ""
	}
	var parts []string
	if withheld := v.withheldPosition(); withheld != "" {
		parts = append(parts, withheld)
	}
	if accepted := v.acceptedPosition(); accepted != "" {
		parts = append(parts, accepted)
	}
	return strings.Join(parts, "\n\n")
}

// withheldPosition explains the recorded decision where it is not the word the reply
// wrote, in the words of the reason [Verdict.standing] recorded.
func (v Verdict) withheldPosition() string {
	s := v.standing()
	switch s.why {
	case byBlocker:
		return fmt.Sprintf("**Changes requested.** The review wrote `%s` but names "+
			"something blocking, and the block decides: %s", v.Said(), v.decidingBlocker())
	case byEmptyComment:
		return "**No position recorded.** The review wrote `comment` and nothing else, " +
			"which is the shape a failed read arrives in, so the check neither approves " +
			"nor objects."
	case byUnreadDiff:
		return fmt.Sprintf("**Approval withheld.** The review wrote `%s` but did not show "+
			"it read the diff (read: %d), so no approval is recorded.",
			v.Said(), intField(v.Structured, "read"))
	case byPreExisting:
		return fmt.Sprintf("**Approval withheld.** The review wrote `%s`. Nothing in this "+
			"change blocks it, but the review names a pre-existing blocker in the code it "+
			"touches, and no approval is recorded while one stands unaccepted: %s",
			v.Said(), v.preExistingBlockers())
	}
	return ""
}

// acceptedPosition names every pre-existing blocker an acceptance cleared, and the
// acceptance, so the authority for not withholding is on the page.
func (v Verdict) acceptedPosition() string {
	var named []string
	for _, issue := range PreExisting(v) {
		if issue.Severity != "suggestion" && issue.Accepted != "" {
			named = append(named, fmt.Sprintf("%s (accepted: %s)",
				positionText(issue.Body), positionText(issue.Accepted)))
		}
	}
	if len(named) == 0 {
		return ""
	}
	return "**Accepted pre-existing blocker** — listed under Accepted in the base " +
		"branch's review standards, so it is reported and does not withhold approval: " +
		joinNamed(named)
}

// preExistingBlockers names every pre-existing issue that withholds approval, which is
// every one not explicitly a suggestion and not accepted; see
// [Verdict.hasPreExistingBlocker].
func (v Verdict) preExistingBlockers() string {
	var named []string
	for _, issue := range PreExisting(v) {
		if issue.Severity != "suggestion" && issue.Accepted == "" {
			named = append(named, positionText(issue.Body))
		}
	}
	return joinNamed(named)
}

// decidingBlocker names what the failure rests on: the blockers bucket first, then any
// line-tied finding that calls itself a blocker.
func (v Verdict) decidingBlocker() string {
	var named []string
	for _, b := range Blockers(v) {
		named = append(named, positionText(b))
	}
	for _, entry := range reportedFindings(v) {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if f := findingFrom(fields); f.Severity == "blocker" && f.Detail != "" {
			named = append(named, positionText(f.Detail))
		}
	}
	return joinNamed(named)
}

// maxPositionNamed bounds how many findings a position names; the rest are counted.
const maxPositionNamed = 3

// maxPositionText bounds one named finding. Tighter than a check bullet: the notice
// leads the summary and rides in the comment's footer, and the finding is rendered in
// full elsewhere on the same page.
const maxPositionText = 400

// joinNamed renders the findings a position rests on, counting what it does not show.
func joinNamed(named []string) string {
	if len(named) == 0 {
		return "the reply names none."
	}
	shown := named
	if len(shown) > maxPositionNamed {
		shown = shown[:maxPositionNamed]
	}
	out := strings.Join(shown, "; ")
	if n := len(named) - len(shown); n > 0 {
		out += fmt.Sprintf("; and %d more", n)
	}
	return out + "."
}

// positionText renders one piece of model text inline beside this package's framing.
func positionText(s string) string {
	return defuseMarkup(clip(oneLine(s), maxPositionText))
}
