package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/rizz404/trading-bot/internal/web"
)

// liveView exposes a session to the dashboard.
type liveView struct{ s *session }

func (v liveView) Mode() string                   { return string(v.s.cfg.App.Mode) }
func (v liveView) RunID() int64                   { return v.s.runID }
func (v liveView) Accounts() []*portfolio.Account { return v.s.book.Accounts() }
func (v liveView) OpenOrders() []domain.Order     { return v.s.eng.OpenOrders() }
func (v liveView) Strategies() []strategy.Config  { return v.s.eng.Strategies() }
func (v liveView) Halted() (bool, string)         { return v.s.risk.Halted() }
func (v liveView) Paused() bool                   { return v.s.eng.Paused() }
func (v liveView) SetPaused(p bool)               { v.s.eng.SetPaused(p) }
func (v liveView) ResetKillSwitch()               { v.s.eng.ResetKillSwitch() }
func (v liveView) Kill(ctx context.Context, reason string) {
	v.s.eng.Kill(ctx, reason)
}
func (v liveView) LastPrice(exchange string, m domain.Market, symbol string) (decimal.Decimal, bool) {
	return v.s.eng.LastPrice(exchange, m, symbol)
}

// startWeb serves the dashboard until shutdown is called.
func startWeb(cfg *config.Config, store *storage.Store, sess *session, log *slog.Logger, errc chan<- error) (shutdown func()) {
	if !cfg.Web.Enabled {
		return func() {}
	}
	var live web.Live // stays a nil interface without a session
	if sess != nil {
		live = liveView{sess}
	}
	srv := &http.Server{
		Addr:              cfg.Web.Listen,
		Handler:           web.New(store, live, cfg.Web.Password.Reveal(), log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("web dashboard listening", "addr", cfg.Web.Listen, "auth", cfg.Web.Password != "")
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			select {
			case errc <- err:
			default:
			}
		}
	}()
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}
