package industrialscraper

import (
	"context"
	"errors"
	"sync"
)

// Conn is one field-device connection, implemented by a receiver over its
// own client. See README.
type Conn interface {
	// Connect establishes or re-establishes the connection; safe to re-call.
	Connect(ctx context.Context) error
	// Ping is a real round-trip liveness check, not a cached flag.
	Ping(ctx context.Context) error
	// Close releases the connection; safe to call when not open.
	Close(ctx context.Context) error
}

// Session adds reconnect-on-failure over a Conn. Safe for concurrent use.
type Session struct {
	mu   sync.Mutex
	conn Conn
}

func NewSession(conn Conn) *Session {
	return &Session{conn: conn}
}

// EnsureLive pings and, on failure, closes the dead connection, reconnects, and
// re-pings. Closing first prevents leaking the old socket (and the device
// connection slot it holds). A non-nil result means the device is unreachable
// this cycle; it wraps both the original ping failure and any reconnect error.
// See README.
func (s *Session) EnsureLive(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pingErr := s.conn.Ping(ctx)
	if pingErr == nil {
		return nil
	}
	_ = s.conn.Close(ctx)
	if err := s.conn.Connect(ctx); err != nil {
		return errors.Join(pingErr, err)
	}
	if err := s.conn.Ping(ctx); err != nil {
		return errors.Join(pingErr, err)
	}
	return nil
}

func (s *Session) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close(ctx)
}
