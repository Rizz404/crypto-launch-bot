// Package strategy defines the interface implemented by trading strategies
// and a registry to create them by name.
//
// Strategies never talk to an exchange. They react to market events and
// return Intents; the engine sizes them, checks risk and places orders.
package strategy

import (
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
)

type IntentKind int

const (
	// Enter opens a position of Side, closing an opposite one first. The
	// engine sizes it and attaches the stop loss / take profit.
	Enter IntentKind = iota + 1
	// Exit closes the position on the symbol.
	Exit
	// Place submits a raw order (grids, market making). The engine still
	// runs risk checks on it.
	Place
	// Cancel cancels an order placed with Place, identified by Ref.
	Cancel
	// CancelAll cancels every open Place order of the strategy on the symbol.
	CancelAll
)

func (k IntentKind) String() string {
	switch k {
	case Enter:
		return "enter"
	case Exit:
		return "exit"
	case Place:
		return "place"
	case Cancel:
		return "cancel"
	case CancelAll:
		return "cancel_all"
	}
	return fmt.Sprintf("intent(%d)", int(k))
}

// Intent is what a strategy wants to happen.
type Intent struct {
	Kind     IntentKind
	Exchange string // defaults to the strategy's exchange
	Market   domain.Market
	Symbol   string

	// Enter
	Side         domain.PositionSide
	StopLoss     decimal.Decimal // absolute price, zero for none
	TakeProfit   decimal.Decimal // absolute price, zero for none
	TrailingStop float64         // trail distance as a fraction of price (0.02 = 2%), 0 for none
	// Quantity overrides risk sizing when positive (risk limits still apply).
	Quantity decimal.Decimal

	// Place / Cancel
	Order OrderSpec
	Ref   string // strategy-chosen id of a Place order, unique per strategy

	Reason string
}

// OrderSpec describes a raw order for Place intents.
type OrderSpec struct {
	Side       domain.Side
	Type       domain.OrderType
	Quantity   decimal.Decimal
	Price      decimal.Decimal
	StopPrice  decimal.Decimal
	ReduceOnly bool
	PostOnly   bool
}

// Config is the configuration of one strategy instance.
type Config struct {
	ID       string // unique instance id, e.g. "emacross-btc-1h"
	Name     string // registered strategy name
	Exchange string
	Market   domain.Market
	Symbols  []string
	Leverage int
	Params   map[string]any
}

// Context gives a strategy read access to the engine state.
type Context interface {
	Now() time.Time
	Log() *slog.Logger
	Position(exchange string, m domain.Market, symbol string) (domain.Position, bool)
	Account(exchange string, m domain.Market) portfolio.Snapshot
	Instrument(exchange string, m domain.Market, symbol string) (domain.Instrument, bool)
	// OpenOrders returns the strategy's open Place orders on symbol.
	OpenOrders(symbol string) []domain.Order
}

// Strategy is implemented by every strategy.
type Strategy interface {
	// Init is called once before any event.
	Init(ctx Context, cfg Config) error
	// Timeframes lists the candle timeframes the strategy consumes.
	Timeframes() []domain.Timeframe
	// OnCandle receives every closed candle of the configured symbols and
	// timeframes, in time order.
	OnCandle(c domain.Candle) []Intent
}

// FillHandler is implemented by strategies that react to their own fills.
type FillHandler interface {
	OnFill(f domain.Fill, o domain.Order) []Intent
}

// PriceHandler is implemented by strategies that need intra-candle prices
// (market making). Updates carry the still-forming candle.
type PriceHandler interface {
	OnPrice(c domain.Candle) []Intent
}

// TimerHandler is implemented by strategies that poll on a schedule
// (funding-rate arbitrage).
type TimerHandler interface {
	Interval() time.Duration
	OnTimer(now time.Time) []Intent
}

var (
	mu       sync.RWMutex
	registry = map[string]func() Strategy{}
)

// Register makes a strategy available by name. It panics on duplicates and
// is meant to be called from init functions.
func Register(name string, factory func() Strategy) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[name]; dup {
		panic("strategy: duplicate registration of " + name)
	}
	registry[name] = factory
}

// New creates a registered strategy.
func New(name string) (Strategy, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown strategy %q (registered: %v)", name, namesLocked())
	}
	return f(), nil
}

// Names lists registered strategies.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return namesLocked()
}

func namesLocked() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// DecodeParams decodes params into dst (a pointer to a struct with
// `mapstructure` tags), rejecting unknown keys. dst should hold defaults.
func DecodeParams(params map[string]any, dst any) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           dst,
		WeaklyTypedInput: true,
		ErrorUnused:      true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			decimalHook,
		),
	})
	if err != nil {
		return err
	}
	if err := dec.Decode(params); err != nil {
		return fmt.Errorf("strategy params: %w", err)
	}
	return nil
}

func decimalHook(_, to reflect.Type, v any) (any, error) {
	if to != reflect.TypeFor[decimal.Decimal]() {
		return v, nil
	}
	switch x := v.(type) {
	case string:
		return decimal.NewFromString(x)
	case float64:
		return decimal.NewFromFloat(x), nil
	case int:
		return decimal.NewFromInt(int64(x)), nil
	}
	return v, nil
}

// HasSymbol reports whether cfg trades symbol.
func (c Config) HasSymbol(symbol string) bool { return slices.Contains(c.Symbols, symbol) }
