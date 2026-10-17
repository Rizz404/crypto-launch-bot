// Package broker defines how the engine sends orders and receives fills,
// and implements the simulated broker used for backtests and paper trading.
// Exchange adapters implement Broker for live trading.
package broker

import (
	"context"

	"github.com/rizz404/trading-bot/internal/domain"
)

// EventKind tells what an Event carries.
type EventKind int

const (
	// EventFill reports an execution; Order holds the order after the fill.
	EventFill EventKind = iota + 1
	// EventOrder reports a status change without a fill (canceled,
	// rejected, expired, or accepted by the exchange).
	EventOrder
)

type Event struct {
	Kind  EventKind
	Order domain.Order
	Fill  domain.Fill
}

// Handler receives broker events. Brokers must not hold internal locks
// while calling it: the handler may place or cancel orders.
type Handler func(Event)

// Broker places and cancels orders on one exchange.
type Broker interface {
	// Exchange is the exchange name used in orders and positions.
	Exchange() string
	// SetHandler installs the event handler. It must be called before use.
	SetHandler(h Handler)
	// PlaceOrder submits o (with ClientID set) and returns it as accepted.
	// Fills are reported through the handler, possibly before PlaceOrder
	// returns.
	PlaceOrder(ctx context.Context, o domain.Order) (domain.Order, error)
	// CancelOrder cancels an open order identified by its ClientID.
	CancelOrder(ctx context.Context, o domain.Order) error
}

// LeverageSetter is implemented by brokers that support per-symbol
// leverage and margin mode (futures).
type LeverageSetter interface {
	SetLeverage(ctx context.Context, m domain.Market, symbol string, leverage int) error
}

// BracketPlacer is implemented by brokers that place a stop loss and take
// profit as one linked order (e.g. an OCO list on Binance spot, where two
// separate sell orders cannot both reserve the same balance). Either leg
// may be absent (zero Quantity).
type BracketPlacer interface {
	PlaceBracket(ctx context.Context, stop, target domain.Order) ([]domain.Order, error)
}

// LinkedBrackets is implemented by brokers whose two-legged brackets on
// market m are linked at the exchange: when one leg executes, the other
// expires (a Binance spot OCO reports the expiry before the fill).
type LinkedBrackets interface {
	LinkedBracket(m domain.Market) bool
}
