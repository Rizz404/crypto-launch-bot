// Package recorder persists engine activity asynchronously, so that engine
// callbacks (which run under the engine lock) never wait on the database.
package recorder

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/storage"
)

const queueSize = 10_000

// Timing of writes; variables so tests can shorten them.
var (
	writeTimeout = 10 * time.Second
	flushTimeout = 10 * time.Second
	retryMin     = time.Second
	retryMax     = 30 * time.Second
)

type job struct {
	name string
	fn   func(ctx context.Context) error
	// idempotent writes may be repeated after an ambiguous failure (one
	// that may have committed); others only when nothing reached the server.
	idempotent bool
}

// Recorder writes orders, fills, trades, equity and positions of one
// session run in the order they were recorded.
type Recorder struct {
	store *storage.Store
	runID int64
	log   *slog.Logger
	jobs  chan job
	done  chan struct{}
}

func New(store *storage.Store, runID int64, log *slog.Logger) *Recorder {
	return &Recorder{store: store, runID: runID, log: log, jobs: make(chan job, queueSize), done: make(chan struct{})}
}

// RunID is the session run the recorder writes to.
func (r *Recorder) RunID() int64 { return r.runID }

// Run processes jobs until ctx is cancelled, then flushes what is queued
// (for at most flushTimeout). A write that fails transiently, for example
// while the database is unreachable, is retried with backoff before the
// next one, so later writes (fills after their order) keep their order.
func (r *Recorder) Run(ctx context.Context) {
	defer close(r.done)
	var pending *job
	for pending == nil && ctx.Err() == nil {
		select {
		case j := <-r.jobs:
			// Shutdown must not interrupt a write already removed from the
			// queue; otherwise it is lost before the flush can see it.
			if !r.write(context.WithoutCancel(ctx), ctx, j) {
				pending = &j
			}
		case <-ctx.Done():
		}
	}
	flushCtx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if pending != nil && !r.write(flushCtx, flushCtx, *pending) {
		r.log.Error("recorder flush timed out, dropping writes", "count", len(r.jobs)+1)
		return
	}
	for {
		select {
		case j := <-r.jobs:
			if !r.write(flushCtx, flushCtx, j) {
				r.log.Error("recorder flush timed out, dropping writes", "count", len(r.jobs)+1)
				return
			}
		default:
			return
		}
	}
}

// Wait blocks until Run has flushed and returned.
func (r *Recorder) Wait() { <-r.done }

// write runs j until it succeeds or fails permanently, each attempt bounded
// by writeTimeout under base. It returns false when stop ends before that.
func (r *Recorder) write(base, stop context.Context, j job) bool {
	delay := retryMin
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(base, writeTimeout)
		err := j.fn(ctx)
		cancel()
		switch {
		case err == nil:
			if attempt > 1 {
				r.log.Info("recorder write recovered", "what", j.name, "attempts", attempt)
			}
			return true
		case !retryable(err, j.idempotent):
			r.log.Error("recorder write failed", "what", j.name, "err", err)
			return true
		case stop.Err() != nil:
			return false
		}
		r.log.Warn("recorder write failed, retrying", "what", j.name, "attempt", attempt, "retry_in", delay, "err", err)
		t := time.NewTimer(delay)
		select {
		case <-t.C:
		case <-stop.Done():
			t.Stop()
			return false
		}
		delay = min(delay*2, retryMax)
	}
}

// retryable reports whether a failed write is worth repeating: the database
// was unreachable or the server reported a transient condition. Errors in
// the data itself (constraint violations, bad values) are permanent.
func retryable(err error, idempotent bool) bool {
	var connErr *pgconn.ConnectError
	if pgconn.SafeToRetry(err) || errors.As(err, &connErr) {
		return true // nothing reached the server
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// The server rejected the statement, so it did not commit.
		switch pgErr.Code[:min(2, len(pgErr.Code))] {
		case "08", "40", "53", "57": // connection, rollback, resources, shutdown
			return true
		}
		return false
	}
	return idempotent // timeout or broken connection: it may have committed
}

func (r *Recorder) enqueue(name string, idempotent bool, fn func(ctx context.Context) error) {
	select {
	case r.jobs <- job{name: name, fn: fn, idempotent: idempotent}:
	default:
		r.log.Error("recorder queue full, dropping write", "what", name)
	}
}

func (r *Recorder) Order(o domain.Order) {
	r.enqueue("order", true, func(ctx context.Context) error { return r.store.UpsertOrder(ctx, r.runID, o) })
}

func (r *Recorder) Fill(f domain.Fill) {
	r.enqueue("fill", true, func(ctx context.Context) error { return r.store.InsertFill(ctx, f) })
}

func (r *Recorder) Trade(t domain.Trade) {
	// Trades have no natural key, so a repeated insert would duplicate one.
	r.enqueue("trade", false, func(ctx context.Context) error { return r.store.InsertTrades(ctx, r.runID, []domain.Trade{t}) })
}

// Account stores an equity sample and the open positions of an account.
func (r *Recorder) Account(a *portfolio.Account, now time.Time) {
	snap := a.Snapshot()
	positions := a.Positions()
	key := a.Key
	r.enqueue("account", true, func(ctx context.Context) error {
		pt := storage.EquityPoint{Time: now, Equity: snap.Equity, Balance: snap.Balance, UnrealizedPnL: snap.UnrealizedPnL}
		if err := r.store.InsertEquity(ctx, r.runID, key.String(), []storage.EquityPoint{pt}); err != nil {
			return err
		}
		return r.store.ReplaceOpenPositions(ctx, r.runID, key.Exchange, key.Market, positions)
	})
}
