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

// totpSecret is the RFC 6238 test key "12345678901234567890" in base32.
const totpSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestTOTPMatchesRFC6238Vectors(t *testing.T) {
	for unix, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := totpCode(totpSecret, time.Unix(unix, 0))
		require.NoError(t, err)
		assert.Equal(t, want, got, "t=%d", unix)
	}
	now := time.Unix(1234567890, 0)
	assert.True(t, checkTOTP(totpSecret, "005924", now.Add(25*time.Second)), "one step of drift is accepted")
	assert.False(t, checkTOTP(totpSecret, "005924", now.Add(95*time.Second)))
	assert.False(t, checkTOTP(totpSecret, "", now))
	assert.False(t, ValidTOTPSecret("short"))
}

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
	h.TOTP = totpSecret
	h.Markets = func(string) *marketview.Hub { return hub }
	return h, rt
}

func login(t *testing.T, h *HTTP) *http.Cookie {
	t.Helper()
	c, _ := loginCode(t, h)
	return c
}

func loginCode(t *testing.T, h *HTTP) (*http.Cookie, string) {
	t.Helper()
	code, _ := totpCode(totpSecret, time.Now())
	req := httptest.NewRequest("POST", "http://localhost/login", strings.NewReader(url.Values{"password": {"pass"}, "code": {code}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code)
	require.Equal(t, "/trade", w.Header().Get("Location"))
	c := w.Result().Cookies()[0]
	require.True(t, c.HttpOnly)
	require.Equal(t, http.SameSiteStrictMode, c.SameSite)
	return c, code
}

func TestLoginNeedsPasswordAndFreshCodeThenLocksOut(t *testing.T) {
	h, _ := screen(t, "paper")
	get := httptest.NewRequest("GET", "http://localhost/trade", nil)
	get.SetBasicAuth("u", "pass")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, get)
	assert.Equal(t, http.StatusSeeOther, w.Code, "basic auth is off once TOTP is configured")
	assert.Equal(t, "/login", w.Header().Get("Location"))

	c, code := loginCode(t, h)
	replay := httptest.NewRequest("POST", "http://localhost/login", strings.NewReader(url.Values{"password": {"pass"}, "code": {code}}.Encode()))
	replay.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, replay)
	assert.Contains(t, w.Header().Get("Location"), "error=", "a used code cannot log in twice")

	for range loginFailures {
		bad := httptest.NewRequest("POST", "http://localhost/login", strings.NewReader("password=wrong&code=000000"))
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
	for _, want := range []string{"Order manual", "a/manual", "data-chart=\"trade\"", "PAPER", "Order book"} {
		assert.Contains(t, body, want)
	}
}

func TestLiveWalletActionsNeedAFreshCode(t *testing.T) {
	h, rt := screen(t, "live")
	c := login(t, h)
	post := func(code string) *httptest.ResponseRecorder {
		form := url.Values{"csrf": {h.token}, "id": {nonce()}, "operation": {"pause"}, "scope": {"a"}, "totp": {code}}
		req := httptest.NewRequest("POST", "http://localhost/control?wallet=test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(c)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	w := post("")
	assert.Contains(t, w.Header().Get("Location"), "TOTP")
	s, _ := rt.Snapshot(t.Context())
	assert.NotEqual(t, "paused", s.Instances["a"].Status)
	// The login used the current code; wait for the next window rather than replay it.
	code, _ := totpCode(totpSecret, time.Now().Add(30*time.Second))
	w = post(code)
	assert.NotContains(t, w.Header().Get("Location"), "error=")
	s, _ = rt.Snapshot(t.Context())
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
