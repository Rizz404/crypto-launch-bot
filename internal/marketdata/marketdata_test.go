package marketdata

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestSelectUniverse(t *testing.T) {
	ins := []domain.Instrument{
		{Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT", Tradable: true},
		{Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", Tradable: true},
		{Symbol: "SOLUSDT", Base: "SOL", Quote: "USDT", Tradable: true},
		{Symbol: "XRPUSDT", Base: "XRP", Quote: "USDT", Tradable: true},
		{Symbol: "USDCUSDT", Base: "USDC", Quote: "USDT", Tradable: true},
		{Symbol: "BTCUSDC", Base: "BTC", Quote: "USDC", Tradable: true},
		{Symbol: "OLDUSDT", Base: "OLD", Quote: "USDT", Tradable: false},
		{Symbol: "WIDEUSDT", Base: "WIDE", Quote: "USDT", Tradable: true},
		{Symbol: "NOBOOKUSDT", Base: "NOBOOK", Quote: "USDT", Tradable: true},
		{Symbol: "TINYUSDT", Base: "TINY", Quote: "USDT", Tradable: true},
	}
	tk := func(sym, vol, bid, ask string) domain.Ticker {
		return domain.Ticker{Symbol: sym, QuoteVolume: dec(vol), Bid: dec(bid), Ask: dec(ask)}
	}
	tickers := []domain.Ticker{
		tk("ETHUSDT", "500000000", "2999.9", "3000"),
		tk("BTCUSDT", "900000000", "81500", "81500.1"),
		tk("SOLUSDT", "200000000", "150", "150.01"),
		tk("XRPUSDT", "200000000", "2.5", "2.5001"),
		tk("USDCUSDT", "999999999", "1", "1.0001"),     // excluded base
		tk("BTCUSDC", "800000000", "81500", "81500.1"), // wrong quote
		tk("OLDUSDT", "700000000", "1", "1.001"),       // not tradable
		tk("WIDEUSDT", "600000000", "1", "1.01"),       // 100 bps spread
		tk("NOBOOKUSDT", "600000000", "0", "0"),        // no bid/ask
		tk("TINYUSDT", "1000", "1", "1.0001"),          // low volume
		tk("UNKNOWNUSDT", "999999999", "1", "1.0001"),  // no instrument
	}
	rules := UniverseRules{
		Quote: "USDT", TopN: 3, MinQuoteVolume: dec("10000000"), MaxSpreadBps: 10,
		ExcludeBases: []string{"usdc"},
	}

	picks := SelectUniverse(ins, tickers, rules)
	assert.Equal(t, []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}, Symbols(picks), "SOL before XRP on equal volume")

	rules.TopN, rules.MaxSpreadBps = 0, 0
	all := Symbols(SelectUniverse(ins, tickers, rules))
	assert.Contains(t, all, "WIDEUSDT", "spread filter disabled")
	assert.Contains(t, all, "NOBOOKUSDT")
	assert.NotContains(t, all, "TINYUSDT")
}

// fakeSource generates one 1m candle per minute up to the real clock and
// streams whatever the test sends on events.
type fakeSource struct {
	limit  int
	events chan exchange.StreamEvent

	mu    sync.Mutex
	calls int
}

func (f *fakeSource) Name() string    { return "fake" }
func (f *fakeSource) KlineLimit() int { return f.limit }
func (f *fakeSource) Instruments(context.Context, domain.Market) ([]domain.Instrument, error) {
	return nil, nil
}
func (f *fakeSource) Tickers(context.Context, domain.Market) ([]domain.Ticker, error) {
	return nil, nil
}

func (f *fakeSource) Klines(_ context.Context, req exchange.KlineRequest) ([]domain.Candle, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	now := time.Now()
	var out []domain.Candle
	for t := req.Timeframe.Truncate(req.Start); !t.After(now) && len(out) < f.limit; t = t.Add(req.Timeframe.Duration()) {
		out = append(out, candle(req.Symbol, req.Timeframe, t, !t.Add(req.Timeframe.Duration()).After(now)))
	}
	return out, nil
}

func (f *fakeSource) StreamKlines(context.Context, domain.Market, []exchange.KlineSub) (<-chan exchange.StreamEvent, error) {
	return f.events, nil
}

func candle(sym string, tf domain.Timeframe, open time.Time, closed bool) domain.Candle {
	return domain.Candle{
		Exchange: "fake", Market: domain.Futures, Symbol: sym, Timeframe: tf, OpenTime: open,
		Open: dec("1"), High: dec("2"), Low: dec("0.5"), Close: dec("1.5"), Volume: dec("10"), Closed: closed,
	}
}

type memStore struct {
	mu      sync.Mutex
	candles map[domain.SeriesKey]map[time.Time]domain.Candle
}

func newMemStore() *memStore {
	return &memStore{candles: map[domain.SeriesKey]map[time.Time]domain.Candle{}}
}

func (s *memStore) InsertCandles(_ context.Context, cs []domain.Candle) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, c := range cs {
		m := s.candles[c.Series()]
		if m == nil {
			m = map[time.Time]domain.Candle{}
			s.candles[c.Series()] = m
		}
		if _, ok := m[c.OpenTime]; !ok {
			m[c.OpenTime] = c
			n++
		}
	}
	return n, nil
}

func (s *memStore) LastCandleTime(_ context.Context, k domain.SeriesKey) (time.Time, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var last time.Time
	for t := range s.candles[k] {
		if t.After(last) {
			last = t
		}
	}
	return last, !last.IsZero(), nil
}

func (s *memStore) times(k domain.SeriesKey) []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ts []time.Time
	for t := range s.candles[k] {
		ts = append(ts, t)
	}
	slices.SortFunc(ts, time.Time.Compare)
	return ts
}

func TestFetchRangePaginates(t *testing.T) {
	src := &fakeSource{limit: 4}
	key := domain.SeriesKey{Exchange: "fake", Market: domain.Futures, Symbol: "BTCUSDT", Timeframe: "1m"}
	current := key.Timeframe.Truncate(time.Now())
	from := current.Add(-10 * time.Minute)

	var got []domain.Candle
	n, err := FetchRange(context.Background(), src, key, from, time.Now(), func(cs []domain.Candle) error {
		got = append(got, cs...)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 10, n, "the forming candle is excluded")
	assert.Len(t, got, 10)
	assert.Equal(t, from, got[0].OpenTime)
	assert.Equal(t, current.Add(-time.Minute), got[9].OpenTime)
	assert.Equal(t, 3, src.calls, "4 + 4 + 3 (short page ends the loop)")

	// "to" bounds the range.
	got = nil
	n, err = FetchRange(context.Background(), &fakeSource{limit: 100}, key, from, from.Add(3*time.Minute), func(cs []domain.Candle) error {
		got = append(got, cs...)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestFeedBackfillsAndDeduplicates(t *testing.T) {
	src := &fakeSource{limit: 1000, events: make(chan exchange.StreamEvent, 16)}
	store := newMemStore()
	tf := domain.Timeframe("1m")
	current := tf.Truncate(time.Now())
	if time.Until(current.Add(time.Minute)) < 2*time.Second {
		t.Skip("too close to a minute boundary for a deterministic run")
	}

	btc := domain.SeriesKey{Exchange: "fake", Market: domain.Futures, Symbol: "BTCUSDT", Timeframe: tf}
	eth := domain.SeriesKey{Exchange: "fake", Market: domain.Futures, Symbol: "ETHUSDT", Timeframe: tf}
	// BTC has history up to 10 minutes ago; ETH has nothing yet.
	_, _ = store.InsertCandles(context.Background(), []domain.Candle{candle("BTCUSDT", tf, current.Add(-10*time.Minute), true)})

	var delivered []domain.Candle
	f := &Feed{
		Source: src, Store: store, Market: domain.Futures, InitialBackfill: 5,
		Subs:     []exchange.KlineSub{{Symbol: "BTCUSDT", Timeframe: tf}, {Symbol: "ETHUSDT", Timeframe: tf}},
		OnCandle: func(c domain.Candle) { delivered = append(delivered, c) },
	}

	src.events <- exchange.StreamEvent{Kind: exchange.EventConnected}
	// The live stream re-sends the last closed candle (already backfilled),
	// an open candle, and then the current bar closing.
	src.events <- exchange.StreamEvent{Kind: exchange.EventCandle, Candle: candle("BTCUSDT", tf, current.Add(-time.Minute), true)}
	src.events <- exchange.StreamEvent{Kind: exchange.EventCandle, Candle: candle("BTCUSDT", tf, current, false)}
	src.events <- exchange.StreamEvent{Kind: exchange.EventConnected} // reconnect, nothing missed
	src.events <- exchange.StreamEvent{Kind: exchange.EventCandle, Candle: candle("BTCUSDT", tf, current, true)}
	close(src.events)

	require.NoError(t, f.Run(context.Background()))

	btcTimes := store.times(btc)
	require.Len(t, btcTimes, 11, "stored 10m-ago + 9 backfilled + 1 live")
	for i := 1; i < len(btcTimes); i++ {
		assert.Equal(t, time.Minute, btcTimes[i].Sub(btcTimes[i-1]), "no gaps")
	}
	assert.Equal(t, current, btcTimes[10])

	ethTimes := store.times(eth)
	require.Len(t, ethTimes, 5, "initial backfill")
	assert.Equal(t, current.Add(-5*time.Minute), ethTimes[0])

	// OnCandle: every new candle once, in order per series.
	perSeries := map[domain.SeriesKey][]time.Time{}
	for _, c := range delivered {
		perSeries[c.Series()] = append(perSeries[c.Series()], c.OpenTime)
	}
	assert.Len(t, perSeries[btc], 10)
	assert.Len(t, perSeries[eth], 5)
	for _, ts := range perSeries {
		assert.True(t, slices.IsSortedFunc(ts, time.Time.Compare))
		assert.Len(t, slices.Compact(slices.Clone(ts)), len(ts), "no duplicates")
	}
}
