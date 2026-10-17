// Package risk sizes positions and enforces trading limits.
package risk

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// ErrTooSmall means the sized order is below the exchange minimums.
var ErrTooSmall = errors.New("order below exchange minimum")

// SizeRequest describes a position to size.
type SizeRequest struct {
	Equity     decimal.Decimal
	FreeMargin decimal.Decimal
	Price      decimal.Decimal // expected entry price
	StopLoss   decimal.Decimal // zero when the strategy gives no stop
	Leverage   int
	Instrument domain.Instrument
	Quantity   decimal.Decimal // strategy override, still capped
}

// Size returns the order quantity for r, rounded to the lot step.
//
// With a stop loss the position risks RiskPerTrade of equity if the stop is
// hit; without one it uses PositionPct of equity as margin. Either way the
// notional is capped at MaxPositionPct × equity × leverage and by free
// margin.
func (m *Manager) Size(r SizeRequest) (decimal.Decimal, error) {
	if !r.Price.IsPositive() {
		return decimal.Zero, errors.New("size: price must be positive")
	}
	if !r.Equity.IsPositive() {
		return decimal.Zero, errors.New("size: no equity")
	}
	lev := decimal.NewFromInt(int64(m.leverage(r.Leverage)))

	qty := r.Quantity
	if !qty.IsPositive() {
		dist := r.Price.Sub(r.StopLoss).Abs()
		if r.StopLoss.IsPositive() && dist.IsPositive() && m.cfg.RiskPerTrade > 0 {
			qty = r.Equity.Mul(decimal.NewFromFloat(m.cfg.RiskPerTrade)).Div(dist)
		} else {
			qty = r.Equity.Mul(decimal.NewFromFloat(m.cfg.PositionPct)).Mul(lev).Div(r.Price)
		}
	}

	maxNotional := r.Equity.Mul(decimal.NewFromFloat(m.cfg.MaxPositionPct)).Mul(lev)
	if r.FreeMargin.IsPositive() {
		// Keep a 2% buffer for fees and price movement until the fill.
		maxNotional = decimal.Min(maxNotional, r.FreeMargin.Mul(lev).Mul(decimal.NewFromFloat(0.98)))
	} else {
		return decimal.Zero, errors.New("size: no free margin")
	}
	if qty.Mul(r.Price).GreaterThan(maxNotional) {
		qty = maxNotional.Div(r.Price)
	}

	qty = r.Instrument.RoundQty(qty)
	if err := r.Instrument.CheckOrder(r.Price, qty); err != nil || !qty.IsPositive() {
		return decimal.Zero, fmt.Errorf("%w: %v", ErrTooSmall, err)
	}
	return qty, nil
}

// leverage caps the requested leverage.
func (m *Manager) leverage(requested int) int {
	l := max(requested, 1)
	if m.cfg.MaxLeverage > 0 {
		l = min(l, m.cfg.MaxLeverage)
	}
	return l
}

// Leverage returns the leverage that will actually be used.
func (m *Manager) Leverage(requested int, market domain.Market) int {
	if market == domain.Spot {
		return 1
	}
	return m.leverage(requested)
}
