package industrialscraper

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
)

// Conn is one field-device connection, implemented by a receiver over its
// own client. See README.
type Conn interface {
	// Connect opens the connection.
	Connect(ctx context.Context) error
	// Close releases the connection; safe to call when not open.
	Close(ctx context.Context) error
}

// Pinger is implemented by a Conn whose protocol has a cheap liveness
// round-trip (e.g. EtherNet/IP ListIdentity). Session then checks an open
// connection on every EnsureLive, so a socket that dropped silently between
// polls is replaced before the reads. Ping must be a real round-trip, not a
// cached flag.
type Pinger interface {
	Ping(ctx context.Context) error
}

// ErrBackingOff is returned by EnsureLive while a failed device is inside its
// reconnect backoff window and no dial was attempted. The returned error also
// wraps the last dial error.
var ErrBackingOff = errors.New("industrialscraper: reconnect backing off")

// Session tracks one device connection: it connects lazily, and after a failed
// connect it waits out a configretry backoff before dialing the device again,
// so a struggling PLC is not hammered with reconnects. Safe for concurrent use.
type Session struct {
	mu      sync.Mutex
	conn    Conn
	backoff configretry.BackOffConfig
	now     func() time.Time

	connected   bool
	lastErr     error
	streakStart time.Time     // first failure of the current failure streak
	interval    time.Duration // current backoff interval; 0 = no streak
	nextAttempt time.Time
}

func NewSession(conn Conn, backoff configretry.BackOffConfig) *Session {
	return &Session{conn: conn, backoff: backoff, now: time.Now}
}

// EnsureLive connects if there is no open connection. If the Conn is a Pinger,
// an open connection is pinged first; when the ping fails it is closed,
// reconnected, and pinged again, and a failure wraps both errors. Inside a
// backoff window it returns ErrBackingOff without touching the device. A
// transport error seen while reading is reported by calling Close, so the next
// EnsureLive redials.
func (s *Session) EnsureLive(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return s.connectLocked(ctx)
	}
	p, canPing := s.conn.(Pinger)
	if !canPing {
		return nil
	}
	pingErr := p.Ping(ctx)
	if pingErr == nil {
		return nil
	}
	_ = s.conn.Close(ctx)
	s.connected = false
	if err := s.connectLocked(ctx); err != nil {
		return errors.Join(pingErr, err)
	}
	if err := p.Ping(ctx); err != nil {
		_ = s.conn.Close(ctx)
		s.connected = false
		s.lastErr = err
		s.scheduleRetry()
		return errors.Join(pingErr, err)
	}
	return nil
}

// connectLocked dials unless inside a backoff window. s.mu must be held.
func (s *Session) connectLocked(ctx context.Context) error {
	if s.now().Before(s.nextAttempt) {
		return errors.Join(ErrBackingOff, s.lastErr)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.conn.Connect(ctx); err != nil {
		s.lastErr = err
		s.scheduleRetry()
		return err
	}
	s.connected = true
	s.lastErr, s.interval, s.nextAttempt = nil, 0, time.Time{}
	return nil
}

// scheduleRetry advances the backoff after a failed connect. MaxElapsedTime
// never stops reconnects: once a failure streak outlasts it, every wait is
// MaxInterval. s.mu must be held.
func (s *Session) scheduleRetry() {
	if !s.backoff.Enabled {
		return
	}
	now := s.now()
	switch {
	case s.interval == 0:
		s.streakStart = now
		s.interval = s.backoff.InitialInterval
	case s.backoff.MaxElapsedTime > 0 && now.Sub(s.streakStart) >= s.backoff.MaxElapsedTime:
		s.interval = s.backoff.MaxInterval
	default:
		s.interval = min(time.Duration(float64(s.interval)*s.backoff.Multiplier), s.backoff.MaxInterval)
	}
	s.nextAttempt = now.Add(jitter(s.interval, s.backoff.RandomizationFactor))
}

// jitter spreads d by ±factor so many receivers do not redial in lockstep.
func jitter(d time.Duration, factor float64) time.Duration {
	return time.Duration(float64(d) * (1 + factor*(2*rand.Float64()-1)))
}

// Close closes the connection; the next EnsureLive reconnects. Call it after a
// transport error and on shutdown (it matches scraper.WithShutdown).
func (s *Session) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = false
	return s.conn.Close(ctx)
}
