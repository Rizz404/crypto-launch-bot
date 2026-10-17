package marketdata

import (
	"context"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

// FetchRange downloads closed candles with from <= open time < to, page by
// page, passing each page to save. It returns the number of candles saved.
func FetchRange(ctx context.Context, src exchange.MarketData, key domain.SeriesKey, from, to time.Time, save func([]domain.Candle) error) (int, error) {
	tf := key.Timeframe.Duration()
	start := key.Timeframe.Truncate(from)
	total := 0
	for start.Before(to) {
		page, err := src.Klines(ctx, exchange.KlineRequest{
			Market: key.Market, Symbol: key.Symbol, Timeframe: key.Timeframe, Start: start,
		})
		if err != nil {
			return total, err
		}

		closed := page[:0]
		for _, c := range page {
			if c.Closed && c.OpenTime.Before(to) {
				closed = append(closed, c)
			}
		}
		if len(closed) > 0 {
			if err := save(closed); err != nil {
				return total, err
			}
			total += len(closed)
		}

		// A short page means we reached the newest data.
		if len(page) < src.KlineLimit() {
			break
		}
		start = page[len(page)-1].OpenTime.Add(tf)
	}
	return total, nil
}
