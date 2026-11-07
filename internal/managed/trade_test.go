package managed

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/marketview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type screenSource struct{ events chan exchange.MarketEvent }

func (s screenSource) Name() string { return "sim" }
func (s screenSource) Depth(context.Context, string, int) (exchange.DepthSnapshot, error) {
	return exchange.DepthSnapshot{LastUpdateID: 1, Bids: []exchange.Level{{Price: d("99"), Qty: d("2")}}, Asks: []exchange.Level{{Price: d("101"), Qty: d("1")}}}, nil
}
func (s screenSource) StreamMarket(context.Context, []string, []domain.Timeframe) (<-chan exchange.MarketEvent, error) {
	return s.events, nil
}
func (s screenSource) Klines(_ context.Context, req exchange.KlineRequest) ([]domain.Candle, error) {
	at := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	return []domain.Candle{{Symbol: req.Symbol, Timeframe: req.Timeframe, OpenTime: at, Open: d("100"), High: d("102"), Low: d("98"), Close: d("101")}}, nil
}

func screen(t *testing.T, mode string) (*HTTP, *Runtime) {
	t.Helper()
	sp := spec()
	sp.Mode = mode
	sp.Instances[0].Strategies = append(sp.Instances[0].Strategies, BindingSpec{ID: "manual", Name: ManualStrategy, Symbols: []string{"ETHUSDT"}, Risk: generous()})
	repo, err := NewMemory(sp)
	require.NoError(t, err)
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	src := screenSource{events: make(chan exchange.MarketEvent, 8)}
	hub := &marketview.Hub{Source: src, Symbols: []string{"ETHUSDT"}, Timeframes: []domain.Timeframe{"15m"}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = hub.Run(ctx) }()
	src.events <- exchange.MarketEvent{Kind: exchange.MarketConnected}
	src.events <- exchange.MarketEvent{Kind: exchange.MarketDepth, Depth: exchange.DepthUpdate{Symbol: "ETHUSDT", FirstID: 2, FinalID: 2}}
	src.events <- exchange.MarketEvent{Kind: exchange.MarketTicker, Ticker: exchange.Ticker24{Symbol: "ETHUSDT", Last: d("100.5"), ChangePct: d("1.5")}}
	h := NewHTTP(map[string]*Runtime{"test": rt}, "pass", nil)
	h.Email = "me@example.com"
	h.Markets = func(string) *marketview.Hub { return hub }
	return h, rt
}

func login(t *testing.T, h *HTTP) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest("POST", "http://localhost/login", strings.NewReader(url.Values{"email": {" Me@Example.com "}, "password": {"pass"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code)
	require.Equal(t, "/trade", w.Header().Get("Location"))
	c := w.Result().Cookies()[0]
	require.True(t, c.HttpOnly)
	require.Equal(t, http.SameSiteStrictMode, c.SameSite)
	return c
}

func TestLoginNeedsEmailAndPasswordThenLocksOut(t *testing.T) {
	h, _ := screen(t, "paper")
	get := httptest.NewRequest("GET", "http://localhost/trade", nil)
	get.SetBasicAuth("u", "pass")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, get)
	assert.Equal(t, http.StatusSeeOther, w.Code, "basic auth is off once an email is configured")
	assert.Equal(t, "/login", w.Header().Get("Location"))

	c := login(t, h)
	for _, form := range []string{"email=other%40example.com&password=pass", "email=me%40example.com&password=wrong", "password=pass"} {
		bad := httptest.NewRequest("POST", "http://localhost/login", strings.NewReader(form))
		bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, bad)
		assert.Contains(t, w.Header().Get("Location"), "error=", form)
	}
	assert.False(t, h.sessions.locked(time.Now()), "three failures stay below the limit")
	for range loginFailures {
		bad := httptest.NewRequest("POST", "http://localhost/login", strings.NewReader("email=me%40example.com&password=wrong"))
		bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.ServeHTTP(httptest.NewRecorder(), bad)
	}
	assert.True(t, h.sessions.locked(time.Now()), "repeated failures lock the login")

	get = httptest.NewRequest("GET", "http://localhost/trade", nil)
	get.AddCookie(c)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, get)
	require.Equal(t, http.StatusOK, w.Code, "an existing session survives the lockout")
	body := w.Body.String()
	for _, want := range []string{"Order manual", "a/manual", "data-chart=\"trade\"", "PAPER", "Order book", "Keluar"} {
		assert.Contains(t, body, want)
	}
}

func TestLiveWalletActionsNeedThePasswordAgain(t *testing.T) {
	h, rt := screen(t, "live")
	c := login(t, h)
	post := func(password string) *httptest.ResponseRecorder {
		form := url.Values{"csrf": {h.token}, "id": {nonce()}, "operation": {"pause"}, "scope": {"a"}, "password": {password}}
		req := httptest.NewRequest("POST", "http://localhost/control?wallet=test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(c)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	for _, wrong := range []string{"", "nope"} {
		w := post(wrong)
		assert.Contains(t, w.Header().Get("Location"), "password")
		s, _ := rt.Snapshot(t.Context())
		assert.NotEqual(t, "paused", s.Instances["a"].Status)
	}
	w := post("pass")
	assert.NotContains(t, w.Header().Get("Location"), "error=")
	s, _ := rt.Snapshot(t.Context())
	assert.Equal(t, "paused", s.Instances["a"].Status)
}

func TestCandlesAndStreamServeFromTheHub(t *testing.T) {
	h, _ := screen(t, "paper")
	c := login(t, h)
	req := httptest.NewRequest("GET", "http://localhost/api/candles?wallet=test&symbol=ETHUSDT&tf=1h", nil)
	req.AddCookie(c)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Candles []map[string]any `json:"candles"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out.Candles, 1)
	assert.Equal(t, 101.0, out.Candles[0]["close"], "numbers, not strings, for the chart")

	srv := httptest.NewServer(h)
	defer srv.Close()
	sreq, _ := http.NewRequestWithContext(t.Context(), "GET", srv.URL+"/stream?wallet=test&symbol=ETHUSDT&tf=15m", nil)
	sreq.AddCookie(c)
	resp, err := http.DefaultClient.Do(sreq)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	r := bufio.NewReader(resp.Body)
	var payload marketPayload
	for payload.Ticker == nil || !payload.Synced {
		line, err := r.ReadString('\n')
		require.NoError(t, err)
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			require.NoError(t, json.Unmarshal([]byte(data), &payload))
		}
	}
	assert.Equal(t, [][2]string{{"99", "2"}}, payload.Bids)
	assert.Equal(t, "100.5", payload.Ticker.Last)
}
