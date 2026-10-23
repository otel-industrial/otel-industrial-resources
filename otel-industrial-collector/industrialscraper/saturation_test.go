package industrialscraper

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/config/configretry"
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

// TestDefaultBackOffConfigIsTunedForFieldDevices proves the wrapper differs from
// the upstream default. If it matched upstream it would add nothing.
func TestDefaultBackOffConfigIsTunedForFieldDevices(t *testing.T) {
	cfg := DefaultBackOffConfig()
	up := configretry.NewDefaultBackOffConfig()
	if cfg.InitialInterval <= up.InitialInterval {
		t.Errorf("InitialInterval %v is not longer than upstream %v; a struggling PLC is hit as often as a cloud backend", cfg.InitialInterval, up.InitialInterval)
	}
	if cfg.MaxInterval <= up.MaxInterval {
		t.Errorf("MaxInterval %v is not longer than upstream %v", cfg.MaxInterval, up.MaxInterval)
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

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := l.Acquire(cancelCtx); err == nil {
		t.Error("third Acquire should have been blocked and returned the cancellation error")
	}

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

// TestAcquireRespectsCancelledContext: a cancelled context must lose even when a
// slot is free. The old select raced the two ready cases and acquired about half
// the time, letting the scrape poll a PLC during shutdown.
func TestAcquireRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 200; i++ {
		l := NewLimiter(1) // slot is free
		if err := l.Acquire(ctx); err == nil {
			t.Fatalf("Acquire returned nil with a cancelled context (iteration %d); must honor cancellation even with a free slot", i)
		}
	}
}

// TestReleaseWithoutAcquirePanics: Release with no matching Acquire must surface
// the misuse, not block forever on the bounded channel and hang shutdown.
func TestReleaseWithoutAcquirePanics(t *testing.T) {
	l := NewLimiter(1)
	defer func() {
		if recover() == nil {
			t.Error("Release without a matching Acquire should panic, not deadlock")
		}
	}()
	l.Release()
}
