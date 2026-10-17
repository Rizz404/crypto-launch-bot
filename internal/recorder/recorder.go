// Package recorder persists engine activity asynchronously, so that engine
// callbacks (which run under the engine lock) never wait on the database.
package recorder

import (
	"context"
	"log/slog"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/storage"
)

const queueSize = 10_000

type job struct {
	name string
	fn   func(ctx context.Context) error
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
// (for at most 10 seconds).
func (r *Recorder) Run(ctx context.Context) {
	defer close(r.done)
	for {
		select {
		case j := <-r.jobs:
			r.exec(ctx, j)
		case <-ctx.Done():
			flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for {
				select {
				case j := <-r.jobs:
					r.exec(flushCtx, j)
				default:
					return
				}
			}
		}
	}
}

// Wait blocks until Run has flushed and returned.
func (r *Recorder) Wait() { <-r.done }

func (r *Recorder) exec(ctx context.Context, j job) {
	if err := j.fn(ctx); err != nil {
		r.log.Error("recorder write failed", "what", j.name, "err", err)
	}
}

func (r *Recorder) enqueue(name string, fn func(ctx context.Context) error) {
	select {
	case r.jobs <- job{name, fn}:
	default:
		r.log.Error("recorder queue full, dropping write", "what", name)
	}
}

func (r *Recorder) Order(o domain.Order) {
	r.enqueue("order", func(ctx context.Context) error { return r.store.UpsertOrder(ctx, r.runID, o) })
}

func (r *Recorder) Fill(f domain.Fill) {
	r.enqueue("fill", func(ctx context.Context) error { return r.store.InsertFill(ctx, f) })
}

func (r *Recorder) Trade(t domain.Trade) {
	r.enqueue("trade", func(ctx context.Context) error { return r.store.InsertTrades(ctx, r.runID, []domain.Trade{t}) })
}

// Account stores an equity sample and the open positions of an account.
func (r *Recorder) Account(a *portfolio.Account, now time.Time) {
	snap := a.Snapshot()
	positions := a.Positions()
	key := a.Key
	r.enqueue("account", func(ctx context.Context) error {
		pt := storage.EquityPoint{Time: now, Equity: snap.Equity, Balance: snap.Balance, UnrealizedPnL: snap.UnrealizedPnL}
		if err := r.store.InsertEquity(ctx, r.runID, key.String(), []storage.EquityPoint{pt}); err != nil {
			return err
		}
		return r.store.ReplaceOpenPositions(ctx, r.runID, key.Exchange, key.Market, positions)
	})
}
