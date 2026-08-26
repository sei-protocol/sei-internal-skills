package omni

import (
	"net/http"
	"testing"
)

// TestTheProductionClientHasH2HealthChecksInstalled follows the configuration to
// the client the SDK is actually handed.
//
// Dropping the configureHealthChecks call from healthCheckedClient left the whole
// suite green. The existing coverage configures a transport itself and asserts the
// fields on that one, then separately checks the production client's header and
// overall timeouts -- neither reaches the h2 handler.
//
// The failure is silent by construction: http.DefaultTransport.Clone carries
// ForceAttemptHTTP2, so https still negotiates h2 through the standard library's
// bundled implementation, which has no ReadIdleTimeout. Requests keep working and
// only dead-connection detection disappears -- on a driver that holds one stream
// open for a twenty minute turn, where a flow dropped without a reset otherwise
// leaves a socket ESTABLISHED and reusable until the kernel's retransmit ceiling.
func TestTheProductionClientHasH2HealthChecksInstalled(t *testing.T) {
	t.Parallel()

	client, err := healthCheckedClient(driverTestLogger())
	if err != nil {
		t.Fatalf("healthCheckedClient: %v", err)
	}

	tracing, ok := client.Transport.(*tracingTransport)
	if !ok {
		t.Fatalf("client.Transport = %T, want *tracingTransport", client.Transport)
	}
	base, ok := tracing.base.(*http.Transport)
	if !ok {
		t.Fatalf("tracingTransport.base = %T, want *http.Transport", tracing.base)
	}

	// ConfigureTransports registers the x/net handler here. Its absence is what the
	// standard library's own h2 leaves behind, and the two are indistinguishable
	// from the request's point of view -- which is why this reads the field rather
	// than issuing a request.
	if base.TLSNextProto["h2"] == nil {
		t.Error("the production client's transport has no x/net/http2 handler " +
			"installed, so ReadIdleTimeout and PingTimeout do not apply and a " +
			"half-open connection is only noticed at the kernel retransmit ceiling")
	}
}
