package exchange

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// WeightLimiter enforces a request-weight budget per minute, as used by
// Binance, and can be paused when the exchange returns Retry-After.
type WeightLimiter struct {
	lim *rate.Limiter

	mu          sync.Mutex
	pausedUntil time.Time
}

// NewWeightLimiter allows perMinute weight per minute, with bursts up to
// the full minute budget.
func NewWeightLimiter(perMinute int) *WeightLimiter {
	return &WeightLimiter{lim: rate.NewLimiter(rate.Limit(float64(perMinute)/60), perMinute)}
}

// Wait blocks until weight can be spent or ctx is done.
func (l *WeightLimiter) Wait(ctx context.Context, weight int) error {
	l.mu.Lock()
	pause := time.Until(l.pausedUntil)
	l.mu.Unlock()
	if pause > 0 {
		t := time.NewTimer(pause)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return l.lim.WaitN(ctx, weight)
}

// Pause blocks all callers for d.
func (l *WeightLimiter) Pause(d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if until := time.Now().Add(d); until.After(l.pausedUntil) {
		l.pausedUntil = until
	}
}
