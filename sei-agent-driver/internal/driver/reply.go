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

// secretPatterns match credential shapes that carry a distinctive prefix. The agent
// holds gh credentials inside its sandbox and can quote anything it reads, and the
// repositories this posts to are public.
//
// This is a backstop, not a boundary, and the difference matters because a clean scan
// is not evidence that a reply carries no credential. Three classes are outside it,
// and no list of patterns closes them:
//
//   - A secret with no distinctive shape. An AWS secret access key is forty characters
//     of base64 with no prefix, and most database passwords and HMAC keys are the same.
//     The aws entry below matches the access key id, which is the public half of the
//     pair.
//   - An encoded secret. Base64, hex, percent-encoding and JSON string escapes each
//     defeat every pattern here, and defeat the literal comparison too, since that is
//     a substring test against the raw value.
//   - A split secret. A token broken across lines, or assembled from pieces, matches
//     nothing.
//
// The controls this actually rests on are elsewhere: the sandbox holds the credential
// rather than this process, the prompt tells the agent the pull request is data and not
// instructions, and the trigger is restricted to a repository's own developers. This
// catches the accident those leave behind. It does not stop an agent that is trying,
// and it is not a review of what may be published.
var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"github token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`)},
	{"github pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	// AKIA is a long-lived IAM key; ASIA is the STS temporary one, which is what a
	// pod using Pod Identity or IRSA actually holds -- so ASIA is the shape this
	// fleet can produce and AKIA is the one it should not have at all. Both are the
	// id. The secret half of the pair has no shape to match.
	{"aws access key id", regexp.MustCompile(`(AKIA|ASIA)[0-9A-Z]{16}`)},
	{"anthropic api key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	// Project and service-account keys carry their own prefix; the legacy form is a
	// bare sk- and a fixed length. None of the three collides with sk-ant-, whose
	// hyphen ends the run the legacy pattern needs.
	{"openai api key", regexp.MustCompile(`sk-(proj|svcacct)-[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9]{48}`)},
	{"google api key", regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},
	{"gitlab token", regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`)},
	{"npm token", regexp.MustCompile(`npm_[A-Za-z0-9]{36}`)},
	{"pypi token", regexp.MustCompile(`pypi-[A-Za-z0-9_-]{50,}`)},
	{"stripe secret key", regexp.MustCompile(`(sk|rk)_live_[A-Za-z0-9]{20,}`)},
	{"hugging face token", regexp.MustCompile(`hf_[A-Za-z0-9]{30,}`)},
	{"age secret key", regexp.MustCompile(`AGE-SECRET-KEY-1[0-9A-Z]{20,}`)},
	{"slack token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)},
	{"slack webhook", regexp.MustCompile(`https://hooks\.slack\.com/services/[A-Za-z0-9/+_-]{20,}`)},
	// Covers the OPENSSH, RSA, EC and PGP spellings, since each renders as
	// "-----BEGIN <something> PRIVATE KEY-----".
	{"pem private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"json web token", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}`)},
}

// ScanSecrets names the first credential shape it recognises in text, or "" when it
// recognises none.
//
// "" is not a clean bill of health. [secretPatterns] states which classes of secret
// this cannot see, and a caller treating an empty result as "no credential present"
// is relying on something this does not provide.
//
// literals are values this process holds, compared as substrings, so an encoded or
// split copy of one is missed the same way. Only the pattern's name is returned, never
// what matched: a diagnostic that quoted the match would leak the thing it exists to
// protect.
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
