// Package trendspot implements selective, long-only spot research strategies.
// It supports channel breakout, absolute momentum, and trend pullback entries.
// History only includes closed candles; channel thresholds exclude the current bar.
package trendspot

import (
	"fmt"
	"math"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/indicator"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

func init() { strategy.Register("trendspot", func() strategy.Strategy { return &Strategy{} }) }

type Params struct {
	Mode        string  `mapstructure:"mode"`
	Timeframe   string  `mapstructure:"timeframe"`
	TrendPeriod int     `mapstructure:"trend_period"`
	EntryPeriod int     `mapstructure:"entry_period"`
	ExitPeriod  int     `mapstructure:"exit_period"`
	ATRPeriod   int     `mapstructure:"atr_period"`
	RSIPeriod   int     `mapstructure:"rsi_period"`
	StopATR     float64 `mapstructure:"stop_atr"`
	TargetATR   float64 `mapstructure:"target_atr"`
	TrailATR    float64 `mapstructure:"trail_atr"`
	MomentumMin float64 `mapstructure:"momentum_min"`
	RSIEntry    float64 `mapstructure:"rsi_entry"`
	RSIExit     float64 `mapstructure:"rsi_exit"`
}
type series struct {
	trend     *indicator.EMA
	atr       *indicator.ATR
	rsi       *indicator.RSI
	history   []domain.Candle
	prevRSI   float64
	prevTrend float64
	seen      bool
}
type Strategy struct {
	ctx    strategy.Context
	cfg    strategy.Config
	p      Params
	tf     domain.Timeframe
	series map[string]*series
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	s.p = Params{Mode: "breakout", Timeframe: "1d", TrendPeriod: 100, EntryPeriod: 55, ExitPeriod: 20, ATRPeriod: 14, RSIPeriod: 14, StopATR: 3, TrailATR: 3, MomentumMin: 0.05, RSIEntry: 40, RSIExit: 65}
	if err := strategy.DecodeParams(cfg.Params, &s.p); err != nil {
		return err
	}
	if cfg.Market != domain.Spot || len(cfg.Symbols) == 0 || len(cfg.Legs) != 0 || cfg.Leverage > 1 {
		return fmt.Errorf("trendspot: requires unleveraged spot symbols and no extra legs")
	}
	switch s.p.Mode {
	case "breakout", "momentum", "pullback":
	default:
		return fmt.Errorf("trendspot: unknown mode %q", s.p.Mode)
	}
	if s.p.TrendPeriod < 2 || s.p.EntryPeriod < 1 || s.p.ExitPeriod < 1 || s.p.ATRPeriod < 1 || s.p.RSIPeriod < 1 || s.p.StopATR <= 0 || s.p.TargetATR < 0 || s.p.TrailATR < 0 || s.p.MomentumMin < 0 || s.p.RSIEntry <= 0 || s.p.RSIExit >= 100 || s.p.RSIEntry >= s.p.RSIExit {
		return fmt.Errorf("trendspot: invalid periods, risk distances or entry/exit thresholds")
	}
	for _, v := range []float64{s.p.StopATR, s.p.TargetATR, s.p.TrailATR, s.p.MomentumMin, s.p.RSIEntry, s.p.RSIExit} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("trendspot: parameters must be finite")
		}
	}
	tf, err := domain.ParseTimeframe(s.p.Timeframe)
	if err != nil {
		return err
	}
	s.ctx, s.cfg, s.tf = ctx, cfg, tf
	s.series = map[string]*series{}
	for _, sym := range cfg.Symbols {
		s.series[sym] = &series{trend: indicator.NewEMA(s.p.TrendPeriod), atr: indicator.NewATR(s.p.ATRPeriod), rsi: indicator.NewRSI(s.p.RSIPeriod)}
	}
	return nil
}
func (s *Strategy) Timeframes() []domain.Timeframe { return []domain.Timeframe{s.tf} }
func (s *Strategy) OnCandle(c domain.Candle) []strategy.Intent {
	st := s.series[c.Symbol]
	if st == nil || !c.Closed || c.Exchange != s.cfg.Exchange || c.Market != domain.Spot || c.Timeframe != s.tf {
		return nil
	}
	closeF, _ := c.Close.Float64()
	highF, _ := c.High.Float64()
	lowF, _ := c.Low.Float64()
	trend := st.trend.Update(closeF)
	atr := st.atr.Update(highF, lowF, closeF)
	rsi := st.rsi.Update(closeF)
	// Append only after deciding: the current candle cannot move its own breakout boundary.
	defer func() {
		st.prevRSI, st.prevTrend, st.seen = rsi, trend, true
		st.history = append(st.history, c)
		if n := max(s.p.EntryPeriod, s.p.ExitPeriod); len(st.history) > n {
			copy(st.history, st.history[len(st.history)-n:])
			st.history = st.history[:n]
		}
	}()
	if !st.trend.Ready() || !st.atr.Ready() || atr <= 0 {
		return nil
	}
	_, held := s.ctx.Position(c.Exchange, c.Market, c.Symbol)
	enter, exit := false, false
	switch s.p.Mode {
	case "breakout":
		if len(st.history) < max(s.p.EntryPeriod, s.p.ExitPeriod) {
			return nil
		}
		hi := st.history[len(st.history)-s.p.EntryPeriod].High
		for _, b := range st.history[len(st.history)-s.p.EntryPeriod:] {
			hi = decimal.Max(hi, b.High)
		}
		lo := st.history[len(st.history)-s.p.ExitPeriod].Low
		for _, b := range st.history[len(st.history)-s.p.ExitPeriod:] {
			lo = decimal.Min(lo, b.Low)
		}
		enter = c.Close.GreaterThan(hi) && closeF > trend
		exit = c.Close.LessThan(lo) || closeF < trend
	case "momentum":
		if len(st.history) < s.p.EntryPeriod {
			return nil
		}
		past, _ := st.history[len(st.history)-s.p.EntryPeriod].Close.Float64()
		if past <= 0 {
			return nil
		}
		momentum := closeF/past - 1
		enter = momentum > s.p.MomentumMin && closeF > trend
		exit = momentum <= 0 || closeF < trend
	case "pullback":
		if !st.rsi.Ready() || !st.seen {
			return nil
		}
		enter = closeF > trend && trend > st.prevTrend && st.prevRSI < s.p.RSIEntry && rsi >= s.p.RSIEntry && rsi < s.p.RSIExit
		exit = rsi >= s.p.RSIExit || closeF < trend
	}
	if held {
		if exit {
			return []strategy.Intent{{Kind: strategy.Exit, Market: c.Market, Symbol: c.Symbol, Reason: s.p.Mode + " exit"}}
		}
		return nil
	}
	if !enter {
		return nil
	}
	stop := c.Close.Sub(decimal.NewFromFloat(s.p.StopATR * atr))
	if !stop.IsPositive() {
		return nil
	}
	in := strategy.Intent{Kind: strategy.Enter, Market: c.Market, Symbol: c.Symbol, Side: domain.Long, StopLoss: stop, Reason: s.p.Mode + " entry"}
	if s.p.TargetATR > 0 {
		in.TakeProfit = c.Close.Add(decimal.NewFromFloat(s.p.TargetATR * atr))
	}
	if s.p.TrailATR > 0 {
		in.TrailingStop = s.p.TrailATR * atr / closeF
	}
	return []strategy.Intent{in}
}
