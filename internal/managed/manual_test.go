package managed

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/broker"
	"github.com/rizz404/crypto-launch-bot/internal/domain"
	"github.com/rizz404/crypto-launch-bot/internal/engine"
	"github.com/stretchr/testify/require"
)

func TestManualOrdersRunThroughTheBindingEngine(t *testing.T) {
	sp := spec()
	sp.Instances[0].Strategies = append(sp.Instances[0].Strategies, BindingSpec{ID: "manual", Name: ManualStrategy, Symbols: []string{"ETHUSDT"}, Risk: generous()})
	repo, err := NewMemory(sp)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Minute)
	clock := func() time.Time { return now }
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: clock}), testInstruments(), d("0.001"), clock, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	bar := func() {
		now = now.Add(time.Minute)
		require.NoError(t, rt.Record(t.Context(), domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: d("100"), High: d("100"), Low: d("100"), Close: d("100"), Closed: true}, true))
	}
	bar()
	cmd := func(c Command) error { c.Confirm = true; return rt.Command(t.Context(), c) }

	require.ErrorContains(t, rt.Command(t.Context(), Command{ID: "noconfirm", Operation: "place_order", Scope: "a/manual", Symbol: "ETHUSDT", Side: "buy", OrderType: "market", Quantity: d("0.1")}), "confirmation")
	require.ErrorContains(t, cmd(Command{ID: "bot", Operation: "place_order", Scope: "a/one", Symbol: "ETHUSDT", Side: "buy", OrderType: "market", Quantity: d("0.1")}), "manual strategy")
	require.ErrorContains(t, cmd(Command{ID: "short", Operation: "place_order", Scope: "a/manual", Symbol: "ETHUSDT", Side: "sell", OrderType: "market", Quantity: d("0.1")}), "nothing to sell")
	require.ErrorContains(t, cmd(Command{ID: "badsl", Operation: "place_order", Scope: "a/manual", Symbol: "ETHUSDT", Side: "buy", OrderType: "market", Quantity: d("0.1"), StopLoss: d("120"), TakeProfit: d("110")}), "below take profit")

	require.NoError(t, cmd(Command{ID: "pause", Operation: "pause", Scope: "a/manual"}))
	require.ErrorIs(t, cmd(Command{ID: "paused-buy", Operation: "place_order", Scope: "a/manual", Symbol: "ETHUSDT", Side: "buy", OrderType: "market", Quantity: d("0.1")}), engineNoOrder, "a paused strategy takes no entries")
	require.NoError(t, cmd(Command{ID: "resume", Operation: "resume", Scope: "a/manual"}))
	buy := Command{ID: "buy-1", Operation: "place_order", Scope: "a/manual", Symbol: "ETHUSDT", Side: "buy", OrderType: "market", Quantity: d("0.5"), StopLoss: d("90"), TakeProfit: d("120")}
	require.NoError(t, cmd(buy))
	require.NoError(t, cmd(buy), "a replayed request is a no-op")
	bar()
	s, _ := repo.Load(t.Context())
	require.Equal(t, "0.5", s.holding("a/manual", "ETHUSDT").String(), "only one buy despite the replay")
	require.True(t, s.holding("a/one", "ETHUSDT").IsZero(), "the bot's inventory stays separate")
	protective := 0
	for _, o := range s.Orders {
		if o.Owner == "a/manual" && o.State != "terminal" && (o.Order.Tag == domain.TagStopLoss || o.Order.Tag == domain.TagTakeProfit) {
			protective++
			require.False(t, rt.engines["a/manual"].Cancel(t.Context(), o.Order.ClientID), "protective legs cannot be cancelled one by one")
		}
	}
	require.Positive(t, protective, "stop loss and take profit protect the manual position")

	require.ErrorContains(t, cmd(Command{ID: "oversell", Operation: "place_order", Scope: "a/manual", Symbol: "ETHUSDT", Side: "sell", OrderType: "limit", Quantity: d("0.6"), Price: d("130")}), "at most the holding")
	require.NoError(t, cmd(Command{ID: "lim-buy", Operation: "place_order", Scope: "a/manual", Symbol: "ETHUSDT", Side: "buy", OrderType: "limit", Quantity: d("0.1"), Price: d("80")}))
	s, _ = repo.Load(t.Context())
	var limit string
	for id, o := range s.Orders {
		if o.Owner == "a/manual" && o.State != "terminal" && o.Order.Type == domain.OrderLimit && o.Order.Side == domain.Buy {
			limit = id
		}
	}
	require.NotEmpty(t, limit)
	require.NoError(t, cmd(Command{ID: "cancel-1", Operation: "cancel_order", Order: limit}))
	s, _ = repo.Load(t.Context())
	require.Equal(t, "terminal", s.Orders[limit].State)

	require.NoError(t, cmd(Command{ID: "close-1", Operation: "close_position", Scope: "a/manual", Symbol: "ETHUSDT"}))
	bar()
	s, _ = repo.Load(t.Context())
	require.True(t, s.holding("a/manual", "ETHUSDT").IsZero())
	require.ErrorContains(t, cmd(Command{ID: "close-2", Operation: "close_position", Scope: "a/manual", Symbol: "ETHUSDT"}), "no position")
	require.Equal(t, "1000", s.Equity().String(), "fee-free sim at a flat price keeps equity")
}

var engineNoOrder = engine.ErrNoOrder

// A strategy added from the dashboard has no engine until the next start;
// candles, ticks and commands must skip it instead of crashing the bot.
func TestStrategyAddedAtRuntimeWaitsForRestart(t *testing.T) {
	repo, err := NewMemory(spec())
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Minute)
	clock := func() time.Time { return now }
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: clock}), testInstruments(), d("0.001"), clock, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, rt.Start(t.Context()))
	require.NoError(t, rt.Command(t.Context(), Command{ID: "pause", Operation: "pause", Scope: "a"}))
	require.NoError(t, rt.Command(t.Context(), Command{ID: "bind", Operation: "bind", Scope: "a", Binding: &BindingSpec{ID: "ema", Name: "emacross", Symbols: []string{"ETHUSDT"}, Params: map[string]any{"timeframe": "1h"}}}))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(t.Context(), domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: d("100"), High: d("100"), Low: d("100"), Close: d("100"), Closed: true}, true))
	require.NoError(t, rt.Tick(t.Context(), now))
	require.ErrorContains(t, rt.Command(t.Context(), Command{ID: "resume", Operation: "resume", Scope: "a"}), "restart runtime")
	require.NoError(t, rt.Command(t.Context(), Command{ID: "pause-again", Operation: "pause", Scope: "a"}))
}
