package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange/binance"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
	"github.com/rizz404/trading-bot/internal/storage"
)

func validatePilotDatabase(raw string) error {
	cfg, err := pgxpool.ParseConfig(raw)
	if raw == "" || err != nil || cfg.ConnConfig.User != "catalyst" || !strings.HasPrefix(cfg.ConnConfig.Database, "tradingbot_catalyst_") {
		return errors.New("pilot requires its own tradingbot_catalyst_* database and catalyst role")
	}
	return nil
}

func runPilot(ctx context.Context, policy catalyst.Policy, archive catalyst.Archive, binary, envFile, listen string, out io.Writer) error {
	values, err := loadValues(envFile)
	if err != nil {
		return err
	}
	if err = validatePilotDatabase(values["TB_CATALYST_DATABASE_URL"]); err != nil {
		return err
	}
	if len(values["TB_CATALYST_WEB_PASSWORD"]) < 16 {
		return errors.New("pilot dashboard password must have at least 16 characters")
	}
	analyst, err := catalyst.NewAnalyst(values["TB_CATALYST_BASE_URL"], values["TB_CATALYST_MODEL"], values["TB_CATALYST_API_KEY"])
	if err != nil {
		return err
	}
	collector, err := catalyst.NewSpiderCollector(policy, archive, binary)
	if err != nil {
		return err
	}
	archive.Dir, err = filepath.Abs(archive.Dir)
	if err != nil {
		return err
	}
	collector.Archive = archive
	inbox := filepath.Join(archive.Dir, "inbox")
	spec, err := catalyst.PaperSpec(policy, inbox)
	if err != nil {
		return err
	}
	pool, err := storage.Open(ctx, values["TB_CATALYST_DATABASE_URL"], 4)
	if err != nil {
		return errors.New("pilot database connection unavailable")
	}
	defer pool.Close()
	log := slog.Default().With("component", "catalyst-paper")
	migrator, err := storage.NewMigrator(pool)
	if err != nil {
		return errors.New("pilot database migrator unavailable")
	}
	err = migrator.Up(ctx, log)
	_ = migrator.Close()
	if err != nil {
		return errors.New("pilot schema migration failed")
	}
	lease, err := managed.Acquire(ctx, pool, spec.Key())
	if err != nil {
		return errors.New("pilot wallet already owned or database unavailable")
	}
	defer func() {
		doneCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		lease.Close(doneCtx)
	}()
	repo, err := managed.Init(ctx, pool, spec)
	if err != nil {
		return errors.New("pilot ledger initialization failed; preserve data and inspect policy/config hash")
	}
	market := catalyst.NewMarket(binance.New(binance.Options{Log: log, HTTPClient: &http.Client{Timeout: 10 * time.Second}}))
	ins, err := market.Refresh(ctx, policy, time.Now().UTC())
	if err != nil {
		return errors.New("pilot public Spot universe unavailable")
	}
	runtime, err := catalyst.NewPaper(ctx, repo, policy, inbox, ins, time.Now)
	if err != nil {
		return errors.New("pilot runtime initialization failed")
	}
	runtime.CheckLease = lease.Check
	pilot := &catalyst.Pilot{Policy: policy, Archive: archive, Collector: collector, Market: market, Analyst: analyst, Runtime: runtime, Inbox: inbox, CheckLease: lease.Check, Log: log}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if token := values["TB_CATALYST_TELEGRAM_TOKEN"]; token != "" {
		var chats []int64
		for _, text := range strings.Split(values["TB_CATALYST_TELEGRAM_CHATS"], ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
			if err != nil || id == 0 {
				return errors.New("pilot Telegram chat whitelist invalid")
			}
			chats = append(chats, id)
		}
		bot := telegram.New(token, chats, log)
		go bot.RunSender(runCtx)
		pilot.Notify = func(text string) { bot.Notify(telegram.Escape(text)) }
		runtime.OnExecution = func(e managed.Execution, order domain.Order) {
			pilot.Notify(fmt.Sprintf("P3 paper • %s %s\nJumlah %s @ %s USDT\nKomisi %s %s\n%s", e.Fill.Side, e.Fill.Symbol, e.Fill.Quantity, e.Fill.Price, e.Fill.Fee, e.Fill.FeeAsset, order.Reason))
		}
	}
	server := &http.Server{Handler: pilot.HTTP(values["TB_CATALYST_WEB_PASSWORD"]), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 16 << 10}
	// Bind before the worker starts so a busy port fails startup instead of leaving an orphan worker.
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("pilot dashboard listen: %w", err)
	}
	workerDone, httpDone := make(chan error, 1), make(chan error, 1)
	go func() { workerDone <- pilot.Run(runCtx) }()
	go func() { httpDone <- server.Serve(ln) }()
	_ = json.NewEncoder(out).Encode(map[string]any{"phase": "starting", "paper_only": true, "wallet": spec.Key(), "policy_hash": catalyst.Hash(policy), "daily_ai_limit": catalyst.DailyInferenceLimit})
	workerFinished := false
	select {
	case <-ctx.Done():
	case err = <-workerDone:
		workerFinished = true
	case err = <-httpDone:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}
	cancel()
	shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
	_ = server.Shutdown(shutdownCtx)
	done()
	if !workerFinished {
		err = errors.Join(err, <-workerDone)
	}
	return err
}
