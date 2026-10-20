package binance

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
)

// AccountState fetches balances, positions and open orders of market m.
func (t *Trader) AccountState(ctx context.Context, m domain.Market) (*broker.AccountState, error) {
	st := &broker.AccountState{Market: m, Balances: map[string]broker.Balance{}}
	if m == domain.Futures {
		return st, t.futuresState(ctx, st)
	}
	return st, t.spotState(ctx, st)
}

func (t *Trader) spotState(ctx context.Context, st *broker.AccountState) error {
	var acct struct {
		Balances []struct {
			Asset  string          `json:"asset"`
			Free   decimal.Decimal `json:"free"`
			Locked decimal.Decimal `json:"locked"`
		} `json:"balances"`
	}
	if err := t.signed(ctx, domain.Spot, "GET", "/api/v3/account", url.Values{"omitZeroBalances": {"true"}}, 20, &acct); err != nil {
		return fmt.Errorf("spot account: %w", err)
	}
	for _, b := range acct.Balances {
		st.Balances[b.Asset] = broker.Balance{Asset: b.Asset, Free: b.Free, Locked: b.Locked, Wallet: b.Free.Add(b.Locked)}
	}
	var orders []spotOrder
	if err := t.signed(ctx, domain.Spot, "GET", "/api/v3/openOrders", nil, 80, &orders); err != nil {
		return fmt.Errorf("spot open orders: %w", err)
	}
	for _, o := range orders {
		st.OpenOrders = append(st.OpenOrders, o.domain(t.c.name))
	}
	return nil
}

func (t *Trader) futuresState(ctx context.Context, st *broker.AccountState) error {
	var bals []struct {
		Asset            string          `json:"asset"`
		Balance          decimal.Decimal `json:"balance"`
		AvailableBalance decimal.Decimal `json:"availableBalance"`
	}
	if err := t.signed(ctx, domain.Futures, "GET", "/fapi/v3/balance", nil, 5, &bals); err != nil {
		return fmt.Errorf("futures balance: %w", err)
	}
	for _, b := range bals {
		st.Balances[b.Asset] = broker.Balance{Asset: b.Asset, Free: b.AvailableBalance, Wallet: b.Balance}
	}

	var risk []struct {
		Symbol      string          `json:"symbol"`
		PositionAmt decimal.Decimal `json:"positionAmt"`
		EntryPrice  decimal.Decimal `json:"entryPrice"`
		UpdateTime  int64           `json:"updateTime"`
	}
	if err := t.signed(ctx, domain.Futures, "GET", "/fapi/v3/positionRisk", nil, 5, &risk); err != nil {
		return fmt.Errorf("futures positions: %w", err)
	}
	for _, r := range risk {
		if r.PositionAmt.IsZero() {
			continue
		}
		side := domain.Long
		if r.PositionAmt.IsNegative() {
			side = domain.Short
		}
		st.Positions = append(st.Positions, domain.Position{
			PositionKey: domain.PositionKey{Exchange: t.c.name, Market: domain.Futures, Symbol: r.Symbol},
			Side:        side, Quantity: r.PositionAmt.Abs(), EntryPrice: r.EntryPrice, OpenedAt: msTime(r.UpdateTime),
		})
	}

	var orders []futuresOrder
	if err := t.signed(ctx, domain.Futures, "GET", "/fapi/v1/openOrders", nil, 40, &orders); err != nil {
		return fmt.Errorf("futures open orders: %w", err)
	}
	for _, o := range orders {
		st.OpenOrders = append(st.OpenOrders, o.domain(t.c.name))
	}
	var algos []algoOrder
	if err := t.signed(ctx, domain.Futures, "GET", "/fapi/v1/openAlgoOrders", nil, 40, &algos); err != nil {
		return fmt.Errorf("futures open algo orders: %w", err)
	}
	t.mu.Lock()
	for _, a := range algos {
		o := a.domain(t.c.name)
		st.OpenOrders = append(st.OpenOrders, o)
		if _, ok := t.algos[o.ClientID]; !ok {
			t.algos[o.ClientID] = &algoState{order: o}
		}
	}
	t.mu.Unlock()
	return nil
}

type spotOrder struct {
	Symbol        string          `json:"symbol"`
	OrderID       int64           `json:"orderId"`
	ClientOrderID string          `json:"clientOrderId"`
	Price         decimal.Decimal `json:"price"`
	OrigQty       decimal.Decimal `json:"origQty"`
	ExecutedQty   decimal.Decimal `json:"executedQty"`
	CumQuote      decimal.Decimal `json:"cummulativeQuoteQty"`
	Status        string          `json:"status"`
	Type          string          `json:"type"`
	Side          string          `json:"side"`
	StopPrice     decimal.Decimal `json:"stopPrice"`
	Time          int64           `json:"time"`
	UpdateTime    int64           `json:"updateTime"`
}

func (o spotOrder) domain(exchange string) domain.Order {
	d := domain.Order{
		ClientID: o.ClientOrderID, ExchangeID: strconv.FormatInt(o.OrderID, 10), Exchange: exchange,
		Market: domain.Spot, Symbol: o.Symbol, Side: domain.Side(strings.ToLower(o.Side)), Type: orderType(o.Type),
		Quantity: o.OrigQty, Price: o.Price, StopPrice: o.StopPrice, Status: orderStatus(o.Status),
		FilledQty: o.ExecutedQty, CreatedAt: msTime(o.Time), UpdatedAt: msTime(o.UpdateTime),
	}
	if o.ExecutedQty.IsPositive() {
		d.AvgPrice = o.CumQuote.Div(o.ExecutedQty)
	}
	return d
}

type futuresOrder struct {
	Symbol        string          `json:"symbol"`
	OrderID       int64           `json:"orderId"`
	ClientOrderID string          `json:"clientOrderId"`
	Price         decimal.Decimal `json:"price"`
	OrigQty       decimal.Decimal `json:"origQty"`
	ExecutedQty   decimal.Decimal `json:"executedQty"`
	AvgPrice      decimal.Decimal `json:"avgPrice"`
	Status        string          `json:"status"`
	Type          string          `json:"type"`
	Side          string          `json:"side"`
	StopPrice     decimal.Decimal `json:"stopPrice"`
	ReduceOnly    bool            `json:"reduceOnly"`
	Time          int64           `json:"time"`
	UpdateTime    int64           `json:"updateTime"`
}

func (o futuresOrder) domain(exchange string) domain.Order {
	return domain.Order{
		ClientID: o.ClientOrderID, ExchangeID: strconv.FormatInt(o.OrderID, 10), Exchange: exchange,
		Market: domain.Futures, Symbol: o.Symbol, Side: domain.Side(strings.ToLower(o.Side)), Type: orderType(o.Type),
		Quantity: o.OrigQty, Price: o.Price, StopPrice: o.StopPrice, ReduceOnly: o.ReduceOnly,
		Status: orderStatus(o.Status), FilledQty: o.ExecutedQty, AvgPrice: o.AvgPrice,
		CreatedAt: msTime(o.Time), UpdatedAt: msTime(o.UpdateTime),
	}
}

type algoOrder struct {
	AlgoID        int64           `json:"algoId"`
	ClientAlgoID  string          `json:"clientAlgoId"`
	OrderType     string          `json:"orderType"`
	Symbol        string          `json:"symbol"`
	Side          string          `json:"side"`
	Quantity      decimal.Decimal `json:"quantity"`
	AlgoStatus    string          `json:"algoStatus"`
	ActualOrderID string          `json:"actualOrderId"`
	TriggerPrice  decimal.Decimal `json:"triggerPrice"`
	ReduceOnly    bool            `json:"reduceOnly"`
	CreateTime    int64           `json:"createTime"`
	UpdateTime    int64           `json:"updateTime"`
}

func (a algoOrder) domain(exchange string) domain.Order {
	return domain.Order{
		ClientID: a.ClientAlgoID, ExchangeID: "algo-" + strconv.FormatInt(a.AlgoID, 10), Exchange: exchange,
		Market: domain.Futures, Symbol: a.Symbol, Side: domain.Side(strings.ToLower(a.Side)), Type: orderType(a.OrderType),
		Quantity: a.Quantity, StopPrice: a.TriggerPrice, ReduceOnly: a.ReduceOnly, Status: algoStatus(a.AlgoStatus),
		CreatedAt: msTime(a.CreateTime), UpdatedAt: msTime(a.UpdateTime),
	}
}

func orderType(s string) domain.OrderType {
	switch s {
	case "MARKET":
		return domain.OrderMarket
	case "STOP_LOSS", "STOP_MARKET", "STOP", "STOP_LOSS_LIMIT":
		return domain.OrderStopMarket
	case "TAKE_PROFIT", "TAKE_PROFIT_MARKET", "TAKE_PROFIT_LIMIT":
		return domain.OrderTakeProfit
	default: // LIMIT, LIMIT_MAKER
		return domain.OrderLimit
	}
}

func orderStatus(s string) domain.OrderStatus {
	switch s {
	case "NEW", "PENDING_NEW", "PENDING_CANCEL":
		return domain.OrderNew
	case "PARTIALLY_FILLED":
		return domain.OrderPartiallyFilled
	case "FILLED":
		return domain.OrderFilled
	case "CANCELED":
		return domain.OrderCanceled
	case "REJECTED":
		return domain.OrderRejected
	default: // EXPIRED, EXPIRED_IN_MATCH
		return domain.OrderExpired
	}
}

func algoStatus(s string) domain.OrderStatus {
	switch s {
	case "NEW", "TRIGGERING", "TRIGGERED":
		return domain.OrderNew
	case "FINISHED":
		return domain.OrderFilled
	case "CANCELED":
		return domain.OrderCanceled
	case "REJECTED":
		return domain.OrderRejected
	default:
		return domain.OrderExpired
	}
}

// SyncOrders queries orders the engine still considers open but that are
// no longer open on the exchange, and emits the fills and final states
// that were missed (e.g. while the user stream was down).
func (t *Trader) SyncOrders(ctx context.Context, tracked []domain.Order, exchangeOpen []domain.Order) {
	open := map[string]domain.Order{}
	for _, o := range exchangeOpen {
		open[o.ClientID] = o
	}
	for _, o := range tracked {
		if o.Exchange != t.c.name {
			continue
		}
		final, isOpen := open[o.ClientID]
		var err error
		if !isOpen {
			final, err = t.queryOrder(ctx, o)
		}
		if err != nil {
			t.log.Warn("order sync failed", "client_id", o.ClientID, "symbol", o.Symbol, "err", err)
			continue
		}
		if missing := final.FilledQty.Sub(o.FilledQty); missing.IsPositive() && final.AvgPrice.IsPositive() {
			if o.Market == domain.Spot {
				fills, recoveryErr := t.spotExecutions(ctx, o, final)
				if recoveryErr != nil {
					t.log.Warn("spot trade history recovery failed", "symbol", o.Symbol, "err", recoveryErr)
					continue
				}
				for _, f := range fills {
					t.emit(broker.Event{Kind: broker.EventFill, Order: final, Fill: f})
				}
			} else {
				t.emit(broker.Event{Kind: broker.EventFill, Order: final, Fill: domain.Fill{FeeUnknown: true, OrderClientID: o.ClientID, TradeID: "sync-" + final.ExchangeID + "-" + final.FilledQty.String(), Exchange: t.c.name, Market: o.Market, Symbol: o.Symbol, Side: o.Side, Price: final.AvgPrice, Quantity: missing, Time: time.Now()}})
			}
		}
		if !final.Status.Open() && final.Status != domain.OrderFilled {
			t.emit(broker.Event{Kind: broker.EventOrder, Order: final})
		}
		if isOpen {
			continue
		}
		t.mu.Lock()
		delete(t.open, o.ClientID)
		t.mu.Unlock()
	}
}

// QueryOrder implements broker.OrderQuerier. Binance -2013 ("Order does not
// exist") is reported as broker.ErrOrderNotFound.
func (t *Trader) QueryOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	final, err := t.queryOrder(ctx, o)
	if apiCode(err) == -2013 {
		return o, fmt.Errorf("%w: %w", broker.ErrOrderNotFound, err)
	}
	return final, err
}

func (t *Trader) queryOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	q := url.Values{"symbol": {o.Symbol}}
	switch {
	case o.Market == domain.Futures && o.Type.IsConditional():
		q.Set("clientAlgoId", o.ClientID)
		var a algoOrder
		if err := t.signed(ctx, domain.Futures, "GET", "/fapi/v1/algoOrder", q, 1, &a); err != nil {
			return o, err
		}
		final := a.domain(t.c.name)
		final.ClientID = o.ClientID
		if id, err := strconv.ParseInt(a.ActualOrderID, 10, 64); err == nil && id > 0 {
			// Triggered: the fill state lives on the resulting order.
			q2 := url.Values{"symbol": {o.Symbol}, "orderId": {a.ActualOrderID}}
			var fo futuresOrder
			if err := t.signed(ctx, domain.Futures, "GET", "/fapi/v1/order", q2, 1, &fo); err != nil {
				return o, err
			}
			res := fo.domain(t.c.name)
			res.ClientID, res.Type = o.ClientID, o.Type
			return res, nil
		}
		if final.Status == domain.OrderFilled { // FINISHED without a fill
			final.Status = domain.OrderExpired
		}
		return final, nil
	case o.Market == domain.Futures:
		q.Set("origClientOrderId", o.ClientID)
		var fo futuresOrder
		if err := t.signed(ctx, domain.Futures, "GET", "/fapi/v1/order", q, 1, &fo); err != nil {
			return o, err
		}
		return fo.domain(t.c.name), nil
	default:
		q.Set("origClientOrderId", o.ClientID)
		var so spotOrder
		if err := t.signed(ctx, domain.Spot, "GET", "/api/v3/order", q, 4, &so); err != nil {
			return o, err
		}
		return so.domain(t.c.name), nil
	}
}

// spotExecutions recovers actual trade IDs and commissions, including base/BNB.
// Source: https://developers.binance.com/en/docs/catalog/core-trading-spot-trading/api/rest-api/account#account-trade-list-user_data
func (t *Trader) spotExecutions(ctx context.Context, known, final domain.Order) ([]domain.Fill, error) {
	type trade struct {
		Symbol     string
		ID         int64 `json:"id"`
		OrderID    int64 `json:"orderId"`
		Price      decimal.Decimal
		Quantity   decimal.Decimal `json:"qty"`
		Commission decimal.Decimal
		Asset      string `json:"commissionAsset"`
		Time       int64
		Buyer      bool `json:"isBuyer"`
		Maker      bool `json:"isMaker"`
	}
	orderID, err := strconv.ParseInt(final.ExchangeID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("missing exchange order id")
	}
	q := url.Values{"symbol": {known.Symbol}, "orderId": {final.ExchangeID}, "fromId": {"0"}, "limit": {"1000"}}
	var all []trade
	for page := 0; page < 100; page++ {
		var rows []trade
		if err = t.signed(ctx, domain.Spot, "GET", "/api/v3/myTrades", q, 5, &rows); err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if len(rows) < 1000 {
			break
		}
		q.Set("fromId", strconv.FormatInt(rows[len(rows)-1].ID+1, 10))
		if page == 99 {
			return nil, fmt.Errorf("trade recovery exceeded page bound")
		}
	}
	var out []domain.Fill
	total := decimal.Zero
	for _, tr := range all {
		if tr.OrderID != orderID || tr.Symbol != known.Symbol || tr.Buyer != (known.Side == domain.Buy) || !tr.Quantity.IsPositive() || !tr.Price.IsPositive() {
			return nil, fmt.Errorf("invalid recovered trade identity")
		}
		next := total.Add(tr.Quantity)
		if next.GreaterThan(known.FilledQty) {
			if total.LessThan(known.FilledQty) {
				return nil, fmt.Errorf("local quantity splits a recovered trade")
			}
			f := domain.Fill{OrderClientID: known.ClientID, TradeID: strconv.FormatInt(tr.ID, 10), Exchange: t.c.name, Market: domain.Spot, Symbol: known.Symbol, Side: known.Side, Price: tr.Price, Quantity: tr.Quantity, Commission: tr.Commission, FeeAsset: tr.Asset, Fee: t.feeInQuote(known.Symbol, tr.Commission, tr.Asset, tr.Price), Maker: tr.Maker, Time: msTime(tr.Time)}
			if known.Side == domain.Buy && strings.HasPrefix(known.Symbol, tr.Asset) && tr.Asset != "" {
				f.BaseFee = tr.Commission
			}
			out = append(out, f)
		}
		total = next
	}
	if !total.Equal(final.FilledQty) {
		return nil, fmt.Errorf("trade history does not cover final order quantity")
	}
	return out, nil
}
