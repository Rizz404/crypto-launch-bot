package exchange_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/exchange/binance"
	"github.com/rizz404/trading-bot/internal/exchange/bybit"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

type adapter struct {
	name, symbol string
	md           exchange.MarketData
	live         broker.Live
}

func adapters(client *http.Client) []adapter {
	b := binance.New(binance.Options{Testnet: true, HTTPClient: client})
	y := bybit.New(bybit.Options{Testnet: true, HTTPClient: client})
	return []adapter{
		{"binance-testnet", "BTCUSDT", b, binance.NewTrader(b, "test-key", "test-secret", []domain.Market{domain.Futures})},
		{"bybit-testnet", "BTCUSDT", y, bybit.NewTrader(y, "test-key", "test-secret", []domain.Market{domain.Futures})},
	}
}

// Synthetic protocol fixtures: they test the adapters' shared contract,
// not actual connectivity or acceptance by an exchange.
func TestMarketDataContract(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/fapi/v1/exchangeInfo":
			return reply(`{"symbols":[{"symbol":"BTCUSDT","baseAsset":"BTC","quoteAsset":"USDT","status":"TRADING","contractType":"PERPETUAL","filters":[{"filterType":"PRICE_FILTER","tickSize":"0.1"},{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001"}]}]}`), nil
		case "/v5/market/instruments-info":
			return reply(`{"retCode":0,"result":{"list":[{"symbol":"BTCUSDT","contractType":"LinearPerpetual","status":"Trading","baseCoin":"BTC","quoteCoin":"USDT","priceFilter":{"tickSize":"0.1"},"lotSizeFilter":{"qtyStep":"0.001","minOrderQty":"0.001","minNotionalValue":"5"}}]}}`), nil
		case "/fapi/v1/klines":
			return reply(`[[1767225600000,"100","110","90","105","2",1767229199999,"210",10,"1","105","0"],[1767229200000,"105","115","95","110","3",1767232799999,"330",12,"1","110","0"]]`), nil
		case "/v5/market/kline":
			return reply(`{"retCode":0,"result":{"list":[["1767229200000","105","115","95","110","3","330"],["1767225600000","100","110","90","105","2","210"]]}}`), nil
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			return reply(`{}`), nil
		}
	})}
	for _, a := range adapters(client) {
		t.Run(a.name, func(t *testing.T) {
			assert.Equal(t, a.name, a.md.Name())
			ins, err := a.md.Instruments(context.Background(), domain.Futures)
			require.NoError(t, err)
			require.Len(t, ins, 1)
			assert.Equal(t, "0.001", ins[0].StepSize.String())
			assert.Equal(t, "0.001", ins[0].MinQty.String())
			assert.Equal(t, "BTC", ins[0].Base)
			assert.True(t, ins[0].Tradable)
			start := time.UnixMilli(1767225600000).UTC()
			cs, err := a.md.Klines(context.Background(), exchange.KlineRequest{Market: domain.Futures, Symbol: a.symbol, Timeframe: "1h", Start: start, Limit: 2})
			require.NoError(t, err)
			require.Len(t, cs, 2)
			assert.Equal(t, start, cs[0].OpenTime)
			assert.Equal(t, start.Add(time.Hour), cs[1].OpenTime)
			assert.Equal(t, "2", cs[0].Volume.String(), "volume always in base units")
			assert.Equal(t, "210", cs[0].QuoteVolume.String())
			assert.Equal(t, a.name, cs[0].Exchange)
			assert.True(t, cs[0].Closed)
		})
	}
}

func TestLiveOrderContractAndSignatures(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		var raw []byte
		if r.Body != nil {
			var err error
			raw, err = io.ReadAll(r.Body)
			require.NoError(t, err)
		}
		if strings.Contains(r.URL.Host, "binance") {
			q := r.URL.Query()
			sig := q.Get("signature")
			q.Del("signature")
			assert.Equal(t, signature(q.Encode()), sig)
			assert.Equal(t, "test-key", r.Header.Get("X-MBX-APIKEY"))
			assert.Equal(t, "tbtest123", q.Get("newClientOrderId")+q.Get("origClientOrderId"))
			if r.Method == http.MethodPost {
				assert.Equal(t, "0.02", q.Get("quantity"))
				assert.Equal(t, "MARKET", q.Get("type"))
			}
			return reply(`{"orderId":123,"status":"NEW"}`), nil
		}
		var body map[string]any
		require.NoError(t, json.Unmarshal(raw, &body))
		if strings.Contains(r.URL.Host, "bybit") {
			assert.Equal(t, "test-key", r.Header.Get("X-BAPI-API-KEY"))
			payload := r.Header.Get("X-BAPI-TIMESTAMP") + "test-key" + r.Header.Get("X-BAPI-RECV-WINDOW") + string(raw)
			assert.Equal(t, signature(payload), r.Header.Get("X-BAPI-SIGN"))
			assert.Equal(t, "tbtest123", body["orderLinkId"])
			if strings.HasSuffix(r.URL.Path, "/create") {
				assert.Equal(t, "0.02", body["qty"])
				assert.Equal(t, "Market", body["orderType"])
			}
			return reply(`{"retCode":0,"result":{"orderId":"123"}}`), nil
		}
		t.Errorf("unexpected request %s", r.URL.Host)
		return reply(`{}`), nil
	})}
	for _, a := range adapters(client) {
		t.Run(a.name, func(t *testing.T) {
			o, err := a.live.PlaceOrder(context.Background(), domain.Order{ClientID: "tbtest123", Market: domain.Futures, Symbol: a.symbol,
				Type: domain.OrderMarket, Side: domain.Buy, Quantity: decimal.RequireFromString("0.02")})
			require.NoError(t, err)
			assert.Equal(t, domain.OrderNew, o.Status)
			assert.Equal(t, "123", o.ExchangeID)
			assert.Equal(t, a.name, o.Exchange)
			require.NoError(t, a.live.CancelOrder(context.Background(), o))
		})
	}
}

func signature(payload string) string {
	m := hmac.New(sha256.New, []byte("test-secret"))
	_, _ = m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}
