package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagedNotificationsRoundTripAndPreferences(t *testing.T) {
	for _, pref := range []string{"info", "warn", "error", "off", ""} {
		t.Run(pref, func(t *testing.T) {
			ctx := t.Context()
			s, err := managedFixture(t)["eth-research"].Snapshot(ctx)
			require.NoError(t, err)
			s.Spec.Instances[0].Notify = pref
			repo, err := managed.NewMemory(s.Spec)
			require.NoError(t, err)
			now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
			clock := func() time.Time { return now }
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			ins := []domain.Instrument{{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.001")}}
			rt, err := managed.NewRuntime(ctx, repo, broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: clock, Fees: map[domain.Market]broker.Fees{domain.Spot: {Taker: 0.001}}}), ins, decimal.RequireFromString("0.001"), clock, log)
			require.NoError(t, err)
			var messages []string
			configureManagedNotifications(ctx, rt, s.Spec.ID, func() money { return money{} }, func(msg string) { messages = append(messages, msg) }, log)
			require.NoError(t, rt.Start(ctx))
			bar := func(value int64) domain.Candle {
				p := decimal.NewFromInt(value)
				return domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: p, High: p, Low: p, Close: p, Closed: true}
			}
			require.NoError(t, rt.Record(ctx, bar(100), true))
			require.NoError(t, rt.Tick(ctx, now))
			now = now.Add(time.Minute)
			require.NoError(t, rt.Record(ctx, bar(100), true))
			if pref == "info" {
				require.Len(t, messages, 1)
				assert.Contains(t, messages[0], "Pembelian terisi: ETHUSDT")
				assert.Contains(t, messages[0], "eth-research / momentum/eth4h")
				assert.Contains(t, messages[0], "Komisi eksekusi: 0.02 USDT")
				assert.Contains(t, messages[0], "Paper")
			} else {
				assert.Empty(t, messages)
			}
			now = now.Add(time.Minute)
			require.NoError(t, rt.Record(ctx, bar(110), true))
			now = now.Add(time.Minute)
			require.NoError(t, rt.Record(ctx, bar(110), true))
			if pref == "info" {
				require.Len(t, messages, 2)
				assert.Contains(t, messages[1], "Penjualan terisi: ETHUSDT")
				assert.Contains(t, messages[1], "+1.96 USDT")
				assert.Contains(t, messages[1], "Sisa posisi tercatat: 0 ETH")
			} else {
				assert.Empty(t, messages)
			}
			before := len(messages)
			rt.OnNotice("momentum/eth4h", slog.LevelError, "failure <private>")
			rt.OnNotice("momentum/eth4h", slog.LevelError, "failure <private>")
			if pref == "off" {
				assert.Len(t, messages, before)
			} else {
				require.Len(t, messages, before+1)
				assert.Contains(t, messages[len(messages)-1], "failure &lt;private&gt;")
			}
		})
	}
}

func TestManagedExecutionUsesPartialFillAndLedgerPnL(t *testing.T) {
	s, err := managedFixture(t)["eth-research"].Snapshot(t.Context())
	require.NoError(t, err)
	owner := "momentum/eth4h"
	e := managed.Execution{Owner: owner, Realized: decimal.RequireFromString("-0.12345"), Fill: domain.Fill{Symbol: "ETHUSDT", Side: domain.Sell, Quantity: decimal.RequireFromString("0.1"), Price: decimal.NewFromInt(100), Fee: decimal.RequireFromString("0.01"), Time: time.Now()}}
	s.Holdings["eth"] = &managed.Holding{Owner: owner, Symbol: "ETHUSDT", Quantity: decimal.RequireFromString("0.2")}
	msg := msgManagedExecution(s, e, domain.Order{Tag: domain.TagStopLoss, FilledQty: decimal.NewFromInt(9)}, money{})
	assert.Contains(t, msg, "Jumlah: 0.1 @")
	assert.Contains(t, msg, "-0.12 USDT")
	assert.Contains(t, msg, "Sisa posisi tercatat: 0.2 ETH")
	assert.NotContains(t, msg, "Posisi ditutup")
	s.FeeIssue = "fee <unknown>"
	assert.Contains(t, msgManagedExecution(s, e, domain.Order{}, money{}), "fee &lt;unknown&gt;")
	e.Fill.FeeAsset, e.Fill.Commission = "BNB", decimal.RequireFromString("0.001")
	assert.Contains(t, msgManagedExecution(s, e, domain.Order{}, money{}), "nilai quote belum lengkap")
	e.Fill.Side = domain.Buy
	s.Orders["stop"] = &managed.Order{Owner: owner, State: "open", Order: domain.Order{Symbol: "ETHUSDT", Tag: domain.TagStopLoss, StopPrice: decimal.NewFromInt(95)}}
	s.Orders["target"] = &managed.Order{Owner: owner, State: "open", Order: domain.Order{Symbol: "ETHUSDT", Tag: domain.TagTakeProfit, StopPrice: decimal.NewFromInt(105)}}
	s.Orders["foreign"] = &managed.Order{Owner: "other", State: "open", Order: domain.Order{Symbol: "ETHUSDT", Tag: domain.TagStopLoss, StopPrice: decimal.NewFromInt(12345)}}
	msg = msgManagedExecution(s, e, domain.Order{Reason: "signal <cross>"}, money{})
	assert.Contains(t, msg, "Stop loss tercatat: 95")
	assert.Contains(t, msg, "Take profit tercatat: 105")
	assert.Contains(t, msg, "signal &lt;cross&gt;")
	assert.NotContains(t, msg, "12345")
}

func TestManagedNoticeSeverityEscalationIsNotSuppressed(t *testing.T) {
	rt := managedFixture(t)["eth-research"]
	var messages []string
	configureManagedNotifications(t.Context(), rt, "eth-research", func() money { return money{} }, func(msg string) { messages = append(messages, msg) }, slog.Default())
	rt.OnNotice("momentum/eth4h", slog.LevelInfo, "same message")
	rt.OnNotice("momentum/eth4h", slog.LevelWarn, "same message")
	rt.OnNotice("momentum/eth4h", slog.LevelError, "same message")
	rt.OnNotice("momentum/eth4h", slog.LevelError, "same message")
	assert.Len(t, messages, 2, "default warn filters info, but must allow a later error with the same text")
}

func TestManagedDailyUsesHalfOpenUTCPeriodAndShowsCompletedActivity(t *testing.T) {
	ctx := context.Background()
	s, err := managedFixture(t)["eth-research"].Snapshot(ctx)
	require.NoError(t, err)
	repo, err := managed.NewMemory(s.Spec)
	require.NoError(t, err)
	day := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Update(ctx, "fixture", "fixture", nil, func(s *managed.State) error {
		for n, at := range []time.Time{day.Add(-time.Nanosecond), day, day.Add(23 * time.Hour), day.Add(24 * time.Hour)} {
			side := domain.Buy
			if n == 2 {
				side = domain.Sell
			}
			s.Executions = append(s.Executions, managed.Execution{Owner: "momentum/eth4h", Fill: domain.Fill{Time: at, Side: side, Fee: decimal.RequireFromString("0.02")}, Realized: decimal.NewFromInt(int64(n))})
		}
		s.Bindings["momentum/eth4h"].Realized = decimal.NewFromInt(42)
		s.Attention = "review <needed>"
		return nil
	}))
	rt, err := managed.NewRuntime(ctx, repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), nil, decimal.Zero, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	msg := managedDaily(ctx, map[string]*managed.Runtime{"eth-research": rt}, day, money{})
	for _, want := range []string{"22 Oct 2026 07:00:00 WIB", "23 Oct 2026 07:00:00 WIB", "Eksekusi beli: 1 · jual: 1", "eksekusi periode: +3.00 USDT", "Komisi pada eksekusi periode: 0.04 USDT", "+42.00 USDT", "Posisi terbuka dalam laporan: 0", "review &lt;needed&gt;"} {
		assert.Contains(t, msg, want)
	}
	assert.Less(t, len([]rune(strings.ReplaceAll(msg, "&lt;", "<"))), 4000)
	require.NoError(t, repo.Update(ctx, "unknown-fee", "fixture", nil, func(s *managed.State) error {
		s.Executions[1].Fill.FeeUnknown = true
		return nil
	}))
	assert.Contains(t, managedDaily(ctx, map[string]*managed.Runtime{"eth-research": rt}, day, money{}), "Komisi pada eksekusi periode: belum lengkap")
}
