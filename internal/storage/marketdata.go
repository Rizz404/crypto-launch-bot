package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// Store provides typed queries on top of a pool.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// InsertCandles stores closed candles, ignoring ones already stored, and
// returns the number of new rows.
func (s *Store) InsertCandles(ctx context.Context, cs []domain.Candle) (int64, error) {
	if len(cs) == 0 {
		return 0, nil
	}
	n := len(cs)
	var (
		exchanges, markets, symbols, tfs = make([]string, n), make([]string, n), make([]string, n), make([]string, n)
		openTimes                        = make([]time.Time, n)
		o, h, l, c, v, qv                = make([]decimal.Decimal, n), make([]decimal.Decimal, n), make([]decimal.Decimal, n), make([]decimal.Decimal, n), make([]decimal.Decimal, n), make([]decimal.Decimal, n)
		trades                           = make([]int64, n)
	)
	for i, k := range cs {
		if !k.Closed {
			return 0, fmt.Errorf("refusing to store open candle %s %s", k.Series(), k.OpenTime)
		}
		exchanges[i], markets[i], symbols[i], tfs[i] = k.Exchange, string(k.Market), k.Symbol, string(k.Timeframe)
		openTimes[i] = k.OpenTime
		o[i], h[i], l[i], c[i], v[i], qv[i] = k.Open, k.High, k.Low, k.Close, k.Volume, k.QuoteVolume
		trades[i] = k.Trades
	}
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO candles (exchange, market, symbol, timeframe, open_time,
		                     open, high, low, close, volume, quote_volume, trades)
		SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::timestamptz[],
		                     $6::numeric[], $7::numeric[], $8::numeric[], $9::numeric[],
		                     $10::numeric[], $11::numeric[], $12::bigint[])
		ON CONFLICT DO NOTHING`,
		exchanges, markets, symbols, tfs, openTimes, o, h, l, c, v, qv, trades)
	if err != nil {
		return 0, fmt.Errorf("insert candles: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CandleRange returns the open times of the first and last stored candle
// of a series; ok is false when the series is empty.
func (s *Store) CandleRange(ctx context.Context, k domain.SeriesKey) (first, last time.Time, ok bool, err error) {
	var f, l *time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT min(open_time), max(open_time) FROM candles
		WHERE exchange = $1 AND market = $2 AND symbol = $3 AND timeframe = $4`,
		k.Exchange, k.Market, k.Symbol, k.Timeframe).Scan(&f, &l)
	if err != nil || f == nil {
		return time.Time{}, time.Time{}, false, err
	}
	return f.UTC(), l.UTC(), true, nil
}

// LastCandleTime returns the open time of the newest stored candle.
func (s *Store) LastCandleTime(ctx context.Context, k domain.SeriesKey) (time.Time, bool, error) {
	_, last, ok, err := s.CandleRange(ctx, k)
	return last, ok, err
}

// Candles returns stored candles with from <= open_time < to, oldest first.
func (s *Store) Candles(ctx context.Context, k domain.SeriesKey, from, to time.Time) ([]domain.Candle, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT open_time, open, high, low, close, volume, quote_volume, trades
		FROM candles
		WHERE exchange = $1 AND market = $2 AND symbol = $3 AND timeframe = $4
		  AND open_time >= $5 AND open_time < $6
		ORDER BY open_time`,
		k.Exchange, k.Market, k.Symbol, k.Timeframe, from, to)
	if err != nil {
		return nil, fmt.Errorf("query candles: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Candle, error) {
		c := domain.Candle{Exchange: k.Exchange, Market: k.Market, Symbol: k.Symbol, Timeframe: k.Timeframe, Closed: true}
		err := row.Scan(&c.OpenTime, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume, &c.QuoteVolume, &c.Trades)
		c.OpenTime = c.OpenTime.UTC()
		return c, err
	})
}

// UpsertInstruments inserts or refreshes trading rules.
func (s *Store) UpsertInstruments(ctx context.Context, ins []domain.Instrument) error {
	batch := &pgx.Batch{}
	for _, in := range ins {
		var maxLev *int
		if in.MaxLeverage > 0 {
			maxLev = &in.MaxLeverage
		}
		batch.Queue(`
			INSERT INTO instruments (exchange, market, symbol, base_asset, quote_asset, status,
			                         tick_size, step_size, min_qty, min_notional, contract_size, max_leverage, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
			ON CONFLICT (exchange, market, symbol) DO UPDATE SET
				base_asset = excluded.base_asset, quote_asset = excluded.quote_asset, status = excluded.status,
				tick_size = excluded.tick_size, step_size = excluded.step_size, min_qty = excluded.min_qty,
				min_notional = excluded.min_notional, contract_size = excluded.contract_size,
				max_leverage = excluded.max_leverage, updated_at = now()`,
			in.Exchange, in.Market, in.Symbol, in.Base, in.Quote, in.Status,
			in.TickSize, in.StepSize, in.MinQty, in.MinNotional, in.ContractSize, maxLev)
	}
	if err := s.pool.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("upsert instruments: %w", err)
	}
	return nil
}
