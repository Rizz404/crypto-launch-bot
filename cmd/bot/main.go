// Command bot runs the trading bot.
//
// Usage:
//
//	bot [flags] run                 start the bot
//	bot [flags] migrate [up|down|status]
//	bot version
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rizz404/trading-bot/internal/app"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/logging"
	"github.com/rizz404/trading-bot/internal/marketdata"
	"github.com/rizz404/trading-bot/internal/metrics"
	"github.com/rizz404/trading-bot/internal/storage"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("bot", flag.ContinueOnError)
	configPath := fs.String("config", envOr("TB_CONFIG", "configs/config.yaml"), "path to YAML config")
	envFile := fs.String("env-file", ".env", "optional .env file with secrets (process env takes precedence)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "usage: bot [flags] <run|migrate [up|down|status]|version>\n\nflags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return errors.New("missing command")
	}
	if args[0] == "version" {
		fmt.Println(version)
		return nil
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

	switch args[0] {
	case "run":
		return runBot(ctx, cfg, log)
	case "migrate":
		sub := "up"
		if len(args) > 1 {
			sub = args[1]
		}
		return runMigrate(ctx, cfg, log, sub)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runMigrate(ctx context.Context, cfg *config.Config, log *slog.Logger, sub string) error {
	pool, err := storage.Open(ctx, cfg.Database.URL.Reveal(), cfg.Database.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	m, err := storage.NewMigrator(pool)
	if err != nil {
		return err
	}
	defer func() { _ = m.Close() }()

	switch sub {
	case "up":
		return m.Up(ctx, log)
	case "down":
		return m.Down(ctx, log)
	case "status":
		return m.Status(ctx, log)
	default:
		return fmt.Errorf("unknown migrate command %q (want up, down or status)", sub)
	}
}

func runBot(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	if cfg.App.Mode == config.ModeBacktest {
		return errors.New("backtest mode is not run by \"bot run\"; use cmd/backtest (fase 2)")
	}
	log.Info("starting trading bot", "version", version, "mode", cfg.App.Mode, "exchanges", cfg.EnabledExchanges())

	pool, err := storage.Open(ctx, cfg.Database.URL.Reveal(), cfg.Database.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	m, err := storage.NewMigrator(pool)
	if err != nil {
		return err
	}
	pending, err := m.Pending(ctx)
	_ = m.Close()
	if err != nil {
		return fmt.Errorf("check migrations: %w", err)
	}
	if pending > 0 {
		return fmt.Errorf("%d pending database migration(s), run \"bot migrate up\" first", pending)
	}

	reg := metrics.NewRegistry(version, string(cfg.App.Mode))
	errc := make(chan error, 1)
	var srv *http.Server
	if cfg.Metrics.Enabled {
		srv = metrics.NewServer(cfg.Metrics.Listen, reg, pool.Ping)
		go func() {
			log.Info("metrics server listening", "addr", cfg.Metrics.Listen)
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				errc <- fmt.Errorf("metrics server: %w", err)
			}
		}()
	}

	// Live data comes from testnet only in testnet mode, so paper trading
	// sees real mainnet prices.
	feeds, err := startFeeds(ctx, cfg, storage.NewStore(pool), marketdata.NewMetrics(reg), log, errc)
	if err != nil {
		return err
	}

	// TODO(fase 3): strategies, risk manager and OMS consume the feeds.
	log.Info("bot ready; recording market data, trading engine not implemented yet")

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errc:
		return err
	}

	stop := time.Now()
	feeds.Wait()
	log.Info("feeds stopped", "took", time.Since(stop).Round(time.Millisecond))

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("metrics server shutdown", "err", err)
		}
	}
	log.Info("bot stopped")
	return nil
}

// startFeeds starts one live candle feed per configured market. A feed that
// stops for any reason other than shutdown reports to errc.
func startFeeds(ctx context.Context, cfg *config.Config, store *storage.Store, m *marketdata.Metrics,
	log *slog.Logger, errc chan<- error,
) (*sync.WaitGroup, error) {
	src, err := app.MarketSource(cfg.MarketData.Exchange, cfg.App.Mode == config.ModeTestnet, log)
	if err != nil {
		return nil, err
	}
	tfs, err := app.Timeframes(cfg.MarketData.Timeframes)
	if err != nil {
		return nil, err
	}
	rules := app.UniverseRules(cfg.Universe)

	var wg sync.WaitGroup
	for _, name := range cfg.MarketData.Markets {
		market, err := domain.ParseMarket(name)
		if err != nil {
			return nil, err
		}
		symbols, err := app.ResolveSymbols(ctx, src, store, market, cfg.MarketData.Symbols, rules, log)
		if err != nil {
			return nil, err
		}
		feed := &marketdata.Feed{
			Source:          src,
			Store:           store,
			Market:          market,
			Subs:            app.Subscriptions(symbols, tfs),
			InitialBackfill: cfg.MarketData.InitialBackfill,
			Log:             log,
			Metrics:         m,
		}
		log.Info("starting feed", "source", src.Name(), "market", market, "symbols", len(symbols), "timeframes", cfg.MarketData.Timeframes)
		wg.Go(func() {
			if err := feed.Run(ctx); err != nil && ctx.Err() == nil {
				select {
				case errc <- fmt.Errorf("%s feed: %w", market, err):
				default:
				}
			}
		})
	}
	return &wg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
