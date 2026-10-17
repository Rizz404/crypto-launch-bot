package main

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/metrics"
	"github.com/rizz404/trading-bot/internal/portfolio"
)

func TestReconcileRequiresPersistentMismatchAndPreservesMetadata(t *testing.T) {
	s := &session{log: slog.New(slog.NewTextHandler(io.Discard, nil)), mismatch: map[domain.PositionKey]int{},
		leverages: map[domain.PositionKey]int{}, metrics: metrics.NewTrading(prometheus.NewRegistry())}
	key := domain.PositionKey{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"}
	a := portfolio.NewAccount(portfolio.AccountKey{Exchange: key.Exchange, Market: key.Market}, decimal.NewFromInt(1000), nil)
	p := domain.Position{PositionKey: key, Side: domain.Long, Quantity: decimal.NewFromInt(1), EntryPrice: decimal.NewFromInt(100),
		Strategy: "test", Fees: decimal.NewFromInt(2), Funding: decimal.NewFromInt(1), OpenedAt: time.Now()}
	a.SetPosition(p)
	x := p
	x.Quantity, x.Strategy, x.Fees, x.Funding = decimal.NewFromInt(2), "", decimal.Zero, decimal.Zero
	s.reconcilePositions(a, []domain.Position{x}, false)
	got, _ := a.Position(key.Symbol)
	assert.Equal(t, "1", got.Quantity.String(), "first mismatch may be an in-flight fill")
	s.reconcilePositions(a, []domain.Position{p}, false)
	s.reconcilePositions(a, []domain.Position{x}, false)
	got, _ = a.Position(key.Symbol)
	assert.Equal(t, "1", got.Quantity.String(), "matching sample resets mismatch counter")
	s.reconcilePositions(a, []domain.Position{x}, false)
	got, _ = a.Position(key.Symbol)
	assert.Equal(t, "2", got.Quantity.String())
	assert.Equal(t, "test", got.Strategy)
	assert.Equal(t, "2", got.Fees.String())
	assert.Equal(t, "1", got.Funding.String())
	assert.Equal(t, p.OpenedAt, got.OpenedAt)
	s.reconcilePositions(a, nil, false)
	_, exists := a.Position(key.Symbol)
	require.True(t, exists)
	s.reconcilePositions(a, nil, false)
	assert.Empty(t, a.Positions())
	s.reconcilePositions(a, []domain.Position{x}, true)
	_, exists = a.Position(key.Symbol)
	assert.True(t, exists, "initial reconciliation loads positions immediately")
}
