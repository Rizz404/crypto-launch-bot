package managed

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// StartPrice returns the first 1m open of symbol on exchange at or after at.
type StartPrice func(ctx context.Context, exchange, symbol string, at time.Time) (decimal.Decimal, bool)

// benchmark is the equal-weight buy-and-hold return of a scope's pairs.
type benchmark struct {
	Pct   decimal.Decimal
	Pairs int
	Since time.Time
}

// BuyHold compares a scope ("account" or an instance) with simply holding
// its pairs, equal weight, from the moment it started.
func (v view) BuyHold(scope string) (benchmark, bool) {
	if v.startPrice == nil {
		return benchmark{}, false
	}
	since := v.State.StartedAt
	if i := v.State.Instances[scope]; i != nil && !i.Started.IsZero() {
		since = i.Started
	}
	symbols := map[string]bool{}
	for _, b := range v.State.Bindings {
		if b.Spec.Name != ManualStrategy && b.Status != "archived" && (scope == "account" || b.InstanceID == scope) {
			for _, s := range b.Spec.Symbols {
				symbols[s] = true
			}
		}
	}
	if len(symbols) == 0 {
		return benchmark{}, false
	}
	sum := decimal.Zero
	for sym := range symbols {
		start, ok := v.startPrice(sym, since)
		now := v.State.Prices[sym]
		if !ok || !start.IsPositive() || !now.IsPositive() {
			return benchmark{}, false
		}
		sum = sum.Add(now.Div(start).Sub(decimal.NewFromInt(1)))
	}
	pct := sum.Div(decimal.NewFromInt(int64(len(symbols)))).Mul(decimal.NewFromInt(100))
	return benchmark{Pct: pct, Pairs: len(symbols), Since: since}, true
}

// ReturnPct is a gain or loss over its base as a signed percentage.
func ReturnPct(gain, base decimal.Decimal) decimal.Decimal {
	if !base.IsPositive() {
		return decimal.Zero
	}
	return gain.Div(base).Mul(decimal.NewFromInt(100))
}

// signedPct formats -4.85 as "−4,85%" and 1.2 as "+1,20%".
func signedPct(d decimal.Decimal) string {
	s := idNumber(d, 2)
	if d.Round(2).IsPositive() {
		s = "+" + s
	}
	return s + "%"
}
