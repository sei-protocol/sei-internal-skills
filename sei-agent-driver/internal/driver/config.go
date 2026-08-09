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

	// DefaultBaseURL is loopback, matching the address a self-hosted server
	// advertises. Plain http is legitimate here only because nothing leaves the
	// machine — the SDK exempts loopback from its cleartext-credential refusal for
	// that reason, and so does [MintToken].
	//
	// Deliberately not a deployment's address. This binary is fetched and built by
	// caller repositories at a pinned ref, so a hostname baked in here would couple
	// every caller to one deployment and would point a bare local run at it by
	// default. A real deployment's URL belongs in OMNIGENT_BASE_URL, which the
	// review workflow always sets.
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
	// server's OMNIGENT_MACHINE_CLIENT_ID.
	MachineClientID string

	// MachineClientSecret is that client's secret in plaintext.
	//
	// The server stores only its digest, under
	// OMNIGENT_MACHINE_CLIENT_SECRET_HASH — the missing _HASH here is the whole
	// distinction, so do not cross-wire the two. Never logged.
	MachineClientSecret string

	// RunDeadline bounds the whole run: resolve, create, drive and teardown. On
	// expiry the turn is stopped and the conversation kept.
	RunDeadline time.Duration

	// RequestTimeout bounds the requests this package times itself: the token
	// mint, the release, and the post-turn reply read.
	//
	// It is deliberately not handed to the SDK as a unary timeout, so the client's
	// own calls — listing, create, send, resolve — keep the SDK's default instead.
	// Tightening this knob therefore does not tighten those. The event stream is
	// not covered either, because a stream outliving a long turn cannot carry a
	// whole-exchange deadline; StreamIdleTimeout bounds that.
	RequestTimeout time.Duration

	// StreamIdleTimeout is how long the stream may be silent before it is
	// treated as dead. The server emits a heartbeat every 15s on an idle
	// stream, so this must stay comfortably above that or a healthy idle
	// stream is torn down between turns.
	//
	// The heartbeat cadence is the wrong thing to size this against on a session
	// that was just created, which is why this is minutes rather than seconds. A
	// cold managed sandbox provisions, clones the repository and connects a runner
	// before it announces itself, and stays quiet throughout — long enough on a
	// large repository to outlast a heartbeat-sized budget. The run deadline is the
	// backstop.
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

		// Named to mirror the server's own machine-client variables so an
		// operator configures one pair, not two vocabularies.
		MachineClientID:     strings.TrimSpace(os.Getenv("OMNIGENT_MACHINE_CLIENT_ID")),
		MachineClientSecret: strings.TrimSpace(os.Getenv("OMNIGENT_MACHINE_CLIENT_SECRET")),
	}

	// Durations are configured in seconds because that is what the original
	// driver's variables meant, and an operator's existing values must keep
	// working.
	for _, d := range []struct {
		name string
		secs float64
		dst  *time.Duration
	}{
		{"XREVIEW_RUN_DEADLINE_S", 1200, &cfg.RunDeadline},
		{"XREVIEW_REQUEST_TIMEOUT_S", 30, &cfg.RequestTimeout},
		{"XREVIEW_STREAM_IDLE_TIMEOUT_S", 300, &cfg.StreamIdleTimeout},
	} {
		secs, err := secondsOr(d.name, d.secs)
		if err != nil {
			return Config{}, err
		}
		*d.dst = time.Duration(secs * float64(time.Second))
	}

	return cfg, nil
}

// RequireAuth reports whether a usable credential was supplied, in either of the
// two accepted forms: a bearer token, or a machine client id and secret to mint
// one with.
//
// Separate from [LoadConfig] and loud rather than silent: no credential would
// otherwise send an anonymous request, and the server's answer to that is a 401
// that reads like a misconfigured server rather than a missing secret.
//
// A half-supplied machine client is rejected on its own rather than falling
// through to "no credential", because the two mistakes have different fixes and
// an operator who set one of the pair does not need to be told about the token
// variables.
func (c Config) RequireAuth() error {
	id, secret := c.MachineClientID, c.MachineClientSecret
	switch {
	case c.Token != "":
		return nil
	case id != "" && secret != "":
		return nil
	case id != "" || secret != "":
		missing := "OMNIGENT_MACHINE_CLIENT_SECRET"
		if id == "" {
			missing = "OMNIGENT_MACHINE_CLIENT_ID"
		}
		return fmt.Errorf("%w: machine client is half-configured; %s is not set",
			ErrConfig, missing)
	default:
		return fmt.Errorf("%w: no API credential; set OMNIGENT_API_TOKEN, "+
			"OMNIGENT_API_TOKEN_FILE, or OMNIGENT_MACHINE_CLIENT_ID with "+
			"OMNIGENT_MACHINE_CLIENT_SECRET", ErrConfig)
	}
}

// MintsOwnToken reports whether the driver will exchange machine credentials for
// a token rather than use one it was handed. An explicit token wins, so a
// caller can override the exchange without unsetting the client.
func (c Config) MintsOwnToken() bool {
	return c.Token == "" && c.MachineClientID != "" && c.MachineClientSecret != ""
}

// resolveToken prefers a mounted file over an inline variable.
//
// The file is read on each invocation so a rotated credential is picked up
// without a redeploy. An unreadable file yields an empty token rather than an
// error here, which [Config.RequireAuth] then rejects with one message for both
// causes — the distinction does not change what an operator has to do.
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
