package main

// Telegram texts. They are written in Indonesian for a beginner: every
// number says what it means, and jargon (stop loss, equity, ...) comes with
// a short explanation.

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/fx"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
)

var esc = telegram.Escape

// modeText explains the trading mode, so nobody mistakes test money for real money.
func modeText(m config.Mode) string {
	switch m {
	case config.ModeLive:
		return "🔴 <b>LIVE — uang asli</b>"
	case config.ModeTestnet:
		return "🧪 Testnet — uang uji dari Binance, bukan uang asli"
	case config.ModePaper:
		return "📝 Paper — simulasi dengan harga asli, tanpa order ke exchange"
	}
	return esc(string(m))
}

// strategyText describes a strategy type in one line.
func strategyText(name string) string {
	switch name {
	case "emacross":
		return "ikut tren: masuk saat rata-rata harga cepat (EMA) memotong rata-rata lambat"
	case "dca":
		return "DCA: membeli rutin dengan nominal tetap, jual saat target untung tercapai"
	case "grid":
		return "grid: order beli dan jual berjenjang di dalam rentang harga"
	case "mm":
		return "market making: memasang harga beli dan jual di sekitar harga pasar"
	case "funding":
		return "funding: memungut biaya funding dengan posisi spot dan futures yang saling menutup"
	case "xspread":
		return "arbitrase selisih harga antar-exchange"
	}
	return name
}

func usdt(d decimal.Decimal) string { return d.StringFixed(2) }

// money writes USDT amounts with their US dollar and rupiah value when
// exchange rates are known (see internal/fx), e.g.
// "+4.80 USDT (≈ +$4.80 · +Rp 86.570)".
type money struct {
	rates fx.Rates
	ok    bool
}

// amount is a size or balance (no sign).
func (m money) amount(d decimal.Decimal) string {
	if !m.ok {
		return usdt(d) + " USDT"
	}
	return fmt.Sprintf("%s USDT (≈ $%s · %s)", usdt(d), d.Mul(m.rates.USD).StringFixed(2), rupiah(d.Mul(m.rates.IDR), false))
}

// exactAmount keeps commission precision so its USDT and rupiah values agree.
func (m money) exactAmount(d decimal.Decimal) string {
	if !m.ok {
		return d.String() + " USDT"
	}
	return fmt.Sprintf("%s USDT (≈ $%s · %s)", d.String(), d.Mul(m.rates.USD).String(), rupiah(d.Mul(m.rates.IDR), false))
}

// change is a profit or loss (always signed).
func (m money) change(d decimal.Decimal) string {
	if !m.ok {
		return signed(d) + " USDT"
	}
	usd := d.Mul(m.rates.USD)
	sign := ""
	switch {
	case usd.IsPositive():
		sign = "+"
	case usd.IsNegative():
		sign, usd = "-", usd.Neg()
	}
	return fmt.Sprintf("%s USDT (≈ %s$%s · %s)", signed(d), sign, usd.StringFixed(2), rupiah(d.Mul(m.rates.IDR), true))
}

// rateLine names the rate used, so the conversion can be judged.
func (m money) rateLine() string {
	if !m.ok {
		return "Kurs rupiah belum tersedia: nilai hanya dalam USDT."
	}
	return fmt.Sprintf("Kurs: 1 USDT ≈ $%s ≈ %s (%s, %s)", m.rates.USD.StringFixed(4), rupiah(m.rates.IDR, false),
		esc(m.rates.Source), m.rates.At.In(wib).Format("02 Jan 15:04 WIB"))
}

var wib = time.FixedZone("WIB", 7*3600)

// rupiah formats whole rupiah with Indonesian thousand separators: Rp 1.234.567.
func rupiah(d decimal.Decimal, withSign bool) string {
	v := d.Round(0)
	sign := ""
	switch {
	case v.IsNegative():
		sign, v = "-", v.Neg()
	case withSign && v.IsPositive():
		sign = "+"
	}
	digits := v.String()
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return sign + "Rp " + b.String()
}

func signed(d decimal.Decimal) string {
	if d.IsPositive() {
		return "+" + d.StringFixed(2)
	}
	return d.StringFixed(2)
}

// price shows enough decimals for both BTC (80000.12) and DOGE (0.07765).
func price(d decimal.Decimal) string {
	a := d.Abs()
	switch {
	case a.GreaterThanOrEqual(decimal.NewFromInt(1000)):
		return d.StringFixed(2)
	case a.GreaterThanOrEqual(decimal.NewFromInt(1)):
		return d.StringFixed(4)
	}
	return d.StringFixed(6)
}

// pct is part/whole in percent with a sign.
func pct(part, whole decimal.Decimal) string {
	if whole.IsZero() {
		return "0%"
	}
	v := part.Div(whole).Mul(decimal.NewFromInt(100))
	s := v.StringFixed(2) + "%"
	if v.IsPositive() {
		s = "+" + s
	}
	return s
}

func duration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d detik", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d menit", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d jam %d menit", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%d hari", int(d.Hours()/24))
}

func sideText(s domain.Side) string {
	if s == domain.Buy {
		return "BELI"
	}
	return "JUAL"
}

var (
	reEMACross = regexp.MustCompile(`^ema cross (up|down), rsi ([\d.]+)$`)
	reDCABuy   = regexp.MustCompile(`^dca buy (\d+)/(\d+)$`)
	reGrid     = regexp.MustCompile(`^grid level (\d+)$`)
)

// reasonText turns a strategy's entry reason into a sentence.
func reasonText(reason string) string {
	if m := reEMACross.FindStringSubmatch(reason); m != nil {
		dir := "naik: EMA cepat memotong ke atas EMA lambat"
		if m[1] == "down" {
			dir = "turun: EMA cepat memotong ke bawah EMA lambat"
		}
		return fmt.Sprintf("Sinyal tren %s (RSI %s; RSI di atas 70 berarti sudah terlalu mahal)", dir, m[2])
	}
	if m := reDCABuy.FindStringSubmatch(reason); m != nil {
		return fmt.Sprintf("Pembelian rutin ke-%s dari maksimal %s", m[1], m[2])
	}
	if m := reGrid.FindStringSubmatch(reason); m != nil {
		return "Order di tingkat grid ke-" + m[1]
	}
	switch reason {
	case "mm quote":
		return "Harga beli/jual market making"
	case "dca take profit":
		return "Target untung DCA tercapai"
	case "dca stop loss":
		return "Batas rugi DCA tersentuh"
	case "grid stop":
		return "Harga keluar di bawah rentang grid, semua order grid ditutup"
	case "opposite ema cross":
		return "Tren berbalik (EMA memotong ke arah sebaliknya)"
	}
	return esc(reason)
}

// exitText explains why a position was closed (the trade's exit tag).
func exitText(tag string) string {
	switch tag {
	case domain.TagStopLoss:
		return "🛡 Kena stop loss: batas stop tersentuh; rincian stop awal/trailing belum tercatat."
	case domain.TagTakeProfit:
		return "🎯 Target untung (take profit) tercapai"
	case domain.TagExit:
		return "Posisi ditutup atas permintaan bot; alasan rinci belum tercatat (sinyal, manual, atau batas risiko)."
	case domain.TagStrategy:
		return "Order jual dari strategi (grid/DCA/market making)"
	}
	return esc(tag)
}

func msgStarted(m config.Mode, strategies []strategy.Config, version string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "🤖 <b>Bot aktif</b>\nMode: %s\n", modeText(m))
	if len(strategies) == 0 {
		sb.WriteString("Tidak ada strategi aktif: bot hanya merekam data harga.\n")
	} else {
		fmt.Fprintf(&sb, "\n<b>Strategi berjalan (%d)</b>\n", len(strategies))
		for _, sc := range strategies {
			fmt.Fprintf(&sb, "• <b>%s</b> · %s\n  %s\n", esc(sc.ID), esc(strings.Join(sc.Symbols, ", ")), strategyText(sc.Name))
		}
	}
	fmt.Fprintf(&sb, "\nKetik /help untuk daftar perintah.\n<i>versi %s</i>", esc(shortVersion(version)))
	return sb.String()
}

func msgStopped(m config.Mode, err error) string {
	var sb strings.Builder
	sb.WriteString("🛑 <b>Bot berhenti</b>\n")
	if err != nil {
		fmt.Fprintf(&sb, "Penyebab: kesalahan, <code>%s</code>\n", esc(err.Error()))
	}
	if m == config.ModePaper {
		sb.WriteString("Posisi simulasi disimpan dan dilanjutkan saat bot jalan lagi.")
	} else {
		sb.WriteString("Posisi yang masih terbuka tetap dilindungi stop loss di exchange.")
	}
	return sb.String()
}

// msgEntry announces a filled entry with its protection.
func msgEntry(o domain.Order, base, stratID string, stop, target decimal.Decimal, m money) string {
	notional := o.FilledQty.Mul(o.AvgPrice)
	var sb strings.Builder
	fmt.Fprintf(&sb, "🔵 <b>Posisi dibuka: %s %s</b>\nStrategi: %s\n\n", sideText(o.Side), esc(o.Symbol), esc(stratID))
	fmt.Fprintf(&sb, "Jumlah: %s %s · %s\nHarga masuk: %s\n", o.FilledQty, esc(base), m.amount(notional), price(o.AvgPrice))
	sign := decimal.NewFromInt(1)
	if o.Side == domain.Sell {
		sign = sign.Neg()
	}
	if stop.IsPositive() {
		loss := stop.Sub(o.AvgPrice).Mul(o.FilledQty).Mul(sign)
		fmt.Fprintf(&sb, "🛡 Stop loss: %s (%s)\n   perkiraan rugi di harga stop ≈ %s\n   Belum termasuk komisi dan selisih harga eksekusi; bukan batas rugi yang dijamin.\n", price(stop), pct(stop.Sub(o.AvgPrice), o.AvgPrice), m.change(loss))
	} else {
		sb.WriteString("⚠️ Tanpa stop loss\n")
	}
	if target.IsPositive() {
		gain := target.Sub(o.AvgPrice).Mul(o.FilledQty).Mul(sign)
		fmt.Fprintf(&sb, "🎯 Take profit: %s (%s)\n   untung ≈ %s\n", price(target), pct(target.Sub(o.AvgPrice), o.AvgPrice), m.change(gain))
	}
	fmt.Fprintf(&sb, "\nAlasan: %s", reasonText(o.Reason))
	return sb.String()
}

// msgStrategyFill announces a completed raw strategy order (DCA buys, grid
// levels): they open or reduce inventory without a stop loss of their own.
func msgStrategyFill(o domain.Order, base string, m money) string {
	icon := "🟦"
	if o.Side == domain.Sell {
		icon = "🟪"
	}
	return fmt.Sprintf("%s <b>%s %s terisi</b> · %s\nJumlah: %s %s @ %s\nNilai: %s\n%s",
		icon, sideText(o.Side), esc(o.Symbol), esc(o.Strategy), o.FilledQty, esc(base), price(o.AvgPrice),
		m.amount(o.FilledQty.Mul(o.AvgPrice)), reasonText(o.Reason))
}

func msgTradeClosed(t domain.Trade, base string, m money, instanceCapital decimal.Decimal) string {
	net := t.NetPnL()
	cost := t.Quantity.Mul(t.EntryPrice)
	head := "🔴 <b>Posisi ditutup: RUGI</b>"
	if net.IsPositive() {
		head = "🟢 <b>Posisi ditutup: UNTUNG</b>"
	}
	side := "Long (beli dulu)"
	if t.Side == domain.Short {
		side = "Short (jual dulu)"
	}
	impact := "Dampak terhadap modal instance: belum tersedia (modal acuan belum dicatat)."
	if instanceCapital.IsPositive() {
		impact = "Dampak transaksi terhadap modal acuan instance: " + pct(net, instanceCapital) + " (acuan " + m.amount(instanceCapital) + ")."
	}
	return fmt.Sprintf("%s\n<b>%s</b>\nHasil posisi: %s dari nilai pembelian\n%s\n%s · %s · %s\n\nMasuk %s → Keluar %s\nJumlah: %s %s · modal %s · lama %s\nKotor %s USDT · komisi %s\n\nAlasan keluar: %s\nWaktu keluar: %s",
		head, m.change(net), pct(net, cost), impact, esc(t.Symbol), side, esc(t.Strategy),
		price(t.EntryPrice), price(t.ExitPrice), t.Quantity, esc(base), m.amount(cost), duration(t.ClosedAt.Sub(t.OpenedAt)),
		signed(t.GrossPnL), m.exactAmount(t.Fees), exitText(t.ExitReason), telegramTime(t.ClosedAt))
}

// noticeText gives the engine's English notices a plain explanation.
var noticeText = map[string]string{
	"KILL SWITCH: trading halted":                                                "Kill switch aktif: batas rugi tercapai. Bot berhenti membuka posisi baru dan menutup semua posisi.",
	"KILL SWITCH: manual":                                                        "Kill switch dinyalakan manual: semua posisi ditutup dan trading dihentikan.",
	"kill switch reset, trading resumed":                                         "Kill switch direset, trading berjalan lagi.",
	"stop loss rejected, closing position":                                       "Exchange menolak stop loss, jadi posisi ditutup demi keamanan.",
	"protective orders failed, closing position":                                 "Stop loss/take profit gagal dipasang, jadi posisi ditutup demi keamanan.",
	"stop loss failed, closing position":                                         "Stop loss gagal dipasang, jadi posisi ditutup demi keamanan.",
	"stop already crossed, exiting at market":                                    "Harga sudah melewati stop loss, posisi ditutup dengan harga pasar.",
	"exit order failed, position still open":                                     "Order penutupan gagal: posisi MASIH TERBUKA, mohon dicek.",
	"exit order did not fill":                                                    "Order penutupan tidak terisi: posisi mungkin masih terbuka.",
	"order blocked by risk limits":                                               "Order dibatalkan karena melewati batas risk.",
	"position closed outside the bot (manual, liquidation or missed fill)":       "Posisi tertutup di luar bot (manual, likuidasi, atau fill yang terlewat).",
	"position corrected from exchange":                                           "Posisi disesuaikan dengan data di exchange.",
	"position loaded from exchange":                                              "Posisi dimuat dari exchange.",
	"open position without stop loss on the exchange":                            "Ada posisi tanpa stop loss di exchange: risikonya tidak dibatasi.",
	"restored position has no stop loss":                                         "Posisi yang dipulihkan tidak punya stop loss.",
	"protective order without a position canceled":                               "Order stop loss/take profit sisa (tanpa posisi) dibatalkan.",
	"take profit ended unfilled after its linked stop expired, closing position": "Take profit gagal terisi setelah stop loss pasangannya kedaluwarsa, jadi posisi ditutup.",
	"pending paper entry canceled on restart":                                    "Order masuk yang belum terisi dibatalkan saat restart.",
}

func msgNotice(level slog.Level, msg string, attrs ...any) string {
	icon, label := "ℹ️", "Info"
	switch {
	case level >= slog.LevelError:
		icon, label = "🚨", "Penting"
	case level >= slog.LevelWarn:
		icon, label = "⚠️", "Peringatan"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s <b>%s</b>\n", icon, label)
	if text, ok := noticeText[msg]; ok {
		sb.WriteString(esc(text))
	} else {
		sb.WriteString(esc(msg))
	}
	for i := 0; i+1 < len(attrs); i += 2 {
		fmt.Fprintf(&sb, "\n• %v: %s", attrs[i], esc(fmt.Sprint(attrs[i+1])))
	}
	return sb.String()
}

func winRate(st storage.TradeStats) string {
	if st.Trades == 0 {
		return "-"
	}
	return fmt.Sprintf("%d%%", st.Wins*100/st.Trades)
}

// statsLines describes trade statistics of a period.
func statsLines(st storage.TradeStats, m money) string {
	if st.Trades == 0 {
		return "Belum ada trade yang selesai."
	}
	return fmt.Sprintf("Trade: %d (untung %d · rugi %d · win rate %s)\nHasil bersih: <b>%s</b>\nFee (sudah dipotong): %s\nTerbaik %s · terburuk %s USDT",
		st.Trades, st.Wins, st.Trades-st.Wins, winRate(st), m.change(st.NetPnL), m.amount(st.Fees), signed(st.Best), signed(st.Worst))
}

func perStrategyLines(by map[string]storage.TradeStats, ids []string, m money) string {
	var sb strings.Builder
	for _, id := range ids {
		st, ok := by[id]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "• %s: %d trade, win %s\n   %s\n", esc(id), st.Trades, winRate(st), m.change(st.NetPnL))
	}
	return sb.String()
}

func msgDaily(day time.Time, st storage.TradeStats, by map[string]storage.TradeStats, ids []string,
	equity decimal.Decimal, open int, mode config.Mode, m money) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "📊 <b>Ringkasan harian %s</b>\n%s\n\n%s\n", day.Format("02 Jan 2006"), modeText(mode), statsLines(st, m))
	if per := perStrategyLines(by, ids, m); per != "" {
		sb.WriteString("\n<b>Per strategi</b>\n" + per)
	}
	fmt.Fprintf(&sb, "\nModal (equity): %s\nPosisi terbuka: %d\n\n<i>%s</i>", m.amount(equity), open, m.rateLine())
	return sb.String()
}

func shortVersion(v string) string {
	if len(v) > 12 {
		return v[:7]
	}
	return v
}

func msgManagedStarted(ctx context.Context, runtimes map[string]*managed.Runtime, m money, version string) string {
	return "🤖 <b>Bot aktif</b>\n" + managedStatus(ctx, runtimes, m) + "\nKetik / untuk menu perintah.\n<i>versi " + esc(shortVersion(version)) + "</i>"
}

func msgManagedStopped(m config.Mode, err error) string {
	var sb strings.Builder
	sb.WriteString("🛑 <b>Bot berhenti</b>\n")
	if err != nil {
		fmt.Fprintf(&sb, "Penyebab: kesalahan, <code>%s</code>\n", esc(err.Error()))
	}
	if m == config.ModePaper {
		sb.WriteString("Ledger, posisi dan order simulasi tersimpan; dilanjutkan saat bot jalan lagi.")
	} else {
		sb.WriteString("Order dan koin di exchange tetap ada; tidak ada penjualan otomatis saat berhenti.")
	}
	return sb.String()
}
