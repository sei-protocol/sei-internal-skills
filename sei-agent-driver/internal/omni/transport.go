package omni

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"time"
)

// requestIDHeader is the id Envoy correlates a request by. Setting it here rather
// than letting the proxy mint one is what lets a client attempt be joined to a
// gateway access-log line: without it the two can only be matched on timestamps,
// and a request the proxy never received leaves nothing to match against at all.
const requestIDHeader = "X-Request-Id"

// tracingTransport records what a request did below the response.
//
// A transport error says what went wrong and nothing about where. Whether a request
// reused a connection, how long that connection had been idle, and whether headers
// were written before the silence are the facts that separate a dead connection
// from a server that never answered, and none of them survive into the error.
type tracingTransport struct {
	base http.RoundTripper
	log  *slog.Logger
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	id := newRequestID()

	// Cloned because a RoundTripper must not modify the request it is given.
	req = req.Clone(req.Context())
	req.Header.Set(requestIDHeader, id)

	// Atomic because httptrace states its hooks may run on other goroutines, and on
	// HTTP/2 they do: the header write happens on the goroutine x/net/http2 spawns
	// per request, while RoundTrip returns on the one that receives the response
	// headers. As plain locals these raced, and the read below is what builds the
	// log line -- so the failure was a stale wrote_headers=false on exactly the
	// request whose failure this line exists to place.
	var (
		conn         atomic.Pointer[httptrace.GotConnInfo]
		wroteHeaders atomic.Bool
		firstByte    atomic.Bool
	)
	trace := &httptrace.ClientTrace{
		GotConn:              func(i httptrace.GotConnInfo) { conn.Store(&i) },
		WroteHeaders:         func() { wroteHeaders.Store(true) },
		GotFirstResponseByte: func() { firstByte.Store(true) },
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	elapsed := time.Since(start)

	got := conn.Load()
	attrs := []any{
		"request_id", id,
		"method", req.Method,
		"path", req.URL.Path,
		"elapsed", elapsed,
		// The three that place a failure. No connection at all is a path that could
		// not be reached; headers written with no first byte is a peer that took the
		// request and said nothing; a reused connection that does neither is the one
		// worth suspecting, since a fresh one cannot have gone stale.
		"conn_reused", got != nil && got.Reused,
		"conn_was_idle", got != nil && got.WasIdle,
		"wrote_headers", wroteHeaders.Load(),
		"got_first_byte", firstByte.Load(),
	}
	if got != nil && got.WasIdle {
		attrs = append(attrs, "conn_idle_for", got.IdleTime)
	}

	if err != nil {
		t.log.Warn("request failed", append(attrs, "error", err)...)
		return nil, err
	}

	attrs = append(attrs, "status", resp.StatusCode)
	// Envoy's own account of the request, when it is the one answering. Its flags
	// name an upstream failure, an ejection or a timeout that the status alone does
	// not distinguish.
	for _, h := range []string{"X-Envoy-Upstream-Service-Time", "X-Envoy-Decorator-Operation"} {
		if v := resp.Header.Get(h); v != "" {
			attrs = append(attrs, h, v)
		}
	}
	t.log.Debug("request completed", attrs...)
	return resp, nil
}

// newRequestID returns an id for one request. Random rather than sequential: ids
// from concurrent runs share a log index and a counter would collide there.
func newRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Unreachable in practice, and an id is a correlation aid rather than
		// something correctness rests on, so a run continues without one.
		return "unavailable"
	}
	return hex.EncodeToString(b[:])
}
