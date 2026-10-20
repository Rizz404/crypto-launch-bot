package strategy_test

import (
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func trendBar(px int64) domain.Candle {
	c := bar(px)
	c.Open = c.Close
	c.High = c.Close.Add(decimal.NewFromInt(1))
	c.Low = c.Close.Sub(decimal.NewFromInt(1))
	c.Closed = true
	return c
}
func trendCfg(mode string) strategy.Config {
	return cfg("trendspot", domain.Spot, map[string]any{"mode": mode, "timeframe": "1m", "trend_period": 2, "atr_period": 2, "entry_period": 3, "exit_period": 2})
}
func TestTrendSpotBreakoutUsesPriorBarsAndClosedCandles(t *testing.T) {
	c := ctx()
	s := makeStrategy(t, c, trendCfg("breakout"))
	for range 4 {
		assert.Empty(t, s.OnCandle(trendBar(100)))
	}
	forming := trendBar(1000)
	forming.Closed = false
	assert.Empty(t, s.OnCandle(forming))
	b := trendBar(102)
	b.High = decimal.NewFromInt(110)
	out := s.OnCandle(b)
	require.Len(t, out, 1)
	assert.Equal(t, strategy.Enter, out[0].Kind)
	assert.True(t, out[0].StopLoss.LessThan(b.Close))
	assert.True(t, out[0].StopLoss.IsPositive())
	// Including b.High=110 in the entry threshold would incorrectly suppress this close=102 entry.
	k := strategy.Leg{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT"}
	c.positions[k] = domain.Position{Side: domain.Long}
	assert.Empty(t, s.OnCandle(trendBar(112)), "held inventory must not add a new entry")
	out = s.OnCandle(trendBar(90))
	require.Len(t, out, 1)
	assert.Equal(t, strategy.Exit, out[0].Kind)
}
func TestTrendSpotMomentumWaitsForHistoryAndExitsDowntrend(t *testing.T) {
	c := ctx()
	s := makeStrategy(t, c, trendCfg("momentum"))
	for range 3 {
		assert.Empty(t, s.OnCandle(trendBar(100)))
	}
	out := s.OnCandle(trendBar(110))
	require.Len(t, out, 1)
	assert.Equal(t, strategy.Enter, out[0].Kind)
	c.positions[strategy.Leg{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT"}] = domain.Position{Side: domain.Long}
	out = s.OnCandle(trendBar(90))
	require.Len(t, out, 1)
	assert.Equal(t, strategy.Exit, out[0].Kind)
}
func TestTrendSpotPullbackWaitsForRecoveryAndDoesNotBuyFallingTrend(t *testing.T) {
	c := ctx()
	conf := trendCfg("pullback")
	conf.Params["trend_period"] = 10
	conf.Params["rsi_period"] = 2
	s := makeStrategy(t, c, conf)
	for px := int64(100); px <= 230; px += 10 {
		assert.Empty(t, s.OnCandle(trendBar(px)))
	}
	for _, px := range []int64{225, 220, 218} {
		assert.Empty(t, s.OnCandle(trendBar(px)), "wait for recovery")
	}
	out := s.OnCandle(trendBar(220))
	require.Len(t, out, 1)
	assert.Equal(t, strategy.Enter, out[0].Kind)
	c.positions[strategy.Leg{Exchange: "sim", Market: domain.Spot, Symbol: "BTCUSDT"}] = domain.Position{Side: domain.Long}
	out = s.OnCandle(trendBar(230))
	require.Len(t, out, 1)
	assert.Equal(t, strategy.Exit, out[0].Kind)
	c = ctx()
	s = makeStrategy(t, c, conf)
	for px := int64(230); px >= 100; px -= 5 {
		assert.Empty(t, s.OnCandle(trendBar(px)))
	}
	assert.Empty(t, s.OnCandle(trendBar(102)), "an RSI rebound below trend must not enter")
}
func TestTrendSpotValidatesParametersAndMarkets(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value any
	}{{"mode", "unknown"}, {"trend_period", 0}, {"stop_atr", 0}, {"rsi_entry", 90}, {"target_atr", -1}, {"unexpected", true}} {
		s, err := strategy.New("trendspot")
		require.NoError(t, err)
		conf := trendCfg("breakout")
		conf.Params[tc.key] = tc.value
		assert.Error(t, s.Init(ctx(), conf))
	}
	s, err := strategy.New("trendspot")
	require.NoError(t, err)
	conf := trendCfg("breakout")
	conf.Market = domain.Futures
	assert.Error(t, s.Init(ctx(), conf))
}
