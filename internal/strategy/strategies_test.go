package strategy_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/strategy"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
)

type testContext struct {
	now       time.Time
	positions map[strategy.Leg]domain.Position
	prices    map[strategy.Leg]decimal.Decimal
	orders    []domain.Order
	funding   strategy.FundingInfo
}

func ctx() *testContext {
	return &testContext{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), positions: map[strategy.Leg]domain.Position{}, prices: map[strategy.Leg]decimal.Decimal{}}
}
func (c *testContext) Now() time.Time  { return c.now }
func (*testContext) Log() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func (c *testContext) Position(ex string, m domain.Market, sym string) (domain.Position, bool) {
	p, ok := c.positions[strategy.Leg{Exchange: ex, Market: m, Symbol: sym}]
	return p, ok
}
func (*testContext) Account(string, domain.Market) portfolio.Snapshot { return portfolio.Snapshot{} }
func (*testContext) Instrument(ex string, m domain.Market, sym string) (domain.Instrument, bool) {
	return domain.Instrument{Exchange: ex, Market: m, Symbol: sym, StepSize: decimal.RequireFromString("0.001"), TickSize: decimal.RequireFromString("0.01")}, true
}
func (c *testContext) OpenOrders(string) []domain.Order { return c.orders }
func (c *testContext) LastPrice(ex string, m domain.Market, sym string) (decimal.Decimal, bool) {
	p, ok := c.prices[strategy.Leg{Exchange: ex, Market: m, Symbol: sym}]
	return p, ok
}
func (c *testContext) Funding(string, string) (strategy.FundingInfo, bool) {
	return c.funding, !c.funding.Rate.IsZero()
}
func cfg(name string, m domain.Market, params map[string]any) strategy.Config {
	return strategy.Config{ID: name, Name: name, Exchange: "sim", Market: m, Symbols: []string{"BTCUSDT"}, Params: params}
}
func makeStrategy(t *testing.T, c *testContext, cfg strategy.Config) strategy.Strategy {
	t.Helper()
	s, err := strategy.New(cfg.Name)
	require.NoError(t, err)
	require.NoError(t, s.Init(c, cfg))
	return s
}
func bar(px int64) domain.Candle {
	return domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT", Timeframe: "1m", Close: decimal.NewFromInt(px)}
}

func TestGridLevelsFillRestartAndStop(t *testing.T) {
	c := ctx()
	conf := cfg("grid", domain.Spot, map[string]any{"lower": 90, "upper": 110, "levels": 4, "quote_per_level": 10, "stop_below": 85})
	s := makeStrategy(t, c, conf)
	orders := s.OnCandle(bar(101))
	require.Len(t, orders, 3)
	assert.Equal(t, "90", orders[0].Order.Price.String())
	assert.Equal(t, "95", orders[1].Order.Price.String())
	assert.Equal(t, "100", orders[2].Order.Price.String())
	assert.Empty(t, s.OnCandle(bar(101)), "startup grid placed once")
	fh := s.(strategy.FillHandler)
	o := domain.Order{Symbol: "BTCUSDT", Market: domain.Spot, Side: domain.Buy, Price: decimal.RequireFromString("94.99"),
		Status: domain.OrderPartiallyFilled, Tag: domain.TagStrategy, FilledQty: decimal.NewFromInt(1)}
	assert.Empty(t, fh.OnFill(domain.Fill{}, o))
	o.Status = domain.OrderFilled
	response := fh.OnFill(domain.Fill{}, o)
	require.Len(t, response, 1)
	assert.Equal(t, domain.Sell, response[0].Order.Side)
	assert.Equal(t, "100", response[0].Order.Price.String())
	assert.True(t, response[0].Order.Quantity.Equal(o.FilledQty))
	c.orders = []domain.Order{o}
	s = makeStrategy(t, c, conf)
	assert.Empty(t, s.OnCandle(bar(101)), "restart must preserve existing orders")
	stop := s.OnCandle(bar(80))
	require.Len(t, stop, 2)
	assert.Equal(t, strategy.CancelAll, stop[0].Kind)
	assert.Equal(t, strategy.Exit, stop[1].Kind)
	assert.Empty(t, s.OnCandle(bar(101)), "stopped grid stays stopped")
}

func TestDCAScheduleDipLimitAndAverageEntryExit(t *testing.T) {
	c := ctx()
	k := strategy.Leg{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT"}
	c.prices[k] = decimal.NewFromInt(100)
	s := makeStrategy(t, c, cfg("dca", domain.Spot, map[string]any{"quote_amount": 10, "interval": "1h", "max_buys": 2, "dip_pct": 0.05, "take_profit": 0.1}))
	timer := s.(strategy.TimerHandler)
	orders := timer.OnTimer(c.now)
	require.Len(t, orders, 1)
	assert.Equal(t, "0.1", orders[0].Order.Quantity.String())
	assert.Empty(t, timer.OnTimer(c.now.Add(30*time.Minute)))
	assert.Empty(t, timer.OnTimer(c.now.Add(time.Hour)), "unchanged price fails dip filter")
	c.prices[k] = decimal.NewFromInt(90)
	require.Len(t, timer.OnTimer(c.now.Add(time.Hour)), 1)
	assert.Empty(t, timer.OnTimer(c.now.Add(2*time.Hour)), "max buys reached")
	c.positions[k] = domain.Position{EntryPrice: decimal.NewFromInt(95), Quantity: decimal.NewFromInt(1), Side: domain.Long}
	exit := s.OnCandle(bar(105))
	require.Len(t, exit, 1)
	assert.Equal(t, strategy.Exit, exit[0].Kind)
	delete(c.positions, k)
	require.Len(t, timer.OnTimer(c.now.Add(3*time.Hour)), 1, "starts a fresh cycle")
}

// After a restart the counters are gone but the position is restored: the
// cycle must not restart at buy 1 and buy again at once.
func TestDCAResumesCycleFromRestoredPosition(t *testing.T) {
	c := ctx()
	k := strategy.Leg{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT"}
	c.prices[k] = decimal.NewFromInt(80)
	// Three buys of 10 quote each: cost 30 at an average of 100.
	c.positions[k] = domain.Position{EntryPrice: decimal.NewFromInt(100), Quantity: decimal.RequireFromString("0.3"), Side: domain.Long}
	s := makeStrategy(t, c, cfg("dca", domain.Spot, map[string]any{"quote_amount": 10, "interval": "1h", "max_buys": 4, "dip_pct": 0.05, "take_profit": 0.1}))
	timer := s.(strategy.TimerHandler)
	assert.Empty(t, timer.OnTimer(c.now), "the next buy waits a full interval after the restart")
	orders := timer.OnTimer(c.now.Add(time.Hour))
	require.Len(t, orders, 1)
	assert.Equal(t, "dca buy 4/4", orders[0].Reason, "counts the three buys the position already holds")
	c.prices[k] = decimal.NewFromInt(50)
	assert.Empty(t, timer.OnTimer(c.now.Add(2*time.Hour)), "max buys reached")
}

func TestFundingHedgeDirectionCooldownAndRepair(t *testing.T) {
	c := ctx()
	spot := strategy.Leg{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT"}
	perp := strategy.Leg{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"}
	c.prices[perp] = decimal.NewFromInt(100)
	conf := cfg("funding", domain.Futures, map[string]any{"quote_amount": 10})
	conf.Legs = []strategy.Leg{spot}
	s := makeStrategy(t, c, conf).(strategy.TimerHandler)
	assert.Empty(t, s.OnTimer(c.now), "funding must be known")
	c.funding.Rate = decimal.RequireFromString("0.001")
	orders := s.OnTimer(c.now)
	require.Len(t, orders, 2)
	assert.Equal(t, domain.Buy, orders[0].Order.Side)
	assert.Equal(t, domain.Spot, orders[0].Market)
	assert.Equal(t, domain.Sell, orders[1].Order.Side)
	assert.Equal(t, domain.Futures, orders[1].Market)
	assert.True(t, orders[0].Order.Quantity.Equal(orders[1].Order.Quantity))
	assert.Empty(t, s.OnTimer(c.now.Add(time.Minute)), "cooldown prevents duplicate legs")
	c.positions[spot] = domain.Position{Side: domain.Long, Quantity: decimal.NewFromInt(1)}
	assert.Empty(t, s.OnTimer(c.now.Add(2*time.Minute)))
	fix := s.OnTimer(c.now.Add(4 * time.Minute))
	require.Len(t, fix, 2)
	assert.Equal(t, strategy.Exit, fix[0].Kind)
	assert.Equal(t, strategy.Exit, fix[1].Kind)
}

func TestCrossExchangeSpreadDirection(t *testing.T) {
	for _, rich := range []bool{true, false} {
		c := ctx()
		a := strategy.Leg{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"}
		b := strategy.Leg{Exchange: "other", Market: domain.Futures, Symbol: "BTCUSDT"}
		c.prices[a] = decimal.NewFromInt(100)
		c.prices[b] = decimal.NewFromInt(99)
		if rich {
			c.prices[b] = decimal.NewFromInt(101)
		}
		conf := cfg("xspread", domain.Futures, map[string]any{"quote_amount": 10})
		conf.Legs = []strategy.Leg{b}
		s := makeStrategy(t, c, conf)
		orders := s.OnCandle(bar(100))
		require.Len(t, orders, 2)
		assert.Equal(t, map[bool]domain.Side{true: domain.Buy, false: domain.Sell}[rich], orders[0].Order.Side)
		assert.Equal(t, orders[0].Order.Side.Opposite(), orders[1].Order.Side)
		assert.Empty(t, s.OnCandle(bar(100)), "orders still in flight")
		c.now = c.now.Add(2 * time.Minute)
		c.positions[a], c.positions[b] = domain.Position{Quantity: decimal.NewFromInt(1)}, domain.Position{Quantity: decimal.NewFromInt(1)}
		c.prices[b] = c.prices[a]
		orders = s.OnCandle(bar(100))
		require.Len(t, orders, 2)
		assert.Equal(t, strategy.Exit, orders[0].Kind)
	}
}

func TestMarketMakerSpotInventoryAndRequote(t *testing.T) {
	c := ctx()
	s := makeStrategy(t, c, cfg("mm", domain.Spot, map[string]any{"order_quote": 10, "max_inventory_quote": 100, "min_requote": "1s"}))
	orders := s.OnCandle(bar(100))
	require.Len(t, orders, 2, "cancel + bid; no naked ask on spot")
	assert.Equal(t, strategy.CancelAll, orders[0].Kind)
	assert.Equal(t, domain.Buy, orders[1].Order.Side)
	assert.True(t, orders[1].Order.PostOnly)
	c.orders = []domain.Order{{ClientID: "bid"}}
	c.now = c.now.Add(2 * time.Second)
	assert.Empty(t, s.OnCandle(bar(100)), "unchanged quote is retained")
	c.positions[strategy.Leg{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT"}] = domain.Position{Side: domain.Long, Quantity: decimal.NewFromInt(1)}
	s.(strategy.FillHandler).OnFill(domain.Fill{}, domain.Order{Symbol: "BTCUSDT"})
	orders = s.OnCandle(bar(100))
	require.Len(t, orders, 2, "full inventory: cancel + ask")
	assert.Equal(t, domain.Sell, orders[1].Order.Side)
}

func TestStrategyUnknownParametersRejected(t *testing.T) {
	s, err := strategy.New("emacross")
	require.NoError(t, err)
	assert.ErrorContains(t, s.Init(ctx(), cfg("emacross", domain.Spot, map[string]any{"typo": 1})), "typo")
	_, err = strategy.New("not-registered")
	assert.ErrorContains(t, err, "unknown strategy")
}
