package industrialscraper

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
)

// fakeConn counts the calls Session makes and fails Connect while connectErr
// is set.
type fakeConn struct {
	connectErr error
	closeErr   error
	connects   int
	closes     int
}

func (f *fakeConn) Connect(context.Context) error {
	f.connects++
	return f.connectErr
}

func (f *fakeConn) Close(context.Context) error {
	f.closes++
	return f.closeErr
}

// testBackOff is a jitter-free backoff: 10s, doubling, capped at 40s.
func testBackOff() configretry.BackOffConfig {
	c := DefaultBackOffConfig()
	c.InitialInterval = 10 * time.Second
	c.MaxInterval = 40 * time.Second
	c.Multiplier = 2
	c.RandomizationFactor = 0
	return c
}

// newTestSession returns a session on a fake clock and a func to advance it.
func newTestSession(fc *fakeConn, b configretry.BackOffConfig) (*Session, func(time.Duration)) {
	fclock := time.Unix(1_000_000, 0)
	s := NewSession(fc, b)
	s.now = func() time.Time { return fclock }
	return s, func(d time.Duration) { fclock = fclock.Add(d) }
}

func TestSessionEnsureLiveConnectsOnce(t *testing.T) {
	fc := &fakeConn{}
	s, _ := newTestSession(fc, testBackOff())
	for range 3 {
		if err := s.EnsureLive(context.Background()); err != nil {
			t.Fatalf("EnsureLive returned %v, want nil", err)
		}
	}
	if fc.connects != 1 {
		t.Errorf("connects = %d, want 1 (an open connection is reused)", fc.connects)
	}
}

func TestSessionCloseThenReconnect(t *testing.T) {
	fc := &fakeConn{}
	s, _ := newTestSession(fc, testBackOff())
	ctx := context.Background()
	if err := s.EnsureLive(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureLive(ctx); err != nil {
		t.Fatalf("EnsureLive after Close returned %v, want nil", err)
	}
	if fc.closes != 1 || fc.connects != 2 {
		t.Errorf("closes = %d, connects = %d, want 1 and 2 (Close drops the connection, next EnsureLive redials at once)", fc.closes, fc.connects)
	}
}

func TestSessionCloseReturnsConnError(t *testing.T) {
	closeErr := errors.New("close boom")
	fc := &fakeConn{closeErr: closeErr}
	s, _ := newTestSession(fc, testBackOff())
	if err := s.Close(context.Background()); !errors.Is(err, closeErr) {
		t.Fatalf("Close returned %v, want %v", err, closeErr)
	}
}

func TestSessionBackoff(t *testing.T) {
	dialErr := errors.New("connection refused")
	fc := &fakeConn{connectErr: dialErr}
	s, advance := newTestSession(fc, testBackOff())
	ctx := context.Background()

	if err := s.EnsureLive(ctx); !errors.Is(err, dialErr) {
		t.Fatalf("first EnsureLive returned %v, want %v", err, dialErr)
	}

	// Each failure doubles the wait: 10s, 20s, then capped at 40s.
	for _, wait := range []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second, 40 * time.Second} {
		advance(wait - time.Second)
		err := s.EnsureLive(ctx)
		if !errors.Is(err, ErrBackingOff) || !errors.Is(err, dialErr) {
			t.Fatalf("EnsureLive %v into a %v window returned %v, want ErrBackingOff wrapping the last dial error", wait-time.Second, wait, err)
		}
		dials := fc.connects
		advance(time.Second)
		if err := s.EnsureLive(ctx); !errors.Is(err, dialErr) {
			t.Fatalf("EnsureLive at the end of a %v window returned %v, want a fresh dial error", wait, err)
		}
		if fc.connects != dials+1 {
			t.Fatalf("connects = %d, want %d (one dial once the window ends)", fc.connects, dials+1)
		}
	}

	// A successful connect resets the backoff.
	fc.connectErr = nil
	advance(40 * time.Second)
	if err := s.EnsureLive(ctx); err != nil {
		t.Fatalf("EnsureLive returned %v, want nil once the device is back", err)
	}
	_ = s.Close(ctx)
	fc.connectErr = dialErr
	_ = s.EnsureLive(ctx)
	advance(10 * time.Second)
	dials := fc.connects
	_ = s.EnsureLive(ctx)
	if fc.connects != dials+1 {
		t.Errorf("after a success the next window should be InitialInterval (10s) again")
	}
}

// MaxElapsedTime never stops reconnects: once a failure streak outlasts it,
// every later wait is MaxInterval.
func TestSessionBackoffMaxElapsedHoldsAtMaxInterval(t *testing.T) {
	fc := &fakeConn{connectErr: errors.New("down")}
	b := testBackOff()
	b.Multiplier = 1 // no growth, so only MaxElapsedTime can raise the wait
	b.MaxElapsedTime = 30 * time.Second
	s, advance := newTestSession(fc, b)
	ctx := context.Background()

	_ = s.EnsureLive(ctx) // streak starts; next wait 10s
	for _, wait := range []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second, 40 * time.Second, 40 * time.Second} {
		advance(wait - time.Second)
		if err := s.EnsureLive(ctx); !errors.Is(err, ErrBackingOff) {
			t.Fatalf("expected a %v window, but EnsureLive dialed early (err %v)", wait, err)
		}
		advance(time.Second)
		dials := fc.connects
		_ = s.EnsureLive(ctx)
		if fc.connects != dials+1 {
			t.Fatalf("expected a dial at the end of a %v window", wait)
		}
	}
}

func TestSessionBackoffDisabledRedialsEveryTime(t *testing.T) {
	fc := &fakeConn{connectErr: errors.New("down")}
	b := testBackOff()
	b.Enabled = false
	s, _ := newTestSession(fc, b)
	for range 3 {
		_ = s.EnsureLive(context.Background())
	}
	if fc.connects != 3 {
		t.Errorf("connects = %d, want 3 (backoff disabled dials on every call)", fc.connects)
	}
}

func TestSessionEnsureLiveCancelledContext(t *testing.T) {
	fc := &fakeConn{}
	s, _ := newTestSession(fc, testBackOff())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.EnsureLive(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("EnsureLive returned %v, want context.Canceled", err)
	}
	if fc.connects != 0 {
		t.Errorf("connects = %d, want 0 (no dial on a cancelled scrape)", fc.connects)
	}
}

func TestJitterWithinFactor(t *testing.T) {
	for range 1000 {
		got := jitter(10*time.Second, 0.5)
		if got < 5*time.Second || got > 15*time.Second {
			t.Fatalf("jitter(10s, 0.5) = %v, want within [5s, 15s]", got)
		}
	}
	if got := jitter(10*time.Second, 0); got != 10*time.Second {
		t.Errorf("jitter(10s, 0) = %v, want 10s", got)
	}
}

// TestSessionConcurrent exercises EnsureLive and Close from several goroutines.
// Run with -race.
func TestSessionConcurrent(t *testing.T) {
	fc := &fakeConn{connectErr: errors.New("down")}
	s := NewSession(fc, testBackOff())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_ = s.EnsureLive(context.Background())
			_ = s.Close(context.Background())
		})
	}
	wg.Wait()
}
