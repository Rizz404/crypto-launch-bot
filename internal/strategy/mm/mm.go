// Package mm is a basic market maker: it quotes a post-only bid and ask
// around the last price, skews both quotes against its inventory and
// requotes when the price moves or an order fills. It sees trade prices,
// not the order book, so it suits liquid symbols; expect small edges and
// keep max_inventory_quote tight.
package mm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
)

func init() { strategy.Register("mm", func() strategy.Strategy { return &Strategy{} }) }

type Params struct {
	SpreadBps         float64       `mapstructure:"spread_bps"`  // bid-ask distance
	OrderQuote        float64       `mapstructure:"order_quote"` // size per quote, quote currency
	MaxInventoryQuote float64       `mapstructure:"max_inventory_quote"`
	SkewBps           float64       `mapstructure:"skew_bps"`    // quote shift at full inventory
	RequoteBps        float64       `mapstructure:"requote_bps"` // price move that triggers a requote
	MinRequote        time.Duration `mapstructure:"min_requote"` // minimum time between requotes
}

type Strategy struct {
	ctx    strategy.Context
	cfg    strategy.Config
	p      Params
	symbol string
	quoted decimal.Decimal // mid of the live quotes
	lastQ  time.Time
	gen    int
	dirty  bool
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	s.ctx, s.cfg = ctx, cfg
	s.p = Params{SpreadBps: 20, RequoteBps: 5, MinRequote: 5 * time.Second}
	if err := strategy.DecodeParams(cfg.Params, &s.p); err != nil {
		return err
	}
	if len(cfg.Symbols) != 1 {
		return fmt.Errorf("mm: exactly one symbol per instance")
	}
	if s.p.SpreadBps <= 0 || s.p.OrderQuote <= 0 || s.p.MaxInventoryQuote < s.p.OrderQuote {
		return fmt.Errorf("mm: need spread_bps > 0, order_quote > 0, max_inventory_quote >= order_quote")
	}
	s.symbol = cfg.Symbols[0]
	return nil
}

func (s *Strategy) Timeframes() []domain.Timeframe { return []domain.Timeframe{"1m"} }

func (s *Strategy) OnCandle(c domain.Candle) []strategy.Intent {
	if c.Symbol != s.symbol {
		return nil
	}
	return s.quote(c.Close) // keeps quoting when price updates are sparse
}

func (s *Strategy) OnPrice(c domain.Candle) []strategy.Intent {
	if c.Symbol != s.symbol {
		return nil
	}
	return s.quote(c.Close)
}

func (s *Strategy) OnFill(_ domain.Fill, o domain.Order) []strategy.Intent {
	if o.Symbol == s.symbol {
		s.dirty = true // inventory changed: requote on the next price
	}
	return nil
}

func bps(v float64) decimal.Decimal { return decimal.NewFromFloat(v / 10_000) }

func (s *Strategy) quote(price decimal.Decimal) []strategy.Intent {
	now := s.ctx.Now()
	if !price.IsPositive() || now.Sub(s.lastQ) < s.p.MinRequote {
		return nil
	}
	open := s.ctx.OpenOrders(s.symbol)
	moved := s.quoted.IsZero() || price.Sub(s.quoted).Abs().Div(s.quoted).GreaterThan(bps(s.p.RequoteBps))
	if !moved && !s.dirty && len(open) > 0 {
		return nil
	}
	s.lastQ, s.quoted, s.dirty = now, price, false
	s.gen++

	// Inventory in quote currency, signed.
	inv := decimal.Zero
	if pos, ok := s.ctx.Position(s.cfg.Exchange, s.cfg.Market, s.symbol); ok {
		inv = pos.Notional(price).Mul(pos.Side.Sign())
	}
	maxInv := decimal.NewFromFloat(s.p.MaxInventoryQuote)
	ratio, _ := inv.Div(maxInv).Float64()
	half := s.p.SpreadBps / 2
	skew := s.p.SkewBps * ratio // long inventory lowers both quotes
	bid := price.Mul(decimal.NewFromInt(1).Sub(bps(half + skew)))
	ask := price.Mul(decimal.NewFromInt(1).Add(bps(half - skew)))
	size := decimal.NewFromFloat(s.p.OrderQuote)

	out := []strategy.Intent{{Kind: strategy.CancelAll, Market: s.cfg.Market, Symbol: s.symbol, Reason: "requote"}}
	gen := strconv.Itoa(s.gen)
	if inv.Add(size).LessThanOrEqual(maxInv) {
		out = append(out, s.place(domain.Buy, bid, size.Div(bid), "b"+gen))
	}
	canSell := inv.Sub(size).GreaterThanOrEqual(maxInv.Neg())
	if s.cfg.Market == domain.Spot {
		canSell = inv.GreaterThanOrEqual(size) // spot cannot sell what it does not hold
	}
	if canSell {
		out = append(out, s.place(domain.Sell, ask, size.Div(ask), "a"+gen))
	}
	return out
}

func (s *Strategy) place(side domain.Side, price, qty decimal.Decimal, ref string) strategy.Intent {
	return strategy.Intent{
		Kind: strategy.Place, Market: s.cfg.Market, Symbol: s.symbol, Ref: ref,
		Order:  strategy.OrderSpec{Side: side, Type: domain.OrderLimit, Price: price, Quantity: qty, PostOnly: true},
		Reason: "mm quote",
	}
}

type savedState struct {
	Quoted     decimal.Decimal
	LastQ      time.Time
	Generation int
	Dirty      bool
}

func (s *Strategy) SaveState() ([]byte, error) {
	return json.Marshal(savedState{s.quoted, s.lastQ, s.gen, s.dirty})
}
func (s *Strategy) LoadState(raw []byte) error {
	var v savedState
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	s.quoted, s.lastQ, s.gen, s.dirty = v.Quoted, v.LastQ, v.Generation, v.Dirty
	return nil
}
