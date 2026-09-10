package omni

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// frameWatch ends a stream attempt that has decoded no frame for longer than its
// timeout.
//
// It is the driver's own liveness bound, one level above [omnigent.Client.Stream]'s
// byte monitor. That monitor resets on any bytes a read delivers, and a keepalive
// comment or a partial write counts, so an intermediary that keeps traffic moving
// without ever completing a frame holds it satisfied indefinitely. This one is fed
// only from the consume loop, once per decoded frame, so the same silence is
// measured against what the driver can actually act on.
//
// Same shape as the SDK's monitor, for the same reason: a timer is never re-armed
// on arrival, because time.Timer.Reset does not un-run a callback that has already
// started, and a frame landing as the timer expires would then cancel a healthy
// stream. An arrival only records a reading of the monotonic clock; the callback is
// the only thing that arms the timer, and it re-reads that record and fires only if
// the silence really is older than the timeout.
type frameWatch struct {
	timeout time.Duration
	cancel  context.CancelFunc
	start   time.Time

	// last is time.Since(start) at the most recent frame, stored as nanoseconds.
	// fired records that this watch is what ended the attempt; it is set before
	// cancel so the consume loop can read it once the stream reports the error.
	last  atomic.Int64
	fired atomic.Bool

	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
}

func newFrameWatch(timeout time.Duration, cancel context.CancelFunc) *frameWatch {
	w := &frameWatch{timeout: timeout, cancel: cancel, start: time.Now()}
	w.mu.Lock()
	w.timer = time.AfterFunc(timeout, w.check)
	w.mu.Unlock()
	return w
}

// observe records that a frame was decoded now.
func (w *frameWatch) observe() { w.last.Store(int64(time.Since(w.start))) }

// expired reports whether the watch ended the attempt.
func (w *frameWatch) expired() bool { return w.fired.Load() }

func (w *frameWatch) check() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	silence := time.Since(w.start) - time.Duration(w.last.Load())
	if silence >= w.timeout {
		w.fired.Store(true)
		w.cancel()
		return
	}
	w.timer = time.AfterFunc(w.timeout-silence, w.check)
}

func (w *frameWatch) stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
	if w.timer != nil {
		w.timer.Stop()
	}
}
