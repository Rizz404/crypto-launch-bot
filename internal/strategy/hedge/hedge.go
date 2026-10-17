// Package hedge helps strategies that hold two offsetting legs (funding
// arbitrage, cross-exchange spreads): opening both, closing both, and
// unwinding a leg left alone when the other did not fill.
package hedge

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
)

// Pair is two instruments traded together.
type Pair struct {
	A, B strategy.Leg
	// Grace is how long a one-legged state may last (orders in flight)
	// before the open leg is unwound.
	Grace time.Duration

	busyUntil time.Time
	lopsided  time.Time
}

// State is what the pair holds.
type State int

const (
	Flat     State = iota
	Hedged         // both legs open
	Lopsided       // one leg open
)

func (p *Pair) legs(ctx strategy.Context) (a, b domain.Position, hasA, hasB bool) {
	a, hasA = ctx.Position(p.A.Exchange, p.A.Market, p.A.Symbol)
	b, hasB = ctx.Position(p.B.Exchange, p.B.Market, p.B.Symbol)
	return a, b, hasA, hasB
}

// State reports the current holdings.
func (p *Pair) State(ctx strategy.Context) State {
	_, _, hasA, hasB := p.legs(ctx)
	switch {
	case hasA && hasB:
		return Hedged
	case hasA || hasB:
		return Lopsided
	}
	return Flat
}

// Busy reports whether orders were sent recently and may still be in
// flight; strategies should not act until it returns false.
func (p *Pair) Busy(now time.Time) bool { return now.Before(p.busyUntil) }

// Open buys qty of the buy leg and sells qty of the other at market.
func (p *Pair) Open(now time.Time, buyA bool, qty decimal.Decimal, reason string) []strategy.Intent {
	p.busyUntil = now.Add(p.Grace)
	sideA, sideB := domain.Buy, domain.Sell
	if !buyA {
		sideA, sideB = domain.Sell, domain.Buy
	}
	return []strategy.Intent{order(p.A, sideA, qty, reason), order(p.B, sideB, qty, reason)}
}

// Close exits both legs.
func (p *Pair) Close(now time.Time, reason string) []strategy.Intent {
	p.busyUntil = now.Add(p.Grace)
	return []strategy.Intent{
		{Kind: strategy.Exit, Exchange: p.A.Exchange, Market: p.A.Market, Symbol: p.A.Symbol, Reason: reason},
		{Kind: strategy.Exit, Exchange: p.B.Exchange, Market: p.B.Market, Symbol: p.B.Symbol, Reason: reason},
	}
}

// Repair unwinds a leg that stayed alone longer than Grace.
func (p *Pair) Repair(ctx strategy.Context, now time.Time) []strategy.Intent {
	if p.State(ctx) != Lopsided || p.Busy(now) {
		p.lopsided = time.Time{}
		return nil
	}
	if p.lopsided.IsZero() {
		p.lopsided = now
		return nil
	}
	if now.Sub(p.lopsided) < p.Grace {
		return nil
	}
	p.lopsided = time.Time{}
	ctx.Log().Warn("hedge leg left alone, unwinding it", "a", p.A.Symbol, "b", p.B.Symbol)
	return p.Close(now, "unwind lopsided hedge")
}

// Quantity sizes both legs for a quote amount at price, rounded to the
// coarser lot step of the two instruments.
func (p *Pair) Quantity(ctx strategy.Context, quote float64, price decimal.Decimal) (decimal.Decimal, bool) {
	ia, okA := ctx.Instrument(p.A.Exchange, p.A.Market, p.A.Symbol)
	ib, okB := ctx.Instrument(p.B.Exchange, p.B.Market, p.B.Symbol)
	if !okA || !okB || !price.IsPositive() {
		return decimal.Zero, false
	}
	qty := decimal.NewFromFloat(quote).Div(price)
	qty = ia.RoundQty(ib.RoundQty(qty))
	qty = ib.RoundQty(qty)
	if ia.CheckOrder(price, qty) != nil || ib.CheckOrder(price, qty) != nil {
		return decimal.Zero, false
	}
	return qty, true
}

func order(l strategy.Leg, side domain.Side, qty decimal.Decimal, reason string) strategy.Intent {
	return strategy.Intent{
		Kind: strategy.Place, Exchange: l.Exchange, Market: l.Market, Symbol: l.Symbol,
		Order:  strategy.OrderSpec{Side: side, Type: domain.OrderMarket, Quantity: qty},
		Reason: reason,
	}
}
