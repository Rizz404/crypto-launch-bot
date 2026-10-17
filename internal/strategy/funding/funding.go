// Package funding is cash-and-carry funding arbitrage: when a perpetual's
// funding rate is high, buy the spot asset and short the perpetual in the
// same size (market neutral) and collect funding; close both when the rate
// falls. Needs separate spot and futures wallets (Binance), and a market
// data source that reports funding rates.
//
// Config: market futures, symbols [the perpetual], legs [{market: spot, symbol: ...}].
package funding

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/rizz404/trading-bot/internal/strategy/hedge"
)

func init() { strategy.Register("funding", func() strategy.Strategy { return &Strategy{} }) }

type Params struct {
	EntryRate   float64 `mapstructure:"entry_rate"` // per funding interval, e.g. 0.0003
	ExitRate    float64 `mapstructure:"exit_rate"`
	QuoteAmount float64 `mapstructure:"quote_amount"` // per leg
}

type Strategy struct {
	ctx  strategy.Context
	p    Params
	pair hedge.Pair
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	s.ctx = ctx
	s.p = Params{EntryRate: 0.0003, ExitRate: 0.00005}
	if err := strategy.DecodeParams(cfg.Params, &s.p); err != nil {
		return err
	}
	if cfg.Market != domain.Futures || len(cfg.Symbols) != 1 || len(cfg.Legs) != 1 || cfg.Legs[0].Market != domain.Spot {
		return fmt.Errorf("funding: needs market futures, one perpetual symbol and one spot leg")
	}
	if cfg.Legs[0].Exchange != cfg.Exchange {
		return fmt.Errorf("funding: the spot leg must be on the same exchange")
	}
	if s.p.QuoteAmount <= 0 || s.p.ExitRate >= s.p.EntryRate {
		return fmt.Errorf("funding: need quote_amount > 0 and exit_rate < entry_rate")
	}
	s.pair = hedge.Pair{
		A:     cfg.Legs[0],                                                                          // spot: bought
		B:     strategy.Leg{Exchange: cfg.Exchange, Market: domain.Futures, Symbol: cfg.Symbols[0]}, // perp: shorted
		Grace: 2 * time.Minute,
	}
	return nil
}

func (s *Strategy) Timeframes() []domain.Timeframe           { return []domain.Timeframe{"1m"} }
func (s *Strategy) OnCandle(domain.Candle) []strategy.Intent { return nil }
func (s *Strategy) Interval() time.Duration                  { return time.Minute }

func (s *Strategy) OnTimer(now time.Time) []strategy.Intent {
	if fix := s.pair.Repair(s.ctx, now); fix != nil {
		return fix
	}
	if s.pair.Busy(now) {
		return nil
	}
	f, known := s.ctx.Funding(s.pair.B.Exchange, s.pair.B.Symbol)
	switch s.pair.State(s.ctx) {
	case hedge.Flat:
		if !known || f.Rate.LessThan(decimal.NewFromFloat(s.p.EntryRate)) {
			return nil
		}
		price, ok := s.ctx.LastPrice(s.pair.B.Exchange, s.pair.B.Market, s.pair.B.Symbol)
		qty, ok2 := s.pair.Quantity(s.ctx, s.p.QuoteAmount, price)
		if !ok || !ok2 {
			return nil
		}
		return s.pair.Open(now, true, qty, fmt.Sprintf("funding %s ≥ entry %g", f.Rate, s.p.EntryRate))
	case hedge.Hedged:
		if known && f.Rate.GreaterThan(decimal.NewFromFloat(s.p.ExitRate)) {
			return nil
		}
		return s.pair.Close(now, "funding fell below exit rate")
	}
	return nil
}
