package main

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
)

func TestEntryMessageShowsRiskAndTargetInMoney(t *testing.T) {
	d := decimal.RequireFromString
	o := domain.Order{Symbol: "BTCUSDT", Side: domain.Buy, FilledQty: d("0.003"), AvgPrice: d("80000"), Reason: "ema cross up, rsi 55.1"}
	msg := msgEntry(o, "BTC", "ema-btc", d("79200"), d("81600"))
	assert.Contains(t, msg, "Posisi dibuka: BELI BTCUSDT")
	assert.Contains(t, msg, "0.003 BTC (≈ 240.00 USDT)")
	assert.Contains(t, msg, "Stop loss: 79200.00 (-1.00%) → rugi maks ≈ -2.40 USDT")
	assert.Contains(t, msg, "Take profit: 81600.00 (+2.00%) → untung ≈ +4.80 USDT")
	assert.Contains(t, msg, "Sinyal tren naik")

	short := msgEntry(domain.Order{Symbol: "ETHUSDT", Side: domain.Sell, FilledQty: d("1"), AvgPrice: d("2000")}, "ETH", "s", d("2100"), decimal.Zero)
	assert.Contains(t, short, "rugi maks ≈ -100.00 USDT", "a short loses when the price rises to the stop")
	assert.NotContains(t, short, "Take profit")
}

func TestTradeClosedMessageExplainsTheResult(t *testing.T) {
	d := decimal.RequireFromString
	open := time.Date(2026, 10, 18, 15, 54, 0, 0, time.UTC)
	tr := domain.Trade{PositionKey: domain.PositionKey{Symbol: "ETHUSDT"}, Strategy: "ema-eth", Side: domain.Long,
		Quantity: d("0.1"), EntryPrice: d("2400"), ExitPrice: d("2376"), OpenedAt: open, ClosedAt: open.Add(7 * time.Minute),
		GrossPnL: d("-2.4"), Fees: d("0.48"), ExitReason: domain.TagStopLoss}
	msg := msgTradeClosed(tr, "ETH")
	assert.Contains(t, msg, "RUGI -2.88 USDT")
	assert.Contains(t, msg, "(-1.20%)")
	assert.Contains(t, msg, "lama 7 menit")
	assert.Contains(t, msg, "Kena stop loss")
	tr.GrossPnL, tr.ExitReason = d("5"), domain.TagTakeProfit
	assert.Contains(t, msgTradeClosed(tr, "ETH"), "UNTUNG +4.52 USDT")
}

func TestMessagesTranslateAndEscape(t *testing.T) {
	assert.Equal(t, "Pembelian rutin ke-3 dari maksimal 10", reasonText("dca buy 3/10"))
	assert.Equal(t, "Order di tingkat grid ke-5", reasonText("grid level 5"))
	assert.Equal(t, "&lt;b&gt;", reasonText("<b>"), "unknown reasons are escaped")
	n := msgNotice(slog.LevelError, "KILL SWITCH: trading halted", "reason", "daily loss <5%")
	assert.Contains(t, n, "🚨 <b>Penting</b>")
	assert.Contains(t, n, "batas rugi tercapai")
	assert.Contains(t, n, "daily loss &lt;5%")

	started := msgStarted(config.ModeTestnet, []strategy.Config{{ID: "grid-sol", Name: "grid", Symbols: []string{"SOLUSDT"}}}, "eb4c11ba8a836f22006f52e9ea8c2fb5130bd14b")
	assert.Contains(t, started, "bukan uang asli")
	assert.Contains(t, started, "grid-sol")
	assert.Contains(t, started, "versi eb4c11b")
	assert.Contains(t, msgStarted(config.ModeLive, nil, "dev"), "LIVE — uang asli")
	assert.Contains(t, msgStopped(config.ModeTestnet, errors.New("boom")), "boom")

	daily := msgDaily(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC),
		storage.TradeStats{Trades: 4, Wins: 1, NetPnL: decimal.RequireFromString("-1.5"), Fees: decimal.RequireFromString("1"),
			Best: decimal.RequireFromString("0.8"), Worst: decimal.RequireFromString("-1.2")},
		map[string]storage.TradeStats{"grid-sol": {Trades: 4, Wins: 1, NetPnL: decimal.RequireFromString("-1.5")}}, []string{"grid-sol"},
		decimal.RequireFromString("998.5"), 1, config.ModeTestnet)
	assert.Contains(t, daily, "18 Oct 2026")
	assert.Contains(t, daily, "win rate 25%")
	assert.Contains(t, daily, "grid-sol: 4 trade, -1.50 USDT")
	assert.False(t, strings.Contains(daily, "<script"), "plain HTML only")
}
