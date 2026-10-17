package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// nullDec turns a zero decimal into NULL.
func nullDec(d decimal.Decimal) *decimal.Decimal {
	if d.IsZero() {
		return nil
	}
	return &d
}

// UpsertOrder inserts an order or updates its state.
func (s *Store) UpsertOrder(ctx context.Context, runID int64, o domain.Order) error {
	var exID *string
	if o.ExchangeID != "" {
		exID = &o.ExchangeID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO orders (run_id, exchange, market, symbol, client_order_id, exchange_order_id, side, type,
		                    reduce_only, price, stop_price, quantity, filled_quantity, avg_fill_price, status, tag,
		                    strategy, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (client_order_id) DO UPDATE SET
			run_id = excluded.run_id, -- adopted orders move to the current session
			exchange_order_id = coalesce(excluded.exchange_order_id, orders.exchange_order_id),
			filled_quantity = excluded.filled_quantity, avg_fill_price = excluded.avg_fill_price,
			status = excluded.status, reason = coalesce(excluded.reason, orders.reason), updated_at = excluded.updated_at`,
		runID, o.Exchange, o.Market, o.Symbol, o.ClientID, exID, o.Side, o.Type, o.ReduceOnly,
		nullDec(o.Price), nullDec(o.StopPrice), o.Quantity, o.FilledQty, nullDec(o.AvgPrice), o.Status, o.Tag,
		o.Strategy, nullStr(o.Reason), o.CreatedAt, o.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert order %s: %w", o.ClientID, err)
	}
	return nil
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// InsertFill stores an execution of a stored order.
func (s *Store) InsertFill(ctx context.Context, f domain.Fill) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO fills (order_id, trade_id, price, quantity, fee, fee_asset, is_maker, filled_at)
		SELECT id, $2, $3, $4, $5, $6, $7, $8 FROM orders WHERE client_order_id = $1
		ON CONFLICT DO NOTHING`,
		f.OrderClientID, f.TradeID, f.Price, f.Quantity, f.Fee, nullStr(f.FeeAsset), f.Maker, f.Time)
	if err != nil {
		return fmt.Errorf("insert fill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("insert fill: order %s not stored (or duplicate fill %s)", f.OrderClientID, f.TradeID)
	}
	return nil
}

// ReplaceOpenPositions stores the current open positions of one account of
// a run, replacing the previous snapshot.
func (s *Store) ReplaceOpenPositions(ctx context.Context, runID int64, exchange string, m domain.Market, ps []domain.Position) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM positions WHERE run_id = $1 AND exchange = $2 AND market = $3 AND status = 'open'`,
			runID, exchange, m); err != nil {
			return err
		}
		for _, p := range ps {
			var margin *string
			if m == domain.Futures {
				iso := "isolated"
				margin = &iso
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO positions (run_id, exchange, market, symbol, side, quantity, entry_price, leverage,
				                       margin_mode, fees, status, opened_at, strategy)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'open', $11, $12)`,
				runID, exchange, m, p.Symbol, p.Side, p.Quantity, p.EntryPrice, max(p.Leverage, 1),
				margin, p.Fees, p.OpenedAt, p.Strategy); err != nil {
				return err
			}
		}
		return nil
	})
}

// SessionState is what a previous session left behind.
type SessionState struct {
	RunID     int64
	Balances  map[string]decimal.Decimal // by account key "exchange:market"
	Positions []domain.Position
}

// LastSessionState loads the balances and open positions of the most recent
// session run of mode, or nil if there is none.
func (s *Store) LastSessionState(ctx context.Context, mode string) (*SessionState, error) {
	st := &SessionState{Balances: map[string]decimal.Decimal{}}
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM strategy_runs WHERE mode = $1 AND instance_id = 'session'
		ORDER BY started_at DESC, id DESC LIMIT 1`, mode).Scan(&st.RunID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("last session: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (exchange) exchange, balance FROM equity_snapshots
		WHERE run_id = $1 ORDER BY exchange, ts DESC`, st.RunID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var acct string
		var bal decimal.Decimal
		if err := rows.Scan(&acct, &bal); err != nil {
			rows.Close()
			return nil, err
		}
		st.Balances[acct] = bal
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	prow, err := s.pool.Query(ctx, `
		SELECT exchange, market, symbol, side, quantity, entry_price, leverage, fees, opened_at, strategy
		FROM positions WHERE run_id = $1 AND status = 'open'`, st.RunID)
	if err != nil {
		return nil, err
	}
	st.Positions, err = pgx.CollectRows(prow, func(row pgx.CollectableRow) (domain.Position, error) {
		var p domain.Position
		var openedAt time.Time
		err := row.Scan(&p.Exchange, &p.Market, &p.Symbol, &p.Side, &p.Quantity, &p.EntryPrice, &p.Leverage,
			&p.Fees, &openedAt, &p.Strategy)
		p.OpenedAt = openedAt
		return p, err
	})
	return st, err
}

// RecentCandles returns the newest n closed candles of a series, oldest first.
func (s *Store) RecentCandles(ctx context.Context, k domain.SeriesKey, n int) ([]domain.Candle, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT open_time, open, high, low, close, volume, quote_volume, trades FROM (
			SELECT * FROM candles
			WHERE exchange = $1 AND market = $2 AND symbol = $3 AND timeframe = $4
			ORDER BY open_time DESC LIMIT $5
		) t ORDER BY open_time`,
		k.Exchange, k.Market, k.Symbol, k.Timeframe, n)
	if err != nil {
		return nil, fmt.Errorf("recent candles: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Candle, error) {
		c := domain.Candle{Exchange: k.Exchange, Market: k.Market, Symbol: k.Symbol, Timeframe: k.Timeframe, Closed: true}
		err := row.Scan(&c.OpenTime, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume, &c.QuoteVolume, &c.Trades)
		c.OpenTime = c.OpenTime.UTC()
		return c, err
	})
}

// OpenOrders returns the orders of a run that were still open when it
// stopped.
func (s *Store) OpenOrders(ctx context.Context, runID int64) ([]domain.Order, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT client_order_id, coalesce(exchange_order_id, ''), exchange, market, symbol, side, type, quantity,
		       coalesce(price, 0), coalesce(stop_price, 0), reduce_only, status, filled_quantity,
		       coalesce(avg_fill_price, 0), coalesce(tag, ''), strategy, created_at, updated_at
		FROM orders WHERE run_id = $1 AND status IN ('new', 'partially_filled')`, runID)
	if err != nil {
		return nil, fmt.Errorf("open orders: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Order, error) {
		var o domain.Order
		err := row.Scan(&o.ClientID, &o.ExchangeID, &o.Exchange, &o.Market, &o.Symbol, &o.Side, &o.Type, &o.Quantity,
			&o.Price, &o.StopPrice, &o.ReduceOnly, &o.Status, &o.FilledQty, &o.AvgPrice, &o.Tag, &o.Strategy,
			&o.CreatedAt, &o.UpdatedAt)
		return o, err
	})
}

// OrderMeta is what the bot knows about an order beyond the exchange data.
type OrderMeta struct {
	Tag      string
	Strategy string
}

// OrderMetas returns the stored tag and strategy of the given client ids.
func (s *Store) OrderMetas(ctx context.Context, clientIDs []string) (map[string]OrderMeta, error) {
	out := map[string]OrderMeta{}
	if len(clientIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT client_order_id, coalesce(tag, ''), strategy FROM orders WHERE client_order_id = ANY($1)`, clientIDs)
	if err != nil {
		return nil, fmt.Errorf("order metas: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var m OrderMeta
		if err := rows.Scan(&id, &m.Tag, &m.Strategy); err != nil {
			return nil, err
		}
		out[id] = m
	}
	return out, rows.Err()
}
