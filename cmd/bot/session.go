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
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/metrics"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
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
	cfg            *config.Config
	log            *slog.Logger
	store          *storage.Store
	book           *portfolio.Book
	risk           *risk.Manager
	eng            *engine.Engine
	rec            *recorder.Recorder
	recCancel      context.CancelFunc
	sims           map[string]*broker.Sim // paper brokers by exchange name
	lives          map[string]broker.Live // testnet/live brokers by exchange name
	liveWG         sync.WaitGroup         // live broker goroutines
	liveCancel     context.CancelFunc     // stops the live broker goroutines
	resync         chan struct{}          // reconcile requests from live brokers
	mismatch       map[domain.PositionKey]int
	quotes         map[portfolio.AccountKey]string
	leverages      map[domain.PositionKey]int
	instrument     map[domain.PositionKey]domain.Instrument
	strategies     []strategy.Config
	runID          int64
	metrics        *metrics.Trading
	tg             *telegram.Bot // optional
	notifyMin      slog.Level
	fundingSources map[string]exchange.FundingSource
	realized       map[string]decimal.Decimal // net realized PnL per strategy, this session
	day            time.Time                  // UTC day of the last daily summary
}

// newSession builds the trading side of the bot for the configured mode.
func newSession(ctx context.Context, cfg *config.Config, store *storage.Store, strategies []strategy.Config,
	m *metrics.Trading, tg *telegram.Bot, log *slog.Logger,
) (*session, error) {
	mode := cfg.App.Mode
	paper := mode == config.ModePaper
	s := &session{
		cfg: cfg, log: log, store: store, book: portfolio.NewBook(), strategies: strategies,
		sims: map[string]*broker.Sim{}, lives: map[string]broker.Live{}, resync: make(chan struct{}, 1),
		mismatch: map[domain.PositionKey]int{}, quotes: map[portfolio.AccountKey]string{},
		leverages: map[domain.PositionKey]int{}, instrument: map[domain.PositionKey]domain.Instrument{},
		metrics: m, tg: tg, realized: map[string]decimal.Decimal{}, day: time.Now().UTC().Truncate(24 * time.Hour),
	}
	_ = s.notifyMin.UnmarshalText([]byte(cfg.Telegram.NotifyLevel))

	var prev *storage.SessionState
	// Live spot positions are the bot's own fills and only the database
	// knows them; the wallet may hold more of the same coin.
	if !paper || cfg.Paper.Resume {
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
		Book: s.book, Risk: s.risk, Log: log, Clock: time.Now, IDSeed: time.Now().UnixMilli(), HoldRisk: true,
		Hooks: engine.Hooks{
			OnOrder:  s.onOrder,
			OnFill:   s.onFill,
			OnNotice: s.onNotice,
			OnPlaced: m.ObservePlace,
		},
	})

	// Every instrument a strategy trades (its symbols and legs) needs an
	// account; live brokers serve all markets traded on their exchange.
	type traded struct {
		leg      strategy.Leg
		leverage int
	}
	var instruments []traded
	marketsByExchange := map[string][]domain.Market{}
	for _, sc := range strategies {
		for _, in := range sc.Instruments() {
			instruments = append(instruments, traded{in, sc.Leverage})
			if !slices.Contains(marketsByExchange[in.Exchange], in.Market) {
				marketsByExchange[in.Exchange] = append(marketsByExchange[in.Exchange], in.Market)
			}
		}
	}
	loaded := map[portfolio.AccountKey][]domain.Instrument{}
	for _, t := range instruments {
		key := portfolio.AccountKey{Exchange: t.leg.Exchange, Market: t.leg.Market}
		ins, ok := loaded[key]
		if !ok {
			var err error
			if ins, err = store.Instruments(ctx, key.Exchange, key.Market); err != nil {
				return nil, err
			}
			loaded[key] = ins
			s.eng.SetInstruments(ins)
		}
		for _, in := range ins {
			if in.Symbol == t.leg.Symbol {
				pk := domain.PositionKey{Exchange: key.Exchange, Market: key.Market, Symbol: in.Symbol}
				s.instrument[pk] = in
				s.leverages[pk] = s.risk.Leverage(t.leverage, key.Market)
				s.quotes[key] = in.Quote
			}
		}
	}

	for _, t := range instruments {
		exName := t.leg.Exchange
		key := portfolio.AccountKey{Exchange: exName, Market: t.leg.Market}
		if s.book.Account(key) != nil {
			continue
		}

		if !paper {
			// Live balances come from the exchange on the first reconcile.
			s.book.Add(portfolio.NewAccount(key, decimal.Zero, s.onTrade))
			if _, ok := s.lives[exName]; !ok {
				base := strings.TrimSuffix(exName, "-testnet")
				lb, err := app.LiveBroker(cfg, base, marketsByExchange[exName], log)
				if err != nil {
					return nil, err
				}
				if lb.Exchange() != exName {
					return nil, fmt.Errorf("broker name %s does not match account %s", lb.Exchange(), exName)
				}
				if uw, ok := lb.(broker.UnifiedWallet); ok && uw.UnifiedWallet() && len(marketsByExchange[exName]) > 1 {
					// One wallet backs both markets; separate ledgers would
					// count the same cash twice.
					return nil, fmt.Errorf("%s uses one wallet for spot and futures: trade a single market there (found %v)",
						base, marketsByExchange[exName])
				}
				s.lives[exName] = lb
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
		if _, ok := s.sims[exName]; !ok {
			sim := broker.NewSim(broker.SimConfig{
				Exchange: exName, Fees: app.Fees(cfg.Backtest.Fees), SlippageBps: cfg.Backtest.SlippageBps, Clock: time.Now,
			})
			s.sims[exName] = sim
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

	switch {
	case prev != nil && paper:
		if err := s.resume(ctx, prev); err != nil {
			return nil, err
		}
	case prev != nil:
		s.resumeLiveSpot(prev)
	}
	return s, nil
}

// resumeLiveSpot restores the spot positions of the previous live session.
// Reconciliation then caps them by the wallet. Futures positions and all
// open orders come from the exchange.
func (s *session) resumeLiveSpot(prev *storage.SessionState) {
	for _, p := range prev.Positions {
		if p.Market != domain.Spot || s.lives[p.Exchange] == nil {
			continue
		}
		if acct := s.book.Account(portfolio.AccountKey{Exchange: p.Exchange, Market: p.Market}); acct != nil {
			acct.SetPosition(p)
			s.log.Info("spot position restored", "run_id", prev.RunID, "symbol", p.Symbol, "qty", p.Quantity, "entry", p.EntryPrice)
		}
	}
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
		// Entry plans (ATR stop/target/trailing settings) are not persisted.
		// Replaying the remaining entry would open inventory without its
		// intended protection. Keep existing positions and native brackets,
		// but require a fresh strategy signal for any unfinished entry.
		if o.Tag == domain.TagEntry {
			o.Status, o.UpdatedAt = domain.OrderCanceled, time.Now()
			o.Reason = "paper restart: entry plan unavailable"
			s.rec.Order(o)
			s.notice(slog.LevelWarn, "pending paper entry canceled on restart", "client_id", o.ClientID, "symbol", o.Symbol)
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
		s.notice(slog.LevelWarn, "restored position has no stop loss", "symbol", p.Symbol, "side", p.Side)
	}
	s.log.Info("previous paper session resumed", "run_id", prev.RunID, "positions", len(prev.Positions), "orders", len(adopted))
	return nil
}

// Subscriptions returns the series the strategies need, per account.
func (s *session) Subscriptions() map[portfolio.AccountKey]map[domain.SeriesKey]bool {
	out := map[portfolio.AccountKey]map[domain.SeriesKey]bool{}
	tfs := s.eng.Timeframes()
	for _, sc := range s.strategies {
		for _, in := range sc.Instruments() {
			key := portfolio.AccountKey{Exchange: in.Exchange, Market: in.Market}
			if out[key] == nil {
				out[key] = map[domain.SeriesKey]bool{}
			}
			for _, tf := range tfs {
				out[key][domain.SeriesKey{Exchange: in.Exchange, Market: in.Market, Symbol: in.Symbol, Timeframe: tf}] = true
			}
		}
	}
	return out
}

// addSource registers a market data client for funding rate polling.
func (s *session) addSource(src exchange.MarketData) {
	if fs, ok := src.(exchange.FundingSource); ok {
		if s.fundingSources == nil {
			s.fundingSources = map[string]exchange.FundingSource{}
		}
		s.fundingSources[src.Name()] = fs
	}
}

// pollFunding refreshes funding rates of the perpetuals the strategies trade.
func (s *session) pollFunding(ctx context.Context) {
	wanted := map[string]map[string]bool{}
	for _, sc := range s.strategies {
		for _, in := range sc.Instruments() {
			if in.Market == domain.Futures {
				if wanted[in.Exchange] == nil {
					wanted[in.Exchange] = map[string]bool{}
				}
				wanted[in.Exchange][in.Symbol] = true
			}
		}
	}
	for name, symbols := range wanted {
		fs := s.fundingSources[name]
		if fs == nil {
			continue
		}
		rates, err := fs.FundingRates(ctx)
		if err != nil {
			s.log.Warn("funding rates", "exchange", name, "err", err)
			continue
		}
		for _, r := range rates {
			if symbols[r.Symbol] {
				s.eng.SetFunding(name, r.Symbol, strategy.FundingInfo{Rate: r.Rate, Next: r.Next})
			}
		}
	}
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
	gauges := time.NewTicker(10 * time.Second)
	defer gauges.Stop()
	funding := time.NewTicker(5 * time.Minute)
	defer funding.Stop()
	s.pollFunding(ctx)
	snap := time.NewTicker(snapshotInterval)
	defer snap.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			s.eng.Tick(ctx, now)
		case now := <-gauges.C:
			s.updateGauges()
			s.dailySummary(ctx, now)
		case now := <-snap.C:
			s.reconcile(ctx, false)
			s.snapshot(ctx, now)
		case <-s.resync:
			s.reconcile(ctx, false)
		case <-funding.C:
			s.pollFunding(ctx)
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

// onTrade runs under the account lock: no engine calls here.
func (s *session) onTrade(t domain.Trade) {
	s.rec.Trade(t)
	net := t.NetPnL()
	s.log.Info("trade closed", "strategy", t.Strategy, "symbol", t.Symbol, "side", t.Side, "qty", t.Quantity,
		"entry", t.EntryPrice, "exit", t.ExitPrice, "net_pnl", net.StringFixed(4), "reason", t.ExitReason)

	result := "loss"
	if net.IsPositive() {
		result = "win"
	}
	s.metrics.Trades.WithLabelValues(t.Strategy, result, t.ExitReason).Inc()
	s.realized[t.Strategy] = s.realized[t.Strategy].Add(net)
	pnl, _ := s.realized[t.Strategy].Float64()
	s.metrics.RealizedPnL.WithLabelValues(t.Strategy).Set(pnl)

	if s.tg != nil {
		icon := "🔴"
		if net.IsPositive() {
			icon = "🟢"
		}
		s.tg.Notify(fmt.Sprintf("%s <b>Closed %s %s</b> (%s)\nqty %s · %s → %s\nnet PnL <b>%s</b> · fees %s\nexit: %s",
			icon, t.Side, telegram.Escape(t.Symbol), telegram.Escape(t.Strategy), t.Quantity, t.EntryPrice, t.ExitPrice,
			net.StringFixed(2), t.Fees.StringFixed(2), telegram.Escape(t.ExitReason)))
	}
}

func (s *session) onOrder(o domain.Order) {
	s.rec.Order(o)
	s.metrics.Orders.WithLabelValues(o.Exchange+":"+string(o.Market), o.Tag, string(o.Status)).Inc()
}

func (s *session) onFill(f domain.Fill, o domain.Order) {
	s.rec.Fill(f)
	// Store the position now rather than at the next snapshot: after a
	// crash, live spot restarts rely on it to know which coins are the bot's.
	if acct := s.book.Account(portfolio.AccountKey{Exchange: f.Exchange, Market: f.Market}); acct != nil {
		s.rec.Account(acct, time.Now())
	}
	s.metrics.Fills.WithLabelValues(f.Exchange+":"+string(f.Market), o.Tag).Inc()
	if s.tg != nil && o.Tag == domain.TagEntry && o.Status == domain.OrderFilled {
		s.tg.Notify(fmt.Sprintf("🔵 <b>Opened %s %s</b> (%s)\nqty %s @ %s\n%s",
			strings.ToUpper(string(o.Side)), telegram.Escape(o.Symbol), telegram.Escape(o.Strategy),
			o.FilledQty, o.AvgPrice.StringFixed(4), telegram.Escape(o.Reason)))
	}
}

// notice logs a session event and forwards it like an engine notice
// (engine notices are already logged by the engine).
func (s *session) notice(level slog.Level, msg string, attrs ...any) {
	s.log.Log(context.Background(), level, msg, attrs...)
	s.onNotice(level, msg, attrs...)
}

func (s *session) onNotice(level slog.Level, msg string, attrs ...any) {
	s.metrics.Notices.WithLabelValues(strings.ToLower(level.String())).Inc()
	if s.tg == nil || level < s.notifyMin {
		return
	}
	icon := "ℹ️"
	switch {
	case level >= slog.LevelError:
		icon = "🚨"
	case level >= slog.LevelWarn:
		icon = "⚠️"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s <b>%s</b>", icon, telegram.Escape(msg))
	for i := 0; i+1 < len(attrs); i += 2 {
		fmt.Fprintf(&sb, "\n%v: %s", attrs[i], telegram.Escape(fmt.Sprint(attrs[i+1])))
	}
	s.tg.Notify(sb.String())
}

func (s *session) updateGauges() {
	for _, acct := range s.book.Accounts() {
		snap := acct.Snapshot()
		k := acct.Key.String()
		f := func(d decimal.Decimal) float64 { v, _ := d.Float64(); return v }
		s.metrics.Equity.WithLabelValues(k).Set(f(snap.Equity))
		s.metrics.Balance.WithLabelValues(k).Set(f(snap.Balance))
		s.metrics.Unrealized.WithLabelValues(k).Set(f(snap.UnrealizedPnL))
		s.metrics.Exposure.WithLabelValues(k).Set(f(snap.Exposure))
		s.metrics.OpenPositions.WithLabelValues(k).Set(float64(snap.OpenPositions))
	}
	killed, _ := s.risk.Halted()
	s.metrics.SetFlags(killed, s.eng.Paused())
}

// dailySummary reports the previous UTC day once it has ended.
func (s *session) dailySummary(ctx context.Context, now time.Time) {
	day := now.UTC().Truncate(24 * time.Hour)
	if !day.After(s.day) {
		return
	}
	prev := s.day
	s.day = day
	if s.tg == nil || !s.cfg.Telegram.DailySummary {
		return
	}
	st, err := s.store.TradeStatsBetween(ctx, s.runID, prev, day)
	if err != nil {
		s.log.Error("daily summary", "err", err)
		return
	}
	total := s.book.Total()
	s.tg.Notify(fmt.Sprintf("📊 <b>Daily summary %s</b>\ntrades %d (wins %d) · net PnL <b>%s</b> · fees %s\nequity %s · open positions %d",
		prev.Format(time.DateOnly), st.Trades, st.Wins, st.NetPnL.StringFixed(2), st.Fees.StringFixed(2),
		total.Equity.StringFixed(2), total.OpenPositions))
}
