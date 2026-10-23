package managed

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rizz404/trading-bot/internal/exchange"
	"log/slog"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/engine"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/shopspring/decimal"
)

// DefaultMarkInterval keeps intrabar price writes to one per symbol every
// ten seconds; the entry freshness check allows two minutes.
const DefaultMarkInterval = 10 * time.Second

func nonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

type Runtime struct {
	AllowGaps bool
	// Retention is how long settled detail stays in the hot state before it
	// moves to the archive; zero means DefaultRetention.
	Retention time.Duration
	// MarkInterval bounds how often a still-forming candle's price is written
	// to the ledger; zero means DefaultMarkInterval. Closed candles, fills,
	// orders and controls are always written immediately. Engines see every
	// price through the in-memory marks regardless.
	MarkInterval time.Duration
	marks        map[string]decimal.Decimal
	markedAt     map[string]time.Time
	compacted    time.Time
	lastClosed   map[domain.SeriesKey]time.Time
	runID        string
	revisions    map[string]string
	warmMarks    map[string]domain.Candle
	CheckLease   func(context.Context) error
	initial      map[string][]byte
	mu           sync.Mutex
	qmu          sync.Mutex
	events       []broker.Event
	repo         Repository
	physical     broker.Broker
	engines      map[string]*engine.Engine
	accounts     map[string]*portfolio.Account
	routes       map[string]func(broker.Event)
	instruments  map[string]domain.Instrument
	log          *slog.Logger
	fee          decimal.Decimal
	clock        func() time.Time
	failure      error
	ready        bool
	OnNotice     func(string, slog.Level, string)
	// OnExecution observes a fill after its engine checkpoint and inbox ack commit.
	// Delivery is best effort; a crash after commit can omit a notification.
	OnExecution func(Execution, domain.Order)
}

func NewRuntime(ctx context.Context, repo Repository, physical broker.Broker, ins []domain.Instrument, fee decimal.Decimal, clock func() time.Time, log *slog.Logger) (*Runtime, error) {
	if clock == nil {
		clock = time.Now
	}
	if log == nil {
		log = slog.Default()
	}
	s, err := repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	r := &Runtime{marks: map[string]decimal.Decimal{}, markedAt: map[string]time.Time{}, lastClosed: map[domain.SeriesKey]time.Time{}, revisions: map[string]string{}, warmMarks: map[string]domain.Candle{}, initial: map[string][]byte{}, repo: repo, physical: physical, engines: map[string]*engine.Engine{}, accounts: map[string]*portfolio.Account{}, routes: map[string]func(broker.Event){}, instruments: map[string]domain.Instrument{}, log: log, fee: fee, clock: clock}
	for _, in := range ins {
		r.instruments[in.Symbol] = in
	}
	physical.SetHandler(func(ev broker.Event) { r.qmu.Lock(); r.events = append(r.events, ev); r.qmu.Unlock() })
	for _, owner := range sortedBindings(s) {
		b := s.Bindings[owner]
		r.revisions[owner] = Hash(b.Spec.Config(physical.Exchange(), owner))
		book := portfolio.NewBook()
		acct := portfolio.NewAccount(portfolio.AccountKey{Exchange: physical.Exchange(), Market: domain.Spot}, decimal.Zero, nil)
		book.Add(acct)
		r.accounts[owner] = acct
		rc := effectiveRisk(b.Spec.Risk)
		rc.DailyLossLimit = 0
		rc.MaxDrawdown = 0
		rc.MaxLeverage = 1
		e := engine.New(engine.Config{Book: book, Risk: risk.NewManager(rc, log), Log: log, Clock: clock, IDSeed: clock().UnixNano(), Hooks: engine.Hooks{
			BeforeBroker: func(raw []byte) error { return r.checkpoint(ctx, owner, raw) },
			OnNotice: func(level slog.Level, msg string, attrs ...any) {
				_ = r.repo.Update(ctx, nonce(), "notice", msg, func(s *State) error { s.Bindings[owner].LastNotice = msg + " " + fmt.Sprint(attrs...); return nil })
				if r.OnNotice != nil {
					r.OnNotice(owner, level, msg)
				}
			},
		}})
		r.engines[owner] = e
		e.AddBroker(&scopedBroker{r: r, owner: owner})
		e.SetInstruments(ins)
		if err = e.AddStrategy(b.Spec.Config(physical.Exchange(), owner)); err != nil {
			return nil, err
		}
		r.initial[owner], err = e.Checkpoint()
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}
func sortedBindings(s *State) []string {
	out := make([]string, 0, len(s.Bindings))
	for id := range s.Bindings {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
func (r *Runtime) checkpoint(ctx context.Context, owner string, raw []byte) error {
	if r.CheckLease != nil {
		if err := r.CheckLease(ctx); err != nil {
			r.failure = err
			return err
		}
	}
	st, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	if Hash(st.Bindings[owner].Spec.Config(r.physical.Exchange(), owner)) != r.revisions[owner] {
		return nil
	}
	err = r.repo.Update(ctx, nonce(), "engine_checkpoint", map[string]any{"owner": owner, "state": jsonRaw(raw)}, func(s *State) error {
		// The stored copy comes back with JSONB key order; only a different
		// value is a change, otherwise every save would rewrite the ledger.
		if sameJSON(s.Bindings[owner].Engine, raw) {
			return nil
		}
		s.Bindings[owner].Engine = append([]byte(nil), raw...)
		return nil
	})
	if err != nil {
		r.failure = err
	}
	return err
}

type jsonRaw = []byte

// sameJSON reports whether two JSON documents hold the same value,
// regardless of key order or spacing.
func sameJSON(a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

func (r *Runtime) Warmup(c domain.Candle) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old := r.lastClosed[c.Series()]; c.Closed && c.OpenTime.After(old) {
		r.lastClosed[c.Series()] = c.OpenTime
	}
	for _, e := range r.engines {
		e.Warmup(c)
	}
	if old, ok := r.warmMarks[c.Symbol]; !ok || !c.CloseTime().Before(old.CloseTime()) {
		r.warmMarks[c.Symbol] = c
	}
}
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ready {
		return errors.New("runtime already started")
	}
	r.runID = nonce()
	if err := r.repo.Update(ctx, "run/"+r.runID, "run_start", r.runID, func(s *State) error {
		bindings := map[string]BindingSpec{}
		for id, b := range s.Bindings {
			bindings[id] = b.Spec
		}
		instances := map[string]any{}
		for id, i := range s.Instances {
			instances[id] = map[string]any{"name": i.Name, "risk": i.Risk, "notify": i.Notify}
		}
		cfg, err := json.Marshal(map[string]any{"wallet": s.Spec, "instances": instances, "bindings": bindings})
		if err != nil {
			return err
		}
		for n := range s.Runs {
			if s.Runs[n].Status == "running" {
				s.Runs[n].Status = "interrupted"
			}
		}
		if len(s.Runs) == 0 {
			s.StartedAt = r.clock()
		}
		s.Runs = append(s.Runs, Run{ID: r.runID, Mode: s.Spec.Mode, Status: "running", StartedAt: r.clock(), ConfigurationHash: Hash(json.RawMessage(cfg)), Configuration: cfg})
		return nil
	}); err != nil {
		return err
	}
	s, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	for _, id := range sortedBindings(s) {
		b := s.Bindings[id]
		raw := b.Engine
		if len(raw) == 0 || string(raw) == "null" {
			raw = r.initial[id]
		}
		if len(raw) > 0 {
			if err = r.engines[id].Restore(raw); err != nil {
				return err
			}
		}
	}
	// A checkpoint can commit just before a reservation fails or a crash.
	// Absence from the durable ledger proves that broker I/O was never reached.
	for owner, e := range r.engines {
		for _, o := range e.OpenOrders() {
			if s.Orders[o.ClientID] == nil {
				o.Status = domain.OrderRejected
				o.Reason = "recovered checkpoint without durable submission"
				r.routes[owner](broker.Event{Kind: broker.EventOrder, Order: o})
			}
		}
		if err = r.save(ctx, owner); err != nil {
			return err
		}
	}
	// Replay the durable inbox against pre-event engine checkpoints before
	// rebasing execution views to the already-updated financial ledger.
	for len(s.Pending) > 0 {
		ev := s.Pending[0]
		if err = r.deliver(ctx, ev); err != nil {
			return err
		}
		s, err = r.repo.Load(ctx)
		if err != nil {
			return err
		}
	}
	if err = r.repo.Update(ctx, nonce(), "warmup_prices", nil, func(s *State) error {
		for _, c := range r.warmMarks {
			s.Mark(c.Symbol, c.Close)
			s.PriceAt[c.Symbol] = c.CloseTime()
		}
		return nil
	}); err != nil {
		return err
	}
	r.ready = true
	if live, ok := r.physical.(broker.Live); ok {
		var tracked []domain.Order
		for _, o := range s.Orders {
			if o.State != "terminal" {
				tracked = append(tracked, o.Order)
			}
		}
		live.Track(tracked)
		st, err := live.AccountState(ctx, domain.Spot)
		if err != nil {
			return err
		}
		live.SyncOrders(ctx, tracked, st.OpenOrders)
		if err = r.repo.Update(ctx, nonce(), "reconcile", nil, func(s *State) error { s.Reconcile(st, r.instruments); return nil }); err != nil {
			return err
		}
	} else if sim, ok := r.physical.(*broker.Sim); ok {
		for _, o := range s.Orders {
			if o.State != "terminal" {
				if _, err = sim.PlaceOrder(ctx, o.Order); err != nil {
					return err
				}
			}
		}
	}
	return r.drain(ctx)
}
func (r *Runtime) Snapshot(ctx context.Context) (*State, error) { return r.repo.Load(ctx) }
func (r *Runtime) syncView(ctx context.Context, owner string) error {
	s, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	r.syncState(s, owner)
	return nil
}
func (r *Runtime) syncState(s *State, owner string) {
	b := s.Bindings[owner]
	i := s.Instances[b.InstanceID]
	a := r.accounts[owner]
	price := func(symbol string) decimal.Decimal {
		if p, ok := r.marks[symbol]; ok {
			return p
		}
		return s.Prices[symbol]
	}
	for symbol, p := range s.Prices {
		r.engines[owner].SetPrice(r.physical.Exchange(), domain.Spot, symbol, p)
	}
	// In-memory marks are newer than throttled ledger marks.
	for symbol, p := range r.marks {
		r.engines[owner].SetPrice(r.physical.Exchange(), domain.Spot, symbol, p)
	}
	for _, p := range a.Positions() {
		p.Quantity = decimal.Zero
		a.SetPosition(p)
	}
	cost := decimal.Zero
	for _, p := range s.Holdings {
		if p.Owner != owner || !p.Quantity.IsPositive() {
			continue
		}
		cost = cost.Add(p.Cost)
		a.SetPosition(domain.Position{PositionKey: domain.PositionKey{Exchange: r.physical.Exchange(), Market: domain.Spot, Symbol: p.Symbol}, Side: domain.Long, Quantity: p.Quantity, EntryPrice: p.Cost.Div(p.Quantity), Leverage: 1, Strategy: owner})
		a.Mark(p.Symbol, price(p.Symbol))
	}
	pending, _ := s.Reserved("", owner, "")
	a.SetBalance(s.FreeCash(i.ID).Add(pending).Add(cost))
	r.engines[owner].SetPaused(i.Status != "running" || b.Status != "running" || i.Halted || b.Halted || s.Halted || s.Attention != "" || Hash(b.Spec.Config(r.physical.Exchange(), owner)) != r.revisions[owner])
}
func (r *Runtime) Record(ctx context.Context, c domain.Candle, closed bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.ready {
		return errors.New("managed runtime not started")
	}
	if closed {
		key := c.Series()
		previous := r.lastClosed[key]
		if !previous.IsZero() && c.OpenTime.After(previous.Add(c.Timeframe.Duration())) && !r.AllowGaps {
			r.failure = fmt.Errorf("managed candle gap for %s %s; restart with complete warmup before resuming", c.Symbol, c.Timeframe)
			return r.failure
		}
		if c.OpenTime.After(previous) {
			r.lastClosed[key] = c.OpenTime
		}
	}
	if closed && r.clock().Sub(c.CloseTime()) > c.Timeframe.Duration() {
		for _, e := range r.engines {
			e.Warmup(c)
		}
		return nil
	}
	if r.failure != nil {
		return r.failure
	}
	if c.Close.IsPositive() {
		r.marks[c.Symbol] = c.Close
	}
	interval := r.MarkInterval
	if interval <= 0 {
		interval = DefaultMarkInterval
	}
	persist := closed || r.clock().Sub(r.markedAt[c.Symbol]) >= interval
	if persist {
		if err := r.repo.Update(ctx, nonce(), "mark", c, func(s *State) error {
			s.Mark(c.Symbol, c.Close)
			s.PriceAt[c.Symbol] = c.CloseTime()
			s.Observe(r.clock())
			return nil
		}); err != nil {
			return err
		}
		r.markedAt[c.Symbol] = r.clock()
	}
	if sim, ok := r.physical.(*broker.Sim); ok && c.Timeframe == "1m" {
		if err := sim.MatchCandle(c, func() error { return r.drain(ctx) }); err != nil {
			return err
		}
	}
	if err := r.drain(ctx); err != nil {
		return err
	}
	if persist {
		if err := r.repo.Update(ctx, nonce(), "mark", nil, func(s *State) error { s.Observe(r.clock()); return nil }); err != nil {
			return err
		}
	}
	s, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	for _, id := range sortedBindings(s) {
		if err = r.syncView(ctx, id); err != nil {
			return err
		}
		e := r.engines[id]
		if r.mustClose(s, id) {
			e.CloseAll(ctx, "parent risk limit")
		} else if closed {
			e.OnCandle(ctx, c)
		} else {
			e.OnPrice(ctx, c)
		}
		if err = r.save(ctx, id); err != nil {
			return err
		}
		if err = r.drain(ctx); err != nil {
			return err
		}
	}
	return r.failure
}
func (r *Runtime) Tick(ctx context.Context, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.ready {
		return nil
	}
	if r.failure != nil {
		return r.failure
	}
	if err := r.drain(ctx); err != nil {
		return err
	}
	if err := r.repo.Update(ctx, nonce(), "mark", nil, func(s *State) error { s.Observe(r.clock()); return nil }); err != nil {
		return err
	}
	s, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	for _, id := range sortedBindings(s) {
		if err = r.syncView(ctx, id); err != nil {
			return err
		}
		if r.mustClose(s, id) {
			r.engines[id].CloseAll(ctx, "parent risk limit")
		} else {
			r.engines[id].Tick(ctx, now)
		}
		if err = r.save(ctx, id); err != nil {
			return err
		}
		if err = r.drain(ctx); err != nil {
			return err
		}
	}
	if err = r.repo.Update(ctx, nonce(), "sample", nil, func(s *State) error { s.Sample(now); return nil }); err != nil {
		return err
	}
	if hour := now.UTC().Truncate(time.Hour); hour.After(r.compacted) {
		keep := r.Retention
		if keep <= 0 {
			keep = DefaultRetention
		}
		if err = r.repo.Update(ctx, nonce(), "compact", nil, func(s *State) error { s.Compact(now, keep); return nil }); err != nil {
			return err
		}
		r.compacted = hour
	}
	if err = r.finalize(ctx); err != nil {
		return err
	}
	return r.failure
}
func (r *Runtime) save(ctx context.Context, id string) error {
	raw, err := r.engines[id].Checkpoint()
	if err != nil {
		return err
	}
	return r.checkpoint(ctx, id, raw)
}
func (r *Runtime) stage(ctx context.Context) error {
	for n := 0; n < 10000; n++ {
		r.qmu.Lock()
		if len(r.events) == 0 {
			r.qmu.Unlock()
			return nil
		}
		ev := r.events[0]
		r.events = r.events[1:]
		r.qmu.Unlock()
		err := r.repo.Update(ctx, "event/"+Hash(ev), "execution", ev, func(s *State) error {
			if s.Orders[ev.Order.ClientID] == nil && s.Orders[ev.Fill.OrderClientID] == nil {
				s.Attention = "execution without known owner"
				s.Note("reconcile", "account", s.Attention)
				return nil
			}
			if ev.Kind == broker.EventFill {
				applied, err := s.Fill(ev.Fill)
				if err != nil {
					return err
				}
				if !applied {
					return nil
				}
			} else {
				s.UpdateOrder(ev.Order)
				if o := s.Orders[ev.Order.ClientID]; o != nil && o.Deferred != nil {
					return nil
				}
			}
			s.Pending = append(s.Pending, ev)
			return nil
		})
		if err != nil {
			r.qmu.Lock()
			r.events = append([]broker.Event{ev}, r.events...)
			r.qmu.Unlock()
			r.failure = err
			return err
		}
	}
	return errors.New("execution staging exceeded safety bound")
}
func (r *Runtime) drain(ctx context.Context) error {
	for n := 0; n < 10000; n++ {
		if err := r.stage(ctx); err != nil {
			return err
		}
		s, err := r.repo.Load(ctx)
		if err != nil {
			return err
		}
		if len(s.Pending) == 0 {
			// A late execution can arrive after its entry plan was canceled.
			// Rebase only after the inbox is empty, then enforce the native-stop
			// policy of directional strategies instead of leaving orphan coins.
			for _, owner := range sortedBindings(s) {
				b := s.Bindings[owner]
				if b.Spec.Name != "trendspot" && b.Spec.Name != "emacross" {
					continue
				}
				r.syncState(s, owner)
				if len(r.engines[owner].Unprotected()) > 0 {
					r.engines[owner].CloseAll(ctx, "recovered position lacks required protection")
					if err := r.save(ctx, owner); err != nil {
						return err
					}
					// Closing can reserve funds or stage an execution for the next owner.
					s, err = r.repo.Load(ctx)
					if err != nil {
						return err
					}
				}
			}
			r.qmu.Lock()
			more := len(r.events) > 0
			r.qmu.Unlock()
			if more {
				continue
			}
			return r.failure
		}
		if err = r.deliver(ctx, s.Pending[0]); err != nil {
			return err
		}
	}
	return errors.New("execution loop exceeded safety bound")
}
func (r *Runtime) deliver(ctx context.Context, ev broker.Event) error {
	s, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	id := ev.Order.ClientID
	if ev.Kind == broker.EventFill {
		id = ev.Fill.OrderClientID
	}
	o := s.Orders[id]
	if o == nil {
		return errors.New("pending execution without owner")
	}
	h := r.routes[o.Owner]
	if h == nil {
		return errors.New("missing event route")
	}
	h(ev)
	laterFill := false
	for _, next := range s.Pending[1:] {
		if next.Kind == broker.EventFill && next.Fill.OrderClientID == id {
			laterFill = true
			break
		}
	}
	if ev.Kind == broker.EventFill && !laterFill && o.State == "terminal" && o.Order.Status != domain.OrderFilled {
		h(broker.Event{Kind: broker.EventOrder, Order: o.Order})
	}
	raw, err := r.engines[o.Owner].Checkpoint()
	if err != nil {
		return err
	}
	err = r.repo.Update(ctx, nonce(), "event_checkpoint", map[string]any{"owner": o.Owner, "event": ev}, func(s *State) error {
		if len(s.Pending) == 0 {
			return errors.New("durable inbox empty")
		}
		s.Bindings[o.Owner].Engine = raw
		s.Pending = s.Pending[1:]
		return nil
	})
	if err == nil && ev.Kind == broker.EventFill && r.OnExecution != nil {
		for _, execution := range s.Executions {
			if execution.Fill.OrderClientID == id && execution.Fill.TradeID == ev.Fill.TradeID {
				r.OnExecution(execution, o.Order)
				break
			}
		}
	}
	return err
}
func (r *Runtime) Reconcile(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reconcile(ctx)
}
func (r *Runtime) reconcile(ctx context.Context) error {
	live, ok := r.physical.(broker.Live)
	if !ok {
		return nil
	}
	if err := r.drain(ctx); err != nil {
		return err
	}
	s, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	var tracked []domain.Order
	for _, o := range s.Orders {
		if o.State != "terminal" {
			tracked = append(tracked, o.Order)
		}
	}
	st, err := live.AccountState(ctx, domain.Spot)
	if err != nil {
		return err
	}
	live.SyncOrders(ctx, tracked, st.OpenOrders)
	if err = r.drain(ctx); err != nil {
		return err
	}
	return r.repo.Update(ctx, nonce(), "reconcile", nil, func(s *State) error { s.Reconcile(st, r.instruments); return nil })
}

// scopedBroker routes orders to one physical broker. Event dispatch is done by
// Runtime, so strategies using the same symbol never share an engine position.
type scopedBroker struct {
	r     *Runtime
	owner string
}

func (b *scopedBroker) Exchange() string            { return b.r.physical.Exchange() }
func (b *scopedBroker) SetHandler(h broker.Handler) { b.r.routes[b.owner] = h }
func (b *scopedBroker) PlaceOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	if err := b.r.stage(ctx); err != nil {
		return o, err
	}
	if b.r.failure != nil {
		return o, b.r.failure
	}
	if o.Side == domain.Buy {
		st, err := b.r.repo.Load(ctx)
		if err != nil {
			return o, err
		}
		at := st.PriceAt[o.Symbol]
		if at.IsZero() || b.r.clock().Sub(at) > 2*time.Minute {
			return o, &exchange.APIError{Exchange: o.Exchange, HTTPStatus: 400, Code: -1013, Message: "fresh market price unavailable; entry held"}
		}
	}
	if in, ok := b.r.instruments[o.Symbol]; !ok {
		return o, &exchange.APIError{Exchange: o.Exchange, HTTPStatus: 400, Code: -1013, Message: "unknown instrument"}
	} else {
		price := o.Price
		if !price.IsPositive() {
			st, err := b.r.repo.Load(ctx)
			if err != nil {
				return o, err
			}
			price = st.Prices[o.Symbol]
		}
		if !in.RoundQty(o.Quantity).Equal(o.Quantity) || in.CheckOrder(price, o.Quantity) != nil {
			return o, &exchange.APIError{Exchange: o.Exchange, HTTPStatus: 400, Code: -1013, Message: "order below instrument quantity or notional rules"}
		}
	}
	if err := b.r.repo.Update(ctx, "reserve/"+o.ClientID, "reserve", o, func(s *State) error { return s.ReserveOrder(b.owner, o, "", b.r.fee) }); err != nil {
		return o, err
	}
	placed, err := b.r.physical.PlaceOrder(ctx, o)
	if err != nil {
		if definitive(err) {
			b.r.persist(ctx, nonce(), "submit_rejected", o.ClientID, func(s *State) error { s.Reject(o.ClientID, "exchange rejected order"); return nil })
		} else {
			b.r.persist(ctx, nonce(), "submit_unknown", o.ClientID, func(s *State) error {
				s.Attention = "order submission failed; query exchange before retry"
				s.Orders[o.ClientID].State = "unknown"
				return nil
			})
			b.r.failure = fmt.Errorf("managed submission uncertain: %w", err)
		}
	} else {
		err = b.r.repo.Update(ctx, nonce(), "accepted", placed.ClientID, func(s *State) error {
			tracked := s.Orders[o.ClientID]
			tracked.Order.ExchangeID = placed.ExchangeID
			if tracked.State == "submitting" {
				tracked.State = "open"
			}
			return nil
		})
		if err != nil {
			b.r.failure = err
		}
	}

	return placed, err
}
func (b *scopedBroker) CancelOrder(ctx context.Context, o domain.Order) error {
	if err := b.r.repo.Update(ctx, nonce(), "cancel_requested", o.ClientID, func(s *State) error {
		if old := s.Orders[o.ClientID]; old != nil && old.State != "terminal" {
			old.State = "canceling"
		}
		return nil
	}); err != nil {
		return err
	}
	err := b.r.physical.CancelOrder(ctx, o)
	if live, ok := b.r.physical.(broker.Live); ok && (err == nil || unknownOrder(err)) {
		// Exchanges acknowledge a cancel on the user stream, possibly after the
		// engine already submits the exit or replacement bracket that needs the
		// released coins. Query the final state now so the ledger can settle it.
		if err := b.r.stage(ctx); err != nil {
			return err
		}
		st, loadErr := b.r.repo.Load(ctx)
		if loadErr != nil {
			return loadErr
		}
		if tracked := st.Orders[o.ClientID]; tracked != nil && tracked.State != "terminal" {
			live.SyncOrders(ctx, []domain.Order{tracked.Order}, nil)
		}
	}
	return err
}

// unknownOrder reports Binance -2011/-2013: the order is no longer open, e.g.
// the other leg of an already canceled OCO list.
func unknownOrder(err error) bool {
	var api *exchange.APIError
	return errors.As(err, &api) && (api.Code == -2011 || api.Code == -2013)
}
func (b *scopedBroker) LinkedBracket(m domain.Market) bool {
	if linked, ok := b.r.physical.(broker.LinkedBrackets); ok {
		return linked.LinkedBracket(m)
	}
	return false
}
func (b *scopedBroker) PlaceBracket(ctx context.Context, stop, target domain.Order) ([]domain.Order, error) {
	if err := b.r.stage(ctx); err != nil {
		return nil, err
	}
	group := "oco/" + stop.ClientID
	err := b.r.repo.Update(ctx, group, "reserve_bracket", []domain.Order{stop, target}, func(s *State) error {
		for _, o := range []domain.Order{stop, target} {
			if o.ClientID != "" {
				if err := s.ReserveOrder(b.owner, o, group, b.r.fee); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	bp, ok := b.r.physical.(broker.BracketPlacer)
	var out []domain.Order
	if ok {
		out, err = bp.PlaceBracket(ctx, stop, target)
	} else {
		for _, o := range []domain.Order{stop, target} {
			if o.ClientID != "" {
				var placed domain.Order
				placed, err = b.r.physical.PlaceOrder(ctx, o)
				if err != nil {
					break
				}
				out = append(out, placed)
			}
		}
	}
	if err != nil {
		if definitive(err) {
			accepted := map[string]bool{}
			for _, o := range out {
				accepted[o.ClientID] = true
			}
			b.r.persist(ctx, nonce(), "bracket_rejected", group, func(s *State) error {
				for _, o := range []domain.Order{stop, target} {
					if o.ClientID != "" && !accepted[o.ClientID] {
						s.Reject(o.ClientID, "exchange rejected bracket")
					}
				}
				return nil
			})
		} else {
			b.r.persist(ctx, nonce(), "bracket_unknown", group, func(s *State) error { s.Attention = "bracket submission requires reconciliation"; return nil })
			b.r.failure = err
		}
	}
	if len(out) > 0 {
		// Accepted legs are open, as for single orders; brokers need not
		// report NEW on their stream for the ledger to know that.
		b.r.persist(ctx, nonce(), "accepted", group, func(s *State) error {
			for _, placed := range out {
				if tracked := s.Orders[placed.ClientID]; tracked != nil && tracked.State == "submitting" {
					tracked.Order.ExchangeID = placed.ExchangeID
					tracked.State = "open"
				}
			}
			return nil
		})
	}
	return out, err
}

func definitive(err error) bool {
	var api *exchange.APIError
	return errors.As(err, &api) && api.HTTPStatus < 500 && api.Code != -1007
}

func (r *Runtime) mustClose(s *State, owner string) bool {
	b := s.Bindings[owner]
	i := s.Instances[b.InstanceID]
	return (s.Halted && effectiveRisk(s.Spec.Risk).CloseOnKill) || (i.Halted && effectiveRisk(i.Risk).CloseOnKill) || (b.Halted && effectiveRisk(b.Spec.Risk).CloseOnKill)
}

// Financial error handling must itself be durable. A failed reject/cancel write
// stops the coordinator rather than silently continuing with an old reservation.
func (r *Runtime) persist(ctx context.Context, id, op string, payload any, fn func(*State) error) {
	if err := r.repo.Update(ctx, id, op, payload, fn); err != nil {
		r.failure = err
	}
}

// End records process termination without canceling or selling owned assets.
func (r *Runtime) End(ctx context.Context, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.repo.Update(ctx, "run-end/"+r.runID, "run_end", status, func(s *State) error {
		for n := range s.Runs {
			if s.Runs[n].ID == r.runID {
				s.Runs[n].Status = status
				s.Runs[n].EndedAt = r.clock()
			}
		}
		return nil
	})
}
