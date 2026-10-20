package managed

import (
	"errors"
	"fmt"
	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
)

// Reserve records the intention BEFORE submitting an order. Only a definitive
// reject/terminal acknowledgment releases funds. OCO legs share one reservation.
func (s *State) ReserveOrder(owner string, o domain.Order, group string, fee decimal.Decimal) error {
	b := s.Bindings[owner]
	if b == nil {
		return errors.New("unknown strategy owner")
	}
	i := s.Instances[b.InstanceID]
	if old := s.Orders[o.ClientID]; old != nil {
		return errors.New("duplicate client order id")
	}
	reducing := o.Side == domain.Sell
	if s.Attention != "" && !reducing {
		return ErrAttention
	}
	if !reducing && (s.Halted || i.Halted || b.Halted || i.Status != "running" || b.Status != "running") {
		return errors.New("entries paused by scope or risk")
	}
	if !o.Quantity.IsPositive() {
		return errors.New("invalid order quantity")
	}
	price := o.Price
	if !price.IsPositive() {
		price = s.Prices[o.Symbol]
	}
	if !price.IsPositive() {
		return errors.New("price unavailable")
	}
	r := &Order{Owner: owner, Order: o, Group: group, State: "submitting"}
	shared := false
	for _, old := range s.Orders {
		if group != "" && old.Group == group && old.State != "terminal" {
			if old.Owner != owner || old.Order.Symbol != o.Symbol || old.Order.Side != o.Side || !old.Order.Quantity.Equal(o.Quantity) {
				return errors.New("invalid shared reservation")
			}
			shared = true
			r.QuoteReserved = old.QuoteReserved
			r.BaseReserved = old.BaseReserved
			break
		}
	}
	if !shared {
		if o.Side == domain.Buy {
			r.QuoteReserved = o.Quantity.Mul(price).Mul(decimal.NewFromInt(1).Add(fee))
			if s.FreeCash(i.ID).LessThan(r.QuoteReserved) {
				return ErrFunds
			}
			pending, _ := s.Reserved("", owner, "")
			if b.Spec.Cap.IsPositive() && s.Inventory(owner).Add(pending).Add(r.QuoteReserved).GreaterThan(b.Spec.Cap) {
				return fmt.Errorf("strategy capital cap exceeded")
			}
			if err := s.CheckExposure(owner, o.Symbol, o.Quantity.Mul(price)); err != nil {
				return err
			}
		} else {
			_, reserved := s.Reserved("", owner, o.Symbol)
			held := s.Holding(owner, o.Symbol)
			r.BaseReserved = o.Quantity
			if held.Quantity.Sub(reserved).LessThan(o.Quantity) {
				return ErrFunds
			}
		}
	}
	s.Orders[o.ClientID] = r
	return nil
}
func (s *State) Reject(id, reason string) {
	if o := s.Orders[id]; o != nil {
		o.State = "terminal"
		o.Order.Status = domain.OrderRejected
		o.Order.Reason = reason
		o.QuoteReserved = decimal.Zero
		o.BaseReserved = decimal.Zero
	}
}
func (s *State) UpdateOrder(upd domain.Order) {
	o := s.Orders[upd.ClientID]
	if o == nil {
		return
	}
	o.Order.ExchangeID = upd.ExchangeID
	o.Order.UpdatedAt = upd.UpdatedAt
	if upd.Status.Open() {
		if o.State == "submitting" {
			o.State = "open"
		}
		return
	}
	// A filled status can precede the executions. Do not discard reservations.
	if upd.Status == domain.OrderFilled && o.Order.FilledQty.LessThan(o.Order.Quantity) {
		return
	}
	o.Order.Status = upd.Status
	o.State = "terminal"
	o.QuoteReserved = decimal.Zero
	o.BaseReserved = decimal.Zero
}

// Fill applies one execution once. Decimal Fee is quote-equivalent; BaseFee
// subtracts the coin commission from received inventory, not a second cash fee.
func (s *State) Fill(f domain.Fill) (bool, error) {
	o := s.Orders[f.OrderClientID]
	if o == nil {
		return false, errors.New("fill has no owner")
	}
	if f.TradeID == "" {
		return false, errors.New("execution requires a stable trade id")
	}
	key := f.OrderClientID + ":" + f.TradeID
	if s.Seen[key] {
		return false, nil
	}
	if !f.Quantity.IsPositive() || !f.Price.IsPositive() || f.BaseFee.IsNegative() || f.BaseFee.GreaterThanOrEqual(f.Quantity) || f.Fee.IsNegative() {
		return false, errors.New("invalid execution")
	}
	if f.Symbol != o.Order.Symbol || f.Side != o.Order.Side || f.Exchange != o.Order.Exchange || f.Market != domain.Spot {
		return false, errors.New("execution identity mismatch")
	}
	b := s.Bindings[o.Owner]
	i := s.Instances[b.InstanceID]
	p := s.Holding(o.Owner, f.Symbol)
	notional := f.Quantity.Mul(f.Price)
	quoteFee := f.Fee.Sub(f.BaseFee.Mul(f.Price))
	if quoteFee.IsNegative() {
		quoteFee = decimal.Zero
	}
	if f.Side == domain.Buy {
		i.Cash = i.Cash.Sub(notional).Sub(quoteFee)
		p.Quantity = p.Quantity.Add(f.Quantity.Sub(f.BaseFee))
		p.Cost = p.Cost.Add(notional).Add(quoteFee)
	} else {
		if p.Quantity.LessThan(f.Quantity) {
			return false, errors.New("sell execution exceeds owned inventory")
		}
		cost := p.Cost.Mul(f.Quantity).Div(p.Quantity)
		p.Quantity = p.Quantity.Sub(f.Quantity)
		p.Cost = p.Cost.Sub(cost)
		proceeds := notional.Sub(f.Fee)
		i.Cash = i.Cash.Add(proceeds)
		b.Realized = b.Realized.Add(proceeds.Sub(cost))
	}
	b.Fees = b.Fees.Add(f.Fee)
	s.Seen[key] = true
	s.Mark(f.Symbol, f.Price)
	previousRemaining := o.Order.Quantity.Sub(o.Order.FilledQty)
	o.Order.FilledQty = o.Order.FilledQty.Add(f.Quantity)
	remaining := decimal.Max(o.Order.Quantity.Sub(o.Order.FilledQty), decimal.Zero)
	ratio := remaining.Div(previousRemaining)
	o.QuoteReserved = o.QuoteReserved.Mul(ratio)
	o.BaseReserved = remaining
	if o.Order.Side == domain.Buy {
		o.BaseReserved = decimal.Zero
	}
	// Update all linked legs to the remaining owned quantity until terminal ack.
	if o.Group != "" {
		for _, leg := range s.Orders {
			if leg.Group == o.Group && leg != o {
				leg.BaseReserved = remaining
			}
		}
	}
	if !remaining.IsPositive() {
		o.State = "terminal"
		o.Order.Status = domain.OrderFilled
		o.QuoteReserved = decimal.Zero
		o.BaseReserved = decimal.Zero
	} else {
		o.Order.Status = domain.OrderPartiallyFilled
	}
	return true, nil
}
func (s *State) Reconcile(st *broker.AccountState, instruments map[string]domain.Instrument) {
	expected := map[string]decimal.Decimal{s.Spec.Quote: s.Reserve}
	for _, i := range s.Instances {
		expected[s.Spec.Quote] = expected[s.Spec.Quote].Add(i.Cash)
	}
	for _, p := range s.Holdings {
		in := instruments[p.Symbol]
		expected[in.Base] = expected[in.Base].Add(p.Quantity)
	}
	if len(s.Baseline) == 0 {
		for asset, b := range st.Balances {
			s.Baseline[asset] = b.Wallet.Sub(expected[asset])
		}
		for asset, v := range expected {
			if st.Balances[asset].Wallet.LessThan(v) {
				s.Attention = "wallet does not cover funded bot assets"
				return
			}
		}
		return
	}
	for asset, v := range expected {
		actual := st.Balances[asset].Wallet
		want := v.Add(s.Baseline[asset])
		if !actual.Equal(want) {
			s.Attention = "wallet discrepancy in " + asset
			return
		}
	}
	for _, o := range st.OpenOrders {
		if s.Orders[o.ClientID] == nil {
			s.Attention = "unknown exchange order " + o.Symbol
			return
		}
	}
	s.Attention = ""
}
