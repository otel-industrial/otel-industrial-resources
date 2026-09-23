package industrialscraper

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
)

// DefaultBackOffConfig returns retry defaults tuned for field devices:
// enabled, with a bounded elapsed time. See README.
func DefaultBackOffConfig() configretry.BackOffConfig {
	c := configretry.NewDefaultBackOffConfig()
	c.Enabled = true
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

// Acquire takes a slot, blocking until one frees or ctx is done.
func (l *Limiter) Acquire(ctx context.Context) error {
	if l.sem == nil {
		return nil
	}
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a slot taken by Acquire.
func (l *Limiter) Release() {
	if l.sem != nil {
		<-l.sem
	}
}
