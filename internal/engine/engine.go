// Package engine connects strategies, risk and brokers. It routes candles
// to strategies, turns their intents into orders and manages the order
// lifecycle: entries, protective stop loss / take profit, trailing stops,
// exits and reversals.
//
// The same engine runs backtests (with the simulated broker), paper trading
// and live trading.
package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
)

// Hooks observe the engine; all are optional and are called with the
// engine lock held, so they must not call back into the engine.
type Hooks struct {
	OnOrder  func(domain.Order)                                   // every order state change
	OnFill   func(domain.Fill, domain.Order)                      // every execution
	OnTrade  func(domain.Trade)                                   // every closed round trip
	OnNotice func(level slog.Level, msg string, attrs ...any)     // alerts for humans
	OnPlaced func(exchange string, took time.Duration, err error) // broker call latency
}

type Config struct {
	Book  *portfolio.Book
	Risk  *risk.Manager
	Log   *slog.Logger
	Clock func() time.Time
	Hooks Hooks
	// IDSeed starts the client order id sequence. Use a time-based seed in
	// live trading so ids never repeat across restarts.
	IDSeed int64
}

type instKey struct {
	exchange string
	market   domain.Market
	symbol   string
}

type Engine struct {
	cfg Config
	log *slog.Logger

	mu          sync.Mutex
	brokers     map[string]broker.Broker
	instruments map[instKey]domain.Instrument
	lastPrice   map[instKey]decimal.Decimal
	instances   []*instance
	orders      map[string]*domain.Order // open orders by client id
	refs        map[string]string        // strategy/ref -> client id
	entries     map[domain.PositionKey]*entryPlan
	exits       map[domain.PositionKey]string // pending exit order id
	protections map[domain.PositionKey]*protection
	leverage    map[domain.PositionKey]int
	leverageSet map[domain.PositionKey]int
	seq         int64
	paused      bool

	qmu   sync.Mutex
	queue []broker.Event
}

type instance struct {
	cfg       strategy.Config
	s         strategy.Strategy
	tfs       map[domain.Timeframe]bool
	symbols   map[string]bool
	idPrefix  string
	nextTimer time.Time
}

// entryPlan is what to do once an entry order fills.
type entryPlan struct {
	orderID    string
	side       domain.PositionSide
	stopLoss   decimal.Decimal
	takeProfit decimal.Decimal
	trailing   float64
}

// protection tracks the protective orders of an open position.
type protection struct {
	stopID, targetID string
	stopPrice        decimal.Decimal
	takeProfit       decimal.Decimal
	trailing         float64
	best             decimal.Decimal // best price since entry, for trailing
}

func New(cfg Config) *Engine {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Engine{
		cfg:         cfg,
		log:         cfg.Log,
		brokers:     map[string]broker.Broker{},
		instruments: map[instKey]domain.Instrument{},
		lastPrice:   map[instKey]decimal.Decimal{},
		orders:      map[string]*domain.Order{},
		refs:        map[string]string{},
		entries:     map[domain.PositionKey]*entryPlan{},
		exits:       map[domain.PositionKey]string{},
		protections: map[domain.PositionKey]*protection{},
		leverage:    map[domain.PositionKey]int{},
		leverageSet: map[domain.PositionKey]int{},
		seq:         cfg.IDSeed,
	}
}

// AddBroker registers the broker of one exchange.
func (e *Engine) AddBroker(b broker.Broker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.brokers[b.Exchange()] = b
	b.SetHandler(e.onBrokerEvent)
}

// SetInstruments loads trading rules used for rounding and minimums.
func (e *Engine) SetInstruments(ins []domain.Instrument) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, in := range ins {
		e.instruments[instKey{in.Exchange, in.Market, in.Symbol}] = in
	}
}

// AddStrategy creates and initialises a strategy instance.
func (e *Engine) AddStrategy(cfg strategy.Config) error {
	s, err := strategy.New(cfg.Name)
	if err != nil {
		return err
	}
	if cfg.ID == "" {
		cfg.ID = cfg.Name
	}
	inst := &instance{cfg: cfg, s: s, tfs: map[domain.Timeframe]bool{}, symbols: map[string]bool{}, idPrefix: shortHash(cfg.ID)}
	for _, sym := range cfg.Symbols {
		inst.symbols[sym] = true
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, other := range e.instances {
		if other.cfg.ID == cfg.ID {
			return fmt.Errorf("duplicate strategy id %q", cfg.ID)
		}
	}
	if err := s.Init(&stratCtx{e: e, inst: inst}, cfg); err != nil {
		return fmt.Errorf("init strategy %s: %w", cfg.ID, err)
	}
	for _, tf := range s.Timeframes() {
		inst.tfs[tf] = true
	}
	e.instances = append(e.instances, inst)
	return nil
}

// Strategies returns the configs of all strategy instances.
func (e *Engine) Strategies() []strategy.Config {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]strategy.Config, len(e.instances))
	for i, inst := range e.instances {
		out[i] = inst.cfg
	}
	return out
}

// Timeframes returns the union of timeframes the strategies need.
func (e *Engine) Timeframes() []domain.Timeframe {
	e.mu.Lock()
	defer e.mu.Unlock()
	seen := map[domain.Timeframe]bool{}
	var out []domain.Timeframe
	for _, inst := range e.instances {
		for tf := range inst.tfs {
			if !seen[tf] {
				seen[tf] = true
				out = append(out, tf)
			}
		}
	}
	return out
}

// SetPaused stops (true) or resumes (false) new entries. Exits and
// protective orders keep working while paused.
func (e *Engine) SetPaused(p bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.paused = p
	e.notice(slog.LevelWarn, "engine paused state changed", "paused", p)
}

func (e *Engine) Paused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

// OnCandle processes a closed candle: marks positions, moves trailing
// stops and runs the strategies subscribed to it.
func (e *Engine) OnCandle(ctx context.Context, c domain.Candle) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)

	k := instKey{c.Exchange, c.Market, c.Symbol}
	e.lastPrice[k] = c.Close
	if acct := e.cfg.Book.Account(portfolio.AccountKey{Exchange: c.Exchange, Market: c.Market}); acct != nil {
		acct.Mark(c.Symbol, c.Close)
	}
	e.trail(ctx, c)

	// Candles that closed more than one bar ago come from warmup or gap
	// backfill: strategies still see them (indicators need history) but
	// their intents are dropped, since acting on stale signals is unsafe.
	stale := e.cfg.Clock().Sub(c.CloseTime()) > c.Timeframe.Duration()
	for _, inst := range e.instances {
		if inst.cfg.Exchange != c.Exchange || inst.cfg.Market != c.Market || !inst.symbols[c.Symbol] || !inst.tfs[c.Timeframe] {
			continue
		}
		intents := inst.s.OnCandle(c)
		if stale {
			if len(intents) > 0 {
				e.log.Debug("dropping intents from stale candle", "strategy", inst.cfg.ID, "symbol", c.Symbol, "close", c.CloseTime())
			}
			continue
		}
		e.apply(ctx, inst, intents)
	}
	e.observeRisk(ctx)
}

// ObserveRisk feeds the current equity to the kill switch.
func (e *Engine) ObserveRisk(ctx context.Context) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)
	e.observeRisk(ctx)
}

func (e *Engine) observeRisk(ctx context.Context) {
	tripped, reason := e.cfg.Risk.Observe(e.cfg.Clock(), e.cfg.Book.Total().Equity)
	if !tripped {
		return
	}
	e.paused = true
	e.notice(slog.LevelError, "KILL SWITCH: trading halted", "reason", reason, "close_positions", e.cfg.Risk.Config().CloseOnKill)
	if e.cfg.Risk.Config().CloseOnKill {
		e.closeAll(ctx, "kill_switch")
	}
}

// ResetKillSwitch clears the kill switch and resumes entries.
func (e *Engine) ResetKillSwitch() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg.Risk.Reset(e.cfg.Book.Total().Equity)
	e.paused = false
	e.notice(slog.LevelWarn, "kill switch reset, trading resumed")
}

// Kill trips the kill switch manually and closes all positions.
func (e *Engine) Kill(ctx context.Context, reason string) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)
	e.cfg.Risk.Halt(reason)
	e.paused = true
	e.notice(slog.LevelError, "KILL SWITCH: manual", "reason", reason)
	e.closeAll(ctx, "kill_switch")
}

// OnPrice processes a still-forming candle (live price update).
func (e *Engine) OnPrice(ctx context.Context, c domain.Candle) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)

	k := instKey{c.Exchange, c.Market, c.Symbol}
	e.lastPrice[k] = c.Close
	if acct := e.cfg.Book.Account(portfolio.AccountKey{Exchange: c.Exchange, Market: c.Market}); acct != nil {
		acct.Mark(c.Symbol, c.Close)
	}
	for _, inst := range e.instances {
		ph, ok := inst.s.(strategy.PriceHandler)
		if !ok || inst.cfg.Exchange != c.Exchange || inst.cfg.Market != c.Market || !inst.symbols[c.Symbol] {
			continue
		}
		e.apply(ctx, inst, ph.OnPrice(c))
	}
}

// Tick runs strategies that implement strategy.TimerHandler when due.
func (e *Engine) Tick(ctx context.Context, now time.Time) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)
	for _, inst := range e.instances {
		th, ok := inst.s.(strategy.TimerHandler)
		if !ok || now.Before(inst.nextTimer) {
			continue
		}
		inst.nextTimer = now.Add(th.Interval())
		e.apply(ctx, inst, th.OnTimer(now))
	}
}

// LastPrice returns the latest known price of a symbol.
func (e *Engine) LastPrice(exchange string, m domain.Market, symbol string) (decimal.Decimal, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	p, ok := e.lastPrice[instKey{exchange, m, symbol}]
	return p, ok
}

// OpenOrders returns copies of all open orders tracked by the engine.
func (e *Engine) OpenOrders() []domain.Order {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]domain.Order, 0, len(e.orders))
	for _, o := range e.orders {
		out = append(out, *o)
	}
	return out
}

// AdoptOrders starts tracking open orders that the engine did not place in
// this session (left by a previous session, or found on the exchange during
// reconciliation). Stop-loss and take-profit orders become the protection
// of their position again; trailing settings are not recovered.
func (e *Engine) AdoptOrders(orders []domain.Order) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, o := range orders {
		if !o.Status.Open() {
			continue
		}
		cp := o
		e.orders[o.ClientID] = &cp
		key := domain.PositionKey{Exchange: o.Exchange, Market: o.Market, Symbol: o.Symbol}
		switch o.Tag {
		case domain.TagStopLoss, domain.TagTakeProfit:
			p := e.protections[key]
			if p == nil {
				p = &protection{}
				e.protections[key] = p
			}
			if o.Tag == domain.TagStopLoss {
				p.stopID, p.stopPrice, p.best = o.ClientID, o.StopPrice, o.StopPrice
			} else {
				p.targetID, p.takeProfit = o.ClientID, o.StopPrice
			}
		}
	}
}

// Unprotected returns open positions that have no stop loss order.
func (e *Engine) Unprotected() []domain.Position {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []domain.Position
	for _, acct := range e.cfg.Book.Accounts() {
		for _, pos := range acct.Positions() {
			if p := e.protections[pos.PositionKey]; p == nil || p.stopID == "" {
				out = append(out, pos)
			}
		}
	}
	return out
}

// CloseAll exits every open position (kill switch, shutdown on demand).
func (e *Engine) CloseAll(ctx context.Context, reason string) {
	e.mu.Lock()
	defer e.unlockAndDrain(ctx)
	e.closeAll(ctx, reason)
}

func (e *Engine) closeAll(ctx context.Context, reason string) {
	for _, acct := range e.cfg.Book.Accounts() {
		for _, pos := range acct.Positions() {
			e.exit(ctx, pos.PositionKey, reason)
		}
	}
	for id, o := range e.orders {
		if o.Tag == domain.TagStrategy || o.Tag == domain.TagEntry {
			e.cancel(ctx, id)
		}
	}
}

// onBrokerEvent queues the event and processes it unless the engine is
// busy, in which case the current lock holder drains the queue.
func (e *Engine) onBrokerEvent(ev broker.Event) {
	e.qmu.Lock()
	e.queue = append(e.queue, ev)
	e.qmu.Unlock()
	if e.mu.TryLock() {
		e.unlockAndDrain(context.Background())
	}
}

// unlockAndDrain processes queued broker events, then releases the lock,
// re-acquiring it if events arrived in between.
func (e *Engine) unlockAndDrain(ctx context.Context) {
	for {
		for {
			e.qmu.Lock()
			if len(e.queue) == 0 {
				e.qmu.Unlock()
				break
			}
			ev := e.queue[0]
			e.queue = e.queue[1:]
			e.qmu.Unlock()
			e.handleEvent(ctx, ev)
		}
		e.mu.Unlock()

		e.qmu.Lock()
		pending := len(e.queue) > 0
		e.qmu.Unlock()
		if !pending || !e.mu.TryLock() {
			return
		}
	}
}

func (e *Engine) notice(level slog.Level, msg string, attrs ...any) {
	e.log.Log(context.Background(), level, msg, attrs...)
	if e.cfg.Hooks.OnNotice != nil {
		e.cfg.Hooks.OnNotice(level, msg, attrs...)
	}
}

// newClientID returns "tb" + a 7-character strategy hash + a base-36
// sequence: alphanumeric only (OKX rejects other characters), at most 32
// characters, and the fixed-width hash keeps ids of different strategies
// from colliding.
func (e *Engine) newClientID(inst *instance) string {
	e.seq++
	prefix := "tb0000000"
	if inst != nil {
		prefix = "tb" + inst.idPrefix
	}
	return prefix + strconv.FormatInt(e.seq, 36)
}

func shortHash(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%07s", strconv.FormatUint(uint64(h.Sum32()), 36))
}

func (e *Engine) instanceByID(id string) *instance {
	for _, inst := range e.instances {
		if inst.cfg.ID == id {
			return inst
		}
	}
	return nil
}

func (e *Engine) account(k domain.PositionKey) *portfolio.Account {
	return e.cfg.Book.Account(portfolio.AccountKey{Exchange: k.Exchange, Market: k.Market})
}

var errNoBroker = fmt.Errorf("no broker for exchange")
