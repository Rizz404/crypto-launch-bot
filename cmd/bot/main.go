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
	"syscall"
	"time"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/logging"
	"github.com/rizz404/trading-bot/internal/marketdata"
	"github.com/rizz404/trading-bot/internal/metrics"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
	"github.com/rizz404/trading-bot/internal/storage"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
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

	// /stop on Telegram cancels this context like a signal does.
	ctx, stopBot := context.WithCancel(ctx)
	defer stopBot()

	// Telegram outlives ctx briefly so the shutdown message is delivered.
	var tg *telegram.Bot
	tgCtx, tgCancel := context.WithCancel(context.Background())
	tgDone := make(chan struct{})
	defer func() {
		tgCancel()
		<-tgDone
	}()
	if cfg.Telegram.Enabled {
		tg = telegram.New(cfg.Telegram.Token.Reveal(), cfg.Telegram.ChatIDs, log)
		go func() { defer close(tgDone); tg.Run(tgCtx) }()
	} else {
		close(tgDone)
	}

	store := storage.NewStore(pool)
	plan := newFeedPlan(cfg, log)
	if err := plan.addMarketData(ctx, store); err != nil {
		return err
	}
	strategies, err := plan.strategies(ctx, store)
	if err != nil {
		return err
	}

	var sess *session
	if len(strategies) > 0 {
		if sess, err = newSession(ctx, cfg, store, strategies, metrics.NewTrading(reg), tg, log); err != nil {
			return err
		}
		if tg != nil {
			sess.registerCommands(tg, stopBot)
		}
		for _, series := range sess.Subscriptions() {
			for key := range series {
				plan.add(key.Exchange, key.Market, key.Symbol, key.Timeframe)
			}
		}
		if err := sess.warmup(ctx); err != nil {
			sess.close("startup failed")
			return err
		}
		if err := sess.start(ctx); err != nil {
			sess.close("startup failed")
			return err
		}
		go sess.loop(ctx)
	}

	feeds := plan.start(ctx, store, marketdata.NewMetrics(reg), sess, errc)
	log.Info("bot ready", "trading", sess != nil, "strategies", len(strategies))
	if tg != nil {
		tg.Notify(fmt.Sprintf("🤖 <b>Bot started</b> · mode %s · %d strategies · version %s",
			cfg.App.Mode, len(strategies), telegram.Escape(version)))
	}

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case runErr = <-errc:
		log.Error("stopping after error", "err", runErr)
	}

	stop := time.Now()
	feeds.Wait()
	log.Info("feeds stopped", "took", time.Since(stop).Round(time.Millisecond))
	if sess != nil {
		reason := "stopped"
		if runErr != nil {
			reason = "error: " + runErr.Error()
		}
		sess.close(reason)
	}

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("metrics server shutdown", "err", err)
		}
	}
	log.Info("bot stopped")
	if tg != nil {
		msg := "🛑 <b>Bot stopped</b>"
		if runErr != nil {
			msg += "\nerror: " + telegram.Escape(runErr.Error())
		}
		tg.Notify(msg)
	}
	return runErr
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
