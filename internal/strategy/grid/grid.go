// Package grid is a long grid: buy limits on price levels below the market;
// each filled buy places a sell one level up, each filled sell places a buy
// one level down. It profits from oscillation inside [lower, upper] and
// holds inventory when the price falls.
package grid

import (
	"fmt"
	"math"
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
)

func init() { strategy.Register("grid", func() strategy.Strategy { return &Strategy{} }) }

type Params struct {
	Timeframe     string  `mapstructure:"timeframe"` // candle used to start the grid and check the stop
	Lower         float64 `mapstructure:"lower"`
	Upper         float64 `mapstructure:"upper"`
	Levels        int     `mapstructure:"levels"`          // number of intervals
	Geometric     bool    `mapstructure:"geometric"`       // equal ratios instead of equal distances
	QuotePerLevel float64 `mapstructure:"quote_per_level"` // order size in quote currency
	StopBelow     float64 `mapstructure:"stop_below"`      // close everything below this price (0 = never)
}

type Strategy struct {
	ctx     strategy.Context
	cfg     strategy.Config
	p       Params
	tf      domain.Timeframe
	symbol  string
	levels  []decimal.Decimal
	started bool
	stopped bool
	// baseFees sums the base-coin commission per order until it fills.
	baseFees map[string]decimal.Decimal
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	s.ctx, s.cfg = ctx, cfg
	s.p = Params{Timeframe: "1m", Levels: 10}
	if err := strategy.DecodeParams(cfg.Params, &s.p); err != nil {
		return err
	}
	if len(cfg.Symbols) != 1 {
		return fmt.Errorf("grid: exactly one symbol per instance, got %v", cfg.Symbols)
	}
	if s.p.Lower <= 0 || s.p.Upper <= s.p.Lower || s.p.Levels < 2 || s.p.QuotePerLevel <= 0 {
		return fmt.Errorf("grid: need 0 < lower < upper, levels >= 2, quote_per_level > 0")
	}
	tf, err := domain.ParseTimeframe(s.p.Timeframe)
	if err != nil {
		return err
	}
	s.tf, s.symbol, s.baseFees = tf, cfg.Symbols[0], map[string]decimal.Decimal{}
	for i := 0; i <= s.p.Levels; i++ {
		var px float64
		if s.p.Geometric {
			px = s.p.Lower * math.Pow(s.p.Upper/s.p.Lower, float64(i)/float64(s.p.Levels))
		} else {
			px = s.p.Lower + (s.p.Upper-s.p.Lower)*float64(i)/float64(s.p.Levels)
		}
		s.levels = append(s.levels, decimal.NewFromFloat(px))
	}
	return nil
}

func (s *Strategy) Timeframes() []domain.Timeframe { return []domain.Timeframe{s.tf} }

func (s *Strategy) OnCandle(c domain.Candle) []strategy.Intent {
	if c.Symbol != s.symbol || c.Timeframe != s.tf || s.stopped {
		return nil
	}
	// Warmup and gap backfill replay old candles. The engine drops intents
	// they produce, so the grid must not start (and consider itself
	// started) on one: it would never place its orders.
	if s.ctx.Now().Sub(c.CloseTime()) > s.tf.Duration() {
		return nil
	}
	if s.p.StopBelow > 0 && c.Close.LessThan(decimal.NewFromFloat(s.p.StopBelow)) {
		s.stopped = true
		s.ctx.Log().Warn("grid stop hit: cancelling orders and closing inventory", "price", c.Close)
		return []strategy.Intent{
			{Kind: strategy.CancelAll, Market: c.Market, Symbol: s.symbol, Reason: "grid stop"},
			{Kind: strategy.Exit, Market: c.Market, Symbol: s.symbol, Reason: "grid stop"},
		}
	}
	if s.started {
		return nil
	}
	s.started = true
	if len(s.ctx.OpenOrders(s.symbol)) > 0 {
		return nil // orders survive from a previous session
	}
	var out []strategy.Intent
	for i, lvl := range s.levels {
		if lvl.LessThan(c.Close) {
			out = append(out, s.order(c.Market, domain.Buy, i, s.qty(lvl)))
		}
	}
	return out
}

// OnFill places the opposite order one level away once an order is filled.
func (s *Strategy) OnFill(f domain.Fill, o domain.Order) []strategy.Intent {
	if o.Symbol != s.symbol || s.stopped || o.Tag != domain.TagStrategy {
		return nil
	}
	if f.BaseFee.IsPositive() {
		s.baseFees[o.ClientID] = s.baseFees[o.ClientID].Add(f.BaseFee)
	}
	if o.Status != domain.OrderFilled {
		return nil
	}
	// Spot buys may pay the commission in the base coin: sell what arrived.
	received := o.FilledQty.Sub(s.baseFees[o.ClientID])
	delete(s.baseFees, o.ClientID)
	i := s.levelIndex(o.Price)
	switch {
	case o.Side == domain.Buy && i+1 < len(s.levels):
		return []strategy.Intent{s.order(o.Market, domain.Sell, i+1, received)}
	case o.Side == domain.Sell && i-1 >= 0:
		return []strategy.Intent{s.order(o.Market, domain.Buy, i-1, s.qty(s.levels[i-1]))}
	}
	return nil
}

func (s *Strategy) qty(price decimal.Decimal) decimal.Decimal {
	return decimal.NewFromFloat(s.p.QuotePerLevel).Div(price)
}

func (s *Strategy) order(m domain.Market, side domain.Side, level int, qty decimal.Decimal) strategy.Intent {
	ref := "b"
	if side == domain.Sell {
		ref = "s"
	}
	return strategy.Intent{
		Kind: strategy.Place, Market: m, Symbol: s.symbol, Ref: ref + strconv.Itoa(level),
		Order: strategy.OrderSpec{
			Side: side, Type: domain.OrderLimit, Quantity: qty, Price: s.levels[level],
			// On futures the sells only take profit on grid inventory.
			ReduceOnly: side == domain.Sell && m == domain.Futures,
		},
		Reason: fmt.Sprintf("grid level %d", level),
	}
}

// levelIndex finds the level closest to price (order prices are rounded
// to the tick size).
func (s *Strategy) levelIndex(price decimal.Decimal) int {
	best, bestDist := 0, decimal.NewFromInt(-1)
	for i, lvl := range s.levels {
		d := lvl.Sub(price).Abs()
		if bestDist.IsNegative() || d.LessThan(bestDist) {
			best, bestDist = i, d
		}
	}
	return best
}
