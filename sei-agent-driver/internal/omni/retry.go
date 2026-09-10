package omni

import (
	"context"
	"errors"
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
