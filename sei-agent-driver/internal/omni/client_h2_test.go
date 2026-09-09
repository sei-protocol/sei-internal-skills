package omni

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
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

// TestTheMintClientCarriesTheSameProtection reads the client the token exchange is
// handed.
//
// The exchange is the rarest call the driver makes and the one that decides whether
// a run starts at all: one invocation mints per scout, again for the review, and
// again per session a --close reclaims, with an agent turn in between. A client with
// no transport of its own falls back to http.DefaultTransport, a pool shared with
// everything else in the process and carrying no ReadIdleTimeout, so the connection
// it offers has been idle across that turn with nothing asking whether it is still
// there. The request is written into a socket nobody reads and the wait is the
// kernel's.
func TestTheMintClientCarriesTheSameProtection(t *testing.T) {
	t.Parallel()

	api, err := healthCheckedClient(driverTestLogger())
	if err != nil {
		t.Fatalf("healthCheckedClient: %v", err)
	}
	mint := mintClient(api, 7*time.Second)

	if mint.Timeout != 7*time.Second {
		t.Errorf("mint client Timeout = %v, want 7s: the exchange is bounded by the "+
			"configured request timeout", mint.Timeout)
	}
	if api.Timeout != 0 {
		t.Error("the exchange's timeout was written onto the API client instead of a " +
			"copy, so it now bounds calls the SDK times for itself")
	}
	if mint.Transport != api.Transport {
		t.Error("the exchange runs on a transport of its own, so it shares neither the " +
			"API client's pool nor its configuration")
	}

	tracing, ok := mint.Transport.(*tracingTransport)
	if !ok {
		t.Fatalf("mint client Transport = %T, want *tracingTransport: a mint that times "+
			"out is placed by conn_reused and conn_idle_for, and by the request id a "+
			"gateway access-log line joins on", mint.Transport)
	}
	base, ok := tracing.base.(*http.Transport)
	if !ok {
		t.Fatalf("tracingTransport.base = %T, want *http.Transport", tracing.base)
	}
	if base == http.DefaultTransport {
		t.Error("the exchange draws from http.DefaultTransport, the connection pool " +
			"every caller in the process shares")
	}
	if base.TLSNextProto["h2"] == nil {
		t.Error("the mint client's transport has no x/net/http2 handler installed, so " +
			"ReadIdleTimeout and PingTimeout do not apply to the API calls that reuse it " +
			"after the mint, and a connection one of them leaves idle is only found dead " +
			"at the kernel retransmit ceiling")
	}
}

// TestTheMintRunsOnTheClientTheDriverBuilt follows the wiring rather than the
// fields. newClient is what decides which client the exchange gets, and a test that
// reads healthCheckedClient's output stays green when that decision changes.
//
// The request id is the observable: tracingTransport sets it, nothing else does, and
// it is what joins a client attempt to a gateway access-log line — the join that
// answers whether a mint which timed out ever arrived.
func TestTheMintRunsOnTheClientTheDriverBuilt(t *testing.T) {
	t.Parallel()

	var traced atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traced.Store(r.Header.Get(requestIDHeader) != "")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w,
			`{"access_token":"tok","token_type":"Bearer","expires_in":1800}`)
	}))
	defer srv.Close()

	h := New(driver.Config{
		BaseURL:             srv.URL,
		MachineClientID:     "machine-id",
		MachineClientSecret: "machine-secret",
	}, driver.Policy{}, driverTestLogger())

	if _, err := h.newClient(t.Context()); err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if !traced.Load() {
		t.Error("the token exchange carried no request id, so it did not run on the " +
			"client newClient built: it is on a transport with neither the HTTP/2 " +
			"health checks nor the logging that places a failure")
	}
}
