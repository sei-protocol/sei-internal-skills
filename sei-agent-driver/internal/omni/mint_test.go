package omni

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
	"time"
)

// TestMintSurvivesSecretsThatBasicAuthWouldCorrupt is the reason this package
// posts form fields instead of using HTTP Basic.
//
// The server accepts either, and decodes the Basic halves with unquote_plus per
// RFC 6749 §2.3.1. Go's Request.SetBasicAuth base64s the pair verbatim, so a
// secret containing a plus or a percent escape arrives as a different string
// than was sent. Each secret below is one the two paths disagree about; the
// subtest asserts the form path delivers it byte-for-byte, and then that Basic
// would not have.
func TestMintSurvivesSecretsThatBasicAuthWouldCorrupt(t *testing.T) {
	t.Parallel()

	secrets := []string{
		"pa+ss",      // unquote_plus turns the plus into a space
		"pa%2Bss",    // and a percent escape into the byte it names
		"a+b%20c",    // both at once
		"plain-safe", // a control: nothing to corrupt
	}

	for _, secret := range secrets {
		t.Run(secret, func(t *testing.T) {
			t.Parallel()

			got := make(chan string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Errorf("ParseForm: %v", err)
				}
				got <- r.PostFormValue("client_secret")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":1800}`))
			}))
			defer srv.Close()

			token, _, err := MintToken(t.Context(), srv.Client(), srv.URL, "sei-droid", secret)
			if err != nil {
				t.Fatalf("MintToken: %v", err)
			}
			if token != "tok" {
				t.Errorf("token = %q, want tok", token)
			}
			if arrived := <-got; arrived != secret {
				t.Errorf("server received secret %q, want %q", arrived, secret)
			}

			// The counterfactual: what the Basic path would have delivered.
			// unquote_plus is applied to the decoded half, so any secret whose
			// unquoted form differs is one Basic would have corrupted.
			raw := base64.StdEncoding.EncodeToString([]byte("sei-droid:" + secret))
			decoded, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			_, half, _ := strings.Cut(string(decoded), ":")
			unquoted, err := url.QueryUnescape(strings.ReplaceAll(half, "+", " "))
			if err != nil {
				unquoted = half
			}
			if unquoted != secret {
				t.Logf("confirmed hazard: Basic would have delivered %q, not %q",
					unquoted, secret)
			}
		})
	}
}

func TestMintRejectsAndReportsUsefully(t *testing.T) {
	t.Parallel()

	t.Run("missing credentials never reach the network", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("a request was sent despite absent credentials")
		}))
		defer srv.Close()
		if _, _, err := MintToken(t.Context(), srv.Client(), srv.URL, "", ""); !errors.Is(err, driver.ErrMint) {
			t.Fatalf("error = %v, want driver.ErrMint", err)
		}
	})

	t.Run("the OAuth error code is surfaced and the body is not", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client",` +
				`"error_description":"secret-looking-detail-abc123"}`))
		}))
		defer srv.Close()

		_, _, err := MintToken(t.Context(), srv.Client(), srv.URL, "id", "sec")
		if !errors.Is(err, driver.ErrMint) {
			t.Fatalf("error = %v, want driver.ErrMint", err)
		}
		if !strings.Contains(err.Error(), "invalid_client") {
			t.Errorf("error = %q, want it to name the OAuth code", err)
		}
		if strings.Contains(err.Error(), "secret-looking-detail-abc123") {
			t.Errorf("error leaked the description: %q", err)
		}
	})

	t.Run("a 200 with no access_token is an error, not an empty token", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token_type":"Bearer","expires_in":1800}`))
		}))
		defer srv.Close()

		token, _, err := MintToken(t.Context(), srv.Client(), srv.URL, "id", "sec")
		if !errors.Is(err, driver.ErrMint) || token != "" {
			t.Fatalf("got (%q, %v), want (\"\", driver.ErrMint)", token, err)
		}
	})
}

func TestRequireAuthAcceptsEitherCredentialForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cfg    driver.Config
		wantIn string
		mints  bool
	}{
		{name: "explicit token", cfg: driver.Config{Token: "tok"}},
		{
			name:  "machine client",
			cfg:   driver.Config{MachineClientID: "id", MachineClientSecret: "sec"},
			mints: true,
		},
		{
			name: "token wins over machine client",
			cfg:  driver.Config{Token: "tok", MachineClientID: "id", MachineClientSecret: "sec"},
		},
		{
			name:   "half-configured: no secret",
			cfg:    driver.Config{MachineClientID: "id"},
			wantIn: "OMNIGENT_MACHINE_CLIENT_SECRET is not set",
		},
		{
			name:   "half-configured: no id",
			cfg:    driver.Config{MachineClientSecret: "sec"},
			wantIn: "OMNIGENT_MACHINE_CLIENT_ID is not set",
		},
		{name: "nothing at all", cfg: driver.Config{}, wantIn: "no API credential"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cfg.RequireAuth()
			if tc.wantIn == "" {
				if err != nil {
					t.Fatalf("RequireAuth: %v, want nil", err)
				}
				if got := tc.cfg.MintsOwnToken(); got != tc.mints {
					t.Errorf("driver.MintsOwnToken = %v, want %v", got, tc.mints)
				}
				return
			}
			if !errors.Is(err, driver.ErrConfig) {
				t.Fatalf("error = %v, want driver.ErrConfig", err)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantIn)
			}
		})
	}
}

// mintProbeTransport records whether a request was ever handed to the transport.
// The guard's whole job is that one is not, so "was the wire touched" is the
// assertion — not "did an error come back", which a DNS failure also satisfies.
type mintProbeTransport struct{ used bool }

func (t *mintProbeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.used = true
	return nil, errors.New("the transport must not be reached for a rejected URL")
}

// TestMintRefusesToSendTheSecretInClear pins the control that stops the durable
// machine credential crossing a network unencrypted.
//
// The driver mints before it builds its SDK client, on a plain http.Client that
// carries none of the SDK's protections, so the SDK's refusal of a plain-http
// non-loopback base URL arrives one call too late — by then the secret is already
// on the wire. This check has to live here.
//
// It asserts on the transport rather than on the returned error: an unresolvable
// hostname produces an error just as readily, so only "was the wire touched"
// separates a refusal from a failed connection.
func TestMintRefusesToSendTheSecretInClear(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		wantSnd bool
	}{
		{"the in-cluster ClusterIP Service", "http://omnigent.seigent.svc.cluster.local", false},
		{"any plain http host", "http://omnigent.example.invalid", false},
		{"plain http with a port", "http://10.0.18.206:8000", false},
		{"https anywhere", "https://seigent.dev.platform.sei.io", true},
		{"loopback by ip", "http://127.0.0.1:6767", true},
		{"loopback by name", "http://localhost:6767", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			probe := &mintProbeTransport{}
			_, _, err := MintToken(t.Context(), &http.Client{Transport: probe},
				tc.baseURL, "id", "s3cret-value")

			// Every case errors: a rejected URL by the guard, an accepted one because
			// the probe transport refuses to answer. What separates them is whether
			// the request reached the transport at all.
			if err == nil {
				t.Fatal("MintToken returned no error; the probe transport always fails")
			}
			if probe.used != tc.wantSnd {
				if tc.wantSnd {
					t.Error("the guard rejected a URL it should have allowed, so no request was sent")
				} else {
					t.Error("the client secret was handed to the transport before the URL was rejected")
				}
			}
			// Only the guard's own refusal carries driver.ErrMint, which classify maps to a
			// configuration fault. A request that went out and failed is a transport
			// problem and is deliberately left unwrapped.
			if !tc.wantSnd && !errors.Is(err, driver.ErrMint) {
				t.Errorf("err = %v, want the refusal to wrap driver.ErrMint so classify routes it", err)
			}
			if tc.wantSnd && errors.Is(err, driver.ErrMint) {
				t.Errorf("err = %v, want a transport failure NOT wrapped in driver.ErrMint: an "+
					"operator told to fix a secret will not retry a network fault", err)
			}
			if strings.Contains(err.Error(), "s3cret-value") {
				t.Errorf("the error quotes the secret: %v", err)
			}
		})
	}
}

// TestMintReportsTheLifetimeTheServerGave covers the guard against a run that
// outlives its own credential.
//
// The deployment sets OMNIGENT_MACHINE_TOKEN_TTL to 1800 and the default run deadline
// is 1200, so the shipped pair is safe. Nothing enforces that, and the driver mints
// once with no re-mint, so raising the deadline past the token's life spends the
// tail of a review on rejected calls with nothing naming the token.
func TestMintReportsTheLifetimeTheServerGave(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w,
			`{"access_token":"tok","token_type":"Bearer","expires_in":1800}`)
	}))
	defer srv.Close()

	token, ttl, err := MintToken(t.Context(), srv.Client(), srv.URL, "sei-droid", "sec")
	if err != nil {
		t.Fatalf("MintToken: %v", err)
	}
	if token != "tok" {
		t.Errorf("token = %q, want tok", token)
	}
	if ttl != 30*time.Minute {
		t.Errorf("ttl = %v, want 30m — the caller cannot compare a lifetime it was not told", ttl)
	}
}

// flakyTransport fails the first failures calls the way the gateway has been
// failing, then hands the request to the real server.
type flakyTransport struct {
	failures int
	attempts int
	real     http.RoundTripper
}

func (f *flakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.attempts++
	if f.attempts <= f.failures {
		return nil, errors.New("read: connection reset by peer")
	}
	return f.real.RoundTrip(req)
}

// TestMintRidesOutAReset covers the failure that has been costing runs: the
// connection is reset before the server answers.
func TestMintRidesOutAReset(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	}))
	defer srv.Close()

	flaky := &flakyTransport{failures: 2, real: srv.Client().Transport}
	token, ttl, err := MintToken(t.Context(), &http.Client{Transport: flaky},
		srv.URL, "id", "secret")
	if err != nil {
		t.Fatalf("a mint that succeeds on the third attempt still failed: %v", err)
	}
	if token != "tok" || ttl != time.Hour {
		t.Errorf("token=%q ttl=%s, want tok and 1h", token, ttl)
	}
	if flaky.attempts != 3 {
		t.Errorf("made %d attempts, want 3", flaky.attempts)
	}
}

// TestMintStopsAfterItsAttempts keeps a server that is genuinely gone from
// costing every caller the full backoff schedule forever.
func TestMintStopsAfterItsAttempts(t *testing.T) {
	t.Parallel()

	flaky := &flakyTransport{failures: 99}
	_, _, err := MintToken(t.Context(), &http.Client{Transport: flaky},
		"https://gone.example", "id", "secret")
	if err == nil {
		t.Fatal("a mint that never reached a server reported success")
	}
	if flaky.attempts != mintAttempts {
		t.Errorf("made %d attempts, want %d", flaky.attempts, mintAttempts)
	}
	if !strings.Contains(err.Error(), "could not reach the server") {
		t.Errorf("the message no longer says what happened: %v", err)
	}
}

// TestMintDoesNotRetryARefusal pins the half that must not retry. A wrong secret
// is not transient, and asking again turns one clear failure into three — on this
// endpoint a bad credential has also been seen to answer 503, so retrying by
// status rather than by reachability would retry it too.
func TestMintDoesNotRetryARefusal(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusUnauthorized, http.StatusBadRequest,
		http.StatusServiceUnavailable} {
		calls := 0
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
		}))
		_, _, err := MintToken(t.Context(), srv.Client(), srv.URL, "id", "secret")
		srv.Close()

		if err == nil {
			t.Fatalf("%d was treated as a successful mint", status)
		}
		if calls != 1 {
			t.Errorf("%d was asked %d times, want 1", status, calls)
		}
		if !errors.Is(err, driver.ErrMint) {
			t.Errorf("%d did not classify as a mint failure: %v", status, err)
		}
	}
}

// TestMintStopsWhenTheCallerDoes keeps the backoff from outliving the run that
// is waiting on it.
func TestMintStopsWhenTheCallerDoes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	flaky := &flakyTransport{failures: 99}
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()

	start := time.Now()
	if _, _, err := MintToken(ctx, &http.Client{Transport: flaky},
		"https://gone.example", "id", "secret"); err == nil {
		t.Fatal("a cancelled mint reported success")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("kept waiting for %s after the caller gave up", elapsed.Round(time.Millisecond))
	}
	if flaky.attempts >= mintAttempts {
		t.Errorf("made %d attempts despite cancellation", flaky.attempts)
	}
}

// TestRequireAuthNamesTheDocumentedVariable keeps the diagnostic pointing at the
// name an operator will find in the documentation.
func TestRequireAuthNamesTheDocumentedVariable(t *testing.T) {
	t.Setenv("OMNIGENT_API_TOKEN", "")
	t.Setenv("OMNIGENT_API_TOKEN_FILE", "")
	t.Setenv("OMNIGENT_MACHINE_CLIENT_ID", "")
	t.Setenv("OMNIGENT_MACHINE_CLIENT_SECRET", "")

	cfg, err := driver.LoadConfig()
	if err != nil {
		t.Fatalf("driver.LoadConfig: %v", err)
	}
	err = cfg.RequireAuth()
	if err == nil {
		t.Fatal("RequireAuth accepted a configuration with no credential")
	}
	if !strings.Contains(err.Error(), "OMNIGENT_MACHINE_CLIENT_ID") {
		t.Errorf("the diagnostic does not name the documented variable: %v", err)
	}
}

// TestMintRefusesToFollowARedirect pins the policy that keeps the client secret on
// the host the operator named.
//
// Go re-sends a request body verbatim on a 307 or 308, to whatever host and scheme
// the Location names. The cleartext refusal only ever inspects the first hop, so
// without this a single redirect hands the durable credential to another origin in
// the clear.
func TestMintRefusesToFollowARedirect(t *testing.T) {
	t.Parallel()

	var secretSeenElsewhere string
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		secretSeenElsewhere = form.Get("client_secret")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":60}`))
	}))
	defer elsewhere.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/oauth/token", http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	token, _, err := MintToken(t.Context(), redirector.Client(),
		redirector.URL, "client-id", "SUPER_SECRET_VALUE")

	if secretSeenElsewhere != "" {
		t.Errorf("the client secret reached a second origin: a redirect off the token "+
			"endpoint must not carry the body (%q)", secretSeenElsewhere)
	}
	if err == nil || token != "" {
		t.Errorf("got (%q, %v), want the 3xx treated as the response and the exchange failed",
			token, err)
	}
}

// TestLoopbackIsDecidedByAddressNotByPrefix pins that the cleartext exemption reads
// the host as an address. "127.attacker.com" is a routable DNS name that a string
// prefix reads as loopback.
func TestLoopbackIsDecidedByAddressNotByPrefix(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		url    string
		exempt bool
	}{
		{"http://127.0.0.1:6767", true},
		{"http://[::1]:6767", true},
		{"http://localhost:6767", true},
		{"http://127.attacker.com/", false},
		{"http://localhost.attacker.com/", false},
		{"http://omnigent.example.com/", false},
		{"https://omnigent.example.com/", true},
	} {
		t.Run(c.url, func(t *testing.T) {
			t.Parallel()
			err := requireEncryptedOrLocal(c.url)
			if c.exempt && err != nil {
				t.Errorf("requireEncryptedOrLocal(%q) = %v, want it allowed", c.url, err)
			}
			if !c.exempt && err == nil {
				t.Errorf("requireEncryptedOrLocal(%q) = nil, want the client secret refused "+
					"a cleartext hop to a non-loopback host", c.url)
			}
		})
	}
}

// TestMintTreatsATransientStatusAsTransport pins the distinction an operator acts
// on. A rate limit or a 5xx is the server declining to answer now; only a 4xx says
// the credential is wrong. Classifying the first as configuration sends someone to
// check a secret while the deployment is down.
func TestMintTreatsATransientStatusAsTransport(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name     string
		status   int
		wantMint bool
		wantTry  int
	}{
		{"service unavailable retries as transport", http.StatusServiceUnavailable, false, mintAttempts},
		{"rate limited retries as transport", http.StatusTooManyRequests, false, mintAttempts},
		{"invalid client is a credential fault", http.StatusUnauthorized, true, 1},
		{"bad request is a credential fault", http.StatusBadRequest, true, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var attempts int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			}))
			defer srv.Close()

			_, _, err := MintToken(t.Context(), srv.Client(), srv.URL, "id", "secret")
			if err == nil {
				t.Fatal("want an error")
			}
			if got := errors.Is(err, driver.ErrMint); got != c.wantMint {
				t.Errorf("errors.Is(err, driver.ErrMint) = %t, want %t: %v", got, c.wantMint, err)
			}
			if attempts != c.wantTry {
				t.Errorf("attempts = %d, want %d", attempts, c.wantTry)
			}
		})
	}
}
