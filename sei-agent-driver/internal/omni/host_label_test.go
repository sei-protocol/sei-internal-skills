package omni

import "testing"

// TestRunKeyLabelMatchesTheDeployment pins the literal label the deployed driver writes.
//
// [RunKeyLabel] is the only thing adopt matches on and the only label create writes, so
// its value is persisted state shared with every session in flight. A change orphans
// them: the next dispatch matches nothing and opens a second session for the same pull
// request, and --close walks the listing, finds nothing and reclaims nothing while
// reporting success. The server applies no lifetime cap and runs no sweep, so those
// sandboxes hold cpu and memory with no handle left to free them.
//
// The literal is spelled out rather than read from the constant, which is the point: a
// test that read the constant would agree with whatever the constant became. The same
// reasoning rules out a comment -- a rename sweep rewrites the warning along with the
// value it guards.
//
// Updating this line is not the way to change the label. Either verify the deployment
// holds no sessions, or write a migration that reads both labels before it writes the
// new one.
func TestRunKeyLabelMatchesTheDeployment(t *testing.T) {
	t.Parallel()

	const deployed = "review.seinetwork.io/run-key"
	if RunKeyLabel != deployed {
		t.Errorf("RunKeyLabel = %q, want %q — every session the deployed driver has "+
			"already created carries the latter, and this build would match none of "+
			"them, leaking their sandboxes with no reclaim path",
			RunKeyLabel, deployed)
	}
}
