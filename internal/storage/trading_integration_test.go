//go:build integration

package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
)

func TestTradingPersistenceAndSessionRecovery(t *testing.T) {
	s, ex := testStore(t)
	ctx := context.Background()
	d := decimal.RequireFromString
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var ids []int64
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = s.pool.Exec(ctx, "DELETE FROM fills WHERE order_id IN (SELECT id FROM orders WHERE run_id=$1)", id)
			for _, table := range []string{"orders", "positions", "trades", "equity_snapshots"} {
				_, _ = s.pool.Exec(ctx, "DELETE FROM "+table+" WHERE run_id=$1", id)
			}
			_, _ = s.pool.Exec(ctx, "DELETE FROM strategy_runs WHERE id=$1", id)
		}
	})
	create := func() int64 {
		id, err := s.CreateRun(ctx, Run{InstanceID: "session", Strategy: "session", Mode: "paper", Exchange: ex, Market: domain.Futures, Symbols: []string{"BTCUSDT"}, StartedAt: time.Now()})
		require.NoError(t, err)
		ids = append(ids, id)
		return id
	}
	id := create()
	o := domain.Order{ClientID: fmt.Sprintf("tbtest%d", id), ExchangeID: fmt.Sprint(id), Exchange: ex, Market: domain.Futures,
		Symbol: "BTCUSDT", Side: domain.Buy, Type: domain.OrderLimit, Price: d("100.01"), Quantity: d("0.12345678"),
		Status: domain.OrderNew, Tag: domain.TagStrategy, Strategy: "grid", CreatedAt: at, UpdatedAt: at}
	f := domain.Fill{OrderClientID: o.ClientID, TradeID: "trade1", Price: o.Price, Quantity: o.Quantity, Fee: d("0.000123"), Time: at, Maker: true}
	assert.Error(t, s.InsertFill(ctx, f), "fill requires its parent order")
	require.NoError(t, s.UpsertOrder(ctx, id, o))
	require.NoError(t, s.InsertFill(ctx, f))
	var count int
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM fills WHERE trade_id='trade1' AND order_id=(SELECT id FROM orders WHERE client_order_id=$1)", o.ClientID).Scan(&count))
	assert.Equal(t, 1, count)
	orders, err := s.OpenOrders(ctx, id)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.True(t, o.Quantity.Equal(orders[0].Quantity))
	metas, err := s.OrderMetas(ctx, []string{o.ClientID})
	require.NoError(t, err)
	assert.Equal(t, OrderMeta{Tag: domain.TagStrategy, Strategy: "grid"}, metas[o.ClientID])

	newID := create()
	require.NoError(t, s.UpsertOrder(ctx, newID, o))
	orders, err = s.OpenOrders(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, orders, "adopted orders belong to the active run")
	ps := []domain.Position{{PositionKey: domain.PositionKey{Exchange: ex, Market: domain.Futures, Symbol: "BTCUSDT"}, Side: domain.Long,
		Quantity: d("0.12345678"), EntryPrice: d("100.01"), Fees: d("0.000123"), Leverage: 2, Strategy: "grid", OpenedAt: at}}
	require.NoError(t, s.ReplaceOpenPositions(ctx, newID, ex, domain.Futures, ps))
	require.NoError(t, s.InsertEquity(ctx, newID, ex+":futures", []EquityPoint{{Time: at, Balance: d("999.99"), Equity: d("1001.01")},
		{Time: at.Add(time.Minute), Balance: d("1000.12"), Equity: d("1002.01")}}))
	state, err := s.LastSessionState(ctx, "paper")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, newID, state.RunID)
	assert.Equal(t, "1000.12", state.Balances[ex+":futures"].String())
	require.Len(t, state.Positions, 1)
	assert.Equal(t, "0.12345678", state.Positions[0].Quantity.String())
	assert.Equal(t, "grid", state.Positions[0].Strategy)
	require.NoError(t, s.ReplaceOpenPositions(ctx, newID, ex, domain.Futures, nil))
	state, err = s.LastSessionState(ctx, "paper")
	require.NoError(t, err)
	assert.Empty(t, state.Positions)
	require.NoError(t, s.FinishRun(ctx, newID, "completed", map[string]any{"TotalReturn": 0.01}))
	run, err := s.GetRun(ctx, newID)
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.NotNil(t, run.StoppedAt)
	assert.JSONEq(t, `{"TotalReturn":0.01}`, string(run.Summary))
}

// Rollbacks are opt-in and only allowed on a dedicated database ending
// in _test; normal integration runs never roll back an existing database.
func TestMigrationCycle(t *testing.T) {
	if os.Getenv("TB_TEST_MIGRATIONS") != "1" {
		t.Skip("set TB_TEST_MIGRATIONS=1 on an isolated *_test database")
	}
	pool, err := Open(context.Background(), os.Getenv("TB_TEST_DATABASE_URL"), 2)
	require.NoError(t, err)
	defer pool.Close()
	var name string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&name))
	require.True(t, strings.HasSuffix(name, "_test"), "refusing rollbacks outside a dedicated test database")
	m, err := NewMigrator(pool)
	require.NoError(t, err)
	defer func() { _ = m.Close() }()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	require.NoError(t, m.Up(ctx, log))
	for want := 1; want <= 3; want++ {
		require.NoError(t, m.Down(ctx, log))
		n, err := m.Pending(ctx)
		require.NoError(t, err)
		assert.Equal(t, want, n)
	}
	require.NoError(t, m.Up(ctx, log))
	n, err := m.Pending(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)
}
