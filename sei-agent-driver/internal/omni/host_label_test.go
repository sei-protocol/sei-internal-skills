package omni

import "testing"

// TestRunKeyLabelIsNotSwept pins the literal label a deployed driver has already
// written on live sessions.
//
// [RunKeyLabel] is the only thing adopt matches on and the only label create writes, so
// its value is persisted state shared with every session in flight. A change orphans
// them: the next dispatch matches nothing and opens a second session for the same pull
// request, and --close walks the listing, finds nothing and reclaims nothing. The server
// applies no lifetime cap and runs no sweep, so those sandboxes hold cpu and memory with
// no handle left to free them.
//
// The literal is spelled out here rather than referenced, which is the whole point: a
// test that read the constant would agree with any value the constant took. A comment
// used to carry this invariant and did not survive contact with a rename sweep, which is
// why it is a test now.
//
// Changing this requires a migration that reads both labels before it writes the new
// one, not an edit to this line.
func TestRunKeyLabelIsNotSwept(t *testing.T) {
	t.Parallel()

	const deployed = "xreview.seinetwork.io/run-key"
	if RunKeyLabel != deployed {
		t.Errorf("RunKeyLabel = %q, want %q — every session the deployed driver has "+
			"already created carries the latter, and this build would match none of "+
			"them, leaking their sandboxes with no reclaim path",
			RunKeyLabel, deployed)
	}
}
