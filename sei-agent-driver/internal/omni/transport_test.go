package omni

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTracingTransportCarriesAnIDAndPlacesAFailure covers the two things a
// transport error does not say on its own: which request it was, and where it got
// to before it stopped.
func TestTracingTransportCarriesAnIDAndPlacesAFailure(t *testing.T) {
	t.Parallel()

	t.Run("the proxy is given an id to correlate on", func(t *testing.T) {
		t.Parallel()
		var seen string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get(requestIDHeader)
			w.Header().Set("X-Envoy-Upstream-Service-Time", "7")
		}))
		defer srv.Close()

		sink := &driverLogSink{}
		client := &http.Client{Transport: &tracingTransport{
			base: srv.Client().Transport,
			log:  slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug})),
		}}
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if seen == "" {
			t.Error("no request id reached the server, so nothing joins this to a gateway log line")
		}
		out := sink.String()
		if !strings.Contains(out, seen) {
			t.Errorf("the id sent (%s) is not in the log, so the join is one-way", seen)
		}
		if !strings.Contains(out, "X-Envoy-Upstream-Service-Time") {
			t.Error("the proxy's own account of the request was dropped")
		}
	})

	t.Run("a failure says how far it got", func(t *testing.T) {
		t.Parallel()
		// A port with nothing behind it: no connection, so no headers written and no
		// first byte — which is what distinguishes it from a peer that went silent.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		addr := ln.Addr().String()
		if err := ln.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}

		sink := &driverLogSink{}
		client := &http.Client{Transport: &tracingTransport{
			base: http.DefaultTransport,
			log:  slog.New(slog.NewTextHandler(sink, nil)),
		}}
		if _, err := client.Get("http://" + addr); err == nil {
			t.Fatal("Get: want an error against a closed port")
		}

		out := sink.String()
		for _, want := range []string{"request failed", "wrote_headers=false", "got_first_byte=false", "request_id="} {
			if !strings.Contains(out, want) {
				t.Errorf("log is missing %q, which is what places the failure:\n%s", want, out)
			}
		}
	})
}
