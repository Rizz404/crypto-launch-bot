package managed

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

// ManualStrategy is the strategy name of bindings that take operator orders.
const ManualStrategy = "manual"

var manualOperations = map[string]bool{"place_order": true, "cancel_order": true, "close_position": true}

// manual runs an operator order through the binding's engine, so sizing, risk
// limits, reservations and protection are the same as for strategy orders.
//   - place_order: only on a "manual" binding; scope is the binding
//   - close_position: any binding (intervening in a bot); scope is the binding
//   - cancel_order: an open entry/strategy order of any binding, by client id
//
// The request is journaled first under its idempotency key; a replay is a no-op.
func (r *Runtime) manual(ctx context.Context, c Command) error {
	if !c.Confirm {
		return errors.New("explicit confirmation required")
	}
	st, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	if _, ok := st.Requests[c.ID]; ok {
		return nil
	}
	owner, intents, err := manualIntents(st, c)
	if err != nil {
		return err
	}
	e := r.engines[owner]
	if e == nil {
		return errors.New("restart runtime before using a newly created strategy")
	}
	note := fmt.Sprintf("%s %s %s qty=%s price=%s", c.Side, c.OrderType, c.Symbol, c.Quantity, c.Price)
	if c.Operation == "cancel_order" {
		note = "order " + c.Order
	}
	if err = r.repo.Update(ctx, c.ID, "control", c, func(s *State) error { s.Note(c.Operation, owner, note); return nil }); err != nil {
		return err
	}
	if err = r.syncView(ctx, owner); err != nil {
		return err
	}
	var actErr error
	if c.Operation == "cancel_order" {
		if !e.Cancel(ctx, c.Order) {
			actErr = errors.New("order is not open, or is a stop/target leg that protects a position")
		}
	} else {
		actErr = e.Submit(ctx, owner, intents)
	}
	if err = r.save(ctx, owner); err != nil {
		return err
	}
	if err = r.drain(ctx); err != nil {
		return err
	}
	if err = r.finalize(ctx); err != nil {
		return err
	}
	return actErr
}

// manualIntents validates c against the ledger and returns the engine owner
// and the intents to submit.
func manualIntents(s *State, c Command) (string, []strategy.Intent, error) {
	if c.Operation == "cancel_order" {
		o := s.Orders[c.Order]
		if o == nil || o.State == "terminal" {
			return "", nil, errors.New("unknown or finished order")
		}
		return o.Owner, nil, nil
	}
	b := s.Bindings[c.Scope]
	if b == nil {
		return "", nil, errors.New("choose a strategy (instance/strategy) as scope")
	}
	if !slices.Contains(b.Spec.Symbols, c.Symbol) {
		return "", nil, fmt.Errorf("%s is not a symbol of %s", c.Symbol, b.ID)
	}
	if c.Operation == "close_position" {
		if s.positionFlat(b.ID) {
			return "", nil, errors.New("no position to close")
		}
		return b.ID, []strategy.Intent{{Kind: strategy.Exit, Symbol: c.Symbol, Reason: "manual close"}}, nil
	}
	if b.Spec.Name != ManualStrategy {
		return "", nil, errors.New("manual orders need a manual strategy binding; bots are changed with close or cancel")
	}
	if c.Quantity.IsNegative() || c.Price.IsNegative() || c.StopLoss.IsNegative() || c.TakeProfit.IsNegative() {
		return "", nil, errors.New("negative order values")
	}
	limit := c.OrderType == "limit"
	if !limit && c.OrderType != "market" {
		return "", nil, errors.New("order type must be market or limit")
	}
	if limit && !c.Price.IsPositive() {
		return "", nil, errors.New("limit order needs a price")
	}
	if (limit || c.Side == "sell") && (c.StopLoss.IsPositive() || c.TakeProfit.IsPositive()) {
		return "", nil, errors.New("stop loss and take profit attach to market buys only")
	}
	held := s.holding(b.ID, c.Symbol)
	in := strategy.Intent{Symbol: c.Symbol, Reason: "manual " + c.Side + " " + c.OrderType}
	switch c.Side {
	case "buy":
		if !c.Quantity.IsPositive() {
			return "", nil, errors.New("quantity required")
		}
		if limit {
			in.Kind, in.Ref = strategy.Place, c.ID
			in.Order = strategy.OrderSpec{Side: domain.Buy, Type: domain.OrderLimit, Quantity: c.Quantity, Price: c.Price}
		} else {
			if c.StopLoss.IsPositive() && c.TakeProfit.IsPositive() && !c.StopLoss.LessThan(c.TakeProfit) {
				return "", nil, errors.New("stop loss must be below take profit")
			}
			in.Kind, in.Side, in.Quantity = strategy.Enter, domain.Long, c.Quantity
			in.StopLoss, in.TakeProfit = c.StopLoss, c.TakeProfit
		}
	case "sell":
		if !held.IsPositive() {
			return "", nil, errors.New("nothing to sell; spot has no shorts")
		}
		qty := c.Quantity
		if !qty.IsPositive() || qty.GreaterThan(held) {
			return "", nil, errors.New("sell quantity must be positive and at most the holding")
		}
		if !limit && qty.Equal(held) {
			in.Kind = strategy.Exit
			break
		}
		typ := domain.OrderMarket
		if limit {
			typ = domain.OrderLimit
		}
		in.Kind, in.Ref = strategy.Place, c.ID
		in.Order = strategy.OrderSpec{Side: domain.Sell, Type: typ, Quantity: qty, Price: c.Price}
	default:
		return "", nil, errors.New("side must be buy or sell")
	}
	return b.ID, []strategy.Intent{in}, nil
}

func (s *State) holding(owner, symbol string) (q decimal.Decimal) {
	for _, h := range s.Holdings {
		if h.Owner == owner && h.Symbol == symbol {
			q = q.Add(h.Quantity)
		}
	}
	return q
}
