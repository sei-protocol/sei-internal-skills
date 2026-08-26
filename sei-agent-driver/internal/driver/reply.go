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

// secretPatterns match credential shapes carrying a distinctive prefix. The agent holds gh
// credentials in its sandbox, can quote anything it reads, and posts to public repositories.
//
// A backstop, not a boundary: a clean scan is not evidence that a reply carries none. An
// unprefixed secret, an encoded one and one split across lines all pass, and no pattern list
// closes them -- [TestScanSecretsDoesNotSeeWhatItDoesNotClaimTo] pins each.
var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"github token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`)},
	{"github pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	// ASIA is the STS key a pod using Pod Identity holds; AKIA is the long-lived one this
	// fleet should not have at all. Both are the id -- the secret half has no shape to match.
	{"aws access key id", regexp.MustCompile(`(AKIA|ASIA)[0-9A-Z]{16}`)},
	{"anthropic api key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	// The legacy form is a bare sk- and a fixed length, which cannot collide with sk-ant-:
	// the hyphen ends the run it needs.
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
// recognises none. "" is not a clean bill of health -- see [secretPatterns].
//
// literals are values this process holds, compared as substrings, so an encoded or split copy
// is missed too. Only the pattern's name is returned, never the match.
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
