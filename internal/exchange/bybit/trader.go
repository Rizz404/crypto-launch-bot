package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

// Trader trades on a Bybit unified trading account.
type Trader struct {
	c       *Client
	markets []domain.Market

	mu      sync.Mutex
	handler broker.Handler
	open    map[string]domain.Order // by orderLinkId
	resync  chan struct{}
}

var (
	_ broker.Live          = (*Trader)(nil)
	_ broker.UnifiedWallet = (*Trader)(nil)
)

func NewTrader(c *Client, apiKey, apiSecret string, markets []domain.Market) *Trader {
	c.key, c.secret = apiKey, []byte(apiSecret)
	return &Trader{c: c, markets: markets, open: map[string]domain.Order{}, resync: make(chan struct{}, 1)}
}

func (t *Trader) Exchange() string        { return t.c.name }
func (t *Trader) UnifiedWallet() bool     { return true }
func (t *Trader) Resync() <-chan struct{} { return t.resync }

func (t *Trader) SetHandler(h broker.Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = h
}

func (t *Trader) emit(ev broker.Event) {
	t.mu.Lock()
	h := t.handler
	t.mu.Unlock()
	if h != nil {
		h(ev)
	}
}

func (t *Trader) requestResync() {
	select {
	case t.resync <- struct{}{}:
	default:
	}
}

func (t *Trader) signed(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	return t.c.call(ctx, method, path, q, body, true, out)
}

func bybitSide(s domain.Side) string {
	if s == domain.Buy {
		return "Buy"
	}
	return "Sell"
}

// PlaceOrder implements broker.Broker.
func (t *Trader) PlaceOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	o.Exchange = t.c.name
	t.mu.Lock()
	t.open[o.ClientID] = o
	t.mu.Unlock()

	body := map[string]any{
		"category":    category(o.Market),
		"symbol":      o.Symbol,
		"side":        bybitSide(o.Side),
		"qty":         o.Quantity.String(),
		"orderLinkId": o.ClientID,
	}
	if o.Market == domain.Futures {
		body["positionIdx"] = 0
		if o.ReduceOnly {
			body["reduceOnly"] = true
		}
	}
	switch o.Type {
	case domain.OrderMarket:
		body["orderType"] = "Market"
		if o.Market == domain.Spot {
			body["marketUnit"] = "baseCoin" // market buys default to quote units
		}
	case domain.OrderLimit:
		body["orderType"], body["price"] = "Limit", o.Price.String()
		if o.PostOnly {
			body["timeInForce"] = "PostOnly"
		}
	case domain.OrderStopMarket, domain.OrderTakeProfit:
		body["orderType"], body["triggerPrice"] = "Market", o.StopPrice.String()
		if o.Market == domain.Spot {
			// Spot conditional order: assets are not locked before it
			// triggers, so a stop and a take-profit can coexist.
			body["orderFilter"] = "StopOrder"
		} else {
			body["triggerBy"] = "MarkPrice"
			body["triggerDirection"] = triggerDirection(o)
		}
	default:
		return o, fmt.Errorf("unsupported order type %s", o.Type)
	}

	var r struct {
		OrderID string `json:"orderId"`
	}
	if err := t.signed(ctx, http.MethodPost, "/v5/order/create", nil, body, &r); err != nil {
		t.mu.Lock()
		delete(t.open, o.ClientID)
		t.mu.Unlock()
		return o, err
	}
	o.ExchangeID, o.Status = r.OrderID, domain.OrderNew
	t.mu.Lock()
	if _, still := t.open[o.ClientID]; still {
		t.open[o.ClientID] = o
	}
	t.mu.Unlock()
	return o, nil
}

// triggerDirection: 1 = triggers when the price rises to the trigger, 2 = falls.
func triggerDirection(o domain.Order) int {
	rises := (o.Type == domain.OrderStopMarket) == (o.Side == domain.Buy)
	if rises {
		return 1
	}
	return 2
}

func (t *Trader) CancelOrder(ctx context.Context, o domain.Order) error {
	body := map[string]any{"category": category(o.Market), "symbol": o.Symbol, "orderLinkId": o.ClientID}
	if o.Market == domain.Spot && o.Type.IsConditional() {
		body["orderFilter"] = "StopOrder"
	}
	err := t.signed(ctx, http.MethodPost, "/v5/order/cancel", nil, body, nil)
	if c := apiCode(err); c == 110001 || c == 170213 { // order does not exist
		t.requestResync()
	}
	return err
}

// PlaceBracket places the stop and take-profit as two conditional orders.
func (t *Trader) PlaceBracket(ctx context.Context, stop, target domain.Order) ([]domain.Order, error) {
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

// SetLeverage sets the symbol leverage (margin mode is account-wide on a
// unified account and set in Start).
func (t *Trader) SetLeverage(ctx context.Context, m domain.Market, symbol string, leverage int) error {
	if m != domain.Futures {
		return nil
	}
	l := strconv.Itoa(leverage)
	err := t.signed(ctx, http.MethodPost, "/v5/position/set-leverage", nil,
		map[string]any{"category": "linear", "symbol": symbol, "buyLeverage": l, "sellLeverage": l}, nil)
	if err != nil && apiCode(err) != 110043 { // 110043: leverage not modified
		return fmt.Errorf("set leverage: %w", err)
	}
	return nil
}

// Start prepares the account and runs the private stream.
func (t *Trader) Start(ctx context.Context, wg *sync.WaitGroup) error {
	if t.c.key == "" || len(t.c.secret) == 0 {
		return fmt.Errorf("%s: API key and secret are required", t.c.name)
	}
	if err := t.c.SyncTime(ctx); err != nil {
		return err
	}
	if slicesContains(t.markets, domain.Futures) {
		err := t.signed(ctx, http.MethodPost, "/v5/account/set-margin-mode", nil, map[string]any{"setMarginMode": "ISOLATED_MARGIN"}, nil)
		if err != nil {
			t.c.log.Warn("could not switch the unified account to isolated margin", "err", err)
		}
		err = t.signed(ctx, http.MethodPost, "/v5/position/switch-mode", nil, map[string]any{"category": "linear", "coin": "USDT", "mode": 0}, nil)
		if err != nil && apiCode(err) != 110025 { // 110025: already one-way
			return fmt.Errorf("set one-way mode: %w", err)
		}
	}
	ws := &exchange.WSStream{
		URL: t.c.ep.PrivateWS, HTTPClient: t.c.http, Log: t.c.log.With("stream", "private"),
		IdleTimeout: 6 * time.Hour, Ping: ping, PingInterval: pingInterval, Setup: t.authenticate,
	}
	wg.Go(func() { _ = ws.Run(ctx, t.requestResync, t.handlePrivate) })
	return nil
}

func slicesContains(ms []domain.Market, m domain.Market) bool {
	for _, x := range ms {
		if x == m {
			return true
		}
	}
	return false
}

func (t *Trader) authenticate(ctx context.Context, conn *websocket.Conn) error {
	expires := strconv.FormatInt(time.Now().UnixMilli()+t.c.timeOffset.Load()+10_000, 10)
	m := hmac.New(sha256.New, t.c.secret)
	_, _ = m.Write([]byte("GET/realtime" + expires))
	if err := wsjson.Write(ctx, conn, map[string]any{"op": "auth", "args": []any{t.c.key, json.Number(expires), hex.EncodeToString(m.Sum(nil))}}); err != nil {
		return err
	}
	return wsjson.Write(ctx, conn, map[string]any{"op": "subscribe", "args": []string{"order", "execution"}})
}

type wsOrder struct {
	Category     string          `json:"category"`
	OrderID      string          `json:"orderId"`
	OrderLinkID  string          `json:"orderLinkId"`
	Symbol       string          `json:"symbol"`
	Side         string          `json:"side"`
	OrderType    string          `json:"orderType"`
	OrderStatus  string          `json:"orderStatus"`
	Qty          decimal.Decimal `json:"qty"`
	Price        decimal.Decimal `json:"price"`
	TriggerPrice decimal.Decimal `json:"triggerPrice"`
	CumExecQty   decimal.Decimal `json:"cumExecQty"`
	AvgPrice     decimal.Decimal `json:"avgPrice"`
	RejectReason string          `json:"rejectReason"`
	ReduceOnly   bool            `json:"reduceOnly"`
	StopType     string          `json:"stopOrderType"`
	CreatedTime  string          `json:"createdTime"`
	UpdatedTime  string          `json:"updatedTime"`
}

type wsExecution struct {
	Category    string          `json:"category"`
	Symbol      string          `json:"symbol"`
	OrderID     string          `json:"orderId"`
	OrderLinkID string          `json:"orderLinkId"`
	Side        string          `json:"side"`
	ExecID      string          `json:"execId"`
	ExecPrice   decimal.Decimal `json:"execPrice"`
	ExecQty     decimal.Decimal `json:"execQty"`
	ExecFee     decimal.Decimal `json:"execFee"`
	ExecTime    string          `json:"execTime"`
	ExecType    string          `json:"execType"`
	IsMaker     bool            `json:"isMaker"`
}

func (t *Trader) handlePrivate(b []byte) {
	var msg struct {
		Op      string          `json:"op"`
		Success *bool           `json:"success"`
		RetMsg  string          `json:"ret_msg"`
		Topic   string          `json:"topic"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		return
	}
	switch {
	case msg.Op == "auth" || msg.Op == "subscribe":
		if msg.Success != nil && !*msg.Success {
			t.c.log.Error("private stream "+msg.Op+" failed", "msg", msg.RetMsg)
		}
	case msg.Topic == "execution":
		var execs []wsExecution
		if err := json.Unmarshal(msg.Data, &execs); err == nil {
			for _, e := range execs {
				t.onExecution(e)
			}
		}
	case msg.Topic == "order":
		var orders []wsOrder
		if err := json.Unmarshal(msg.Data, &orders); err == nil {
			for _, o := range orders {
				t.onOrder(o)
			}
		}
	}
}

func market(category string) domain.Market {
	if category == "linear" {
		return domain.Futures
	}
	return domain.Spot
}

func side(s string) domain.Side {
	if s == "Buy" {
		return domain.Buy
	}
	return domain.Sell
}

func ms(s string) time.Time {
	v, _ := strconv.ParseInt(s, 10, 64)
	return time.UnixMilli(v).UTC()
}

func (t *Trader) onExecution(e wsExecution) {
	if e.ExecType != "Trade" { // funding, ADL and liquidation settle through reconciliation
		if e.ExecType == "AdlTrade" || e.ExecType == "BustTrade" {
			t.c.log.Error("position liquidated or auto-deleveraged", "symbol", e.Symbol, "type", e.ExecType)
			t.requestResync()
		}
		return
	}
	m := market(e.Category)
	fee, baseFee := e.ExecFee, decimal.Zero
	if m == domain.Spot && e.Side == "Buy" {
		// Spot buys pay the fee in the base coin: less arrives in the wallet.
		fee, baseFee = fee.Mul(e.ExecPrice), e.ExecFee
	}
	t.mu.Lock()
	o := t.open[e.OrderLinkID]
	t.mu.Unlock()
	o.ClientID, o.Market, o.Symbol = e.OrderLinkID, m, e.Symbol
	t.emit(broker.Event{Kind: broker.EventFill, Order: o, Fill: domain.Fill{
		OrderClientID: e.OrderLinkID, TradeID: e.ExecID, Exchange: t.c.name, Market: m, Symbol: e.Symbol,
		Side: side(e.Side), Price: e.ExecPrice, Quantity: e.ExecQty, Fee: fee, FeeAsset: "QUOTE", BaseFee: baseFee, Maker: e.IsMaker,
		Time: ms(e.ExecTime),
	}})
}

func status(s string) domain.OrderStatus {
	switch s {
	case "New", "Untriggered", "Triggered", "Created":
		return domain.OrderNew
	case "PartiallyFilled":
		return domain.OrderPartiallyFilled
	case "Filled":
		return domain.OrderFilled
	case "Rejected":
		return domain.OrderRejected
	default: // Cancelled, PartiallyFilledCanceled, Deactivated
		return domain.OrderCanceled
	}
}

func orderType(o wsOrder) domain.OrderType {
	switch {
	case strings.Contains(o.StopType, "StopLoss") || o.StopType == "Stop":
		return domain.OrderStopMarket
	case strings.Contains(o.StopType, "TakeProfit"):
		return domain.OrderTakeProfit
	case o.OrderType == "Market":
		return domain.OrderMarket
	}
	return domain.OrderLimit
}

func (o wsOrder) domain(exchange string) domain.Order {
	d := domain.Order{
		ClientID: o.OrderLinkID, ExchangeID: o.OrderID, Exchange: exchange, Market: market(o.Category),
		Symbol: o.Symbol, Side: side(o.Side), Type: orderType(o), Quantity: o.Qty, Price: o.Price,
		StopPrice: o.TriggerPrice, ReduceOnly: o.ReduceOnly, Status: status(o.OrderStatus),
		FilledQty: o.CumExecQty, AvgPrice: o.AvgPrice, Reason: o.RejectReason,
		CreatedAt: ms(o.CreatedTime), UpdatedAt: ms(o.UpdatedTime),
	}
	if d.Reason == "EC_NoError" {
		d.Reason = ""
	}
	return d
}

func (t *Trader) onOrder(o wsOrder) {
	d := o.domain(t.c.name)
	if d.Status.Open() || d.Status == domain.OrderFilled {
		if d.Status == domain.OrderFilled {
			t.forget(d.ClientID)
		}
		return // fills come from the execution topic
	}
	t.emit(broker.Event{Kind: broker.EventOrder, Order: d})
	t.forget(d.ClientID)
}

func (t *Trader) forget(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.open, id)
}

func (t *Trader) Track(orders []domain.Order) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, o := range orders {
		if o.Exchange == t.c.name {
			t.open[o.ClientID] = o
		}
	}
}

// AccountState fetches the unified wallet, positions and open orders.
func (t *Trader) AccountState(ctx context.Context, m domain.Market) (*broker.AccountState, error) {
	st := &broker.AccountState{Market: m, Balances: map[string]broker.Balance{}}
	var wallet struct {
		List []struct {
			Coin []struct {
				Coin          string          `json:"coin"`
				WalletBalance decimal.Decimal `json:"walletBalance"`
				Locked        decimal.Decimal `json:"locked"`
			} `json:"coin"`
		} `json:"list"`
	}
	if err := t.signed(ctx, http.MethodGet, "/v5/account/wallet-balance", url.Values{"accountType": {"UNIFIED"}}, nil, &wallet); err != nil {
		return nil, fmt.Errorf("wallet balance: %w", err)
	}
	for _, l := range wallet.List {
		for _, c := range l.Coin {
			st.Balances[c.Coin] = broker.Balance{Asset: c.Coin, Free: c.WalletBalance.Sub(c.Locked), Locked: c.Locked, Wallet: c.WalletBalance}
		}
	}

	if m == domain.Futures {
		positions, err := t.positions(ctx)
		if err != nil {
			return nil, err
		}
		st.Positions = positions
	}

	orders, err := t.openOrders(ctx, m)
	if err != nil {
		return nil, err
	}
	st.OpenOrders = orders
	return st, nil
}

func (t *Trader) positions(ctx context.Context) ([]domain.Position, error) {
	var out []domain.Position
	cursor := ""
	for {
		q := url.Values{"category": {"linear"}, "settleCoin": {"USDT"}, "limit": {"200"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var r struct {
			List []struct {
				Symbol      string          `json:"symbol"`
				Side        string          `json:"side"`
				Size        decimal.Decimal `json:"size"`
				AvgPrice    decimal.Decimal `json:"avgPrice"`
				Leverage    string          `json:"leverage"`
				UpdatedTime string          `json:"updatedTime"`
			} `json:"list"`
			Cursor string `json:"nextPageCursor"`
		}
		if err := t.signed(ctx, http.MethodGet, "/v5/position/list", q, nil, &r); err != nil {
			return nil, fmt.Errorf("positions: %w", err)
		}
		for _, p := range r.List {
			if !p.Size.IsPositive() {
				continue
			}
			ps := domain.Long
			if p.Side == "Sell" {
				ps = domain.Short
			}
			lev, _ := strconv.ParseFloat(p.Leverage, 64)
			out = append(out, domain.Position{
				PositionKey: domain.PositionKey{Exchange: t.c.name, Market: domain.Futures, Symbol: p.Symbol},
				Side:        ps, Quantity: p.Size, EntryPrice: p.AvgPrice, Leverage: int(lev), OpenedAt: ms(p.UpdatedTime),
			})
		}
		if r.Cursor == "" || len(r.List) == 0 {
			return out, nil
		}
		cursor = r.Cursor
	}
}

func (t *Trader) openOrders(ctx context.Context, m domain.Market) ([]domain.Order, error) {
	var out []domain.Order
	cursor := ""
	for {
		q := url.Values{"category": {category(m)}, "openOnly": {"0"}, "limit": {"50"}}
		if m == domain.Futures {
			q.Set("settleCoin", "USDT")
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var r struct {
			List   []wsOrder `json:"list"`
			Cursor string    `json:"nextPageCursor"`
		}
		if err := t.signed(ctx, http.MethodGet, "/v5/order/realtime", q, nil, &r); err != nil {
			return nil, fmt.Errorf("open orders: %w", err)
		}
		for _, o := range r.List {
			o.Category = category(m)
			if d := o.domain(t.c.name); d.Status.Open() {
				out = append(out, d)
			}
		}
		if r.Cursor == "" || len(r.List) == 0 {
			return out, nil
		}
		cursor = r.Cursor
	}
}

// SyncOrders emits fills and final states missed while disconnected.
func (t *Trader) SyncOrders(ctx context.Context, tracked, exchangeOpen []domain.Order) {
	open := map[string]bool{}
	for _, o := range exchangeOpen {
		open[o.ClientID] = true
	}
	for _, o := range tracked {
		if o.Exchange != t.c.name || open[o.ClientID] {
			continue
		}
		q := url.Values{"category": {category(o.Market)}, "symbol": {o.Symbol}, "orderLinkId": {o.ClientID}}
		var r struct {
			List []wsOrder `json:"list"`
		}
		err := t.signed(ctx, http.MethodGet, "/v5/order/realtime", q, nil, &r)
		if err == nil && len(r.List) == 0 {
			err = t.signed(ctx, http.MethodGet, "/v5/order/history", q, nil, &r)
		}
		if err != nil || len(r.List) == 0 {
			t.c.log.Warn("order sync failed", "client_id", o.ClientID, "err", errors.Join(err, errNotFound(len(r.List))))
			continue
		}
		r.List[0].Category = category(o.Market)
		final := r.List[0].domain(t.c.name)
		final.Type = o.Type
		if missing := final.FilledQty.Sub(o.FilledQty); missing.IsPositive() && final.AvgPrice.IsPositive() {
			t.c.log.Warn("recovered missed fill", "client_id", o.ClientID, "qty", missing, "price", final.AvgPrice)
			t.emit(broker.Event{Kind: broker.EventFill, Order: final, Fill: domain.Fill{
				OrderClientID: o.ClientID, TradeID: "sync-" + final.ExchangeID + "-" + final.FilledQty.String(),
				Exchange: t.c.name, Market: o.Market, Symbol: o.Symbol, Side: o.Side,
				Price: final.AvgPrice, Quantity: missing, Time: time.Now(),
			}})
		}
		if !final.Status.Open() && final.Status != domain.OrderFilled {
			t.emit(broker.Event{Kind: broker.EventOrder, Order: final})
		}
		t.forget(o.ClientID)
	}
}

func errNotFound(n int) error {
	if n == 0 {
		return errors.New("order not found")
	}
	return nil
}
