package driver

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Defaults for the knobs an operator usually leaves alone.
const (
	// DefaultOrigin is the server's first-party non-browser sentinel Origin.
	// State-changing POSTs are gated by a trusted-origin CSRF check. This driver
	// is not a browser and sends no Origin of its own, so it announces the
	// sentinel to pass that guard — the same value the Python client sends.
	DefaultOrigin = "omnigent://internal"

	// DefaultBaseURL is loopback rather than a deployment's address: callers pin
	// this binary at a ref, so a hostname here would tie every one of them to one
	// deployment and point a bare local run at it. OMNIGENT_BASE_URL carries the
	// real one.
	//
	// https or loopback, with no opt-out. The SDK offers one
	// (WithInsecureCredentialTransport) and this package does not pass it, so the
	// plain-http ClusterIP is not usable here. That costs nothing — which runner
	// runs the job and which URL it dials are independent, so the runner can still
	// be in-cluster — and what the opt-out would unlock is worse: header auth is
	// safe only because nothing outside the mesh can set X-Forwarded-Email.
	DefaultBaseURL = "http://127.0.0.1:6767"

	// DefaultAgent is the agent name to resolve. A name, not an id: ids differ
	// per deployment, so the workflow that calls this cannot hardcode one.
	DefaultAgent = "sei-droid"
)

// Config is the driver's whole configuration. Every field comes from the
// environment so no credential lives in source and the binary stays portable
// between a workflow runner and a pod.
type Config struct {
	// BaseURL is the Omnigent server. Trailing slashes are trimmed because the
	// SDK joins paths onto it.
	BaseURL string

	// Origin is sent on every request to satisfy the server's CSRF guard.
	Origin string

	// Agent is the agent *name* to resolve to an id.
	Agent string

	// Token is the bearer credential, when one was minted elsewhere. Never
	// logged, and never included in an error from this package.
	//
	// Leave it empty and set MachineClientID and MachineClientSecret to have the
	// driver mint its own, which is preferable: a token minted in-process never
	// transits a workflow step output.
	Token string

	// MachineClientID is the confidential client's identifier, matching the
	// server's OMNIGENT_M2M_CLIENT_ID.
	//
	// The grant is mounted only in the server's cookie-based auth modes, oidc and
	// accounts, because it signs with the same cookie secret those configure. A
	// deployment running header auth does not serve /oauth/token at all, and the
	// exchange there fails on a route that is not there rather than on the
	// credential.
	//
	// It is not upstream: the deployment gets it from the image built at
	// omnigent 664732a0, and omnigent-ai/omnigent#3977 is where it is proposed for
	// merge. Do not read upstream main to check this contract.
	MachineClientID string

	// MachineClientSecret is that client's secret in plaintext.
	//
	// The server keeps only its digest, in OMNIGENT_M2M_CLIENT_SECRET_HASH, hashed
	// under the cookie secret. The _HASH suffix is the whole distinction between
	// what the server holds and what this sends, so do not cross-wire the two.
	// Never logged.
	MachineClientSecret string

	// RunDeadline bounds the whole run: resolve, create or adopt, drive. On expiry
	// the run ends and the session is left as it is — the turn keeps running
	// server-side and the next invocation's prompt queues behind it.
	RunDeadline time.Duration

	// RequestTimeout bounds the requests this package times itself: the token mint
	// and the post-turn reply read.
	//
	// Not handed to the SDK as a unary timeout, so tightening it does not tighten
	// the client's own calls. The stream is bounded by StreamIdleTimeout instead,
	// since a stream outliving a long turn cannot carry a whole-exchange deadline.
	RequestTimeout time.Duration

	// UnaryTimeout bounds one non-streaming exchange, and exists because the SDK's
	// own default of 90 seconds is shorter than creating a session takes: measured
	// against the deployment, POST /v1/sessions did not return headers inside 90
	// seconds and the run died having created nothing.
	//
	// It also prices stream recovery, which is not obvious and cost a run. The SDK
	// gives the streaming client no whole-response timeout, correctly, but shares
	// the unary transport, whose ResponseHeaderTimeout is this value or 30 seconds,
	// whichever is larger. So this is also how long a stream open that will never
	// answer takes to give up, and the re-subscribe loop pays it per attempt. At 300
	// seconds, four consecutive failed opens spent the whole run deadline.
	//
	// Halved to 150 for that reason. Above the 90 that demonstrably fails a create,
	// and cheap enough that a run can absorb several dead opens and still have time
	// to review. There is no measurement of what a create actually needs -- the run
	// that set 300 adopted a session and never created one -- so this is bounded
	// guessing on both sides, and the number to revisit once a create is timed.
	UnaryTimeout time.Duration

	// StreamIdleTimeout is how long the stream may be silent before it is treated
	// as dead.
	//
	// Sized against the server's 15-second heartbeat, which exists so a client can
	// run a timeout this tight: an idle stream keeps emitting on that cadence from
	// the moment it is subscribed, so silence past a few intervals is a half-open
	// socket rather than slow work. A launching sandbox is not the exception it
	// looks like — the cadence does not wait for it, and a stream dropped early is
	// re-established with the prompt still unsent.
	StreamIdleTimeout time.Duration
}

// LoadConfig reads the configuration from the environment.
//
// It does not check the credential; that is [Config.RequireAuth], kept separate
// so a caller can load and inspect a configuration without holding a secret.
func LoadConfig() (Config, error) {
	cfg := Config{
		BaseURL: strings.TrimRight(envOr("OMNIGENT_BASE_URL", DefaultBaseURL), "/"),
		Origin:  envOr("OMNIGENT_ORIGIN", DefaultOrigin),
		Agent:   envOr("SEIDROID_AGENT_ID", DefaultAgent),
		Token:   resolveToken(),

		// M2M, matching what the server names its own registry entry, so an
		// operator configures one vocabulary rather than translating between two.
		MachineClientID:     strings.TrimSpace(os.Getenv("OMNIGENT_M2M_CLIENT_ID")),
		MachineClientSecret: strings.TrimSpace(os.Getenv("OMNIGENT_M2M_CLIENT_SECRET")),
	}

	// Seconds, because that is what an operator's existing values mean.
	for _, d := range []struct {
		name string
		secs float64
		dst  *time.Duration
	}{
		{"XREVIEW_RUN_DEADLINE_S", 1200, &cfg.RunDeadline},
		{"XREVIEW_REQUEST_TIMEOUT_S", 30, &cfg.RequestTimeout},
		{"XREVIEW_UNARY_TIMEOUT_S", 150, &cfg.UnaryTimeout},
		{"XREVIEW_STREAM_IDLE_TIMEOUT_S", 60, &cfg.StreamIdleTimeout},
	} {
		secs, err := secondsOr(d.name, d.secs)
		if err != nil {
			return Config{}, err
		}
		*d.dst = time.Duration(secs * float64(time.Second))
	}

	return cfg, nil
}

// RequireAuth reports whether a usable credential was supplied: a bearer token,
// or a machine client to mint one with.
//
// Separate from [LoadConfig] and loud, because an anonymous request earns a 401
// that reads like a misconfigured server rather than a missing secret. A
// half-supplied machine client is called out on its own — that mistake has a
// different fix, and naming the token variables would send an operator the wrong
// way.
func (c Config) RequireAuth() error {
	id, secret := c.MachineClientID, c.MachineClientSecret
	switch {
	case c.Token != "":
		return nil
	case id != "" && secret != "":
		return nil
	case id != "" || secret != "":
		missing := "OMNIGENT_M2M_CLIENT_SECRET"
		if id == "" {
			missing = "OMNIGENT_M2M_CLIENT_ID"
		}
		return fmt.Errorf("%w: machine client is half-configured; %s is not set",
			ErrConfig, missing)
	default:
		return fmt.Errorf("%w: no API credential; set OMNIGENT_M2M_CLIENT_ID with "+
			"OMNIGENT_M2M_CLIENT_SECRET, or OMNIGENT_API_TOKEN or "+
			"OMNIGENT_API_TOKEN_FILE", ErrConfig)
	}
}

// MintsOwnToken reports whether the driver will exchange machine credentials for
// a token rather than use one it was handed. An explicit token wins, so a
// caller can override the exchange without unsetting the client.
func (c Config) MintsOwnToken() bool {
	return c.Token == "" && c.MachineClientID != "" && c.MachineClientSecret != ""
}

// resolveToken prefers a mounted file over an inline variable, re-read each run
// so a rotated credential needs no redeploy. An unreadable file yields an empty
// token for [Config.RequireAuth] to reject: the distinction does not change what
// an operator has to do.
func resolveToken() string {
	if path := os.Getenv("OMNIGENT_API_TOKEN_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(raw))
	}
	return strings.TrimSpace(os.Getenv("OMNIGENT_API_TOKEN"))
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// secondsOr parses a duration-in-seconds variable, rejecting a value that is not
// a positive number. A zero or negative deadline would disable the bound it
// exists to enforce, so it is a configuration error rather than a silent
// unbounded run.
func secondsOr(name string, fallback float64) (float64, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	secs, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a number, got %q", ErrConfig, name, raw)
	}
	if secs <= 0 {
		return 0, fmt.Errorf("%w: %s must be positive, got %q", ErrConfig, name, raw)
	}
	return secs, nil
}
