package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Side is the direction of an order.
type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

// Opposite returns the other side.
func (s Side) Opposite() Side {
	if s == Buy {
		return Sell
	}
	return Buy
}

// PositionSide is the direction of a position (one-way mode).
type PositionSide string

const (
	Long  PositionSide = "long"
	Short PositionSide = "short"
)

// EntrySide is the order side that opens a position of side p.
func (p PositionSide) EntrySide() Side {
	if p == Long {
		return Buy
	}
	return Sell
}

// ExitSide is the order side that reduces a position of side p.
func (p PositionSide) ExitSide() Side { return p.EntrySide().Opposite() }

// Sign is +1 for long and -1 for short.
func (p PositionSide) Sign() decimal.Decimal {
	if p == Long {
		return decimal.NewFromInt(1)
	}
	return decimal.NewFromInt(-1)
}

type OrderType string

const (
	OrderMarket     OrderType = "market"
	OrderLimit      OrderType = "limit"
	OrderStopMarket OrderType = "stop_market"        // triggers when price crosses StopPrice against the position
	OrderTakeProfit OrderType = "take_profit_market" // triggers when price crosses StopPrice in favour
)

// IsConditional reports whether the order waits for a trigger price.
func (t OrderType) IsConditional() bool { return t == OrderStopMarket || t == OrderTakeProfit }

type OrderStatus string

const (
	OrderNew             OrderStatus = "new"
	OrderPartiallyFilled OrderStatus = "partially_filled"
	OrderFilled          OrderStatus = "filled"
	OrderCanceled        OrderStatus = "canceled"
	OrderRejected        OrderStatus = "rejected"
	OrderExpired         OrderStatus = "expired"
)

// Open reports whether the order can still fill.
func (s OrderStatus) Open() bool { return s == OrderNew || s == OrderPartiallyFilled }

// Order tags describe why the OMS placed an order.
const (
	TagEntry      = "entry"
	TagExit       = "exit"
	TagStopLoss   = "stop_loss"
	TagTakeProfit = "take_profit"
	TagStrategy   = "strategy" // placed directly by a strategy (grid, market making)
)

type Order struct {
	ClientID   string // our idempotent id
	ExchangeID string // assigned by the exchange
	Exchange   string
	Market     Market
	Symbol     string
	Side       Side
	Type       OrderType
	Quantity   decimal.Decimal
	Price      decimal.Decimal // limit price
	StopPrice  decimal.Decimal // trigger price for conditional orders
	ReduceOnly bool
	PostOnly   bool

	Status    OrderStatus
	FilledQty decimal.Decimal
	AvgPrice  decimal.Decimal
	Tag       string
	Strategy  string // strategy instance id
	Reason    string // reject or cancel reason
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Remaining is the unfilled quantity.
func (o Order) Remaining() decimal.Decimal { return o.Quantity.Sub(o.FilledQty) }

// Fill is one execution of an order.
type Fill struct {
	OrderClientID string
	TradeID       string
	Exchange      string
	Market        Market
	Symbol        string
	Side          Side
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	Fee           decimal.Decimal // in quote currency
	FeeAsset      string
	Maker         bool
	Time          time.Time
}

// PositionKey identifies a position in one-way mode.
type PositionKey struct {
	Exchange string
	Market   Market
	Symbol   string
}

type Position struct {
	PositionKey
	Side       PositionSide
	Quantity   decimal.Decimal // always positive
	EntryPrice decimal.Decimal
	Leverage   int
	OpenedAt   time.Time
	Fees       decimal.Decimal // fees paid while the position was open
	Funding    decimal.Decimal // funding paid (+) or received (-)
	Strategy   string
}

// Notional is the position value at price.
func (p Position) Notional(price decimal.Decimal) decimal.Decimal { return p.Quantity.Mul(price) }

// UnrealizedPnL at mark price.
func (p Position) UnrealizedPnL(mark decimal.Decimal) decimal.Decimal {
	return mark.Sub(p.EntryPrice).Mul(p.Quantity).Mul(p.Side.Sign())
}

// Trade is a completed round trip, recorded when a position closes.
type Trade struct {
	PositionKey
	Strategy   string
	Side       PositionSide
	Quantity   decimal.Decimal
	EntryPrice decimal.Decimal
	ExitPrice  decimal.Decimal
	OpenedAt   time.Time
	ClosedAt   time.Time
	GrossPnL   decimal.Decimal
	Fees       decimal.Decimal
	Funding    decimal.Decimal
	ExitReason string
}

// NetPnL is the profit after fees and funding.
func (t Trade) NetPnL() decimal.Decimal { return t.GrossPnL.Sub(t.Fees).Sub(t.Funding) }
