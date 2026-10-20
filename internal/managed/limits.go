package managed

import (
	"errors"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/shopspring/decimal"
	"time"
)

func effectiveRisk(c risk.Config) risk.Config {
	if c == (risk.Config{}) {
		return risk.DefaultConfig()
	}
	return c
}
func (s *State) CheckExposure(owner, symbol string, amount decimal.Decimal) error {
	b := s.Bindings[owner]
	for _, scope := range []string{"account", b.InstanceID, owner} {
		c := s.Spec.Risk
		if scope == b.InstanceID {
			c = s.Instances[b.InstanceID].Risk
		}
		if scope == owner {
			c = b.Spec.Risk
		}
		c = effectiveRisk(c)
		positions := map[string]bool{}
		same := decimal.Zero
		for _, p := range s.Holdings {
			binding := s.Bindings[p.Owner]
			if (scope == "account" || scope == binding.InstanceID || scope == p.Owner) && p.Quantity.IsPositive() {
				positions[p.Owner+":"+p.Symbol] = true
				if p.Symbol == symbol {
					same = same.Add(p.Quantity.Mul(s.Prices[p.Symbol]))
				}
			}
		}
		for _, o := range s.Orders {
			binding := s.Bindings[o.Owner]
			if o.State != "terminal" && o.Order.Side == "buy" && (scope == "account" || scope == binding.InstanceID || scope == o.Owner) {
				positions[o.Owner+":"+o.Order.Symbol] = true
				if o.Order.Symbol == symbol {
					same = same.Add(o.QuoteReserved)
				}
			}
		}
		if !positions[owner+":"+symbol] && c.MaxOpenPositions > 0 && len(positions) >= c.MaxOpenPositions {
			return errors.New("parent position count limit exceeded")
		}
		equity := s.Equity()
		if scope != "account" {
			equity = s.InstanceEquity(b.InstanceID)
		}
		if c.MaxPositionPct > 0 && same.Add(amount).GreaterThan(equity.Mul(decimal.NewFromFloat(c.MaxPositionPct))) {
			return errors.New("aggregate symbol position limit exceeded")
		}
	}

	i := s.Instances[b.InstanceID]
	check := func(c risk.Config, equity, held, pending decimal.Decimal) error {
		c = effectiveRisk(c)
		if c.MaxExposurePct > 0 && held.Add(pending).Add(amount).GreaterThan(equity.Mul(decimal.NewFromFloat(c.MaxExposurePct))) {
			return errors.New("parent exposure limit exceeded")
		}
		if c.MaxPositionPct > 0 && amount.GreaterThan(equity.Mul(decimal.NewFromFloat(c.MaxPositionPct))) {
			return errors.New("position limit exceeded")
		}
		return nil
	}
	pending, _ := s.Reserved("", "", "")
	if err := check(s.Spec.Risk, s.Equity(), s.Inventory(""), pending); err != nil {
		return err
	}
	inv := decimal.Zero
	for id, other := range s.Bindings {
		if other.InstanceID == i.ID {
			inv = inv.Add(s.Inventory(id))
		}
	}
	pending, _ = s.Reserved(i.ID, "", "")
	if err := check(i.Risk, s.InstanceEquity(i.ID), inv, pending); err != nil {
		return err
	}
	pending, _ = s.Reserved("", owner, "")
	return check(b.Spec.Risk, s.InstanceEquity(i.ID), s.Inventory(owner), pending)
}
func observe(c risk.Config, equity decimal.Decimal, day, now time.Time, start, peak decimal.Decimal) (time.Time, decimal.Decimal, decimal.Decimal, bool) {
	c = effectiveRisk(c)
	today := now.UTC().Truncate(24 * time.Hour)
	if !day.Equal(today) {
		day = today
		start = equity
	}
	peak = decimal.Max(peak, equity)
	halt := (c.DailyLossLimit > 0 && start.IsPositive() && equity.LessThanOrEqual(start.Mul(decimal.NewFromFloat(1-c.DailyLossLimit)))) || (c.MaxDrawdown > 0 && peak.IsPositive() && equity.LessThanOrEqual(peak.Mul(decimal.NewFromFloat(1-c.MaxDrawdown))))
	return day, start, peak, halt
}
func (s *State) Observe(now time.Time) {
	var halt bool
	s.Day, s.DayStart, s.Peak, halt = observe(s.Spec.Risk, s.Equity(), s.Day, now, s.DayStart, s.Peak)
	if halt {
		s.Halted = true
		s.Reason = "account loss limit"
	}
	for id, i := range s.Instances {
		i.Day, i.DayStart, i.Peak, halt = observe(i.Risk, s.InstanceEquity(id), i.Day, now, i.DayStart, i.Peak)
		if halt {
			i.Halted = true
			i.Reason = "instance loss limit"
		}
	}
	for owner, b := range s.Bindings {
		capital := s.Instances[b.InstanceID].Funded
		if b.Spec.Cap.IsPositive() {
			capital = b.Spec.Cap
		}
		equity := capital.Add(b.Realized).Add(s.Inventory(owner))
		for _, p := range s.Holdings {
			if p.Owner == owner {
				equity = equity.Sub(p.Cost)
			}
		}
		b.Day, b.DayStart, b.Peak, halt = observe(b.Spec.Risk, equity, b.Day, now, b.DayStart, b.Peak)
		if halt {
			b.Halted = true
			b.Reason = "strategy loss limit"
		}
	}

}
