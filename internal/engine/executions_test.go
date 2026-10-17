package engine

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
)

func (h *harness) fill(o domain.Order, id, qty, price, fee string, status domain.OrderStatus) {
	o.Status = status
	h.e.onBrokerEvent(broker.Event{Kind: broker.EventFill, Order: o, Fill: domain.Fill{
		OrderClientID: o.ClientID, TradeID: id, Exchange: o.Exchange, Market: o.Market, Symbol: o.Symbol,
		Side: o.Side, Quantity: decimal.RequireFromString(qty), Price: decimal.RequireFromString(price),
		Fee: decimal.RequireFromString(fee), Time: h.now,
	}})
}

func TestPartialFillReplayDoesNotChangeLedgerTwice(t *testing.T) {
	h := setup(t)
	var fills int
	h.e.cfg.Hooks.OnFill = func(domain.Fill, domain.Order) { fills++ }
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, Quantity: decimal.NewFromInt(2)})
	o := h.e.OpenOrders()[0]
	h.fill(o, "trade1", "0.5", "100", "0.1", domain.OrderPartiallyFilled)
	h.fill(o, "trade1", "0.5", "100", "0.1", domain.OrderPartiallyFilled)
	p, ok := h.a.Position(o.Symbol)
	require.True(t, ok)
	assert.Equal(t, "0.5", p.Quantity.String(), "replayed execution must be ignored")
	assert.Equal(t, "999.9", h.a.Snapshot().Balance.String())
	assert.Equal(t, 1, fills)
	h.fill(o, "trade2", "1.5", "102", "0.3", domain.OrderFilled)
	p, _ = h.a.Position(o.Symbol)
	assert.Equal(t, "2", p.Quantity.String())
	assert.Equal(t, "101.5", p.EntryPrice.String())
	assert.Equal(t, "999.6", h.a.Snapshot().Balance.String())
	assert.Equal(t, 2, fills)
	assert.Empty(t, h.e.OpenOrders())
}

func TestFilledSnapshotDoesNotDiscardUnseenExecutions(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, Quantity: decimal.NewFromInt(2)})
	o := h.e.OpenOrders()[0]
	// A reconnect/status snapshot can be newer than the individual trades
	// being replayed. Only executions advance the local filled quantity.
	h.fill(o, "trade1", "0.5", "100", "0", domain.OrderFilled)
	require.Len(t, h.e.OpenOrders(), 1)
	h.fill(o, "trade2", "1.5", "100", "0", domain.OrderFilled)
	p, _ := h.a.Position(o.Symbol)
	assert.Equal(t, "2", p.Quantity.String())
	assert.Empty(t, h.e.OpenOrders())
}

func TestPartialEntryGetsProtectionBeforeFinalFill(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, Quantity: decimal.NewFromInt(2),
		StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
	o := h.e.OpenOrders()[0]
	h.fill(o, "trade1", "0.5", "100", "0", domain.OrderPartiallyFilled)
	assert.Empty(t, h.e.Unprotected(), "even a partial entry must have a native stop")
	assertProtectionQty := func(want string) {
		t.Helper()
		var legs int
		for _, order := range h.e.OpenOrders() {
			if order.Tag == domain.TagStopLoss || order.Tag == domain.TagTakeProfit {
				legs++
				assert.Equal(t, want, order.Remaining().String())
			}
		}
		assert.Equal(t, 2, legs)
	}
	assertProtectionQty("0.5")
	h.fill(o, "trade2", "1.5", "100", "0", domain.OrderFilled)
	assertProtectionQty("2")
	assert.Empty(t, h.e.Unprotected())
}

func TestPartialEntryStopCancelsUnfilledRemainder(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, Quantity: decimal.NewFromInt(2),
		StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
	o := h.e.OpenOrders()[0]
	h.fill(o, "entry1", "0.5", "100", "0", domain.OrderPartiallyFilled)
	var stop domain.Order
	for _, order := range h.e.OpenOrders() {
		if order.Tag == domain.TagStopLoss {
			stop = order
		}
	}
	require.NotEmpty(t, stop.ClientID)
	h.fill(stop, "stop1", "0.5", "95", "0", domain.OrderFilled)
	assert.Empty(t, h.a.Positions())
	assert.Empty(t, h.e.OpenOrders(), "stop must also cancel the unfilled entry")
}

func TestPendingRawOrdersReserveExposure(t *testing.T) {
	h := setup(t)
	for range 2 {
		h.intent(strategy.Intent{Kind: strategy.Place, Order: strategy.OrderSpec{
			Type: domain.OrderLimit, Side: domain.Buy, Quantity: decimal.NewFromInt(8), Price: decimal.NewFromInt(100)}})
	}
	assert.Len(t, h.e.OpenOrders(), 1, "two pending orders of 800 must not exceed exposure limit 1500")
}

func TestPendingRawOrdersReservePositionSlots(t *testing.T) {
	h := setup(t)
	cfg := risk.DefaultConfig()
	cfg.MaxOpenPositions = 1
	h.e.cfg.Risk = risk.NewManager(cfg, h.e.log)
	h.intent(strategy.Intent{Kind: strategy.Place, Order: strategy.OrderSpec{
		Type: domain.OrderLimit, Side: domain.Buy, Quantity: decimal.NewFromInt(1), Price: decimal.NewFromInt(100)}})
	h.e.SetInstruments([]domain.Instrument{{Exchange: "sim", Market: domain.Futures, Symbol: "ETHUSDT", StepSize: decimal.RequireFromString("0.01")}})
	h.e.mu.Lock()
	h.e.apply(t.Context(), h.i, []strategy.Intent{{Kind: strategy.Place, Symbol: "ETHUSDT", Order: strategy.OrderSpec{
		Type: domain.OrderLimit, Side: domain.Buy, Quantity: decimal.NewFromInt(1), Price: decimal.NewFromInt(100)}}})
	h.e.unlockAndDrain(t.Context())
	assert.Len(t, h.e.OpenOrders(), 1, "a pending BTC order reserves the only position slot")
}

func TestPendingOrdersReduceMarginAvailableForSizing(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Place, Order: strategy.OrderSpec{
		Type: domain.OrderLimit, Side: domain.Buy, Quantity: decimal.NewFromInt(8), Price: decimal.NewFromInt(100)}})
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, Quantity: decimal.NewFromInt(10)})
	var entry domain.Order
	for _, o := range h.e.OpenOrders() {
		if o.Tag == domain.TagEntry {
			entry = o
		}
	}
	require.NotEmpty(t, entry.ClientID)
	assert.Equal(t, "1.96", entry.Quantity.String(), "only 200 margin remains; keep the 2% buffer")
}

func TestCancelAllIsScopedToExchange(t *testing.T) {
	h := setup(t)
	other := broker.NewSim(broker.SimConfig{Exchange: "other", Clock: func() time.Time { return h.now }})
	h.e.AddBroker(other)
	h.e.cfg.Book.Add(portfolio.NewAccount(portfolio.AccountKey{Exchange: "other", Market: domain.Futures}, decimal.NewFromInt(1000), nil))
	h.e.SetInstruments([]domain.Instrument{{Exchange: "other", Market: domain.Futures, Symbol: "BTCUSDT", StepSize: decimal.RequireFromString("0.01")}})
	for _, ex := range []string{"sim", "other"} {
		h.intent(strategy.Intent{Kind: strategy.Place, Exchange: ex, Order: strategy.OrderSpec{
			Type: domain.OrderLimit, Side: domain.Buy, Quantity: decimal.NewFromInt(1), Price: decimal.NewFromInt(100)}})
	}
	h.intent(strategy.Intent{Kind: strategy.CancelAll, Exchange: "sim"})
	orders := h.e.OpenOrders()
	require.Len(t, orders, 1)
	assert.Equal(t, "other", orders[0].Exchange)
}

func TestReversalWaitsForExitFill(t *testing.T) {
	h := setup(t)
	h.a.SetPosition(domain.Position{PositionKey: domain.PositionKey{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"},
		Side: domain.Long, Quantity: decimal.NewFromInt(1), EntryPrice: decimal.NewFromInt(100), Leverage: 1, Strategy: "test"})
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Short, Quantity: decimal.NewFromInt(2), StopLoss: decimal.NewFromInt(105)})
	orders := h.e.OpenOrders()
	require.Len(t, orders, 1, "new entry must wait until the opposite position is flat")
	assert.Equal(t, domain.TagExit, orders[0].Tag)
	h.fill(orders[0], "exit1", "1", "100", "0", domain.OrderFilled)
	orders = h.e.OpenOrders()
	require.Len(t, orders, 1)
	assert.Equal(t, domain.TagEntry, orders[0].Tag)
	h.fill(orders[0], "entry1", "2", "100", "0", domain.OrderFilled)
	p, ok := h.a.Position("BTCUSDT")
	require.True(t, ok)
	assert.Equal(t, domain.Short, p.Side)
	assert.Equal(t, "2", p.Quantity.String())
	orders = h.e.OpenOrders()
	require.Len(t, orders, 1)
	assert.Equal(t, "2", orders[0].Quantity.String())
	assert.Equal(t, domain.TagStopLoss, orders[0].Tag)
	h.e.CloseAll(context.Background(), "cleanup")
}

func TestPartialProtectiveFillResizesRemainingBracket(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, Quantity: decimal.NewFromInt(2), StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
	entry := h.e.OpenOrders()[0]
	h.fill(entry, "entry", "2", "100", "0", domain.OrderFilled)
	var stop domain.Order
	for _, o := range h.e.OpenOrders() {
		if o.Tag == domain.TagStopLoss {
			stop = o
		}
	}
	h.fill(stop, "stop-part", "0.5", "95", "0", domain.OrderPartiallyFilled)
	p, _ := h.a.Position("BTCUSDT")
	assert.Equal(t, "1.5", p.Quantity.String())
	orders := h.e.OpenOrders()
	require.Len(t, orders, 2)
	for _, o := range orders {
		assert.Equal(t, "1.5", o.Remaining().String())
	}
}

func TestPausedReversalDoesNotOpenAfterExit(t *testing.T) {
	h := setup(t)
	h.a.SetPosition(domain.Position{PositionKey: domain.PositionKey{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"}, Side: domain.Long,
		Quantity: decimal.NewFromInt(1), EntryPrice: decimal.NewFromInt(100), Leverage: 1})
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Short})
	exit := h.e.OpenOrders()[0]
	h.e.SetPaused(true)
	h.fill(exit, "exit", "1", "100", "0", domain.OrderFilled)
	assert.Empty(t, h.e.OpenOrders())
	assert.Empty(t, h.a.Positions())
	h.e.SetPaused(false)
	assert.Empty(t, h.e.OpenOrders(), "resuming does not replay a stale reversal")
}

type deferredCancel struct{ *broker.Sim }

func (deferredCancel) CancelOrder(context.Context, domain.Order) error { return nil }

func TestReservationPersistsUntilCancelAcknowledgement(t *testing.T) {
	h := setup(t)
	h.e.AddBroker(deferredCancel{h.s})
	in := strategy.Intent{Kind: strategy.Place, Order: strategy.OrderSpec{Type: domain.OrderLimit, Side: domain.Buy, Quantity: decimal.NewFromInt(8), Price: decimal.NewFromInt(100)}}
	h.intent(in)
	first := h.e.OpenOrders()[0]
	h.intent(strategy.Intent{Kind: strategy.CancelAll})
	h.intent(in)
	require.Len(t, h.e.OpenOrders(), 1, "a cancel request does not release exposure before acknowledgement")
	first.Status = domain.OrderCanceled
	h.e.onBrokerEvent(broker.Event{Kind: broker.EventOrder, Order: first})
	h.intent(in)
	orders := h.e.OpenOrders()
	require.Len(t, orders, 1)
	assert.NotEqual(t, first.ClientID, orders[0].ClientID)
}
