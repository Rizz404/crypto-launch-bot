package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
)

type harness struct {
	e   *Engine
	s   *broker.Sim
	a   *portfolio.Account
	i   *instance
	now time.Time
}

func setup(t *testing.T) *harness {
	t.Helper()
	h := &harness{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	h.a = portfolio.NewAccount(portfolio.AccountKey{Exchange: "sim", Market: domain.Futures}, decimal.NewFromInt(1000), nil)
	book := portfolio.NewBook()
	book.Add(h.a)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h.e = New(Config{Book: book, Risk: risk.NewManager(risk.DefaultConfig(), log), Log: log, Clock: func() time.Time { return h.now }})
	h.s = broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: func() time.Time { return h.now }})
	h.e.AddBroker(h.s)
	h.e.SetInstruments([]domain.Instrument{{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT",
		TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.01"), MinNotional: decimal.NewFromInt(5)}})
	h.i = &instance{cfg: strategy.Config{ID: "test", Exchange: "sim", Market: domain.Futures, Leverage: 1}, idPrefix: shortHash("test")}
	h.e.lastPrice[instKey{"sim", domain.Futures, "BTCUSDT"}] = decimal.NewFromInt(100)
	return h
}

func (h *harness) intent(in strategy.Intent) {
	in.Symbol = "BTCUSDT"
	h.e.mu.Lock()
	h.e.apply(context.Background(), h.i, []strategy.Intent{in})
	h.e.unlockAndDrain(context.Background())
}

func (h *harness) bar(open, high, low, close int64) domain.Candle {
	h.now = h.now.Add(time.Minute)
	d := decimal.NewFromInt
	return domain.Candle{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT", Timeframe: "1m", OpenTime: h.now,
		Open: d(open), High: d(high), Low: d(low), Close: d(close), Closed: true}
}

func TestEntryBracketStopWinsAndOtherLegCanceled(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
	require.Len(t, h.e.OpenOrders(), 1)
	h.s.OnCandle(h.bar(100, 101, 99, 100))
	require.Len(t, h.e.OpenOrders(), 2)
	assert.Empty(t, h.e.Unprotected())
	h.s.OnCandle(h.bar(100, 110, 90, 100))
	assert.Empty(t, h.a.Positions())
	assert.Empty(t, h.e.OpenOrders())
	assert.Equal(t, "990", h.a.Snapshot().Balance.String(), "two units stopped at 95; TP must not reopen a short")
}

func TestPauseCloseAllAndKill(t *testing.T) {
	h := setup(t)
	in := strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(95)}
	h.e.SetPaused(true)
	h.intent(in)
	assert.Empty(t, h.e.OpenOrders())
	h.e.SetPaused(false)
	h.intent(in)
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	h.e.Kill(context.Background(), "test")
	assert.True(t, h.e.Paused())
	orders := h.e.OpenOrders()
	require.Len(t, orders, 1)
	assert.True(t, orders[0].ReduceOnly)
	assert.Equal(t, domain.TagExit, orders[0].Tag)
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	assert.Empty(t, h.a.Positions())
	assert.Empty(t, h.e.OpenOrders())
	h.intent(in)
	assert.Empty(t, h.e.OpenOrders())
	h.e.ResetKillSwitch()
	assert.False(t, h.e.Paused())
	h.intent(in)
	require.Len(t, h.e.OpenOrders(), 1)
	h.e.CloseAll(context.Background(), "shutdown")
	assert.Empty(t, h.e.OpenOrders(), "unfilled entry must be canceled without deadlock")
}

func TestTrailingStopOnlyMovesInFavorableDirection(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, TrailingStop: 0.1})
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	require.Len(t, h.e.OpenOrders(), 1)
	assert.Equal(t, "90", h.e.OpenOrders()[0].StopPrice.String())
	h.e.OnCandle(context.Background(), h.bar(105, 110, 104, 109))
	require.Len(t, h.e.OpenOrders(), 1)
	assert.Equal(t, "99", h.e.OpenOrders()[0].StopPrice.String())
	h.e.OnCandle(context.Background(), h.bar(105, 108, 104, 105))
	require.Len(t, h.e.OpenOrders(), 1)
	assert.Equal(t, "99", h.e.OpenOrders()[0].StopPrice.String())
}

type rejectStop struct{ *broker.Sim }

func (r rejectStop) PlaceOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	if o.Type == domain.OrderStopMarket {
		return o, errors.New("stop rejected")
	}
	return r.Sim.PlaceOrder(ctx, o)
}

func TestRejectedOrCrossedStopClosesPosition(t *testing.T) {
	for _, crossed := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "crossed"}[crossed], func(t *testing.T) {
			h := setup(t)
			stop := int64(95)
			if crossed {
				stop = 101
			} else {
				h.e.AddBroker(rejectStop{h.s})
			}
			h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(stop)})
			h.s.OnCandle(h.bar(100, 100, 100, 100))
			orders := h.e.OpenOrders()
			require.Len(t, orders, 1)
			assert.True(t, orders[0].ReduceOnly)
			h.s.OnCandle(h.bar(100, 100, 100, 100))
			assert.Empty(t, h.a.Positions())
		})
	}
}

func TestAdoptOrdersRestoresProtection(t *testing.T) {
	h := setup(t)
	var stored []domain.Order
	h.e.cfg.Hooks.OnOrder = func(o domain.Order) { stored = append(stored, o) }
	p := domain.Position{PositionKey: domain.PositionKey{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"},
		Side: domain.Long, Quantity: decimal.NewFromInt(1), EntryPrice: decimal.NewFromInt(100)}
	h.a.SetPosition(p)
	require.Len(t, h.e.Unprotected(), 1)
	h.e.AdoptOrders([]domain.Order{{ClientID: "oldstop", Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT",
		Status: domain.OrderNew, Type: domain.OrderStopMarket, StopPrice: decimal.NewFromInt(95), Tag: domain.TagStopLoss}})
	assert.Empty(t, h.e.Unprotected())
	require.Len(t, stored, 1, "adopted orders must be persisted in the active session for the next restart")
}

type bracketBroker struct {
	*broker.Sim
	failTarget bool
}

func (b bracketBroker) PlaceBracket(ctx context.Context, stop, target domain.Order) ([]domain.Order, error) {
	var out []domain.Order
	for _, o := range []domain.Order{stop, target} {
		if o.ClientID == "" {
			continue
		}
		if b.failTarget && o.Type == domain.OrderTakeProfit {
			return out, errors.New("target rejected")
		}
		placed, err := b.PlaceOrder(ctx, o)
		if err != nil {
			return out, err
		}
		out = append(out, placed)
	}
	return out, nil
}

func TestBracketOrdersPersistBeforeFills(t *testing.T) {
	h := setup(t)
	h.e.AddBroker(bracketBroker{Sim: h.s})
	stored := map[string]domain.Order{}
	h.e.cfg.Hooks.OnOrder = func(o domain.Order) { stored[o.ClientID] = o }
	h.e.cfg.Hooks.OnFill = func(f domain.Fill, _ domain.Order) {
		assert.Contains(t, stored, f.OrderClientID, "fill must have a stored parent order")
	}
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	for _, o := range h.e.OpenOrders() {
		assert.NotEmpty(t, o.ExchangeID, "accepted bracket exchange id must be tracked")
		assert.Contains(t, stored, o.ClientID, "bracket acceptance must call the recorder hook")
	}
	h.s.OnCandle(h.bar(100, 110, 99, 105))
}

// linkedBracketBroker behaves like an OCO: cancelling one leg of a bracket
// cancels the other at the "exchange" too.
type linkedBracketBroker struct {
	bracketBroker
	pairs   map[string]string
	cancels int
}

func newLinkedBroker(s *broker.Sim) *linkedBracketBroker {
	return &linkedBracketBroker{bracketBroker: bracketBroker{Sim: s}, pairs: map[string]string{}}
}

func (*linkedBracketBroker) LinkedBracket(domain.Market) bool { return true }

func (b *linkedBracketBroker) PlaceBracket(ctx context.Context, stop, target domain.Order) ([]domain.Order, error) {
	b.pairs[stop.ClientID], b.pairs[target.ClientID] = target.ClientID, stop.ClientID
	return b.bracketBroker.PlaceBracket(ctx, stop, target)
}

func (b *linkedBracketBroker) CancelOrder(ctx context.Context, o domain.Order) error {
	b.cancels++
	if err := b.Sim.CancelOrder(ctx, o); err != nil {
		return err
	}
	if other, ok := b.pairs[o.ClientID]; ok {
		delete(b.pairs, o.ClientID)
		delete(b.pairs, other)
		return b.Sim.CancelOrder(ctx, domain.Order{ClientID: other})
	}
	return nil
}

// Exiting a position with a linked bracket cancels the list once and keeps
// the strategy's reason on the exit order.
func TestExitCancelsLinkedBracketOnceWithStrategyReason(t *testing.T) {
	h := setup(t)
	lb := newLinkedBroker(h.s)
	h.e.AddBroker(lb)
	var exitReason string
	h.e.cfg.Hooks.OnOrder = func(o domain.Order) {
		if o.Tag == domain.TagExit {
			exitReason = o.Reason
		}
	}
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	require.Len(t, h.e.OpenOrders(), 2)
	h.intent(strategy.Intent{Kind: strategy.Exit, Reason: "opposite ema cross"})
	assert.Equal(t, 1, lb.cancels, "one cancel removes the whole OCO list")
	assert.Equal(t, "opposite ema cross", exitReason)
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	assert.Empty(t, h.a.Positions())
	assert.Empty(t, h.e.OpenOrders(), "the other leg's cancel event clears it")
}

// The Binance spot OCO sends "stop EXPIRED" before the take-profit fill
// (recorded on the testnet). That expiry must not trigger a second exit.
func TestLinkedStopExpiryWaitsForTakeProfit(t *testing.T) {
	for _, tc := range []struct {
		name         string
		linked       bool
		targetFills  bool
		wantExitSent bool
	}{
		{name: "linked take profit fills", linked: true, targetFills: true},
		{name: "linked take profit expires too", linked: true, wantExitSent: true},
		{name: "independent legs", wantExitSent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setup(t)
			var b broker.Broker = bracketBroker{Sim: h.s}
			if tc.linked {
				b = newLinkedBroker(h.s)
			}
			h.e.AddBroker(b)
			exits := 0
			h.e.cfg.Hooks.OnOrder = func(o domain.Order) {
				if o.Tag == domain.TagExit && o.Status == domain.OrderNew {
					exits++
				}
			}
			h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
			h.s.OnCandle(h.bar(100, 100, 100, 100))
			var stop, target domain.Order
			for _, o := range h.e.OpenOrders() {
				switch o.Tag {
				case domain.TagStopLoss:
					stop = o
				case domain.TagTakeProfit:
					target = o
				}
			}
			require.NotEmpty(t, stop.ClientID)
			require.NotEmpty(t, target.ClientID)

			stop.Status = domain.OrderExpired
			h.e.onBrokerEvent(broker.Event{Kind: broker.EventOrder, Order: stop})
			if !tc.linked {
				assert.Equal(t, 1, exits, "an independent stop that expired leaves the position naked")
				return
			}
			assert.Zero(t, exits, "the linked stop expired because the take profit triggered")
			if tc.targetFills {
				require.NoError(t, h.s.CancelOrder(context.Background(), stop)) // gone at the exchange
				h.s.OnCandle(h.bar(100, 110, 99, 105))
				assert.Empty(t, h.a.Positions())
				assert.Empty(t, h.e.OpenOrders())
				assert.Zero(t, exits)
				assert.Equal(t, "1010", h.a.Snapshot().Balance.String(), "two units sold once at 105")
				return
			}
			target.Status = domain.OrderExpired
			h.e.onBrokerEvent(broker.Event{Kind: broker.EventOrder, Order: target})
			assert.Equal(t, 1, exits, "both linked legs ended unfilled: close at market")
		})
	}
}

// A restored live position is marked through warmup candles while the cash
// balance is still unknown. Those historical swings tripped the kill switch
// and sold the position on the testnet before the user stream was up.
func TestHeldRiskIgnoresWarmupUntilArmed(t *testing.T) {
	h := setup(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	rm := risk.NewManager(risk.DefaultConfig(), log)
	h.e = New(Config{Book: h.e.cfg.Book, Risk: rm, Log: log, Clock: func() time.Time { return h.now }, HoldRisk: true})
	h.e.AddBroker(h.s)
	h.a.SetBalance(decimal.Zero)
	h.a.SetPosition(domain.Position{PositionKey: domain.PositionKey{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"},
		Side: domain.Long, Quantity: decimal.NewFromInt(1), EntryPrice: decimal.NewFromInt(100), Leverage: 1})

	stale := h.bar(100, 101, 99, 101)
	stale.OpenTime = h.now.Add(-time.Hour)
	h.e.OnCandle(context.Background(), stale)
	stale.OpenTime, stale.Close = stale.OpenTime.Add(time.Minute), decimal.NewFromInt(95)
	h.e.OnCandle(context.Background(), stale)
	h.e.OnCandle(context.Background(), h.bar(95, 96, 94, 96)) // live, but not armed yet
	halted, reason := rm.Halted()
	require.False(t, halted, reason)

	h.a.SetBalance(decimal.NewFromInt(1000)) // the first reconcile
	h.e.ArmRisk(context.Background())
	halted, _ = rm.Halted()
	assert.False(t, halted)
	h.e.OnCandle(context.Background(), h.bar(96, 96, 40, 40))
	halted, reason = rm.Halted()
	assert.True(t, halted, "armed: a real loss of >5%% trips")
	assert.Contains(t, reason, "daily loss")
}

func TestPartialBracketFailureCancelsAcceptedLeg(t *testing.T) {
	h := setup(t)
	h.e.AddBroker(bracketBroker{Sim: h.s, failTarget: true})
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, StopLoss: decimal.NewFromInt(95), TakeProfit: decimal.NewFromInt(105)})
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	orders := h.s.OpenOrders()
	require.Len(t, orders, 1, "cancel the native stop before closing a failed bracket")
	assert.Equal(t, domain.OrderMarket, orders[0].Type)
	assert.Equal(t, domain.TagExit, orders[0].Tag)
}

func TestReversalClosesBeforeOpeningOppositePosition(t *testing.T) {
	h := setup(t)
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Long, Quantity: decimal.NewFromInt(1)})
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	h.intent(strategy.Intent{Kind: strategy.Enter, Side: domain.Short, Quantity: decimal.NewFromInt(2)})
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	assert.Empty(t, h.a.Positions(), "exit fills before the deferred reversal entry")
	h.s.OnCandle(h.bar(100, 100, 100, 100))
	p, ok := h.a.Position("BTCUSDT")
	require.True(t, ok)
	assert.Equal(t, domain.Short, p.Side)
	assert.Equal(t, "2", p.Quantity.String())
}

func TestClientIDsAlphanumericBoundedAndUnique(t *testing.T) {
	h := setup(t)
	seen := map[string]bool{}
	for range 1000 {
		id := h.e.newClientID(h.i)
		assert.Regexp(t, `^tb[0-9a-z]+$`, id)
		assert.LessOrEqual(t, len(id), 32)
		assert.False(t, seen[id])
		seen[id] = true
	}
}

type candleSignal struct{ calls int }

func (*candleSignal) Init(strategy.Context, strategy.Config) error { return nil }
func (*candleSignal) Timeframes() []domain.Timeframe               { return []domain.Timeframe{"1m"} }
func (s *candleSignal) OnCandle(domain.Candle) []strategy.Intent {
	s.calls++
	return []strategy.Intent{{Kind: strategy.Enter, Symbol: "BTCUSDT", Side: domain.Long}}
}

func TestStaleCandlesWarmIndicatorsWithoutTrading(t *testing.T) {
	h := setup(t)
	s := &candleSignal{}
	h.i.s, h.i.routes, h.i.tfs = s, map[instKey]bool{{"sim", domain.Futures, "BTCUSDT"}: true}, map[domain.Timeframe]bool{"1m": true}
	h.e.instances = []*instance{h.i}
	c := h.bar(100, 100, 100, 100)
	c.OpenTime = h.now.Add(-3 * time.Minute)
	h.e.OnCandle(context.Background(), c)
	assert.Equal(t, 1, s.calls)
	assert.Empty(t, h.e.OpenOrders())
	h.e.OnCandle(context.Background(), h.bar(100, 100, 100, 100))
	assert.Equal(t, 2, s.calls)
	assert.Len(t, h.e.OpenOrders(), 1)
}
