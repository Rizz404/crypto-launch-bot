package binance

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
)

func binanceSide(s domain.Side) string { return strings.ToUpper(string(s)) }

// PlaceOrder implements broker.Broker. Fills arrive on the user stream.
func (t *Trader) PlaceOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	o.Exchange = t.c.name
	// Register before sending: the fill can arrive on the user stream
	// before the REST response.
	t.mu.Lock()
	t.open[o.ClientID] = o
	if o.Market == domain.Futures && o.Type.IsConditional() {
		t.algos[o.ClientID] = &algoState{order: o}
	}
	t.mu.Unlock()

	var err error
	switch {
	case o.Market == domain.Futures && o.Type.IsConditional():
		err = t.placeAlgo(ctx, &o)
	case o.Market == domain.Futures:
		err = t.placeFutures(ctx, &o)
	default:
		err = t.placeSpot(ctx, &o)
	}
	if err != nil {
		t.mu.Lock()
		delete(t.open, o.ClientID)
		delete(t.algos, o.ClientID)
		t.mu.Unlock()
		return o, err
	}
	t.mu.Lock()
	if _, still := t.open[o.ClientID]; still {
		t.open[o.ClientID] = o
	}
	if st, ok := t.algos[o.ClientID]; ok {
		st.order = o
	}
	t.mu.Unlock()
	return o, nil
}

// Track registers orders placed earlier (adopted during reconciliation) so
// their events are mapped like the orders placed in this session.
func (t *Trader) Track(orders []domain.Order) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, o := range orders {
		if o.Exchange != t.c.name {
			continue
		}
		t.open[o.ClientID] = o
		if o.Market == domain.Futures && o.Type.IsConditional() {
			if _, ok := t.algos[o.ClientID]; !ok {
				t.algos[o.ClientID] = &algoState{order: o}
			}
		}
	}
}

type orderResp struct {
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
}

func (t *Trader) placeSpot(ctx context.Context, o *domain.Order) error {
	q := url.Values{
		"symbol":           {o.Symbol},
		"side":             {binanceSide(o.Side)},
		"quantity":         {o.Quantity.String()},
		"newClientOrderId": {o.ClientID},
		"newOrderRespType": {"RESULT"},
	}
	switch o.Type {
	case domain.OrderMarket:
		q.Set("type", "MARKET")
	case domain.OrderLimit:
		q.Set("price", o.Price.String())
		if o.PostOnly {
			q.Set("type", "LIMIT_MAKER")
		} else {
			q.Set("type", "LIMIT")
			q.Set("timeInForce", "GTC")
		}
	case domain.OrderStopMarket:
		q.Set("type", "STOP_LOSS")
		q.Set("stopPrice", o.StopPrice.String())
	case domain.OrderTakeProfit:
		q.Set("type", "TAKE_PROFIT")
		q.Set("stopPrice", o.StopPrice.String())
	default:
		return fmt.Errorf("unsupported spot order type %s", o.Type)
	}
	var r orderResp
	if err := t.signed(ctx, domain.Spot, "POST", "/api/v3/order", q, 1, &r); err != nil {
		return err
	}
	o.ExchangeID = strconv.FormatInt(r.OrderID, 10)
	o.Status = domain.OrderNew
	return nil
}

func (t *Trader) placeFutures(ctx context.Context, o *domain.Order) error {
	q := url.Values{
		"symbol":           {o.Symbol},
		"side":             {binanceSide(o.Side)},
		"quantity":         {o.Quantity.String()},
		"newClientOrderId": {o.ClientID},
		"newOrderRespType": {"RESULT"},
	}
	if o.ReduceOnly {
		q.Set("reduceOnly", "true")
	}
	switch o.Type {
	case domain.OrderMarket:
		q.Set("type", "MARKET")
	case domain.OrderLimit:
		q.Set("type", "LIMIT")
		q.Set("price", o.Price.String())
		q.Set("timeInForce", "GTC")
		if o.PostOnly {
			q.Set("timeInForce", "GTX")
		}
	default:
		return fmt.Errorf("unsupported futures order type %s", o.Type)
	}
	var r orderResp
	if err := t.signed(ctx, domain.Futures, "POST", "/fapi/v1/order", q, 0, &r); err != nil {
		return err
	}
	o.ExchangeID = strconv.FormatInt(r.OrderID, 10)
	o.Status = domain.OrderNew
	return nil
}

// placeAlgo places a futures stop / take-profit through the algo service
// (conditional orders are no longer accepted by /fapi/v1/order).
func (t *Trader) placeAlgo(ctx context.Context, o *domain.Order) error {
	typ := "STOP_MARKET"
	if o.Type == domain.OrderTakeProfit {
		typ = "TAKE_PROFIT_MARKET"
	}
	q := url.Values{
		"algoType":     {"CONDITIONAL"},
		"symbol":       {o.Symbol},
		"side":         {binanceSide(o.Side)},
		"type":         {typ},
		"quantity":     {o.Quantity.String()},
		"triggerPrice": {o.StopPrice.String()},
		// Mark price resists wicks on thin books; price protection refuses
		// to trigger when mark and last price diverge abnormally.
		"workingType":  {"MARK_PRICE"},
		"priceProtect": {"true"},
		"clientAlgoId": {o.ClientID},
	}
	if o.ReduceOnly {
		q.Set("reduceOnly", "true")
	}
	var r struct {
		AlgoID int64 `json:"algoId"`
	}
	if err := t.signed(ctx, domain.Futures, "POST", "/fapi/v1/algoOrder", q, 0, &r); err != nil {
		return err
	}
	o.ExchangeID = "algo-" + strconv.FormatInt(r.AlgoID, 10)
	o.Status = domain.OrderNew
	return nil
}

// CancelOrder implements broker.Broker. The resulting cancel event arrives
// on the user stream.
func (t *Trader) CancelOrder(ctx context.Context, o domain.Order) error {
	q := url.Values{"symbol": {o.Symbol}}
	var err error
	switch {
	case o.Market == domain.Futures && o.Type.IsConditional():
		t.mu.Lock()
		st := t.algos[o.ClientID]
		t.mu.Unlock()
		if st != nil && st.orderID != 0 {
			// Already triggered: cancel the resulting order instead.
			q.Set("orderId", strconv.FormatInt(st.orderID, 10))
			err = t.signed(ctx, domain.Futures, "DELETE", "/fapi/v1/order", q, 1, nil)
			break
		}
		q.Set("clientAlgoId", o.ClientID)
		err = t.signed(ctx, domain.Futures, "DELETE", "/fapi/v1/algoOrder", q, 1, nil)
	case o.Market == domain.Futures:
		q.Set("origClientOrderId", o.ClientID)
		err = t.signed(ctx, domain.Futures, "DELETE", "/fapi/v1/order", q, 1, nil)
	default:
		q.Set("origClientOrderId", o.ClientID)
		err = t.signed(ctx, domain.Spot, "DELETE", "/api/v3/order", q, 1, nil)
	}
	// -2011 / -2013: unknown order (already filled, canceled, or the other
	// leg of an OCO). Reconciliation settles its final state.
	if c := apiCode(err); c == -2011 || c == -2013 {
		t.requestResync()
	}
	return err
}

// PlaceBracket implements broker.BracketPlacer: an OCO list on spot (both
// legs cannot reserve the same balance otherwise), two reduce-only
// conditional orders on futures.
func (t *Trader) PlaceBracket(ctx context.Context, stop, target domain.Order) ([]domain.Order, error) {
	hasStop, hasTarget := stop.ClientID != "", target.ClientID != ""
	if stop.Market == domain.Futures || target.Market == domain.Futures || !hasStop || !hasTarget {
		var out []domain.Order
		for _, o := range []domain.Order{stop, target} {
			if o.ClientID == "" {
				continue
			}
			placed, err := t.PlaceOrder(ctx, o)
			if err != nil {
				return out, err
			}
			out = append(out, placed)
		}
		return out, nil
	}
	if !stop.Quantity.Equal(target.Quantity) || stop.Side != target.Side {
		return nil, errors.New("oco legs must have the same side and quantity")
	}

	// For a SELL list the take-profit is above the market and the stop
	// below; a BUY list is the mirror image.
	above, below := target, stop
	aboveType, belowType := "TAKE_PROFIT", "STOP_LOSS"
	if stop.Side == domain.Buy {
		above, below = stop, target
		aboveType, belowType = "STOP_LOSS", "TAKE_PROFIT"
	}
	q := url.Values{
		"symbol":             {stop.Symbol},
		"side":               {binanceSide(stop.Side)},
		"quantity":           {stop.Quantity.String()},
		"listClientOrderId":  {stop.ClientID + "-L"},
		"aboveType":          {aboveType},
		"aboveStopPrice":     {above.StopPrice.String()},
		"aboveClientOrderId": {above.ClientID},
		"belowType":          {belowType},
		"belowStopPrice":     {below.StopPrice.String()},
		"belowClientOrderId": {below.ClientID},
		"newOrderRespType":   {"RESULT"},
	}
	var r struct {
		Orders []struct {
			OrderID       int64  `json:"orderId"`
			ClientOrderID string `json:"clientOrderId"`
		} `json:"orders"`
	}
	if err := t.signed(ctx, domain.Spot, "POST", "/api/v3/orderList/oco", q, 1, &r); err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for _, o := range r.Orders {
		ids[o.ClientOrderID] = strconv.FormatInt(o.OrderID, 10)
	}
	out := []domain.Order{stop, target}
	t.mu.Lock()
	for i := range out {
		out[i].ExchangeID, out[i].Status, out[i].Exchange = ids[out[i].ClientID], domain.OrderNew, t.c.name
		t.open[out[i].ClientID] = out[i]
	}
	t.mu.Unlock()
	return out, nil
}

// SetLeverage implements broker.LeverageSetter: isolated margin with the
// given leverage.
func (t *Trader) SetLeverage(ctx context.Context, m domain.Market, symbol string, leverage int) error {
	if m != domain.Futures {
		return nil
	}
	err := t.signed(ctx, m, "POST", "/fapi/v1/marginType", url.Values{"symbol": {symbol}, "marginType": {"ISOLATED"}}, 1, nil)
	switch apiCode(err) {
	case 0, -4046: // -4046: already isolated
	case -4048: // margin type cannot change with an open position or orders
		t.log.Warn("cannot switch to isolated margin while positions or orders are open", "symbol", symbol)
	default:
		return fmt.Errorf("set isolated margin: %w", err)
	}
	q := url.Values{"symbol": {symbol}, "leverage": {strconv.Itoa(leverage)}}
	if err := t.signed(ctx, m, "POST", "/fapi/v1/leverage", q, 1, nil); err != nil {
		return fmt.Errorf("set leverage: %w", err)
	}
	return nil
}

// EnsureOneWay switches the futures account to one-way position mode.
func (t *Trader) EnsureOneWay(ctx context.Context) error {
	err := t.signed(ctx, domain.Futures, "POST", "/fapi/v1/positionSide/dual", url.Values{"dualSidePosition": {"false"}}, 1, nil)
	if c := apiCode(err); c != 0 && c != -4059 { // -4059: no need to change
		return fmt.Errorf("set one-way mode: %w", err)
	}
	return nil
}

func msTime(ms int64) time.Time { return time.UnixMilli(ms).UTC() }
