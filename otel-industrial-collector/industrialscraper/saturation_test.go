package industrialscraper

import (
	"context"
	"testing"
)

func TestDefaultBackOffConfigIsValid(t *testing.T) {
	cfg := DefaultBackOffConfig()
	if !cfg.Enabled {
		t.Error("retry should be enabled by default")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("default backoff config is invalid: %v", err)
	}
	if cfg.MaxElapsedTime == 0 {
		t.Error("MaxElapsedTime should be finite so retries cannot run unbounded against a PLC")
	}
}

func TestLimiterCapsConcurrency(t *testing.T) {
	l := NewLimiter(2)
	ctx := context.Background()

	if err := l.Acquire(ctx); err != nil {
		t.Fatalf("Acquire 1: %v", err)
	}
	if err := l.Acquire(ctx); err != nil {
		t.Fatalf("Acquire 2: %v", err)
	}

	// Third acquire must block until a slot is released. Prove it blocks by
	// cancelling the context and expecting the cancellation error back.
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := l.Acquire(cancelCtx); err == nil {
		t.Error("third Acquire should have been blocked and returned the cancellation error")
	}

	// After releasing one slot, an acquire succeeds again.
	l.Release()
	if err := l.Acquire(ctx); err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
}

func TestLimiterZeroIsUnlimited(t *testing.T) {
	l := NewLimiter(0)
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		if err := l.Acquire(ctx); err != nil {
			t.Fatalf("unlimited Acquire %d: %v", i, err)
		}
	}
	l.Release() // must not panic on an unlimited limiter
}
