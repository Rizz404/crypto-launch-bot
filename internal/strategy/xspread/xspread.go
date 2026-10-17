// Package xspread trades the price difference of the same perpetual on two
// exchanges: when one trades rich, short it and buy the other; close both
// when the spread converges. Each exchange needs its own margin.
//
// Config: exchange A, market futures, symbols [A's symbol],
// legs [{exchange: B, market: futures, symbol: B's symbol}].
package xspread

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/rizz404/trading-bot/internal/strategy/hedge"
)

func init() { strategy.Register("xspread", func() strategy.Strategy { return &Strategy{} }) }

type Params struct {
	EntryBps    float64 `mapstructure:"entry_bps"` // must exceed both sides' fees and slippage
	ExitBps     float64 `mapstructure:"exit_bps"`
	QuoteAmount float64 `mapstructure:"quote_amount"` // per leg
}

type Strategy struct {
	ctx  strategy.Context
	p    Params
	pair hedge.Pair
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	s.ctx = ctx
	s.p = Params{EntryBps: 30, ExitBps: 5}
	if err := strategy.DecodeParams(cfg.Params, &s.p); err != nil {
		return err
	}
	if len(cfg.Symbols) != 1 || len(cfg.Legs) != 1 || cfg.Legs[0].Exchange == cfg.Exchange {
		return fmt.Errorf("xspread: needs one symbol and one leg on another exchange")
	}
	if cfg.Market != domain.Futures || cfg.Legs[0].Market != domain.Futures {
		return fmt.Errorf("xspread: both legs must be perpetual futures")
	}
	if s.p.QuoteAmount <= 0 || s.p.ExitBps >= s.p.EntryBps {
		return fmt.Errorf("xspread: need quote_amount > 0 and exit_bps < entry_bps")
	}
	s.pair = hedge.Pair{
		A:     strategy.Leg{Exchange: cfg.Exchange, Market: domain.Futures, Symbol: cfg.Symbols[0]},
		B:     cfg.Legs[0],
		Grace: time.Minute,
	}
	return nil
}

func (s *Strategy) Timeframes() []domain.Timeframe           { return []domain.Timeframe{"1m"} }
func (s *Strategy) OnCandle(domain.Candle) []strategy.Intent { return s.evaluate() }
func (s *Strategy) OnPrice(domain.Candle) []strategy.Intent  { return s.evaluate() }

// spreadBps is (B - A) / A in basis points.
func (s *Strategy) spreadBps() (float64, decimal.Decimal, bool) {
	a, okA := s.ctx.LastPrice(s.pair.A.Exchange, s.pair.A.Market, s.pair.A.Symbol)
	b, okB := s.ctx.LastPrice(s.pair.B.Exchange, s.pair.B.Market, s.pair.B.Symbol)
	if !okA || !okB || !a.IsPositive() {
		return 0, decimal.Zero, false
	}
	bps, _ := b.Sub(a).Div(a).Mul(decimal.NewFromInt(10_000)).Float64()
	return bps, a, true
}

func (s *Strategy) evaluate() []strategy.Intent {
	now := s.ctx.Now()
	if fix := s.pair.Repair(s.ctx, now); fix != nil {
		return fix
	}
	if s.pair.Busy(now) {
		return nil
	}
	spread, priceA, ok := s.spreadBps()
	if !ok {
		return nil
	}
	switch s.pair.State(s.ctx) {
	case hedge.Flat:
		if spread < s.p.EntryBps && spread > -s.p.EntryBps {
			return nil
		}
		qty, ok := s.pair.Quantity(s.ctx, s.p.QuoteAmount, priceA)
		if !ok {
			return nil
		}
		// B rich (positive spread): buy A, short B; otherwise the reverse.
		return s.pair.Open(now, spread > 0, qty, fmt.Sprintf("spread %.1f bps", spread))
	case hedge.Hedged:
		if spread > s.p.ExitBps || spread < -s.p.ExitBps {
			return nil
		}
		return s.pair.Close(now, fmt.Sprintf("spread converged to %.1f bps", spread))
	}
	return nil
}
