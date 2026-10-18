package main

import (
	"context"
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
)

// start starts the live brokers and loads the exchange state (after
// warmup, so last prices are known), arms the kill switch baseline and then
// runs the strategies on the candles warmup held back.
func (s *session) start(ctx context.Context) error {
	if err := s.startLive(ctx); err != nil {
		return err
	}
	for _, c := range s.fresh {
		s.eng.OnCandle(ctx, c)
	}
	s.fresh = nil
	return nil
}

func (s *session) startLive(ctx context.Context) error {
	// The engine holds kill switch observations until the ledger has been
	// reconciled: before that, equity is 0 or a restored position alone.
	defer func() { s.eng.ArmRisk(ctx) }()
	if len(s.lives) == 0 {
		return nil
	}
	ctx, s.liveCancel = context.WithCancel(ctx)
	for name, lb := range s.lives {
		if err := lb.Start(ctx, &s.liveWG); err != nil {
			return err
		}
		ch := lb.Resync()
		s.liveWG.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-ch:
					select {
					case s.resync <- struct{}{}:
					default:
					}
				}
			}
		})
		if wo, ok := lb.(interface {
			SetWeightObserver(func(domain.Market, int))
		}); ok {
			wo.SetWeightObserver(func(m domain.Market, used int) {
				s.metrics.UsedWeight.WithLabelValues(name, string(m)).Set(float64(used))
			})
		}
		s.log.Info("live broker started", "exchange", name)
	}
	s.reconcile(ctx, true)
	for _, p := range s.eng.Unprotected() {
		s.notice(slog.LevelError, "open position without stop loss on the exchange", "symbol", p.Symbol, "market", p.Market, "side", p.Side, "qty", p.Quantity)
	}
	return nil
}

// reconcile aligns the ledger with the exchange: balances, positions and
// orders. The exchange is the source of truth. Outside of startup a
// position is only corrected when the difference persists across two
// reconciliations, so fills still in flight on the user stream are not
// counted twice.
func (s *session) reconcile(ctx context.Context, initial bool) {
	if len(s.lives) == 0 {
		return
	}
	for _, acct := range s.book.Accounts() {
		lb := s.lives[acct.Key.Exchange]
		if lb == nil {
			continue
		}
		st, err := lb.AccountState(ctx, acct.Key.Market)
		if err != nil {
			s.log.Error("reconcile: account state failed", "account", acct.Key, "err", err)
			continue
		}
		quote := s.quotes[acct.Key]
		if acct.Key.Market == domain.Futures {
			acct.SetBalance(st.Balances[quote].Wallet)
			s.reconcilePositions(acct, st.Positions, initial)
		} else {
			s.reconcileSpot(acct, st, quote, initial)
		}
		s.reconcileOrders(ctx, lb, acct.Key, st.OpenOrders, initial)
		s.metrics.Reconciles.WithLabelValues("ok").Inc()
	}
	s.eng.ObserveRisk(ctx)
}

func (s *session) reconcilePositions(acct *portfolio.Account, exchange []domain.Position, initial bool) {
	ex := map[string]domain.Position{}
	for _, p := range exchange {
		ex[p.Symbol] = p
	}
	ours := map[string]domain.Position{}
	for _, p := range acct.Positions() {
		ours[p.Symbol] = p
	}
	symbols := map[string]bool{}
	for sym := range ex {
		symbols[sym] = true
	}
	for sym := range ours {
		symbols[sym] = true
	}

	for sym := range symbols {
		key := domain.PositionKey{Exchange: acct.Key.Exchange, Market: acct.Key.Market, Symbol: sym}
		x, hasX := ex[sym]
		o, hasO := ours[sym]
		if hasX == hasO && (!hasX || (x.Side == o.Side && x.Quantity.Equal(o.Quantity))) {
			delete(s.mismatch, key)
			continue
		}
		s.mismatch[key]++
		if !initial && s.mismatch[key] < 2 {
			continue
		}
		delete(s.mismatch, key)
		s.metrics.Reconciles.WithLabelValues("position_corrected").Inc()

		if !hasX {
			acct.SetPosition(domain.Position{PositionKey: key})
			s.notice(slog.LevelWarn, "position closed outside the bot (manual, liquidation or missed fill)",
				"symbol", sym, "side", o.Side, "qty", o.Quantity)
			continue
		}
		p := x
		p.Leverage = max(s.leverages[key], 1)
		if hasO && o.Side == x.Side {
			p.Strategy, p.OpenedAt, p.Fees, p.Funding = o.Strategy, o.OpenedAt, o.Fees, o.Funding
		}
		acct.SetPosition(p)
		level, msg := slog.LevelWarn, "position corrected from exchange"
		if initial && !hasO {
			level, msg = slog.LevelInfo, "position loaded from exchange"
		}
		s.notice(level, msg, "symbol", sym, "side", p.Side, "qty", p.Quantity, "entry", p.EntryPrice)
	}
}

// reconcileSpot checks the bot's spot positions against the base asset
// holdings. A spot wallet also holds coins the bot never bought (decision
// of the user, 2026-10-17): those are left alone, so a position is the
// quantity the bot's own fills built, capped by what the wallet still
// holds (coins sold outside the bot, or fees paid in the base asset). The
// ledger balance is cash plus the cost of the bot's holdings, so that
// equity = balance + unrealized PnL holds as on futures.
func (s *session) reconcileSpot(acct *portfolio.Account, st *broker.AccountState, quote string, initial bool) {
	var positions []domain.Position
	cost := decimal.Zero
	for key, in := range s.instrument {
		if key.Exchange != acct.Key.Exchange || key.Market != acct.Key.Market {
			continue
		}
		held := in.RoundQty(st.Balances[in.Base].Wallet)
		ours, hasO := acct.Position(key.Symbol)
		if !hasO {
			if initial && held.IsPositive() {
				s.log.Info("spot holdings not bought by the bot are left alone", "symbol", key.Symbol, "asset", in.Base, "qty", held)
			}
			continue
		}
		qty := decimal.Min(ours.Quantity, held)
		if !qty.IsPositive() || in.CheckOrder(ours.EntryPrice, qty) != nil {
			continue // sold outside the bot, or only dust left
		}
		p := ours
		p.Quantity = qty
		positions = append(positions, p)
		cost = cost.Add(qty.Mul(p.EntryPrice))
	}
	s.reconcilePositions(acct, positions, initial)
	acct.SetBalance(st.Balances[quote].Wallet.Add(cost))
}

func (s *session) reconcileOrders(ctx context.Context, lb broker.Live, key portfolio.AccountKey, exchangeOpen []domain.Order, initial bool) {
	var tracked []domain.Order
	known := map[string]bool{}
	for _, o := range s.eng.OpenOrders() {
		if o.Exchange == key.Exchange && o.Market == key.Market {
			tracked = append(tracked, o)
			known[o.ClientID] = true
		}
	}
	lb.SyncOrders(ctx, tracked, exchangeOpen)

	var adopt []domain.Order
	var ids []string
	for _, o := range exchangeOpen {
		if known[o.ClientID] {
			continue
		}
		if !strings.HasPrefix(o.ClientID, "tb") {
			if initial {
				s.log.Warn("open order not placed by the bot is ignored", "symbol", o.Symbol, "client_id", o.ClientID, "type", o.Type)
			}
			continue
		}
		adopt = append(adopt, o)
		ids = append(ids, o.ClientID)
	}
	if len(adopt) == 0 {
		return
	}
	metas, err := s.store.OrderMetas(ctx, ids)
	if err != nil {
		s.log.Error("reconcile: order metadata", "err", err)
	}
	for i := range adopt {
		if m, ok := metas[adopt[i].ClientID]; ok {
			adopt[i].Tag, adopt[i].Strategy = m.Tag, m.Strategy
			continue
		}
		switch adopt[i].Type {
		case domain.OrderStopMarket:
			adopt[i].Tag = domain.TagStopLoss
		case domain.OrderTakeProfit:
			adopt[i].Tag = domain.TagTakeProfit
		default:
			adopt[i].Tag = domain.TagStrategy
		}
	}
	adopt = s.cancelOrphanedProtection(ctx, lb, key, adopt)
	if len(adopt) == 0 {
		return
	}
	lb.Track(adopt)
	s.eng.AdoptOrders(adopt)
	s.log.Info("adopted open orders from exchange", "account", key, "count", len(adopt))
}

// cancelOrphanedProtection cancels stop-loss and take-profit orders left
// from an earlier session whose position the bot no longer holds (closed
// while it was down, or by a kill switch before they were adopted). On spot
// they would otherwise sell coins the bot does not own. The rest is returned.
func (s *session) cancelOrphanedProtection(ctx context.Context, lb broker.Live, key portfolio.AccountKey, orders []domain.Order) []domain.Order {
	acct := s.book.Account(key)
	keep := orders[:0]
	canceled := map[string]bool{}
	for _, o := range orders {
		protective := o.Tag == domain.TagStopLoss || o.Tag == domain.TagTakeProfit
		if _, open := acct.Position(o.Symbol); !protective || open {
			keep = append(keep, o)
			continue
		}
		s.notice(slog.LevelWarn, "protective order without a position canceled", "symbol", o.Symbol, "client_id", o.ClientID, "tag", o.Tag)
		lb.Track([]domain.Order{o})
		// The second leg of an OCO is gone once the first is canceled.
		if err := lb.CancelOrder(ctx, o); err != nil && !canceled[o.Symbol] {
			s.log.Warn("cancel orphaned protective order", "client_id", o.ClientID, "err", err)
		}
		canceled[o.Symbol] = true
	}
	return keep
}
