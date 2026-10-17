package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// Run is one execution of a strategy instance.
type Run struct {
	ID         int64
	InstanceID string
	Strategy   string
	Mode       string
	Exchange   string
	Market     domain.Market
	Symbols    []string
	Params     map[string]any
	StartedAt  time.Time
	StoppedAt  *time.Time
	StopReason string
	Summary    json.RawMessage
}

// CreateRun inserts a run and returns its id.
func (s *Store) CreateRun(ctx context.Context, r Run) (int64, error) {
	// A trading session spans multiple strategies and does not supply a
	// single symbols list. pgx encodes a nil slice as SQL NULL, whereas
	// strategy_runs.symbols is NOT NULL.
	if r.Symbols == nil {
		r.Symbols = []string{}
	}
	params, err := json.Marshal(r.Params)
	if err != nil {
		return 0, err
	}
	if r.StartedAt.IsZero() {
		r.StartedAt = time.Now()
	}
	var id int64
	err = s.pool.QueryRow(ctx, `
		INSERT INTO strategy_runs (instance_id, strategy, mode, exchange, market, symbols, params, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		r.InstanceID, r.Strategy, r.Mode, r.Exchange, r.Market, r.Symbols, params, r.StartedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create run: %w", err)
	}
	return id, nil
}

// FinishRun marks a run as stopped and stores its summary (any JSON value).
func (s *Store) FinishRun(ctx context.Context, id int64, reason string, summary any) error {
	var raw []byte
	if summary != nil {
		var err error
		if raw, err = json.Marshal(summary); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE strategy_runs SET stopped_at = now(), stop_reason = $2, summary = $3 WHERE id = $1`,
		id, reason, raw)
	if err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	return nil
}

// InsertTrades stores closed round trips of a run.
func (s *Store) InsertTrades(ctx context.Context, runID int64, ts []domain.Trade) error {
	if len(ts) == 0 {
		return nil
	}
	rows := make([][]any, len(ts))
	for i, t := range ts {
		rows[i] = []any{runID, t.Exchange, t.Market, t.Symbol, t.Strategy, t.Side, t.Quantity, t.EntryPrice, t.ExitPrice,
			t.OpenedAt, t.ClosedAt, t.GrossPnL, t.Fees, t.Funding, t.NetPnL(), t.ExitReason}
	}
	_, err := s.pool.CopyFrom(ctx, pgx.Identifier{"trades"},
		[]string{"run_id", "exchange", "market", "symbol", "strategy", "side", "quantity", "entry_price", "exit_price",
			"opened_at", "closed_at", "gross_pnl", "fees", "funding", "net_pnl", "exit_reason"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("insert trades: %w", err)
	}
	return nil
}

// EquityPoint is one equity sample.
type EquityPoint struct {
	Time          time.Time
	Equity        decimal.Decimal
	Balance       decimal.Decimal
	UnrealizedPnL decimal.Decimal
}

// InsertEquity stores equity samples of a run, ignoring duplicates.
func (s *Store) InsertEquity(ctx context.Context, runID int64, exchange string, pts []EquityPoint) error {
	batch := &pgx.Batch{}
	for _, p := range pts {
		batch.Queue(`
			INSERT INTO equity_snapshots (run_id, exchange, ts, equity, balance, unrealized_pnl)
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
			runID, exchange, p.Time, p.Equity, p.Balance, p.UnrealizedPnL)
	}
	if err := s.pool.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert equity: %w", err)
	}
	return nil
}

// Instruments returns the stored trading rules of a market.
func (s *Store) Instruments(ctx context.Context, exchange string, m domain.Market) ([]domain.Instrument, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT symbol, base_asset, quote_asset, status, tick_size, step_size, min_qty, min_notional, contract_size,
		       coalesce(max_leverage, 0)
		FROM instruments WHERE exchange = $1 AND market = $2`, exchange, m)
	if err != nil {
		return nil, fmt.Errorf("query instruments: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Instrument, error) {
		in := domain.Instrument{Exchange: exchange, Market: m}
		err := row.Scan(&in.Symbol, &in.Base, &in.Quote, &in.Status, &in.TickSize, &in.StepSize, &in.MinQty,
			&in.MinNotional, &in.ContractSize, &in.MaxLeverage)
		in.Tradable = in.Status == "TRADING"
		return in, err
	})
}

// TradeStats summarises the trades of a run closed since a time.
type TradeStats struct {
	Trades int
	Wins   int
	NetPnL decimal.Decimal
	Fees   decimal.Decimal
}

func (s *Store) TradeStats(ctx context.Context, runID int64, since time.Time) (TradeStats, error) {
	return s.TradeStatsBetween(ctx, runID, since, time.Now().Add(time.Hour))
}

// TradeStatsBetween summarises the trades of a run closed in [from, to).
func (s *Store) TradeStatsBetween(ctx context.Context, runID int64, from, to time.Time) (TradeStats, error) {
	var st TradeStats
	err := s.pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE net_pnl > 0), coalesce(sum(net_pnl), 0), coalesce(sum(fees), 0)
		FROM trades WHERE run_id = $1 AND closed_at >= $2 AND closed_at < $3`, runID, from, to).
		Scan(&st.Trades, &st.Wins, &st.NetPnL, &st.Fees)
	if err != nil {
		return st, fmt.Errorf("trade stats: %w", err)
	}
	return st, nil
}
