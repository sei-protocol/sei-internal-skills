package omni

import (
	"context"
	"testing"
	"time"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// TestBoundWalkSpendsAShareOfWhatIsLeft pins the three terms of the walk budget
// separately, because an end-to-end timing assertion cannot.
//
// The elapsed-time check in TestCloseReclaimsWhatItFoundWhenTheSearchRunsOut is
// satisfied by whichever term happens to be smallest, so the other two are slack:
// with RequestTimeout at 100ms the floor alone decided, and removing the halving or
// raising the multiplier to the value host.go's comment forbids both left the suite
// green. Each term needs a case where it is the one that binds.
func TestBoundWalkSpendsAShareOfWhatIsLeft(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		timeout   time.Duration
		remaining time.Duration
		want      time.Duration
		binds     string
	}{{
		name: "the multiplier binds when the caller has room to spare",
		// 2*2s = 4s, well under half of 60s and over the floor.
		timeout: 2 * time.Second, remaining: 60 * time.Second,
		want: 4 * time.Second, binds: "listingWalkBudget",
	}, {
		name: "the halving binds when the caller is close to its own deadline",
		// 2*20s = 40s would outlast the 30s left; half of it is 15s.
		timeout: 20 * time.Second, remaining: 30 * time.Second,
		want: 15 * time.Second, binds: "half of what remains",
	}, {
		name: "the floor binds when the share rounds to nothing",
		// A zero RequestTimeout would otherwise hand the walk an expired context.
		timeout: 0, remaining: 60 * time.Second,
		want: minListingWalkBudget, binds: "minListingWalkBudget",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &Host{cfg: driver.Config{RequestTimeout: tc.timeout}}
			parent, cancelParent := context.WithTimeout(context.Background(), tc.remaining)
			defer cancelParent()

			walk, cancel := h.boundWalk(parent)
			defer cancel()

			deadline, ok := walk.Deadline()
			if !ok {
				t.Fatal("the walk got no deadline, so nothing bounds it")
			}
			// Scheduling noise moves this by microseconds; the terms are seconds apart.
			if got := time.Until(deadline); got < tc.want-time.Second || got > tc.want+time.Second {
				t.Errorf("walk budget = %v, want about %v (%s should bind)",
					got.Round(time.Millisecond), tc.want, tc.binds)
			}
		})
	}
}

// TestBoundWalkNeverSpendsTheWholeCallersBudget is the invariant the three cases
// above share, and the one the fix exists for: a search must leave the caller room
// for what the search was serving. On Close that is the deletes.
func TestBoundWalkNeverSpendsTheWholeCallersBudget(t *testing.T) {
	t.Parallel()

	// Ratios around and past the point where the multiplier would outgrow the
	// caller, so a walk that took its multiplier unconditionally is caught.
	for _, remaining := range []time.Duration{
		2 * time.Second, 10 * time.Second, 30 * time.Second, 120 * time.Second,
	} {
		h := &Host{cfg: driver.Config{RequestTimeout: 30 * time.Second}}
		parent, cancelParent := context.WithTimeout(context.Background(), remaining)
		walk, cancel := h.boundWalk(parent)

		deadline, ok := walk.Deadline()
		if !ok {
			t.Fatalf("remaining %v: the walk got no deadline", remaining)
		}
		// Strictly less: equal means the walk may consume the caller entirely.
		if budget := time.Until(deadline); budget >= remaining {
			t.Errorf("remaining %v: walk budget %v leaves the caller nothing",
				remaining, budget.Round(time.Millisecond))
		}
		cancel()
		cancelParent()
	}
}
