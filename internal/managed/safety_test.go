package managed

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCancelBeforeFillKeepsReservationAndDoesNotRegressTerminal(t *testing.T) {
	s, _ := NewState(spec())
	s.Mark("ETHUSDT", d("100"))
	o := order("pending", domain.Buy, "2", "100")
	require.NoError(t, s.ReserveOrder("a/one", o, "", d("0.001")))
	ack := o
	ack.Status = domain.OrderCanceled
	ack.FilledQty = d("1")
	s.UpdateOrder(ack)
	require.Equal(t, "submitting", s.Orders[o.ClientID].State)
	q, _ := s.Reserved("a", "", "")
	require.Equal(t, "200.2", q.String())
	_, err := s.Fill(fill(o, "1", "1", "100"))
	require.NoError(t, err)
	require.Equal(t, "terminal", s.Orders[o.ClientID].State)
	require.Equal(t, domain.OrderCanceled, s.Orders[o.ClientID].Order.Status)
	s.UpdateOrder(o)
	require.Equal(t, "terminal", s.Orders[o.ClientID].State)
	q, _ = s.Reserved("a", "", "")
	require.True(t, q.IsZero())
}

func TestParentExposureCannotBeBypassedWithSmallOrders(t *testing.T) {
	sp := spec()
	sp.Risk.MaxPositionPct = .1
	s, _ := NewState(sp)
	s.Mark("ETHUSDT", d("100"))
	require.NoError(t, s.ReserveOrder("a/one", order("a", domain.Buy, "0.6", "100"), "", decimal.Zero))
	require.ErrorContains(t, s.ReserveOrder("b/two", order("b", domain.Buy, "0.6", "100"), "", decimal.Zero), "aggregate symbol")
}

type failRepo struct {
	Repository
	fail bool
}

func (f *failRepo) Update(ctx context.Context, id, op string, payload any, fn func(*State) error) error {
	if f.fail && op == "event_checkpoint" {
		return errors.New("injected DB outage")
	}
	return f.Repository.Update(ctx, id, op, payload, fn)
}

func TestRestartReplaysDurableInboxWithoutDuplicateDebit(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	mem, _ := NewMemory(spec())
	repo := &failRepo{Repository: mem}
	newRuntime := func() *Runtime {
		r, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: func() time.Time { return now }}), testInstruments(), d("0.001"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
		require.NoError(t, err)
		require.NoError(t, r.Start(t.Context()))
		return r
	}
	rt := newRuntime()
	bar := func() domain.Candle {
		return domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: d("100"), High: d("100"), Low: d("100"), Close: d("100"), Closed: true}
	}
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	require.NoError(t, rt.Tick(t.Context(), now))
	repo.fail = true
	now = now.Add(time.Minute)
	require.Error(t, rt.Record(t.Context(), bar(), true))
	s, _ := mem.Load(t.Context())
	require.NotEmpty(t, s.Pending)
	before := s.Instances["a"].Cash
	repo.fail = false
	rt = newRuntime()
	s, _ = rt.Snapshot(t.Context())
	require.Empty(t, s.Pending)
	require.True(t, before.Equal(s.Instances["a"].Cash))
	require.Equal(t, "0.2", s.Holding("a/one", "ETHUSDT").Quantity.String())
	require.Equal(t, "1000", s.Equity().String())
}

func testInstruments() []domain.Instrument {
	return []domain.Instrument{{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", TickSize: d("0.01"), StepSize: d("0.001"), MinQty: d("0.001"), MinNotional: d("5")}}
}

func TestCloneDoesNotCopyCapitalAndEditedBindingNeedsRestart(t *testing.T) {
	mem, _ := NewMemory(spec())
	rt, err := NewRuntime(t.Context(), mem, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), nil, nil)
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	require.NoError(t, rt.Command(t.Context(), Command{ID: "clone", Operation: "clone", Scope: "a", Instance: &InstanceSpec{ID: "copy", Name: "Copy", Allocation: d("999")}}))
	s, _ := mem.Load(t.Context())
	require.True(t, s.Instances["copy"].Cash.IsZero())
	require.Equal(t, "1000", s.Equity().String())
	require.ErrorContains(t, rt.Command(t.Context(), Command{ID: "resume-new", Operation: "resume", Scope: "copy"}), "restart")
	require.NoError(t, rt.Command(t.Context(), Command{ID: "pause", Operation: "pause", Scope: "a"}))
	b := s.Bindings["a/one"].Spec
	b.Params = map[string]any{"quote_amount": 10, "interval": "2h"}
	require.NoError(t, rt.Command(t.Context(), Command{ID: "edit", Operation: "edit_binding", Scope: "a", Binding: &b}))
	require.ErrorContains(t, rt.Command(t.Context(), Command{ID: "resume-edited", Operation: "resume", Scope: "a"}), "restart")
	require.ErrorContains(t, rt.Command(t.Context(), Command{ID: "reset-edited", Operation: "reset", Scope: "a", Confirm: true}), "restart")
}

func TestRiskHaltIsStickyAndTransferIsNotProfit(t *testing.T) {
	s, _ := NewState(spec())
	now := time.Now()
	s.Observe(now)
	require.NoError(t, s.Transfer("a", "b", d("100")))
	s.Observe(now)
	require.False(t, s.Instances["a"].Halted)
	require.False(t, s.Instances["b"].Halted)
	s.Instances["a"].Cash = d("10")
	s.Observe(now)
	require.True(t, s.Instances["a"].Halted)
	s.Instances["a"].Cash = d("400")
	s.Observe(now)
	require.True(t, s.Instances["a"].Halted)
}

func TestManagedSimSettlesStopBeforeOtherLegInSameCandle(t *testing.T) {
	now := time.Now().UTC()
	repo, _ := NewMemory(spec())
	require.NoError(t, repo.Update(t.Context(), "fixture", "fixture", nil, func(s *State) error {
		s.Instances["a"].Cash = d("480")
		p := s.Holding("a/one", "ETHUSDT")
		p.Quantity = d("0.2")
		p.Cost = d("20")
		s.Mark("ETHUSDT", d("100"))
		return nil
	}))
	sim := broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: func() time.Time { return now }})
	rt, err := NewRuntime(t.Context(), repo, sim, testInstruments(), d("0.001"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	require.NoError(t, rt.syncView(t.Context(), "a/one"))
	stop := order("stop", domain.Sell, "0.2", "0")
	stop.Type = domain.OrderStopMarket
	stop.StopPrice = d("95")
	stop.Tag = domain.TagStopLoss
	stop.Strategy = "a/one"
	target := stop
	target.ClientID = "target"
	target.Type = domain.OrderTakeProfit
	target.StopPrice = d("105")
	target.Tag = domain.TagTakeProfit
	b := scopedBroker{r: rt, owner: "a/one"}
	_, err = b.PlaceBracket(t.Context(), stop, target)
	require.NoError(t, err)
	rt.engines["a/one"].AdoptOrders([]domain.Order{stop, target})
	require.NoError(t, rt.save(t.Context(), "a/one"))
	now = now.Add(time.Minute)
	bar := domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: d("100"), High: d("110"), Low: d("90"), Close: d("100"), Closed: true}
	require.NoError(t, rt.Record(t.Context(), bar, true))
	s, _ := repo.Load(t.Context())
	require.True(t, s.Holding("a/one", "ETHUSDT").Quantity.IsZero())
	require.Len(t, s.Executions, 1)
	require.Equal(t, "499", s.Instances["a"].Cash.String())
	require.Equal(t, "terminal", s.Orders["target"].State)
}

func TestTwoStrategiesShareOneInstanceCapitalWithoutSharingInventory(t *testing.T) {
	sp := spec()
	other := sp.Instances[1].Strategies[0]
	sp.Instances = sp.Instances[:1]
	sp.Instances[0].Strategies = append(sp.Instances[0].Strategies, other)
	repo, err := NewMemory(sp)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Minute)
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: func() time.Time { return now }}), testInstruments(), d("0.001"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	bar := func() domain.Candle {
		return domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: d("100"), High: d("100"), Low: d("100"), Close: d("100"), Closed: true}
	}
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	require.NoError(t, rt.Tick(t.Context(), now))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	s, _ := repo.Load(t.Context())
	require.Equal(t, "450", s.Instances["a"].Cash.String())
	require.Equal(t, "0.2", s.Holding("a/one", "ETHUSDT").Quantity.String())
	require.Equal(t, "0.3", s.Holding("a/two", "ETHUSDT").Quantity.String())
	require.Equal(t, "1000", s.Equity().String())
	require.NoError(t, rt.Command(t.Context(), Command{ID: "close-one", Operation: "flatten", Scope: "a/one", Confirm: true}))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	s, _ = repo.Load(t.Context())
	require.True(t, s.Holding("a/one", "ETHUSDT").Quantity.IsZero())
	require.Equal(t, "0.3", s.Holding("a/two", "ETHUSDT").Quantity.String())
}

func TestLateDirectionalFillWithoutEntryPlanGetsFallbackExit(t *testing.T) {
	sp := spec()
	sp.Instances = sp.Instances[:1]
	sp.Instances[0].Strategies[0].Name = "trendspot"
	sp.Instances[0].Strategies[0].Params = map[string]any{"mode": "momentum", "timeframe": "4h"}
	repo, _ := NewMemory(sp)
	now := time.Now().UTC()
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: func() time.Time { return now }}), testInstruments(), d("0.001"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	o := order("late-entry", domain.Buy, "0.2", "100")
	o.Tag = domain.TagEntry
	o.Strategy = "a/one"
	require.NoError(t, repo.Update(t.Context(), "late-fixture", "fixture", nil, func(s *State) error {
		s.Mark("ETHUSDT", d("100"))
		require.NoError(t, s.ReserveOrder("a/one", o, "", d("0.001")))
		o.Status = domain.OrderCanceled
		s.UpdateOrder(o)
		return nil
	}))
	rt.events = append(rt.events, broker.Event{Kind: broker.EventFill, Order: o, Fill: fill(o, "late", "0.2", "100")})
	require.NoError(t, rt.Tick(t.Context(), now))
	s, _ := repo.Load(t.Context())
	found := false
	for _, o := range s.Orders {
		if o.Owner == "a/one" && o.Order.Side == domain.Sell && o.Order.Tag == domain.TagExit && o.State != "terminal" {
			found = true
		}
	}
	require.True(t, found, "a recovered directional position must have a fallback exit")
}

func TestMissingCandleStopsRuntimeBeforeNewEntries(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	repo, _ := NewMemory(spec())
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	c := domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: d("100"), High: d("100"), Low: d("100"), Close: d("100"), Closed: true}
	require.NoError(t, rt.Record(t.Context(), c, true))
	now = now.Add(3 * time.Minute)
	c.OpenTime = now.Add(-time.Minute)
	require.ErrorContains(t, rt.Record(t.Context(), c, true), "candle gap")
	require.Error(t, rt.Tick(t.Context(), now))
	s, _ := repo.Load(t.Context())
	require.Empty(t, s.Orders)
}

func TestReconciliationFindsInitialUnknownOrdersAndNewDeposits(t *testing.T) {
	s, _ := NewState(spec())
	account := &broker.AccountState{Balances: map[string]broker.Balance{"USDT": {Wallet: d("1100")}}, OpenOrders: []domain.Order{order("manual", domain.Buy, "1", "100")}}
	s.Reconcile(account, nil)
	require.Contains(t, s.Attention, "unknown exchange order")
	account.OpenOrders = nil
	s.Reconcile(account, nil)
	require.Empty(t, s.Attention)
	account.Balances["DOGE"] = broker.Balance{Wallet: d("1")}
	s.Reconcile(account, nil)
	require.Contains(t, s.Attention, "DOGE")
	s.FeeIssue = "waiting for commission asset valuation"
	s.valueFees()
	require.Contains(t, s.Attention, "DOGE", "valuing a fee cannot clear an unrelated discrepancy")
}

func TestReconciliationKeepsUnderfundedWalletBlocked(t *testing.T) {
	s, _ := NewState(spec())
	account := &broker.AccountState{Balances: map[string]broker.Balance{"USDT": {Wallet: d("600")}}}
	s.Reconcile(account, nil)
	require.Contains(t, s.Attention, "does not cover")
	// The next periodic reconciliation must not accept the shortfall as baseline.
	s.Reconcile(account, nil)
	require.Contains(t, s.Attention, "does not cover")
	require.Empty(t, s.Baseline)
	account.Balances["USDT"] = broker.Balance{Wallet: d("1050")}
	s.Reconcile(account, nil)
	require.Empty(t, s.Attention)
	require.True(t, s.Baseline["USDT"].Equal(d("50")))
}
