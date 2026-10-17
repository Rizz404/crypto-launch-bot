package backtest

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
)

func TestMetricsReferenceAndJSON(t *testing.T) {
	d := decimal.NewFromInt
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	eq := []EquityPoint{{at, d(100)}, {at.Add(24 * time.Hour), d(120)}, {at.Add(365 * 24 * time.Hour), d(90)}}
	trades := []domain.Trade{
		{GrossPnL: d(25), Fees: d(3), Funding: d(2), OpenedAt: at, ClosedAt: at.Add(time.Hour)},
		{GrossPnL: d(-20), Fees: d(5), Funding: d(5), OpenedAt: at, ClosedAt: at.Add(3 * time.Hour)},
	}
	m := Compute(d(100), eq, trades)
	assert.InDelta(t, -0.1, m.TotalReturn, 1e-12)
	assert.InDelta(t, -0.1, m.CAGR, 1e-12)
	assert.Equal(t, 0.25, m.MaxDrawdown)
	assert.Equal(t, 0.5, m.WinRate)
	assert.InDelta(t, 2.0/3, m.ProfitFactor, 1e-12)
	assert.Equal(t, -5.0, m.AvgTrade)
	assert.Equal(t, 8.0, m.TotalFees)
	assert.Equal(t, 7.0, m.TotalFunding)
	assert.Equal(t, 2*time.Hour, m.AvgHoldingTime)
	m = Compute(d(100), eq, trades[:1])
	assert.True(t, math.IsInf(m.ProfitFactor, 1))
	b, err := json.Marshal(m)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"ProfitFactor":null`)
	assert.NotContains(t, string(b), `"ProfitFactor":0`)
	assert.Zero(t, Compute(d(100), nil, nil).Trades)
}

func TestRatiosAndDailyUTCSampling(t *testing.T) {
	at := time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)
	d := decimal.NewFromInt
	rets := dailyReturns(100, []EquityPoint{{at, d(110)}, {at.Add(time.Hour), d(99)}, {at.Add(2 * time.Hour), d(121)}})
	require.Len(t, rets, 2)
	assert.InDelta(t, 0.1, rets[0], 1e-12)
	assert.InDelta(t, 0.1, rets[1], 1e-12)
	s, so := ratios([]float64{0.1, -0.1})
	assert.Zero(t, s)
	assert.Zero(t, so)
	s, so = ratios([]float64{0.2, -0.1})
	assert.InDelta(t, 0.05/math.Sqrt(0.045)*math.Sqrt(365), s, 1e-12)
	assert.InDelta(t, 0.05/math.Sqrt(0.005)*math.Sqrt(365), so, 1e-12)
}

type candleSource func(context.Context, domain.SeriesKey, time.Time, time.Time) ([]domain.Candle, error)

func (f candleSource) Candles(ctx context.Context, k domain.SeriesKey, from, to time.Time) ([]domain.Candle, error) {
	return f(ctx, k, from, to)
}

func syntheticConfig() Config {
	d := decimal.NewFromInt
	r := risk.DefaultConfig()
	r.DailyLossLimit, r.MaxDrawdown = 0, 0
	return Config{
		Strategy: strategy.Config{ID: "golden", Name: "emacross", Exchange: "sim", Market: domain.Futures, Symbols: []string{"BTCUSDT"}, Leverage: 1,
			Params: map[string]any{"fast": 2, "slow": 4, "rsi_period": 2, "atr_period": 2, "rsi_long_max": 100, "rsi_short_min": 0}},
		From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), InitialBalance: d(1000), Risk: r,
		Fees: map[domain.Market]broker.Fees{domain.Futures: {Maker: 0.001, Taker: 0.002}}, SlippageBps: 10,
		Instruments: []domain.Instrument{{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT", TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.01"), MinNotional: d(5)}},
	}
}

func syntheticSource(_ context.Context, k domain.SeriesKey, from, _ time.Time) ([]domain.Candle, error) {
	var out []domain.Candle
	for i := range 120 {
		phase := i % 20
		if phase > 10 {
			phase = 20 - phase
		}
		px := decimal.NewFromInt(int64(100 + phase*2))
		out = append(out, domain.Candle{Exchange: k.Exchange, Market: k.Market, Symbol: k.Symbol, Timeframe: k.Timeframe,
			OpenTime: from.Add(time.Duration(i) * k.Timeframe.Duration()), Open: px, Close: px,
			High: px.Add(decimal.NewFromInt(1)), Low: px.Sub(decimal.NewFromInt(1)), Closed: true})
	}
	return out, nil
}

func TestEMACrossGoldenAndCSV(t *testing.T) {
	cfg := syntheticConfig()
	r, err := Run(context.Background(), cfg, candleSource(syntheticSource))
	require.NoError(t, err)
	require.NotEmpty(t, r.Trades)
	// Freeze a compact ledger/curve, avoiding platform-sensitive CAGR floats.
	type golden struct {
		Candles, Trades int
		Final, Drawdown string
		Exits           []string
	}
	g := golden{Candles: r.Candles, Trades: len(r.Trades), Final: r.Equity[len(r.Equity)-1].Equity.String(), Drawdown: decimal.NewFromFloat(r.Metrics.MaxDrawdown).StringFixed(8)}
	for _, tr := range r.Trades {
		g.Exits = append(g.Exits, tr.ExitReason+":"+tr.NetPnL().String())
	}
	b, err := json.MarshalIndent(g, "", "  ")
	require.NoError(t, err)
	want, err := os.ReadFile("testdata/emacross_synthetic.json")
	require.NoError(t, err, "actual golden: %s", b)
	assert.JSONEq(t, string(want), string(b))
	dir := t.TempDir()
	require.NoError(t, WriteCSV(dir, "golden", r))
	f, err := os.Open(filepath.Join(dir, "golden-trades.csv"))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, len(r.Trades)+1)
	assert.Equal(t, "net_pnl", rows[0][10])
	assert.Equal(t, r.Trades[0].NetPnL().StringFixed(4), rows[1][10])
}

func TestBacktestPropagatesSourceAndCancellationErrors(t *testing.T) {
	cfg := syntheticConfig()
	want := errors.New("load failed")
	_, err := Run(context.Background(), cfg, candleSource(func(context.Context, domain.SeriesKey, time.Time, time.Time) ([]domain.Candle, error) {
		return nil, want
	}))
	assert.ErrorIs(t, err, want)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Run(ctx, cfg, candleSource(syntheticSource))
	assert.ErrorIs(t, err, context.Canceled)
}

type buyOnce struct {
	cfg  strategy.Config
	seen map[string]bool
}

func (s *buyOnce) Init(_ strategy.Context, cfg strategy.Config) error {
	s.cfg = cfg
	s.seen = map[string]bool{}
	return nil
}
func (*buyOnce) Timeframes() []domain.Timeframe { return []domain.Timeframe{"1h"} }
func (s *buyOnce) OnCandle(c domain.Candle) []strategy.Intent {
	k := c.Exchange + ":" + c.Symbol
	if s.seen[k] {
		return nil
	}
	s.seen[k] = true
	return []strategy.Intent{{Kind: strategy.Place, Exchange: c.Exchange, Market: c.Market, Symbol: c.Symbol,
		Order: strategy.OrderSpec{Type: domain.OrderMarket, Side: domain.Buy, Quantity: decimal.NewFromInt(1)}}}
}
func init() { strategy.Register("test-buy-once", func() strategy.Strategy { return &buyOnce{} }) }

func TestMultiAccountEquityIncludesAllEventsAtSameTimestamp(t *testing.T) {
	cfg := syntheticConfig()
	cfg.Strategy.Name, cfg.Strategy.Params = "test-buy-once", nil
	cfg.Strategy.Legs = []strategy.Leg{{Exchange: "other", Market: domain.Futures, Symbol: "BTCUSDT"}}
	ins := cfg.Instruments[0]
	ins.Exchange = "other"
	cfg.Instruments = append(cfg.Instruments, ins)
	cfg.Fees, cfg.SlippageBps = nil, 0
	source := candleSource(func(ctx context.Context, k domain.SeriesKey, from, to time.Time) ([]domain.Candle, error) {
		cs, err := syntheticSource(ctx, k, from, to)
		cs = cs[:3]
		for i := range cs {
			px := int64(100)
			if i == 2 {
				px = 110
				if k.Exchange == "other" {
					px = 120
				}
			}
			cs[i].Open, cs[i].High, cs[i].Low, cs[i].Close = decimal.NewFromInt(px), decimal.NewFromInt(px), decimal.NewFromInt(px), decimal.NewFromInt(px)
		}
		return cs, err
	})
	r, err := Run(context.Background(), cfg, source)
	require.NoError(t, err)
	require.Len(t, r.OpenPositions, 2)
	assert.Equal(t, 2000.0, r.Metrics.InitialEquity)
	assert.Equal(t, 2030.0, r.Metrics.FinalEquity, "one unit on each account gained 10 and 20")
	assert.Len(t, r.Equity, 3, "one point per completed timestamp")
}

func TestTimerStrategyAndEightHourlyFunding(t *testing.T) {
	cfg := syntheticConfig()
	cfg.Strategy.Name = "dca"
	cfg.Strategy.Params = map[string]any{"timeframe": "1h", "quote_amount": 100, "max_buys": 1, "interval": "24h"}
	cfg.Fees, cfg.SlippageBps = nil, 0
	cfg.FundingRate = decimal.RequireFromString("0.01")
	src := candleSource(func(ctx context.Context, k domain.SeriesKey, from, to time.Time) ([]domain.Candle, error) {
		cs, err := syntheticSource(ctx, k, from, to)
		cs = cs[:16]
		for i := range cs {
			cs[i].Open, cs[i].High, cs[i].Low, cs[i].Close = decimal.NewFromInt(100), decimal.NewFromInt(100), decimal.NewFromInt(100), decimal.NewFromInt(100)
		}
		return cs, err
	})
	r, err := Run(context.Background(), cfg, src)
	require.NoError(t, err)
	require.Len(t, r.OpenPositions, 1)
	assert.Equal(t, "2", r.OpenPositions[0].Funding.String())
	assert.Equal(t, 998.0, r.Metrics.FinalEquity)
}
