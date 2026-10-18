package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
)

// registerCommands installs the Telegram commands of a trading session.
// stop shuts the bot down gracefully. Replies are in Indonesian for a
// beginner (see messages.go).
func (s *session) registerCommands(bot *telegram.Bot, stop context.CancelFunc) {
	confirm := func(args []string) bool { return len(args) > 0 && args[0] == "confirm" }

	bot.SetHelpIntro("🤖 <b>Perintah bot trading</b>\n" + modeText(s.cfg.App.Mode) + "\n\n" +
		"<b>Melihat keadaan</b>: /status, /positions, /orders, /balance, /pnl, /strategies\n" +
		"<b>Mengendalikan</b>: /pause, /resume, /killswitch, /reset, /stop\n\n" +
		"Perintah yang menutup posisi atau mematikan bot meminta kata <code>confirm</code> supaya tidak terkirim tanpa sengaja.")

	bot.Handle("status", "ringkasan: bot jalan atau berhenti, modal, posisi, hasil hari ini", func(ctx context.Context, _ []string) string {
		total := s.book.Total()
		killed, reason := s.risk.Halted()
		state := "✅ Bot berjalan normal"
		switch {
		case killed:
			state = "🛑 Kill switch aktif: " + esc(reason) + "\nTidak ada posisi baru sampai /reset confirm."
		case s.eng.Paused():
			state = "⏸ Dijeda: tidak membuka posisi baru (posisi yang ada tetap dijaga). Lanjutkan dengan /resume."
		}
		today, _ := s.store.TradeStats(ctx, s.runID, time.Now().UTC().Truncate(24*time.Hour))
		return fmt.Sprintf("<b>Status</b>\n%s\nMode: %s\n\nModal (equity): <b>%s USDT</b>\n• untung/rugi posisi yang masih terbuka: %s USDT\nPosisi terbuka: %d · nilai %s USDT\n\n<b>Hari ini (sejak 07:00 WIB)</b>\n%s\n\nDetail: /positions · /pnl",
			state, modeText(s.cfg.App.Mode), usdt(total.Equity), signed(total.UnrealizedPnL),
			total.OpenPositions, usdt(total.Exposure), statsLines(today))
	})

	bot.Handle("positions", "posisi terbuka beserta stop loss, take profit, dan untung/rugi berjalan", func(context.Context, []string) string {
		protect := map[string][]domain.Order{}
		for _, o := range s.eng.OpenOrders() {
			if o.Tag == domain.TagStopLoss || o.Tag == domain.TagTakeProfit {
				protect[o.Symbol] = append(protect[o.Symbol], o)
			}
		}
		var sb strings.Builder
		sb.WriteString("<b>Posisi terbuka</b>\n")
		n := 0
		for _, acct := range s.book.Accounts() {
			for _, p := range acct.Positions() {
				n++
				mark, _ := s.eng.LastPrice(p.Exchange, p.Market, p.Symbol)
				upnl := p.UnrealizedPnL(mark)
				side := "Long (untung kalau harga naik)"
				if p.Side == domain.Short {
					side = "Short (untung kalau harga turun)"
				}
				fmt.Fprintf(&sb, "\n<b>%s</b> · %s · %s\nJumlah %s %s · masuk %s · sekarang %s\nUntung/rugi berjalan: <b>%s USDT</b> (%s)\n",
					esc(p.Symbol), side, esc(p.Strategy), p.Quantity, esc(s.base(p.PositionKey)),
					price(p.EntryPrice), price(mark), signed(upnl), pct(upnl, p.Quantity.Mul(p.EntryPrice)))
				guarded := false
				for _, o := range protect[p.Symbol] {
					label := "🛡 Stop loss"
					if o.Tag == domain.TagTakeProfit {
						label = "🎯 Take profit"
					} else {
						guarded = true
					}
					fmt.Fprintf(&sb, "%s: %s (%s dari sekarang)\n", label, price(o.StopPrice), pct(o.StopPrice.Sub(mark), mark))
				}
				if !guarded && p.Market == domain.Futures {
					sb.WriteString("⚠️ Tanpa stop loss\n")
				}
			}
		}
		if n == 0 {
			return "Tidak ada posisi terbuka saat ini."
		}
		return sb.String()
	})

	bot.Handle("orders", "order yang menunggu di exchange", func(context.Context, []string) string {
		orders := s.eng.OpenOrders()
		if len(orders) == 0 {
			return "Tidak ada order yang menunggu."
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "<b>Order menunggu (%d)</b>\n", len(orders))
		for _, o := range orders {
			at := o.Price
			if o.Type.IsConditional() {
				at = o.StopPrice
			}
			fmt.Fprintf(&sb, "• %s %s · %s @ %s · %s %s\n", sideText(o.Side), esc(o.Symbol), orderKind(o), price(at),
				o.Remaining(), esc(s.base(domain.PositionKey{Exchange: o.Exchange, Market: o.Market, Symbol: o.Symbol})))
		}
		return sb.String()
	})

	bot.Handle("balance", "saldo dan modal per akun", func(context.Context, []string) string {
		var sb strings.Builder
		sb.WriteString("<b>Saldo</b>\n")
		for _, acct := range s.book.Accounts() {
			snap := acct.Snapshot()
			fmt.Fprintf(&sb, "\n<b>%s</b>\nSaldo: %s USDT\nModal (equity): %s USDT\nBebas dipakai: %s USDT\n",
				esc(acct.Key.String()), usdt(snap.Balance), usdt(snap.Equity), usdt(snap.FreeMargin()))
		}
		sb.WriteString("\n<i>Saldo = uang tunai ditambah modal di posisi bot. Equity = saldo ditambah untung/rugi posisi yang masih terbuka.</i>")
		if s.cfg.App.Mode != config.ModePaper {
			sb.WriteString("\n<i>Koin yang tidak dibeli bot tidak dihitung dan tidak akan dijual.</i>")
		}
		return sb.String()
	})

	bot.Handle("pnl", "hasil trading hari ini dan sejak bot dinyalakan", func(ctx context.Context, _ []string) string {
		dayStart := time.Now().UTC().Truncate(24 * time.Hour)
		today, err := s.store.TradeStats(ctx, s.runID, dayStart)
		if err != nil {
			return "Gagal membaca data: " + esc(err.Error())
		}
		session, err := s.store.TradeStats(ctx, s.runID, time.Time{})
		if err != nil {
			return "Gagal membaca data: " + esc(err.Error())
		}
		by, _ := s.store.TradeStatsByStrategy(ctx, s.runID, time.Time{}, time.Now().Add(time.Hour))
		var sb strings.Builder
		fmt.Fprintf(&sb, "<b>Hari ini</b>\n%s\n\n<b>Sejak bot dinyalakan</b>\n%s\n", statsLines(today), statsLines(session))
		if per := perStrategyLines(by, s.strategyIDs()); per != "" {
			sb.WriteString("\n<b>Per strategi (sejak dinyalakan)</b>\n" + per)
		}
		sb.WriteString("\n<i>Win rate = persentase trade yang untung. Fee sudah dipotong dari hasil bersih.</i>")
		return sb.String()
	})

	bot.Handle("strategies", "strategi yang berjalan dan cara kerjanya", func(context.Context, []string) string {
		var sb strings.Builder
		sb.WriteString("<b>Strategi berjalan</b>\n")
		for _, sc := range s.eng.Strategies() {
			fmt.Fprintf(&sb, "\n<b>%s</b> · %s\n%s\n", esc(sc.ID), esc(strings.Join(sc.Symbols, ", ")), strategyText(sc.Name))
		}
		return sb.String()
	})

	bot.Handle("pause", "jeda: berhenti membuka posisi baru (posisi yang ada tetap dijaga)", func(context.Context, []string) string {
		s.eng.SetPaused(true)
		return "⏸ Dijeda. Bot tidak membuka posisi baru; stop loss dan take profit posisi yang ada tetap aktif.\nLanjutkan dengan /resume."
	})

	bot.Handle("resume", "lanjutkan setelah /pause", func(context.Context, []string) string {
		if killed, _ := s.risk.Halted(); killed {
			return "Kill switch masih aktif. Kirim <code>/reset confirm</code> dulu kalau yakin ingin trading lagi."
		}
		s.eng.SetPaused(false)
		return "▶️ Berjalan lagi: bot boleh membuka posisi baru."
	})

	bot.Handle("killswitch", "darurat: tutup SEMUA posisi dan hentikan trading", func(ctx context.Context, args []string) string {
		if !confirm(args) {
			return "⚠️ Ini menutup <b>semua</b> posisi dengan harga pasar dan menghentikan trading sampai di-reset.\nKalau yakin, kirim <code>/killswitch confirm</code>."
		}
		s.eng.Kill(ctx, "telegram /killswitch")
		return "🛑 Kill switch aktif: semua posisi sedang ditutup. Trading berhenti sampai <code>/reset confirm</code>."
	})

	bot.Handle("reset", "matikan kill switch dan izinkan trading lagi", func(_ context.Context, args []string) string {
		if !confirm(args) {
			return "Ini mengizinkan bot trading lagi setelah kill switch. Kirim <code>/reset confirm</code> untuk melanjutkan."
		}
		s.eng.ResetKillSwitch()
		return "✅ Kill switch dimatikan, trading berjalan lagi."
	})

	bot.Handle("stop", "matikan bot (posisi dan stop loss di exchange tetap ada)", func(_ context.Context, args []string) string {
		if !confirm(args) {
			return "Ini mematikan bot. Posisi terbuka dan stop loss-nya di exchange tetap ada, tetapi tidak ada yang mengelolanya sampai bot dinyalakan lagi.\nKirim <code>/stop confirm</code> kalau yakin."
		}
		go func() {
			time.Sleep(time.Second) // let the reply go out first
			stop()
		}()
		return "Mematikan bot…"
	})
}

// orderKind names an order for a beginner.
func orderKind(o domain.Order) string {
	switch o.Tag {
	case domain.TagStopLoss:
		return "stop loss"
	case domain.TagTakeProfit:
		return "take profit"
	case domain.TagEntry:
		return "order masuk"
	case domain.TagExit:
		return "order keluar"
	}
	if o.Type == domain.OrderLimit {
		return "limit (" + esc(o.Strategy) + ")"
	}
	return string(o.Type) + " (" + esc(o.Strategy) + ")"
}
