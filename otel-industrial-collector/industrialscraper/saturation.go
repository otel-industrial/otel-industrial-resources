package industrialscraper

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
)

// DefaultBackOffConfig returns reconnect backoff defaults tuned for field
// devices, for Session. Legacy PLCs tolerate far less polling pressure than a
// cloud backend, so it backs off more slowly and to a higher ceiling than the
// upstream 5s/30s, giving a struggling device room to recover. Session never
// gives up on a device: after MaxElapsedTime it keeps retrying at MaxInterval.
// See README.
func DefaultBackOffConfig() configretry.BackOffConfig {
	c := configretry.NewDefaultBackOffConfig()
	c.Enabled = true
	c.InitialInterval = 10 * time.Second
	c.MaxInterval = 60 * time.Second
	c.MaxElapsedTime = 5 * time.Minute
	return c
}

// Limiter caps concurrent device operations so polling cannot overload a
// legacy PLC. Zero/negative max means unlimited. Safe for concurrent use.
type Limiter struct{ sem chan struct{} }

func NewLimiter(max int) *Limiter {
	if max <= 0 {
		return &Limiter{}
	}
	return &Limiter{sem: make(chan struct{}, max)}
}

// Acquire takes a slot, blocking until one frees or ctx is done. A cancelled
// context loses even when a slot is free, so a scrape cannot start a poll during
// shutdown.
func (l *Limiter) Acquire(ctx context.Context) error {
	if l.sem == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a slot taken by Acquire. Calling it without a matching
// successful Acquire is a bug: it panics rather than blocking forever on the
// bounded channel and hanging collector shutdown.
func (l *Limiter) Release() {
	if l.sem == nil {
		return
	}
	select {
	case <-l.sem:
	default:
		panic("industrialscraper: Limiter.Release called without a matching Acquire")
	}
}

var deviceLimiters sync.Map // device key -> *Limiter

// DeviceLimiter returns the process-wide limiter for one device, keyed by its
// address (e.g. "10.0.0.5:502"). It allows one poll at a time, so several
// receivers pointed at the same PLC take turns instead of polling it at once.
//
// The limit is fixed at one poll per device and never evicted; add a per-device
// max when a device is shown to tolerate parallel polls.
func DeviceLimiter(key string) *Limiter {
	l, _ := deviceLimiters.LoadOrStore(key, NewLimiter(1))
	return l.(*Limiter)
}
