package okx

import (
	"context"
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

// Trader trades on an OKX account (spot and USDT swaps, net position mode,
// isolated margin for swaps).
type Trader struct {
	c        *Client
	markets  []domain.Market
	triggers *exchange.TriggerBook

	mu      sync.Mutex
	handler broker.Handler
	open    map[string]domain.Order // by client id (clOrdId or algoClOrdId)
	resync  chan struct{}
}

var (
	_ broker.Live          = (*Trader)(nil)
	_ broker.UnifiedWallet = (*Trader)(nil)
)

func NewTrader(c *Client, apiKey, apiSecret, passphrase string, markets []domain.Market) *Trader {
	c.key, c.secret, c.passphrase = apiKey, []byte(apiSecret), passphrase
	return &Trader{c: c, markets: markets, triggers: exchange.NewTriggerBook(10 * time.Second),
		open: map[string]domain.Order{}, resync: make(chan struct{}, 1)}
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

func tdMode(m domain.Market) string {
	if m == domain.Futures {
		return "isolated"
	}
	return "cash"
}

// toContracts converts a base quantity to the exchange size unit.
func (t *Trader) toContracts(ctx context.Context, m domain.Market, instID string, qty decimal.Decimal) (string, error) {
	ct, err := t.c.contractValue(ctx, m, instID)
	if err != nil {
		return "", err
	}
	return qty.Div(ct).String(), nil
}

func (t *Trader) toBase(ctx context.Context, m domain.Market, instID string, sz decimal.Decimal) decimal.Decimal {
	ct, err := t.c.contractValue(ctx, m, instID)
	if err != nil {
		t.c.log.Error("contract value unknown, size left in contracts", "inst", instID, "err", err)
		return sz
	}
	return sz.Mul(ct)
}

// PlaceOrder implements broker.Broker.
func (t *Trader) PlaceOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	o.Exchange = t.c.name
	sz, err := t.toContracts(ctx, o.Market, o.Symbol, o.Quantity)
	if err != nil {
		return o, err
	}
	t.mu.Lock()
	t.open[o.ClientID] = o
	t.mu.Unlock()

	body := map[string]any{"instId": o.Symbol, "tdMode": tdMode(o.Market), "side": string(o.Side), "sz": sz}
	if o.Market == domain.Futures && o.ReduceOnly {
		body["reduceOnly"] = true
	}
	var r []struct {
		OrdID  string `json:"ordId"`
		AlgoID string `json:"algoId"`
	}
	path := "/api/v5/trade/order"
	switch o.Type {
	case domain.OrderMarket:
		body["ordType"], body["clOrdId"] = "market", o.ClientID
		if o.Market == domain.Spot {
			body["tgtCcy"] = "base_ccy" // market buys default to quote units
		}
	case domain.OrderLimit:
		body["ordType"], body["px"], body["clOrdId"] = "limit", o.Price.String(), o.ClientID
		if o.PostOnly {
			body["ordType"] = "post_only"
		}
	case domain.OrderStopMarket, domain.OrderTakeProfit:
		// Conditional orders do not freeze margin or balance before they
		// trigger, so a stop and a take-profit can coexist.
		path = "/api/v5/trade/order-algo"
		body["ordType"], body["algoClOrdId"] = "conditional", o.ClientID
		pxType := "last"
		if o.Market == domain.Futures {
			pxType = "mark"
			body["reduceOnly"] = true
		}
		if o.Type == domain.OrderStopMarket {
			body["slTriggerPx"], body["slOrdPx"], body["slTriggerPxType"] = o.StopPrice.String(), "-1", pxType
		} else {
			body["tpTriggerPx"], body["tpOrdPx"], body["tpTriggerPxType"] = o.StopPrice.String(), "-1", pxType
		}
	default:
		return o, fmt.Errorf("unsupported order type %s", o.Type)
	}

	if err := t.signed(ctx, http.MethodPost, path, nil, body, &r); err != nil {
		t.mu.Lock()
		delete(t.open, o.ClientID)
		t.mu.Unlock()
		return o, err
	}
	if len(r) > 0 {
		o.ExchangeID = r[0].OrdID
		if r[0].AlgoID != "" {
			o.ExchangeID = "algo-" + r[0].AlgoID
		}
	}
	o.Status = domain.OrderNew
	t.mu.Lock()
	if _, still := t.open[o.ClientID]; still {
		t.open[o.ClientID] = o
	}
	t.mu.Unlock()
	return o, nil
}

func (t *Trader) CancelOrder(ctx context.Context, o domain.Order) error {
	var err error
	if o.Type.IsConditional() {
		err = t.signed(ctx, http.MethodPost, "/api/v5/trade/cancel-algos", nil,
			[]map[string]string{{"instId": o.Symbol, "algoClOrdId": o.ClientID}}, nil)
	} else {
		err = t.signed(ctx, http.MethodPost, "/api/v5/trade/cancel-order", nil,
			map[string]string{"instId": o.Symbol, "clOrdId": o.ClientID}, nil)
	}
	if err != nil {
		t.requestResync() // possibly filled or already gone
	}
	return err
}

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

func (t *Trader) SetLeverage(ctx context.Context, m domain.Market, symbol string, leverage int) error {
	if m != domain.Futures {
		return nil
	}
	err := t.signed(ctx, http.MethodPost, "/api/v5/account/set-leverage", nil,
		map[string]string{"instId": symbol, "lever": strconv.Itoa(leverage), "mgnMode": "isolated"}, nil)
	if err != nil {
		return fmt.Errorf("set leverage: %w", err)
	}
	return nil
}

// Start prepares the account and runs the private stream.
func (t *Trader) Start(ctx context.Context, wg *sync.WaitGroup) error {
	if t.c.key == "" || len(t.c.secret) == 0 || t.c.passphrase == "" {
		return fmt.Errorf("%s: API key, secret and passphrase are required", t.c.name)
	}
	if err := t.c.SyncTime(ctx); err != nil {
		return err
	}
	for _, m := range t.markets {
		if _, err := t.c.Instruments(ctx, m); err != nil { // fills the contract value cache
			return err
		}
	}
	if err := t.signed(ctx, http.MethodPost, "/api/v5/account/set-position-mode", nil, map[string]string{"posMode": "net_mode"}, nil); err != nil {
		t.c.log.Warn("could not set net position mode (open positions or orders?)", "err", err)
	}
	ws := &exchange.WSStream{
		URL: t.c.ep.PrivateWS, HTTPClient: t.c.http, Log: t.c.log.With("stream", "private"),
		IdleTimeout: 6 * time.Hour, Ping: ping, PingInterval: pingInterval, Setup: t.login,
	}
	wg.Go(func() { _ = ws.Run(ctx, t.requestResync, func(b []byte) { t.handlePrivate(ctx, b) }) })
	wg.Go(func() { t.flushLoop(ctx) })
	return nil
}

// login authenticates the session and subscribes once login succeeded.
func (t *Trader) login(ctx context.Context, conn *websocket.Conn) error {
	ts := strconv.FormatInt((time.Now().UnixMilli()+t.c.timeOffset.Load())/1000, 10)
	err := wsjson.Write(ctx, conn, map[string]any{"op": "login", "args": []map[string]string{{
		"apiKey": t.c.key, "passphrase": t.c.passphrase, "timestamp": ts, "sign": t.c.sign(ts + "GET/users/self/verify"),
	}}})
	if err != nil {
		return err
	}
	for {
		var resp struct {
			Event string `json:"event"`
			Code  string `json:"code"`
			Msg   string `json:"msg"`
		}
		if err := wsjson.Read(ctx, conn, &resp); err != nil {
			return fmt.Errorf("login: %w", err)
		}
		switch resp.Event {
		case "login":
			return wsjson.Write(ctx, conn, map[string]any{"op": "subscribe", "args": []wsArg{
				{Channel: "orders", InstType: "ANY"}, {Channel: "orders-algo", InstType: "ANY"},
			}})
		case "error":
			return fmt.Errorf("login: %s %s", resp.Code, resp.Msg)
		}
	}
}

type wsOrder struct {
	InstType   string `json:"instType"`
	InstID     string `json:"instId"`
	OrdID      string `json:"ordId"`
	ClOrdID    string `json:"clOrdId"`
	Side       string `json:"side"`
	OrdType    string `json:"ordType"`
	Sz         string `json:"sz"`
	Px         string `json:"px"`
	State      string `json:"state"`
	AccFillSz  string `json:"accFillSz"`
	AvgPx      string `json:"avgPx"`
	FillSz     string `json:"fillSz"`
	FillPx     string `json:"fillPx"`
	TradeID    string `json:"tradeId"`
	FillTime   string `json:"fillTime"`
	Fee        string `json:"fee"` // negative when charged
	FeeCcy     string `json:"feeCcy"`
	ExecType   string `json:"execType"` // T taker, M maker
	ReduceOnly string `json:"reduceOnly"`
	CTime      string `json:"cTime"`
	UTime      string `json:"uTime"`
}

type wsAlgo struct {
	InstType    string `json:"instType"`
	InstID      string `json:"instId"`
	AlgoID      string `json:"algoId"`
	AlgoClOrdID string `json:"algoClOrdId"`
	OrdID       string `json:"ordId"` // order created when triggered
	State       string `json:"state"`
	Side        string `json:"side"`
	Sz          string `json:"sz"`
	SlTriggerPx string `json:"slTriggerPx"`
	TpTriggerPx string `json:"tpTriggerPx"`
	CTime       string `json:"cTime"`
	UTime       string `json:"uTime"`
}

func market(instType string) domain.Market {
	if instType == "SWAP" {
		return domain.Futures
	}
	return domain.Spot
}

func orderStatus(s string) domain.OrderStatus {
	switch s {
	case "live":
		return domain.OrderNew
	case "partially_filled":
		return domain.OrderPartiallyFilled
	case "filled":
		return domain.OrderFilled
	default: // canceled, mmp_canceled
		return domain.OrderCanceled
	}
}

func algoStatus(s string) domain.OrderStatus {
	switch s {
	case "live", "pause", "effective", "partially_effective":
		return domain.OrderNew
	case "canceled":
		return domain.OrderCanceled
	default: // order_failed, partially_failed
		return domain.OrderRejected
	}
}

func (t *Trader) handlePrivate(ctx context.Context, b []byte) {
	if string(b) == "pong" {
		return
	}
	var msg struct {
		Event string          `json:"event"`
		Msg   string          `json:"msg"`
		Arg   wsArg           `json:"arg"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		return
	}
	if msg.Event == "error" {
		t.c.log.Error("private stream error", "msg", msg.Msg)
		return
	}
	switch msg.Arg.Channel {
	case "orders":
		var orders []wsOrder
		if json.Unmarshal(msg.Data, &orders) == nil {
			for _, o := range orders {
				t.onOrder(ctx, o)
			}
		}
	case "orders-algo":
		var algos []wsAlgo
		if json.Unmarshal(msg.Data, &algos) == nil {
			for _, a := range algos {
				t.onAlgo(a)
			}
		}
	}
}

func (t *Trader) onOrder(ctx context.Context, w wsOrder) {
	m := market(w.InstType)
	id := w.ClOrdID
	if mapped, ok := t.triggers.Lookup(w.OrdID); ok {
		id = mapped
	}
	t.mu.Lock()
	tracked, known := t.open[id]
	t.mu.Unlock()

	o := domain.Order{
		ClientID: id, ExchangeID: w.OrdID, Exchange: t.c.name, Market: m, Symbol: w.InstID,
		Side: domain.Side(w.Side), Type: tracked.Type, Quantity: t.toBase(ctx, m, w.InstID, dec(w.Sz)),
		Price: dec(w.Px), Status: orderStatus(w.State), FilledQty: t.toBase(ctx, m, w.InstID, dec(w.AccFillSz)),
		AvgPrice: dec(w.AvgPx), CreatedAt: msTime(w.CTime), UpdatedAt: msTime(w.UTime),
	}
	var ev broker.Event
	switch {
	case w.TradeID != "" && dec(w.FillSz).IsPositive():
		price := dec(w.FillPx)
		fee := dec(w.Fee).Neg()
		if m == domain.Spot && strings.HasPrefix(w.InstID, w.FeeCcy+"-") {
			fee = fee.Mul(price) // fee charged in the base currency
		}
		ev = broker.Event{Kind: broker.EventFill, Order: o, Fill: domain.Fill{
			OrderClientID: id, TradeID: w.TradeID, Exchange: t.c.name, Market: m, Symbol: w.InstID, Side: o.Side,
			Price: price, Quantity: t.toBase(ctx, m, w.InstID, dec(w.FillSz)), Fee: fee, FeeAsset: w.FeeCcy,
			Maker: w.ExecType == "M", Time: msTime(w.FillTime),
		}}
	case o.Status == domain.OrderCanceled:
		ev = broker.Event{Kind: broker.EventOrder, Order: o}
	default:
		return
	}
	if !known {
		// Possibly created by a conditional order whose orders-algo update
		// (with the mapping) has not arrived yet: hold it briefly.
		t.triggers.Hold(w.OrdID, ev)
		return
	}
	t.emit(ev)
	if !o.Status.Open() {
		t.forget(id)
		t.triggers.Forget(w.OrdID)
	}
}

func (t *Trader) onAlgo(a wsAlgo) {
	t.mu.Lock()
	tracked, ok := t.open[a.AlgoClOrdID]
	t.mu.Unlock()
	if !ok {
		return
	}
	if a.OrdID != "" && a.OrdID != "0" {
		for _, ev := range t.triggers.Map(a.OrdID, a.AlgoClOrdID) {
			ev.Order.Type = tracked.Type
			t.emit(ev)
		}
	}
	switch st := algoStatus(a.State); st {
	case domain.OrderCanceled, domain.OrderRejected:
		o := tracked
		o.Status, o.UpdatedAt = st, msTime(a.UTime)
		if st == domain.OrderRejected {
			o.Reason = a.State
		}
		t.emit(broker.Event{Kind: broker.EventOrder, Order: o})
		t.forget(a.AlgoClOrdID)
	}
}

func (t *Trader) flushLoop(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			for _, ev := range t.triggers.Expired() {
				t.emit(ev)
			}
		}
	}
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

// AccountState fetches balances, positions and open orders.
func (t *Trader) AccountState(ctx context.Context, m domain.Market) (*broker.AccountState, error) {
	st := &broker.AccountState{Market: m, Balances: map[string]broker.Balance{}}
	var bal []struct {
		Details []struct {
			Ccy       string `json:"ccy"`
			CashBal   string `json:"cashBal"`
			AvailBal  string `json:"availBal"`
			FrozenBal string `json:"frozenBal"`
		} `json:"details"`
	}
	if err := t.signed(ctx, http.MethodGet, "/api/v5/account/balance", nil, nil, &bal); err != nil {
		return nil, fmt.Errorf("balance: %w", err)
	}
	for _, b := range bal {
		for _, d := range b.Details {
			st.Balances[d.Ccy] = broker.Balance{Asset: d.Ccy, Free: dec(d.AvailBal), Locked: dec(d.FrozenBal), Wallet: dec(d.CashBal)}
		}
	}

	if m == domain.Futures {
		var ps []struct {
			InstID string `json:"instId"`
			Pos    string `json:"pos"` // contracts, signed in net mode
			AvgPx  string `json:"avgPx"`
			Lever  string `json:"lever"`
			UTime  string `json:"uTime"`
		}
		if err := t.signed(ctx, http.MethodGet, "/api/v5/account/positions", url.Values{"instType": {"SWAP"}}, nil, &ps); err != nil {
			return nil, fmt.Errorf("positions: %w", err)
		}
		for _, p := range ps {
			pos := dec(p.Pos)
			if pos.IsZero() {
				continue
			}
			side := domain.Long
			if pos.IsNegative() {
				side = domain.Short
			}
			lev, _ := strconv.ParseFloat(p.Lever, 64)
			st.Positions = append(st.Positions, domain.Position{
				PositionKey: domain.PositionKey{Exchange: t.c.name, Market: m, Symbol: p.InstID},
				Side:        side, Quantity: t.toBase(ctx, m, p.InstID, pos.Abs()), EntryPrice: dec(p.AvgPx),
				Leverage: int(lev), OpenedAt: msTime(p.UTime),
			})
		}
	}

	var orders []wsOrder
	if err := t.signed(ctx, http.MethodGet, "/api/v5/trade/orders-pending", url.Values{"instType": {instType(m)}}, nil, &orders); err != nil {
		return nil, fmt.Errorf("open orders: %w", err)
	}
	for _, w := range orders {
		st.OpenOrders = append(st.OpenOrders, domain.Order{
			ClientID: w.ClOrdID, ExchangeID: w.OrdID, Exchange: t.c.name, Market: m, Symbol: w.InstID,
			Side: domain.Side(w.Side), Type: okxOrderType(w.OrdType), Quantity: t.toBase(ctx, m, w.InstID, dec(w.Sz)),
			Price: dec(w.Px), Status: orderStatus(w.State), FilledQty: t.toBase(ctx, m, w.InstID, dec(w.AccFillSz)),
			ReduceOnly: w.ReduceOnly == "true", CreatedAt: msTime(w.CTime), UpdatedAt: msTime(w.UTime),
		})
	}
	var algos []wsAlgo
	q := url.Values{"instType": {instType(m)}, "ordType": {"conditional"}}
	if err := t.signed(ctx, http.MethodGet, "/api/v5/trade/orders-algo-pending", q, nil, &algos); err != nil {
		return nil, fmt.Errorf("open algo orders: %w", err)
	}
	for _, a := range algos {
		st.OpenOrders = append(st.OpenOrders, t.algoOrder(ctx, m, a))
	}
	return st, nil
}

func okxOrderType(s string) domain.OrderType {
	if s == "market" {
		return domain.OrderMarket
	}
	return domain.OrderLimit
}

func (t *Trader) algoOrder(ctx context.Context, m domain.Market, a wsAlgo) domain.Order {
	o := domain.Order{
		ClientID: a.AlgoClOrdID, ExchangeID: "algo-" + a.AlgoID, Exchange: t.c.name, Market: m, Symbol: a.InstID,
		Side: domain.Side(a.Side), Type: domain.OrderStopMarket, StopPrice: dec(a.SlTriggerPx),
		Quantity: t.toBase(ctx, m, a.InstID, dec(a.Sz)), ReduceOnly: m == domain.Futures,
		Status: algoStatus(a.State), CreatedAt: msTime(a.CTime), UpdatedAt: msTime(a.UTime),
	}
	if a.TpTriggerPx != "" {
		o.Type, o.StopPrice = domain.OrderTakeProfit, dec(a.TpTriggerPx)
	}
	return o
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
		final, err := t.queryOrder(ctx, o)
		if err != nil {
			t.c.log.Warn("order sync failed", "client_id", o.ClientID, "err", err)
			continue
		}
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

func (t *Trader) queryOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	ordQ := url.Values{"instId": {o.Symbol}, "clOrdId": {o.ClientID}}
	if o.Type.IsConditional() {
		var algos []wsAlgo
		if err := t.signed(ctx, http.MethodGet, "/api/v5/trade/order-algo", url.Values{"algoClOrdId": {o.ClientID}}, nil, &algos); err != nil {
			return o, err
		}
		if len(algos) == 0 {
			return o, errors.New("algo order not found")
		}
		a := algos[0]
		if a.OrdID == "" || a.OrdID == "0" {
			final := t.algoOrder(ctx, o.Market, a)
			if a.State == "effective" { // triggered but no order recorded
				final.Status = domain.OrderExpired
			}
			return final, nil
		}
		ordQ = url.Values{"instId": {o.Symbol}, "ordId": {a.OrdID}}
	}
	var ws []wsOrder
	if err := t.signed(ctx, http.MethodGet, "/api/v5/trade/order", ordQ, nil, &ws); err != nil {
		return o, err
	}
	if len(ws) == 0 {
		return o, errors.New("order not found")
	}
	w := ws[0]
	final := o
	final.ExchangeID, final.Status = w.OrdID, orderStatus(w.State)
	final.FilledQty, final.AvgPrice = t.toBase(ctx, o.Market, o.Symbol, dec(w.AccFillSz)), dec(w.AvgPx)
	final.UpdatedAt = msTime(w.UTime)
	return final, nil
}
