package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rizz404/trading-bot/internal/domain"
)

// TradeRow is a stored trade with its run.
type TradeRow struct {
	domain.Trade
	ID     int64
	RunID  int64
	NetPnL float64
}

const tradeCols = `t.id, t.run_id, t.exchange, t.market, t.symbol, t.strategy, t.side, t.quantity, t.entry_price, t.exit_price,
	t.opened_at, t.closed_at, t.gross_pnl, t.fees, t.funding, coalesce(t.exit_reason, '')`

func scanTrade(row pgx.CollectableRow) (TradeRow, error) {
	var t TradeRow
	err := row.Scan(&t.ID, &t.RunID, &t.Exchange, &t.Market, &t.Symbol, &t.Strategy, &t.Side, &t.Quantity, &t.EntryPrice,
		&t.ExitPrice, &t.OpenedAt, &t.ClosedAt, &t.GrossPnL, &t.Fees, &t.Funding, &t.ExitReason)
	t.NetPnL, _ = t.Trade.NetPnL().Float64()
	return t, err
}

// RecentTrades returns the newest trades of non-backtest runs.
func (s *Store) RecentTrades(ctx context.Context, limit int) ([]TradeRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+tradeCols+` FROM trades t JOIN strategy_runs r ON r.id = t.run_id
		WHERE r.mode <> 'backtest' ORDER BY t.closed_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent trades: %w", err)
	}
	return pgx.CollectRows(rows, scanTrade)
}

// RunTrades returns all trades of a run, oldest first.
func (s *Store) RunTrades(ctx context.Context, runID int64) ([]TradeRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+tradeCols+` FROM trades t WHERE t.run_id = $1 ORDER BY t.closed_at`, runID)
	if err != nil {
		return nil, fmt.Errorf("run trades: %w", err)
	}
	return pgx.CollectRows(rows, scanTrade)
}

// SymbolTrades returns trades on a symbol that overlap [from, to), for
// chart markers. runID 0 means all non-backtest runs.
func (s *Store) SymbolTrades(ctx context.Context, exchange string, m domain.Market, symbol string, from, to time.Time, runID int64) ([]TradeRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+tradeCols+` FROM trades t JOIN strategy_runs r ON r.id = t.run_id
		WHERE t.exchange = $1 AND t.market = $2 AND t.symbol = $3 AND t.closed_at >= $4 AND t.opened_at < $5
		  AND (($6 = 0 AND r.mode <> 'backtest') OR t.run_id = $6)
		ORDER BY t.opened_at`, exchange, m, symbol, from, to, runID)
	if err != nil {
		return nil, fmt.Errorf("symbol trades: %w", err)
	}
	return pgx.CollectRows(rows, scanTrade)
}

// EquitySeries returns the stored equity curve of a run, summed over its
// accounts, oldest first.
func (s *Store) EquitySeries(ctx context.Context, runID int64, since time.Time) ([]EquityPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ts, sum(equity), sum(balance), sum(unrealized_pnl) FROM equity_snapshots
		WHERE run_id = $1 AND ts >= $2 GROUP BY ts ORDER BY ts`, runID, since)
	if err != nil {
		return nil, fmt.Errorf("equity series: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (EquityPoint, error) {
		var p EquityPoint
		err := row.Scan(&p.Time, &p.Equity, &p.Balance, &p.UnrealizedPnL)
		return p, err
	})
}

const runCols = `id, instance_id, strategy, mode, exchange, market, symbols, params, started_at, stopped_at,
	coalesce(stop_reason, ''), summary`

func scanRun(row pgx.CollectableRow) (Run, error) {
	var r Run
	var params []byte
	err := row.Scan(&r.ID, &r.InstanceID, &r.Strategy, &r.Mode, &r.Exchange, &r.Market, &r.Symbols, &params,
		&r.StartedAt, &r.StoppedAt, &r.StopReason, &r.Summary)
	if err == nil && len(params) > 0 {
		err = json.Unmarshal(params, &r.Params)
	}
	return r, err
}

// Runs lists runs of a mode, newest first.
func (s *Store) Runs(ctx context.Context, mode string, limit int) ([]Run, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+runCols+` FROM strategy_runs WHERE mode = $1 ORDER BY id DESC LIMIT $2`, mode, limit)
	if err != nil {
		return nil, fmt.Errorf("runs: %w", err)
	}
	return pgx.CollectRows(rows, scanRun)
}

// GetRun returns one run, or nil if it does not exist.
func (s *Store) GetRun(ctx context.Context, id int64) (*Run, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+runCols+` FROM strategy_runs WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, scanRun)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

// SeriesList returns the stored candle series (for the chart picker).
func (s *Store) SeriesList(ctx context.Context) ([]domain.SeriesKey, error) {
	// Instruments are small; candles are huge, so derive the list from the
	// newest chunk via a time filter.
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT exchange, market, symbol, timeframe FROM candles
		WHERE open_time > now() - interval '7 days' ORDER BY 1, 2, 3, 4`)
	if err != nil {
		return nil, fmt.Errorf("series list: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.SeriesKey, error) {
		var k domain.SeriesKey
		err := row.Scan(&k.Exchange, &k.Market, &k.Symbol, &k.Timeframe)
		return k, err
	})
}
