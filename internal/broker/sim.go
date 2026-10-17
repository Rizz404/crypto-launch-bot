package broker

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// Fees are commission rates as fractions of notional (0.0005 = 0.05%).
type Fees struct {
	Maker float64 `koanf:"maker"`
	Taker float64 `koanf:"taker"`
}

// SimConfig configures a simulated broker.
type SimConfig struct {
	Exchange    string // name used in orders, fills and positions
	Fees        map[domain.Market]Fees
	SlippageBps float64 // applied against us on market and stop fills
	// Clock returns the current time: candle close time in backtests,
	// wall-clock time in paper trading.
	Clock func() time.Time
}

// Sim fills orders against candles. Backtests feed it historical candles;
// paper trading feeds it live price updates.
//
// Fill rules (conservative):
//   - market orders fill at the open of the first candle that opens at or
//     after the order time, plus slippage;
//   - limit orders fill at their price (or a better open) when the range
//     touches it, as maker;
//   - stop / take-profit orders trigger when the range crosses the stop
//     price and fill at the stop or a worse open (gaps), plus slippage;
//   - within one candle, stops are processed before take-profits, so a
//     candle that touches both counts as a loss.
type Sim struct {
	cfg SimConfig

	mu      sync.Mutex
	handler Handler
	orders  map[string]*simOrder
	seq     int
}

type simOrder struct {
	order    domain.Order
	placedAt time.Time
}

func NewSim(cfg SimConfig) *Sim {
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Sim{cfg: cfg, orders: map[string]*simOrder{}}
}

func (s *Sim) Exchange() string { return s.cfg.Exchange }

func (s *Sim) SetHandler(h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = h
}

func (s *Sim) PlaceOrder(_ context.Context, o domain.Order) (domain.Order, error) {
	if o.ClientID == "" {
		return o, errors.New("sim: order without client id")
	}
	if !o.Quantity.IsPositive() {
		return o, fmt.Errorf("sim: quantity must be positive, got %s", o.Quantity)
	}
	if o.Type == domain.OrderLimit && !o.Price.IsPositive() {
		return o, errors.New("sim: limit order without price")
	}
	if o.Type.IsConditional() && !o.StopPrice.IsPositive() {
		return o, errors.New("sim: conditional order without stop price")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.orders[o.ClientID]; dup {
		return o, fmt.Errorf("sim: duplicate client id %s", o.ClientID)
	}
	now := s.cfg.Clock()
	s.seq++
	o.ExchangeID = "sim-" + strconv.Itoa(s.seq)
	o.Exchange = s.cfg.Exchange
	o.Status = domain.OrderNew
	o.CreatedAt, o.UpdatedAt = now, now
	s.orders[o.ClientID] = &simOrder{order: o, placedAt: now}
	return o, nil
}

func (s *Sim) CancelOrder(_ context.Context, o domain.Order) error {
	s.mu.Lock()
	so, ok := s.orders[o.ClientID]
	if !ok || !so.order.Status.Open() {
		s.mu.Unlock()
		return fmt.Errorf("sim: order %s is not open", o.ClientID)
	}
	delete(s.orders, o.ClientID)
	so.order.Status = domain.OrderCanceled
	so.order.UpdatedAt = s.cfg.Clock()
	h := s.handler
	s.mu.Unlock()

	if h != nil {
		h(Event{Kind: EventOrder, Order: so.order})
	}
	return nil
}

// OpenOrders returns the open orders, oldest first.
func (s *Sim) OpenOrders() []domain.Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Order, 0, len(s.orders))
	for _, so := range s.orders {
		out = append(out, so.order)
	}
	slices.SortFunc(out, func(a, b domain.Order) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out
}

// OnCandle matches open orders of c's symbol against the candle range.
// Only orders placed at or before c.OpenTime are eligible.
func (s *Sim) OnCandle(c domain.Candle) {
	for _, id := range s.candidates(c) {
		s.tryFill(id, c)
	}
}

// candidates returns eligible order ids in processing order: market,
// stops, limits, take-profits.
func (s *Sim) candidates(c domain.Candle) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	type cand struct {
		id   string
		rank int
		at   time.Time
	}
	var cs []cand
	for id, so := range s.orders {
		o := so.order
		if o.Symbol != c.Symbol || o.Market != c.Market || so.placedAt.After(c.OpenTime) {
			continue
		}
		rank := map[domain.OrderType]int{
			domain.OrderMarket: 0, domain.OrderStopMarket: 1, domain.OrderLimit: 2, domain.OrderTakeProfit: 3,
		}[o.Type]
		cs = append(cs, cand{id, rank, so.placedAt})
	}
	slices.SortFunc(cs, func(a, b cand) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}
		return a.at.Compare(b.at)
	})
	ids := make([]string, len(cs))
	for i, c := range cs {
		ids[i] = c.id
	}
	return ids
}

func (s *Sim) tryFill(id string, c domain.Candle) {
	s.mu.Lock()
	so, ok := s.orders[id]
	if !ok { // canceled by the handler of an earlier fill
		s.mu.Unlock()
		return
	}
	price, maker, hit := s.match(so.order, c)
	if !hit {
		s.mu.Unlock()
		return
	}
	o := so.order
	delete(s.orders, id)

	fee := s.cfg.Fees[o.Market]
	rate := fee.Taker
	if maker {
		rate = fee.Maker
	}
	now := s.cfg.Clock()
	fill := domain.Fill{
		OrderClientID: o.ClientID,
		TradeID:       o.ExchangeID + "-1",
		Exchange:      s.cfg.Exchange,
		Market:        o.Market,
		Symbol:        o.Symbol,
		Side:          o.Side,
		Price:         price,
		Quantity:      o.Quantity,
		Fee:           price.Mul(o.Quantity).Mul(decimal.NewFromFloat(rate)),
		FeeAsset:      "QUOTE",
		Maker:         maker,
		Time:          now,
	}
	o.Status, o.FilledQty, o.AvgPrice, o.UpdatedAt = domain.OrderFilled, o.Quantity, price, now
	h := s.handler
	s.mu.Unlock()

	if h != nil {
		h(Event{Kind: EventFill, Order: o, Fill: fill})
	}
}

// match decides whether o fills on c and at what price.
func (s *Sim) match(o domain.Order, c domain.Candle) (price decimal.Decimal, maker, hit bool) {
	buy := o.Side == domain.Buy
	switch o.Type {
	case domain.OrderMarket:
		return s.slip(c.Open, buy), false, true

	case domain.OrderLimit:
		if buy && c.Low.LessThanOrEqual(o.Price) {
			return decimal.Min(o.Price, c.Open), true, true
		}
		if !buy && c.High.GreaterThanOrEqual(o.Price) {
			return decimal.Max(o.Price, c.Open), true, true
		}

	case domain.OrderStopMarket:
		// Buy stops protect shorts (trigger on rise), sell stops protect longs.
		if buy && c.High.GreaterThanOrEqual(o.StopPrice) {
			return s.slip(decimal.Max(o.StopPrice, c.Open), true), false, true
		}
		if !buy && c.Low.LessThanOrEqual(o.StopPrice) {
			return s.slip(decimal.Min(o.StopPrice, c.Open), false), false, true
		}

	case domain.OrderTakeProfit:
		// Sell take-profits close longs on a rise, buy ones close shorts on a drop.
		if !buy && c.High.GreaterThanOrEqual(o.StopPrice) {
			return s.slip(decimal.Max(o.StopPrice, c.Open), false), false, true
		}
		if buy && c.Low.LessThanOrEqual(o.StopPrice) {
			return s.slip(decimal.Min(o.StopPrice, c.Open), true), false, true
		}
	}
	return decimal.Zero, false, false
}

// slip moves price against the taker.
func (s *Sim) slip(p decimal.Decimal, buy bool) decimal.Decimal {
	f := decimal.NewFromFloat(s.cfg.SlippageBps / 10_000)
	if buy {
		return p.Add(p.Mul(f))
	}
	return p.Sub(p.Mul(f))
}
