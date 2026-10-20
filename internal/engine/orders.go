package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
)

// apply executes strategy intents. Called with e.mu held.
func (e *Engine) apply(ctx context.Context, inst *instance, intents []strategy.Intent) {
	for _, in := range intents {
		if in.Exchange == "" {
			in.Exchange = inst.cfg.Exchange
		}
		if in.Market == "" {
			in.Market = inst.cfg.Market
		}
		key := domain.PositionKey{Exchange: in.Exchange, Market: in.Market, Symbol: in.Symbol}
		switch in.Kind {
		case strategy.Enter:
			e.enter(ctx, inst, key, in)
		case strategy.Exit:
			delete(e.reversals, key)
			reason := in.Reason
			if reason == "" {
				reason = domain.TagExit
			}
			e.exit(ctx, key, reason)
		case strategy.Place:
			e.placeRaw(ctx, inst, key, in)
		case strategy.Cancel:
			if id, ok := e.refs[inst.cfg.ID+"/"+in.Ref]; ok {
				e.cancel(ctx, id)
			}
		case strategy.CancelAll:
			for id, o := range e.orders {
				if o.Strategy == inst.cfg.ID && o.Tag == domain.TagStrategy && o.Exchange == in.Exchange && o.Symbol == in.Symbol && o.Market == in.Market {
					e.cancel(ctx, id)
				}
			}
		}
	}
}

func (e *Engine) enter(ctx context.Context, inst *instance, key domain.PositionKey, in strategy.Intent) {
	log := e.log.With("strategy", inst.cfg.ID, "symbol", key.Symbol, "market", key.Market, "side", in.Side)
	if e.paused {
		log.Info("entry skipped: engine paused", "reason", in.Reason)
		return
	}
	if key.Market == domain.Spot && in.Side == domain.Short {
		log.Warn("entry rejected: no shorts on spot")
		return
	}
	if _, pending := e.entries[key]; pending {
		return
	}
	acct := e.account(key)
	ins, okIns := e.instruments[instKey{key.Exchange, key.Market, key.Symbol}]
	price, okPrice := e.lastPrice[instKey{key.Exchange, key.Market, key.Symbol}]
	if acct == nil || !okIns || !okPrice {
		log.Error("entry rejected: missing account, instrument or price", "account", acct != nil, "instrument", okIns, "price", okPrice)
		return
	}

	snap := acct.Snapshot()
	freeMargin := snap.FreeMargin().Sub(e.pendingMargin(key.Exchange, key.Market))
	if pos, has := acct.Position(key.Symbol); has {
		if pos.Side == in.Side {
			return // already positioned
		}
		// Defer the new entry until executions confirm that the old
		// position is flat. Submission order alone does not imply fill order.
		e.reversals[key] = reversal{inst: inst, intent: in}
		e.exit(ctx, key, "reverse")
		if _, pending := e.exits[key]; !pending {
			delete(e.reversals, key)
		}
		return
	}
	if _, pending := e.exits[key]; pending {
		return
	}

	lev := e.cfg.Risk.Leverage(inst.cfg.Leverage, key.Market)
	qty, err := e.cfg.Risk.Size(risk.SizeRequest{
		Equity: snap.Equity, FreeMargin: freeMargin, Price: price, StopLoss: in.StopLoss,
		Leverage: lev, Instrument: ins, Quantity: in.Quantity,
	})
	if err != nil {
		log.Warn("entry rejected by sizing", "err", err, "reason", in.Reason)
		return
	}
	o := domain.Order{
		ClientID: e.newClientID(inst), Exchange: key.Exchange, Market: key.Market, Symbol: key.Symbol,
		Side: in.Side.EntrySide(), Type: domain.OrderMarket, Quantity: qty,
		Tag: domain.TagEntry, Strategy: inst.cfg.ID, Reason: in.Reason,
	}
	if err := e.cfg.Risk.CheckOrder(o, price, e.cfg.Book, e.pendingRiskOrders()...); err != nil {
		e.notice(slog.LevelWarn, "entry blocked by risk limits", "strategy", inst.cfg.ID, "symbol", key.Symbol, "err", err)
		return
	}
	if err := e.ensureLeverage(ctx, key, lev); err != nil {
		log.Error("entry rejected: set leverage failed", "err", err)
		return
	}

	e.leverage[key] = lev
	e.entries[key] = &entryPlan{orderID: o.ClientID, side: in.Side, stopLoss: in.StopLoss, takeProfit: in.TakeProfit, trailing: in.TrailingStop}
	log.Info("entering", "qty", qty, "price", price, "stop", in.StopLoss, "target", in.TakeProfit, "reason", in.Reason)
	if err := e.place(ctx, o); err != nil {
		delete(e.entries, key)
	}
}

func (e *Engine) ensureLeverage(ctx context.Context, key domain.PositionKey, lev int) error {
	if key.Market != domain.Futures || e.leverageSet[key] == lev {
		return nil
	}
	ls, ok := e.brokers[key.Exchange].(broker.LeverageSetter)
	if !ok {
		return nil
	}
	if err := ls.SetLeverage(ctx, key.Market, key.Symbol, lev); err != nil {
		return err
	}
	e.leverageSet[key] = lev
	return nil
}

// exit closes the position on key with a market order.
func (e *Engine) exit(ctx context.Context, key domain.PositionKey, tag string) {
	if id, ok := e.entries[key]; ok {
		// Entry not filled yet: cancel it instead.
		e.cancel(ctx, id.orderID)
		delete(e.entries, key)
	}
	e.clearProtection(ctx, key)
	if _, pending := e.exits[key]; pending {
		return
	}
	acct := e.account(key)
	if acct == nil {
		return
	}
	pos, has := acct.Position(key.Symbol)
	if !has {
		return
	}
	reason := tag
	if tag != domain.TagExit && tag != domain.TagStopLoss && tag != domain.TagTakeProfit {
		tag = domain.TagExit
	}
	o := domain.Order{
		ClientID: e.newClientID(e.instanceByID(pos.Strategy)), Exchange: key.Exchange, Market: key.Market, Symbol: key.Symbol,
		Side: pos.Side.ExitSide(), Type: domain.OrderMarket, Quantity: pos.Quantity, ReduceOnly: true,
		Tag: tag, Strategy: pos.Strategy, Reason: reason,
	}
	e.exits[key] = o.ClientID
	e.log.Info("exiting", "symbol", key.Symbol, "market", key.Market, "qty", pos.Quantity, "reason", reason)
	if err := e.place(ctx, o); err != nil {
		delete(e.exits, key)
		e.notice(slog.LevelError, "exit order failed, position still open", "symbol", key.Symbol, "err", err)
	}
}

func (e *Engine) placeRaw(ctx context.Context, inst *instance, key domain.PositionKey, in strategy.Intent) {
	if e.paused && !in.Order.ReduceOnly && (key.Market != domain.Spot || in.Order.Side != domain.Sell) {
		return
	}
	if in.Ref != "" {
		if id, ok := e.refs[inst.cfg.ID+"/"+in.Ref]; ok {
			if _, open := e.orders[id]; open {
				return // ref still open; strategies cancel before re-using a ref
			}
		}
	}
	ins, ok := e.instruments[instKey{key.Exchange, key.Market, key.Symbol}]
	if !ok {
		e.log.Error("order rejected: unknown instrument", "symbol", key.Symbol)
		return
	}
	spec := in.Order
	o := domain.Order{
		ClientID: e.newClientID(inst), Exchange: key.Exchange, Market: key.Market, Symbol: key.Symbol,
		Side: spec.Side, Type: spec.Type, Quantity: ins.RoundQty(spec.Quantity),
		Price: ins.RoundPrice(spec.Price), StopPrice: ins.RoundPrice(spec.StopPrice),
		ReduceOnly: spec.ReduceOnly, PostOnly: spec.PostOnly,
		Tag: domain.TagStrategy, Strategy: inst.cfg.ID, Reason: in.Reason,
	}
	ref := o.Price
	if !ref.IsPositive() {
		ref = e.lastPrice[instKey{key.Exchange, key.Market, key.Symbol}]
	}
	if err := ins.CheckOrder(ref, o.Quantity); err != nil {
		e.log.Warn("order rejected: below exchange minimum", "strategy", inst.cfg.ID, "err", err)
		return
	}
	if !o.ReduceOnly {
		if err := e.cfg.Risk.CheckOrder(o, ref, e.cfg.Book, e.pendingRiskOrders()...); err != nil {
			e.notice(slog.LevelWarn, "order blocked by risk limits", "strategy", inst.cfg.ID, "symbol", key.Symbol, "err", err)
			return
		}
	}
	if err := e.ensureLeverage(ctx, key, e.cfg.Risk.Leverage(inst.cfg.Leverage, key.Market)); err != nil {
		e.log.Error("order rejected: set leverage failed", "err", err)
		return
	}
	e.leverage[key] = e.cfg.Risk.Leverage(inst.cfg.Leverage, key.Market)
	if in.Ref != "" {
		e.refs[inst.cfg.ID+"/"+in.Ref] = o.ClientID
	}
	_ = e.place(ctx, o)
}

// place tracks o and sends it to its broker.
func (e *Engine) place(ctx context.Context, o domain.Order) error {
	b, ok := e.brokers[o.Exchange]
	if !ok {
		o.Status, o.Reason = domain.OrderRejected, "no broker for exchange "+o.Exchange
		e.orderChanged(o)
		e.log.Error("order rejected", "client_id", o.ClientID, "reason", o.Reason)
		return errNoBroker
	}
	o.Status = domain.OrderNew
	o.CreatedAt, o.UpdatedAt = e.cfg.Clock(), e.cfg.Clock()
	tracked := o
	e.orders[o.ClientID] = &tracked

	start := time.Now()
	if err := e.beforeBroker(); err != nil {
		delete(e.orders, o.ClientID)
		return err
	}
	accepted, err := b.PlaceOrder(ctx, o)
	if e.cfg.Hooks.OnPlaced != nil {
		e.cfg.Hooks.OnPlaced(o.Exchange, time.Since(start), err)
	}
	if err != nil {
		delete(e.orders, o.ClientID)
		o.Status, o.Reason = domain.OrderRejected, err.Error()
		e.orderChanged(o)
		e.log.Error("order rejected", "client_id", o.ClientID, "symbol", o.Symbol, "type", o.Type, "err", err)
		return err
	}
	if t, ok := e.orders[o.ClientID]; ok && t.Status == domain.OrderNew {
		t.ExchangeID = accepted.ExchangeID
		e.orderChanged(*t)
	}
	return nil
}

func (e *Engine) cancel(ctx context.Context, clientID string) {
	o, ok := e.orders[clientID]
	if !ok {
		return
	}
	b := e.brokers[o.Exchange]
	if b == nil {
		return
	}
	if err := e.beforeBroker(); err != nil {
		e.log.Error("cancel checkpoint failed")
		return
	}
	if err := b.CancelOrder(ctx, *o); err != nil {
		e.log.Warn("cancel failed", "client_id", clientID, "symbol", o.Symbol, "err", err)
	}
}

func (e *Engine) orderChanged(o domain.Order) {
	if e.cfg.Hooks.OnOrder != nil {
		e.cfg.Hooks.OnOrder(o)
	}
}

// handleEvent processes one broker event. Called with e.mu held.
func (e *Engine) handleEvent(ctx context.Context, ev broker.Event) {
	switch ev.Kind {
	case broker.EventFill:
		e.onFill(ctx, ev.Fill, ev.Order)
	case broker.EventOrder:
		e.onOrderUpdate(ctx, ev.Order)
	}
}

func (e *Engine) onOrderUpdate(ctx context.Context, upd domain.Order) {
	o, ok := e.orders[upd.ClientID]
	if !ok {
		return
	}
	o.Status, o.UpdatedAt = upd.Status, upd.UpdatedAt
	if upd.ExchangeID != "" {
		o.ExchangeID = upd.ExchangeID
	}
	if upd.Reason != "" {
		o.Reason = upd.Reason
	}
	e.orderChanged(*o)
	if o.Status.Open() {
		return
	}
	delete(e.orders, o.ClientID)
	delete(e.fillIDs, o.ClientID)

	key := domain.PositionKey{Exchange: o.Exchange, Market: o.Market, Symbol: o.Symbol}
	switch o.Tag {
	case domain.TagEntry:
		if plan := e.entries[key]; plan != nil && plan.orderID == o.ClientID {
			delete(e.entries, key)
			// A partially filled entry that got canceled still needs protection.
			if o.FilledQty.IsPositive() {
				e.protect(ctx, key, plan)
			}
		}
	case domain.TagExit:
		if e.exits[key] == o.ClientID {
			delete(e.exits, key)
			delete(e.reversals, key)
			if o.Status != domain.OrderCanceled {
				e.notice(slog.LevelError, "exit order did not fill", "symbol", o.Symbol, "status", o.Status, "reason", o.Reason)
			}
		}
	case domain.TagStopLoss:
		p := e.protections[key]
		if p == nil || p.stopID != o.ClientID || o.Status == domain.OrderCanceled {
			break
		}
		if p.linked && e.isOpen(p.targetID) {
			// The exchange expires a linked stop when the take-profit
			// triggers, before reporting its fill. The take-profit's own
			// outcome decides whether the position is still protected.
			p.stopGone = true
			e.log.Info("linked stop expired, waiting for the take profit", "symbol", o.Symbol, "status", o.Status)
			break
		}
		// The exchange refused our stop: never leave a position naked.
		e.notice(slog.LevelError, "stop loss rejected, closing position", "symbol", o.Symbol, "status", o.Status, "reason", o.Reason)
		e.exit(ctx, key, domain.TagExit)
	case domain.TagTakeProfit:
		if p := e.protections[key]; p != nil && p.targetID == o.ClientID && p.stopGone {
			e.notice(slog.LevelError, "take profit ended unfilled after its linked stop expired, closing position",
				"symbol", o.Symbol, "status", o.Status, "reason", o.Reason)
			e.exit(ctx, key, domain.TagExit)
		}
	}
}

// isOpen reports whether the engine tracks id as an open order.
func (e *Engine) isOpen(id string) bool {
	o, ok := e.orders[id]
	return ok && o.Status.Open()
}

func (e *Engine) onFill(ctx context.Context, f domain.Fill, upd domain.Order) {
	o, ok := e.orders[f.OrderClientID]
	if !ok {
		e.log.Warn("fill for unknown order", "client_id", f.OrderClientID, "symbol", f.Symbol)
		return
	}
	if !f.Quantity.IsPositive() || !f.Price.IsPositive() {
		e.log.Error("invalid execution ignored", "client_id", f.OrderClientID, "trade_id", f.TradeID)
		return
	}
	if f.TradeID != "" {
		seen := e.fillIDs[f.OrderClientID]
		if seen == nil {
			seen = map[string]bool{}
			e.fillIDs[f.OrderClientID] = seen
		}
		if seen[f.TradeID] {
			return
		}
		seen[f.TradeID] = true
	}
	newQty := o.FilledQty.Add(f.Quantity)
	o.AvgPrice = o.AvgPrice.Mul(o.FilledQty).Add(f.Price.Mul(f.Quantity)).Div(newQty)
	o.FilledQty = newQty
	// The status snapshot can be ahead of executions replayed after a
	// reconnect. Keep tracking until the local execution ledger is full.
	o.Status = domain.OrderPartiallyFilled
	if o.FilledQty.GreaterThanOrEqual(o.Quantity) {
		o.Status = domain.OrderFilled
	}
	o.UpdatedAt = f.Time
	order := *o

	key := domain.PositionKey{Exchange: f.Exchange, Market: f.Market, Symbol: f.Symbol}
	if acct := e.account(key); acct != nil {
		acct.ApplyFill(f, portfolio.FillMeta{Strategy: o.Strategy, Tag: o.Tag, Leverage: e.leverage[key]})
	}
	if e.cfg.Hooks.OnFill != nil {
		e.cfg.Hooks.OnFill(f, order)
	}
	e.orderChanged(order)
	done := order.Status == domain.OrderFilled
	if done {
		delete(e.orders, order.ClientID)
		delete(e.fillIDs, order.ClientID)
		if e.exits[key] == order.ClientID {
			delete(e.exits, key)
		}
	}

	switch order.Tag {
	case domain.TagEntry:
		if plan := e.entries[key]; plan != nil && plan.orderID == order.ClientID {
			if done {
				delete(e.entries, key)
				if e.cfg.Hooks.OnEntry != nil {
					e.cfg.Hooks.OnEntry(order, plan.stopLoss, plan.takeProfit)
				}
			}
			e.protect(ctx, key, plan)
		}
	case domain.TagExit:
		if done && e.exits[key] == order.ClientID {
			delete(e.exits, key)
		}
	case domain.TagStopLoss, domain.TagTakeProfit:
		// A protective execution ends the entry attempt. Otherwise its
		// unfilled remainder could reopen the position after this exit.
		if plan := e.entries[key]; plan != nil {
			delete(e.entries, key)
			e.cancel(ctx, plan.orderID)
		}
		if done {
			e.log.Info("protective order filled", "symbol", key.Symbol, "tag", order.Tag, "price", f.Price)
		}
		if p := e.protections[key]; p != nil {
			if acct := e.account(key); acct != nil {
				if pos, open := acct.Position(key.Symbol); open {
					next := &protection{stopPrice: p.stopPrice, takeProfit: p.takeProfit, trailing: p.trailing, best: p.best}
					e.clearProtection(ctx, key)
					e.placeProtection(ctx, key, pos, next)
				}
			}
		}
	}

	// Position gone: drop the remaining protective orders (the other leg).
	if acct := e.account(key); acct != nil {
		if _, open := acct.Position(key.Symbol); !open {
			e.clearProtection(ctx, key)
			if next, ok := e.reversals[key]; ok && done && order.Tag == domain.TagExit {
				delete(e.reversals, key)
				e.enter(ctx, next.inst, key, next.intent)
			}
		}
	}

	if inst := e.instanceByID(order.Strategy); inst != nil {
		if fh, ok := inst.s.(strategy.FillHandler); ok {
			e.apply(ctx, inst, fh.OnFill(f, order))
		}
	}
}

// protect places the stop loss and take profit of a freshly opened position.
func (e *Engine) protect(ctx context.Context, key domain.PositionKey, plan *entryPlan) {
	acct := e.account(key)
	if acct == nil {
		return
	}
	pos, has := acct.Position(key.Symbol)
	if !has {
		return
	}
	if p := e.protections[key]; p != nil && p.quantity.Equal(pos.Quantity) {
		return
	}
	e.clearProtection(ctx, key)
	stop := plan.stopLoss
	if !stop.IsPositive() && plan.trailing > 0 {
		trail := decimal.NewFromFloat(plan.trailing)
		stop = pos.EntryPrice.Sub(pos.EntryPrice.Mul(trail).Mul(pos.Side.Sign()))
	}
	p := &protection{stopPrice: stop, takeProfit: plan.takeProfit, trailing: plan.trailing, best: pos.EntryPrice}
	e.placeProtection(ctx, key, pos, p)
}

// placeProtection places the protective orders described by p for pos.
func (e *Engine) placeProtection(ctx context.Context, key domain.PositionKey, pos domain.Position, p *protection) {
	p.quantity = pos.Quantity
	ins := e.instruments[instKey{key.Exchange, key.Market, key.Symbol}]
	price := e.lastPrice[instKey{key.Exchange, key.Market, key.Symbol}]
	if !price.IsPositive() {
		price = pos.EntryPrice
	}
	long := pos.Side == domain.Long

	var stop, target domain.Order
	if p.stopPrice.IsPositive() {
		sp := ins.RoundPrice(p.stopPrice)
		// A stop already beyond the market would trigger immediately (and
		// is rejected by exchanges): exit now instead.
		if (long && sp.GreaterThanOrEqual(price)) || (!long && sp.LessThanOrEqual(price)) {
			e.notice(slog.LevelWarn, "stop already crossed, exiting at market", "symbol", key.Symbol, "stop", sp, "price", price)
			e.exit(ctx, key, domain.TagStopLoss)
			return
		}
		stop = e.protectiveOrder(pos, domain.OrderStopMarket, sp, domain.TagStopLoss)
		p.stopID = stop.ClientID
	}
	if p.takeProfit.IsPositive() {
		tp := ins.RoundPrice(p.takeProfit)
		if (long && tp.GreaterThan(price)) || (!long && tp.LessThan(price)) {
			target = e.protectiveOrder(pos, domain.OrderTakeProfit, tp, domain.TagTakeProfit)
			p.targetID = target.ClientID
		}
	}
	if p.stopID == "" && p.targetID == "" {
		return
	}
	e.protections[key] = p

	if bp, ok := e.brokers[key.Exchange].(broker.BracketPlacer); ok {
		if lb, ok := bp.(broker.LinkedBrackets); ok && stop.ClientID != "" && target.ClientID != "" {
			p.linked = lb.LinkedBracket(key.Market)
		}
		for _, o := range []domain.Order{stop, target} {
			if o.ClientID != "" {
				t := o
				t.Status, t.CreatedAt, t.UpdatedAt = domain.OrderNew, e.cfg.Clock(), e.cfg.Clock()
				e.orders[o.ClientID] = &t
			}
		}
		if err := e.beforeBroker(); err != nil {
			e.paused = true
			e.log.Error("bracket checkpoint failed")
			return
		}
		placed, err := bp.PlaceBracket(ctx, stop, target)
		accepted := make(map[string]bool, len(placed))
		for _, o := range placed {
			accepted[o.ClientID] = true
			if tracked := e.orders[o.ClientID]; tracked != nil {
				tracked.ExchangeID = o.ExchangeID
				e.orderChanged(*tracked)
			}
		}
		if err != nil {
			delete(e.protections, key)
			// Some brokers place the legs sequentially. An accepted first
			// leg still exists at the exchange if the second one fails.
			for _, o := range []domain.Order{stop, target} {
				if accepted[o.ClientID] {
					e.cancel(ctx, o.ClientID)
				} else if tracked := e.orders[o.ClientID]; tracked != nil {
					tracked.Status, tracked.Reason = domain.OrderRejected, err.Error()
					e.orderChanged(*tracked)
					delete(e.orders, o.ClientID)
				}
			}
			e.notice(slog.LevelError, "protective orders failed, closing position", "symbol", key.Symbol, "err", err)
			e.exit(ctx, key, domain.TagExit)
		}
		return
	}
	if stop.ClientID != "" {
		if err := e.place(ctx, stop); err != nil {
			delete(e.protections, key)
			e.notice(slog.LevelError, "stop loss failed, closing position", "symbol", key.Symbol, "err", err)
			e.exit(ctx, key, domain.TagExit)
			return
		}
	}
	if target.ClientID != "" {
		if err := e.place(ctx, target); err != nil {
			p.targetID = ""
			e.log.Warn("take profit failed; stop loss stays", "symbol", key.Symbol, "err", err)
		}
	}
}

func (e *Engine) protectiveOrder(pos domain.Position, t domain.OrderType, price decimal.Decimal, tag string) domain.Order {
	return domain.Order{
		ClientID: e.newClientID(e.instanceByID(pos.Strategy)), Exchange: pos.Exchange, Market: pos.Market, Symbol: pos.Symbol,
		Side: pos.Side.ExitSide(), Type: t, Quantity: pos.Quantity, StopPrice: price, ReduceOnly: true,
		Tag: tag, Strategy: pos.Strategy,
	}
}

// clearProtection cancels the protective orders of key.
func (e *Engine) clearProtection(ctx context.Context, key domain.PositionKey) {
	p := e.protections[key]
	if p == nil {
		return
	}
	delete(e.protections, key)
	ids := []string{p.stopID, p.targetID}
	if p.linked && p.targetID != "" && e.isOpen(p.stopID) {
		// Cancelling one leg cancels the whole list at the exchange; a
		// second request would only fail with "unknown order".
		ids = []string{p.stopID}
	}
	for _, id := range ids {
		if id != "" {
			e.cancel(ctx, id)
		}
	}
}

// trail moves trailing stops in the position's favour on candle close.
func (e *Engine) trail(ctx context.Context, c domain.Candle) {
	key := domain.PositionKey{Exchange: c.Exchange, Market: c.Market, Symbol: c.Symbol}
	p := e.protections[key]
	if p == nil || p.trailing <= 0 {
		return
	}
	acct := e.account(key)
	pos, has := acct.Position(c.Symbol)
	if !has {
		return
	}
	ins := e.instruments[instKey{key.Exchange, key.Market, key.Symbol}]
	trail := decimal.NewFromFloat(p.trailing)
	var newStop decimal.Decimal
	if pos.Side == domain.Long {
		p.best = decimal.Max(p.best, c.High)
		newStop = ins.RoundPrice(p.best.Sub(p.best.Mul(trail)))
		if !newStop.GreaterThan(p.stopPrice.Add(ins.TickSize)) {
			return
		}
	} else {
		p.best = decimal.Min(p.best, c.Low)
		newStop = ins.RoundPrice(p.best.Add(p.best.Mul(trail)))
		if !newStop.Add(ins.TickSize).LessThan(p.stopPrice) {
			return
		}
	}
	e.log.Debug("trailing stop moved", "symbol", c.Symbol, "from", p.stopPrice, "to", newStop)
	// Replace both legs: bracket brokers cannot amend one leg alone.
	next := &protection{stopPrice: newStop, takeProfit: p.takeProfit, trailing: p.trailing, best: p.best}
	e.clearProtection(ctx, key)
	e.placeProtection(ctx, key, pos, next)
}

// stratCtx implements strategy.Context. Its methods run inside engine
// callbacks (lock held) and must not lock e.mu.
type stratCtx struct {
	e    *Engine
	inst *instance
}

func (c *stratCtx) Now() time.Time { return c.e.cfg.Clock() }
func (c *stratCtx) Log() *slog.Logger {
	return c.e.log.With("strategy", c.inst.cfg.ID)
}

func (c *stratCtx) Position(exchange string, m domain.Market, symbol string) (domain.Position, bool) {
	acct := c.e.cfg.Book.Account(portfolio.AccountKey{Exchange: exchange, Market: m})
	if acct == nil {
		return domain.Position{}, false
	}
	return acct.Position(symbol)
}

func (c *stratCtx) Account(exchange string, m domain.Market) portfolio.Snapshot {
	acct := c.e.cfg.Book.Account(portfolio.AccountKey{Exchange: exchange, Market: m})
	if acct == nil {
		return portfolio.Snapshot{}
	}
	return acct.Snapshot()
}

func (c *stratCtx) Instrument(exchange string, m domain.Market, symbol string) (domain.Instrument, bool) {
	in, ok := c.e.instruments[instKey{exchange, m, symbol}]
	return in, ok
}

func (c *stratCtx) LastPrice(exchange string, m domain.Market, symbol string) (decimal.Decimal, bool) {
	p, ok := c.e.lastPrice[instKey{exchange, m, symbol}]
	return p, ok
}

func (c *stratCtx) Funding(exchange, symbol string) (strategy.FundingInfo, bool) {
	f, ok := c.e.funding[instKey{exchange, domain.Futures, symbol}]
	return f, ok
}

func (c *stratCtx) OpenOrders(symbol string) []domain.Order {
	var out []domain.Order
	for _, o := range c.e.orders {
		if o.Strategy == c.inst.cfg.ID && o.Symbol == symbol && o.Tag == domain.TagStrategy {
			out = append(out, *o)
		}
	}
	return out
}
