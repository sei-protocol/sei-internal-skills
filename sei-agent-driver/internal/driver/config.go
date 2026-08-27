package driver

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Defaults for the knobs an operator usually leaves alone.
const (
	// defaultOrigin is the server's first-party non-browser sentinel Origin.
	// State-changing POSTs are gated by a trusted-origin CSRF check. This driver
	// is not a browser and sends no Origin of its own, so it announces the
	// sentinel to pass that guard — the same value the Python client sends.
	defaultOrigin = "omnigent://internal"

	// defaultBaseURL is loopback rather than a deployment's address: callers pin
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
	defaultBaseURL = "http://127.0.0.1:6767"

	// defaultAgent is the agent name to resolve. A name, not an id: ids differ
	// per deployment, so the workflow that calls this cannot hardcode one.
	//
	// It has to match the bundle's name on the server exactly -- there is no
	// lookup-by-alias -- so changing it is a coordinated change with the deployment,
	// not a rename. A value the server does not know fails the run at agent
	// resolution, which is loud, and SEIDROID_AGENT_ID overrides it meanwhile.
	defaultAgent = "seidroid"
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
	// The server keeps only its digest, in OMNIGENT_MACHINE_CLIENT_SECRET_HASH, hashed
	// under the cookie secret. The _HASH suffix is the whole distinction between
	// what the server holds and what this sends, so do not cross-wire the two.
	// Never logged.
	MachineClientSecret string

	// RunDeadline bounds the whole run: resolve, create or adopt, drive. On expiry
	// the run ends and the session is left as it is — the turn keeps running
	// server-side and the next invocation's prompt queues behind it.
	RunDeadline time.Duration

	// RequestTimeout bounds each request a [Host] times for itself rather than
	// leaving to the SDK: the token mint, the liveness probe, the reply read, the
	// salvage read after a lost stream, at twice this value each paginated listing
	// walk, and at four times it the whole of a close.
	//
	// So it is not only a per-request knob. Lowering it to fail a slow mint faster
	// also shortens the liveness probe -- and a probe that times out reads as
	// not-live, which is what decides whether the prompt goes in at all.
	//
	// Not handed to the SDK as a unary timeout, so tightening it does not tighten
	// the client's own calls. The stream is bounded by StreamIdleTimeout instead,
	// since a stream outliving a long turn cannot carry a whole-exchange deadline.
	RequestTimeout time.Duration

	// UnaryTimeout bounds one non-streaming exchange. The SDK's own default is
	// shorter than a session create, which provisions a sandbox before it answers.
	//
	// It does not price stream recovery, which is worth stating because it reads as
	// though it should. The SDK would derive its transport's response-header timeout
	// from this value, but only when no HTTP client is supplied -- and a client is
	// supplied, so what a dead stream open costs is a constant on that client
	// instead. Raising this cannot lengthen the wait for a stream that never answers.
	//
	// Raise it against a timed create, which is the exchange it does price.
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

// LogValue renders the configuration without its credentials.
//
// The whole struct is the natural thing to log once at startup, and two of its
// fields are secrets. Without this, one slog.Any("config", cfg) writes a bearer
// token into a workflow log that the author of the pull request under review can
// read.
//
// The credentials are reported as set or not rather than dropped, because "which
// credential did this run use" is the first question a 401 raises, and the answer
// decides whether an operator looks at the token or at the machine client.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("base_url", c.BaseURL),
		slog.String("origin", c.Origin),
		slog.String("agent", c.Agent),
		slog.Bool("token_set", c.Token != ""),
		slog.String("machine_client_id", c.MachineClientID),
		slog.Bool("machine_client_secret_set", c.MachineClientSecret != ""),
		slog.Duration("run_deadline", c.RunDeadline),
		slog.Duration("request_timeout", c.RequestTimeout),
		slog.Duration("unary_timeout", c.UnaryTimeout),
		slog.Duration("stream_idle_timeout", c.StreamIdleTimeout),
	)
}

// String keeps a %v or %s of the configuration as safe as logging it, since a
// struct reaches a message that way just as easily. Derived from [Config.LogValue]
// so the two cannot come to disagree about which field is a secret.
//
// %#v is covered by neither, and nothing reaches it. Nothing renders a Config that
// way today.
func (c Config) String() string { return c.LogValue().String() }

// MarshalJSON renders the configuration without its credentials, so an encoder a
// debug dump reaches for cannot leak one either.
//
// Derived from [Config.LogValue], like [Config.String], and that is what makes a
// field added later safe: this reports the fields the redacted view names, so a new
// one is absent until somebody puts it there. Per-field json:"-" tags run the other
// way round -- every secret is exposed until tagged -- and a tag on the field that
// needs it most is exactly the one that gets forgotten.
func (c Config) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any)
	for _, attr := range c.LogValue().Group() {
		fields[attr.Key] = attr.Value.Any()
	}
	return json.Marshal(fields)
}

// The timeout defaults, exported because a caller that builds a [Config] without
// [LoadConfig] still needs a usable value: the constructors that take one substitute
// these rather than accept a zero, and a zero here is not a slow timeout but an
// already-expired one.
const (
	// DefaultRunDeadline bounds a whole run.
	DefaultRunDeadline = 1200 * time.Second

	// DefaultRequestTimeout bounds one request a host times for itself.
	DefaultRequestTimeout = 30 * time.Second

	// DefaultUnaryTimeout bounds one non-streaming SDK call. Longer than
	// DefaultRequestTimeout because a session create is slower than a read.
	DefaultUnaryTimeout = 150 * time.Second

	// DefaultStreamIdleTimeout bounds silence on the event stream before the read
	// is abandoned and re-established.
	DefaultStreamIdleTimeout = 300 * time.Second
)

// LoadConfig reads the configuration from the environment.
//
// It does not check the credential; that is [Config.RequireAuth], kept separate
// so a caller can load and inspect a configuration without holding a secret.
func LoadConfig() (Config, error) {
	cfg := Config{
		BaseURL: strings.TrimRight(envOr("OMNIGENT_BASE_URL", defaultBaseURL), "/"),
		Origin:  envOr("OMNIGENT_ORIGIN", defaultOrigin),
		Agent:   envOr("SEIDROID_AGENT_ID", defaultAgent),
		Token:   resolveToken(),

		MachineClientID:     strings.TrimSpace(os.Getenv("OMNIGENT_MACHINE_CLIENT_ID")),
		MachineClientSecret: strings.TrimSpace(os.Getenv("OMNIGENT_MACHINE_CLIENT_SECRET")),
	}

	// Seconds, because that is what an operator's existing values mean.
	for _, d := range []struct {
		name string
		secs float64
		dst  *time.Duration
	}{
		{"XREVIEW_RUN_DEADLINE_S", DefaultRunDeadline.Seconds(), &cfg.RunDeadline},
		{"XREVIEW_REQUEST_TIMEOUT_S", DefaultRequestTimeout.Seconds(), &cfg.RequestTimeout},
		{"XREVIEW_UNARY_TIMEOUT_S", DefaultUnaryTimeout.Seconds(), &cfg.UnaryTimeout},
		{"XREVIEW_STREAM_IDLE_TIMEOUT_S", DefaultStreamIdleTimeout.Seconds(), &cfg.StreamIdleTimeout},
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
		missing := "OMNIGENT_MACHINE_CLIENT_SECRET"
		if id == "" {
			missing = "OMNIGENT_MACHINE_CLIENT_ID"
		}
		return fmt.Errorf("%w: machine client is half-configured; %s is not set",
			ErrConfig, missing)
	default:
		return fmt.Errorf("%w: no API credential; set OMNIGENT_MACHINE_CLIENT_ID with "+
			"OMNIGENT_MACHINE_CLIENT_SECRET, or OMNIGENT_API_TOKEN or "+
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

// envOr is the value of name, or fallback when it carries none.
//
// Looked up rather than read, so present-but-empty is a case this decides rather
// than one it stumbles into -- and it decides that empty means absent, for every
// variable here that has a safe default.
//
// That is not indifference to the difference. A workflow makes a value overridable
// by writing `env: NAME: ${{ vars.NAME }}`, and Actions expands an undefined
// variable to the empty string rather than omitting the entry, as do a
// reusable-workflow input defaulting to "" and an unset matrix key. So NAME= is
// usually the runner's artefact and not something an operator said, and there is no
// way to unset a ${{ }} interpolation from inside the workflow that wrote it.
// Refusing it would fail every run in the most idiomatic setup there is.
//
// Trimmed, because the value travels into an HTTP header and a URL: a trailing
// newline from a heredoc or a file-backed variable is rejected far from here, by
// something that names neither the variable nor the newline.
func envOr(name, fallback string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	if trimmed := strings.TrimSpace(v); trimmed != "" {
		return trimmed
	}
	return fallback
}

// maxSeconds bounds every duration read from the environment. Well above any real
// budget, and far below where a float64 stops converting to a Duration.
const maxSeconds float64 = 86_400

// secondsOr parses a duration-in-seconds variable, rejecting a value that is not
// a positive number. A zero or negative deadline would disable the bound it
// exists to enforce, so it is a configuration error rather than a silent
// unbounded run.
func secondsOr(name string, fallback float64) (float64, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	// Empty means absent here for the same reason as in envOr: an undefined
	// repository variable arrives as an empty entry, not a missing one.
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	secs, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a number, got %q", ErrConfig, name, raw)
	}
	// Finite as well as positive. ParseFloat accepts Inf and NaN, and neither
	// survives the conversion to a Duration: the Go spec leaves an out-of-range
	// float-to-int conversion implementation-dependent, so "no limit" becomes either
	// an instantly expired run or a 292-year one depending on the architecture.
	// Overflow starts around 9.2e9 seconds, so the ceiling bounds that too.
	if math.IsNaN(secs) || math.IsInf(secs, 0) || secs <= 0 || secs > maxSeconds {
		return 0, fmt.Errorf("%w: %s must be a positive number of seconds under %g, got %q",
			ErrConfig, name, maxSeconds, raw)
	}
	return secs, nil
}
