package managed

import (
	"errors"
	"fmt"
	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
	"strings"
	"time"
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
		o.SettledAt = time.Now().UTC()
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
	if o.State == "terminal" && (upd.Status.Open() || o.Order.Status == domain.OrderFilled) {
		return
	}
	o.Order.ExchangeID = upd.ExchangeID
	o.Order.UpdatedAt = upd.UpdatedAt
	if upd.Status.Open() {
		if o.State == "submitting" || o.State == "unknown" {
			o.State = "open"
		}
		return
	}
	// A terminal acknowledgment may arrive before its executions. Keep the
	// reservation until the acknowledged cumulative quantity has been booked.
	if o.Order.FilledQty.LessThan(upd.FilledQty) || (upd.Status == domain.OrderFilled && o.Order.FilledQty.LessThan(o.Order.Quantity)) {
		cp := upd
		o.Deferred = &cp
		return
	}
	o.Deferred = nil
	o.Order.Status = upd.Status
	o.State = "terminal"
	o.SettledAt = time.Now().UTC()
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
	if f.Quantity.GreaterThan(o.Order.Quantity.Sub(o.Order.FilledQty)) {
		return false, errors.New("execution exceeds remaining order quantity")
	}
	b := s.Bindings[o.Owner]
	i := s.Instances[b.InstanceID]
	p := s.Holding(o.Owner, f.Symbol)
	realizedBefore := b.Realized
	notional := f.Quantity.Mul(f.Price)
	thirdFee := f.FeeAsset != "" && f.FeeAsset != "QUOTE" && f.FeeAsset != s.Spec.Quote && !strings.HasPrefix(f.Symbol, f.FeeAsset)
	if f.FeeUnknown {
		s.FeeIssue = "recovered execution needs trade-level fee review"
		s.Attention = s.FeeIssue
	}
	if thirdFee && f.Commission.IsPositive() {
		s.FeeAssets[f.FeeAsset] = s.FeeAssets[f.FeeAsset].Add(f.Commission)
		s.PendingFees[key] = FeeCharge{Owner: o.Owner, Asset: f.FeeAsset, Quantity: f.Commission}
	}
	quoteFee := f.Fee.Sub(f.BaseFee.Mul(f.Price))
	if thirdFee {
		quoteFee = decimal.Zero
	}
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
		sellFee := f.Fee
		if thirdFee {
			sellFee = decimal.Zero
		}
		proceeds := notional.Sub(sellFee)
		i.Cash = i.Cash.Add(proceeds)
		b.Realized = b.Realized.Add(proceeds.Sub(cost))
	}
	if !thirdFee {
		b.Fees = b.Fees.Add(f.Fee)
	}
	s.valueFees()
	s.Seen[key] = true
	s.Executions = append(s.Executions, Execution{Owner: o.Owner, Fill: f, Realized: b.Realized.Sub(realizedBefore)})
	s.Mark(f.Symbol, f.Price)
	previousRemaining := o.Order.Quantity.Sub(o.Order.FilledQty)
	if f.Quantity.GreaterThan(previousRemaining) {
		return false, errors.New("execution exceeds remaining order quantity")
	}
	o.Order.AvgPrice = o.Order.AvgPrice.Mul(o.Order.FilledQty).Add(f.Price.Mul(f.Quantity)).Div(o.Order.FilledQty.Add(f.Quantity))
	o.Order.FilledQty = o.Order.FilledQty.Add(f.Quantity)
	remaining := decimal.Max(o.Order.Quantity.Sub(o.Order.FilledQty), decimal.Zero)
	o.QuoteReserved = o.QuoteReserved.Div(previousRemaining).Mul(remaining)
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
		o.SettledAt = time.Now().UTC()
		o.Order.Status = domain.OrderFilled
		o.QuoteReserved = decimal.Zero
		o.BaseReserved = decimal.Zero
	} else {
		o.Order.Status = domain.OrderPartiallyFilled
	}
	if o.Deferred != nil {
		s.UpdateOrder(*o.Deferred)
	}
	return true, nil
}
func (s *State) Reconcile(st *broker.AccountState, instruments map[string]domain.Instrument) {
	s.ReconciledAt = time.Now().UTC()
	expected := map[string]decimal.Decimal{s.Spec.Quote: s.Reserve.Add(s.FeeQuote)}
	for _, i := range s.Instances {
		expected[s.Spec.Quote] = expected[s.Spec.Quote].Add(i.Cash)
	}
	for _, p := range s.Holdings {
		in := instruments[p.Symbol]
		expected[in.Base] = expected[in.Base].Add(p.Quantity)
	}
	if len(s.Baseline) == 0 {
		// Check coverage before persisting a baseline: a negative baseline would
		// make the next reconciliation accept the shortfall and clear attention.
		for asset, v := range expected {
			if st.Balances[asset].Wallet.LessThan(v) {
				s.Attention = "wallet does not cover funded bot assets"
				return
			}
		}
		for asset, b := range st.Balances {
			s.Baseline[asset] = b.Wallet.Sub(expected[asset]).Add(s.FeeAssets[asset])
		}
	}
	for asset := range st.Balances {
		if _, ok := expected[asset]; !ok {
			expected[asset] = decimal.Zero
		}
	}
	for asset := range s.Baseline {
		if _, ok := expected[asset]; !ok {
			expected[asset] = decimal.Zero
		}
	}
	for asset, v := range expected {
		actual := st.Balances[asset].Wallet
		want := v.Add(s.Baseline[asset]).Sub(s.FeeAssets[asset])
		if !actual.Equal(want) {
			s.Attention = "wallet discrepancy in " + asset
			return
		}
	}
	for _, o := range st.OpenOrders {
		if s.Orders[o.ClientID] == nil || s.Orders[o.ClientID].State == "terminal" {
			s.Attention = "unknown exchange order " + o.Symbol
			return
		}
	}
	for _, o := range s.Orders {
		if o.State == "canceling" {
			s.Attention = "cancellation awaiting exchange confirmation"
			return
		}
		if o.State == "unknown" || o.State == "submitting" {
			found := false
			for _, ex := range st.OpenOrders {
				if ex.ClientID == o.Order.ClientID {
					found = true
				}
			}
			if !found {
				s.Attention = "unresolved order submission"
				return
			}
		}
	}
	s.Attention = s.FeeIssue
}

// valueFees charges a strategy the quote value of third-asset commissions while
// preserving the actual fee asset debit for wallet reconciliation.
func (s *State) valueFees() {
	for key, f := range s.PendingFees {
		price := s.Prices[f.Asset+s.Spec.Quote]
		if !price.IsPositive() {
			s.FeeIssue = "waiting for commission asset valuation"
			s.Attention = s.FeeIssue
			continue
		}
		value := f.Quantity.Mul(price)
		b := s.Bindings[f.Owner]
		s.Instances[b.InstanceID].Cash = s.Instances[b.InstanceID].Cash.Sub(value)
		b.Realized = b.Realized.Sub(value)
		b.Fees = b.Fees.Add(value)
		s.FeeQuote = s.FeeQuote.Add(value)
		delete(s.PendingFees, key)
	}
	if len(s.PendingFees) == 0 && s.FeeIssue == "waiting for commission asset valuation" {
		if s.Attention == s.FeeIssue {
			s.Attention = ""
		}
		s.FeeIssue = ""
	}
}
