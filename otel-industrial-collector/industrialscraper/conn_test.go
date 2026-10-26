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

// pingConn is a fakeConn that also implements Pinger and records the call
// order. A successful Connect sets pingErr to pingErrAfterConnect.
type pingConn struct {
	fakeConn
	pingErr             error
	pingErrAfterConnect error
	pings               int
	order               []string
}

func (p *pingConn) Connect(ctx context.Context) error {
	p.order = append(p.order, "connect")
	err := p.fakeConn.Connect(ctx)
	if err == nil {
		p.pingErr = p.pingErrAfterConnect
	}
	return err
}

func (p *pingConn) Close(ctx context.Context) error {
	p.order = append(p.order, "close")
	return p.fakeConn.Close(ctx)
}

func (p *pingConn) Ping(context.Context) error {
	p.pings++
	p.order = append(p.order, "ping")
	return p.pingErr
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

// Backoff on an open connection (a device reporting it is busy) keeps the
// connection, holds off polls for the window, and grows while the device stays
// busy. One clean poll after the window resets it.
func TestSessionBackoffWhileConnected(t *testing.T) {
	fc := &fakeConn{}
	s, advance := newTestSession(fc, testBackOff())
	ctx := context.Background()
	busy := errors.New("server device busy")
	if err := s.EnsureLive(ctx); err != nil {
		t.Fatal(err)
	}

	// expectWindow checks EnsureLive is held off for exactly d, then passes.
	expectWindow := func(d time.Duration) {
		t.Helper()
		advance(d - time.Second)
		if err := s.EnsureLive(ctx); !errors.Is(err, ErrBackingOff) || !errors.Is(err, busy) {
			t.Fatalf("EnsureLive inside a %v window returned %v, want ErrBackingOff wrapping %v", d, err, busy)
		}
		advance(time.Second)
		if err := s.EnsureLive(ctx); err != nil {
			t.Fatalf("EnsureLive after a %v window returned %v, want nil", d, err)
		}
	}

	s.Backoff(busy)
	expectWindow(10 * time.Second)
	s.Backoff(busy) // still busy on the first poll after the window
	expectWindow(20 * time.Second)
	if err := s.EnsureLive(ctx); err != nil { // a clean poll in between resets
		t.Fatal(err)
	}
	s.Backoff(busy)
	expectWindow(10 * time.Second)

	if fc.connects != 1 || fc.closes != 0 {
		t.Errorf("connects = %d, closes = %d, want 1 and 0 (busy keeps the connection)", fc.connects, fc.closes)
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

// The first EnsureLive only connects: a client with no connection is never
// pinged (a nil-connection ping panicked the EtherNet/IP receiver).
func TestSessionPingNotBeforeFirstConnect(t *testing.T) {
	pc := &pingConn{}
	s, _ := newTestSession(&pc.fakeConn, testBackOff())
	s.conn = pc
	if err := s.EnsureLive(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"connect"}; !equalStrs(pc.order, want) {
		t.Errorf("call order = %v, want %v", pc.order, want)
	}
}

func TestSessionPing(t *testing.T) {
	// connected returns a session with an open connection to pc.
	connected := func(t *testing.T, pc *pingConn) (*Session, func(time.Duration)) {
		t.Helper()
		s, advance := newTestSession(&pc.fakeConn, testBackOff())
		s.conn = pc
		if err := s.EnsureLive(context.Background()); err != nil {
			t.Fatal(err)
		}
		pc.order = nil
		return s, advance
	}
	ctx := context.Background()

	t.Run("ping ok, no reconnect", func(t *testing.T) {
		pc := &pingConn{}
		s, _ := connected(t, pc)
		if err := s.EnsureLive(ctx); err != nil {
			t.Fatalf("EnsureLive returned %v, want nil", err)
		}
		if want := []string{"ping"}; !equalStrs(pc.order, want) {
			t.Errorf("call order = %v, want %v", pc.order, want)
		}
	})

	t.Run("ping fails, close then reconnect and verify", func(t *testing.T) {
		pc := &pingConn{}
		s, _ := connected(t, pc)
		pc.pingErr = errors.New("socket dropped")
		if err := s.EnsureLive(ctx); err != nil {
			t.Fatalf("EnsureLive returned %v, want nil after a successful reconnect", err)
		}
		if want := []string{"ping", "close", "connect", "ping"}; !equalStrs(pc.order, want) {
			t.Errorf("call order = %v, want %v (close before connect, verify ping after)", pc.order, want)
		}
	})

	t.Run("ping fails, reconnect fails, joins both and backs off", func(t *testing.T) {
		pc := &pingConn{}
		s, _ := connected(t, pc)
		pingErr := errors.New("socket dropped")
		connErr := errors.New("device unreachable")
		pc.pingErr, pc.connectErr = pingErr, connErr
		err := s.EnsureLive(ctx)
		if !errors.Is(err, pingErr) || !errors.Is(err, connErr) {
			t.Fatalf("EnsureLive returned %v, want it to wrap %v and %v", err, pingErr, connErr)
		}
		if err := s.EnsureLive(ctx); !errors.Is(err, ErrBackingOff) {
			t.Errorf("next EnsureLive returned %v, want ErrBackingOff", err)
		}
	})

	t.Run("reconnect ok, re-ping fails, joins both and backs off", func(t *testing.T) {
		pc := &pingConn{}
		s, _ := connected(t, pc)
		pingErr := errors.New("socket dropped")
		repingErr := errors.New("still no response")
		pc.pingErr, pc.pingErrAfterConnect = pingErr, repingErr
		err := s.EnsureLive(ctx)
		if !errors.Is(err, pingErr) || !errors.Is(err, repingErr) {
			t.Fatalf("EnsureLive returned %v, want it to wrap %v and %v", err, pingErr, repingErr)
		}
		if pc.closes != 2 {
			t.Errorf("closes = %d, want 2 (the dead connection and the unverified one)", pc.closes)
		}
		dials := pc.connects
		if err := s.EnsureLive(ctx); !errors.Is(err, ErrBackingOff) {
			t.Errorf("next EnsureLive returned %v, want ErrBackingOff", err)
		}
		if pc.connects != dials {
			t.Errorf("dialed inside the backoff window")
		}
	})
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
