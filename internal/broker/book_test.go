package broker

import (
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBookSpreadSharedDepthAndPartialRecovery(t *testing.T) {
	d := decimal.RequireFromString
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	cfg := SimConfig{Exchange: "binance", Clock: func() time.Time { return now }, SlippageBps: 2, Fees: map[domain.Market]Fees{domain.Spot: {Taker: 0.001}}}
	s := NewSim(cfg)
	var events []Event
	s.SetHandler(func(e Event) { events = append(events, e) })
	for _, id := range []string{"first", "second"} {
		_, err := s.PlaceOrder(t.Context(), domain.Order{ClientID: id, Market: domain.Spot, Symbol: "ETHUSDT", Side: domain.Buy, Type: domain.OrderMarket, Quantity: d("1.8")})
		require.NoError(t, err)
	}
	q := domain.BookQuote{Exchange: "binance", Market: domain.Spot, Symbol: "ETHUSDT", Bid: d("99"), Ask: d("101"), BidQty: d("10"), AskQty: d("10"), At: now}
	require.NoError(t, s.MatchBook(q, d("0.3"), nil))
	require.Len(t, events, 1, "two orders cannot reuse the same available ask depth")
	require.Equal(t, "0.9", events[0].Fill.Quantity.String(), "participation is rounded down to the lot step")
	require.Equal(t, "101.0202", events[0].Fill.Price.String())
	require.Equal(t, domain.OrderPartiallyFilled, events[0].Order.Status)
	require.Equal(t, "0.09091818", events[0].Fill.Fee.String())
	firstTrade := events[0].Fill.TradeID
	restored := NewSim(cfg)
	for _, o := range s.OpenOrders() {
		_, err := restored.PlaceOrder(t.Context(), o)
		require.NoError(t, err)
	}
	events = nil
	restored.SetHandler(func(e Event) { events = append(events, e) })
	now = now.Add(time.Second)
	q.At, q.AskQty = now, d("30")
	require.NoError(t, restored.MatchBook(q, d("0.3"), nil))
	require.Len(t, events, 2)
	require.NotEqual(t, firstTrade, events[0].Fill.TradeID)
	require.Empty(t, restored.OpenOrders())
	quantity := events[0].Fill.Quantity.Add(events[1].Fill.Quantity)
	require.Equal(t, "2.7", quantity.String(), "restored partial order fills only its remainder")
}

func TestBookGapStopRemainsTriggeredAfterPartialRestart(t *testing.T) {
	d := decimal.RequireFromString
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	cfg := SimConfig{Exchange: "binance", Clock: func() time.Time { return now }, SlippageBps: 2}
	s := NewSim(cfg)
	_, err := s.PlaceOrder(t.Context(), domain.Order{ClientID: "stop", Market: domain.Spot, Symbol: "ETHUSDT", Side: domain.Sell, Type: domain.OrderStopMarket, StopPrice: d("95"), Quantity: d("2")})
	require.NoError(t, err)
	var events []Event
	s.SetHandler(func(e Event) { events = append(events, e) })
	q := domain.BookQuote{Exchange: "binance", Market: domain.Spot, Symbol: "ETHUSDT", Bid: d("90"), Ask: d("91"), BidQty: d("10"), AskQty: d("10"), At: now}
	require.NoError(t, s.MatchBook(q, d("0.001"), nil))
	require.Len(t, events, 1)
	require.Equal(t, "89.982", events[0].Fill.Price.String(), "a gap is filled at the worse bid, never at the old stop")
	require.Equal(t, "1", events[0].Fill.Quantity.String())
	restored := NewSim(cfg)
	_, err = restored.PlaceOrder(t.Context(), events[0].Order)
	require.NoError(t, err)
	restored.SetHandler(func(e Event) { events = append(events, e) })
	now = now.Add(time.Second)
	q.At, q.Bid, q.Ask = now, d("100"), d("101")
	require.NoError(t, restored.MatchBook(q, d("0.001"), nil))
	require.Len(t, events, 2)
	require.Equal(t, "99.98", events[1].Fill.Price.String())
	require.Equal(t, domain.OrderFilled, events[1].Order.Status)
	q.At = now.Add(-time.Minute)
	require.Error(t, restored.MatchBook(q, d("0.001"), nil))
}
