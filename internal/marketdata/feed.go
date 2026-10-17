package marketdata

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

// CandleStore persists candles. Implemented by storage.Store.
type CandleStore interface {
	InsertCandles(ctx context.Context, cs []domain.Candle) (int64, error)
	LastCandleTime(ctx context.Context, k domain.SeriesKey) (time.Time, bool, error)
}

// Feed streams live candles for one market, stores every closed candle and
// fills gaps through REST after each (re)connect.
type Feed struct {
	Source exchange.MarketData
	Store  CandleStore
	Market domain.Market
	Subs   []exchange.KlineSub
	// InitialBackfill is how many candles to fetch for a series that has
	// no stored data yet. Defaults to the exchange kline limit.
	InitialBackfill int
	// OnCandle, if set, receives every closed candle exactly once per
	// series, in open-time order (backfilled and live alike).
	OnCandle func(domain.Candle)
	// OnUpdate, if set, receives every update of still-forming candles
	// (their Close is the latest trade price).
	OnUpdate func(domain.Candle)
	Log      *slog.Logger
	Metrics  *Metrics // optional

	last map[domain.SeriesKey]time.Time // newest delivered open time
}

// Run blocks until ctx is cancelled.
func (f *Feed) Run(ctx context.Context) error {
	if f.Log == nil {
		f.Log = slog.Default()
	}
	f.Log = f.Log.With("market", f.Market)
	if f.InitialBackfill <= 0 {
		f.InitialBackfill = f.Source.KlineLimit()
	}
	f.last = make(map[domain.SeriesKey]time.Time, len(f.Subs))

	events, err := f.Source.StreamKlines(ctx, f.Market, f.Subs)
	if err != nil {
		return err
	}
	for ev := range events {
		switch ev.Kind {
		case exchange.EventConnected:
			if f.Metrics != nil {
				f.Metrics.connects.WithLabelValues(f.Source.Name(), string(f.Market)).Inc()
			}
			// Candles that closed while we were disconnected never arrive
			// on the stream; fetch them before handling newer events.
			f.backfill(ctx)
		case exchange.EventCandle:
			if !ev.Candle.Closed {
				if f.OnUpdate != nil {
					f.OnUpdate(ev.Candle)
				}
				continue
			}
			if f.Metrics != nil {
				f.Metrics.closedCandles.WithLabelValues(f.Source.Name(), string(f.Market), string(ev.Candle.Timeframe)).Inc()
			}
			if err := f.deliver(ctx, []domain.Candle{ev.Candle}); err != nil {
				f.Log.Error("store candle", "series", ev.Candle.Series(), "err", err)
			}
		}
	}
	return ctx.Err()
}

func (f *Feed) key(s exchange.KlineSub) domain.SeriesKey {
	return domain.SeriesKey{Exchange: f.Source.Name(), Market: f.Market, Symbol: s.Symbol, Timeframe: s.Timeframe}
}

func (f *Feed) backfill(ctx context.Context) {
	start := time.Now()
	total := 0
	for _, s := range f.Subs {
		if ctx.Err() != nil {
			return
		}
		key := f.key(s)
		n, err := f.backfillSeries(ctx, key)
		total += n
		if err != nil {
			f.Log.Error("backfill failed", "series", key, "err", err)
		}
	}
	f.Log.Info("backfill done", "series", len(f.Subs), "candles", total, "took", time.Since(start).Round(time.Millisecond))
}

func (f *Feed) backfillSeries(ctx context.Context, key domain.SeriesKey) (int, error) {
	tf := key.Timeframe.Duration()
	current := key.Timeframe.Truncate(time.Now()) // open time of the forming bar

	var from time.Time
	if last, ok := f.last[key]; ok {
		from = last.Add(tf)
	} else {
		stored, found, err := f.Store.LastCandleTime(ctx, key)
		if err != nil {
			return 0, err
		}
		if found {
			f.last[key] = stored
			from = stored.Add(tf)
		} else {
			from = current.Add(-time.Duration(f.InitialBackfill) * tf)
		}
	}
	if !from.Before(current) {
		return 0, nil // up to date
	}

	n, err := FetchRange(ctx, f.Source, key, from, current, func(cs []domain.Candle) error {
		return f.deliver(ctx, cs)
	})
	if f.Metrics != nil && n > 0 {
		f.Metrics.backfilled.WithLabelValues(key.Exchange, string(key.Market), string(key.Timeframe)).Add(float64(n))
	}
	return n, err
}

// deliver stores candles newer than the last delivered one of their series
// and forwards them to OnCandle.
func (f *Feed) deliver(ctx context.Context, cs []domain.Candle) error {
	fresh := cs[:0:0]
	for _, c := range cs {
		if last, ok := f.last[c.Series()]; ok && !c.OpenTime.After(last) {
			continue
		}
		fresh = append(fresh, c)
	}
	if len(fresh) == 0 {
		return nil
	}
	if _, err := f.Store.InsertCandles(ctx, fresh); err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	for _, c := range fresh {
		f.last[c.Series()] = c.OpenTime
		if f.Metrics != nil {
			f.Metrics.lastClose.WithLabelValues(c.Exchange, string(c.Market), c.Symbol, string(c.Timeframe)).Set(float64(c.CloseTime().Unix()))
		}
		if f.OnCandle != nil {
			f.OnCandle(c)
		}
	}
	return nil
}
