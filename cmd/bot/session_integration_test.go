//go:build integration

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/metrics"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
)

type errorLog struct {
	mu       sync.Mutex
	messages []string
}

func (*errorLog) Enabled(context.Context, slog.Level) bool { return true }
func (h *errorLog) Handle(_ context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		h.mu.Lock()
		msg := r.Message
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "err" {
				msg += ": " + a.Value.String()
			}
			return true
		})
		h.messages = append(h.messages, msg)
		h.mu.Unlock()
	}
	return nil
}
func (h *errorLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *errorLog) WithGroup(string) slog.Handler      { return h }

// Separate opt-in: session recovery selects the latest run globally, so
// use a dedicated database without another bot or storage suite running.
func TestPaperSessionRestartPersistsNewOrders(t *testing.T) {
	u := os.Getenv("TB_TEST_SESSION_DATABASE_URL")
	if u == "" {
		t.Skip("set TB_TEST_SESSION_DATABASE_URL to a dedicated migrated database")
	}
	ctx := context.Background()
	pool, err := storage.Open(ctx, u, 4)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	store := storage.NewStore(pool)
	ex := fmt.Sprintf("paper-test-%d", time.Now().UnixNano())
	ins := domain.Instrument{Exchange: ex, Market: domain.Futures, Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT", Status: "TRADING",
		TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.001"), ContractSize: decimal.NewFromInt(1)}
	require.NoError(t, store.UpsertInstruments(ctx, []domain.Instrument{ins}))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM fills WHERE order_id IN (SELECT id FROM orders WHERE exchange=$1)", ex)
		_, _ = pool.Exec(ctx, "DELETE FROM orders WHERE exchange=$1", ex)
		_, _ = pool.Exec(ctx, "DELETE FROM positions WHERE exchange=$1", ex)
		_, _ = pool.Exec(ctx, "DELETE FROM equity_snapshots WHERE exchange=$1", ex+":futures")
		_, _ = pool.Exec(ctx, "DELETE FROM instruments WHERE exchange=$1", ex)
	})
	conf := &config.Config{App: config.AppConfig{Mode: config.ModePaper}, Risk: risk.DefaultConfig(),
		Paper: config.PaperConfig{InitialBalance: 1000}, Telegram: config.TelegramConfig{NotifyLevel: "warn"}}
	sc := strategy.Config{ID: "dca-test", Name: "dca", Exchange: ex, Market: domain.Futures, Symbols: []string{"BTCUSDT"}, Leverage: 1,
		Params: map[string]any{"quote_amount": 10, "max_buys": 1, "interval": "24h"}}
	h := &errorLog{}
	log := slog.New(h)
	for i := range 2 {
		conf.Paper.Resume = i > 0
		s, err := newSession(ctx, conf, store, []strategy.Config{sc}, metrics.NewTrading(prometheus.NewRegistry()), nil, log)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, "DELETE FROM fills WHERE order_id IN (SELECT id FROM orders WHERE run_id=$1)", s.runID)
			for _, table := range []string{"orders", "positions", "trades", "equity_snapshots"} {
				_, _ = pool.Exec(ctx, "DELETE FROM "+table+" WHERE run_id=$1", s.runID)
			}
			_, _ = pool.Exec(ctx, "DELETE FROM strategy_runs WHERE id=$1", s.runID)
		})
		func() {
			defer s.close("test completed")
			if i > 0 {
				orders := s.eng.OpenOrders()
				require.Len(t, orders, 1, "retain raw limit order, discard entry whose stop plan was not persisted")
				assert.Equal(t, domain.TagStrategy, orders[0].Tag)
			}
			c := domain.Candle{Exchange: ex, Market: domain.Futures, Symbol: "BTCUSDT", Timeframe: "1m", Close: decimal.NewFromInt(100)}
			s.onUpdate(ctx, c)
			s.eng.Tick(ctx, time.Now())
			s.onUpdate(ctx, c)
			positions := s.book.Total().OpenPositions
			assert.Equal(t, 1, positions)
			p, ok := s.book.Account(portfolio.AccountKey{Exchange: ex, Market: domain.Futures}).Position("BTCUSDT")
			assert.True(t, ok)
			want := decimal.NewFromInt(int64(i + 1)).Div(decimal.NewFromInt(10))
			assert.True(t, p.Quantity.Equal(want), "restart must retain the previous position")
			if i == 0 {
				for _, tag := range []string{domain.TagStrategy, domain.TagEntry} {
					o, err := s.sims[ex].PlaceOrder(ctx, domain.Order{ClientID: fmt.Sprintf("tbpending%d%s", s.runID, tag), Exchange: ex,
						Market: domain.Futures, Symbol: "BTCUSDT", Side: domain.Buy, Type: domain.OrderLimit,
						Price: decimal.NewFromInt(1), Quantity: decimal.RequireFromString("0.01"), Strategy: sc.ID, Tag: tag})
					require.NoError(t, err)
					s.eng.AdoptOrders([]domain.Order{o})
				}
			}
		}()
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE run_id=$1 AND status='filled'", s.runID).Scan(&n))
		assert.Equal(t, 1, n, "each session's new fill must be persisted")
		if i > 0 {
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE run_id=$1 AND tag='entry' AND status='canceled'", s.runID).Scan(&n))
			assert.Equal(t, 1, n, "discarded pending entry must be recorded in the active run")
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	assert.Empty(t, h.messages, "paper execution and recorder must be error-free")
}
