package main

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/shopspring/decimal"
)

func configureManagedNotifications(ctx context.Context, rt *managed.Runtime, wallet string, moneyFor func() money, notify func(string), log *slog.Logger) {
	stateFor := func(owner string, level slog.Level) *managed.State {
		s, err := rt.Snapshot(ctx)
		if err != nil {
			log.Warn("managed notification snapshot", "wallet", wallet, "err", err)
			return nil
		}
		b := s.Bindings[owner]
		if b == nil {
			return nil
		}
		pref := s.Instances[b.InstanceID].Notify
		if pref == "off" {
			return nil
		}
		threshold := slog.LevelWarn
		if pref != "" {
			_ = threshold.UnmarshalText([]byte(pref))
		}
		if level < threshold {
			return nil
		}
		return s
	}
	last := map[string]time.Time{}
	rt.OnNotice = func(owner string, level slog.Level, msg string) {
		// Keep only the current minute, even when notice text contains order IDs.
		for key, at := range last {
			if time.Since(at) >= time.Minute {
				delete(last, key)
			}
		}
		key := owner + "\x00" + level.String() + "\x00" + msg
		if _, ok := last[key]; ok {
			return
		}
		if stateFor(owner, level) != nil {
			last[key] = time.Now()
			notify("<b>" + esc(wallet+" / "+owner) + "</b>\n" + esc(msg))
		}
	}
	rt.OnExecution = func(execution managed.Execution, order domain.Order) {
		s := stateFor(execution.Owner, slog.LevelInfo)
		if s == nil || s.Bindings[execution.Owner].Spec.Name == "mm" {
			return
		}
		notify(msgManagedExecution(s, execution, order, moneyFor()))
	}
}

func msgManagedExecution(s *managed.State, execution managed.Execution, order domain.Order, m money) string {
	f := execution.Fill
	var out strings.Builder
	fmt.Fprintf(&out, "<b>Akun: %s / %s</b>\n%s\n", esc(s.Spec.ID), esc(execution.Owner), modeText(config.Mode(s.Spec.Mode)))
	if f.Side == domain.Buy {
		fmt.Fprintf(&out, "🔵 <b>Pembelian terisi: %s</b>\nJumlah: %s %s @ %s\nNilai: %s\nAlasan: %s", esc(f.Symbol), f.Quantity, esc(strings.TrimSuffix(f.Symbol, s.Spec.Quote)), price(f.Price), m.amount(f.Quantity.Mul(f.Price)), reasonText(order.Reason))
		for _, id := range slices.Sorted(maps.Keys(s.Orders)) {
			o := s.Orders[id]
			if o.Owner != execution.Owner || o.Order.Symbol != f.Symbol || o.State == "terminal" {
				continue
			}
			switch o.Order.Tag {
			case domain.TagStopLoss:
				fmt.Fprintf(&out, "\n🛡 Stop loss tercatat: %s", price(o.Order.StopPrice))
			case domain.TagTakeProfit:
				fmt.Fprintf(&out, "\n🎯 Take profit tercatat: %s", price(o.Order.StopPrice))
			}
		}
	} else {
		fmt.Fprintf(&out, "🟪 <b>Penjualan terisi: %s</b>\nJumlah: %s @ %s\nNilai: %s\nHasil terealisasi eksekusi: %s\nTermasuk biaya pada modal yang dijual dan biaya penjualan yang sudah dinilai.\nAlasan: %s", esc(f.Symbol), f.Quantity, price(f.Price), m.amount(f.Quantity.Mul(f.Price)), m.change(execution.Realized), exitText(order.Tag))
	}
	remaining := decimal.Zero
	for _, h := range s.Holdings {
		if h.Owner == execution.Owner && h.Symbol == f.Symbol {
			remaining = h.Quantity
		}
	}
	fee := m.exactAmount(f.Fee)
	if !managedFeeKnown(f, s.Spec.Quote) {
		fee = "nilai quote belum lengkap pada eksekusi; lihat total komisi di /pnl"
	}
	fmt.Fprintf(&out, "\nKomisi eksekusi: %s\nSisa posisi tercatat: %s %s\nWaktu eksekusi: %s", fee, remaining, esc(strings.TrimSuffix(f.Symbol, s.Spec.Quote)), telegramTime(f.Time))
	if s.FeeIssue != "" {
		fmt.Fprintf(&out, "\n⚠️ Penilaian komisi perlu diperiksa: %s", esc(s.FeeIssue))
	}
	return out.String()
}

// Third-asset fees may be valued later, outside the execution record.
func managedFeeKnown(f domain.Fill, quote string) bool {
	return !f.FeeUnknown && (f.FeeAsset == "" || f.FeeAsset == "QUOTE" || f.FeeAsset == quote || f.FeeAsset == strings.TrimSuffix(f.Symbol, quote))
}

// The daily boundary is UTC midnight (07:00 WIB), as in the existing scheduler.
func managedDaily(ctx context.Context, runtimes map[string]*managed.Runtime, day time.Time, m money) string {
	end := day.Add(24 * time.Hour)
	var out strings.Builder
	fmt.Fprintf(&out, "📊 <b>Ringkasan harian</b>\nPeriode: %s → %s (akhir tidak termasuk)\n", telegramTime(day), telegramTime(end))
	for _, id := range slices.Sorted(maps.Keys(runtimes)) {
		s, err := runtimes[id].Snapshot(ctx)
		if err != nil {
			fmt.Fprintf(&out, "\n<b>Akun: %s</b>\nCatatan bot belum bisa dibaca: %s\n", esc(id), esc(err.Error()))
			continue
		}
		buys, sells := 0, 0
		realized, fees := decimal.Zero, decimal.Zero
		feesKnown := true
		for _, e := range s.Executions {
			if e.Fill.Time.Before(day) || !e.Fill.Time.Before(end) {
				continue
			}
			if e.Fill.Side == domain.Buy {
				buys++
			} else {
				sells++
			}
			realized = realized.Add(e.Realized)
			fees = fees.Add(e.Fill.Fee)
			feesKnown = feesKnown && managedFeeKnown(e.Fill, s.Spec.Quote)
		}
		fee := m.exactAmount(fees)
		if !feesKnown {
			fee = "belum lengkap pada data eksekusi; lihat total sejak awal di bawah"
		}
		fmt.Fprintf(&out, "\n<b>Akun: %s</b>\n%s\nEksekusi beli: %d · jual: %d (termasuk isi parsial)\nHasil realisasi pada eksekusi periode: %s\nKomisi pada eksekusi periode: %s\n", esc(id), modeText(config.Mode(s.Spec.Mode)), buys, sells, m.change(realized), fee)
		out.WriteString(managedReportState(s, "pnl", "account", m, 1))
		out.WriteString(managedReportState(s, "status", "account", m, 1))
		if s.FeeIssue != "" {
			fmt.Fprintf(&out, "⚠️ %s\n", esc(s.FeeIssue))
		}
		if s.Attention != "" && s.Attention != s.FeeIssue {
			fmt.Fprintf(&out, "⚠️ Perlu diperiksa: %s\n", esc(s.Attention))
		}
		// Delayed fee valuations belong to ledger totals, not necessarily fill time.
		out.WriteString("Realisasi periode mengikuti catatan eksekusi; koreksi komisi terlambat tercakup pada total sejak awal.\n")
	}
	fmt.Fprintf(&out, "\nDetail: /pnl dan /strategies\n<i>%s</i>", m.rateLine())
	return out.String()
}
