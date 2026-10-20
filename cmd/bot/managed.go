package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rizz404/trading-bot/internal/app"
	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/fx"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/marketdata"
	"github.com/rizz404/trading-bot/internal/metrics"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

func lockWallets(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) ([]*managed.Lease, error) {
	keys := map[string]bool{}
	for _, w := range cfg.Wallets {
		keys[w.Exchange+":"+string(cfg.App.Mode)+":spot"] = true
	}
	for _, sc := range cfg.Strategies {
		if sc.Enabled {
			keys[sc.Exchange+":"+string(cfg.App.Mode)+":"+sc.Market] = true
		}
	}
	if int(cfg.Database.MaxConns) < len(keys)+2 {
		return nil, errors.New("database.max_conns must allow one ownership connection per wallet plus two working connections")
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	slices.Sort(sorted)
	var leases []*managed.Lease
	for _, key := range sorted {
		l, err := managed.Acquire(ctx, pool, "trading-bot/"+key)
		if err != nil {
			for _, old := range leases {
				old.Close(context.Background())
			}
			return nil, err
		}
		leases = append(leases, l)
	}
	return leases, nil
}
func runManaged(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, leases []*managed.Lease, log *slog.Logger) (runErr error) {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup

	errc := make(chan error, 8)
	fail := func(err error) {
		if err != nil && ctx.Err() == nil {
			select {
			case errc <- err:
			default:
			}
		}
	}
	reg := metrics.NewRegistry(version, string(cfg.App.Mode))
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_managed_equity", Help: "Managed equity by virtual instance."}, []string{"wallet", "instance"})
	reg.MustRegister(gauge)
	feedMetrics := marketdata.NewMetrics(reg)
	store := storage.NewStore(pool)
	runtimes := map[string]*managed.Runtime{}
	// Telegram outlives ctx briefly so the shutdown message is delivered.
	var tg *telegram.Bot
	tgCtx, tgCancel := context.WithCancel(context.Background())
	tgDone := make(chan struct{})
	tgStarted := false
	defer func() {
		cancel()
		wg.Wait()
		status := "stopped"
		if runErr != nil {
			status = "interrupted"
		}
		for _, rt := range runtimes {
			endCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
			if err := rt.End(endCtx, status); err != nil {
				log.Warn("persist managed shutdown", "err", err)
			}
			done()
		}
		if tgStarted {
			tg.Notify(msgManagedStopped(cfg.App.Mode, runErr))
		}
		tgCancel()
		if tgStarted {
			<-tgDone
		}
	}()
	var fxs *fx.Service
	if cfg.Telegram.FX {
		fxs = fx.New(log)
		wg.Go(func() { fxs.Run(ctx) })
	}
	moneyFor := func() money {
		m := money{}
		if fxs != nil {
			m.rates, m.ok = fxs.Rates()
		}
		return m
	}
	if cfg.Telegram.Enabled {
		tg = telegram.New(cfg.Telegram.Token.Reveal(), cfg.Telegram.ChatIDs, log)
	}
	for _, spec := range cfg.Wallets {
		repo, err := managed.Init(ctx, pool, spec)
		if err != nil {
			return err
		}
		// Mark prior sessions for explicit review, without modifying legacy history.
		_, err = pool.Exec(ctx, `INSERT INTO managed_legacy_imports(wallet_id,legacy_run_id,status,notes) SELECT $1,id,'review',jsonb_build_object('reason','legacy ownership and capital require explicit review') FROM strategy_runs WHERE mode=$2 AND instance_id='session' ON CONFLICT DO NOTHING`, spec.Key(), string(cfg.App.Mode))
		if err != nil {
			return err
		}
		source, err := app.MarketSource(spec.Exchange, cfg.App.Mode == config.ModeTestnet, log)
		if err != nil {
			return err
		}
		ins, err := source.Instruments(ctx, domain.Spot)
		if err != nil {
			return err
		}
		// Asset categories remain excluded even for explicitly configured symbols.
		excluded := map[string]bool{}
		if tagger, ok := source.(exchange.AssetTagger); ok {
			tags, tagErr := tagger.AssetTags(ctx)
			if tagErr != nil {
				return fmt.Errorf("managed asset tags: %w", tagErr)
			}
			for base, ts := range tags {
				for _, tag := range ts {
					if strings.EqualFold(tag, "bStocks") {
						excluded[base] = true
					}
				}
			}
		}
		if err = store.UpsertInstruments(ctx, ins); err != nil {
			return err
		}
		var physical broker.Broker
		if cfg.App.Mode == config.ModePaper {
			physical = broker.NewSim(broker.SimConfig{Exchange: source.Name(), Fees: app.Fees(cfg.Backtest.Fees), SlippageBps: cfg.Backtest.SlippageBps, Clock: time.Now})
		} else {
			physical, err = app.LiveBroker(cfg, spec.Exchange, []domain.Market{domain.Spot}, log)
			if err != nil {
				return err
			}
		}
		fee := decimal.NewFromFloat(max(cfg.Backtest.Fees["spot"].Maker, cfg.Backtest.Fees["spot"].Taker) + cfg.Backtest.SlippageBps/10000)
		rt, err := managed.NewRuntime(ctx, repo, physical, ins, fee, time.Now, log)
		if err != nil {
			return err
		}
		rt.CheckLease = func(check context.Context) error {
			for _, l := range leases {
				if err := l.Check(check); err != nil {
					return err
				}
			}
			return nil
		}
		snapshot, err := rt.Snapshot(ctx)
		if err != nil {
			return err
		}
		seen := map[exchange.KlineSub]bool{}
		var subs []exchange.KlineSub
		if cfg.App.Mode != config.ModePaper {
			sub := exchange.KlineSub{Symbol: "BNBUSDT", Timeframe: "1m"}
			subs = append(subs, sub)
			seen[sub] = true
		}
		for owner, b := range snapshot.Bindings {
			st, err := strategy.New(b.Spec.Name)
			if err != nil {
				return err
			}
			if err = st.Init(nil, b.Spec.Config(source.Name(), owner)); err != nil {
				return err
			}
			for _, symbol := range b.Spec.Symbols {
				valid := false
				for _, in := range ins {
					if in.Symbol == symbol && in.Tradable && !excluded[in.Base] {
						valid = true
					}
				}
				if !valid {
					return fmt.Errorf("managed symbol %s is not tradable", symbol)
				}
				tfs := append(st.Timeframes(), domain.Timeframe("1m"))
				for _, tf := range tfs {
					sub := exchange.KlineSub{Symbol: symbol, Timeframe: tf}
					if !seen[sub] {
						seen[sub] = true
						subs = append(subs, sub)
					}
				}
			}
		}
		slices.SortFunc(subs, func(a, b exchange.KlineSub) int {
			if a.Symbol != b.Symbol {
				return strings.Compare(a.Symbol, b.Symbol)
			}
			return strings.Compare(string(a.Timeframe), string(b.Timeframe))
		})
		// Closed history is indicator warmup only. Execution state is restored after
		// replay so grid/DCA operational counters are not consumed by history.
		for _, sub := range subs {
			bars, err := source.Klines(ctx, exchange.KlineRequest{Market: domain.Spot, Symbol: sub.Symbol, Timeframe: sub.Timeframe, Start: time.Now().Add(-time.Duration(warmupBars+2) * sub.Timeframe.Duration()), Limit: warmupBars})
			if err != nil {
				return err
			}
			var closed []domain.Candle
			var previous time.Time
			for _, c := range bars {
				if c.Closed && c.CloseTime().Before(time.Now()) {
					if !previous.IsZero() && !c.OpenTime.Equal(previous.Add(c.Timeframe.Duration())) {
						return fmt.Errorf("incomplete managed warmup for %s %s", sub.Symbol, sub.Timeframe)
					}
					previous = c.OpenTime
					closed = append(closed, c)
					rt.Warmup(c)
				}
			}
			if len(closed) == 0 {
				return fmt.Errorf("warmup missing for %s %s", sub.Symbol, sub.Timeframe)
			}
			if _, err = store.InsertCandles(ctx, closed); err != nil {
				return err
			}
		}
		if live, ok := physical.(broker.Live); ok {
			if err = live.Start(ctx, &wg); err != nil {
				return err
			}
		}
		runtimes[spec.ID] = rt
		if err = rt.Start(ctx); err != nil {
			return err
		}
		if live, ok := physical.(broker.Live); ok {
			wg.Go(func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-live.Resync():
						if err := rt.Reconcile(ctx); err != nil {
							log.Warn("managed reconnect reconciliation", "err", err)
						}
					}
				}
			})
		}
		if tg != nil {
			last := map[string]time.Time{}
			rt.OnNotice = func(owner string, level slog.Level, msg string) {
				key := owner + msg
				if time.Since(last[key]) < time.Minute {
					return
				}
				last[key] = time.Now()
				s, err := rt.Snapshot(ctx)
				if err != nil {
					return
				}
				b := s.Bindings[owner]
				if b == nil {
					return
				}
				pref := s.Instances[b.InstanceID].Notify
				if pref == "off" {
					return
				}
				threshold := slog.LevelWarn
				if pref != "" {
					_ = threshold.UnmarshalText([]byte(pref))
				}
				if level >= threshold {
					tg.Notify("<b>" + html.EscapeString(spec.ID+" / "+owner) + "</b>\n" + html.EscapeString(msg))
				}
			}
		}
		feed := &marketdata.Feed{Source: source, Store: store, Market: domain.Spot, Subs: subs, InitialBackfill: cfg.MarketData.InitialBackfill, Log: log, Metrics: feedMetrics, OnCandle: func(c domain.Candle) { fail(rt.Record(ctx, c, true)) }, OnUpdate: func(c domain.Candle) { fail(rt.Record(ctx, c, false)) }}
		wg.Go(func() { fail(feed.Run(ctx)) })
	}
	if tg != nil {
		registerManagedTelegram(tg, runtimes, moneyFor, cancel)
		tgStarted = true
		go func() { defer close(tgDone); tg.Run(tgCtx) }()
		tg.Notify(msgManagedStarted(ctx, cfg.App.Mode, runtimes, version))
	}
	var servers []*http.Server
	startHTTP := func(srv *http.Server) {
		servers = append(servers, srv)
		wg.Go(func() {
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				fail(err)
			}
		})
	}
	if cfg.Web.Enabled {
		handler := managed.NewHTTP(runtimes, cfg.Web.Password.Reveal(), func(v decimal.Decimal) string { return moneyFor().amount(v) })
		handler.RateLine = func() string { return moneyFor().rateLine() }
		startHTTP(&http.Server{Addr: cfg.Web.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second})
	}
	if cfg.Metrics.Enabled {
		startHTTP(metrics.NewServer(cfg.Metrics.Listen, reg, func(check context.Context) error {
			if err := pool.Ping(check); err != nil {
				return err
			}
			for _, l := range leases {
				if err := l.Check(check); err != nil {
					return err
				}
			}
			return nil
		}))
	}
	defer func() {
		for _, srv := range servers {
			shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
			_ = srv.Shutdown(shutdown)
			stop()
		}
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	reconcile := time.NewTicker(time.Minute)
	defer reconcile.Stop()
	day := time.Now().UTC().Truncate(24 * time.Hour)
	for {
		select {
		case <-ctx.Done():
			cancel()
			return nil
		case err := <-errc:
			cancel()
			return err
		case now := <-tick.C:
			for wallet, rt := range runtimes {
				if err := rt.Tick(ctx, now); err != nil {
					cancel()
					return err
				}
				s, err := rt.Snapshot(ctx)
				if err != nil {
					return err
				}
				for id := range s.Instances {
					v, _ := s.InstanceEquity(id).Float64()
					gauge.WithLabelValues(wallet, id).Set(v)
				}
			}
			today := now.UTC().Truncate(24 * time.Hour)
			if today.After(day) {
				day = today
				if tg != nil && cfg.Telegram.DailySummary {
					tg.Notify(managedStatus(ctx, runtimes, moneyFor()))
				}
			}
		case <-reconcile.C:
			for _, rt := range runtimes {
				if err := rt.Reconcile(ctx); err != nil {
					log.Warn("managed reconciliation unavailable", "err", err)
				}
			}
		}
	}
}
func managedStatus(ctx context.Context, runtimes map[string]*managed.Runtime, m money) string {
	var out strings.Builder
	out.WriteString("<b>Portofolio bot</b>\n")
	for id, r := range runtimes {
		s, err := r.Snapshot(ctx)
		if err != nil {
			continue
		}
		fmt.Fprintf(&out, "\n<b>%s</b> · %s\nNilai: %s\nCadangan: %s\n", html.EscapeString(id), html.EscapeString(s.Spec.Mode), m.amount(s.Equity()), m.amount(s.Reserve))
		for key, i := range s.Instances {
			fmt.Fprintf(&out, "• %s: %s · %s\n", html.EscapeString(key), m.amount(s.InstanceEquity(key)), html.EscapeString(i.Status))
		}
	}
	out.WriteString("\n" + m.rateLine())
	return out.String()
}
func registerManagedTelegram(tg *telegram.Bot, runtimes map[string]*managed.Runtime, moneyFor func() money, stop context.CancelFunc) {
	ids := slices.Sorted(maps.Keys(runtimes))
	tg.SetHelpIntro("<b>Perintah bot</b>\nAkun: <code>" + html.EscapeString(strings.Join(ids, ", ")) + "</code>. Scope: <code>account</code>, ID instance, atau <code>instance/strategi</code>. Aksi dana memerlukan kata <code>confirm</code>.")
	tg.Handle("status", "ringkasan nilai akun dan instance", func(ctx context.Context, _ []string) string { return managedStatus(ctx, runtimes, moneyFor()) })
	help := map[string]string{
		"pause":   "jeda entry baru · /pause akun scope",
		"resume":  "lanjutkan entry · /resume akun scope",
		"cancel":  "batalkan order pending · /cancel akun scope confirm",
		"flatten": "jual posisi milik scope · /flatten akun scope confirm",
		"finish":  "akhiri eksperimen · /finish akun scope retain|flatten confirm",
		"kill":    "halt darurat · /kill akun scope confirm",
		"reset":   "buka halt risiko · /reset akun scope confirm",
		"archive": "arsipkan instance kosong · /archive akun scope confirm",
	}
	for _, op := range []string{"pause", "resume", "cancel", "flatten", "finish", "kill", "reset", "archive"} {
		tg.Handle(op, help[op], func(ctx context.Context, args []string) string {
			if len(args) < 2 {
				return "Gunakan /" + op + " akun scope; scope = account, ID instance, atau instance/strategi. Aksi dana memerlukan confirm."
			}
			rt := runtimes[args[0]]
			if rt == nil {
				return "Akun tidak ditemukan."
			}
			policy := ""
			for _, v := range args[2:] {
				if v == "retain" || v == "flatten" {
					policy = v
				}
			}
			c := managed.Command{ID: fmt.Sprintf("telegram-%d", time.Now().UnixNano()), Operation: op, Scope: args[1], Policy: policy, Confirm: slices.Contains(args[2:], "confirm")}
			if err := rt.Command(ctx, c); err != nil {
				return "Belum diterapkan: " + html.EscapeString(err.Error())
			}
			return "Perintah diterima. Periksa /status; penutupan menunggu konfirmasi order."
		})
	}
	tg.Handle("resolve", "verifikasi order tak pasti ke exchange · /resolve akun order confirm", func(ctx context.Context, args []string) string {
		if len(args) < 2 {
			return "Gunakan /resolve akun order confirm. ID order ada di dashboard, panel Order perlu verifikasi."
		}
		rt := runtimes[args[0]]
		if rt == nil {
			return "Akun tidak ditemukan."
		}
		c := managed.Command{ID: fmt.Sprintf("telegram-%d", time.Now().UnixNano()), Operation: "resolve_order", Order: args[1], Confirm: slices.Contains(args[2:], "confirm")}
		if err := rt.Command(ctx, c); err != nil {
			return "Belum diselesaikan: " + html.EscapeString(err.Error())
		}
		return "Order diverifikasi ke exchange dan ledger diperbarui. Periksa /status."
	})
	tg.Handle("stop", "stop proses untuk resume; posisi dan order tetap · /stop confirm", func(_ context.Context, args []string) string {
		if !slices.Contains(args, "confirm") {
			return "Order dan posisi tetap ada di exchange. Untuk stop proses kirim /stop confirm."
		}
		stop()
		return "Proses dihentikan untuk resume; posisi dan order tetap dipertahankan."
	})
}
