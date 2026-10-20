package engine

import (
	"testing"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCheckpointRestoresPendingProtectionPlanAndTrailing(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(120), TrailingStop: .05})
	raw, err := h.e.Checkpoint()
	require.NoError(t, err)
	r := setup(t)
	require.NoError(t, r.e.Restore(raw))
	for _, o := range r.e.OpenOrders() {
		_, err = r.s.PlaceOrder(t.Context(), o)
		require.NoError(t, err)
	}
	r.s.OnCandle(r.bar(100, 101, 99, 100))
	require.Empty(t, r.e.Unprotected())
	require.Len(t, r.e.OpenOrders(), 2)
	raw, err = r.e.Checkpoint()
	require.NoError(t, err)
	restored := setup(t)
	require.NoError(t, restored.e.Restore(raw))
	require.Empty(t, restored.e.Unprotected())
	require.Len(t, restored.e.protections, 1)
	for _, p := range restored.e.protections {
		require.Equal(t, .05, p.trailing)
		require.Equal(t, "120", p.takeProfit.String())
	}
	for _, o := range restored.e.OpenOrders() {
		_, err = restored.s.PlaceOrder(t.Context(), o)
		require.NoError(t, err)
	}
	restored.s.OnCandle(restored.bar(100, 121, 90, 100))
	require.Empty(t, restored.a.Positions())
	require.Empty(t, restored.e.OpenOrders())
}
