package review

import "testing"

// TestRunKeyIdentifiesThePullRequestNotTheTrigger is the session-continuity
// contract. One session per pull request is the point: the agent keeps the
// conversation from its previous review and can say what changed. If the key
// varied with the trigger, every comment would start a fresh session and that
// context would be thrown away.
func TestRunKeyIdentifiesThePullRequestNotTheTrigger(t *testing.T) {
	t.Parallel()

	const repo, pr = "sei-protocol/sandbox", 42
	base := RunKey(repo, pr)

	if len(base) != runKeyLength {
		t.Errorf("len = %d, want %d", len(base), runKeyLength)
	}
	if got := RunKey(repo, pr); got != base {
		t.Errorf("not deterministic: %q then %q", base, got)
	}

	// Different pull request, or different repository, must be a different
	// session. Nothing else may change it.
	for _, tc := range []struct {
		name string
		repo string
		pr   int
	}{
		{"another pull request", repo, 43},
		{"another repository", "sei-protocol/other", pr},
		{"same names, different owner", "someone-else/sandbox", pr},
	} {
		if got := RunKey(tc.repo, tc.pr); got == base {
			t.Errorf("%s: RunKey(%q, %d) == the base key %q, want distinct",
				tc.name, tc.repo, tc.pr, got)
		}
	}
}

// TestRunKeyNULJoinPreventsConcatenationCollisions keeps the separator honest: a
// repository name containing the separator must not be able to spell a different
// (repo, pr) pair.
func TestRunKeyNULJoinPreventsConcatenationCollisions(t *testing.T) {
	t.Parallel()

	// Under a naive "repo + pr" concatenation both of these render "a1" + "2".
	if RunKey("a1", 2) == RunKey("a", 12) {
		t.Error(`RunKey("a1",2) == RunKey("a",12): the parts are not delimited`)
	}
	if RunKey("a", 1) == RunKey("a\x001", 0) {
		t.Error("a repo containing the delimiter collided with another pair")
	}
}

func TestTriggerIDPrecedence(t *testing.T) {
	t.Parallel()

	const repo, pr = "sei-protocol/sandbox", 42

	tests := []struct {
		name     string
		explicit string
		runID    string
		attempt  string
		want     string
	}{
		{"explicit wins over everything", "comment-999", "run-1", "1", "comment-999"},
		{"whitespace-only explicit falls through to run+attempt", "   ", "run-1", "1", "run:run-1/1"},
		{"run and attempt, no explicit", "", "run-1", "1", "run:run-1/1"},
		{"run alone, attempt blank", "", "run-1", "", "run:run-1"},
		{"run alone, attempt whitespace-only", "", "run-1", "  ", "run:run-1"},
		{"nothing at all falls back to manual", "", "", "", "manual:sei-protocol/sandbox#42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := TriggerID(tc.explicit, tc.runID, tc.attempt, repo, pr); got != tc.want {
				t.Errorf("TriggerID(%q, %q, %q, ...) = %q, want %q",
					tc.explicit, tc.runID, tc.attempt, got, tc.want)
			}
		})
	}

	t.Run("a re-run attempt yields a different id from the first attempt", func(t *testing.T) {
		t.Parallel()
		first := TriggerID("", "run-1", "1", repo, pr)
		rerun := TriggerID("", "run-1", "2", repo, pr)
		if first == rerun {
			t.Errorf("attempt 1 and attempt 2 both gave %q, want a re-run to get its own id", first)
		}
	})

	t.Run("manual fallback is deterministic for the same pull request", func(t *testing.T) {
		t.Parallel()
		first := TriggerID("", "", "", repo, pr)
		second := TriggerID("", "", "", repo, pr)
		if first != second {
			t.Errorf("manual fallback gave %q then %q, want the same value both times", first, second)
		}
	})

	t.Run("manual fallback differs across pull requests", func(t *testing.T) {
		t.Parallel()
		if a, b := TriggerID("", "", "", repo, 1), TriggerID("", "", "", repo, 2); a == b {
			t.Errorf("manual fallback for pr 1 and pr 2 both gave %q, want them to differ", a)
		}
	})
}
