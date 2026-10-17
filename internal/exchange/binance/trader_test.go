package binance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
)

func TestSpotExecutionCaseSensitiveFieldsAndBaseFee(t *testing.T) {
	tr := NewTrader(New(Options{Testnet: true}), "key", "secret", []domain.Market{domain.Spot})
	var events []broker.Event
	tr.SetHandler(func(ev broker.Event) { events = append(events, ev) })
	tr.handleSpotMessage([]byte(`{"event":{"e":"executionReport","E":1767225600000,"s":"BTCUSDT","c":"tb123","S":"BUY","o":"MARKET","q":"2","Q":"200","p":"0","P":"0","x":"TRADE","X":"FILLED","i":123,"I":999,"l":"2","L":"100","n":"0.002","N":"BTC","T":1767225600001,"t":321,"z":"2","Z":"200","m":false,"M":true,"w":false,"W":1767225600000,"v":0,"V":"NONE"}}`))
	require.Len(t, events, 1)
	assert.Equal(t, "2", events[0].Fill.Quantity.String())
	assert.Equal(t, "100", events[0].Fill.Price.String())
	assert.Equal(t, "321", events[0].Fill.TradeID)
	assert.Equal(t, "0.2", events[0].Fill.Fee.String())
	assert.Equal(t, "2", events[0].Order.Quantity.String())
	assert.Equal(t, domain.OrderFilled, events[0].Order.Status)
	assert.Equal(t, int64(1767225600001), events[0].Fill.Time.UnixMilli())
}

// A timed-out order request may still execute; the trader asks the exchange
// before reporting a failure, so a later fill is not orphaned.
func TestAmbiguousPlaceFailureIsResolvedByQuery(t *testing.T) {
	for _, tc := range []struct {
		name     string
		query    func(w http.ResponseWriter)
		placed   bool
		wantOpen bool
	}{
		{name: "executed", placed: true, wantOpen: true, query: func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","orderId":42,"clientOrderId":"tbx1","price":"0","origQty":"0.001","executedQty":"0.001","cummulativeQuoteQty":"81","status":"FILLED","type":"MARKET","side":"BUY","stopPrice":"0","time":1,"updateTime":2}`))
		}},
		{name: "never reached the exchange", query: func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":-2013,"msg":"Order does not exist."}`))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPost:
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`{"code":-1007,"msg":"Timeout waiting for response from backend server. Send status unknown; execution status unknown."}`))
				case http.MethodGet:
					assert.Equal(t, "tbx1", r.URL.Query().Get("origClientOrderId"))
					tc.query(w)
				}
			}))
			defer srv.Close()
			tr := NewTrader(testClient(srv), "key", "secret", []domain.Market{domain.Spot})
			tr.SetHandler(func(broker.Event) {})
			o, err := tr.PlaceOrder(context.Background(), domain.Order{ClientID: "tbx1", Market: domain.Spot, Symbol: "BTCUSDT",
				Side: domain.Buy, Type: domain.OrderMarket, Quantity: decimal.RequireFromString("0.001")})
			if !tc.placed {
				require.Error(t, err)
				assert.Equal(t, -1007, apiCode(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "42", o.ExchangeID)
			assert.Equal(t, domain.OrderFilled, o.Status)
			tr.mu.Lock()
			_, open := tr.open["tbx1"]
			tr.mu.Unlock()
			assert.Equal(t, tc.wantOpen, open, "kept for mapping the fill event")
		})
	}
}
