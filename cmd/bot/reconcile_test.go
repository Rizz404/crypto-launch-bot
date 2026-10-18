package main

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/engine"
	"github.com/rizz404/trading-bot/internal/metrics"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
)

// The testnet account starts with 1 BTC that the bot never bought; a spot
// wallet on mainnet may hold the user's own coins the same way.
func TestReconcileSpotManagesOnlyTheBotsHoldings(t *testing.T) {
	s := &session{log: slog.New(slog.NewTextHandler(io.Discard, nil)), mismatch: map[domain.PositionKey]int{},
		leverages: map[domain.PositionKey]int{}, metrics: metrics.NewTrading(prometheus.NewRegistry()),
		instrument: map[domain.PositionKey]domain.Instrument{}}
	key := domain.PositionKey{Exchange: "binance-testnet", Market: domain.Spot, Symbol: "BTCUSDT"}
	s.instrument[key] = domain.Instrument{Exchange: key.Exchange, Market: key.Market, Symbol: key.Symbol, Base: "BTC", Quote: "USDT",
		TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.00001"),
		MinQty: decimal.RequireFromString("0.00001"), MinNotional: decimal.NewFromInt(5)}
	a := portfolio.NewAccount(portfolio.AccountKey{Exchange: key.Exchange, Market: key.Market}, decimal.Zero, nil)
	d := decimal.RequireFromString
	state := func(btc string) *broker.AccountState {
		return &broker.AccountState{Market: domain.Spot, Balances: map[string]broker.Balance{
			"BTC":  {Asset: "BTC", Wallet: d(btc)},
			"USDT": {Asset: "USDT", Wallet: d("10000")},
		}}
	}

	s.reconcileSpot(a, state("1"), "USDT", true)
	assert.Empty(t, a.Positions(), "holdings the bot did not buy are not its position")
	assert.Equal(t, "10000", a.Snapshot().Balance.String())

	ours := domain.Position{PositionKey: key, Side: domain.Long, Quantity: d("0.00025"), EntryPrice: d("80000"),
		Leverage: 1, Strategy: "ema"}
	a.SetPosition(ours)
	s.reconcileSpot(a, state("1.00025"), "USDT", false)
	got, ok := a.Position(key.Symbol)
	require.True(t, ok)
	assert.Equal(t, "0.00025", got.Quantity.String(), "never adopts the rest of the wallet")
	assert.Equal(t, "ema", got.Strategy)
	assert.Equal(t, "10020", a.Snapshot().Balance.String(), "cash plus the cost of the bot's coins")

	// Without other coins in the wallet, a base-asset fee or a manual sale
	// leaves less than the ledger expects: corrected after two samples.
	s.reconcileSpot(a, state("0.0002"), "USDT", false)
	got, _ = a.Position(key.Symbol)
	assert.Equal(t, "0.00025", got.Quantity.String())
	s.reconcileSpot(a, state("0.0002"), "USDT", false)
	got, _ = a.Position(key.Symbol)
	assert.Equal(t, "0.0002", got.Quantity.String())

	s.reconcileSpot(a, state("0"), "USDT", false)
	s.reconcileSpot(a, state("0"), "USDT", false)
	assert.Empty(t, a.Positions(), "coins sold outside the bot close its position")
}

// fakeLive serves a fixed account state; orders go to the embedded Sim.
type fakeLive struct {
	*broker.Sim
	state  *broker.AccountState
	resync chan struct{}
}

func (f *fakeLive) PlaceBracket(context.Context, domain.Order, domain.Order) ([]domain.Order, error) {
	return nil, nil
}
func (f *fakeLive) SetLeverage(context.Context, domain.Market, string, int) error { return nil }
func (f *fakeLive) Start(context.Context, *sync.WaitGroup) error                  { return nil }
func (f *fakeLive) AccountState(context.Context, domain.Market) (*broker.AccountState, error) {
	return f.state, nil
}
func (f *fakeLive) SyncOrders(context.Context, []domain.Order, []domain.Order) {}
func (f *fakeLive) Track([]domain.Order)                                       {}
func (f *fakeLive) Resync() <-chan struct{}                                    { return f.resync }

// Live accounts start with an empty ledger until the first reconcile; the
// kill switch baseline must use the reconciled equity (found on the testnet:
// every start tripped "daily loss 100%").
func TestLiveStartDoesNotTripKillSwitch(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	key := portfolio.AccountKey{Exchange: "binance-testnet", Market: domain.Spot}
	book := portfolio.NewBook()
	book.Add(portfolio.NewAccount(key, decimal.Zero, nil))
	rm := risk.NewManager(risk.DefaultConfig(), log)
	live := &fakeLive{Sim: broker.NewSim(broker.SimConfig{Exchange: key.Exchange, Clock: time.Now}), resync: make(chan struct{}),
		state: &broker.AccountState{Market: domain.Spot, Balances: map[string]broker.Balance{"USDT": {Asset: "USDT", Wallet: decimal.NewFromInt(10000)}}}}
	s := &session{log: log, book: book, risk: rm, lives: map[string]broker.Live{key.Exchange: live},
		eng: engine.New(engine.Config{Book: book, Risk: rm, Log: log}), resync: make(chan struct{}, 1),
		mismatch: map[domain.PositionKey]int{}, leverages: map[domain.PositionKey]int{},
		quotes: map[portfolio.AccountKey]string{key: "USDT"}, instrument: map[domain.PositionKey]domain.Instrument{},
		metrics: metrics.NewTrading(prometheus.NewRegistry())}
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); s.liveWG.Wait() }()
	require.NoError(t, s.start(ctx))
	halted, reason := rm.Halted()
	assert.False(t, halted, reason)
	assert.Equal(t, "10000", book.Total().Equity.String())
}

// A restart within one bar warms up on a candle recent enough to act on.
// Acting before the first reconcile meant an empty ledger and no orders of
// the previous session: on the testnet the grid laid a second ladder, held
// back only by the exposure limit of the empty ledger.
func TestFreshWarmupCandleRunsAfterReconcile(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	key := portfolio.AccountKey{Exchange: "binance-testnet", Market: domain.Spot}
	book := portfolio.NewBook()
	book.Add(portfolio.NewAccount(key, decimal.Zero, nil))
	rm := risk.NewManager(risk.DefaultConfig(), log)
	live := &fakeLive{Sim: broker.NewSim(broker.SimConfig{Exchange: key.Exchange, Clock: time.Now}), resync: make(chan struct{}),
		state: &broker.AccountState{Market: domain.Spot, Balances: map[string]broker.Balance{"USDT": {Asset: "USDT", Wallet: decimal.NewFromInt(10000)}}}}
	eng := engine.New(engine.Config{Book: book, Risk: rm, Log: log, HoldRisk: true})
	eng.AddBroker(live)
	eng.SetInstruments([]domain.Instrument{{Exchange: key.Exchange, Market: key.Market, Symbol: "SOLUSDT", Base: "SOL", Quote: "USDT",
		TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.001"),
		MinQty: decimal.RequireFromString("0.001"), MinNotional: decimal.NewFromInt(5)}})
	require.NoError(t, eng.AddStrategy(strategy.Config{ID: "grid-sol", Name: "grid", Exchange: key.Exchange, Market: key.Market,
		Symbols: []string{"SOLUSDT"}, Leverage: 1,
		Params: map[string]any{"timeframe": "1m", "lower": 90, "upper": 110, "levels": 4, "quote_per_level": 20}}))
	s := &session{log: log, book: book, risk: rm, eng: eng, lives: map[string]broker.Live{key.Exchange: live},
		resync: make(chan struct{}, 1), mismatch: map[domain.PositionKey]int{}, leverages: map[domain.PositionKey]int{},
		quotes: map[portfolio.AccountKey]string{key: "USDT"}, instrument: map[domain.PositionKey]domain.Instrument{},
		metrics: metrics.NewTrading(prometheus.NewRegistry())}
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); s.liveWG.Wait() }()

	now := time.Now()
	candle := func(age time.Duration) domain.Candle {
		px := decimal.NewFromInt(100)
		return domain.Candle{Exchange: key.Exchange, Market: key.Market, Symbol: "SOLUSDT", Timeframe: "1m",
			OpenTime: now.Add(-age - time.Minute), Open: px, High: px, Low: px, Close: px, Volume: decimal.NewFromInt(1)}
	}
	s.replayWarmup(ctx, []domain.Candle{candle(10 * time.Minute), candle(10 * time.Second)}, now)
	assert.Empty(t, eng.OpenOrders(), "nothing is placed before the exchange state is loaded")
	assert.Len(t, s.fresh, 1)

	require.NoError(t, s.start(ctx))
	assert.Len(t, eng.OpenOrders(), 2, "the grid starts with the reconciled balance")
	assert.Empty(t, s.fresh)
}

// A stop/take-profit pair left from a position closed before adoption
// (seen on the testnet after a startup kill switch) must not be adopted: on
// spot it would sell coins the bot does not own.
func TestOrphanedProtectionIsCanceledNotAdopted(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	key := portfolio.AccountKey{Exchange: "sim", Market: domain.Spot}
	book := portfolio.NewBook()
	book.Add(portfolio.NewAccount(key, decimal.NewFromInt(1000), nil))
	sim := broker.NewSim(broker.SimConfig{Exchange: key.Exchange, Clock: time.Now})
	sim.SetHandler(func(broker.Event) {})
	live := &fakeLive{Sim: sim, resync: make(chan struct{})}
	s := &session{log: log, book: book, metrics: metrics.NewTrading(prometheus.NewRegistry())}
	ctx := context.Background()
	order := func(id, tag string, typ domain.OrderType, symbol string) domain.Order {
		o := domain.Order{ClientID: id, Exchange: key.Exchange, Market: key.Market, Symbol: symbol, Side: domain.Sell,
			Type: typ, Quantity: decimal.RequireFromString("0.001"), StopPrice: decimal.NewFromInt(90), Tag: tag}
		placed, err := sim.PlaceOrder(ctx, o)
		require.NoError(t, err)
		placed.Tag = tag
		return placed
	}
	stop := order("tbstop", domain.TagStopLoss, domain.OrderStopMarket, "BTCUSDT")
	target := order("tbtp", domain.TagTakeProfit, domain.OrderTakeProfit, "BTCUSDT")
	grid := order("tbgrid", domain.TagStrategy, domain.OrderStopMarket, "BTCUSDT")
	book.Account(key).SetPosition(domain.Position{PositionKey: domain.PositionKey{Exchange: key.Exchange, Market: key.Market, Symbol: "ETHUSDT"},
		Side: domain.Long, Quantity: decimal.NewFromInt(1), EntryPrice: decimal.NewFromInt(100), Leverage: 1})
	ethStop := order("tbethstop", domain.TagStopLoss, domain.OrderStopMarket, "ETHUSDT")

	kept := s.cancelOrphanedProtection(ctx, live, key, []domain.Order{stop, target, grid, ethStop})
	var ids []string
	for _, o := range kept {
		ids = append(ids, o.ClientID)
	}
	assert.Equal(t, []string{"tbgrid", "tbethstop"}, ids, "strategy orders and protection of open positions are adopted")
	var open []string
	for _, o := range sim.OpenOrders() {
		open = append(open, o.ClientID)
	}
	assert.ElementsMatch(t, []string{"tbgrid", "tbethstop"}, open)
}

func TestReconcileRequiresPersistentMismatchAndPreservesMetadata(t *testing.T) {
	s := &session{log: slog.New(slog.NewTextHandler(io.Discard, nil)), mismatch: map[domain.PositionKey]int{},
		leverages: map[domain.PositionKey]int{}, metrics: metrics.NewTrading(prometheus.NewRegistry())}
	key := domain.PositionKey{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"}
	a := portfolio.NewAccount(portfolio.AccountKey{Exchange: key.Exchange, Market: key.Market}, decimal.NewFromInt(1000), nil)
	p := domain.Position{PositionKey: key, Side: domain.Long, Quantity: decimal.NewFromInt(1), EntryPrice: decimal.NewFromInt(100),
		Strategy: "test", Fees: decimal.NewFromInt(2), Funding: decimal.NewFromInt(1), OpenedAt: time.Now()}
	a.SetPosition(p)
	x := p
	x.Quantity, x.Strategy, x.Fees, x.Funding = decimal.NewFromInt(2), "", decimal.Zero, decimal.Zero
	s.reconcilePositions(a, []domain.Position{x}, false)
	got, _ := a.Position(key.Symbol)
	assert.Equal(t, "1", got.Quantity.String(), "first mismatch may be an in-flight fill")
	s.reconcilePositions(a, []domain.Position{p}, false)
	s.reconcilePositions(a, []domain.Position{x}, false)
	got, _ = a.Position(key.Symbol)
	assert.Equal(t, "1", got.Quantity.String(), "matching sample resets mismatch counter")
	s.reconcilePositions(a, []domain.Position{x}, false)
	got, _ = a.Position(key.Symbol)
	assert.Equal(t, "2", got.Quantity.String())
	assert.Equal(t, "test", got.Strategy)
	assert.Equal(t, "2", got.Fees.String())
	assert.Equal(t, "1", got.Funding.String())
	assert.Equal(t, p.OpenedAt, got.OpenedAt)
	s.reconcilePositions(a, nil, false)
	_, exists := a.Position(key.Symbol)
	require.True(t, exists)
	s.reconcilePositions(a, nil, false)
	assert.Empty(t, a.Positions())
	s.reconcilePositions(a, []domain.Position{x}, true)
	_, exists = a.Position(key.Symbol)
	assert.True(t, exists, "initial reconciliation loads positions immediately")
}
