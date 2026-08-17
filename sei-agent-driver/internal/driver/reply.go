package driver

import (
	"regexp"
	"strings"
)

// Reply is the attributed answer to one turn.
type Reply struct {
	// Text is the message's published text.
	Text string

	// ItemID is the conversation item it came from, recorded so a published
	// comment can name its own provenance.
	ItemID string

	// TurnID is the response this reply answers, carried so a caller publishing
	// it can name its own provenance.
	TurnID string

	// Reason renders why there is no usable reply, for the operator who has to
	// act on it. Empty when Text carries one.
	Reason string
}

// secretPatterns match shapes that must never reach a public pull request. The
// agent holds gh credentials inside its sandbox and can quote anything it reads,
// and the repositories this posts to are public.
var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"github token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`)},
	{"github pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	// AKIA is a long-lived IAM key; ASIA is the STS temporary one, which is what a
	// pod using Pod Identity or IRSA actually holds -- so ASIA is the shape this
	// fleet can produce and AKIA is the one it should not have at all.
	{"aws access key id", regexp.MustCompile(`(AKIA|ASIA)[0-9A-Z]{16}`)},
	{"anthropic api key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{"age secret key", regexp.MustCompile(`AGE-SECRET-KEY-1[0-9A-Z]{20,}`)},
	{"slack token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)},
	{"pem private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"json web token", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}`)},
}

// ScanSecrets names the first credential shape found in text, or "" when there is
// none. The literals are values this process holds and must never republish.
//
// Only the pattern's name is returned, never what matched: a diagnostic that
// quoted the match would leak the thing it exists to protect.
func ScanSecrets(text string, literals ...string) string {
	for _, literal := range literals {
		// A short or empty literal would match everything. An unset credential is
		// not a reason to refuse every review.
		if len(literal) >= 8 && strings.Contains(text, literal) {
			return "a credential this process holds"
		}
	}
	for _, pattern := range secretPatterns {
		if pattern.re.MatchString(text) {
			return pattern.name
		}
	}
	return ""
}
