package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/notify/telegram"
)

// registerCommands installs the Telegram commands of a trading session.
// stop shuts the bot down gracefully.
func (s *session) registerCommands(bot *telegram.Bot, stop context.CancelFunc) {
	confirm := func(args []string) bool { return len(args) > 0 && args[0] == "confirm" }

	bot.Handle("status", "mode, equity, kill switch and pause state", func(ctx context.Context, _ []string) string {
		total := s.book.Total()
		killed, reason := s.risk.Halted()
		state := "running"
		switch {
		case killed:
			state = "🛑 KILL SWITCH: " + telegram.Escape(reason)
		case s.eng.Paused():
			state = "⏸ paused (no new entries)"
		}
		today, _ := s.store.TradeStats(ctx, s.runID, time.Now().UTC().Truncate(24*time.Hour))
		return fmt.Sprintf("<b>Status</b> · %s · %s\nequity <b>%s</b> (unrealized %s)\nopen positions %d · exposure %s\ntoday: %d trades, net %s",
			s.cfg.App.Mode, state, total.Equity.StringFixed(2), total.UnrealizedPnL.StringFixed(2),
			total.OpenPositions, total.Exposure.StringFixed(2), today.Trades, today.NetPnL.StringFixed(2))
	})

	bot.Handle("positions", "open positions", func(context.Context, []string) string {
		var sb strings.Builder
		sb.WriteString("<b>Positions</b>\n")
		n := 0
		for _, acct := range s.book.Accounts() {
			for _, p := range acct.Positions() {
				n++
				mark, _ := s.eng.LastPrice(p.Exchange, p.Market, p.Symbol)
				fmt.Fprintf(&sb, "%s %s %s qty %s @ %s · mark %s · uPnL <b>%s</b>\n",
					telegram.Escape(acct.Key.String()), p.Side, telegram.Escape(p.Symbol), p.Quantity,
					p.EntryPrice.StringFixed(4), mark.StringFixed(4), p.UnrealizedPnL(mark).StringFixed(2))
			}
		}
		if n == 0 {
			return "No open positions."
		}
		return sb.String()
	})

	bot.Handle("balance", "balance and equity per account", func(context.Context, []string) string {
		var sb strings.Builder
		sb.WriteString("<b>Accounts</b>\n")
		for _, acct := range s.book.Accounts() {
			snap := acct.Snapshot()
			fmt.Fprintf(&sb, "%s: balance %s · equity %s · free margin %s\n", telegram.Escape(acct.Key.String()),
				snap.Balance.StringFixed(2), snap.Equity.StringFixed(2), snap.FreeMargin().StringFixed(2))
		}
		return sb.String()
	})

	bot.Handle("pnl", "realized PnL today and this session", func(ctx context.Context, _ []string) string {
		today, err := s.store.TradeStats(ctx, s.runID, time.Now().UTC().Truncate(24*time.Hour))
		if err != nil {
			return "error: " + telegram.Escape(err.Error())
		}
		session, err := s.store.TradeStats(ctx, s.runID, time.Time{})
		if err != nil {
			return "error: " + telegram.Escape(err.Error())
		}
		return fmt.Sprintf("<b>PnL</b>\ntoday: %d trades (%d wins), net <b>%s</b>, fees %s\nsession: %d trades (%d wins), net <b>%s</b>, fees %s",
			today.Trades, today.Wins, today.NetPnL.StringFixed(2), today.Fees.StringFixed(2),
			session.Trades, session.Wins, session.NetPnL.StringFixed(2), session.Fees.StringFixed(2))
	})

	bot.Handle("orders", "open orders", func(context.Context, []string) string {
		orders := s.eng.OpenOrders()
		if len(orders) == 0 {
			return "No open orders."
		}
		var sb strings.Builder
		sb.WriteString("<b>Open orders</b>\n")
		for _, o := range orders {
			price := o.Price
			if o.Type.IsConditional() {
				price = o.StopPrice
			}
			fmt.Fprintf(&sb, "%s %s %s %s qty %s @ %s (%s)\n", telegram.Escape(o.Symbol), o.Side, o.Type, o.Tag,
				o.Quantity, price, telegram.Escape(o.Strategy))
		}
		return sb.String()
	})

	bot.Handle("strategies", "running strategy instances", func(context.Context, []string) string {
		var sb strings.Builder
		sb.WriteString("<b>Strategies</b>\n")
		for _, sc := range s.eng.Strategies() {
			fmt.Fprintf(&sb, "%s (%s) %s:%s %s\n", telegram.Escape(sc.ID), telegram.Escape(sc.Name),
				telegram.Escape(sc.Exchange), sc.Market, telegram.Escape(strings.Join(sc.Symbols, ",")))
		}
		return sb.String()
	})

	bot.Handle("pause", "stop opening new positions (exits keep working)", func(context.Context, []string) string {
		s.eng.SetPaused(true)
		return "⏸ Paused: no new entries."
	})

	bot.Handle("resume", "resume new entries", func(context.Context, []string) string {
		if killed, _ := s.risk.Halted(); killed {
			return "Kill switch is active; use /reset confirm first."
		}
		s.eng.SetPaused(false)
		return "▶️ Resumed."
	})

	bot.Handle("killswitch", "close ALL positions and halt (needs: /killswitch confirm)", func(ctx context.Context, args []string) string {
		if !confirm(args) {
			return "This closes every position and halts trading. Send <code>/killswitch confirm</code>."
		}
		s.eng.Kill(ctx, "telegram /killswitch")
		return "🛑 Kill switch activated: closing all positions."
	})

	bot.Handle("reset", "clear the kill switch (needs: /reset confirm)", func(_ context.Context, args []string) string {
		if !confirm(args) {
			return "Send <code>/reset confirm</code> to clear the kill switch and resume trading."
		}
		s.eng.ResetKillSwitch()
		return "Kill switch cleared, trading resumed."
	})

	bot.Handle("stop", "shut the bot down; positions and stops stay (needs: /stop confirm)", func(_ context.Context, args []string) string {
		if !confirm(args) {
			return "Send <code>/stop confirm</code> to shut the bot down. Open positions and their exchange stops stay in place."
		}
		go func() {
			time.Sleep(time.Second) // let the reply go out first
			stop()
		}()
		return "Shutting down…"
	})
}
