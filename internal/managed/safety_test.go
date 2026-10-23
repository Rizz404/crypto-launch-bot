package managed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
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

type countedLoads struct {
	Repository
	loads int
}

func (r *countedLoads) Load(ctx context.Context) (*State, error) {
	r.loads++
	return r.Repository.Load(ctx)
}

func TestIdleDirectionalDrainSharesOneLedgerSnapshot(t *testing.T) {
	sp := spec()
	for i := range sp.Instances {
		sp.Instances[i].Strategies[0].Name = "emacross"
		sp.Instances[i].Strategies[0].Params = nil
	}
	mem, err := NewMemory(sp)
	require.NoError(t, err)
	repo := &countedLoads{Repository: mem}
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), nil, nil)
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	require.NoError(t, mem.Update(t.Context(), "transfer", "transfer", nil, func(s *State) error {
		return s.Transfer("a", "b", d("50"))
	}))
	repo.loads = 0
	require.NoError(t, rt.drain(t.Context()))
	require.Equal(t, 1, repo.loads, "idle checks must not reread the full ledger for each binding")
	require.Equal(t, "450", rt.accounts["a/one"].Snapshot().Balance.String())
	require.Equal(t, "450", rt.accounts["b/two"].Snapshot().Balance.String())
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
	var notified []Execution
	newRuntime := func() *Runtime {
		r, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: func() time.Time { return now }}), testInstruments(), d("0.001"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
		require.NoError(t, err)
		r.OnExecution = func(e Execution, o domain.Order) {
			s, err := mem.Load(t.Context())
			require.NoError(t, err)
			for _, pending := range s.Pending {
				require.NotEqual(t, e.Fill, pending.Fill, "notification must follow durable inbox acknowledgement")
			}
			require.Equal(t, e.Fill.OrderClientID, o.ClientID)
			notified = append(notified, e)
		}
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
	require.Empty(t, notified, "failed checkpoint must not announce an execution")
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
	require.NotEmpty(t, notified, "durable inbox replay must announce recovered executions")
	count := len(notified)
	newRuntime()
	require.Len(t, notified, count, "settled history must not be announced again on restart")
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

// exchangeLike mimics Binance Spot: a successful cancel emits nothing; the
// terminal state arrives later on the user stream or through an order query.
type exchangeLike struct {
	h        broker.Handler
	open     map[string]domain.Order
	queried  int
	failNext error
	balances map[string]broker.Balance
	query    func(domain.Order) (domain.Order, error)
}

func (x *exchangeLike) QueryOrder(_ context.Context, o domain.Order) (domain.Order, error) {
	return x.query(o)
}

func (x *exchangeLike) Exchange() string                             { return "sim" }
func (x *exchangeLike) SetHandler(h broker.Handler)                  { x.h = h }
func (x *exchangeLike) Track([]domain.Order)                         {}
func (x *exchangeLike) Resync() <-chan struct{}                      { return nil }
func (x *exchangeLike) Start(context.Context, *sync.WaitGroup) error { return nil }
func (x *exchangeLike) SetLeverage(context.Context, domain.Market, string, int) error {
	return nil
}
func (x *exchangeLike) AccountState(context.Context, domain.Market) (*broker.AccountState, error) {
	st := &broker.AccountState{Balances: x.balances}
	for _, o := range x.open {
		if o.Status.Open() {
			st.OpenOrders = append(st.OpenOrders, o)
		}
	}
	return st, nil
}
func (x *exchangeLike) PlaceOrder(_ context.Context, o domain.Order) (domain.Order, error) {
	if err := x.failNext; err != nil {
		x.failNext = nil
		return o, err
	}
	if x.open == nil {
		x.open = map[string]domain.Order{}
	}
	o.Status = domain.OrderNew
	x.open[o.ClientID] = o
	return o, nil
}
func (x *exchangeLike) PlaceBracket(ctx context.Context, stop, target domain.Order) ([]domain.Order, error) {
	a, _ := x.PlaceOrder(ctx, stop)
	b, _ := x.PlaceOrder(ctx, target)
	return []domain.Order{a, b}, nil
}
func (x *exchangeLike) CancelOrder(_ context.Context, o domain.Order) error {
	if leg, ok := x.open[o.ClientID]; !ok || !leg.Status.Open() {
		return &exchange.APIError{Exchange: "sim", HTTPStatus: 400, Code: -2011, Message: "Unknown order sent."}
	}
	// Canceling one OCO leg cancels the whole list on the exchange.
	for id, leg := range x.open {
		if leg.Symbol == o.Symbol && leg.Side == o.Side {
			leg.Status = domain.OrderCanceled
			x.open[id] = leg
		}
	}
	delete(x.open, o.ClientID)
	return nil
}
func (x *exchangeLike) SyncOrders(_ context.Context, tracked, _ []domain.Order) {
	for _, o := range tracked {
		x.queried++
		if x.query != nil {
			final, err := x.query(o)
			if err == nil && !final.Status.Open() {
				x.h(broker.Event{Kind: broker.EventOrder, Order: final})
			}
			continue
		}
		final := o
		final.Status = domain.OrderCanceled
		if leg, ok := x.open[o.ClientID]; ok {
			if leg.Status.Open() {
				continue
			}
			delete(x.open, o.ClientID)
		}
		x.h(broker.Event{Kind: broker.EventOrder, Order: final})
	}
}

func TestLiveCancelReleasesProtectionBeforeExit(t *testing.T) {
	repo, _ := NewMemory(spec())
	x := &exchangeLike{}
	now := time.Now()
	rt, err := NewRuntime(t.Context(), repo, x, testInstruments(), d("0.001"), func() time.Time { return now }, nil)
	require.NoError(t, err)
	require.NoError(t, repo.Update(t.Context(), "seed", "seed", nil, func(s *State) error {
		s.Mark("ETHUSDT", d("100"))
		s.Holding("a/one", "ETHUSDT").Quantity = d("0.2")
		s.Holding("a/one", "ETHUSDT").Cost = d("20")
		return nil
	}))
	b := &scopedBroker{r: rt, owner: "a/one"}
	stop := order("stop", domain.Sell, "0.2", "90")
	stop.Type = domain.OrderStopMarket
	target := order("target", domain.Sell, "0.2", "120")
	_, err = b.PlaceBracket(t.Context(), stop, target)
	require.NoError(t, err)

	// Engine exit: cancel both legs, then sell immediately.
	require.NoError(t, b.CancelOrder(t.Context(), stop))
	require.Error(t, b.CancelOrder(t.Context(), target), "second leg is already gone with the list")
	exit := order("exit", domain.Sell, "0.2", "0")
	exit.Type = domain.OrderMarket
	_, err = b.PlaceOrder(t.Context(), exit)
	require.NoError(t, err)
	s, _ := repo.Load(t.Context())
	require.Equal(t, "terminal", s.Orders["stop"].State)
	require.Equal(t, "terminal", s.Orders["target"].State)
	_, base := s.Reserved("", "a/one", "ETHUSDT")
	require.True(t, base.Equal(d("0.2")), "only the exit order holds the coins")
}

func TestCapitalFlowsDoNotTripStrategyLossLimits(t *testing.T) {
	s, _ := NewState(spec())
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	s.Observe(now)
	require.NoError(t, s.Transfer("a", "reserve", d("400")))
	s.Observe(now.Add(time.Minute))
	require.False(t, s.Bindings["a/one"].Halted, "moving free cash out is not a strategy loss")
	require.NoError(t, s.Apply(Command{Operation: "limits", Scope: "a/one", Amount: d("50"), Confirm: true}))
	s.Observe(now.Add(2 * time.Minute))
	require.False(t, s.Bindings["a/one"].Halted, "setting a cap is not a strategy loss")
	require.NoError(t, s.Transfer("reserve", "a", d("400")))
	s.Observe(now.Add(3 * time.Minute))
	require.False(t, s.Instances["a"].Halted)
	require.False(t, s.Bindings["a/one"].Halted)
}

func TestResolveOrderUsesExchangeEvidence(t *testing.T) {
	timeout := &exchange.APIError{Exchange: "sim", HTTPStatus: 503, Code: -1007, Message: "execution status unknown"}
	for _, tc := range []struct {
		name      string
		query     func(domain.Order) (domain.Order, error)
		wantState string
		wantErr   string
	}{
		{name: "never reached exchange", wantState: "terminal", query: func(o domain.Order) (domain.Order, error) {
			return o, fmt.Errorf("%w: -2013", broker.ErrOrderNotFound)
		}},
		{name: "resting on exchange", wantState: "open", query: func(o domain.Order) (domain.Order, error) {
			o.Status, o.ExchangeID = domain.OrderNew, "77"
			return o, nil
		}},
		{name: "query unavailable", wantState: "unknown", wantErr: "left unresolved", query: func(o domain.Order) (domain.Order, error) {
			return o, errors.New("connection reset")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := NewMemory(spec())
			now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
			clock := func() time.Time { return now }
			x := &exchangeLike{failNext: timeout, query: tc.query, balances: map[string]broker.Balance{"USDT": {Wallet: d("1000")}}}
			rt, err := NewRuntime(t.Context(), repo, x, testInstruments(), d("0.001"), clock, nil)
			require.NoError(t, err)
			require.NoError(t, repo.Update(t.Context(), "price", "mark", nil, func(s *State) error {
				s.Mark("ETHUSDT", d("100"))
				s.PriceAt["ETHUSDT"] = now
				return nil
			}))
			o := order("uncertain", domain.Buy, "1", "100")
			o.CreatedAt = now
			_, err = (&scopedBroker{r: rt, owner: "a/one"}).PlaceOrder(t.Context(), o)
			require.Error(t, err)
			s, _ := repo.Load(t.Context())
			require.Equal(t, "unknown", s.Orders["uncertain"].State)
			require.NotEmpty(t, s.Attention)

			// The process stops on an uncertain submission; resolve after restart.
			rt, err = NewRuntime(t.Context(), repo, x, testInstruments(), d("0.001"), clock, nil)
			require.NoError(t, err)
			cmd := Command{ID: "resolve-" + tc.name, Operation: "resolve_order", Order: "uncertain"}
			require.ErrorContains(t, rt.Command(t.Context(), cmd), "confirmation")
			cmd.Confirm = true
			require.ErrorContains(t, rt.Command(t.Context(), cmd), "in flight")
			now = now.Add(ResolveMinAge)
			err = rt.Command(t.Context(), cmd)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			s, _ = repo.Load(t.Context())
			require.Equal(t, tc.wantState, s.Orders["uncertain"].State)
			quote, _ := s.Reserved("", "a/one", "")
			switch tc.wantState {
			case "terminal":
				require.True(t, quote.IsZero(), "reservation released")
				require.Empty(t, s.Pending, "engine was told about the rejection")
				require.Empty(t, s.Attention, "reconciliation cleared attention from evidence")
			case "open":
				require.True(t, quote.IsPositive(), "a resting order keeps its funds")
			default:
				require.NotEmpty(t, s.Attention)
			}
		})
	}
}

func TestCompactArchivesSettledDetailOnly(t *testing.T) {
	repo, _ := NewMemory(spec())
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	old := now.Add(-DefaultRetention - time.Hour)
	require.NoError(t, repo.Update(t.Context(), "seed", "seed", nil, func(s *State) error {
		s.Mark("ETHUSDT", d("100"))
		buy := order("old-buy", domain.Buy, "1", "100")
		require.NoError(t, s.ReserveOrder("a/one", buy, "", d("0")))
		f := fill(buy, "t1", "1", "100")
		f.Time = old
		_, err := s.Fill(f)
		require.NoError(t, err)
		s.Orders["old-buy"].SettledAt = old
		// An old OCO leg whose sibling is still open must stay.
		stop := order("stop", domain.Sell, "1", "90")
		target := order("target", domain.Sell, "1", "120")
		require.NoError(t, s.ReserveOrder("a/one", stop, "oco/stop", d("0")))
		require.NoError(t, s.ReserveOrder("a/one", target, "oco/stop", d("0")))
		s.Reject("target", "expired")
		s.Orders["target"].SettledAt = old
		s.History = append(s.History, Audit{At: old, Kind: "transfer", Message: "old"})
		s.Note("recent", "account", "kept")
		s.Requests["old-request"] = "hash"
		s.RequestAt["old-request"] = old
		return nil
	}))
	before, _ := repo.Load(t.Context())
	require.NoError(t, repo.Update(t.Context(), nonce(), "compact", nil, func(s *State) error { s.Compact(now, DefaultRetention); return nil }))
	s, _ := repo.Load(t.Context())
	require.Nil(t, s.Orders["old-buy"])
	require.NotNil(t, s.Orders["target"], "sibling of an open leg")
	require.NotNil(t, s.Orders["stop"])
	require.Empty(t, s.Executions)
	require.False(t, s.Seen["old-buy:t1"])
	require.Len(t, s.History, 1)
	require.NotContains(t, s.Requests, "old-request")
	kinds := map[string]int{}
	for _, a := range repo.Archived {
		kinds[a.Kind]++
	}
	require.Equal(t, map[string]int{"order": 1, "execution": 1, "audit": 1}, kinds)
	require.True(t, before.Bindings["a/one"].Fees.Equal(s.Bindings["a/one"].Fees))
	require.True(t, before.Equity().Equal(s.Equity()), "aggregates do not move")
	require.True(t, s.Holding("a/one", "ETHUSDT").Quantity.Equal(d("1")))

	// A pruned idempotency key is still rejected through the journal.
	require.NoError(t, repo.Update(t.Context(), "seed", "seed", nil, func(*State) error { panic("replayed") }))
}

// A storeOps wrapper counts ledger writes so throttling is observable.
type countingRepo struct {
	Repository
	ops map[string]int
}

func (c *countingRepo) Update(ctx context.Context, id, op string, payload any, fn func(*State) error) error {
	c.ops[op]++
	return c.Repository.Update(ctx, id, op, payload, fn)
}

func TestIntrabarPricesAreThrottledInTheLedger(t *testing.T) {
	mem, _ := NewMemory(spec())
	repo := &countingRepo{Repository: mem, ops: map[string]int{}}
	now := time.Date(2026, 10, 20, 12, 0, 30, 0, time.UTC)
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), func() time.Time { return now }, nil)
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	tick := func(price string, closed bool) {
		c := domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Truncate(time.Minute), Open: d(price), High: d(price), Low: d(price), Close: d(price), Closed: closed}
		if closed {
			c.OpenTime = c.OpenTime.Add(-time.Minute)
		}
		require.NoError(t, rt.Record(t.Context(), c, closed))
	}
	tick("100", false)
	before := repo.ops["mark"]
	for _, p := range []string{"101", "102", "103"} {
		now = now.Add(time.Second)
		tick(p, false)
	}
	s, _ := mem.Load(t.Context())
	require.True(t, s.Prices["ETHUSDT"].Equal(d("100")), "intrabar ticks inside the interval are not written")
	require.Equal(t, before, repo.ops["mark"], "no ledger transaction for throttled ticks")
	now = now.Add(DefaultMarkInterval)
	tick("104", false)
	s, _ = mem.Load(t.Context())
	require.True(t, s.Prices["ETHUSDT"].Equal(d("104")), "written again after the interval")
	tick("105", true)
	s, _ = mem.Load(t.Context())
	require.True(t, s.Prices["ETHUSDT"].Equal(d("105")), "closed candles are always written")
}

func TestSameJSONIgnoresKeyOrderAndSpacing(t *testing.T) {
	require.True(t, sameJSON([]byte(`{"b":1,"a":{"y":[1,2],"x":"s"}}`), []byte(`{"a": {"x": "s", "y": [1, 2]}, "b": 1}`)))
	require.False(t, sameJSON([]byte(`{"a":1}`), []byte(`{"a":2}`)))
	require.False(t, sameJSON(nil, []byte(`{"a":1}`)))
}
