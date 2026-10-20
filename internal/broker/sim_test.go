package broker

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
)

func TestSimFillPricesFeesAndEligibility(t *testing.T) {
	d := decimal.RequireFromString
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		typ   domain.OrderType
		side  domain.Side
		level string
		want  string
		maker bool
	}{
		{"market buy", domain.OrderMarket, domain.Buy, "0", "100.1", false},
		{"market sell", domain.OrderMarket, domain.Sell, "0", "99.9", false},
		{"buy limit", domain.OrderLimit, domain.Buy, "95", "95", true},
		{"sell limit", domain.OrderLimit, domain.Sell, "105", "105", true},
		{"buy limit better open", domain.OrderLimit, domain.Buy, "105", "100", true},
		{"sell stop", domain.OrderStopMarket, domain.Sell, "95", "94.905", false},
		{"buy stop gap", domain.OrderStopMarket, domain.Buy, "95", "100.1", false},
		{"sell target", domain.OrderTakeProfit, domain.Sell, "105", "104.895", false},
		{"buy target", domain.OrderTakeProfit, domain.Buy, "95", "95.095", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sim := NewSim(SimConfig{Exchange: "sim", Clock: func() time.Time { return at }, SlippageBps: 10,
				Fees: map[domain.Market]Fees{domain.Futures: {Maker: 0.001, Taker: 0.002}}})
			var events []Event
			sim.SetHandler(func(e Event) { events = append(events, e) })
			o := domain.Order{ClientID: "tb1", Symbol: "BTCUSDT", Market: domain.Futures, Side: tc.side, Type: tc.typ,
				Quantity: d("2"), Price: d(tc.level), StopPrice: d(tc.level)}
			_, err := sim.PlaceOrder(context.Background(), o)
			require.NoError(t, err)
			_, err = sim.PlaceOrder(context.Background(), o)
			assert.ErrorContains(t, err, "duplicate")
			c := domain.Candle{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT", OpenTime: at.Add(-time.Minute),
				Open: d("100"), High: d("110"), Low: d("90"), Close: d("100")}
			sim.OnCandle(c)
			assert.Empty(t, events, "cannot fill an order using a candle opened before placement")
			c.OpenTime = at
			sim.OnCandle(c)
			require.Len(t, events, 1)
			f := events[0].Fill
			assert.Equal(t, tc.want, f.Price.String())
			assert.Equal(t, tc.maker, f.Maker)
			rate := d("0.002")
			if tc.maker {
				rate = d("0.001")
			}
			assert.True(t, f.Fee.Equal(d(tc.want).Mul(d("2")).Mul(rate)))
			assert.Equal(t, domain.OrderFilled, events[0].Order.Status)
			assert.Empty(t, sim.OpenOrders())
			sim.OnCandle(c)
			assert.Len(t, events, 1, "filled order cannot fill again")
		})
	}
}

func TestSimStopBeforeTargetAndSynchronousCancel(t *testing.T) {
	d := decimal.NewFromInt
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewSim(SimConfig{Exchange: "sim", Clock: func() time.Time { return at }})
	stop := domain.Order{ClientID: "stop", Symbol: "BTCUSDT", Market: domain.Futures, Side: domain.Sell,
		Type: domain.OrderStopMarket, Quantity: d(1), StopPrice: d(95)}
	target := stop
	target.ClientID, target.Type, target.StopPrice = "target", domain.OrderTakeProfit, d(105)
	_, err := s.PlaceOrder(context.Background(), target)
	require.NoError(t, err)
	_, err = s.PlaceOrder(context.Background(), stop)
	require.NoError(t, err)
	var fills []string
	s.SetHandler(func(ev Event) {
		if ev.Kind == EventFill {
			fills = append(fills, ev.Order.ClientID)
			require.NoError(t, s.CancelOrder(context.Background(), target))
		}
	})
	s.OnCandle(domain.Candle{Market: domain.Futures, Symbol: "BTCUSDT", OpenTime: at,
		Open: d(100), High: d(110), Low: d(90)})
	assert.Equal(t, []string{"stop"}, fills)
	assert.Empty(t, s.OpenOrders())
}

func TestSimRestoredPartialOrderFillsOnlyRemainingQuantity(t *testing.T) {
	d := decimal.RequireFromString
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewSim(SimConfig{Exchange: "sim", Clock: func() time.Time { return at }, Fees: map[domain.Market]Fees{domain.Futures: {Taker: 0.001}}})
	o, err := s.PlaceOrder(context.Background(), domain.Order{ClientID: "restored", Market: domain.Futures, Symbol: "BTCUSDT", Side: domain.Buy,
		Type: domain.OrderMarket, Quantity: d("2"), FilledQty: d("0.5"), AvgPrice: d("90")})
	require.NoError(t, err)
	assert.Equal(t, domain.OrderPartiallyFilled, o.Status)
	var events []Event
	s.SetHandler(func(ev Event) { events = append(events, ev) })
	s.OnCandle(domain.Candle{Market: domain.Futures, Symbol: "BTCUSDT", OpenTime: at, Open: d("100"), High: d("100"), Low: d("100")})
	require.Len(t, events, 1)
	assert.Equal(t, "1.5", events[0].Fill.Quantity.String())
	assert.Equal(t, "0.15", events[0].Fill.Fee.String())
	assert.Equal(t, "2", events[0].Order.FilledQty.String())
	assert.Equal(t, "97.5", events[0].Order.AvgPrice.String())
}

// Orders placed at the same clock instant fill in placement order on every
// run, so backtests and recovery tests are reproducible.
func TestSimFillsSimultaneousOrdersInPlacementOrder(t *testing.T) {
	at := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	ids := []string{"zz", "aa", "mm", "bb", "yy", "cc"}
	for range 50 {
		s := NewSim(SimConfig{Exchange: "sim", Clock: func() time.Time { return at }})
		var filled []string
		s.SetHandler(func(ev Event) {
			if ev.Kind == EventFill {
				filled = append(filled, ev.Fill.OrderClientID)
			}
		})
		for _, id := range ids {
			_, err := s.PlaceOrder(context.Background(), domain.Order{ClientID: id, Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Side: domain.Buy, Type: domain.OrderMarket, Quantity: decimal.NewFromInt(1)})
			require.NoError(t, err)
		}
		c := domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: at, Open: decimal.NewFromInt(100), High: decimal.NewFromInt(100), Low: decimal.NewFromInt(100), Close: decimal.NewFromInt(100), Closed: true}
		require.NoError(t, s.MatchCandle(c, nil))
		assert.Equal(t, ids, filled)
	}
}
