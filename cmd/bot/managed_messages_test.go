package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/notify/telegram"
	"github.com/rizz404/trading-bot/internal/risk"
)

func managedFixture(t *testing.T) map[string]*managed.Runtime {
	t.Helper()
	r := risk.DefaultConfig()
	r.MaxLeverage = 1
	spec := managed.Spec{ID: "eth-research", Exchange: "sim", Mode: "paper", Quote: "USDT", Capital: decimal.RequireFromString("110.79"), Risk: r,
		Instances: []managed.InstanceSpec{{ID: "momentum", Name: "ETH momentum 4h", Allocation: decimal.RequireFromString("110.79"), Risk: r,
			Strategies: []managed.BindingSpec{{ID: "eth4h", Name: "dca", Symbols: []string{"ETHUSDT"}, Risk: r, Params: map[string]any{"quote_amount": 20, "max_buys": 2, "interval": "1h", "take_profit": 0.03}}}}}}
	repo, err := managed.NewMemory(spec)
	require.NoError(t, err)
	rt, err := managed.NewRuntime(context.Background(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), nil, decimal.Zero, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	return map[string]*managed.Runtime{"eth-research": rt}
}

func TestManagedStartAndStopMessages(t *testing.T) {
	msg := msgManagedStarted(context.Background(), managedFixture(t), testMoney, "db18c0e8c73aad752c44c2749ed92e7218a9a4a6")
	for _, want := range []string{"Bot aktif", "eth-research", "110.79 USDT", "Rp 1.994.220", "Instance seluruh akun: 1", "strategi dalam laporan: 1", "Ketik / untuk menu perintah"} {
		assert.Contains(t, msg, want)
	}
	assert.Contains(t, msgManagedStopped(config.ModePaper, nil), "tersimpan")
	assert.Contains(t, msgManagedStopped(config.ModeTestnet, errors.New("feed <lost>")), "feed &lt;lost&gt;")
}

func TestManagedTelegramCommandsHaveMenuDescriptions(t *testing.T) {
	tg := telegram.New("fake-token", []int64{42}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	registerManagedTelegram(tg, managedFixture(t), func() money { return money{} }, func() {})
	help := tg.Help()
	for _, want := range []string{"/status", "/pause", "/resolve — verifikasi order tak pasti ke exchange", "/finish — akhiri eksperimen", "/stop", "eth-research"} {
		assert.Contains(t, help, want)
	}
	var names []string
	for line := range strings.SplitSeq(help, "\n") {
		if strings.HasPrefix(line, "/") {
			names = append(names, strings.Fields(line)[0])
		}
	}
	assert.ElementsMatch(t, []string{"/help", "/start", "/status", "/balance", "/pnl", "/positions", "/orders", "/strategies", "/pause", "/resume", "/cancel", "/flatten", "/finish", "/kill", "/killswitch", "/reset", "/archive", "/resolve", "/stop"}, names)
}

func TestManagedReportsUseLedgerAndPreserveOwnership(t *testing.T) {
	runtimes := managedFixture(t)
	s, err := runtimes["eth-research"].Snapshot(context.Background())
	require.NoError(t, err)
	owner := "momentum/eth4h"
	s.Instances["momentum"].Cash = decimal.RequireFromString("77.79")
	s.Bindings[owner].Realized = decimal.NewFromInt(-3)
	s.Bindings[owner].Fees = decimal.NewFromInt(2)
	s.Bindings[owner].Spec.Params["timeframe"] = "4h"
	s.Bindings[owner].LastNotice = "wait <signal>"
	s.Holdings["eth"] = &managed.Holding{Owner: owner, Symbol: "ETHUSDT", Quantity: decimal.NewFromInt(2), Cost: decimal.NewFromInt(30)}
	s.Prices["ETHUSDT"] = decimal.NewFromInt(20)
	for _, id := range []string{"oco-stop", "oco-target"} {
		s.Orders[id] = &managed.Order{Owner: owner, State: "open", Group: "oco", QuoteReserved: decimal.NewFromInt(5), Order: domain.Order{Symbol: "ETHUSDT", Side: domain.Buy, Type: domain.OrderLimit, Quantity: decimal.NewFromInt(1)}}
	}
	s.Orders["done"] = &managed.Order{Owner: owner, State: "terminal"}
	s.Instances["other"] = &managed.Instance{ID: "other"}
	s.Bindings["other/btc"] = &managed.Binding{ID: "other/btc", InstanceID: "other", Realized: decimal.NewFromInt(999)}
	s.Holdings["btc"] = &managed.Holding{Owner: "other/btc", Symbol: "BTCUSDT", Quantity: decimal.NewFromInt(1)}
	s.Orders["foreign"] = &managed.Order{Owner: "other/btc", State: "unknown"}
	for _, scope := range []string{"momentum", owner} {
		for view, wants := range map[string][]string{
			"balance":    {"Nilai modal saat ini: 117.79 USDT", "Dana untuk order beli: 5.00 USDT", "Uang tersedia: 72.79 USDT"},
			"pnl":        {"Total bersih saat ini: <b>+7.00 USDT</b>", "-3.00 USDT", "+10.00 USDT", "Komisi tercatat: 2 USDT"},
			"positions":  {"ETHUSDT", "Jumlah: 2", "+10.00 USDT"},
			"orders":     {"oco-stop", "oco-target"},
			"strategies": {owner, "Timeframe: 4h", "wait &lt;signal&gt;"},
		} {
			t.Run(scope+"/"+view, func(t *testing.T) {
				msg := managedReportState(s, view, scope, money{}, 1)
				for _, want := range wants {
					assert.Contains(t, msg, want)
				}
				for _, excluded := range []string{"other/btc", "BTCUSDT", "foreign", "<code>done</code>", "wait <signal>"} {
					assert.NotContains(t, msg, excluded)
				}
			})
		}
	}
	s.Orders["uncertain"] = &managed.Order{Owner: owner, State: "unknown"}
	assert.Contains(t, managedReportState(s, "orders", "account", money{}, 1), "uncertain")
	s.FeeIssue = "missing <fee>"
	assert.Contains(t, managedReportState(s, "pnl", "account", money{}, 1), "missing &lt;fee&gt;")
}

func TestManagedReportArgumentsAndEmptyState(t *testing.T) {
	runtimes := managedFixture(t)
	for _, view := range []string{"balance", "pnl", "positions", "orders", "strategies"} {
		msg := managedReport(context.Background(), runtimes, view, nil, money{})
		assert.Contains(t, msg, "eth-research")
		assert.NotContains(t, msg, "tidak ditemukan")
		assert.Contains(t, managedReport(context.Background(), runtimes, view, []string{"missing"}, money{}), "Akun tidak ditemukan")
		assert.Contains(t, managedReport(context.Background(), runtimes, view, []string{"eth-research", "missing"}, money{}), "Sasaran tidak ditemukan")
		assert.Contains(t, managedReport(context.Background(), runtimes, view, []string{"a", "b", "c", "d"}, money{}), "Gunakan /"+view)
	}
	assert.Contains(t, managedReport(context.Background(), runtimes, "positions", nil, money{}), "Tidak ada posisi terbuka")
	assert.Contains(t, managedReport(context.Background(), runtimes, "orders", nil, money{}), "Tidak ada order aktif")
}

func TestManagedSummariesStayShortAndDetailRemainsAvailable(t *testing.T) {
	runtimes := managedFixture(t)
	s, err := runtimes["eth-research"].Snapshot(context.Background())
	require.NoError(t, err)
	s.Bindings = map[string]*managed.Binding{}
	for n := range 72 {
		owner := fmt.Sprintf("momentum/strategy-%02d", n)
		s.Bindings[owner] = &managed.Binding{ID: owner, InstanceID: "momentum", Status: "running", Realized: decimal.NewFromInt(1), Spec: managed.BindingSpec{Name: "trendspot", Symbols: []string{"ETHUSDT"}}}
	}
	status := managedReportState(s, "status", "account", money{}, 1)
	assert.Contains(t, status, "Proses bot: aktif")
	assert.Contains(t, status, "Izin entry menurut pengaturan: 72 dari 72")
	assert.NotContains(t, status, "strategy-00")
	pnl := managedReportState(s, "pnl", "account", testMoney, 1)
	assert.Contains(t, pnl, "+72.00 USDT")
	assert.Contains(t, pnl, "+Rp 1.296.000")
	assert.NotContains(t, pnl, "strategy-00")
	list := managedReportState(s, "strategies", "account", money{}, 1)
	assert.Contains(t, list, "Ditampilkan 5 dari 72")
	assert.NotContains(t, list, "strategy-71")
	assert.Contains(t, managedReportState(s, "strategies", "momentum/strategy-71", money{}, 1), "strategy-71")
	second := managedReportState(s, "strategies", "account", money{}, 2)
	assert.Contains(t, second, "strategy-05")
	assert.NotContains(t, second, "strategy-00")
	assert.Contains(t, managedReportState(s, "strategies", "account", money{}, 15), "strategy-71")
	for _, invalid := range []string{"0", "-1", "text", "10001"} {
		assert.Contains(t, managedReport(context.Background(), runtimes, "strategies", []string{"eth-research", "account", invalid}, money{}), "Nomor halaman")
	}
	assert.Less(t, len(status), 2000)
	assert.Less(t, len(pnl), 2000)
}

func TestManagedEntryStatusDoesNotConfusePermissionWithTrading(t *testing.T) {
	s, err := managedFixture(t)["eth-research"].Snapshot(context.Background())
	require.NoError(t, err)
	owner := "momentum/eth4h"
	for _, tc := range []struct {
		status    string
		halted    bool
		attention string
		want      string
	}{
		{"running", false, "", "Diizinkan mencari peluang beli"},
		{"paused", false, "", "Entry dijeda"},
		{"running", true, "", "batas risiko"},
		{"running", false, "balance mismatch", "perlu diperiksa"},
		{"closing", false, "", "Sedang menyelesaikan penghentian"},
		{"stopped", false, "", "Strategi dihentikan"},
		{"archived", false, "", "Sudah diarsipkan"},
	} {
		s.Instances["momentum"].Status = tc.status
		s.Halted, s.Attention = tc.halted, tc.attention
		assert.Contains(t, managedEntryStatus(s, owner), tc.want)
	}
}

func TestManagedReportsDistinguishPlansAndHideZeroDetails(t *testing.T) {
	s, err := managedFixture(t)["eth-research"].Snapshot(context.Background())
	require.NoError(t, err)
	owner := "momentum/eth4h"
	s.Bindings[owner].Spec.Name = "squeeze"
	s.Bindings[owner].Status = "paused"
	assert.Equal(t, "Belum tersedia — menunggu implementasi", managedEntryStatus(s, owner))
	status := managedReportState(s, "status", "account", money{}, 1)
	assert.Contains(t, status, "Implementasi tersedia: 0 · masih rancangan: 1")
	assert.Contains(t, status, "Izin entry menurut pengaturan: 0 dari 0 strategi tersedia")
	assert.Contains(t, managedReportState(s, "strategies", "account", money{}, 1), "Breakout setelah volatilitas menyempit")
	pnl := managedReportState(s, "pnl", "momentum", money{}, 1)
	assert.Contains(t, pnl, "1 strategi tanpa posisi, hasil, dan komisi")
	assert.NotContains(t, pnl, owner)
	assert.Contains(t, pnl, "Waktu laporan:")
	assert.Contains(t, pnl, "Mulai:")
	s.Bindings[owner].Fees = decimal.RequireFromString("0.081")
	pnl = managedReportState(s, "pnl", "momentum", testMoney, 1)
	assert.Contains(t, pnl, "Komisi tercatat: 0.081 USDT")
	assert.Contains(t, pnl, owner)
	s.Bindings[owner].Spec.Name = "trendspot"
	s.Bindings[owner].Spec.Params["mode"] = "momentum"
	assert.Equal(t, "Momentum mengikuti tren", managedStrategyName(s.Bindings[owner].Spec))
	s.Holdings["eth"] = &managed.Holding{Owner: owner, Symbol: "ETHUSDT", Quantity: decimal.NewFromInt(1), Cost: decimal.NewFromInt(100)}
	s.PriceAt["ETHUSDT"] = time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	assert.Contains(t, managedReportState(s, "positions", "account", money{}, 1), "Waktu candle harga: 21 Oct 2026 07:00:00 WIB")
}

func TestManagedControlResponsesDescribeActionAndPendingWork(t *testing.T) {
	for op, want := range map[string]string{
		"pause":   "Posisi dan order yang sudah ada tetap dipantau",
		"resume":  "Pembelian tetap menunggu sinyal",
		"cancel":  "order pelindung dipertahankan",
		"flatten": "Posisi belum dianggap tertutup",
		"kill":    "penutupan posisi diminta",
		"reset":   "Batas akun dan instance tetap berlaku",
		"archive": "sudah diarsipkan",
		"finish":  "Koin dipertahankan",
	} {
		msg := managedControlReply(managed.Command{Operation: op, Scope: "momentum/<eth>", Policy: "retain"}, "paper<account>")
		assert.Contains(t, msg, want)
		assert.Contains(t, msg, "paper&lt;account&gt;")
		assert.Contains(t, msg, "momentum/&lt;eth&gt;")
		assert.NotContains(t, msg, "<eth>")
	}
	assert.Contains(t, managedControlError(errors.New("explicit confirmation required")), "tambahkan kata confirm")
	assert.Contains(t, managedControlError(errors.New("unexpected <error>")), "unexpected &lt;error&gt;")
	assert.Contains(t, managedOrderStatus(&managed.Order{State: "canceling"}), "menunggu konfirmasi")
	assert.Contains(t, managedOrderStatus(&managed.Order{State: "unknown"}), "belum pasti")
}

// The newest closed bar must be in the warmup window, or the first live
// candle after a restart looks like a gap and stops the runtime.
func TestWarmupRequestReachesTheLatestClosedBar(t *testing.T) {
	now := time.Date(2026, 10, 20, 16, 17, 0, 908_000_000, time.UTC)
	for _, tf := range []domain.Timeframe{"1m", "4h", "1d"} {
		req := warmupRequest(exchange.KlineSub{Symbol: "ETHUSDT", Timeframe: tf}, now)
		d := tf.Duration()
		newest := req.Start.Add(time.Duration(req.Limit-1) * d)
		latestClosed := now.Truncate(d).Add(-d)
		assert.False(t, newest.Before(latestClosed), "%s window ends at %s, latest closed bar opens at %s", tf, newest, latestClosed)
		assert.Equal(t, warmupBars, int(latestClosed.Sub(req.Start)/d)+1, "%s keeps %d closed bars", tf, warmupBars)
		assert.LessOrEqual(t, req.Limit, 1000, "Binance kline limit")
	}
}
