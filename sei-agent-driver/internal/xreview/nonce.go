package xreview

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// A nonce is what separates the agent's own closing block from one it copied.
//
// The agent reviews a pull request whose diff someone else wrote, so a file under
// review can contain a fenced block naming a decision. Position cannot tell the two
// apart, and counting them only refuses while the agent's own block is present and
// recognised -- so an agent that emits no block, or spells its decision off-schema,
// leaves a planted block as the only candidate and it is taken as the verdict.
//
// Nothing inside a block is unforgeable on its own. This is: the prompt carries a
// value, the closing block must echo it, and the pull request's author never sees
// the prompt. A block without it is not the agent's.

// nonceField is the key the closing block echoes it under, in both contracts.
const nonceField = "run"

// Nonce derives the value for one unit of work.
//
// Keyed on a secret the calling workflow holds, so it cannot be recomputed from the
// repository and pull request number, which are public. Stable for a unit of work
// rather than per dispatch, because a session outlives the run that opened it: an
// adopted session is re-prompted with this same value, and one that changed per
// dispatch would refuse the reply it just asked for.
//
// An empty secret yields an empty nonce, which leaves both parsers on their
// unauthenticated rules. That is the pre-existing behaviour and it is what a caller
// that has not been wired yet gets -- see [Request.Nonce].
func Nonce(secret, repo string, pr int) string {
	if strings.TrimSpace(secret) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(RunKey(repo, pr)))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// carriesNonce reports whether a decoded block echoes the expected value.
//
// Constant-time, because the comparison is against attacker-supplied bytes and a
// timing signal would let the value be recovered a byte at a time.
func carriesNonce(fields map[string]any, want string) bool {
	if want == "" {
		return true
	}
	got, _ := fields[nonceField].(string)
	return hmac.Equal([]byte(strings.TrimSpace(got)), []byte(want))
}
