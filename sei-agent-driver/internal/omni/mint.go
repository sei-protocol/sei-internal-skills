package omni

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// requireEncryptedOrLocal rejects a base URL that would put the client secret on
// a network in the clear.
//
// Loopback is exempt because nothing leaves the machine. Everything else must be
// https: the secret travels in the request body, it is the durable credential
// rather than the short-lived token it buys, and the pod network it would cross
// is not guaranteed to be encrypted or segmented.
//
// Loopback is decided by parsing the host as an address rather than by matching its
// text. "127.attacker.com" is a routable DNS name that a prefix test reads as
// loopback, and reading it that way sends the secret to it in the clear.
//
// The offending URL is not quoted back. A base URL is a place a password can hide,
// and an error message is the least controlled place a credential can end up.
func requireEncryptedOrLocal(baseURL string) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("%w: the base URL does not parse", driver.ErrMint)
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	local := host == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !local {
		return fmt.Errorf(
			"%w: refusing to send the client secret to %s, which is plain http to a "+
				"non-loopback host; use https", driver.ErrMint, u.Host)
	}
	return nil
}

// maxTokenBody caps the token response read. The body is three short fields, so
// anything larger is a wrong endpoint or a captive portal, and reading it
// unbounded would let whatever answered decide this process's memory use.
const maxTokenBody = 64 << 10

// errUnreached marks a mint that never got an answer: the connection was reset,
// refused, or timed out. Distinct from [driver.ErrMint], which is the server
// answering that it will not mint — one is worth another attempt and the other is
// not.
//
// Matched with errors.Is and never rendered: unreached carries a message that
// already says what happened, and a sentinel wrapped in front of it would put a
// word an operator cannot act on ahead of the one they can.
var errUnreached = errors.New("unreached")

// unreached carries a transport failure and answers to [errUnreached] without
// adding itself to the message.
type unreached struct{ err error }

func (u unreached) Error() string {
	return "token exchange could not reach the server: " + u.err.Error()
}
func (u unreached) Unwrap() error        { return u.err }
func (u unreached) Is(target error) bool { return target == errUnreached }

// mintBackoff is the wait before each retry, indexed by the attempt just failed.
// Short, because a reset is returned immediately and the run is holding a
// sandbox while this sleeps.
var mintBackoff = [...]time.Duration{500 * time.Millisecond, 2 * time.Second}

// mintAttempts bounds how many times a mint that never reached a server is
// tried. Derived from the backoff table rather than written beside it: the two
// have to agree, and a hand-written 3 makes raising one an index panic in the
// other. Three today, because the failure this absorbs is a single reset rather
// than an outage.
const mintAttempts = len(mintBackoff) + 1

// MintToken exchanges the machine client's credentials for a short-lived access
// token at POST /oauth/token.
//
// The credentials travel as form fields rather than as HTTP Basic. Both are
// accepted by the server and Basic actually takes precedence, but RFC 6749
// §2.3.1 has the Basic halves form-urlencoded, and the server duly
// unquote_plus-decodes them — while Go's Request.SetBasicAuth base64s the pair
// verbatim. A secret containing a plus or a percent would therefore arrive as a
// different secret than was sent, and the failure would look like a wrong
// password. url.Values.Encode and the server's form parser are symmetric, so
// this path has no such asymmetry to get wrong.
//
// The returned token is a credential: it is not logged here and it must not be
// written to a workflow step output, which is the exposure minting in-process
// exists to avoid.
//
// This refuses a cleartext hop itself rather than leaving it to the SDK. The
// driver mints before it builds its client, on a plain [http.Client] that carries
// none of the SDK's protections, so the SDK's own refusal of a plain-http
// non-loopback base URL arrives one call too late — the secret is already on the
// wire. No working configuration loses anything: every URL rejected here is one
// [omnigent.New] rejects a moment later.
// The returned lifetime is what the server said, so a caller can tell whether a
// token minted once will outlive the work it was minted for. Zero when the
// response omitted it.
func MintToken(
	ctx context.Context,
	client *http.Client,
	baseURL, clientID, clientSecret string,
) (string, time.Duration, error) {
	if clientID == "" || clientSecret == "" {
		return "", 0, fmt.Errorf("%w: client id and secret are both required", driver.ErrMint)
	}
	if err := requireEncryptedOrLocal(baseURL); err != nil {
		return "", 0, err
	}

	// Redirects are not followed. Go re-sends a request body verbatim on a 307 or
	// 308, to whatever host the Location names and whatever scheme it names, so a
	// single redirect off the token endpoint hands the client secret to another
	// origin in the clear -- past the cleartext refusal above, which only ever saw
	// the first hop. A token endpoint has no legitimate redirect, so the 3xx is
	// treated as the response and fails the exchange.
	//
	// The caller's client is copied rather than mutated: the policy belongs to this
	// exchange, and the transport it shares is unaffected.
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client = &noRedirect

	for attempt := 1; ; attempt++ {
		token, ttl, err := mintOnce(ctx, client, baseURL, clientID, clientSecret)
		if err == nil || attempt == mintAttempts || !errors.Is(err, errUnreached) {
			return token, ttl, err
		}
		select {
		case <-ctx.Done():
			// The caller's deadline, not this loop's, decides when to stop
			// waiting. Returning the mint's own error rather than the context's
			// keeps the reason the run failed in the message.
			return "", 0, err
		case <-time.After(mintBackoff[attempt-1]):
		}
	}
}

// mintOnce is one exchange. Its failures are classified rather than merged:
// whether another attempt could succeed is the only thing [MintToken] needs from
// it, and only this function can tell.
func mintOnce(
	ctx context.Context,
	client *http.Client,
	baseURL, clientID, clientSecret string,
) (string, time.Duration, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/oauth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", driver.ErrMint, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Deliberately not wrapped in driver.ErrMint, which classifies as a configuration
		// fault. Reaching the endpoint and failing is a transport problem an
		// operator should retry, not a secret they should go and fix.
		//
		// The error from Do embeds the request URL but not the body, so the
		// secret cannot reach a log through here.
		return "", 0, unreached{err}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxTokenBody))
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenBody))
	if err != nil {
		return "", 0, fmt.Errorf("%w: reading response: %w", driver.ErrMint, err)
	}

	if resp.StatusCode != http.StatusOK {
		// The OAuth error code is safe to surface and is the one thing that says
		// what to fix — invalid_client means the id or secret is wrong,
		// unsupported_grant_type means the grant is not enabled on this server.
		// The rest of the body is withheld: a non-2xx here need not have come
		// from this API at all.
		return "", 0, fmt.Errorf("%w: the token endpoint returned %d (%s)",
			driver.ErrMint, resp.StatusCode, oauthErrorCode(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", 0, fmt.Errorf("%w: response was not JSON: %w", driver.ErrMint, err)
	}
	if payload.AccessToken == "" {
		return "", 0, fmt.Errorf("%w: response carried no access_token", driver.ErrMint)
	}
	return payload.AccessToken, time.Duration(payload.ExpiresIn) * time.Second, nil
}

// oauthErrorCode pulls the RFC 6749 error code out of a failed response, or
// reports that there was none. Only the code is returned — never the
// description, which is server prose this package has no reason to relay.
func oauthErrorCode(body []byte) string {
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Error == "" {
		return "no error code in response"
	}
	return payload.Error
}
