package exchange

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// WSStream is a read-only WebSocket connection that reconnects with
// exponential backoff. It also reconnects when no message arrives within
// IdleTimeout (a silently dead or deprecated endpoint) and after
// MaxLifetime (Binance drops connections after 24h).
type WSStream struct {
	URL string
	// URLFunc, if set, is called before every dial instead of using URL
	// (e.g. to obtain a fresh listen key).
	URLFunc func(ctx context.Context) (string, error)
	// Setup, if set, runs after every dial and before onConnect, e.g. to
	// send a subscription request. An error drops the connection.
	Setup       func(ctx context.Context, conn *websocket.Conn) error
	HTTPClient  *http.Client // optional
	Log         *slog.Logger
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
	IdleTimeout time.Duration
	MaxLifetime time.Duration
	// OnReconnect is called before every reconnect attempt (not the first).
	OnReconnect func()

	mu   sync.Mutex
	conn *websocket.Conn
}

// Kick drops the current connection; Run reconnects.
func (s *WSStream) Kick() {
	s.mu.Lock()
	c := s.conn
	s.mu.Unlock()
	if c != nil {
		_ = c.CloseNow()
	}
}

func (s *WSStream) withDefaults() {
	if s.MinBackoff == 0 {
		s.MinBackoff = time.Second
	}
	if s.MaxBackoff == 0 {
		s.MaxBackoff = time.Minute
	}
	if s.IdleTimeout == 0 {
		s.IdleTimeout = time.Minute
	}
	if s.MaxLifetime == 0 {
		s.MaxLifetime = 23 * time.Hour
	}
	if s.Log == nil {
		s.Log = slog.Default()
	}
}

// Run reads messages and passes each to handle until ctx is cancelled.
// onConnect runs after every successful dial. Run only returns ctx.Err().
func (s *WSStream) Run(ctx context.Context, onConnect func(), handle func([]byte)) error {
	s.withDefaults()
	backoff := s.MinBackoff
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			if s.OnReconnect != nil {
				s.OnReconnect()
			}
			// Full jitter avoids reconnect storms across connections.
			wait := time.Duration(rand.Int64N(int64(backoff))) + s.MinBackoff/2
			s.Log.Warn("websocket reconnecting", "in", wait.Round(time.Millisecond))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			backoff = min(backoff*2, s.MaxBackoff)
		}

		start := time.Now()
		err := s.session(ctx, onConnect, handle)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A connection that stayed healthy for a while resets the backoff.
		if time.Since(start) > 5*time.Minute {
			backoff = s.MinBackoff
		}
		s.Log.Warn("websocket disconnected", "err", err, "uptime", time.Since(start).Round(time.Second))
	}
}

var errLifetime = errors.New("max connection lifetime reached")

func (s *WSStream) session(ctx context.Context, onConnect func(), handle func([]byte)) error {
	url := s.URL
	if s.URLFunc != nil {
		var err error
		if url, err = s.URLFunc(ctx); err != nil {
			return err
		}
	}
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	conn, _, err := websocket.Dial(dialCtx, url, &websocket.DialOptions{HTTPClient: s.HTTPClient})
	cancel()
	if err != nil {
		return err
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(1 << 20)
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.conn = nil
		s.mu.Unlock()
	}()

	if s.Setup != nil {
		setupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := s.Setup(setupCtx, conn)
		cancel()
		if err != nil {
			return fmt.Errorf("setup: %w", err)
		}
	}

	if onConnect != nil {
		onConnect()
	}

	lifetime, cancelLifetime := context.WithTimeoutCause(ctx, s.MaxLifetime, errLifetime)
	defer cancelLifetime()
	for {
		readCtx, cancelRead := context.WithTimeoutCause(lifetime, s.IdleTimeout, errors.New("idle timeout"))
		_, msg, err := conn.Read(readCtx)
		cause := context.Cause(readCtx)
		cancelRead()
		if err != nil {
			if cause != nil {
				return cause
			}
			return err
		}
		handle(msg)
	}
}
