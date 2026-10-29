package launch

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
)

func (s *PaperState) broadcast(step PaperStep) error {
	o, ok := s.Orders[step.OrderID]
	if !ok || o.Status != "reserved" || !strings.HasPrefix(step.ExternalID, "fixture:") || !step.At.Before(o.Intent.ExpiresAt) {
		return errors.New("broadcast needs a fresh reserved fixture order")
	}
	if o.Intent.Candidate.Kind == "cex" {
		return errors.New("CEX uses book matching, not chain broadcast")
	}
	if o.Intent.Candidate.Kind == "dex" {
		q := step.Route
		if q == nil || !strings.HasPrefix(q.ID, "fixture:") || q.CandidateID != o.Intent.Candidate.ID || q.AssetKey != o.Intent.Candidate.Asset.Key() || q.At.Before(o.At) || q.At.After(step.At) || step.At.Sub(q.At) > 5*time.Second || !q.ExpiresAt.After(step.At) || !q.CostsKnown || !q.LiquidityVerified || !q.SellableVerified || q.FeeUSDT.IsNegative() || q.GasUSDT.IsNegative() || q.FeeUSDT.Add(q.GasUSDT).GreaterThan(o.Intent.MaxFeeUSDT) {
			return errors.New("DEX quote/route/cost/liquidity/exit evidence missing or stale")
		}
		if s.QuoteUsage[q.ID].IsPositive() {
			return errors.New("DEX route quote already consumed")
		}
		if o.Intent.Action == "buy" {
			if !q.InputUSDT.Equal(o.Intent.NotionalUSDT) || !q.Quantity.IsPositive() || !q.MinQuantity.IsPositive() || q.MinQuantity.GreaterThan(q.Quantity) || q.Quantity.GreaterThan(o.Intent.Quantity) || q.InputUSDT.Div(q.Quantity).GreaterThan(o.Intent.LimitPrice) {
				return errors.New("DEX entry exceeds price/input/min-out contract")
			}
		} else if !q.Quantity.Equal(o.Intent.Quantity) || !q.OutputUSDT.IsPositive() || !q.MinOutputUSDT.IsPositive() || q.MinOutputUSDT.GreaterThan(q.OutputUSDT) || q.OutputUSDT.Div(q.Quantity).LessThan(o.Intent.LimitPrice) {
			return errors.New("DEX exit exceeds price/min-out contract")
		}
		o.Route = q
		s.QuoteUsage[q.ID] = decimal.NewFromInt(1)
	}
	o.ExternalID = step.ExternalID
	o.Status = "pending"
	s.Orders[step.OrderID] = o
	return nil
}

func (s *PaperState) matchBook(orderID string, q PaperBookQuote, stepID string, now time.Time) error {
	o, ok := s.Orders[orderID]
	if !ok || o.Intent.Candidate.Kind != "cex" || (o.Status != "reserved" && o.Status != "partial") || !now.Before(o.Intent.ExpiresAt) {
		return errors.New("book order is not matchable; expired orders need a terminal acknowledgment")
	}
	if q.CandidateID != o.Intent.Candidate.ID || q.At.Before(o.At) || q.At.After(now) || now.Sub(q.At) > 5*time.Second || !q.CostsKnown || q.FeeFraction.IsNegative() || q.FeeFraction.GreaterThanOrEqual(decimal.NewFromInt(1)) || !q.LotStep.IsPositive() || q.MinNotional.IsNegative() || !q.Bid.IsPositive() || q.Bid.GreaterThan(q.Ask) || q.BidQty.IsNegative() || q.AskQty.IsNegative() {
		return errors.New("invalid/stale CEX book, fees or precision")
	}
	remaining := o.Intent.Quantity.Sub(o.Quantity)
	if !remaining.Mod(q.LotStep).IsZero() {
		return errors.New("quantity is not aligned to lot step")
	}
	if o.Intent.Action == "buy" && q.Bid.LessThanOrEqual(q.Ask.Mul(decimal.NewFromInt(1).Sub(o.Intent.StopFraction))) {
		return errors.New("entry book already crosses planned stop")
	}
	quoteKey := catalyst.Hash(q)
	side := domain.Buy
	level := q.Ask
	depth := q.AskQty
	if o.Intent.Action == "sell" {
		side = domain.Sell
		level = q.Bid
		depth = q.BidQty
	}
	usageKey := quoteKey + ":" + string(side)
	depth = decimal.Max(decimal.Zero, depth.Sub(s.QuoteUsage[usageKey].Mul(decimal.NewFromInt(10))))
	if remaining.Mul(level).LessThan(q.MinNotional) {
		return errors.New("remaining order is below minimum notional")
	}
	clock := o.At
	sim := broker.NewSim(broker.SimConfig{Exchange: "binance", Clock: func() time.Time { return clock }})
	var fill domain.Fill
	sim.SetHandler(func(event broker.Event) {
		if event.Kind == broker.EventFill {
			fill = event.Fill
		}
	})
	_, err := sim.PlaceOrder(context.Background(), domain.Order{ClientID: orderID, Exchange: "binance", Market: domain.Spot, Symbol: o.Intent.Candidate.ExternalID,
		Side: side, Type: domain.OrderLimit, Quantity: remaining, Price: o.Intent.LimitPrice})
	if err != nil {
		return err
	}
	clock = now
	book := domain.BookQuote{Exchange: "binance", Market: domain.Spot, Symbol: o.Intent.Candidate.ExternalID, At: q.At, Bid: q.Bid, Ask: q.Ask, BidQty: q.BidQty, AskQty: q.AskQty}
	if side == domain.Buy {
		book.AskQty = depth
	} else {
		book.BidQty = depth
	}
	if err = sim.MatchBook(book, q.LotStep, nil); err != nil {
		return err
	}
	if !fill.Quantity.IsPositive() {
		return nil
	}
	if fill.Quantity.Mul(fill.Price).LessThan(q.MinNotional) {
		return errors.New("available partial depth is below minimum notional")
	}
	notional := fill.Quantity.Mul(fill.Price)
	r := PaperReceipt{ID: "fixture:book:" + stepID, ExternalID: "fixture:order:" + orderID, Status: "partial", Finalized: true,
		Quantity: o.Quantity.Add(fill.Quantity), SpentUSDT: o.SpentUSDT, ReceivedUSDT: o.ReceivedUSDT, FeeUSDT: o.FeeUSDT.Add(notional.Mul(q.FeeFraction))}
	if side == domain.Buy {
		r.SpentUSDT = r.SpentUSDT.Add(notional)
	} else {
		r.ReceivedUSDT = r.ReceivedUSDT.Add(notional)
	}
	if r.Quantity.Equal(o.Intent.Quantity) {
		r.Status = "filled"
	}
	if err = s.receipt(orderID, r, now); err != nil {
		return err
	}
	s.QuoteUsage[usageKey] = s.QuoteUsage[usageKey].Add(fill.Quantity)
	position := s.Positions[o.Intent.PositionID]
	position.MarkAt = q.At
	position.MarkPrice = q.Bid
	if position.StopPrice.IsPositive() && q.Bid.LessThanOrEqual(position.StopPrice) {
		position.ExitRequired = true
	}
	s.Positions[position.ID] = position
	return nil
}

func (s *PaperState) receipt(orderID string, r PaperReceipt, now time.Time) error {
	o, ok := s.Orders[orderID]
	if !ok || !strings.HasPrefix(r.ID, "fixture:") || !strings.HasPrefix(r.ExternalID, "fixture:") {
		return errors.New("receipt requires an existing order and fixture references")
	}
	hash := catalyst.Hash([]any{orderID, r})
	if old, ok := s.Receipts[r.ID]; ok {
		if old != hash {
			return errors.New("receipt ID reused with different payload")
		}
		return nil
	}
	if o.ExternalID != "" && o.ExternalID != r.ExternalID {
		return errors.New("receipt references a different external order/transaction")
	}
	chain := o.Intent.Candidate.Kind != "cex"
	if r.Status == "reorg" {
		if !chain {
			return errors.New("CEX order cannot have an on-chain reorg")
		}
		if orderOpen(o) {
			o.Status = "unknown"
			s.Orders[orderID] = o
		} else {
			s.Attention = true
		}
		s.Receipts[r.ID] = hash
		return nil
	}
	if !orderOpen(o) {
		return errors.New("terminal order cannot settle a new receipt")
	}
	if !slicesReceiptStatus(r.Status) {
		return errors.New("unknown receipt status")
	}
	if chain && o.ExternalID == "" {
		return errors.New("on-chain receipt has no prior durable broadcast")
	}
	o.ExternalID = r.ExternalID
	if chain && (!r.Finalized || r.Status == "pending" || r.Status == "confirmed") {
		o.Status = "pending"
		if r.Status == "confirmed" {
			o.Status = "confirmed"
		}
		s.Orders[orderID] = o
		s.Receipts[r.ID] = hash
		return nil
	}
	if !chain && (r.Status == "pending" || r.Status == "confirmed" || !r.Finalized) {
		return errors.New("CEX settlement requires a terminal/partial fixture acknowledgment")
	}
	if chain && (r.BlockNumber == 0 || !strings.HasPrefix(r.BlockHash, "fixture:")) {
		return errors.New("on-chain settlement lacks finalized fixture block proof")
	}
	if chain && r.Status == "partial" {
		return errors.New("on-chain transactions cannot settle as partial exchange orders")
	}
	if !chain && (o.Intent.Action == "buy" || o.Intent.Action == "sell") {
		if r.Status == "filled" && !r.Quantity.Equal(o.Intent.Quantity) {
			return errors.New("filled CEX order has incomplete quantity")
		}
		if r.Status == "partial" && (!r.Quantity.IsPositive() || !r.Quantity.LessThan(o.Intent.Quantity)) {
			return errors.New("partial CEX quantity is not partial")
		}
	}
	if r.Quantity.LessThan(o.Quantity) || r.SpentUSDT.LessThan(o.SpentUSDT) || r.ReceivedUSDT.LessThan(o.ReceivedUSDT) || r.FeeUSDT.LessThan(o.FeeUSDT) || r.FeeUSDT.GreaterThan(o.Intent.MaxFeeUSDT) {
		return errors.New("cumulative receipt regressed or exceeded fee budget")
	}
	dq := r.Quantity.Sub(o.Quantity)
	spent := r.SpentUSDT.Sub(o.SpentUSDT)
	received := r.ReceivedUSDT.Sub(o.ReceivedUSDT)
	fee := r.FeeUSDT.Sub(o.FeeUSDT)
	if r.Status != "filled" && r.Status != "partial" && (dq.IsPositive() || spent.IsPositive() || received.IsPositive()) {
		return errors.New("failure/cancel acknowledgment cannot invent new fills")
	}
	if (r.Status == "filled" || r.Status == "partial") && !now.Before(o.Intent.ExpiresAt) && o.Intent.Candidate.Kind == "cex" {
		// A late external acknowledgment can settle prior fills after expiry;
		// local book matching cannot create such fills (matchBook checks expiry).
		if !o.At.Before(o.Intent.ExpiresAt) {
			return errors.New("invalid order lifetime")
		}
	}
	position, exists := s.Positions[o.Intent.PositionID]
	if !exists {
		position = PaperPosition{ID: o.Intent.PositionID, Candidate: o.Intent.Candidate, FeeBudgetUSDT: o.Intent.MaxFeeUSDT, Thesis: o.Intent.Thesis, Invalidation: o.Intent.Invalidation}
	}
	if fee.GreaterThan(position.FeeBudgetUSDT) {
		return errors.New("remaining position fee budget exceeded")
	}
	debit := fee
	switch o.Intent.Action {
	case "buy":
		if !r.ReceivedUSDT.IsZero() || r.Quantity.GreaterThan(o.Intent.Quantity) || r.SpentUSDT.GreaterThan(o.Intent.NotionalUSDT) || dq.IsZero() != spent.IsZero() || (dq.IsPositive() && spent.Div(dq).GreaterThan(o.Intent.LimitPrice)) {
			return errors.New("entry fill exceeds quantity, notional or limit")
		}
		if r.Status == "filled" && !r.Quantity.Equal(o.Intent.Quantity) && !chain {
			return errors.New("filled CEX order has incomplete quantity")
		}
		if chain && r.Status == "filled" {
			if err := validateRouteFill(o, r); err != nil {
				return err
			}
		}
		position.Quantity = position.Quantity.Add(dq)
		position.CostUSDT = position.CostUSDT.Add(spent)
		debit = debit.Add(spent)
		if o.Intent.Candidate.Kind == "cex" && position.Quantity.IsPositive() {
			position.StopPrice = position.CostUSDT.Div(position.Quantity).Mul(decimal.NewFromInt(1).Sub(o.Intent.StopFraction))
		}
	case "contribute":
		if !r.Quantity.IsZero() || !r.ReceivedUSDT.IsZero() || r.SpentUSDT.GreaterThan(o.Intent.NotionalUSDT) || (r.Status == "filled" && !spent.IsPositive()) {
			return errors.New("contribution cannot create tradable tokens")
		}
		position.LockedUSDT = position.LockedUSDT.Add(spent)
		position.ContributedUSDT = position.ContributedUSDT.Add(spent)
		debit = debit.Add(spent)
	case "sell":
		if !r.SpentUSDT.IsZero() || r.Quantity.GreaterThan(o.Intent.Quantity) || dq.GreaterThan(position.Quantity) || dq.IsZero() != received.IsZero() || (dq.IsPositive() && received.Div(dq).LessThan(o.Intent.LimitPrice)) {
			return errors.New("exit exceeds tradable inventory or price floor")
		}
		if chain && r.Status == "filled" {
			if err := validateRouteFill(o, r); err != nil {
				return err
			}
		}
		basis := proportion(position.CostUSDT, dq, position.Quantity)
		position.Quantity = position.Quantity.Sub(dq)
		position.CostUSDT = position.CostUSDT.Sub(basis)
		s.RealizedGrossUSDT = s.RealizedGrossUSDT.Add(received.Sub(basis))
		s.CashUSDT = s.CashUSDT.Add(received)
	case "claim":
		if r.Status == "filled" && !r.Quantity.Equal(o.Intent.Quantity) {
			return errors.New("claim result differs from requested unlocked quantity")
		}
		if !r.SpentUSDT.IsZero() || !r.ReceivedUSDT.IsZero() || r.Quantity.GreaterThan(o.Intent.Quantity) || dq.GreaterThan(position.UnlockedQuantity.Sub(position.ClaimedQuantity)) || dq.GreaterThan(position.AllocatedQuantity.Sub(position.ClaimedQuantity)) {
			return errors.New("claim exceeds unlocked allocation")
		}
		basis := proportion(position.LockedUSDT, dq, position.AllocatedQuantity.Sub(position.ClaimedQuantity))
		position.LockedUSDT = position.LockedUSDT.Sub(basis)
		position.CostUSDT = position.CostUSDT.Add(basis)
		position.Quantity = position.Quantity.Add(dq)
		position.ClaimedQuantity = position.ClaimedQuantity.Add(dq)
	case "refund":
		if r.Status == "filled" && !r.ReceivedUSDT.Equal(o.Intent.NotionalUSDT) {
			return errors.New("refund result differs from requested principal")
		}
		if !r.Quantity.IsZero() || !r.SpentUSDT.IsZero() || r.ReceivedUSDT.GreaterThan(o.Intent.NotionalUSDT) || received.GreaterThan(position.RefundableUSDT.Sub(position.RefundedUSDT)) || received.GreaterThan(position.LockedUSDT) {
			return errors.New("refund exceeds verified refundable principal")
		}
		position.RefundedUSDT = position.RefundedUSDT.Add(received)
		position.LockedUSDT = position.LockedUSDT.Sub(received)
		s.CashUSDT = s.CashUSDT.Add(received)
	default:
		return errors.New("invalid settlement action")
	}
	if debit.GreaterThan(o.ReserveUSDT) {
		return errors.New("settlement exceeds durable reservation")
	}
	o.ReserveUSDT = o.ReserveUSDT.Sub(debit)
	o.Quantity = r.Quantity
	o.SpentUSDT = r.SpentUSDT
	o.ReceivedUSDT = r.ReceivedUSDT
	o.FeeUSDT = r.FeeUSDT
	o.Status = r.Status
	position.FeeBudgetUSDT = position.FeeBudgetUSDT.Sub(fee)
	s.FeesUSDT = s.FeesUSDT.Add(fee)
	if !orderOpen(o) {
		s.CashUSDT = s.CashUSDT.Add(o.ReserveUSDT)
		o.ReserveUSDT = decimal.Zero
	}
	s.Orders[orderID] = o
	s.Positions[position.ID] = position
	s.Receipts[r.ID] = hash
	return nil
}

func proportion(total, part, whole decimal.Decimal) decimal.Decimal {
	if part.IsZero() {
		return decimal.Zero
	}
	if part.Equal(whole) {
		return total
	}
	return total.Mul(part).Div(whole)
}
func slicesReceiptStatus(status string) bool {
	switch status {
	case "pending", "confirmed", "partial", "filled", "canceled", "failed", "rejected":
		return true
	}
	return false
}
func validateRouteFill(o PaperOrder, r PaperReceipt) error {
	q := o.Route
	if q == nil {
		return errors.New("DEX fill has no durable route quote")
	}
	if !r.FeeUSDT.Equal(q.FeeUSDT.Add(q.GasUSDT)) {
		return errors.New("DEX fill fees differ from modeled quote")
	}
	if o.Intent.Action == "buy" {
		if !r.SpentUSDT.Equal(q.InputUSDT) || r.Quantity.LessThan(q.MinQuantity) || r.Quantity.GreaterThan(q.Quantity) {
			return errors.New("DEX entry violates route amount/min-out")
		}
	} else if !r.Quantity.Equal(q.Quantity) || r.ReceivedUSDT.LessThan(q.MinOutputUSDT) || r.ReceivedUSDT.GreaterThan(q.OutputUSDT) {
		return errors.New("DEX exit violates route amount/min-out")
	}
	return nil
}
