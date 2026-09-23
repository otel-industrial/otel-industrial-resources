package industrialscraper

import (
	"context"
	"errors"
	"testing"
)

// fakeConn records the calls Session makes so each reconnect path can be
// asserted without a live device.
type fakeConn struct {
	pingErr    error
	connectErr error
	closeErr   error

	pings    int
	connects int
	closes   int
}

func (f *fakeConn) Connect(context.Context) error {
	f.connects++
	return f.connectErr
}

func (f *fakeConn) Ping(context.Context) error {
	f.pings++
	return f.pingErr
}

func (f *fakeConn) Close(context.Context) error {
	f.closes++
	return f.closeErr
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

	t.Run("ping fails, reconnect succeeds", func(t *testing.T) {
		fc := &fakeConn{pingErr: errors.New("socket dropped")}
		s := NewSession(fc)
		if err := s.EnsureLive(context.Background()); err != nil {
			t.Fatalf("EnsureLive returned %v, want nil after successful reconnect", err)
		}
		if fc.pings != 1 {
			t.Errorf("pings = %d, want 1", fc.pings)
		}
		if fc.connects != 1 {
			t.Errorf("connects = %d, want 1", fc.connects)
		}
	})

	t.Run("ping fails, reconnect fails", func(t *testing.T) {
		connErr := errors.New("device unreachable")
		fc := &fakeConn{pingErr: errors.New("socket dropped"), connectErr: connErr}
		s := NewSession(fc)
		err := s.EnsureLive(context.Background())
		if !errors.Is(err, connErr) {
			t.Fatalf("EnsureLive returned %v, want %v", err, connErr)
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
