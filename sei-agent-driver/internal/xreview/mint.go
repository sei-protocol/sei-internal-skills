package xreview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// requireEncryptedOrLocal rejects a base URL that would put the client secret on
// a network in the clear.
//
// Loopback is exempt because nothing leaves the machine. Everything else must be
// https: the secret travels in the request body, it is the durable credential
// rather than the short-lived token it buys, and the pod network it would cross
// is not guaranteed to be encrypted or segmented.
func requireEncryptedOrLocal(baseURL string) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("%w: base URL %q does not parse", ErrMint, baseURL)
	}
	host := u.Hostname()
	local := host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		host == "::1" || strings.HasPrefix(host, "127.")
	if u.Scheme != "https" && !local {
		return fmt.Errorf(
			"%w: refusing to send the client secret to %q, which is plain http to a "+
				"non-loopback host; use https", ErrMint, u.Scheme+"://"+u.Host)
	}
	return nil
}

// maxTokenBody caps the token response read. The body is three short fields, so
// anything larger is a wrong endpoint or a captive portal, and reading it
// unbounded would let whatever answered decide this process's memory use.
const maxTokenBody = 64 << 10

// ErrMint is a failure to exchange machine credentials for an access token.
var ErrMint = errors.New("token exchange")

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
func MintToken(ctx context.Context, client *http.Client, baseURL, clientID, clientSecret string) (string, error) {
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("%w: client id and secret are both required", ErrMint)
	}
	if err := requireEncryptedOrLocal(baseURL); err != nil {
		return "", err
	}

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/oauth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrMint, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Deliberately not wrapped in ErrMint, which classifies as a configuration
		// fault. Reaching the endpoint and failing is a transport problem an
		// operator should retry, not a secret they should go and fix.
		//
		// The error from Do embeds the request URL but not the body, so the
		// secret cannot reach a log through here.
		return "", fmt.Errorf("token exchange could not reach the server: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxTokenBody))
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenBody))
	if err != nil {
		return "", fmt.Errorf("%w: reading response: %w", ErrMint, err)
	}

	if resp.StatusCode != http.StatusOK {
		// The OAuth error code is safe to surface and is the one thing that says
		// what to fix — invalid_client means the id or secret is wrong,
		// unsupported_grant_type means the grant is not enabled on this server.
		// The rest of the body is withheld: a non-2xx here need not have come
		// from this API at all.
		return "", fmt.Errorf("%w: %s returned %d (%s)",
			ErrMint, endpoint, resp.StatusCode, oauthErrorCode(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("%w: response was not JSON: %w", ErrMint, err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("%w: response carried no access_token", ErrMint)
	}
	return payload.AccessToken, nil
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
