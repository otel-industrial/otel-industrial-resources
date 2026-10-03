package industrialscraper

import "context"

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

// Session adds reconnect-on-failure over a Conn.
type Session struct {
	conn Conn
}

func NewSession(conn Conn) *Session {
	return &Session{conn: conn}
}

// EnsureLive pings and, on failure, reconnects once. Non-nil means the device
// is unreachable this cycle; see README.
func (s *Session) EnsureLive(ctx context.Context) error {
	if err := s.conn.Ping(ctx); err == nil {
		return nil
	}
	return s.conn.Connect(ctx)
}

func (s *Session) Close(ctx context.Context) error {
	return s.conn.Close(ctx)
}
