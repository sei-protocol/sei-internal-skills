package omni

import (
	"errors"
	"log/slog"
	"testing"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// TestARefusedBaseURLIsAConfigFaultNotATransportOne pins which half of the
// taxonomy a rejected base URL lands in, because the two differ in what the caller
// does next: a config fault needs a person, a transport fault needs a retry.
//
// The plain-http ClusterIP is the case that matters. config.go anticipates an
// operator reaching for it, and the SDK refuses it because a credential would
// travel in cleartext. Reported as transport, a workflow retrying on transport
// retries it forever and the close path takes the generic arm instead of the one
// that says what to fix.
func TestARefusedBaseURLIsAConfigFaultNotATransportOne(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		url  string
	}{
		{"plain http to a non-loopback host", "http://omnigent.omnigent.svc.cluster.local:6767"},
		{"credentials in the URL", "https://user:pass@omnigent.example.com"},
		{"not a URL at all", "::::not_a_url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := New(driver.Config{BaseURL: tc.url, Token: "t"}, driver.Policy{},
				slog.New(slog.NewTextHandler(discard{}, nil)))
			_, err := h.newClient(t.Context())
			if err == nil {
				t.Fatal("the SDK accepted a base URL it documents as refused")
			}
			if !errors.Is(err, driver.ErrConfig) {
				t.Errorf("error does not wrap driver.ErrConfig, so this exits as "+
					"retryable and a workflow retries a fault no retry can fix: %v", err)
			}
			// Still wraps the SDK's own error, so the message keeps the reason.
			if !errors.Is(err, omnigent.ErrInvalidArgument) {
				t.Errorf("error no longer wraps the SDK's reason: %v", err)
			}
		})
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
