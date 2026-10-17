// Command backtest replays stored candles through a configured strategy.
//
// Examples:
//
//	backtest -strategy emacross-1h -from 2025-01-01 -to 2026-01-01
//	backtest -strategy emacross-1h -from 2025-01-01 -split 0.7 -out results/ -save
//
// Candles must be downloaded first with datafetch. -split runs an
// in-sample and an out-of-sample backtest on the two parts of the range.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/app"
	"github.com/rizz404/trading-bot/internal/backtest"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/logging"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("backtest", flag.ContinueOnError)
	var (
		configPath = fs.String("config", envOr("TB_CONFIG", "configs/config.yaml"), "path to YAML config")
		envFile    = fs.String("env-file", ".env", "optional .env file")
		stratID    = fs.String("strategy", "", "strategy id from the config (required)")
		symFlag    = fs.String("symbols", "", "override the strategy symbols (comma-separated)")
		fromFlag   = fs.String("from", time.Now().UTC().AddDate(-1, 0, 0).Format(time.DateOnly), "start date (UTC)")
		toFlag     = fs.String("to", "", "end date, exclusive (default: now)")
		split      = fs.Float64("split", 0, "in-sample fraction (e.g. 0.7) for an out-of-sample check; 0 disables")
		balance    = fs.Float64("balance", 0, "override backtest.initial_balance")
		execTF     = fs.String("exec-tf", "", "override backtest.exec_timeframe")
		outDir     = fs.String("out", "", "write equity and trades CSV files to this directory")
		save       = fs.Bool("save", false, "store the run, trades and equity curve in the database")
	)
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *stratID == "" {
		return errors.New("-strategy is required")
	}
	if *split < 0 || *split >= 1 {
		return errors.New("-split must be in [0, 1)")
	}
	from, err := time.Parse(time.DateOnly, *fromFlag)
	if err != nil {
		return fmt.Errorf("-from: %w", err)
	}
	to := time.Now().UTC()
	if *toFlag != "" {
		if to, err = time.Parse(time.DateOnly, *toFlag); err != nil {
			return fmt.Errorf("-to: %w", err)
		}
	}

	cfg, err := config.Load(*configPath, *envFile)
	if err != nil {
		return err
	}
	log, err := logging.New(os.Stderr, cfg.App.LogLevel, cfg.App.LogFormat)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sc, ok := cfg.Strategy(*stratID)
	if !ok {
		return fmt.Errorf("strategy %q not found in config", *stratID)
	}
	scfg, err := app.StrategyConfig(sc, cfg.MarketData.Symbols)
	if err != nil {
		return err
	}
	if *symFlag != "" {
		scfg.Symbols = strings.Split(strings.ToUpper(*symFlag), ",")
	}
	if len(scfg.Symbols) == 0 {
		return errors.New("no symbols: set strategies[].symbols, marketdata.symbols or -symbols")
	}

	pool, err := storage.Open(ctx, cfg.Database.URL.Reveal(), cfg.Database.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := storage.NewStore(pool)
	var ins []domain.Instrument
	loaded := map[portfolio.AccountKey]bool{}
	for _, in := range scfg.Instruments() {
		key := portfolio.AccountKey{Exchange: in.Exchange, Market: in.Market}
		if loaded[key] {
			continue
		}
		loaded[key] = true
		got, err := store.Instruments(ctx, in.Exchange, in.Market)
		if err != nil {
			return err
		}
		if len(got) == 0 {
			return fmt.Errorf("no instruments stored for %s %s; run datafetch first", in.Exchange, in.Market)
		}
		ins = append(ins, got...)
	}

	bt := backtest.Config{
		Strategy:       scfg,
		From:           from,
		To:             to,
		InitialBalance: decimal.NewFromFloat(cfg.Backtest.InitialBalance),
		Fees:           app.Fees(cfg.Backtest.Fees),
		SlippageBps:    cfg.Backtest.SlippageBps,
		FundingRate:    decimal.NewFromFloat(cfg.Backtest.FundingRate),
		ExecTimeframe:  domain.Timeframe(cfg.Backtest.ExecTimeframe),
		Risk:           cfg.Risk,
		Instruments:    ins,
		Log:            log,
	}
	if *balance > 0 {
		bt.InitialBalance = decimal.NewFromFloat(*balance)
	}
	if *execTF != "" {
		tf, err := domain.ParseTimeframe(*execTF)
		if err != nil {
			return err
		}
		bt.ExecTimeframe = tf
	}

	type segment struct {
		name     string
		from, to time.Time
	}
	segments := []segment{{"full", from, to}}
	if *split > 0 {
		cut := from.Add(time.Duration(float64(to.Sub(from)) * *split)).Truncate(24 * time.Hour)
		segments = []segment{{"in-sample", from, cut}, {"out-of-sample", cut, to}}
	}

	for _, seg := range segments {
		bt.From, bt.To = seg.from, seg.to
		res, err := backtest.Run(ctx, bt, store)
		if err != nil {
			return fmt.Errorf("%s: %w", seg.name, err)
		}
		backtest.WriteReport(os.Stdout, fmt.Sprintf("%s · %s · %s", sc.ID, strings.Join(scfg.Symbols, ","), seg.name), res)
		if *outDir != "" {
			if err := backtest.WriteCSV(*outDir, sc.ID+"-"+seg.name, res); err != nil {
				return err
			}
		}
		if *save {
			id, err := saveRun(ctx, store, scfg, seg.name, res)
			if err != nil {
				return err
			}
			log.Info("backtest saved", "run_id", id, "segment", seg.name)
		}
	}
	return nil
}

func saveRun(ctx context.Context, store *storage.Store, sc strategy.Config, segment string, res *backtest.Result) (int64, error) {
	params := map[string]any{"params": sc.Params, "segment": segment, "from": res.Config.From, "to": res.Config.To,
		"initial_balance": res.Config.InitialBalance, "slippage_bps": res.Config.SlippageBps}
	id, err := store.CreateRun(ctx, storage.Run{
		InstanceID: sc.ID, Strategy: sc.Name, Mode: "backtest", Exchange: sc.Exchange, Market: sc.Market,
		Symbols: sc.Symbols, Params: params, StartedAt: res.Config.From,
	})
	if err != nil {
		return 0, err
	}
	if err := store.InsertTrades(ctx, id, res.Trades); err != nil {
		return id, err
	}
	// Hourly samples keep the stored curve small.
	var pts []storage.EquityPoint
	var last time.Time
	for _, p := range res.Equity {
		if h := p.Time.Truncate(time.Hour); h.After(last) || len(pts) == 0 {
			last = h
			pts = append(pts, storage.EquityPoint{Time: p.Time, Equity: p.Equity, Balance: p.Equity})
		}
	}
	// The curve is the total over all accounts of the run.
	if err := store.InsertEquity(ctx, id, "total", pts); err != nil {
		return id, err
	}
	return id, store.FinishRun(ctx, id, "completed", res.Metrics)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
