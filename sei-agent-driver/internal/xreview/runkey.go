package xreview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// runKeyLength is how much of the digest the key uses. 24 hex characters is 96 bits.
// That is far past collision range for the number of reviews a deployment will ever
// run, and short enough to read in a log line.
const runKeyLength = 24

// RunKey derives the key identifying one unit of review work: a pull request.
//
// The repository and number are joined with NUL rather than a printable separator, so
// no combination of them can spell another. A repository literally named with a colon
// or a hash could otherwise collide with a different pull request.
//
// Deliberately NOT keyed on the trigger. One session per pull request is the point. The
// agent keeps the conversation from its previous review and can say what changed since,
// rather than meeting the diff fresh every time. A key that varied with the trigger
// would open a session per comment and throw that away.
//
// The cost of this choice is that the agent remembers a tree that has since moved. So
// the prompt for an adopted session has to say so explicitly, rather than assume the
// agent will re-read. See [AdoptedPrompt].
func RunKey(repo string, pr int) string {
	sum := sha256.Sum256([]byte(repo + "\x00" + strconv.Itoa(pr)))
	return hex.EncodeToString(sum[:])[:runKeyLength]
}

// TriggerID identifies this dispatch, for log correlation only.
//
// Not part of [RunKey], which keys on the pull request so a session survives
// across invocations. This answers only "which comment or run produced this log
// line".
//
// The explicit value is what a caller passes for a comment-triggered review: the
// comment id. Absent that, GitHub's run id and attempt number distinguish one
// dispatch from the next. That distinction is a log label: [RunKey] is the pull
// request, so every dispatch adopts the same session whatever the trigger was.
//
// The final fallback is marked manual and is deterministic. That is only a log label
// now. Nothing is skipped as a duplicate, because [RunKey] is the pull request, and
// every invocation adopts the same session and reviews the current tree.
func TriggerID(explicit, runID, attempt string, repo string, pr int) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if runID = strings.TrimSpace(runID); runID != "" {
		if attempt = strings.TrimSpace(attempt); attempt != "" {
			return "run:" + runID + "/" + attempt
		}
		return "run:" + runID
	}
	return fmt.Sprintf("manual:%s#%d", repo, pr)
}
