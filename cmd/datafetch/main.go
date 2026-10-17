// Command datafetch downloads historical candles into the database.
//
// Examples:
//
//	datafetch -market futures -tf 1h,4h -from 2025-01-01
//	datafetch -market spot -symbols BTCUSDT,ETHUSDT -tf 1m -from 2026-09-01 -to 2026-10-01
//
// Without -symbols the universe rules from the config pick the symbols.
// Re-running is safe: existing candles are skipped and a series that
// already covers -from resumes after its newest candle.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rizz404/trading-bot/internal/app"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/logging"
	"github.com/rizz404/trading-bot/internal/marketdata"
	"github.com/rizz404/trading-bot/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("datafetch", flag.ContinueOnError)
	var (
		configPath = fs.String("config", envOr("TB_CONFIG", "configs/config.yaml"), "path to YAML config")
		envFile    = fs.String("env-file", ".env", "optional .env file")
		marketFlag = fs.String("market", "futures", "spot or futures")
		tfFlag     = fs.String("tf", "1h", "comma-separated timeframes")
		symFlag    = fs.String("symbols", "", "comma-separated symbols (default: universe from config)")
		topN       = fs.Int("top", 0, "override universe.top_n")
		fromFlag   = fs.String("from", time.Now().UTC().AddDate(0, 0, -90).Format(time.DateOnly), "start date (YYYY-MM-DD, UTC)")
		toFlag     = fs.String("to", "", "end date, exclusive (default: now)")
		workers    = fs.Int("workers", 4, "parallel downloads")
		testnet    = fs.Bool("testnet", false, "download testnet data instead of mainnet")
	)
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	market, err := domain.ParseMarket(*marketFlag)
	if err != nil {
		return err
	}
	tfs, err := app.Timeframes(splitList(*tfFlag))
	if err != nil {
		return err
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
	if !from.Before(to) {
		return errors.New("-from must be before -to")
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

	pool, err := storage.Open(ctx, cfg.Database.URL.Reveal(), cfg.Database.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := storage.NewStore(pool)

	src, err := app.MarketSource(cfg.MarketData.Exchange, *testnet, log)
	if err != nil {
		return err
	}
	rules := app.UniverseRules(cfg.Universe)
	if *topN > 0 {
		rules.TopN = *topN
	}
	symbols, err := app.ResolveSymbols(ctx, src, store, market, splitList(strings.ToUpper(*symFlag)), rules, log)
	if err != nil {
		return err
	}

	subs := app.Subscriptions(symbols, tfs)
	log.Info("downloading", "source", src.Name(), "market", market, "series", len(subs),
		"from", from.Format(time.DateOnly), "to", to.Format(time.RFC3339))

	jobs := make(chan exchange.KlineSub)
	var (
		wg              sync.WaitGroup
		total, failures atomic.Int64
		start           = time.Now()
	)
	for range max(*workers, 1) {
		wg.Go(func() {
			for sub := range jobs {
				key := domain.SeriesKey{Exchange: src.Name(), Market: market, Symbol: sub.Symbol, Timeframe: sub.Timeframe}
				n, err := fetchSeries(ctx, src, store, key, from, to)
				total.Add(int64(n))
				if err != nil {
					if ctx.Err() == nil {
						failures.Add(1)
						log.Error("series failed", "series", key, "saved", n, "err", err)
					}
					continue
				}
				log.Info("series done", "series", key, "new_candles", n)
			}
		})
	}
	for _, s := range subs {
		select {
		case jobs <- s:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()

	log.Info("finished", "candles", total.Load(), "failed_series", failures.Load(), "took", time.Since(start).Round(time.Second))
	if err := ctx.Err(); err != nil {
		return err
	}
	if n := failures.Load(); n > 0 {
		return fmt.Errorf("%d series failed", n)
	}
	return nil
}

// fetchSeries downloads [from, to), skipping what is already stored when
// the stored data starts at or before from.
func fetchSeries(ctx context.Context, src exchange.MarketData, store *storage.Store, key domain.SeriesKey, from, to time.Time) (int, error) {
	first, last, ok, err := store.CandleRange(ctx, key)
	if err != nil {
		return 0, err
	}
	if ok && !first.After(key.Timeframe.Truncate(from)) {
		from = last.Add(key.Timeframe.Duration())
	}
	if !from.Before(to) {
		return 0, nil
	}
	saved := 0
	_, err = marketdata.FetchRange(ctx, src, key, from, to, func(cs []domain.Candle) error {
		n, err := store.InsertCandles(ctx, cs)
		saved += int(n)
		return err
	})
	return saved, err
}

func splitList(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
