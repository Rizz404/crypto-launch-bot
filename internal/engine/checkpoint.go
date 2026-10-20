package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/shopspring/decimal"
	"time"
)

// Checkpoint contains execution state, not a second financial ledger. Indicators
// are rebuilt from historical bars before restoring this state.
type Checkpoint struct {
	Accounts    []AccountState
	Version     int
	Sequence    int64
	Paused      bool
	Orders      []domain.Order
	Fills       map[string]map[string]bool
	Refs        map[string]string
	Entries     []EntryState
	Protections []ProtectionState
	Exits       []ExitState
	Timers      map[string]time.Time
	Strategies  map[string]json.RawMessage
}
type AccountState struct {
	Key       portfolio.AccountKey
	Balance   decimal.Decimal
	Positions []domain.Position
}
type EntryState struct {
	Key          domain.PositionKey
	OrderID      string
	Side         domain.PositionSide
	Stop, Target decimal.Decimal
	Trailing     float64
}
type ProtectionState struct {
	Key                          domain.PositionKey
	StopID, TargetID             string
	Stop, Target, Best, Quantity decimal.Decimal
	Trailing                     float64
	Linked, StopGone             bool
}
type ExitState struct {
	Key     domain.PositionKey
	OrderID string
}
type StatefulStrategy interface {
	SaveState() ([]byte, error)
	LoadState([]byte) error
}

func (e *Engine) checkpointLocked() ([]byte, error) {
	s := Checkpoint{Version: 1, Sequence: e.seq, Paused: e.paused, Fills: e.fillIDs, Refs: e.refs, Timers: map[string]time.Time{}, Strategies: map[string]json.RawMessage{}}
	for _, a := range e.cfg.Book.Accounts() {
		s.Accounts = append(s.Accounts, AccountState{a.Key, a.Snapshot().Balance, a.Positions()})
	}
	for _, o := range e.orders {
		s.Orders = append(s.Orders, *o)
	}
	for k, p := range e.entries {
		s.Entries = append(s.Entries, EntryState{k, p.orderID, p.side, p.stopLoss, p.takeProfit, p.trailing})
	}
	for k, p := range e.protections {
		s.Protections = append(s.Protections, ProtectionState{k, p.stopID, p.targetID, p.stopPrice, p.takeProfit, p.best, p.quantity, p.trailing, p.linked, p.stopGone})
	}
	for k, id := range e.exits {
		s.Exits = append(s.Exits, ExitState{k, id})
	}
	for _, i := range e.instances {
		s.Timers[i.cfg.ID] = i.nextTimer
		if st, ok := i.s.(StatefulStrategy); ok {
			raw, err := st.SaveState()
			if err != nil {
				return nil, err
			}
			s.Strategies[i.cfg.ID] = raw
		}
	}
	return json.Marshal(s)
}
func (e *Engine) Checkpoint() ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.checkpointLocked()
}
func (e *Engine) beforeBroker() error {
	if e.cfg.Hooks.BeforeBroker == nil {
		return nil
	}
	raw, err := e.checkpointLocked()
	if err != nil {
		return err
	}
	return e.cfg.Hooks.BeforeBroker(raw)
}
func (e *Engine) Restore(raw []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var s Checkpoint
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	if s.Version != 1 {
		return fmt.Errorf("unsupported engine checkpoint")
	}
	for _, a := range s.Accounts {
		if acct := e.cfg.Book.Account(a.Key); acct != nil {
			for _, p := range acct.Positions() {
				p.Quantity = decimal.Zero
				acct.SetPosition(p)
			}
			acct.SetBalance(a.Balance)
			for _, p := range a.Positions {
				acct.SetPosition(p)
			}
		}
	}
	e.seq = max(e.seq, s.Sequence)
	e.paused = s.Paused
	clear(e.orders)
	for _, o := range s.Orders {
		cp := o
		e.orders[o.ClientID] = &cp
	}
	if s.Fills != nil {
		e.fillIDs = s.Fills
	}
	if s.Refs != nil {
		e.refs = s.Refs
	}
	clear(e.entries)
	for _, p := range s.Entries {
		e.entries[p.Key] = &entryPlan{orderID: p.OrderID, side: p.Side, stopLoss: p.Stop, takeProfit: p.Target, trailing: p.Trailing}
	}
	clear(e.protections)
	for _, p := range s.Protections {
		e.protections[p.Key] = &protection{stopID: p.StopID, targetID: p.TargetID, stopPrice: p.Stop, takeProfit: p.Target, trailing: p.Trailing, best: p.Best, quantity: p.Quantity, linked: p.Linked, stopGone: p.StopGone}
	}
	clear(e.exits)
	for _, p := range s.Exits {
		e.exits[p.Key] = p.OrderID
	}
	for _, i := range e.instances {
		i.nextTimer = s.Timers[i.cfg.ID]
		if raw, ok := s.Strategies[i.cfg.ID]; ok {
			if st, ok := i.s.(StatefulStrategy); ok {
				if err := st.LoadState(raw); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// CancelPending requests cancellation without flattening owned positions.
func (e *Engine) CancelPending(ctx context.Context) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)
	for id, o := range e.orders {
		if o.Tag == domain.TagEntry || o.Tag == domain.TagStrategy {
			e.cancel(ctx, id)
		}
	}
}

// Warmup rebuilds indicators without submitting orders or evaluating risk.
// Restore the saved operational checkpoint after replaying history.
func (e *Engine) Warmup(c domain.Candle) {
	e.mu.Lock()
	defer e.mu.Unlock()
	k := instKey{c.Exchange, c.Market, c.Symbol}
	e.lastPrice[k] = c.Close
	for _, i := range e.instances {
		if i.routes[k] && i.tfs[c.Timeframe] {
			_ = i.s.OnCandle(c)
		}
	}
}

// CancelAllOrders requests terminal cancellation of all owned orders; callers
// must explicitly choose to retain any positions left without protection.
func (e *Engine) CancelAllOrders(ctx context.Context) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)
	for id := range e.orders {
		e.cancel(ctx, id)
	}
}

// SetPrice restores a current mark without running strategies or matching orders.
func (e *Engine) SetPrice(exchange string, market domain.Market, symbol string, price decimal.Decimal) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastPrice[instKey{exchange, market, symbol}] = price
	if a := e.cfg.Book.Account(portfolio.AccountKey{Exchange: exchange, Market: market}); a != nil {
		a.Mark(symbol, price)
	}
}

// SetRiskConfig changes sizing limits; managed parent loss limits remain durable.
func (e *Engine) SetRiskConfig(c risk.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.Risk = risk.NewManager(c, e.cfg.Log)
}
