package driver

import (
	"strings"
	"testing"
)

// TestScanSecretsNamesTheShapeWithoutQuotingIt covers each pattern and the
// process's own credentials, and checks the diagnostic never carries the match.
func TestScanSecretsCoversTheShapesThisFleetCanProduce(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, text, want string }{
		{"sts temporary key", "ASIA" + strings.Repeat("Q", 16), "aws access key id"},
		{"long-lived iam key", "AKIA" + strings.Repeat("Q", 16), "aws access key id"},
		{"anthropic key", "sk-ant-" + strings.Repeat("x", 24), "anthropic api key"},
		{"age secret key", "AGE-SECRET-KEY-1" + strings.Repeat("Q", 24), "age secret key"},
		{"slack token", "xoxb-" + strings.Repeat("1", 20), "slack token"},
		{"slack webhook", "https://hooks.slack.com/services/" + strings.Repeat("A", 24), "slack webhook"},
		{"openai project key", "sk-proj-" + strings.Repeat("x", 24), "openai api key"},
		{"openai service account key", "sk-svcacct-" + strings.Repeat("x", 24), "openai api key"},
		{"openai legacy key", "sk-" + strings.Repeat("x", 48), "openai api key"},
		{"google api key", "AIza" + strings.Repeat("x", 35), "google api key"},
		{"gitlab token", "glpat-" + strings.Repeat("x", 20), "gitlab token"},
		{"npm token", "npm_" + strings.Repeat("x", 36), "npm token"},
		{"pypi token", "pypi-" + strings.Repeat("x", 50), "pypi token"},
		{"stripe secret key", "sk_live_" + strings.Repeat("x", 24), "stripe secret key"},
		{"stripe restricted key", "rk_live_" + strings.Repeat("x", 24), "stripe secret key"},
		{"hugging face token", "hf_" + strings.Repeat("x", 34), "hugging face token"},
		{"openssh private key", "-----BEGIN OPENSSH PRIVATE KEY-----", "pem private key"},
		{"ordinary review prose", "The diff looks fine to me.", ""},
		// An anthropic key must not be reported as the openai legacy shape, whose
		// pattern is a bare sk- and a run of 48. The hyphen after ant ends that run.
		{"anthropic not misreported", "sk-ant-api03-" + strings.Repeat("x", 48), "anthropic api key"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ScanSecrets(c.text); got != c.want {
				t.Errorf("ScanSecrets = %q, want %q", got, c.want)
			}
		})
	}
}

func TestScanSecretsNamesTheShapeWithoutQuotingIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want bool
	}{
		{"a clean review", "Two findings, both in a.go. No credentials here.", false},
		{"a github token", "I ran gh with ghp_" + strings.Repeat("A", 36), true},
		{"a github pat", "github_pat_" + strings.Repeat("B", 40), true},
		{"an aws access key id", "AKIAIOSFODNN7EXAMPLE", true},
		{"a pem private key", "-----BEGIN RSA PRIVATE KEY-----", true},
		{"a json web token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0", true},
		{"a token-shaped word that is too short", "ghp_short", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ScanSecrets(tc.text)
			if (got != "") != tc.want {
				t.Errorf("ScanSecrets = %q, want a match: %v", got, tc.want)
			}
			if got != "" && strings.Contains(tc.text, got) {
				t.Errorf("ScanSecrets returned %q, which appears in the scanned text: the "+
					"diagnostic must name the shape, never the match", got)
			}
		})
	}
}

// TestScanSecretsCatchesTheProcessOwnCredentials checks the literal comparison,
// and that an unset or trivially short credential does not make every review
// refuse.
func TestScanSecretsCatchesTheProcessOwnCredentials(t *testing.T) {
	t.Parallel()

	const secret = "s3cret-machine-client-value"
	if got := ScanSecrets("the agent echoed "+secret+" into its review", secret); got == "" {
		t.Error("ScanSecrets missed a credential this process holds")
	}
	if got := ScanSecrets("an ordinary review", "", "abc"); got != "" {
		t.Errorf("ScanSecrets = %q on an ordinary review; an unset or short credential must "+
			"not match everything", got)
	}
}

// TestScanSecretsDoesNotSeeWhatItDoesNotClaimTo pins the limits [secretPatterns]
// states, so they are checked rather than only described.
//
// Every case here returns "" today. That is the documented behaviour and not a bug to
// be fixed quietly: the point is that a caller cannot read "" as "no credential
// present". If a later change gives one of these a pattern, this test should fail, and
// the fix is to move that case into the coverage table above and reword the limit.
func TestScanSecretsDoesNotSeeWhatItDoesNotClaimTo(t *testing.T) {
	t.Parallel()

	// The token this process holds, in the forms the literal comparison misses. It is
	// a substring test against the raw value, so any re-encoding defeats it.
	const token = "ghp_" + "0123456789abcdefghijklmnopqrstuvwxyz"

	for _, c := range []struct{ name, text string }{
		// No prefix to match. The pair's id is caught; this half is the secret.
		{"aws secret access key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"},
		{"a database password", "hunter2-correct-horse-battery-staple-9271"},
		// Encoded. base64 of the token above, which the literals check cannot see.
		{"base64 of a held token", "Z2hwXzAxMjM0NTY3ODlhYmNkZWZnaGlqa2xtbm9wcXJzdHV2d3h5eg=="},
		{"hex of a held token", "6768705f30313233343536373839616263646566"},
		{"percent-encoded token", "ghp%5F0123456789abcdefghijklmnopqrstuvwxyz"},
		// Split. Neither half is long enough for the pattern, and the whole never
		// appears contiguously.
		{"token split across lines", "ghp_0123456789abcde\nfghijklmnopqrstuvwxyz"},
		{"token assembled from pieces", `"ghp_0123456789" + "abcdefghijklmnopqrstuvwxyz"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ScanSecrets(c.text, token); got != "" {
				t.Errorf("ScanSecrets = %q; this case is documented as outside the scan, so "+
					"a match means the limit in secretPatterns is now wrong: move the case "+
					"into the coverage table and reword the limit", got)
			}
		})
	}
}
