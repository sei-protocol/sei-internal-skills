package omni

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
)

// The retry a request that never reached a server gets.
//
// Short, because a reset is returned immediately and the run is holding a sandbox
// while this sleeps. The attempt count is derived from the backoff table rather
// than written beside it: the two have to agree, and a hand-written 3 makes
// raising one an index panic in the other. Three today, because the failure this
// absorbs is a single reset rather than an outage.
var transportBackoff = [...]time.Duration{500 * time.Millisecond, 2 * time.Second}

const transportAttempts = len(transportBackoff) + 1

// retryUnreached runs op until it succeeds, fails for a reason retryable rejects,
// or runs out of attempts.
//
// The caller's deadline, not this loop's, decides when to stop waiting. The op's
// own error is returned rather than the context's, so the reason the run failed
// stays in the message.
//
// The op must be safe to run again for every error retryable admits: a request
// that was written and lost its response is one of them, so a caller whose op
// changes state gets at-least-once delivery.
func retryUnreached(ctx context.Context, retryable func(error) bool, op func() error) error {
	for attempt := 1; ; attempt++ {
		err := op()
		if err == nil || attempt == transportAttempts || !retryable(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(transportBackoff[attempt-1]):
		}
	}
}

// errWalkExpired marks a listing that spent its own walk budget while the
// caller's still had time.
var errWalkExpired = errors.New("the session listing spent its walk budget")

// lookupUnreached reports a run-key walk that got no answer, by either shape the
// blackhole takes: the reset that returns at once, and the hang that returns
// when the walk's own budget runs out. Only the first is a transport error --
// the second arrives as the walk context's deadline, which [transportFailed]
// rejects on its face because a caller's spent deadline is not retryable.
func lookupUnreached(err error) bool {
	return transportFailed(err) || errors.Is(err, errWalkExpired)
}

// transportFailed reports a request that never got an answer: it failed below
// HTTP, so nothing is known about whether the server acted, and another attempt
// can land where this one did not.
//
// A response the server sent is not one, whatever its status — the SDK carries
// that as an [omnigent.APIError], and a retry cannot change a refusal. Neither is
// a spent context: another attempt on it only fails sooner.
func transportFailed(err error) bool {
	var apiErr *omnigent.APIError
	if err == nil || errors.As(err, &apiErr) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// gatewayUnavailable reports an answer that no omnigent handler wrote: a 502, 503
// or 504 that carries no error envelope.
//
// That is a gateway or load balancer speaking for a server it cannot reach, which
// in practice is the server being replaced, and it is the one HTTP answer a retry
// can change. It does not weaken [transportFailed]'s rule that a response the
// server sent is final. Every refusal the server's handlers write carries an
// envelope with a code or a message, so a 503 such as runner_unavailable fails
// this test and stays final.
//
// The request id is deliberately not part of the test. This driver sends
// X-Request-Id on every request (see transport.go), and a gateway configured to
// echo it puts one on its own 503, so its presence proves nothing about who wrote
// the response.
func gatewayUnavailable(err error) bool {
	var apiErr *omnigent.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
	default:
		return false
	}
	return apiErr.Code == "" && apiErr.Title == "" && apiErr.Message == "" && len(apiErr.Detail) == 0
}

// serverUnreachable reports a read that neither the server nor anything speaking
// for it answered: a transport failure, or a gateway reporting the server gone.
func serverUnreachable(err error) bool {
	return transportFailed(err) || gatewayUnavailable(err)
}

// restartPolls is how many times [retryServerRestart] asks across one budget.
//
// A fixed cadence, not a growing one. What ends the wait is a new server coming
// up, which happens once and at no predictable point, so polling evenly finds it
// no later than one interval after it happens. Sixteen reads a run is a load the
// returning server does not notice.
const restartPolls = 16

// minRestartPoll floors the interval, so a small budget cannot spin on the server.
const minRestartPoll = 50 * time.Millisecond

// retryServerRestart runs op until it succeeds, fails for a reason other than an
// unreachable server, or the budget runs out. It calls onWait before each wait.
//
// Only for a read that is safe to repeat. The retry exists because the run holds a
// live sandbox and a turn that survives the restart; waiting out the restart is
// what lets the run collect that turn's answer.
//
// The caller's deadline still bounds the whole wait. A wait that the deadline or
// a cancellation ends returns an error wrapping ctx.Err() as well as the op's
// last error. The context error is what the run's exit code classifies on: a
// stopped run must report a timeout or a cancellation, not a transport fault
// that invites a re-run. The op's error keeps the reason in the message.
func retryServerRestart(ctx context.Context, budget time.Duration, onWait func(error), op func() error) error {
	interval := max(budget/restartPolls, minRestartPoll)
	giveUp := time.Now().Add(budget)
	for {
		err := op()
		if err == nil || !serverUnreachable(err) || time.Now().Add(interval).After(giveUp) {
			return err
		}
		onWait(err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w while waiting out a server restart: %w", ctx.Err(), err)
		case <-time.After(interval):
		}
	}
}
