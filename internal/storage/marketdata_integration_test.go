//go:build integration

package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
)

// Run with: TB_TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/storage/
func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	url := os.Getenv("TB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TB_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := Open(ctx, url, 4)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	m, err := NewMigrator(pool)
	require.NoError(t, err)
	require.NoError(t, m.Up(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))))
	require.NoError(t, m.Close())

	// A unique exchange name isolates this run's rows.
	ex := fmt.Sprintf("test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM candles WHERE exchange = $1", ex)
		_, _ = pool.Exec(ctx, "DELETE FROM instruments WHERE exchange = $1", ex)
	})
	return NewStore(pool), ex
}

func TestCandlesRoundTrip(t *testing.T) {
	s, ex := testStore(t)
	ctx := context.Background()
	key := domain.SeriesKey{Exchange: ex, Market: domain.Futures, Symbol: "BTCUSDT", Timeframe: "1h"}

	_, _, ok, err := s.CandleRange(ctx, key)
	require.NoError(t, err)
	assert.False(t, ok)

	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var cs []domain.Candle
	for i := range 3 {
		cs = append(cs, domain.Candle{
			Exchange: ex, Market: domain.Futures, Symbol: "BTCUSDT", Timeframe: "1h",
			OpenTime: t0.Add(time.Duration(i) * time.Hour),
			Open:     decimal.RequireFromString("81511.10"), High: decimal.RequireFromString("81639.6"),
			Low: decimal.RequireFromString("81373.3"), Close: decimal.RequireFromString("81444"),
			Volume: decimal.RequireFromString("1280.695"), QuoteVolume: decimal.RequireFromString("104366013.7939"),
			Trades: 47482, Closed: true,
		})
	}
	n, err := s.InsertCandles(ctx, cs)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)

	n, err = s.InsertCandles(ctx, cs[1:])
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "duplicates ignored")

	first, last, ok, err := s.CandleRange(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, t0, first)
	assert.Equal(t, t0.Add(2*time.Hour), last)

	got, err := s.Candles(ctx, key, t0.Add(time.Hour), t0.Add(3*time.Hour))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, cs[1].OpenTime, got[0].OpenTime)
	assert.True(t, cs[1].Open.Equal(got[0].Open), "%s vs %s", cs[1].Open, got[0].Open)
	assert.Equal(t, "1280.695", got[0].Volume.String())
	assert.Equal(t, int64(47482), got[0].Trades)

	_, err = s.InsertCandles(ctx, []domain.Candle{{Exchange: ex, Closed: false}})
	assert.ErrorContains(t, err, "open candle")
}

func TestUpsertInstruments(t *testing.T) {
	s, ex := testStore(t)
	ctx := context.Background()
	in := domain.Instrument{
		Exchange: ex, Market: domain.Spot, Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT", Status: "TRADING",
		TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.00001"),
		MinQty: decimal.RequireFromString("0.00001"), MinNotional: decimal.RequireFromString("5"),
		ContractSize: decimal.NewFromInt(1),
	}
	require.NoError(t, s.UpsertInstruments(ctx, []domain.Instrument{in}))
	in.Status = "BREAK"
	require.NoError(t, s.UpsertInstruments(ctx, []domain.Instrument{in}))

	var status string
	var tick decimal.Decimal
	require.NoError(t, s.pool.QueryRow(ctx,
		"SELECT status, tick_size FROM instruments WHERE exchange = $1 AND symbol = 'BTCUSDT'", ex).Scan(&status, &tick))
	assert.Equal(t, "BREAK", status)
	assert.Equal(t, "0.01", tick.String())
}
