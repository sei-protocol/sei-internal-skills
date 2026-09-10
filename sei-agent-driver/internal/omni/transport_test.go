package omni

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
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
		for _, want := range []string{"request failed", "wrote_headers=false", "got_first_byte=false", "request_id=", "dial_error="} {
			if !strings.Contains(out, want) {
				t.Errorf("log is missing %q, which is what places the failure:\n%s", want, out)
			}
		}
	})

	t.Run("a dial the request recovered from is not reported", func(t *testing.T) {
		t.Parallel()
		// One request can dial twice -- the second family of a dual-stack address,
		// or a replay onto a fresh connection. The address it failed on is not the
		// one it went on to hold, and naming it points an exit-6 hunt at a backend
		// this request never used.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer srv.Close()

		base := http.DefaultTransport.(*http.Transport).Clone()
		base.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			// What net.Dialer reports when it moves on to the next address.
			httptrace.ContextClientTrace(ctx).
				ConnectDone(network, "[::1]:1", errors.New("connect: no route to host"))
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}

		sink := &driverLogSink{}
		client := &http.Client{Transport: &tracingTransport{
			base: base,
			log:  slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug})),
		}}
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if out := sink.String(); strings.Contains(out, "dial_error=") {
			t.Errorf("a dial the request recovered from is still in the log:\n%s", out)
		}
	})

	t.Run("the peer it held is named", func(t *testing.T) {
		t.Parallel()
		// Which target answered, not which service was addressed. A failure behind a
		// load balancer is only attributable to one backend if the attempts that
		// worked named theirs too.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
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

		want := strings.TrimPrefix(srv.URL, "http://")
		if out := sink.String(); !strings.Contains(out, "peer="+want) {
			t.Errorf("log does not name the peer %s:\n%s", want, out)
		}
	})
}
