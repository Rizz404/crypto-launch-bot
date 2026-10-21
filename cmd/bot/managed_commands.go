package main

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

func managedReport(ctx context.Context, runtimes map[string]*managed.Runtime, view string, args []string, m money) string {
	if len(args) > 3 {
		return "Gunakan /" + view + " [akun] [sasaran] [halaman]; sasaran = account (semua instance), ID instance, atau instance/strategi."
	}
	page := 1
	if len(args) == 3 {
		n, err := strconv.Atoi(args[2])
		if err != nil || n < 1 || n > 10000 {
			return "Nomor halaman harus berupa angka 1–10000. Contoh: /" + view + " akun account 2."
		}
		page = n
	}
	ids := slices.Sorted(maps.Keys(runtimes))
	if len(args) > 0 {
		if runtimes[args[0]] == nil {
			return "Akun tidak ditemukan."
		}
		ids = []string{args[0]}
	}
	scope := "account"
	if len(args) >= 2 {
		scope = args[1]
	}
	var out strings.Builder
	for _, id := range ids {
		s, err := runtimes[id].Snapshot(ctx)
		if err != nil {
			fmt.Fprintf(&out, "\n<b>%s</b>: catatan bot belum bisa dibaca: %s\n", esc(id), esc(err.Error()))
			continue
		}
		if scope != "account" && s.Instances[scope] == nil && s.Bindings[scope] == nil {
			return "Sasaran tidak ditemukan. Gunakan account (semua instance), ID instance, atau instance/strategi dari /strategies."
		}
		fmt.Fprintf(&out, "\n<b>Akun: %s</b>\n%s\n", esc(id), modeText(config.Mode(s.Spec.Mode)))
		if scope != "account" {
			fmt.Fprintf(&out, "Sasaran: <code>%s</code>\n", esc(scope))
		}
		out.WriteString(managedReportState(s, view, scope, m, page))
		if view == "positions" || view == "orders" || view == "strategies" || (view == "pnl" && scope != "account") {
			fmt.Fprintf(&out, "Halaman lain: <code>/%s %s %s 2</code> (ganti 2 dengan nomor halaman).\n", view, esc(id), esc(scope))
		}
		if s.Attention != "" {
			fmt.Fprintf(&out, "⚠️ Perlu diperiksa: %s\n", esc(s.Attention))
		}
	}
	fmt.Fprintf(&out, "\nDetail: <code>/%s akun instance</code> atau <code>/%s akun instance/strategi</code>\n<i>%s</i>", view, view, m.rateLine())
	return out.String()
}

func managedReportState(s *managed.State, view, scope string, m money, page int) string {
	var out strings.Builder
	shown, total := 0, 0
	offset := (page - 1) * 5
	owners := []string{}
	for _, owner := range slices.Sorted(maps.Keys(s.Bindings)) {
		if scope == "account" || scope == owner || scope == s.Bindings[owner].InstanceID {
			owners = append(owners, owner)
		}
	}
	switch view {
	case "status":
		positions, orders, allowed, unavailable := 0, 0, 0, 0
		for _, h := range s.Holdings {
			if slices.Contains(owners, h.Owner) && h.Quantity.IsPositive() {
				positions++
			}
		}
		for _, o := range s.Orders {
			if slices.Contains(owners, o.Owner) && o.State != "terminal" {
				orders++
			}
		}
		for _, owner := range owners {
			if !slices.Contains(strategy.Names(), s.Bindings[owner].Spec.Name) {
				unavailable++
			}
			if managedEntryStatus(s, owner) == "Diizinkan mencari peluang beli" {
				allowed++
			}
		}
		fmt.Fprintf(&out, "<b>Ringkasan bot</b>\nProses bot: aktif\nModal seluruh akun: %s\nCadangan: %s\nInstance seluruh akun: %d · strategi dalam laporan: %d\nImplementasi tersedia: %d · masih rancangan: %d\nIzin entry menurut pengaturan: %d dari %d strategi tersedia\nPosisi terbuka dalam laporan: %d · order belum selesai: %d\n\nIzin entry belum membuktikan kesiapan operasi atau adanya sinyal beli.\nLihat /strategies untuk keadaan setiap strategi.\n", m.amount(s.Equity()), m.amount(s.Reserve), len(s.Instances), len(owners), len(owners)-unavailable, unavailable, allowed, len(owners)-unavailable, positions, orders)
	case "balance":
		if scope == "account" {
			cash, free := s.Reserve, s.Reserve
			for id, i := range s.Instances {
				cash, free = cash.Add(i.Cash), free.Add(s.FreeCash(id))
			}
			q, _ := s.Reserved("", "", "")
			fmt.Fprintf(&out, "<b>Saldo bot</b>\nNilai modal saat ini: %s\nUang tunai: %s\nDana untuk order beli: %s\nUang tersedia: %s\nCadangan belum dialokasikan: %s\n", m.amount(s.Equity()), m.amount(cash), m.amount(q), m.amount(free), m.amount(s.Reserve))
		}
		for _, id := range slices.Sorted(maps.Keys(s.Instances)) {
			if scope == "account" || (scope != id && (s.Bindings[scope] == nil || s.Bindings[scope].InstanceID != id)) {
				continue
			}
			q, _ := s.Reserved(id, "", "")
			fmt.Fprintf(&out, "<b>Modal bersama instance %s</b>\nNilai modal saat ini: %s\nUang tunai: %s\nDana untuk order beli: %s\nUang tersedia: %s\n", esc(id), m.amount(s.InstanceEquity(id)), m.amount(s.Instances[id].Cash), m.amount(q), m.amount(s.FreeCash(id)))
		}
		out.WriteString("Nilai modal = uang tunai + nilai koin yang dipegang. Strategi berbagi uang instance; saldo pribadi di exchange tidak dihitung.\n")
	case "pnl":
		fmt.Fprintf(&out, "<b>Untung/rugi sejak awal eksperimen</b>\nMulai: %s\nRiwayat tetap dihitung setelah bot dinyalakan ulang.\n", telegramTime(s.StartedAt))
		realized, unrealized, fees := decimal.Zero, decimal.Zero, decimal.Zero
		idle := 0
		for _, owner := range owners {
			b := s.Bindings[owner]
			u := s.Unrealized(owner)
			realized, unrealized, fees = realized.Add(b.Realized), unrealized.Add(u), fees.Add(b.Fees)
			if scope != "account" {
				if b.Realized.IsZero() && u.IsZero() && b.Fees.IsZero() && s.Inventory(owner).IsZero() {
					idle++
					continue
				}
				total++
				if total > offset && shown < 5 {
					shown++
					fmt.Fprintf(&out, "• %s\n  Hasil transaksi tercatat: %s\n  Perubahan nilai posisi terbuka: %s\n  Komisi tercatat: %s\n", esc(owner), m.change(b.Realized), m.change(u), m.exactAmount(b.Fees))
				}
			}
		}
		if idle > 0 {
			fmt.Fprintf(&out, "%d strategi tanpa posisi, hasil, dan komisi tercatat saat ini; rincian nol diringkas.\n", idle)
		}
		fmt.Fprintf(&out, "Total bersih saat ini: <b>%s</b>\nHasil transaksi tercatat: %s\nPerubahan nilai posisi terbuka: %s\nKomisi tercatat: %s (sudah diperhitungkan).\nNilai posisi terbuka masih bisa berubah; biaya penjualan berikutnya belum dipotong.\n", m.change(realized.Add(unrealized)), m.change(realized), m.change(unrealized), m.exactAmount(fees))
		if s.FeeIssue != "" {
			fmt.Fprintf(&out, "⚠️ Perhitungan komisi belum lengkap: %s\n", esc(s.FeeIssue))
		}
	case "positions":
		for _, key := range slices.Sorted(maps.Keys(s.Holdings)) {
			h := s.Holdings[key]
			if !slices.Contains(owners, h.Owner) || !h.Quantity.IsPositive() {
				continue
			}
			total++
			if total <= offset || shown == 5 {
				continue
			}
			shown++
			mark := s.Prices[h.Symbol]
			label := "harga terakhir"
			if !mark.IsPositive() {
				mark, label = h.Cost.Div(h.Quantity), "harga biaya; harga pasar belum tersedia"
			}
			fmt.Fprintf(&out, "• %s · %s\n  Jumlah: %s · modal pembelian: %s\n  %s: %s\n  Untung/rugi sementara: %s\n", esc(h.Owner), esc(h.Symbol), h.Quantity, m.amount(h.Cost), label, price(mark), m.change(h.Quantity.Mul(mark).Sub(h.Cost)))
			fmt.Fprintf(&out, "  Waktu candle harga: %s\n", telegramTime(s.PriceAt[h.Symbol]))
		}
		if total == 0 {
			out.WriteString("Tidak ada posisi terbuka.\n")
		}
		out.WriteString("Order pelindung: /orders\n")
	case "orders":
		for _, id := range slices.Sorted(maps.Keys(s.Orders)) {
			o := s.Orders[id]
			if o.State == "terminal" || !slices.Contains(owners, o.Owner) {
				continue
			}
			total++
			if total <= offset || shown == 5 {
				continue
			}
			shown++
			fmt.Fprintf(&out, "• %s · %s · %s\n  ID: <code>%s</code>\n  Keadaan: %s\n  Terisi: %s dari %s\n", esc(o.Owner), esc(o.Order.Symbol), sideText(o.Order.Side), esc(id), managedOrderStatus(o), o.Order.FilledQty, o.Order.Quantity)
			if o.Order.Price.IsPositive() {
				fmt.Fprintf(&out, "  Harga pesanan: %s\n", price(o.Order.Price))
			}
			if o.Order.StopPrice.IsPositive() {
				fmt.Fprintf(&out, "  Harga pemicu: %s\n", price(o.Order.StopPrice))
			}
		}
		if total == 0 {
			out.WriteString("Tidak ada order aktif atau menunggu verifikasi.\n")
		}
	case "strategies":
		for _, owner := range owners {
			total++
			if total <= offset || shown == 5 {
				continue
			}
			shown++
			b := s.Bindings[owner]
			i := s.Instances[b.InstanceID]
			fmt.Fprintf(&out, "• <b>%s</b> · %s\n  ID: <code>%s</code>\n  %s\n", managedStrategyName(b.Spec), esc(strings.Join(b.Spec.Symbols, ", ")), esc(owner), managedEntryStatus(s, owner))
			if tf, ok := b.Spec.Params["timeframe"]; ok {
				fmt.Fprintf(&out, "  Timeframe: %s\n", esc(fmt.Sprint(tf)))
			}
			if scope == "account" {
				continue
			}
			for _, reason := range []string{s.Reason, i.Reason, b.Reason} {
				if reason != "" {
					fmt.Fprintf(&out, "  Keterangan: %s\n", esc(reason))
				}
			}
			if b.LastNotice != "" {
				fmt.Fprintf(&out, "  Catatan terakhir (bisa berasal dari kejadian sebelumnya): %s\n", esc(b.LastNotice))
			}
			if b.Spec.Cap.IsPositive() {
				fmt.Fprintf(&out, "  Batas modal: %s\n", m.amount(b.Spec.Cap))
			}
		}
		if len(owners) == 0 {
			out.WriteString("Tidak ada strategi terdaftar.\n")
		}
	}
	if total > shown {
		fmt.Fprintf(&out, "\nDitampilkan %d dari %d · halaman %d/%d. Gunakan nomor halaman atau filter instance/strategi.\n", shown, total, page, (total+4)/5)
	}
	fmt.Fprintf(&out, "\nWaktu laporan: %s\n", telegramTime(time.Now()))
	return out.String()
}

func telegramTime(at time.Time) string {
	if at.IsZero() {
		return "belum tercatat"
	}
	return at.In(wib).Format("02 Jan 2006 15:04:05 WIB")
}

func managedStrategyName(b managed.BindingSpec) string {
	if b.Name == "trendspot" {
		switch b.Params["mode"] {
		case "momentum":
			return "Momentum mengikuti tren"
		case "pullback":
			return "Pullback dalam tren naik"
		case "breakout", nil:
			return "Breakout rentang harga"
		}
	}
	switch b.Name {
	case "squeeze":
		return "Breakout setelah volatilitas menyempit"
	case "mean-reversion-sideways":
		return "Kembali ke rata-rata saat sideways"
	case "rotation":
		return "Rotasi momentum antarkoin"
	}
	return esc(strategyText(b.Name))
}

func managedEntryStatus(s *managed.State, owner string) string {
	b := s.Bindings[owner]
	i := s.Instances[b.InstanceID]
	if !slices.Contains(strategy.Names(), b.Spec.Name) {
		return "Belum tersedia — menunggu implementasi"
	}
	for _, status := range []string{i.Status, b.Status} {
		switch status {
		case "archived":
			return "Sudah diarsipkan"
		case "stopped":
			return "Strategi dihentikan"
		case "closing":
			return "Sedang menyelesaikan penghentian; periksa /orders"
		}
	}
	if s.Attention != "" {
		return "Entry ditahan: ada catatan akun yang perlu diperiksa"
	}
	if s.Halted || i.Halted || b.Halted {
		return "Entry dihentikan oleh batas risiko atau perintah darurat"
	}
	if i.Status != "running" || b.Status != "running" {
		return "Entry dijeda; posisi yang ada tetap dipantau"
	}
	if strings.Contains(b.Reason, "restart runtime") {
		return "Menunggu penyiapan strategi; proses perlu dimulai ulang"
	}
	return "Diizinkan mencari peluang beli"
}

func managedOrderStatus(o *managed.Order) string {
	switch o.State {
	case "unknown":
		return "Hasil belum pasti; perlu verifikasi"
	case "submitting":
		return "Sedang dikirim; belum dikonfirmasi"
	case "canceling":
		return "Pembatalan diminta; menunggu konfirmasi"
	}
	if o.Order.FilledQty.IsPositive() {
		return "Terisi sebagian; sisa order masih dipantau"
	}
	if o.Order.Type == domain.OrderStopMarket {
		return "Stop loss menunggu harga pemicu"
	}
	if o.Order.Type == domain.OrderTakeProfit {
		return "Target untung menunggu harga pemicu"
	}
	return "Order aktif; menunggu eksekusi"
}

func managedControlReply(c managed.Command, wallet string) string {
	actions := map[string]string{
		"pause":   "Entry baru dijeda. Posisi dan order yang sudah ada tetap dipantau.",
		"resume":  "Izin mencari entry dilanjutkan. Pembelian tetap menunggu sinyal, saldo dan pemeriksaan risiko.",
		"cancel":  "Entry dijeda dan pembatalan order entry/strategi diminta. Koin tidak otomatis dijual; order pelindung dipertahankan. Pembatalan belum selesai sampai dikonfirmasi.",
		"flatten": "Entry dijeda dan penutupan posisi diminta. Posisi belum dianggap tertutup sampai hasil order dikonfirmasi.",
		"finish":  "Pengakhiran eksperimen diminta. Tunggu penyelesaian order sebelum menganggap eksperimen selesai.",
		"kill":    "Penghentian darurat diterapkan: entry diblokir dan penutupan posisi diminta. Periksa /orders untuk hasilnya.",
		"reset":   "Penghentian risiko direset dan izin entry dibuka pada sasaran ini. Batas akun dan instance tetap berlaku.",
		"archive": "Instance kosong sudah diarsipkan.",
	}
	text := actions[c.Operation]
	if c.Operation == "finish" {
		if c.Policy == "retain" {
			text += " Koin dipertahankan; perlindungan order dapat dibatalkan saat eksperimen diakhiri."
		} else {
			text += " Posisi diminta untuk dijual."
		}
	}
	return "✅ Akun: <b>" + esc(wallet) + "</b>\nSasaran: <code>" + esc(c.Scope) + "</code>\n" + text
}

func managedControlError(err error) string {
	switch err.Error() {
	case "explicit confirmation required":
		return "Tindakan ini memerlukan konfirmasi. Periksa sasaran, lalu tambahkan kata confirm pada perintah yang sama."
	case "choose retain or flatten inventory":
		return "Pilih retain untuk mempertahankan koin, atau flatten untuk menjualnya, lalu tambahkan confirm."
	case "unknown scope":
		return "Sasaran tidak ditemukan. Periksa ID instance/strategi di /strategies."
	case "wallet needs reconciliation":
		return "Catatan saldo akun perlu diperiksa sebelum entry dapat dilanjutkan."
	default:
		return esc(err.Error())
	}
}
