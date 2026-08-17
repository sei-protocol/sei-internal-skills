package driver

import (
	"strings"
	"testing"
)

// TestScanSecretsNamesTheShapeWithoutQuotingIt covers each pattern and the
// process's own credentials, and checks the diagnostic never carries the match.
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
