package driver

import "testing"

// TestEveryExitNumberIsTheOneTheContractPromises pins the numbers themselves.
//
// The doc above these constants says no existing number moves to a different
// meaning, because a workflow pinned to an older ref reads them. Every other
// assertion in this suite compares against the symbol, so the whole set could be
// permuted with the suite green -- and the failure is silent in the direction that
// matters: ExitInternal at 0 exits a recovered panic as success, and CI reports the
// review as passed.
//
// A number added here is a new line. A number changed here is a breaking change to
// a caller this module cannot see, so this test failing is the contract working.
func TestEveryExitNumberIsTheOneTheContractPromises(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		got  int
		want int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitConfig", ExitConfig, 2},
		{"ExitTimeout", ExitTimeout, 3},
		{"ExitTurnFailed", ExitTurnFailed, 4},
		{"ExitNoVerdict", ExitNoVerdict, 5},
		{"ExitTransport", ExitTransport, 6},
		{"ExitCancelled", ExitCancelled, 7},
		{"ExitTeardownLeak", ExitTeardownLeak, 8},
		{"ExitInternal", ExitInternal, 9},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d: the numbers are a contract with a workflow "+
				"pinned to an older ref, so a number may be added but none may move",
				tc.name, tc.got, tc.want)
		}
	}
}

// TestNoTwoExitNumbersCollide keeps a number added later from landing on a meaning
// that is already taken, which the table above cannot catch on its own: a duplicate
// still matches its own row.
func TestNoTwoExitNumbersCollide(t *testing.T) {
	t.Parallel()

	seen := map[int]string{}
	for name, code := range map[string]int{
		"ExitOK": ExitOK, "ExitConfig": ExitConfig, "ExitTimeout": ExitTimeout,
		"ExitTurnFailed": ExitTurnFailed, "ExitNoVerdict": ExitNoVerdict,
		"ExitTransport": ExitTransport, "ExitCancelled": ExitCancelled,
		"ExitTeardownLeak": ExitTeardownLeak, "ExitInternal": ExitInternal,
	} {
		if prior, taken := seen[code]; taken {
			t.Errorf("%s and %s are both %d; a caller cannot tell them apart",
				prior, name, code)
		}
		seen[code] = name
	}
}
