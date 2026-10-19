package industrialscraper

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// fakeConn records the calls Session makes so each reconnect path can be
// asserted without a live device. When healOnConnect is set, a successful
// Connect clears pingErr, modelling a connection that recovers after reconnect.
type fakeConn struct {
	pingErr       error
	connectErr    error
	closeErr      error
	healOnConnect bool

	pings    int
	connects int
	closes   int
	order    []string // call sequence, so close-before-reconnect is checkable
}

func (f *fakeConn) Connect(context.Context) error {
	f.connects++
	f.order = append(f.order, "connect")
	if f.connectErr == nil && f.healOnConnect {
		f.pingErr = nil
	}
	return f.connectErr
}

func (f *fakeConn) Ping(context.Context) error {
	f.pings++
	f.order = append(f.order, "ping")
	return f.pingErr
}

func (f *fakeConn) Close(context.Context) error {
	f.closes++
	f.order = append(f.order, "close")
	return f.closeErr
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

func TestSessionEnsureLive(t *testing.T) {
	t.Run("ping ok, no reconnect", func(t *testing.T) {
		fc := &fakeConn{}
		s := NewSession(fc)
		if err := s.EnsureLive(context.Background()); err != nil {
			t.Fatalf("EnsureLive returned %v, want nil", err)
		}
		if fc.pings != 1 {
			t.Errorf("pings = %d, want 1", fc.pings)
		}
		if fc.connects != 0 {
			t.Errorf("connects = %d, want 0 (healthy ping must not reconnect)", fc.connects)
		}
	})

	t.Run("ping fails, reconnect closes then succeeds", func(t *testing.T) {
		fc := &fakeConn{pingErr: errors.New("socket dropped"), healOnConnect: true}
		s := NewSession(fc)
		if err := s.EnsureLive(context.Background()); err != nil {
			t.Fatalf("EnsureLive returned %v, want nil after successful reconnect", err)
		}
		if fc.closes != 1 {
			t.Errorf("closes = %d, want 1 (dead conn must be closed before reconnect)", fc.closes)
		}
		if fc.connects != 1 {
			t.Errorf("connects = %d, want 1", fc.connects)
		}
		if want := []string{"ping", "close", "connect", "ping"}; !equalStrs(fc.order, want) {
			t.Errorf("call order = %v, want %v (close before connect, verify ping after)", fc.order, want)
		}
	})

	t.Run("ping fails, reconnect fails, joins both errors", func(t *testing.T) {
		pingErr := errors.New("socket dropped")
		connErr := errors.New("device unreachable")
		fc := &fakeConn{pingErr: pingErr, connectErr: connErr}
		s := NewSession(fc)
		err := s.EnsureLive(context.Background())
		if !errors.Is(err, connErr) {
			t.Errorf("EnsureLive error %v does not wrap connect error %v", err, connErr)
		}
		if !errors.Is(err, pingErr) {
			t.Errorf("EnsureLive error %v drops the original ping error %v", err, pingErr)
		}
	})
}

func TestSessionClose(t *testing.T) {
	closeErr := errors.New("close boom")
	fc := &fakeConn{closeErr: closeErr}
	s := NewSession(fc)
	if err := s.Close(context.Background()); !errors.Is(err, closeErr) {
		t.Fatalf("Close returned %v, want %v", err, closeErr)
	}
	if fc.closes != 1 {
		t.Errorf("closes = %d, want 1", fc.closes)
	}
}

// TestSessionConcurrent exercises EnsureLive and Close from several goroutines.
// Run with -race: without a mutex guarding the Session, the concurrent reconnect
// path races on the shared connection state.
func TestSessionConcurrent(t *testing.T) {
	fc := &fakeConn{pingErr: errors.New("down")}
	s := NewSession(fc)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.EnsureLive(context.Background())
			_ = s.Close(context.Background())
		}()
	}
	wg.Wait()
}
