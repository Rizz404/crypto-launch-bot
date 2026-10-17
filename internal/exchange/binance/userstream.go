package binance

import (
	"context"
	"encoding/json"
	"fmt"
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

const (
	listenKeyKeepalive = 30 * time.Minute
	pendingFillTimeout = 10 * time.Second
	// User streams can be silent for hours when nothing trades.
	userStreamIdle = 6 * time.Hour
)

// Start syncs the clock, prepares the futures account and runs the user
// data streams until ctx is cancelled. It returns once streams are started.
func (t *Trader) Start(ctx context.Context, wg *sync.WaitGroup) error {
	if t.key == "" || len(t.secret) == 0 {
		return fmt.Errorf("%s: API key and secret are required", t.c.name)
	}
	if err := t.SyncTime(ctx); err != nil {
		return err
	}
	for _, m := range t.markets {
		switch m {
		case domain.Spot:
			ws := &exchange.WSStream{
				URL: t.c.ep.SpotWSAPI, HTTPClient: t.c.http, Log: t.log.With("stream", "spot-user"),
				IdleTimeout: userStreamIdle, Setup: t.subscribeSpot,
			}
			wg.Go(func() { _ = ws.Run(ctx, t.requestResync, t.handleSpotMessage) })
		case domain.Futures:
			if err := t.EnsureOneWay(ctx); err != nil {
				return err
			}
			ws := &exchange.WSStream{
				HTTPClient: t.c.http, Log: t.log.With("stream", "futures-user"), IdleTimeout: userStreamIdle,
				URLFunc: func(ctx context.Context) (string, error) {
					key, err := t.listenKey(ctx, "POST")
					if err != nil {
						return "", err
					}
					return t.c.ep.FuturesPrivateWS + "/stream?streams=" + key, nil
				},
			}
			wg.Go(func() { _ = ws.Run(ctx, t.requestResync, func(b []byte) { t.handleFuturesMessage(b, ws) }) })
			wg.Go(func() { t.keepalive(ctx, ws) })
		}
	}
	wg.Go(func() { t.flushPendingLoop(ctx) })
	return nil
}

// subscribeSpot subscribes the WS API session to the account's events.
func (t *Trader) subscribeSpot(ctx context.Context, conn *websocket.Conn) error {
	ts := strconv.FormatInt(t.now(), 10)
	// WS API signature: params sorted by name, joined like a query string.
	payload := "apiKey=" + t.key + "&timestamp=" + ts
	req := map[string]any{
		"id":     "uds-" + ts,
		"method": "userDataStream.subscribe.signature",
		"params": map[string]any{"apiKey": t.key, "timestamp": json.Number(ts), "signature": t.sign(payload)},
	}
	return wsjson.Write(ctx, conn, req)
}

func (t *Trader) listenKey(ctx context.Context, method string) (string, error) {
	var r struct {
		ListenKey string `json:"listenKey"`
	}
	if err := t.keyed(ctx, domain.Futures, method, "/fapi/v1/listenKey", 1, &r); err != nil {
		return "", fmt.Errorf("listen key: %w", err)
	}
	return r.ListenKey, nil
}

func (t *Trader) keepalive(ctx context.Context, ws *exchange.WSStream) {
	tick := time.NewTicker(listenKeyKeepalive)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if _, err := t.listenKey(ctx, "PUT"); err != nil {
				t.log.Warn("listen key keepalive failed, reconnecting", "err", err)
				ws.Kick()
			}
		}
	}
}

// spotExecutionReport mirrors the spot executionReport event. Binance uses
// keys that differ only in case; every one of them gets a field so that
// encoding/json's case-insensitive matching cannot mix them up.
type spotExecutionReport struct {
	Event        string          `json:"e"`
	EventTime    int64           `json:"E"`
	Symbol       string          `json:"s"`
	Side         string          `json:"S"`
	ClientID     string          `json:"c"`
	OrigClientID string          `json:"C"`
	Type         string          `json:"o"`
	CreateTime   int64           `json:"O"`
	TimeInForce  string          `json:"f"`
	IcebergQty   json.RawMessage `json:"F"`
	Quantity     decimal.Decimal `json:"q"`
	QuoteOrderQ  json.RawMessage `json:"Q"`
	Price        decimal.Decimal `json:"p"`
	StopPrice    decimal.Decimal `json:"P"`
	ExecType     string          `json:"x"`
	Status       string          `json:"X"`
	RejectReason string          `json:"r"`
	ListID       int64           `json:"g"`
	OrderID      int64           `json:"i"`
	ExecID       json.RawMessage `json:"I"`
	LastQty      decimal.Decimal `json:"l"`
	LastPrice    decimal.Decimal `json:"L"`
	Commission   decimal.Decimal `json:"n"`
	CommAsset    *string         `json:"N"`
	TradeTime    int64           `json:"T"`
	TradeID      int64           `json:"t"`
	CumQty       decimal.Decimal `json:"z"`
	CumQuote     decimal.Decimal `json:"Z"`
	Maker        bool            `json:"m"`
	Ignore       json.RawMessage `json:"M"`
	OnBook       json.RawMessage `json:"w"`
	WorkingTime  json.RawMessage `json:"W"`
	LastQuoteQty json.RawMessage `json:"Y"`
	PreventedID  json.RawMessage `json:"v"`
	STPMode      json.RawMessage `json:"V"`
	TrailDelta   json.RawMessage `json:"d"`
	TrailTime    json.RawMessage `json:"D"`
	StrategyID   json.RawMessage `json:"j"`
	StrategyType json.RawMessage `json:"J"`
	PrevQty      json.RawMessage `json:"A"`
	LastPrevQty  json.RawMessage `json:"B"`
	TradeGroup   json.RawMessage `json:"u"`
	CounterOrder json.RawMessage `json:"U"`
	MatchType    json.RawMessage `json:"b"`
	AllocID      json.RawMessage `json:"a"`
	PrevExecQty  json.RawMessage `json:"pl"`
	PrevExecPx   json.RawMessage `json:"pL"`
	PegType      json.RawMessage `json:"gP"`
	PegPrice     json.RawMessage `json:"gp"`
}

func (t *Trader) handleSpotMessage(b []byte) {
	if t.tap != nil {
		t.tap("spot-user", b)
	}
	var msg struct {
		ID     string          `json:"id"`
		Status int             `json:"status"`
		Error  json.RawMessage `json:"error"`
		Event  json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.log.Warn("bad spot user stream message", "err", err)
		return
	}
	if msg.ID != "" { // response to our subscribe request
		if msg.Status != 200 {
			t.log.Error("spot user stream subscription failed", "status", msg.Status, "error", string(msg.Error))
		} else {
			t.log.Info("spot user stream subscribed")
		}
		return
	}
	if len(msg.Event) == 0 {
		return
	}
	var head struct {
		Event string `json:"e"`
		Time  int64  `json:"E"`
	}
	_ = json.Unmarshal(msg.Event, &head)
	switch head.Event {
	case "executionReport":
		var r spotExecutionReport
		if err := json.Unmarshal(msg.Event, &r); err != nil {
			t.log.Warn("bad executionReport", "err", err)
			return
		}
		t.onSpotExecution(r)
	case "eventStreamTerminated":
		t.log.Warn("spot user stream terminated by exchange")
		t.requestResync()
	}
}

func (t *Trader) onSpotExecution(r spotExecutionReport) {
	id := r.ClientID
	if r.ExecType == "CANCELED" && r.OrigClientID != "" {
		id = r.OrigClientID
	}
	o := domain.Order{
		ClientID: id, ExchangeID: strconv.FormatInt(r.OrderID, 10), Exchange: t.c.name, Market: domain.Spot,
		Symbol: r.Symbol, Side: domain.Side(strings.ToLower(r.Side)), Type: orderType(r.Type),
		Quantity: r.Quantity, Price: r.Price, StopPrice: r.StopPrice, Status: orderStatus(r.Status),
		FilledQty: r.CumQty, UpdatedAt: msTime(r.EventTime),
	}
	if r.CumQty.IsPositive() {
		o.AvgPrice = r.CumQuote.Div(r.CumQty)
	}
	if r.RejectReason != "" && r.RejectReason != "NONE" {
		o.Reason = r.RejectReason
	}

	switch r.ExecType {
	case "TRADE":
		asset := ""
		if r.CommAsset != nil {
			asset = *r.CommAsset
		}
		f := domain.Fill{
			OrderClientID: id, TradeID: strconv.FormatInt(r.TradeID, 10), Exchange: t.c.name, Market: domain.Spot,
			Symbol: r.Symbol, Side: o.Side, Price: r.LastPrice, Quantity: r.LastQty,
			Fee: t.feeInQuote(r.Symbol, r.Commission, asset, r.LastPrice), FeeAsset: asset, Maker: r.Maker,
			Time: msTime(r.TradeTime),
		}
		t.emit(broker.Event{Kind: broker.EventFill, Order: o, Fill: f})
	case "CANCELED", "EXPIRED", "REJECTED", "TRADE_PREVENTION":
		t.emit(broker.Event{Kind: broker.EventOrder, Order: o})
	default: // NEW, REPLACED, AMENDMENT
		return
	}
	if !o.Status.Open() {
		t.mu.Lock()
		delete(t.open, id)
		t.mu.Unlock()
	}
}

// feeInQuote converts a commission to the quote currency where possible.
// Commissions paid in a third asset (e.g. BNB) are reported as zero here
// and show up in the balance on reconciliation.
func (t *Trader) feeInQuote(symbol string, fee decimal.Decimal, asset string, price decimal.Decimal) decimal.Decimal {
	switch {
	case fee.IsZero() || asset == "":
		return decimal.Zero
	case strings.HasSuffix(symbol, asset):
		return fee
	case strings.HasPrefix(symbol, asset):
		return fee.Mul(price)
	}
	return decimal.Zero
}

// futuresOrderUpdate mirrors the "o" object of ORDER_TRADE_UPDATE, with a
// field for every case-twin key.
type futuresOrderUpdate struct {
	Symbol        string          `json:"s"`
	Side          string          `json:"S"`
	ClientID      string          `json:"c"`
	Type          string          `json:"o"`
	TimeInForce   string          `json:"f"`
	Quantity      decimal.Decimal `json:"q"`
	Price         decimal.Decimal `json:"p"`
	AvgPrice      decimal.Decimal `json:"ap"`
	ActivatePrice json.RawMessage `json:"AP"`
	StopPrice     decimal.Decimal `json:"sp"`
	ExecType      string          `json:"x"`
	Status        string          `json:"X"`
	OrderID       int64           `json:"i"`
	LastQty       decimal.Decimal `json:"l"`
	LastPrice     decimal.Decimal `json:"L"`
	CumQty        decimal.Decimal `json:"z"`
	Commission    decimal.Decimal `json:"n"`
	CommAsset     string          `json:"N"`
	TradeTime     int64           `json:"T"`
	TradeID       int64           `json:"t"`
	BidNotional   json.RawMessage `json:"b"`
	AskNotional   json.RawMessage `json:"a"`
	Maker         bool            `json:"m"`
	Ignore        json.RawMessage `json:"M"`
	ReduceOnly    bool            `json:"R"`
	OrigType      string          `json:"ot"`
	RealizedPnL   json.RawMessage `json:"rp"`
	STPMode       json.RawMessage `json:"V"`
	ExpiryReason  string          `json:"er"`
}

// futuresAlgoUpdate mirrors the "o" object of ALGO_UPDATE.
type futuresAlgoUpdate struct {
	ClientAlgoID string          `json:"caid"`
	AlgoID       int64           `json:"aid"`
	Symbol       string          `json:"s"`
	Side         string          `json:"S"`
	OrderType    string          `json:"o"`
	Quantity     decimal.Decimal `json:"q"`
	Status       string          `json:"X"`
	ActualID     json.RawMessage `json:"ai"` // documented as string; accept numbers too
	ExecutedQty  decimal.Decimal `json:"aq"`
	TriggerPrice decimal.Decimal `json:"tp"`
	RejectReason string          `json:"rm"`
}

func (t *Trader) handleFuturesMessage(b []byte, ws *exchange.WSStream) {
	if t.tap != nil {
		t.tap("futures-user", b)
	}
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil || len(wrap.Data) == 0 {
		return
	}
	var head struct {
		Event string          `json:"e"`
		Time  int64           `json:"E"`
		TxnT  int64           `json:"T"`
		O     json.RawMessage `json:"o"`
	}
	if err := json.Unmarshal(wrap.Data, &head); err != nil {
		t.log.Warn("bad futures user stream message", "err", err)
		return
	}
	switch head.Event {
	case "ORDER_TRADE_UPDATE":
		var u futuresOrderUpdate
		if err := json.Unmarshal(head.O, &u); err != nil {
			t.log.Warn("bad ORDER_TRADE_UPDATE", "err", err)
			return
		}
		t.onFuturesOrder(u, head.Time)
	case "ALGO_UPDATE":
		var u futuresAlgoUpdate
		if err := json.Unmarshal(head.O, &u); err != nil {
			t.log.Warn("bad ALGO_UPDATE", "err", err)
			return
		}
		t.onFuturesAlgo(u, head.Time)
	case "listenKeyExpired":
		t.log.Warn("futures listen key expired, reconnecting")
		ws.Kick()
	}
}

func (t *Trader) onFuturesOrder(u futuresOrderUpdate, eventMs int64) {
	t.mu.Lock()
	id := u.ClientID
	if _, known := t.open[id]; !known {
		if mapped, ok := t.triggered[u.OrderID]; ok {
			id = mapped
		}
	}
	_, known := t.open[id]
	t.mu.Unlock()

	o := domain.Order{
		ClientID: id, ExchangeID: strconv.FormatInt(u.OrderID, 10), Exchange: t.c.name, Market: domain.Futures,
		Symbol: u.Symbol, Side: domain.Side(strings.ToLower(u.Side)), Type: orderType(u.Type),
		Quantity: u.Quantity, Price: u.Price, StopPrice: u.StopPrice, ReduceOnly: u.ReduceOnly,
		Status: orderStatus(u.Status), FilledQty: u.CumQty, AvgPrice: u.AvgPrice, UpdatedAt: msTime(eventMs),
		Reason: u.ExpiryReason,
	}
	var ev broker.Event
	switch u.ExecType {
	case "TRADE":
		f := domain.Fill{
			OrderClientID: id, TradeID: strconv.FormatInt(u.TradeID, 10), Exchange: t.c.name, Market: domain.Futures,
			Symbol: u.Symbol, Side: o.Side, Price: u.LastPrice, Quantity: u.LastQty,
			Fee: t.feeInQuote(u.Symbol, u.Commission, u.CommAsset, u.LastPrice), FeeAsset: u.CommAsset, Maker: u.Maker,
			Time: msTime(u.TradeTime),
		}
		ev = broker.Event{Kind: broker.EventFill, Order: o, Fill: f}
	case "CANCELED", "EXPIRED", "REJECTED":
		ev = broker.Event{Kind: broker.EventOrder, Order: o}
	case "CALCULATED": // liquidation or ADL
		t.log.Error("position liquidated or auto-deleveraged", "symbol", u.Symbol, "qty", u.LastQty, "price", u.LastPrice)
		t.requestResync()
		return
	default:
		return
	}

	if !known {
		// Possibly the order created by a triggered algo whose ALGO_UPDATE
		// has not arrived yet: hold it briefly.
		t.mu.Lock()
		t.pendingFills[u.OrderID] = append(t.pendingFills[u.OrderID], pendingFill{ev: ev, at: time.Now()})
		t.mu.Unlock()
		return
	}
	t.emit(ev)
	if !o.Status.Open() {
		t.forget(id, u.OrderID)
	}
}

func (t *Trader) onFuturesAlgo(u futuresAlgoUpdate, eventMs int64) {
	t.mu.Lock()
	st, ok := t.algos[u.ClientAlgoID]
	t.mu.Unlock()
	if !ok {
		return // not ours
	}
	actual := strings.Trim(string(u.ActualID), `"`)
	if id, err := strconv.ParseInt(actual, 10, 64); err == nil && id > 0 {
		t.mu.Lock()
		st.orderID = id
		t.triggered[id] = u.ClientAlgoID
		pending := t.pendingFills[id]
		delete(t.pendingFills, id)
		t.mu.Unlock()
		for _, p := range pending {
			p.ev.Order.ClientID, p.ev.Fill.OrderClientID = u.ClientAlgoID, u.ClientAlgoID
			p.ev.Order.Type = st.order.Type
			t.emit(p.ev)
		}
	}

	o := st.order
	o.UpdatedAt = msTime(eventMs)
	switch u.Status {
	case "CANCELED", "REJECTED", "EXPIRED":
		o.Status, o.Reason = algoStatus(u.Status), u.RejectReason
		t.emit(broker.Event{Kind: broker.EventOrder, Order: o})
		t.forget(u.ClientAlgoID, st.orderID)
	case "FINISHED":
		if !u.ExecutedQty.IsPositive() {
			// Triggered but nothing executed (e.g. reduce-only with no
			// position left).
			o.Status, o.Reason = domain.OrderExpired, "finished without execution"
			t.emit(broker.Event{Kind: broker.EventOrder, Order: o})
			t.forget(u.ClientAlgoID, st.orderID)
		}
	}
}

func (t *Trader) forget(clientID string, orderID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.open, clientID)
	delete(t.algos, clientID)
	if orderID != 0 {
		delete(t.triggered, orderID)
	}
}

// flushPendingLoop emits held events whose algo mapping never arrived.
func (t *Trader) flushPendingLoop(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			var due []broker.Event
			t.mu.Lock()
			for id, ps := range t.pendingFills {
				if time.Since(ps[0].at) < pendingFillTimeout {
					continue
				}
				for _, p := range ps {
					due = append(due, p.ev)
				}
				delete(t.pendingFills, id)
			}
			t.mu.Unlock()
			for _, ev := range due {
				t.emit(ev) // unknown to the engine unless placed outside the bot
			}
		}
	}
}
