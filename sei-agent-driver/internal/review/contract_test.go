package review

import (
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// The driver's interfaces are structural, and this package deliberately imports nothing
// from it: a review is a set of methods, not a dependency. The cost is that neither
// package stops compiling when the two drift. The wiring that would catch it lives in
// the command.
//
// These assertions are that check, brought back to the side that has to satisfy it.
// A test file keeps the production package decoupled, so a renamed or re-signed
// method fails here rather than in a later slice.
var (
	_ driver.Workload   = Review{}
	_ driver.Workload   = Scout{}
	_ driver.AgentNamer = Scout{}
)

// TestOnlyAScoutNamesItsOwnAgent pins which of the two opts out of the run's
// configured agent.
//
// [driver.AgentNamer] is optional, and the driver reads it by asking. So a review that
// gained an AgentName method would silently move every review off the agent the run
// configured. And a scout that lost one would silently move every scout onto it. Neither
// breaks a build, and both change which bundle answers.
func TestOnlyAScoutNamesItsOwnAgent(t *testing.T) {
	if _, ok := any(Review{}).(driver.AgentNamer); ok {
		t.Error("Review names an agent; a review runs on the agent the run configured")
	}
	if _, ok := any(Scout{}).(driver.AgentNamer); !ok {
		t.Error("Scout names no agent; a scout has to run on one of its own")
	}
}
