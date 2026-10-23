package industrialscraper

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
)

// DefaultBackOffConfig returns retry defaults tuned for field devices. Legacy
// PLCs tolerate far less polling pressure than a cloud backend, so it backs off
// more slowly and to a higher ceiling than the upstream 5s/30s, giving a
// struggling device room to recover while staying bounded by MaxElapsedTime.
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
