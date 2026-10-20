package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/config"
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
	msg := msgManagedStarted(context.Background(), config.ModePaper, managedFixture(t), "db18c0e8c73aad752c44c2749ed92e7218a9a4a6")
	for _, want := range []string{"Bot aktif", "eth-research", "110.79 USDT", "ETH momentum 4h", "eth4h · dca · ETHUSDT", "Ketik / untuk menu perintah"} {
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
}
