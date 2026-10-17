// Package dca buys a fixed quote amount on a schedule (optionally only
// after the price dropped since the last buy) and sells the whole position
// once it is up take_profit from its average entry.
package dca

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
)

func init() { strategy.Register("dca", func() strategy.Strategy { return &Strategy{} }) }

type Params struct {
	Timeframe   string        `mapstructure:"timeframe"` // candle used for take-profit / stop checks
	Interval    time.Duration `mapstructure:"interval"`  // time between buys
	QuoteAmount float64       `mapstructure:"quote_amount"`
	MaxBuys     int           `mapstructure:"max_buys"`
	DipPct      float64       `mapstructure:"dip_pct"`     // after the first buy, only buy this far below the last buy
	TakeProfit  float64       `mapstructure:"take_profit"` // fraction above the average entry
	StopLoss    float64       `mapstructure:"stop_loss"`   // fraction below the average entry (0 = none)
}

type series struct {
	buys      int
	lastBuy   time.Time
	lastPrice decimal.Decimal
}

type Strategy struct {
	ctx    strategy.Context
	cfg    strategy.Config
	p      Params
	tf     domain.Timeframe
	series map[string]*series
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	s.ctx, s.cfg = ctx, cfg
	s.p = Params{Timeframe: "1m", Interval: 24 * time.Hour, MaxBuys: 10, TakeProfit: 0.03}
	if err := strategy.DecodeParams(cfg.Params, &s.p); err != nil {
		return err
	}
	if s.p.QuoteAmount <= 0 || s.p.MaxBuys < 1 || s.p.TakeProfit <= 0 || s.p.Interval <= 0 {
		return fmt.Errorf("dca: need quote_amount > 0, max_buys >= 1, take_profit > 0, interval > 0")
	}
	tf, err := domain.ParseTimeframe(s.p.Timeframe)
	if err != nil {
		return err
	}
	s.tf, s.series = tf, map[string]*series{}
	for _, sym := range cfg.Symbols {
		s.series[sym] = &series{}
	}
	return nil
}

func (s *Strategy) Timeframes() []domain.Timeframe { return []domain.Timeframe{s.tf} }
func (s *Strategy) Interval() time.Duration        { return time.Minute }

// OnCandle exits at the take-profit or stop level.
func (s *Strategy) OnCandle(c domain.Candle) []strategy.Intent {
	st := s.series[c.Symbol]
	if st == nil || c.Timeframe != s.tf {
		return nil
	}
	pos, has := s.ctx.Position(c.Exchange, c.Market, c.Symbol)
	if !has {
		return nil
	}
	tp := pos.EntryPrice.Mul(decimal.NewFromFloat(1 + s.p.TakeProfit))
	hitTP := c.Close.GreaterThanOrEqual(tp)
	hitSL := s.p.StopLoss > 0 && c.Close.LessThanOrEqual(pos.EntryPrice.Mul(decimal.NewFromFloat(1-s.p.StopLoss)))
	if !hitTP && !hitSL {
		return nil
	}
	reason := "dca take profit"
	if hitSL {
		reason = "dca stop loss"
	}
	*st = series{} // start a new cycle
	return []strategy.Intent{{Kind: strategy.Exit, Market: c.Market, Symbol: c.Symbol, Reason: reason}}
}

// OnTimer places the scheduled buys.
func (s *Strategy) OnTimer(now time.Time) []strategy.Intent {
	var out []strategy.Intent
	for sym, st := range s.series {
		if st.buys >= s.p.MaxBuys || (!st.lastBuy.IsZero() && now.Sub(st.lastBuy) < s.p.Interval) {
			continue
		}
		price, ok := s.ctx.LastPrice(s.cfg.Exchange, s.cfg.Market, sym)
		if !ok || !price.IsPositive() {
			continue
		}
		if st.buys > 0 && s.p.DipPct > 0 && price.GreaterThan(st.lastPrice.Mul(decimal.NewFromFloat(1-s.p.DipPct))) {
			continue
		}
		st.buys++
		st.lastBuy, st.lastPrice = now, price
		out = append(out, strategy.Intent{
			Kind: strategy.Place, Market: s.cfg.Market, Symbol: sym,
			Order:  strategy.OrderSpec{Side: domain.Buy, Type: domain.OrderMarket, Quantity: decimal.NewFromFloat(s.p.QuoteAmount).Div(price)},
			Reason: fmt.Sprintf("dca buy %d/%d", st.buys, s.p.MaxBuys),
		})
	}
	return out
}
