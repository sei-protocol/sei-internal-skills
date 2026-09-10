package omni

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
	"golang.org/x/net/http2"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// How an HTTP/2 connection proves it is still there.
//
// The idle bound is above the server's 15-second stream heartbeat so a healthy
// stream keeps resetting it, and a ping is only sent once frames have actually
// stopped. The ping bound is short because a live peer answers immediately; the
// cost of waiting is another request written into a dead socket.
const (
	http2ReadIdleTimeout = 20 * time.Second
	http2PingTimeout     = 5 * time.Second

	// defaultResponseHeaderTimeout bounds the wait for response headers. Long
	// enough for a create that provisions a sandbox, short enough that a stream
	// open which will never answer is retried rather than waited out.
	//
	// A constant, and deliberately not derived from the configured unary timeout:
	// this is what a dead stream open costs, and the re-subscribe loop pays it once
	// per attempt. Raising it with that knob would make every reconnect on a wedged
	// server more expensive, which is the opposite of what raising a request budget
	// is meant to buy. Config's SEIDROID_UNARY_TIMEOUT_S prices the SDK's own unary
	// calls; it does not move this wall.
	defaultResponseHeaderTimeout = 60 * time.Second
)

// newClient mints a token when configured to, then builds the SDK client.
//
// Origin is sent on every request because the server gates state-changing POSTs
// behind a trusted-origin check and this caller is not a browser. It rides
// WithAuthHeader, which is a general header setter despite the name — and the
// consequence of that naming is deliberate here: the SDK treats what it sets as
// credential-bearing and so refuses to carry it across an unsafe redirect, which
// is the behaviour this header wants anyway.
func (h *Host) newClient(ctx context.Context) (*omnigent.Client, error) {
	httpClient, err := healthCheckedClient(h.log)
	if err != nil {
		return nil, err
	}

	token := h.cfg.Token
	if h.cfg.MintsOwnToken() {
		minted, ttl, err := mintToken(ctx, mintClient(httpClient, h.cfg.RequestTimeout),
			h.cfg.BaseURL, h.cfg.MachineClientID, h.cfg.MachineClientSecret)
		if err != nil {
			return nil, err
		}
		h.log.Info("minted a machine token",
			"client_id", h.cfg.MachineClientID, "token_ttl", ttl)

		// Minted once and never re-minted, so a deadline past the token's life
		// spends its tail on 401s: the review is still running, every call is
		// rejected, and nothing in that failure names the token as the cause.
		if ttl > 0 && ttl < h.cfg.RunDeadline {
			h.log.Warn("the run deadline outlives the minted token, so a long turn will start failing as unauthorised",
				"token_ttl", ttl, "run_deadline", h.cfg.RunDeadline)
		}
		token = minted
	}

	client, err := omnigent.New(h.cfg.BaseURL,
		omnigent.WithHTTPClient(httpClient),
		omnigent.WithBearerToken(token),
		omnigent.WithAuthHeader("Origin", h.cfg.Origin),
		omnigent.WithUserAgent("seidroid-review"),
		omnigent.WithStreamIdleTimeout(h.cfg.StreamIdleTimeout),
		omnigent.WithUnaryTimeout(h.cfg.UnaryTimeout),
	)
	// A base URL the SDK refuses is a misconfiguration, not a transport fault, and
	// the difference is what the caller does next: a retry cannot fix it. The plain
	// http ClusterIP is the mistake this catches -- the SDK rejects it because the
	// credential would travel in cleartext -- and unwrapped it reported as
	// retryable, so a workflow retrying on transport retried it forever.
	if errors.Is(err, omnigent.ErrInvalidArgument) {
		return nil, fmt.Errorf("%w: %w", driver.ErrConfig, err)
	}
	if err != nil {
		return nil, err
	}
	return client, nil
}

// healthCheckedClient returns a client whose HTTP/2 connections answer for
// themselves.
//
// Without this, nothing tells the transport a connection has stopped carrying
// traffic. A middlebox that drops a flow without a reset leaves the socket
// ESTABLISHED, the connection stays a reuse candidate, and every request handed to
// it is written into a socket nothing is reading. Recovery then waits on the
// kernel's retransmit ceiling, which is minutes.
//
// ReadIdleTimeout makes the transport send a PING once a connection has been quiet;
// PingTimeout drops the connection when the PING is not answered, so the next
// request dials a new one. The idle bound sits above the server's 15-second stream
// heartbeat, so a healthy stream resets the timer and pings only when frames have
// genuinely stopped.
func healthCheckedClient(log *slog.Logger) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// The header bound is the only timeout a streaming request can carry, since a
	// stream's body is unbounded by design.
	transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout

	if _, err := configureHealthChecks(transport); err != nil {
		return nil, err
	}

	return &http.Client{Transport: &tracingTransport{base: transport, log: log}}, nil
}

// mintClient returns the client the token exchange runs on: the API client's
// transport, under the exchange's own overall timeout.
//
// What actually fixes the mint is not the health checks — newClient builds this
// transport fresh, two lines above, so its pool is empty and the exchange always
// dials cold; ReadIdleTimeout/PingTimeout never get a chance to fire on it, and
// could not fire between retries either (transportBackoff's 0.5s/2s are both far under
// the 20s idle bound). What fixes it is that the pool is now PRIVATE to this
// newClient call instead of the process-shared http.DefaultTransport, where an
// earlier invocation's mint — one per scout, one for the review, another per
// session a --close reclaims — left an idle connection for this one to inherit,
// with nothing asking it whether it still existed. The health checks earn their
// place here on the API calls that follow, which DO reuse the connection this
// mint warms for them.
//
// A copy rather than the client itself, because this overall timeout is the
// exchange's: the SDK sets its own on the copies it takes for unary calls and for
// streams. The copy still carries the shared transport's ResponseHeaderTimeout
// (60s), which x/net/http2 honours once ConfigureTransports has run — so this
// Timeout only fully bounds the exchange below that wall; a caller who raises
// SEIDROID_REQUEST_TIMEOUT_S past 60s gets a 60s header wall on each of the three
// retries rather than one longer wait. Benign (the wall still fails as the
// retryable transport error mintToken already retries on), but worth knowing
// before raising that knob to chase a mint timeout.
func mintClient(api *http.Client, timeout time.Duration) *http.Client {
	mint := *api
	mint.Timeout = timeout
	return &mint
}

// configureHealthChecks enables HTTP/2 keepalive pings on transport and returns the
// HTTP/2 transport it configured.
//
// Separate from its caller, and returning what it set, because ReadIdleTimeout and
// PingTimeout live on the *http2.Transport and nothing reaches it from a finished
// client: http2's configure call refuses a transport it has already enabled.
// Whether the call ran at all is readable — it populates Transport.TLSNextProto["h2"]
// — and that is what pins it.
func configureHealthChecks(transport *http.Transport) (*http2.Transport, error) {
	h2, err := http2.ConfigureTransports(transport)
	if err != nil {
		return nil, fmt.Errorf("%w: configuring http/2 health checks: %w", driver.ErrConfig, err)
	}
	h2.ReadIdleTimeout = http2ReadIdleTimeout
	h2.PingTimeout = http2PingTimeout
	return h2, nil
}
