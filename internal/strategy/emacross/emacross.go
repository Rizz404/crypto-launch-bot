// Package emacross is a trend-following strategy: enter when a fast EMA
// crosses a slow EMA, filtered by RSI, with ATR-based stop loss and take
// profit. Shorts are only taken on futures.
package emacross

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/indicator"
	"github.com/rizz404/trading-bot/internal/strategy"
)

func init() { strategy.Register("emacross", func() strategy.Strategy { return &Strategy{} }) }

type Params struct {
	Timeframe   string  `mapstructure:"timeframe"`
	Fast        int     `mapstructure:"fast"`
	Slow        int     `mapstructure:"slow"`
	RSIPeriod   int     `mapstructure:"rsi_period"`
	RSILongMax  float64 `mapstructure:"rsi_long_max"`  // skip longs when RSI is above (overbought)
	RSIShortMin float64 `mapstructure:"rsi_short_min"` // skip shorts when RSI is below (oversold)
	ATRPeriod   int     `mapstructure:"atr_period"`
	StopATR     float64 `mapstructure:"stop_atr"`      // stop distance in ATRs
	TargetATR   float64 `mapstructure:"target_atr"`    // take-profit distance in ATRs, 0 = none
	TrailATR    float64 `mapstructure:"trail_atr"`     // trailing distance in ATRs, 0 = none
	AllowShort  bool    `mapstructure:"allow_short"`   // ignored on spot
	ExitOnCross bool    `mapstructure:"exit_on_cross"` // close on the opposite cross even without a new entry
}

func defaultParams() Params {
	return Params{
		Timeframe: "1h", Fast: 12, Slow: 26,
		RSIPeriod: 14, RSILongMax: 70, RSIShortMin: 30,
		ATRPeriod: 14, StopATR: 2, TargetATR: 3,
		AllowShort: true, ExitOnCross: true,
	}
}

type series struct {
	fast, slow *indicator.EMA
	rsi        *indicator.RSI
	atr        *indicator.ATR
	cross      indicator.Cross
}

type Strategy struct {
	ctx    strategy.Context
	cfg    strategy.Config
	p      Params
	tf     domain.Timeframe
	series map[string]*series
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	s.ctx, s.cfg, s.p = ctx, cfg, defaultParams()
	if err := strategy.DecodeParams(cfg.Params, &s.p); err != nil {
		return err
	}
	tf, err := domain.ParseTimeframe(s.p.Timeframe)
	if err != nil {
		return err
	}
	if s.p.Fast < 1 || s.p.Slow <= s.p.Fast {
		return fmt.Errorf("emacross: need 0 < fast < slow, got %d/%d", s.p.Fast, s.p.Slow)
	}
	if s.p.StopATR <= 0 {
		return fmt.Errorf("emacross: stop_atr must be > 0")
	}
	s.tf = tf
	s.series = map[string]*series{}
	for _, sym := range cfg.Symbols {
		s.series[sym] = &series{
			fast: indicator.NewEMA(s.p.Fast),
			slow: indicator.NewEMA(s.p.Slow),
			rsi:  indicator.NewRSI(s.p.RSIPeriod),
			atr:  indicator.NewATR(s.p.ATRPeriod),
		}
	}
	return nil
}

func (s *Strategy) Timeframes() []domain.Timeframe { return []domain.Timeframe{s.tf} }

func (s *Strategy) OnCandle(c domain.Candle) []strategy.Intent {
	st := s.series[c.Symbol]
	if st == nil || c.Timeframe != s.tf {
		return nil
	}
	closeF, _ := c.Close.Float64()
	highF, _ := c.High.Float64()
	lowF, _ := c.Low.Float64()
	fast, slow := st.fast.Update(closeF), st.slow.Update(closeF)
	rsi := st.rsi.Update(closeF)
	atr := st.atr.Update(highF, lowF, closeF)
	if !st.slow.Ready() || !st.rsi.Ready() || !st.atr.Ready() {
		st.cross.Update(fast, slow) // keep the cross state warm
		return nil
	}
	cross := st.cross.Update(fast, slow)
	if cross == 0 {
		return nil
	}

	pos, hasPos := s.ctx.Position(c.Exchange, c.Market, c.Symbol)
	canShort := s.p.AllowShort && c.Market == domain.Futures
	var out []strategy.Intent

	switch {
	case cross > 0 && rsi <= s.p.RSILongMax:
		if !hasPos || pos.Side != domain.Long {
			out = append(out, s.enter(c, domain.Long, atr, fmt.Sprintf("ema cross up, rsi %.1f", rsi)))
		}
	case cross < 0 && canShort && rsi >= s.p.RSIShortMin:
		if !hasPos || pos.Side != domain.Short {
			out = append(out, s.enter(c, domain.Short, atr, fmt.Sprintf("ema cross down, rsi %.1f", rsi)))
		}
	case s.p.ExitOnCross && hasPos:
		// Opposite cross without a qualifying entry: just leave.
		if (cross < 0 && pos.Side == domain.Long) || (cross > 0 && pos.Side == domain.Short) {
			out = append(out, strategy.Intent{Kind: strategy.Exit, Market: c.Market, Symbol: c.Symbol, Reason: "opposite ema cross"})
		}
	}
	return out
}

func (s *Strategy) enter(c domain.Candle, side domain.PositionSide, atr float64, reason string) strategy.Intent {
	atrD := decimal.NewFromFloat(atr)
	sign := side.Sign()
	in := strategy.Intent{
		Kind:     strategy.Enter,
		Market:   c.Market,
		Symbol:   c.Symbol,
		Side:     side,
		StopLoss: c.Close.Sub(atrD.Mul(decimal.NewFromFloat(s.p.StopATR)).Mul(sign)),
		Reason:   reason,
	}
	if s.p.TargetATR > 0 {
		in.TakeProfit = c.Close.Add(atrD.Mul(decimal.NewFromFloat(s.p.TargetATR)).Mul(sign))
	}
	if s.p.TrailATR > 0 {
		closeF, _ := c.Close.Float64()
		in.TrailingStop = s.p.TrailATR * atr / closeF
	}
	return in
}
