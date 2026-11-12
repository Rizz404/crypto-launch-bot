package marketview

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/crypto-launch-bot/internal/domain"
	"github.com/rizz404/crypto-launch-bot/internal/exchange"
)

type fakeSource struct {
	events     chan exchange.MarketEvent
	snaps      chan exchange.DepthSnapshot
	snapCalls  atomic.Int32
	klineCalls atomic.Int32
}

func (f *fakeSource) Name() string { return "fake" }
func (f *fakeSource) Klines(_ context.Context, req exchange.KlineRequest) ([]domain.Candle, error) {
	f.klineCalls.Add(1)
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return []domain.Candle{
		{Symbol: req.Symbol, Timeframe: req.Timeframe, OpenTime: t0, Close: decimal.NewFromInt(1)},
		{Symbol: req.Symbol, Timeframe: req.Timeframe, OpenTime: t0.Add(time.Minute), Close: decimal.NewFromInt(2)},
	}, nil
}
func (f *fakeSource) Depth(ctx context.Context, _ string, _ int) (exchange.DepthSnapshot, error) {
	f.snapCalls.Add(1)
	select {
	case s := <-f.snaps:
		return s, nil
	case <-ctx.Done():
		return exchange.DepthSnapshot{}, ctx.Err()
	}
}
func (f *fakeSource) StreamMarket(context.Context, []string, []domain.Timeframe) (<-chan exchange.MarketEvent, error) {
	return f.events, nil
}

func lv(p, q string) exchange.Level {
	return exchange.Level{Price: decimal.RequireFromString(p), Qty: decimal.RequireFromString(q)}
}
func diff(first, final int64, bids, asks []exchange.Level) exchange.MarketEvent {
	return exchange.MarketEvent{Kind: exchange.MarketDepth, Depth: exchange.DepthUpdate{Symbol: "BTCUSDT", FirstID: first, FinalID: final, Bids: bids, Asks: asks}}
}

func TestBookFollowsBinanceSnapshotProcedure(t *testing.T) {
	src := &fakeSource{events: make(chan exchange.MarketEvent), snaps: make(chan exchange.DepthSnapshot, 2)}
	h := &Hub{Source: src, Symbols: []string{"BTCUSDT"}, Timeframes: []domain.Timeframe{"1m"}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = h.Run(ctx) }()
	view := func() View { return h.View("BTCUSDT", "1m", 10, 0) }

	src.events <- exchange.MarketEvent{Kind: exchange.MarketConnected}
	// Buffered before the snapshot: 98-100 is older than the snapshot, 101-102 bridges it.
	src.events <- diff(98, 100, []exchange.Level{lv("1", "9")}, nil)
	src.events <- diff(101, 102, []exchange.Level{lv("99", "0"), lv("98", "3")}, nil)
	src.snaps <- exchange.DepthSnapshot{LastUpdateID: 100, Bids: []exchange.Level{lv("99", "1"), lv("97", "2")}, Asks: []exchange.Level{lv("101", "1")}}
	require.Eventually(t, func() bool { return view().Synced }, time.Second, 5*time.Millisecond)
	v := view()
	assert.Equal(t, []string{"98", "97"}, prices(v.Bids), "99 removed by zero qty, stale 1 ignored")
	assert.Equal(t, "99.5", v.Mid().String())

	src.events <- diff(103, 104, nil, []exchange.Level{lv("100.5", "4")})
	require.Eventually(t, func() bool { return len(view().Asks) == 2 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, []string{"100.5", "101"}, prices(view().Asks))

	// A gap drops the book and asks for exactly one new snapshot.
	src.events <- diff(110, 111, nil, nil)
	require.Eventually(t, func() bool { return !view().Synced }, time.Second, 5*time.Millisecond)
	src.snaps <- exchange.DepthSnapshot{LastUpdateID: 110, Bids: []exchange.Level{lv("50", "1")}, Asks: []exchange.Level{lv("51", "1")}}
	require.Eventually(t, func() bool { return view().Synced }, time.Second, 5*time.Millisecond)
	assert.Equal(t, []string{"50"}, prices(view().Bids))
	assert.EqualValues(t, 2, src.snapCalls.Load())

	src.events <- exchange.MarketEvent{Kind: exchange.MarketTrade, Trade: exchange.Trade{Symbol: "BTCUSDT", Price: decimal.NewFromInt(50)}}
	src.events <- exchange.MarketEvent{Kind: exchange.MarketTrade, Trade: exchange.Trade{Symbol: "BTCUSDT", Price: decimal.NewFromInt(51)}}
	require.Eventually(t, func() bool { return len(view().Trades) == 2 }, time.Second, 5*time.Millisecond)
	assert.Len(t, h.View("BTCUSDT", "1m", 10, view().Trades[0].Seq).Trades, 1, "since returns only newer trades")
}

func prices(ls []exchange.Level) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Price.String()
	}
	return out
}

func TestHistoryLoadsOnceThenFollowsTheStream(t *testing.T) {
	src := &fakeSource{events: make(chan exchange.MarketEvent), snaps: make(chan exchange.DepthSnapshot)}
	h := &Hub{Source: src, Symbols: []string{"BTCUSDT"}, Timeframes: []domain.Timeframe{"1m"}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = h.Run(ctx) }()
	src.events <- exchange.MarketEvent{Kind: exchange.MarketConnected}
	hist, err := h.History(ctx, "BTCUSDT", "1m", 10)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	t0 := hist[0].OpenTime
	src.events <- exchange.MarketEvent{Kind: exchange.MarketCandle, Candle: domain.Candle{Symbol: "BTCUSDT", Timeframe: "1m", OpenTime: t0.Add(time.Minute), Close: decimal.NewFromInt(3)}}
	src.events <- exchange.MarketEvent{Kind: exchange.MarketCandle, Candle: domain.Candle{Symbol: "BTCUSDT", Timeframe: "1m", OpenTime: t0.Add(2 * time.Minute), Close: decimal.NewFromInt(4)}}
	require.Eventually(t, func() bool { h, _ := h.History(ctx, "BTCUSDT", "1m", 10); return len(h) == 3 }, time.Second, 5*time.Millisecond)
	hist, _ = h.History(ctx, "BTCUSDT", "1m", 2)
	assert.Equal(t, []string{"3", "4"}, []string{hist[0].Close.String(), hist[1].Close.String()}, "forming candle replaced, new one appended, limit applied")
	assert.EqualValues(t, 1, src.klineCalls.Load(), "REST history is fetched once")
	_, err = h.History(ctx, "ETHUSDT", "1m", 10)
	assert.Error(t, err, "symbols outside the hub are refused without a REST call")
}
