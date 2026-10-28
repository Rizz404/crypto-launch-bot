package broker

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
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
	seq     int64 // placement order; breaks ties between equal timestamps
}

type simOrder struct {
	order     domain.Order
	placedAt  time.Time
	seq       int64
	triggered bool
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
	if o.FilledQty.IsNegative() || !o.Remaining().IsPositive() {
		return o, errors.New("sim: invalid filled quantity")
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
	// Client IDs already identify orders across engine restarts. A local
	// counter would repeat sim-1 in each paper session and violate the
	// database's exchange-order-id uniqueness constraint.
	o.ExchangeID = "sim-" + o.ClientID
	o.Exchange = s.cfg.Exchange
	o.Status = domain.OrderNew
	if o.FilledQty.IsPositive() {
		o.Status = domain.OrderPartiallyFilled
	}
	o.CreatedAt, o.UpdatedAt = now, now
	s.seq++
	s.orders[o.ClientID] = &simOrder{order: o, placedAt: now, seq: s.seq}
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
	_ = s.MatchCandle(c, nil)
}

// MatchCandle lets a durable event coordinator settle each execution before
// considering another candidate (e.g. cancel the other protective leg).
func (s *Sim) MatchCandle(c domain.Candle, settle func() error) error {
	for _, id := range s.candidates(c) {
		s.tryFill(id, c, nil)
		if settle != nil {
			if err := settle(); err != nil {
				return err
			}
		}
	}
	return nil
}

// MatchBook executes on the correct spread side, with at most 10% of each
// observed best level shared across orders. It does not simulate queue/L2.
func (s *Sim) MatchBook(q domain.BookQuote, step decimal.Decimal, settle func() error) error {
	if q.Exchange != s.cfg.Exchange || !step.IsPositive() || q.At.IsZero() || q.At.After(s.cfg.Clock()) || s.cfg.Clock().Sub(q.At) > 5*time.Second || !q.Bid.IsPositive() || !q.Ask.IsPositive() || q.Bid.GreaterThan(q.Ask) || q.BidQty.IsNegative() || q.AskQty.IsNegative() {
		return errors.New("sim: invalid or stale book quote")
	}
	bidDepth, askDepth := q.BidQty.Div(decimal.NewFromInt(10)), q.AskQty.Div(decimal.NewFromInt(10))
	bidDepth, askDepth = bidDepth.Div(step).Floor().Mul(step), askDepth.Div(step).Floor().Mul(step)
	c := domain.Candle{Exchange: q.Exchange, Market: q.Market, Symbol: q.Symbol, OpenTime: q.At}
	for _, id := range s.candidates(c) {
		s.mu.Lock()
		o := s.orders[id]
		buy := o != nil && o.order.Side == domain.Buy
		s.mu.Unlock()
		price, depth := q.Bid, &bidDepth
		if buy {
			price, depth = q.Ask, &askDepth
		}
		c.Open, c.High, c.Low, c.Close = price, price, price, price
		filled := s.tryFill(id, c, depth)
		*depth = depth.Sub(filled)
		if settle != nil {
			if err := settle(); err != nil {
				return err
			}
		}
	}
	return nil
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
		seq  int64
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
		cs = append(cs, cand{id, rank, so.placedAt, so.seq})
	}
	// Orders come from a map: without a total order, equal ranks and times
	// (fixed backtest clocks) would fill in random order between runs.
	slices.SortFunc(cs, func(a, b cand) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}
		if c := a.at.Compare(b.at); c != 0 {
			return c
		}
		return cmp.Compare(a.seq, b.seq)
	})
	ids := make([]string, len(cs))
	for i, c := range cs {
		ids[i] = c.id
	}
	return ids
}

func (s *Sim) tryFill(id string, c domain.Candle, depth *decimal.Decimal) decimal.Decimal {
	s.mu.Lock()
	so, ok := s.orders[id]
	if !ok { // canceled by the handler of an earlier fill
		s.mu.Unlock()
		return decimal.Zero
	}
	price, maker, hit := s.match(so.order, c)
	if depth != nil && so.order.Type.IsConditional() && (so.triggered || so.order.FilledQty.IsPositive()) {
		price, maker, hit = s.slip(c.Open, so.order.Side == domain.Buy), false, true
	}
	if !hit {
		s.mu.Unlock()
		return decimal.Zero
	}
	o := so.order
	if depth != nil && o.Type.IsConditional() {
		so.triggered = true
	}
	quantity := o.Remaining()
	if depth != nil {
		quantity = decimal.Min(quantity, *depth)
	}
	if !quantity.IsPositive() {
		s.mu.Unlock()
		return decimal.Zero
	}
	filled := o.FilledQty.Add(quantity)

	fee := s.cfg.Fees[o.Market]
	rate := fee.Taker
	if maker {
		rate = fee.Maker
	}
	now := s.cfg.Clock()
	fill := domain.Fill{
		OrderClientID: o.ClientID,
		TradeID:       o.ExchangeID + "-" + filled.String(),
		Exchange:      s.cfg.Exchange,
		Market:        o.Market,
		Symbol:        o.Symbol,
		Side:          o.Side,
		Price:         price,
		Quantity:      quantity,
		Fee:           price.Mul(quantity).Mul(decimal.NewFromFloat(rate)),
		FeeAsset:      "QUOTE",
		Maker:         maker,
		Time:          now,
	}
	o.AvgPrice = o.AvgPrice.Mul(o.FilledQty).Add(price.Mul(quantity)).Div(filled)
	o.Status, o.FilledQty, o.UpdatedAt = domain.OrderPartiallyFilled, filled, now
	if o.Remaining().IsZero() {
		o.Status = domain.OrderFilled
		delete(s.orders, id)
	} else {
		so.order = o
	}
	h := s.handler
	s.mu.Unlock()

	if h != nil {
		h(Event{Kind: EventFill, Order: o, Fill: fill})
	}
	return quantity
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
