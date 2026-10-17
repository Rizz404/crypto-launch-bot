//go:build testnet

package binance

// Places real orders on the Binance Spot Testnet (never mainnet: the client
// is built with Testnet endpoints). Opt-in:
//
//	TB_TESTNET_ORDERS=1 TB_EXCHANGES__BINANCE__API_KEY=... TB_EXCHANGES__BINANCE__API_SECRET=... \
//	  go test -count=1 -tags testnet -run TestSpotTestnet -v ./internal/exchange/binance
//
// TB_TESTNET_RECORD=1 also writes the order REST responses and user stream
// messages to testdata/testnet/<test>.json (no credentials are in them).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

const testnetSymbol = "BTCUSDT"

// testnetEvents collects broker events and log messages of one trader.
type testnetEvents struct {
	mu     sync.Mutex
	events []broker.Event
	logs   []string
}

func (e *testnetEvents) handle(ev broker.Event) {
	e.mu.Lock()
	e.events = append(e.events, ev)
	e.mu.Unlock()
}

// wait returns the first event matching ok, failing after timeout.
func (e *testnetEvents) wait(t *testing.T, what string, timeout time.Duration, ok func(broker.Event) bool) broker.Event {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		for _, ev := range e.events {
			if ok(ev) {
				e.mu.Unlock()
				return ev
			}
		}
		e.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
	return broker.Event{}
}

func (e *testnetEvents) fills(clientID string) []domain.Fill {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []domain.Fill
	for _, ev := range e.events {
		if ev.Kind == broker.EventFill && ev.Fill.OrderClientID == clientID {
			out = append(out, ev.Fill)
		}
	}
	return out
}

func (e *testnetEvents) logged(msg string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, m := range e.logs {
		if m == msg {
			return true
		}
	}
	return false
}

// logRecorder keeps message texts so the test can wait for log-only signals
// (the user stream subscription has no broker event).
type logRecorder struct{ e *testnetEvents }

func (h logRecorder) Enabled(context.Context, slog.Level) bool { return true }
func (h logRecorder) Handle(_ context.Context, r slog.Record) error {
	h.e.mu.Lock()
	h.e.logs = append(h.e.logs, r.Message)
	h.e.mu.Unlock()
	return nil
}
func (h logRecorder) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h logRecorder) WithGroup(string) slog.Handler      { return h }

// recordedPayload is one exchange message kept as a fixture.
type recordedPayload struct {
	Stream string          `json:"stream"` // "rest" or the user stream name
	Method string          `json:"method,omitempty"`
	Path   string          `json:"path,omitempty"`
	Body   json.RawMessage `json:"body"`
}

// payloadRecorder collects order REST responses and user stream messages.
type payloadRecorder struct {
	mu       sync.Mutex
	payloads []recordedPayload
}

func (r *payloadRecorder) add(p recordedPayload) {
	r.mu.Lock()
	r.payloads = append(r.payloads, p)
	r.mu.Unlock()
}

// recordedPaths are the REST responses worth keeping; account balances and
// market data are left out.
var recordedPaths = map[string]bool{"/api/v3/order": true, "/api/v3/orderList/oco": true, "/api/v3/openOrders": true}

type recordingTransport struct{ rec *payloadRecorder }

func (rt recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	// The websocket handshake (101) must keep its body untouched.
	if err != nil || resp.StatusCode != http.StatusOK || !recordedPaths[req.URL.Path] {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	rt.rec.add(recordedPayload{Stream: "rest", Method: req.Method, Path: req.URL.Path, Body: body})
	return resp, nil
}

// newPayloadRecorder returns nil unless TB_TESTNET_RECORD=1; the fixture is
// written when the test ends, after its streams have stopped.
func newPayloadRecorder(t *testing.T) *payloadRecorder {
	t.Helper()
	if os.Getenv("TB_TESTNET_RECORD") != "1" {
		return nil
	}
	rec := &payloadRecorder{}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		rec.mu.Lock()
		defer rec.mu.Unlock()
		b, err := json.MarshalIndent(rec.payloads, "", "  ")
		require.NoError(t, err)
		path := filepath.Join("testdata", "testnet", t.Name()+".json")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, append(b, '\n'), 0o644))
		t.Logf("recorded %d payloads to %s", len(rec.payloads), path)
	})
	return rec
}

// startTestnetTrader starts a spot trader and waits for its user stream.
func startTestnetTrader(t *testing.T, ctx context.Context, rec *payloadRecorder) (*Trader, *testnetEvents) {
	t.Helper()
	ev := &testnetEvents{}
	log := slog.New(slog.NewMultiHandler(logRecorder{ev}, slog.NewTextHandler(testWriter{t}, nil)))
	opts := Options{Testnet: true, Log: log}
	if rec != nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second, Transport: recordingTransport{rec}}
	}
	tr := NewTrader(New(opts),
		os.Getenv("TB_EXCHANGES__BINANCE__API_KEY"), os.Getenv("TB_EXCHANGES__BINANCE__API_SECRET"),
		[]domain.Market{domain.Spot})
	require.Equal(t, "binance-testnet", tr.Exchange())
	tr.SetHandler(ev.handle)
	if rec != nil {
		tr.tap = func(stream string, msg []byte) {
			rec.add(recordedPayload{Stream: stream, Body: append(json.RawMessage(nil), msg...)})
		}
	}
	var wg sync.WaitGroup
	streamCtx, stop := context.WithCancel(ctx)
	t.Cleanup(func() { stop(); wg.Wait() })
	require.NoError(t, tr.Start(streamCtx, &wg))
	require.Eventually(t, func() bool { return ev.logged("spot user stream subscribed") }, 15*time.Second, 100*time.Millisecond)
	return tr, ev
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

var _ io.Writer = testWriter{}

func testnetSetup(t *testing.T) (context.Context, domain.Instrument, decimal.Decimal) {
	t.Helper()
	if os.Getenv("TB_TESTNET_ORDERS") != "1" {
		t.Skip("set TB_TESTNET_ORDERS=1 to place orders on the Binance Spot Testnet")
	}
	if os.Getenv("TB_EXCHANGES__BINANCE__API_KEY") == "" || os.Getenv("TB_EXCHANGES__BINANCE__API_SECRET") == "" {
		t.Skip("Spot Testnet API key and secret are not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	c := New(Options{Testnet: true})
	instruments, err := c.Instruments(ctx, domain.Spot)
	require.NoError(t, err)
	var ins domain.Instrument
	for _, i := range instruments {
		if i.Symbol == testnetSymbol {
			ins = i
		}
	}
	require.True(t, ins.Tradable, "%s not tradable on the testnet", testnetSymbol)
	bars, err := c.Klines(ctx, exchange.KlineRequest{Market: domain.Spot, Symbol: testnetSymbol, Timeframe: "1m",
		Start: time.Now().Add(-5 * time.Minute), Limit: 5})
	require.NoError(t, err)
	require.NotEmpty(t, bars)
	return ctx, ins, bars[len(bars)-1].Close
}

var testnetSeq int

// testnetID is a unique alphanumeric client order id.
func testnetID(t *testing.T) string {
	testnetSeq++
	return fmt.Sprintf("tbit%s%d", strconv.FormatInt(time.Now().UnixMilli(), 36), testnetSeq)
}

// orderQty is the smallest step-rounded quantity worth well above the
// minimum notional at price.
func orderQty(ins domain.Instrument, price decimal.Decimal) decimal.Decimal {
	notional := decimal.Max(ins.MinNotional.Mul(decimal.NewFromInt(3)), decimal.NewFromInt(20))
	q := ins.RoundQty(notional.Div(price)).Add(ins.StepSize)
	return decimal.Max(q, ins.MinQty)
}

func freeBalance(t *testing.T, ctx context.Context, tr *Trader, asset string) decimal.Decimal {
	t.Helper()
	st, err := tr.AccountState(ctx, domain.Spot)
	require.NoError(t, err)
	return st.Balances[asset].Free
}

func openOrderIDs(t *testing.T, ctx context.Context, tr *Trader) map[string]domain.Order {
	t.Helper()
	st, err := tr.AccountState(ctx, domain.Spot)
	require.NoError(t, err)
	out := map[string]domain.Order{}
	for _, o := range st.OpenOrders {
		out[o.ClientID] = o
	}
	return out
}

// TestSpotTestnetLifecycle: market entry, OCO protection, cancelling one
// leg cancels the list, the second cancel is an unknown order, flatten.
func TestSpotTestnetLifecycle(t *testing.T) {
	ctx, ins, price := testnetSetup(t)
	tr, ev := startTestnetTrader(t, ctx, newPayloadRecorder(t))
	base := strings.TrimSuffix(testnetSymbol, ins.Quote)

	require.True(t, freeBalance(t, ctx, tr, ins.Quote).GreaterThan(decimal.NewFromInt(100)), "need testnet %s", ins.Quote)
	baseBefore := freeBalance(t, ctx, tr, base)

	qty := orderQty(ins, price)
	entry := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Buy,
		Type: domain.OrderMarket, Quantity: qty}
	placed, err := tr.PlaceOrder(ctx, entry)
	require.NoError(t, err)
	assert.NotEmpty(t, placed.ExchangeID)
	assert.Equal(t, "binance-testnet", placed.Exchange)
	filled := ev.wait(t, "entry filled", 20*time.Second, func(e broker.Event) bool {
		return e.Kind == broker.EventFill && e.Order.ClientID == entry.ClientID && e.Order.Status == domain.OrderFilled
	})
	sum := decimal.Zero
	for _, f := range ev.fills(entry.ClientID) {
		sum = sum.Add(f.Quantity)
		assert.True(t, f.Price.IsPositive())
		assert.False(t, f.Fee.IsNegative())
		assert.NotEmpty(t, f.TradeID)
	}
	assert.True(t, sum.Equal(qty), "fills %s != ordered %s", sum, qty)
	assert.True(t, filled.Order.FilledQty.Equal(qty))
	t.Logf("entry filled: qty=%s avg=%s fills=%d", qty, filled.Order.AvgPrice, len(ev.fills(entry.ClientID)))

	// A fee in the base asset leaves less than qty to sell.
	held := ins.RoundQty(decimal.Min(qty, freeBalance(t, ctx, tr, base).Sub(baseBefore)))
	require.True(t, held.IsPositive())
	ref := filled.Order.AvgPrice
	stop := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Sell,
		Type: domain.OrderStopMarket, Quantity: held, StopPrice: ins.RoundPrice(ref.Mul(decimal.RequireFromString("0.95")))}
	target := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Sell,
		Type: domain.OrderTakeProfit, Quantity: held, StopPrice: ins.RoundPrice(ref.Mul(decimal.RequireFromString("1.05")))}
	legs, err := tr.PlaceBracket(ctx, stop, target)
	require.NoError(t, err)
	require.Len(t, legs, 2)
	for _, l := range legs {
		assert.NotEmpty(t, l.ExchangeID, "leg %s", l.ClientID)
	}
	open := openOrderIDs(t, ctx, tr)
	require.Contains(t, open, stop.ClientID)
	require.Contains(t, open, target.ClientID)
	assert.Equal(t, domain.OrderStopMarket, open[stop.ClientID].Type)
	assert.Equal(t, domain.OrderTakeProfit, open[target.ClientID].Type)
	assert.True(t, open[stop.ClientID].StopPrice.Equal(stop.StopPrice))

	require.NoError(t, tr.CancelOrder(ctx, stop))
	for _, id := range []string{stop.ClientID, target.ClientID} {
		ev.wait(t, "canceled "+id, 20*time.Second, func(e broker.Event) bool {
			return e.Kind == broker.EventOrder && e.Order.ClientID == id && e.Order.Status == domain.OrderCanceled
		})
	}
	// The list is gone: cancelling the other leg is an unknown order.
	err = tr.CancelOrder(ctx, target)
	require.Error(t, err)
	assert.Equal(t, -2011, apiCode(err))
	select {
	case <-tr.Resync():
	case <-time.After(time.Second):
		t.Error("an unknown-order cancel must request reconciliation")
	}
	open = openOrderIDs(t, ctx, tr)
	assert.NotContains(t, open, stop.ClientID)
	assert.NotContains(t, open, target.ClientID)

	exit := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Sell,
		Type: domain.OrderMarket, Quantity: held}
	_, err = tr.PlaceOrder(ctx, exit)
	require.NoError(t, err)
	ev.wait(t, "exit filled", 20*time.Second, func(e broker.Event) bool {
		return e.Kind == broker.EventFill && e.Order.ClientID == exit.ClientID && e.Order.Status == domain.OrderFilled
	})
}

// TestSpotTestnetRestartReconciliation: an order placed by one session is
// found, adopted and cancelled by the next, and an order that closed while
// the bot was down is settled by SyncOrders.
func TestSpotTestnetRestartReconciliation(t *testing.T) {
	ctx, ins, price := testnetSetup(t)
	rec := newPayloadRecorder(t)
	first, _ := startTestnetTrader(t, ctx, rec)
	low := ins.RoundPrice(price.Mul(decimal.RequireFromString("0.8")))
	qty := orderQty(ins, low)
	resting := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Buy,
		Type: domain.OrderLimit, Quantity: qty, Price: low}
	resting, err := first.PlaceOrder(ctx, resting)
	require.NoError(t, err)
	gone := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Buy,
		Type: domain.OrderLimit, Quantity: qty, Price: low}
	gone, err = first.PlaceOrder(ctx, gone)
	require.NoError(t, err)
	// "gone" is cancelled while the next session is not running yet.
	require.NoError(t, first.CancelOrder(ctx, gone))

	second, ev := startTestnetTrader(t, ctx, rec)
	open := openOrderIDs(t, ctx, second)
	require.Contains(t, open, resting.ClientID, "restart must find the resting order")
	assert.NotContains(t, open, gone.ClientID)
	adopted := open[resting.ClientID]
	assert.Equal(t, domain.OrderLimit, adopted.Type)
	assert.True(t, adopted.Price.Equal(low))
	assert.True(t, adopted.Quantity.Equal(qty))

	var exchangeOpen []domain.Order
	for _, o := range open {
		exchangeOpen = append(exchangeOpen, o)
	}
	second.Track([]domain.Order{resting, gone})
	second.SyncOrders(ctx, []domain.Order{resting, gone}, exchangeOpen)
	settled := ev.wait(t, "final state of the order closed while down", 10*time.Second, func(e broker.Event) bool {
		return e.Order.ClientID == gone.ClientID
	})
	assert.Equal(t, domain.OrderCanceled, settled.Order.Status)

	require.NoError(t, second.CancelOrder(ctx, adopted))
	ev.wait(t, "adopted order canceled", 20*time.Second, func(e broker.Event) bool {
		return e.Kind == broker.EventOrder && e.Order.ClientID == resting.ClientID && e.Order.Status == domain.OrderCanceled
	})
	assert.NotContains(t, openOrderIDs(t, ctx, second), resting.ClientID)
}

// TestSpotTestnetTakeProfitTriggers: the take-profit leg of an OCO fills on
// the exchange and the stop leg expires. Best effort: the target sits just
// above the market, and the test skips if the price does not reach it.
func TestSpotTestnetTakeProfitTriggers(t *testing.T) {
	ctx, ins, price := testnetSetup(t)
	tr, ev := startTestnetTrader(t, ctx, newPayloadRecorder(t))
	base := strings.TrimSuffix(testnetSymbol, ins.Quote)
	baseBefore := freeBalance(t, ctx, tr, base)

	qty := orderQty(ins, price)
	entry := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Buy,
		Type: domain.OrderMarket, Quantity: qty}
	_, err := tr.PlaceOrder(ctx, entry)
	require.NoError(t, err)
	filled := ev.wait(t, "entry filled", 20*time.Second, func(e broker.Event) bool {
		return e.Kind == broker.EventFill && e.Order.ClientID == entry.ClientID && e.Order.Status == domain.OrderFilled
	})
	held := ins.RoundQty(decimal.Min(qty, freeBalance(t, ctx, tr, base).Sub(baseBefore)))
	ref := filled.Order.AvgPrice
	stop := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Sell,
		Type: domain.OrderStopMarket, Quantity: held, StopPrice: ins.RoundPrice(ref.Mul(decimal.RequireFromString("0.95")))}
	target := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Sell,
		Type: domain.OrderTakeProfit, Quantity: held,
		StopPrice: ins.RoundPrice(ref.Mul(decimal.RequireFromString("1.0002"))).Add(ins.TickSize)}
	_, err = tr.PlaceBracket(ctx, stop, target)
	require.NoError(t, err)

	triggered := false
	deadline := time.Now().Add(2 * time.Minute)
	for !triggered && time.Now().Before(deadline) {
		time.Sleep(time.Second)
		ev.mu.Lock()
		for _, e := range ev.events {
			if e.Kind == broker.EventFill && e.Order.ClientID == target.ClientID && e.Order.Status == domain.OrderFilled {
				triggered = true
			}
		}
		ev.mu.Unlock()
	}
	if !triggered {
		err := tr.CancelOrder(ctx, stop)
		if apiCode(err) == -2011 { // the list closed just now: the target filled
			ev.wait(t, "take-profit filled", 20*time.Second, func(e broker.Event) bool {
				return e.Kind == broker.EventFill && e.Order.ClientID == target.ClientID && e.Order.Status == domain.OrderFilled
			})
			triggered = true
		} else {
			require.NoError(t, err)
		}
	}
	if !triggered {
		exit := domain.Order{ClientID: testnetID(t), Market: domain.Spot, Symbol: testnetSymbol, Side: domain.Sell,
			Type: domain.OrderMarket, Quantity: held}
		_, err = tr.PlaceOrder(ctx, exit)
		require.NoError(t, err)
		ev.wait(t, "exit filled", 20*time.Second, func(e broker.Event) bool {
			return e.Kind == broker.EventFill && e.Order.ClientID == exit.ClientID && e.Order.Status == domain.OrderFilled
		})
		t.Skipf("price did not reach the take-profit %s within 2 minutes (entry %s)", target.StopPrice, ref)
	}
	sum := decimal.Zero
	for _, f := range ev.fills(target.ClientID) {
		sum = sum.Add(f.Quantity)
	}
	assert.True(t, sum.Equal(held), "take-profit fills %s != %s", sum, held)
	expired := ev.wait(t, "stop leg closed", 20*time.Second, func(e broker.Event) bool {
		return e.Kind == broker.EventOrder && e.Order.ClientID == stop.ClientID && !e.Order.Status.Open()
	})
	t.Logf("take-profit filled at %s; stop leg %s", ev.fills(target.ClientID)[0].Price, expired.Order.Status)
	assert.Contains(t, []domain.OrderStatus{domain.OrderExpired, domain.OrderCanceled}, expired.Order.Status)
	assert.NotContains(t, openOrderIDs(t, ctx, tr), stop.ClientID)
}
