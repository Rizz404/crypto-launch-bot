package binance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

// fixtureServer serves testdata files by request path.
func fixtureServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Errorf("fixture %s: %v", name, err)
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testClient(srv *httptest.Server) *Client {
	return New(Options{Endpoints: &Endpoints{SpotREST: srv.URL, FuturesREST: srv.URL}})
}

func bySymbol(ins []domain.Instrument) map[string]domain.Instrument {
	m := map[string]domain.Instrument{}
	for _, in := range ins {
		m[in.Symbol] = in
	}
	return m
}

func TestInstrumentsSpot(t *testing.T) {
	srv := fixtureServer(t, map[string]string{"/api/v3/exchangeInfo": "spot_exchangeinfo.json"})
	ins, err := testClient(srv).Instruments(context.Background(), domain.Spot)
	require.NoError(t, err)

	m := bySymbol(ins)
	require.Len(t, m, 3)
	btc := m["BTCUSDT"]
	assert.Equal(t, "binance", btc.Exchange)
	assert.Equal(t, domain.Spot, btc.Market)
	assert.Equal(t, "BTC", btc.Base)
	assert.Equal(t, "USDT", btc.Quote)
	assert.True(t, btc.Tradable)
	assert.Equal(t, "0.01", btc.TickSize.String())
	assert.Equal(t, "0.00001", btc.StepSize.String())
	assert.Equal(t, "0.00001", btc.MinQty.String())
	assert.Equal(t, "5", btc.MinNotional.String())

	assert.False(t, m["OLDUSDT"].Tradable)
	assert.Equal(t, "BREAK", m["OLDUSDT"].Status)
}

func TestInstrumentsFuturesOnlyPerpetuals(t *testing.T) {
	srv := fixtureServer(t, map[string]string{"/fapi/v1/exchangeInfo": "futures_exchangeinfo.json"})
	ins, err := testClient(srv).Instruments(context.Background(), domain.Futures)
	require.NoError(t, err)

	m := bySymbol(ins)
	assert.Contains(t, m, "BTCUSDT")
	assert.Contains(t, m, "BTCUSDC", "USDC perpetuals are kept; the universe filters by quote")
	assert.NotContains(t, m, "BTCUSDT_261225", "quarterly contract")
	assert.NotContains(t, m, "XAUUSDT", "TradFi perpetual")

	btc := m["BTCUSDT"]
	assert.Equal(t, "0.1", btc.TickSize.String())
	assert.Equal(t, "0.001", btc.StepSize.String())
	assert.Equal(t, "50", btc.MinNotional.String())
}

func TestTickers(t *testing.T) {
	srv := fixtureServer(t, map[string]string{
		"/api/v3/ticker/24hr":        "spot_ticker24h.json",
		"/fapi/v1/ticker/24hr":       "futures_ticker24h.json",
		"/fapi/v1/ticker/bookTicker": "futures_bookticker.json",
	})
	c := testClient(srv)
	for _, m := range []domain.Market{domain.Spot, domain.Futures} {
		ts, err := c.Tickers(context.Background(), m)
		require.NoError(t, err, m)
		require.NotEmpty(t, ts, m)
		for _, tk := range ts {
			if tk.Symbol != "BTCUSDT" {
				continue
			}
			assert.True(t, tk.QuoteVolume.IsPositive(), m)
			bps, ok := tk.SpreadBps()
			assert.True(t, ok, "%s: bid/ask present", m)
			assert.Less(t, bps, 5.0, m)
		}
	}
}

func TestKlines(t *testing.T) {
	var gotQuery atomic.Value
	srv := fixtureServer(t, map[string]string{"/fapi/v1/klines": "futures_klines.json"})
	srv.Config.Handler = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery.Store(r.URL.RawQuery)
			next.ServeHTTP(w, r)
		})
	}(srv.Config.Handler)

	start := time.UnixMilli(1792202400000)
	ks, err := testClient(srv).Klines(context.Background(), exchange.KlineRequest{
		Market: domain.Futures, Symbol: "BTCUSDT", Timeframe: "1h", Start: start,
	})
	require.NoError(t, err)
	require.Len(t, ks, 3)

	q := gotQuery.Load().(string)
	assert.Contains(t, q, "interval=1h")
	assert.Contains(t, q, "limit=1000")
	assert.Contains(t, q, "startTime=1792202400000")

	k := ks[0]
	assert.Equal(t, start.UTC(), k.OpenTime)
	assert.Equal(t, domain.Timeframe("1h"), k.Timeframe)
	assert.Equal(t, "81511.1", k.Open.String())
	assert.Equal(t, "81639.6", k.High.String())
	assert.Equal(t, "81373.3", k.Low.String())
	assert.Equal(t, "81444", k.Close.String())
	assert.Equal(t, "1280.695", k.Volume.String())
	assert.Equal(t, int64(47482), k.Trades)
	assert.True(t, k.Closed, "bar from the past is closed")
	assert.Equal(t, ks[0].OpenTime.Add(time.Hour), ks[1].OpenTime)
}

func TestAPIErrorAndRateLimit(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.Contains(r.URL.RawQuery, "NOPE") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":-1121,"msg":"Invalid symbol."}`))
			return
		}
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":-1003,"msg":"Too many requests"}`))
	}))
	defer srv.Close()
	c := testClient(srv)
	ctx := context.Background()

	_, err := c.Klines(ctx, exchange.KlineRequest{Market: domain.Spot, Symbol: "NOPE", Timeframe: "1m", Start: time.Now()})
	require.Error(t, err)
	assert.True(t, IsInvalidSymbol(err))

	_, err = c.Klines(ctx, exchange.KlineRequest{Market: domain.Spot, Symbol: "BTCUSDT", Timeframe: "1m", Start: time.Now()})
	var apiErr *exchange.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.RateLimited())
	assert.Equal(t, time.Second, apiErr.RetryAfter)

	// The limiter is now paused: the next request waits ~1s before it is sent.
	start := time.Now()
	_, _ = c.Klines(ctx, exchange.KlineRequest{Market: domain.Spot, Symbol: "BTCUSDT", Timeframe: "1m", Start: time.Now()})
	assert.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond)
	assert.Equal(t, int32(3), calls.Load())
}

func TestTestnetName(t *testing.T) {
	assert.Equal(t, "binance", New(Options{}).Name())
	c := New(Options{Testnet: true})
	assert.Equal(t, "binance-testnet", c.Name())
	assert.Equal(t, Testnet.FuturesREST, c.ep.FuturesREST)
}

func TestStreamURLsChunking(t *testing.T) {
	c := New(Options{})
	subs := make([]exchange.KlineSub, 250)
	for i := range subs {
		subs[i] = exchange.KlineSub{Symbol: "BTCUSDT", Timeframe: "1m"}
	}
	urls := c.StreamURLs(domain.Futures, subs)
	require.Len(t, urls, 2)
	assert.True(t, strings.HasPrefix(urls[0], "wss://fstream.binance.com/market/stream?streams=btcusdt@kline_1m/"))
	assert.Equal(t, 200, strings.Count(urls[0], "@kline_"))
	assert.Equal(t, 50, strings.Count(urls[1], "@kline_"))
}

func TestStreamKlinesReconnects(t *testing.T) {
	msg, err := os.ReadFile("testdata/futures_ws_kline.json")
	require.NoError(t, err)

	var conns atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/stream", r.URL.Path)
		assert.Equal(t, "btcusdt@kline_1m", r.URL.Query().Get("streams"))
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conns.Add(1)
		// Send one message, then drop the connection to force a reconnect.
		_ = conn.Write(r.Context(), websocket.MessageText, msg)
		_ = conn.Close(websocket.StatusGoingAway, "bye")
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := New(Options{Endpoints: &Endpoints{FuturesWS: wsURL}})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := c.StreamKlines(ctx, domain.Futures, []exchange.KlineSub{{Symbol: "BTCUSDT", Timeframe: "1m"}})
	require.NoError(t, err)

	var connected, candles int
	for ev := range ch {
		switch ev.Kind {
		case exchange.EventConnected:
			connected++
		case exchange.EventCandle:
			candles++
			k := ev.Candle
			assert.Equal(t, "BTCUSDT", k.Symbol)
			assert.Equal(t, domain.Futures, k.Market)
			assert.Equal(t, domain.Timeframe("1m"), k.Timeframe)
			// Each field checked against its case-twin in the payload.
			assert.Equal(t, time.UnixMilli(1792212660000).UTC(), k.OpenTime, "t, not T")
			assert.Equal(t, "81513.6", k.Open.String())
			assert.Equal(t, "81513.7", k.High.String())
			assert.Equal(t, "81499.4", k.Low.String(), "l, not L")
			assert.Equal(t, "81499.4", k.Close.String())
			assert.Equal(t, "9.537", k.Volume.String(), "v, not V")
			assert.Equal(t, "777302.6677", k.QuoteVolume.String(), "q, not Q")
			assert.Equal(t, int64(465), k.Trades)
			assert.False(t, k.Closed)
		}
		if connected >= 2 && candles >= 2 {
			cancel()
		}
	}
	require.NotErrorIs(t, ctx.Err(), context.DeadlineExceeded, "stream test timed out")
	assert.GreaterOrEqual(t, candles, 2)
	assert.GreaterOrEqual(t, connected, 2, "reconnected after the server closed")
	assert.GreaterOrEqual(t, int(conns.Load()), 2)
}
