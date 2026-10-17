package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
)

// start starts the live brokers and loads the exchange state (after
// warmup, so last prices are known), then arms the kill switch baseline.
func (s *session) start(ctx context.Context) error {
	defer s.risk.Observe(time.Now(), s.book.Total().Equity)
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
		s.log.Info("live broker started", "exchange", name)
	}
	s.reconcile(ctx, true)
	for _, p := range s.eng.Unprotected() {
		s.onNotice(slog.LevelError, "open position without stop loss on the exchange", "symbol", p.Symbol, "market", p.Market, "side", p.Side, "qty", p.Quantity)
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

		if !hasX {
			acct.SetPosition(domain.Position{PositionKey: key})
			s.onNotice(slog.LevelWarn, "position closed outside the bot (manual, liquidation or missed fill)",
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
		s.onNotice(level, msg, "symbol", sym, "side", p.Side, "qty", p.Quantity, "entry", p.EntryPrice)
	}
}

// reconcileSpot derives positions from base asset holdings of the traded
// symbols. The ledger balance is cash plus the cost of those holdings, so
// that equity = balance + unrealized PnL holds as on futures.
func (s *session) reconcileSpot(acct *portfolio.Account, st *broker.AccountState, quote string, initial bool) {
	var positions []domain.Position
	cost := decimal.Zero
	for key, in := range s.instrument {
		if key.Exchange != acct.Key.Exchange || key.Market != acct.Key.Market {
			continue
		}
		qty := in.RoundQty(st.Balances[in.Base].Wallet)
		ours, hasO := acct.Position(key.Symbol)
		price := ours.EntryPrice
		if !hasO {
			price, _ = s.eng.LastPrice(key.Exchange, key.Market, key.Symbol)
		}
		if !qty.IsPositive() || in.CheckOrder(price, qty) != nil {
			continue // dust
		}
		p := domain.Position{PositionKey: key, Side: domain.Long, Quantity: qty, EntryPrice: price, Leverage: 1}
		if hasO {
			p.Strategy, p.OpenedAt, p.Fees = ours.Strategy, ours.OpenedAt, ours.Fees
		}
		positions = append(positions, p)
		cost = cost.Add(qty.Mul(price))
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
	lb.Track(adopt)
	s.eng.AdoptOrders(adopt)
	s.log.Info("adopted open orders from exchange", "account", key, "count", len(adopt))
}
