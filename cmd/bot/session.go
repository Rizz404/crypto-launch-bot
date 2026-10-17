package main

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/app"
	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/engine"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/recorder"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
)

const (
	warmupBars       = 300
	snapshotInterval = time.Minute
)

// session is one trading run of the bot: engine, accounts, brokers and the
// recorder that persists them.
type session struct {
	cfg        *config.Config
	log        *slog.Logger
	store      *storage.Store
	book       *portfolio.Book
	risk       *risk.Manager
	eng        *engine.Engine
	rec        *recorder.Recorder
	recCancel  context.CancelFunc
	sims       map[string]*broker.Sim // paper brokers by exchange name
	lives      map[string]broker.Live // testnet/live brokers by exchange name
	liveWG     sync.WaitGroup         // live broker goroutines
	liveCancel context.CancelFunc     // stops the live broker goroutines
	resync     chan struct{}          // reconcile requests from live brokers
	mismatch   map[domain.PositionKey]int
	quotes     map[portfolio.AccountKey]string
	leverages  map[domain.PositionKey]int
	instrument map[domain.PositionKey]domain.Instrument
	strategies []strategy.Config
	runID      int64
	notify     func(level slog.Level, msg string, attrs ...any) // optional, set before start
}

// newSession builds the trading side of the bot for the configured mode.
func newSession(ctx context.Context, cfg *config.Config, store *storage.Store, strategies []strategy.Config, log *slog.Logger) (*session, error) {
	mode := cfg.App.Mode
	paper := mode == config.ModePaper
	s := &session{
		cfg: cfg, log: log, store: store, book: portfolio.NewBook(), strategies: strategies,
		sims: map[string]*broker.Sim{}, lives: map[string]broker.Live{}, resync: make(chan struct{}, 1),
		mismatch: map[domain.PositionKey]int{}, quotes: map[portfolio.AccountKey]string{},
		leverages: map[domain.PositionKey]int{}, instrument: map[domain.PositionKey]domain.Instrument{},
	}

	var prev *storage.SessionState
	if paper && cfg.Paper.Resume {
		var err error
		if prev, err = store.LastSessionState(ctx, string(mode)); err != nil {
			return nil, err
		}
	}

	ids := make([]string, len(strategies))
	for i, sc := range strategies {
		ids[i] = sc.ID
	}
	runID, err := store.CreateRun(ctx, storage.Run{InstanceID: "session", Strategy: "engine", Mode: string(mode),
		Params: map[string]any{"strategies": ids, "risk": cfg.Risk}})
	if err != nil {
		return nil, err
	}
	s.runID = runID
	recCtx, recCancel := context.WithCancel(context.Background())
	s.rec, s.recCancel = recorder.New(store, runID, log), recCancel
	go s.rec.Run(recCtx)

	s.risk = risk.NewManager(cfg.Risk, log)
	s.eng = engine.New(engine.Config{
		Book: s.book, Risk: s.risk, Log: log, Clock: time.Now, IDSeed: time.Now().UnixMilli(),
		Hooks: engine.Hooks{
			OnOrder:  s.rec.Order,
			OnFill:   func(f domain.Fill, _ domain.Order) { s.rec.Fill(f) },
			OnNotice: s.onNotice,
		},
	})

	// Live brokers serve every market traded on their exchange.
	marketsByExchange := map[string][]domain.Market{}
	for _, sc := range strategies {
		if !slices.Contains(marketsByExchange[sc.Exchange], sc.Market) {
			marketsByExchange[sc.Exchange] = append(marketsByExchange[sc.Exchange], sc.Market)
		}
	}

	// One account per exchange + market traded.
	for _, sc := range strategies {
		key := portfolio.AccountKey{Exchange: sc.Exchange, Market: sc.Market}
		ins, err := store.Instruments(ctx, key.Exchange, key.Market)
		if err != nil {
			return nil, err
		}
		s.eng.SetInstruments(ins)
		for _, in := range ins {
			if slices.Contains(sc.Symbols, in.Symbol) {
				pk := domain.PositionKey{Exchange: key.Exchange, Market: key.Market, Symbol: in.Symbol}
				s.instrument[pk] = in
				s.leverages[pk] = s.risk.Leverage(sc.Leverage, key.Market)
				s.quotes[key] = in.Quote
			}
		}
		if s.book.Account(key) != nil {
			continue
		}

		if !paper {
			// Live balances come from the exchange on the first reconcile.
			s.book.Add(portfolio.NewAccount(key, decimal.Zero, s.onTrade))
			if _, ok := s.lives[sc.Exchange]; !ok {
				base := strings.TrimSuffix(sc.Exchange, "-testnet")
				lb, err := app.LiveBroker(cfg, base, marketsByExchange[sc.Exchange], log)
				if err != nil {
					return nil, err
				}
				if lb.Exchange() != sc.Exchange {
					return nil, fmt.Errorf("broker name %s does not match account %s", lb.Exchange(), sc.Exchange)
				}
				s.lives[sc.Exchange] = lb
				s.eng.AddBroker(lb)
			}
			log.Info("live account", "account", key, "mode", mode)
			continue
		}

		balance := decimal.NewFromFloat(cfg.Paper.InitialBalance)
		if prev != nil {
			if b, ok := prev.Balances[key.String()]; ok {
				balance = b
			}
		}
		s.book.Add(portfolio.NewAccount(key, balance, s.onTrade))
		if _, ok := s.sims[sc.Exchange]; !ok {
			sim := broker.NewSim(broker.SimConfig{
				Exchange: sc.Exchange, Fees: app.Fees(cfg.Backtest.Fees), SlippageBps: cfg.Backtest.SlippageBps, Clock: time.Now,
			})
			s.sims[sc.Exchange] = sim
			s.eng.AddBroker(sim)
		}
		log.Info("paper account", "account", key, "balance", balance)
	}

	for _, sc := range strategies {
		if err := s.eng.AddStrategy(sc); err != nil {
			return nil, err
		}
		log.Info("strategy loaded", "id", sc.ID, "name", sc.Name, "account", sc.Exchange+":"+string(sc.Market), "symbols", sc.Symbols)
	}

	if prev != nil {
		if err := s.resume(ctx, prev); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// resume restores the positions and open orders of the previous session.
func (s *session) resume(ctx context.Context, prev *storage.SessionState) error {
	for _, p := range prev.Positions {
		if acct := s.book.Account(portfolio.AccountKey{Exchange: p.Exchange, Market: p.Market}); acct != nil {
			acct.SetPosition(p)
			s.log.Info("position restored", "symbol", p.Symbol, "side", p.Side, "qty", p.Quantity, "entry", p.EntryPrice)
		}
	}
	orders, err := s.store.OpenOrders(ctx, prev.RunID)
	if err != nil {
		return err
	}
	var adopted []domain.Order
	for _, o := range orders {
		sim, ok := s.sims[o.Exchange]
		if !ok {
			continue
		}
		accepted, err := sim.PlaceOrder(ctx, o)
		if err != nil {
			s.log.Warn("could not restore order", "client_id", o.ClientID, "err", err)
			continue
		}
		accepted.Tag, accepted.Strategy = o.Tag, o.Strategy
		adopted = append(adopted, accepted)
	}
	s.eng.AdoptOrders(adopted)
	for _, p := range s.eng.Unprotected() {
		s.onNotice(slog.LevelWarn, "restored position has no stop loss", "symbol", p.Symbol, "side", p.Side)
	}
	s.log.Info("previous paper session resumed", "run_id", prev.RunID, "positions", len(prev.Positions), "orders", len(adopted))
	return nil
}

// Subscriptions returns the series the strategies need, per market, keyed
// by exchange name.
func (s *session) Subscriptions() map[portfolio.AccountKey]map[domain.SeriesKey]bool {
	out := map[portfolio.AccountKey]map[domain.SeriesKey]bool{}
	tfs := s.eng.Timeframes()
	for _, sc := range s.strategies {
		key := portfolio.AccountKey{Exchange: sc.Exchange, Market: sc.Market}
		if out[key] == nil {
			out[key] = map[domain.SeriesKey]bool{}
		}
		for _, sym := range sc.Symbols {
			for _, tf := range tfs {
				out[key][domain.SeriesKey{Exchange: sc.Exchange, Market: sc.Market, Symbol: sym, Timeframe: tf}] = true
			}
		}
	}
	return out
}

// warmup replays recent stored candles so indicators are ready. The engine
// drops intents from these stale candles.
func (s *session) warmup(ctx context.Context) error {
	var all []domain.Candle
	for _, series := range s.Subscriptions() {
		for key := range series {
			cs, err := s.store.RecentCandles(ctx, key, warmupBars)
			if err != nil {
				return err
			}
			all = append(all, cs...)
		}
	}
	slices.SortFunc(all, func(a, b domain.Candle) int {
		return cmp.Or(a.CloseTime().Compare(b.CloseTime()), cmp.Compare(a.Timeframe.Duration(), b.Timeframe.Duration()))
	})
	for _, c := range all {
		s.eng.OnCandle(ctx, c)
	}
	s.log.Info("strategies warmed up", "candles", len(all))
	return nil
}

func (s *session) onCandle(ctx context.Context, c domain.Candle) {
	if sim := s.sims[c.Exchange]; sim != nil {
		sim.OnCandle(c)
	}
	s.eng.OnCandle(ctx, c)
}

func (s *session) onUpdate(ctx context.Context, c domain.Candle) {
	if sim := s.sims[c.Exchange]; sim != nil {
		// A live update is a price tick at its arrival time.
		now := time.Now()
		sim.OnCandle(domain.Candle{
			Exchange: c.Exchange, Market: c.Market, Symbol: c.Symbol, Timeframe: c.Timeframe,
			OpenTime: now, Open: c.Close, High: c.Close, Low: c.Close, Close: c.Close,
		})
	}
	s.eng.OnPrice(ctx, c)
}

// loop runs timers until ctx is cancelled.
func (s *session) loop(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	snap := time.NewTicker(snapshotInterval)
	defer snap.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			s.eng.Tick(ctx, now)
		case now := <-snap.C:
			s.reconcile(ctx, false)
			s.snapshot(ctx, now)
		case <-s.resync:
			s.reconcile(ctx, false)
		}
	}
}

func (s *session) snapshot(ctx context.Context, now time.Time) {
	s.eng.ObserveRisk(ctx)
	for _, acct := range s.book.Accounts() {
		s.rec.Account(acct, now)
	}
}

// close records a final snapshot, flushes the recorder and finishes the run.
func (s *session) close(reason string) {
	if s.liveCancel != nil {
		s.liveCancel()
	}
	s.liveWG.Wait()
	s.snapshot(context.Background(), time.Now())
	s.recCancel()
	s.rec.Wait()
	total := s.book.Total()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	summary := map[string]any{"equity": total.Equity, "balance": total.Balance, "open_positions": total.OpenPositions}
	if err := s.store.FinishRun(ctx, s.runID, reason, summary); err != nil {
		s.log.Error("finish run", "err", err)
	}
}

func (s *session) onTrade(t domain.Trade) {
	s.rec.Trade(t)
	s.log.Info("trade closed", "strategy", t.Strategy, "symbol", t.Symbol, "side", t.Side, "qty", t.Quantity,
		"entry", t.EntryPrice, "exit", t.ExitPrice, "net_pnl", t.NetPnL().StringFixed(4), "reason", t.ExitReason)
}

func (s *session) onNotice(level slog.Level, msg string, attrs ...any) {
	if s.notify != nil {
		s.notify(level, msg, attrs...)
	}
}
