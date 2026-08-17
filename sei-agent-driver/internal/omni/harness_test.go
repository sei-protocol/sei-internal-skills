package omni

import (
	"log/slog"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// newTestDriver wires a driver onto a real host.
//
// These tests drive the whole path against a fake server — the SDK, this package,
// and the driver's own classification — because that is what they exist to pin:
// how this server's event shapes become an exit code. The driver's own tests use a
// fake host instead, and need no server at all.
func newTestDriver(cfg driver.Config, policy driver.Policy, log *slog.Logger) *driver.Driver {
	return driver.New(cfg, New(cfg, policy, log), log)
}
