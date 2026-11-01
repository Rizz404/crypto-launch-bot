// Package marketview keeps the live market state of a trading screen: order
// books, recent trades, 24h tickers and forming candles. One exchange stream
// per network feeds every browser, so opening more tabs costs no API weight.
package marketview

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

// snapshotLimit is the REST depth used to align the diff stream (weight 50).
const snapshotLimit = 1000

// tradeKeep is how many recent trades each symbol keeps for the tape.
const tradeKeep = 60

// historyKeep caps the chart candles kept per symbol and timeframe.
const historyKeep = 1000

// ChartTimeframes are the timeframes the trading screen offers.
var ChartTimeframes = []domain.Timeframe{"1m", "5m", "15m", "1h", "4h", "1d"}

// Source is the public market API of one network.
type Source interface {
	exchange.MarketStream
	Klines(ctx context.Context, req exchange.KlineRequest) ([]domain.Candle, error)
}

// Hub owns the market state of one network (mainnet or testnet).
type Hub struct {
	Source     Source
	Symbols    []string
	Timeframes []domain.Timeframe
	Log        *slog.Logger

	mu      sync.RWMutex
	books   map[string]*book
	trades  map[string][]Trade
	seq     int64
	tickers map[string]exchange.Ticker24
	candles map[string]domain.Candle // symbol + "|" + timeframe
	at      map[string]time.Time     // last event per symbol
	// history is loaded once per key by REST (weight 2), then kept current
	// from the kline stream.
	history map[string][]domain.Candle
	hmu     sync.Mutex // serializes history loads
}

// Trade is a tape entry with a hub-wide sequence for incremental reads.
type Trade struct {
	Seq int64 `json:"seq"`
	exchange.Trade
}

type book struct {
	synced     bool
	last       int64
	bids, asks map[string]exchange.Level // keyed by canonical price string
	buffer     []exchange.DepthUpdate
	fetching   bool
}

// View is what the trading screen shows for one symbol.
type View struct {
	Symbol  string
	Synced  bool
	Bids    []exchange.Level // best first
	Asks    []exchange.Level // best first
	Trades  []Trade          // newest last
	Ticker  exchange.Ticker24
	Candle  domain.Candle
	Updated time.Time
}

func (h *Hub) init() {
	h.books = map[string]*book{}
	h.trades = map[string][]Trade{}
	h.tickers = map[string]exchange.Ticker24{}
	h.candles = map[string]domain.Candle{}
	h.at = map[string]time.Time{}
	h.history = map[string][]domain.Candle{}
	for _, s := range h.Symbols {
		h.books[s] = &book{}
	}
}

// Has reports whether the hub streams symbol.
func (h *Hub) Has(symbol string) bool { return slices.Contains(h.Symbols, symbol) }

// Run streams until ctx is cancelled. Snapshots are requested only after a
// (re)connect or a sequence gap, as Binance's book procedure requires.
func (h *Hub) Run(ctx context.Context) error {
	h.mu.Lock()
	h.init()
	h.mu.Unlock()
	events, err := h.Source.StreamMarket(ctx, h.Symbols, h.Timeframes)
	if err != nil {
		return err
	}
	snapshots := make(chan snapshotResult, len(h.Symbols))
	for {
		select {
		case <-ctx.Done():
			return nil
		case r := <-snapshots:
			h.applySnapshot(ctx, r, snapshots)
		case ev, ok := <-events:
			if !ok {
				return ctx.Err()
			}
			h.handle(ctx, ev, snapshots)
		}
	}
}

type snapshotResult struct {
	symbol string
	snap   exchange.DepthSnapshot
	err    error
}

func (h *Hub) fetch(ctx context.Context, symbol string, out chan<- snapshotResult) {
	go func() {
		snap, err := h.Source.Depth(ctx, symbol, snapshotLimit)
		select {
		case out <- snapshotResult{symbol, snap, err}:
		case <-ctx.Done():
		}
	}()
}

func (h *Hub) handle(ctx context.Context, ev exchange.MarketEvent, snapshots chan<- snapshotResult) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch ev.Kind {
	case exchange.MarketConnected:
		// Updates may have been missed while disconnected.
		for _, b := range h.books {
			b.synced, b.buffer = false, nil
		}
	case exchange.MarketDepth:
		b := h.books[ev.Depth.Symbol]
		if b == nil {
			return
		}
		h.at[ev.Depth.Symbol] = time.Now()
		if !b.synced {
			// shortcut: a failing snapshot keeps only recent diffs; older ones are useless anyway.
			if len(b.buffer) >= 2000 {
				b.buffer = slices.Clone(b.buffer[1000:])
			}
			b.buffer = append(b.buffer, ev.Depth)
			if !b.fetching {
				b.fetching = true
				h.fetch(ctx, ev.Depth.Symbol, snapshots)
			}
			return
		}
		if !b.apply(ev.Depth) {
			h.Log.Warn("order book gap, resyncing", "symbol", ev.Depth.Symbol, "last", b.last, "first", ev.Depth.FirstID)
			b.synced, b.buffer, b.fetching = false, []exchange.DepthUpdate{ev.Depth}, true
			h.fetch(ctx, ev.Depth.Symbol, snapshots)
		}
	case exchange.MarketTrade:
		h.seq++
		t := append(h.trades[ev.Trade.Symbol], Trade{Seq: h.seq, Trade: ev.Trade})
		if len(t) > tradeKeep {
			t = slices.Clone(t[len(t)-tradeKeep:])
		}
		h.trades[ev.Trade.Symbol] = t
		h.at[ev.Trade.Symbol] = time.Now()
	case exchange.MarketTicker:
		h.tickers[ev.Ticker.Symbol] = ev.Ticker
	case exchange.MarketCandle:
		key := ev.Candle.Symbol + "|" + string(ev.Candle.Timeframe)
		h.candles[key] = ev.Candle
		if hist, ok := h.history[key]; ok {
			h.history[key] = upsert(hist, ev.Candle)
		}
	}
}

func (h *Hub) applySnapshot(ctx context.Context, r snapshotResult, snapshots chan<- snapshotResult) {
	h.mu.Lock()
	defer h.mu.Unlock()
	b := h.books[r.symbol]
	b.fetching = false
	if r.err != nil {
		h.Log.Warn("order book snapshot failed", "symbol", r.symbol, "err", r.err)
		return // the next diff event retries
	}
	b.reset(r.snap)
	for _, u := range b.buffer {
		if u.FinalID <= b.last {
			continue
		}
		if !b.apply(u) {
			// Snapshot older than the buffered stream start: fetch again.
			b.synced, b.fetching = false, true
			h.fetch(ctx, r.symbol, snapshots)
			return
		}
	}
	b.buffer, b.synced = nil, true
}

func (b *book) reset(s exchange.DepthSnapshot) {
	b.last = s.LastUpdateID
	b.bids, b.asks = map[string]exchange.Level{}, map[string]exchange.Level{}
	set(b.bids, s.Bids)
	set(b.asks, s.Asks)
}

// apply adds one diff. It reports false on a sequence gap, which needs a new
// snapshot. Events entirely older than the book are ignored.
func (b *book) apply(u exchange.DepthUpdate) bool {
	if u.FinalID <= b.last {
		return true
	}
	if u.FirstID > b.last+1 {
		return false
	}
	set(b.bids, u.Bids)
	set(b.asks, u.Asks)
	b.last = u.FinalID
	return true
}

func set(side map[string]exchange.Level, levels []exchange.Level) {
	for _, l := range levels {
		k := l.Price.String()
		if l.Qty.IsZero() {
			delete(side, k)
		} else {
			side[k] = l
		}
	}
}

func top(side map[string]exchange.Level, n int, bids bool) []exchange.Level {
	out := make([]exchange.Level, 0, len(side))
	for _, l := range side {
		out = append(out, l)
	}
	slices.SortFunc(out, func(a, b exchange.Level) int {
		if bids {
			return b.Price.Cmp(a.Price)
		}
		return a.Price.Cmp(b.Price)
	})
	return out[:min(n, len(out))]
}

// View returns the screen state of symbol with up to depth levels per side
// and the trades newer than since.
func (h *Hub) View(symbol string, tf domain.Timeframe, depth int, since int64) View {
	h.mu.RLock()
	defer h.mu.RUnlock()
	v := View{Symbol: symbol, Ticker: h.tickers[symbol], Candle: h.candles[symbol+"|"+string(tf)], Updated: h.at[symbol]}
	if b := h.books[symbol]; b != nil && b.synced {
		v.Synced, v.Bids, v.Asks = true, top(b.bids, depth, true), top(b.asks, depth, false)
	}
	for _, t := range h.trades[symbol] {
		if t.Seq > since {
			v.Trades = append(v.Trades, t)
		}
	}
	return v
}

// Mid returns the middle of the best bid and ask, or zero without a book.
func (v View) Mid() decimal.Decimal {
	if len(v.Bids) == 0 || len(v.Asks) == 0 {
		return decimal.Zero
	}
	return v.Bids[0].Price.Add(v.Asks[0].Price).Div(decimal.NewFromInt(2))
}

func upsert(hist []domain.Candle, c domain.Candle) []domain.Candle {
	if n := len(hist); n > 0 && hist[n-1].OpenTime.Equal(c.OpenTime) {
		hist[n-1] = c
		return hist
	} else if n > 0 && c.OpenTime.Before(hist[n-1].OpenTime) {
		return hist
	}
	hist = append(hist, c)
	if len(hist) > historyKeep {
		hist = slices.Clone(hist[len(hist)-historyKeep:])
	}
	return hist
}

// History returns up to limit chart candles of symbol, loading them from
// REST the first time a symbol and timeframe is opened.
func (h *Hub) History(ctx context.Context, symbol string, tf domain.Timeframe, limit int) ([]domain.Candle, error) {
	if !h.Has(symbol) || !slices.Contains(ChartTimeframes, tf) {
		return nil, fmt.Errorf("%s %s is not streamed", symbol, tf)
	}
	key := symbol + "|" + string(tf)
	h.hmu.Lock()
	defer h.hmu.Unlock()
	h.mu.RLock()
	hist, ok := h.history[key]
	h.mu.RUnlock()
	if !ok {
		start := time.Now().Add(-time.Duration(historyKeep) * tf.Duration())
		bars, err := h.Source.Klines(ctx, exchange.KlineRequest{Market: domain.Spot, Symbol: symbol, Timeframe: tf, Start: start, Limit: historyKeep})
		if err != nil {
			return nil, err
		}
		h.mu.Lock()
		if h.history == nil {
			h.history = map[string][]domain.Candle{}
		}
		// Stream updates that arrived during the request are newer: keep them.
		for _, c := range bars {
			hist = upsert(hist, c)
		}
		if live, ok := h.candles[key]; ok {
			hist = upsert(hist, live)
		}
		h.history[key] = hist
		h.mu.Unlock()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	hist = h.history[key]
	return slices.Clone(hist[max(0, len(hist)-limit):]), nil
}
