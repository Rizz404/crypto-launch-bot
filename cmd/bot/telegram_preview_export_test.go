package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/fx"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTelegramPreviewExport(t *testing.T) {
	path := os.Getenv("TB_PREVIEW_OUTPUT")
	if path == "" {
		t.Skip("explicit preview output required")
	}
	ctx := context.Background()
	s, err := managedFixture(t)["eth-research"].Snapshot(ctx)
	require.NoError(t, err)
	spec := s.Spec
	spec.ID = "demo"
	spec.Instances[0].Name = "Contoh strategi ETH"
	spec.Instances[0].Strategies[0].Name = "trendspot"
	spec.Instances[0].Strategies[0].Params = map[string]any{"timeframe": "4h", "mode": "momentum"}
	repo, err := managed.NewMemory(spec)
	require.NoError(t, err)
	require.NoError(t, repo.Update(ctx, "demo", "fixture", nil, func(s *managed.State) error {
		owner := "momentum/eth4h"
		s.Instances["momentum"].Cash = decimal.RequireFromString("73.79")
		s.Bindings[owner].Realized = decimal.NewFromInt(3)
		s.Bindings[owner].Fees = decimal.NewFromInt(2)
		s.Bindings[owner].Spec.Params["timeframe"] = "4h"
		s.Prices["ETHUSDT"] = decimal.NewFromInt(2200)
		s.Holdings[owner+":ETHUSDT"] = &managed.Holding{Owner: owner, Symbol: "ETHUSDT", Quantity: decimal.RequireFromString("0.02"), Cost: decimal.NewFromInt(40)}
		for _, o := range []struct {
			id      string
			typ     domain.OrderType
			trigger int64
		}{{"demo-stop", domain.OrderStopMarket, 1950}, {"demo-target", domain.OrderTakeProfit, 2400}} {
			s.Orders[o.id] = &managed.Order{Owner: owner, State: "open", Group: "demo-oco", BaseReserved: decimal.RequireFromString("0.02"), Order: domain.Order{Symbol: "ETHUSDT", Type: o.typ, Side: domain.Sell, Quantity: decimal.RequireFromString("0.02"), StopPrice: decimal.NewFromInt(o.trigger), Status: domain.OrderNew}}
		}
		return nil
	}))
	// The runtime is constructed only to read snapshots; Start/Run/Command are never called.
	rt, err := managed.NewRuntime(ctx, repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), nil, decimal.Zero, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	runtimes := map[string]*managed.Runtime{"demo": rt}
	tg := telegram.New("unused-demo-token", []int64{42}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	registerManagedTelegram(tg, runtimes, func() money { return testMoney }, func() {})
	type item struct {
		Title string
		Text  string
	}
	items := []item{}
	add := func(title, text string) { items = append(items, item{title, text}) }
	add("Bot mulai", msgManagedStarted(ctx, runtimes, testMoney, "demo-lokal"))
	add("/help dan /start (respons sama)", tg.Help())
	for _, view := range []string{"status", "balance", "pnl", "positions", "orders", "strategies"} {
		add("/"+view, managedReport(ctx, runtimes, view, nil, testMoney))
	}
	for _, view := range []string{"balance", "pnl", "strategies"} {
		add("/"+view+" demo momentum/eth4h", managedReport(ctx, runtimes, view, []string{"demo", "momentum/eth4h"}, testMoney))
	}
	for _, op := range []string{"pause", "resume", "cancel", "flatten", "kill", "reset", "archive"} {
		title := "/" + op + " demo momentum"
		if op == "kill" {
			title += " (juga /killswitch)"
		}
		add(title, managedControlReply(managed.Command{Operation: op, Scope: "momentum"}, "demo"))
	}
	for _, policy := range []string{"retain", "flatten"} {
		add("/finish demo momentum "+policy+" confirm", managedControlReply(managed.Command{Operation: "finish", Scope: "momentum", Policy: policy}, "demo"))
	}
	add("Konfirmasi diperlukan", managedControlError(errors.New("explicit confirmation required")))
	add("/resolve demo demo-order confirm", "✅ Pemeriksaan order demo-order pada akun demo selesai. Catatan bot diperbarui. Periksa /orders demo.")
	add("/stop confirm", "🛑 Permintaan mematikan proses bot diterima. Posisi dan order dipertahankan; proses menyimpan data sebelum keluar.")
	add("Sasaran tidak ditemukan", managedReport(ctx, runtimes, "pnl", []string{"demo", "tidak-ada"}, testMoney))
	add("Tidak ada posisi / order", "Tidak ada posisi terbuka.\nOrder pelindung: /orders\n\nTidak ada order aktif atau menunggu verifikasi.")
	snapshot, err := rt.Snapshot(ctx)
	require.NoError(t, err)
	for n := range 7 {
		owner := fmt.Sprintf("momentum/demo-%d", n)
		snapshot.Bindings[owner] = &managed.Binding{ID: owner, InstanceID: "momentum", Status: "paused", Spec: managed.BindingSpec{Name: "trendspot", Symbols: []string{"BTCUSDT"}}}
	}
	add("Contoh daftar halaman 1", managedReportState(snapshot, "strategies", "account", testMoney, 1))
	add("Contoh daftar halaman 2", managedReportState(snapshot, "strategies", "account", testMoney, 2))
	add("Order belum pasti / pembatalan tertunda", managedOrderStatus(&managed.Order{State: "unknown"})+"\n"+managedOrderStatus(&managed.Order{State: "canceling"}))
	add("Notifikasi posisi dibuka", msgEntry(domain.Order{Symbol: "ETHUSDT", Side: domain.Buy, FilledQty: decimal.RequireFromString("0.02"), AvgPrice: decimal.NewFromInt(2000), Reason: "ema cross up, rsi 55.1"}, "ETH", "momentum/eth4h", decimal.NewFromInt(1950), decimal.NewFromInt(2400), testMoney))
	add("Notifikasi posisi ditutup", msgTradeClosed(domain.Trade{PositionKey: domain.PositionKey{Symbol: "ETHUSDT"}, Strategy: "momentum/eth4h", Side: domain.Long, Quantity: decimal.RequireFromString("0.02"), EntryPrice: decimal.NewFromInt(2000), ExitPrice: decimal.NewFromInt(2200), GrossPnL: decimal.NewFromInt(4), Fees: decimal.RequireFromString("0.084"), OpenedAt: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), ClosedAt: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), ExitReason: domain.TagExit}, "ETH", testMoney, decimal.Zero))
	add("Notifikasi peringatan", msgNotice(slog.LevelWarn, "order blocked by risk limits", "reason", "contoh batas ukuran posisi"))
	add("Bot berhenti", msgManagedStopped(config.ModePaper, nil))
	for i, it := range items {
		require.LessOrEqual(t, utf8.RuneCountInString(it.Text)+200, 4096, "message %d %s", i, it.Title)
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0600))
	t.Logf("exported %d demo messages", len(items))
}

func TestTelegramStrategyPlanPreviewExport(t *testing.T) {
	path := os.Getenv("TB_STRATEGY_PREVIEW_OUTPUT")
	if path == "" {
		t.Skip("explicit strategy preview output required")
	}
	ctx := context.Background()
	base, err := managedFixture(t)["eth-research"].Snapshot(ctx)
	require.NoError(t, err)
	m := money{ok: true, rates: fx.Rates{USD: decimal.NewFromInt(1), IDR: decimal.NewFromInt(18052), Source: "asumsi demo, bukan kurs terkini", At: testMoney.rates.At}}
	type signal struct {
		id, name, mode, tf string
		symbols            []string
		ready              bool
	}
	universe := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "XRPUSDT", "SOLUSDT", "TRXUSDT", "ZECUSDT", "DOGEUSDT", "LINKUSDT", "ADAUSDT", "NEARUSDT"}
	btc := signal{"btc-momentum", "trendspot", "momentum", "4h", []string{"BTCUSDT"}, true}
	eth := signal{"eth-momentum", "trendspot", "momentum", "4h", []string{"ETHUSDT"}, true}
	pullback := func(symbol string) signal {
		return signal{symbol + "-pullback", "trendspot", "pullback", "4h", []string{symbol + "USDT"}, true}
	}
	squeeze := func(symbol string) signal {
		return signal{symbol + "-squeeze", "squeeze", "", "4h", []string{symbol + "USDT"}, false}
	}
	rangeTrade := func(symbol, tf string) signal {
		return signal{symbol + "-sideways", "mean-reversion-sideways", "", tf, []string{symbol + "USDT"}, false}
	}
	btcDaily := btc
	btcDaily.tf = "1d"
	groups := []struct {
		id, title string
		signals   []signal
	}{
		{"tren", "Tren", []signal{btc, eth, pullback("SOL")}},
		{"breakout", "Breakout", []signal{squeeze("BTC"), squeeze("ETH"), {"bnb-breakout", "trendspot", "breakout", "4h", []string{"BNBUSDT"}, true}}},
		{"sideways", "Sideways", []signal{rangeTrade("BTC", "4h"), rangeTrade("ETH", "4h"), rangeTrade("XRP", "4h")}},
		{"pullback", "Pullback", []signal{pullback("BTC"), pullback("ETH"), pullback("LINK")}},
		{"rotasi", "Rotasi dan tren", []signal{{"rotasi-mingguan", "rotation", "", "1d", universe, false}, btcDaily, pullback("ETH")}},
		{"campuran", "Campuran", []signal{btcDaily, squeeze("ETH"), rangeTrade("DOGE", "1h")}},
	}
	spec := managed.Spec{ID: "demo-strategi", Exchange: "sim", Mode: "paper", Quote: "USDT", Risk: base.Spec.Risk}
	for _, group := range groups {
		for _, capital := range []int64{500000, 2000000, 5000000, 10000000} {
			allocation := decimal.NewFromInt(capital).Div(decimal.NewFromInt(18052))
			i := managed.InstanceSpec{ID: fmt.Sprintf("%s-%dk", group.id, capital/1000), Name: group.title, Allocation: allocation, Risk: base.Spec.Risk}
			for _, sig := range group.signals {
				i.Strategies = append(i.Strategies, managed.BindingSpec{ID: sig.id, Name: sig.name, Symbols: sig.symbols, Params: map[string]any{"timeframe": sig.tf, "mode": sig.mode}, Risk: base.Spec.Risk})
			}
			spec.Instances = append(spec.Instances, i)
			spec.Capital = spec.Capital.Add(allocation)
		}
	}
	// Proposed strategy names are display fixtures only; no Runtime/strategy initialization.
	s, err := managed.NewState(spec)
	require.NoError(t, err)
	require.Len(t, s.Instances, 24)
	require.Len(t, s.Bindings, 72)
	s.StartedAt = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, group := range groups {
		for _, capital := range []int64{500, 2000, 5000, 10000} {
			for _, sig := range group.signals {
				b := s.Bindings[fmt.Sprintf("%s-%dk/%s", group.id, capital, sig.id)]
				if !sig.ready {
					b.Status = "paused"
					b.Reason = "Belum diimplementasikan; hanya rancangan tampilan, tidak bisa dijalankan."
				}
			}
		}
	}
	type item struct{ Title, Text string }
	items := []item{{"Rancangan strategi yang kita sepakati", "Pratinjau format saja, bukan trial berjalan atau hasil backtest.\n24 instance × 3 strategi = 72 binding. Enam susunan: tren, breakout, sideways, pullback, rotasi dan campuran.\nModal tiap susunan: Rp500 ribu, Rp2 juta, Rp5 juta dan Rp10 juta. Total Rp105 juta virtual.\nTimeframe pada portofolio contoh: 1h/4h/1d; 12h masuk matriks backtest.\nRotasi, squeeze dan mean reversion belum diimplementasikan. Nama/aturan final masih perlu dibekukan. Harga, kurs dan transaksi berikut semuanya ilustrasi."}}
	add := func(title, text string) { items = append(items, item{title, text}) }
	for _, view := range []string{"status", "balance", "pnl"} {
		add("Ringkasan /"+view, managedReportState(s, view, "account", m, 1))
	}
	for n, group := range groups {
		capital := []int64{500, 2000, 5000, 10000, 2000, 5000}[n]
		id := fmt.Sprintf("%s-%dk", group.id, capital)
		add(group.title+" · modal "+rupiah(decimal.NewFromInt(capital*1000), false), managedReportState(s, "balance", id, m, 1)+"\n"+managedReportState(s, "strategies", id, m, 1))
	}
	add("Sebelum entry — portofolio tren", "Contoh: strategi momentum ETH 4h diizinkan mencari peluang. Belum membeli sampai momentum >5%, close >EMA100, indikator siap, serta saldo/risiko lolos.\nIni penjelasan aturan referensi; indikator tidak sedang dihitung pada demo ini.")
	owner := "tren-2000k/eth-momentum"
	entry := domain.Order{Symbol: "ETHUSDT", Strategy: owner, Side: domain.Buy, FilledQty: decimal.RequireFromString("0.02"), AvgPrice: decimal.NewFromInt(2000), Reason: "Contoh momentum 14 hari >5% dan close >EMA100; bukan sinyal pasar aktual"}
	add("Ilustrasi entry ETH momentum 4h", msgEntry(entry, "ETH", owner, decimal.NewFromInt(1910), decimal.Zero, m)+"\n\nAngka contoh: ATR14=30 → stop 3 ATR=90; trailing awal 4,5%. Ukuran posisi ilustratif, bukan hasil sizing engine.")
	s.Instances["tren-2000k"].Cash = s.Instances["tren-2000k"].Cash.Sub(decimal.RequireFromString("40.04"))
	s.Holdings[owner+":ETHUSDT"] = &managed.Holding{Owner: owner, Symbol: "ETHUSDT", Quantity: entry.FilledQty, Cost: decimal.RequireFromString("40.04")}
	s.Bindings[owner].Fees = decimal.RequireFromString("0.04")
	s.Prices["ETHUSDT"] = decimal.NewFromInt(2050)
	s.PriceAt["ETHUSDT"] = time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	s.Orders["demo-stop-eth"] = &managed.Order{Owner: owner, State: "open", BaseReserved: entry.FilledQty, Order: domain.Order{Symbol: "ETHUSDT", Side: domain.Sell, Type: domain.OrderStopMarket, StopPrice: decimal.NewFromInt(1910), Quantity: entry.FilledQty, Status: domain.OrderNew}}
	for _, view := range []string{"positions", "orders", "pnl"} {
		add("Sesudah entry: /"+view+" tren-2000k", managedReportState(s, view, "tren-2000k", m, 1))
	}
	add("Ilustrasi exit ETH momentum 4h", msgTradeClosed(domain.Trade{PositionKey: domain.PositionKey{Symbol: "ETHUSDT"}, Strategy: owner, Side: domain.Long, Quantity: entry.FilledQty, EntryPrice: entry.AvgPrice, ExitPrice: decimal.NewFromInt(2050), GrossPnL: decimal.NewFromInt(1), Fees: decimal.RequireFromString("0.081"), OpenedAt: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), ClosedAt: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), ExitReason: domain.TagExit}, "ETH", m, s.Instances["tren-2000k"].Funded)+"\n\nIlustrasi exit sinyal (momentum &lt;=0 atau close &lt;EMA100), bukan hasil replay/backtest.")
	s.Instances["tren-2000k"].Cash = s.Instances["tren-2000k"].Cash.Add(decimal.RequireFromString("40.959"))
	s.Bindings[owner].Realized = decimal.RequireFromString("0.919")
	s.Bindings[owner].Fees = decimal.RequireFromString("0.081")
	delete(s.Holdings, owner+":ETHUSDT")
	s.Orders["demo-stop-eth"].State = "terminal"
	add("Setelah posisi ditutup: /pnl tren-2000k", managedReportState(s, "pnl", "tren-2000k", m, 1))
	require.True(t, s.Equity().Sub(spec.Capital).Equal(decimal.RequireFromString("0.919")))
	for n, it := range items {
		require.LessOrEqual(t, utf8.RuneCountInString(it.Text)+200, 4096, "message %d %s", n, it.Title)
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0600))
	t.Logf("exported %d strategy-plan demo messages", len(items))
}
