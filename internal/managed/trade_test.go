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
	h := NewHTTP(map[string]*Runtime{"test": rt}, "", nil)
	h.Owners = &MemOwners{}
	h.Markets = func(string) *marketview.Hub { return hub }
	hash, err := hashPassword(testPassword)
	require.NoError(t, err)
	_, err = h.Owners.Register(t.Context(), Owner{Email: "me@example.com", Hash: hash})
	require.NoError(t, err)
	return h, rt
}

const testPassword = "secret-pass"

// Full-strength hashing would make every login in the tests slow.
func init() { passwordIterations = 1_000 }

func form(t *testing.T, h *HTTP, path string, values url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func get(t *testing.T, h *HTTP, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "http://localhost"+path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestRegisterOnceWithTheSetupCode(t *testing.T) {
	h, _ := screen(t, "paper")
	h.Owners = &MemOwners{}
	code, err := h.Setup(t.Context())
	require.NoError(t, err)
	require.Len(t, code, 16)

	w := get(t, h, "/login", nil)
	assert.Equal(t, "/register", w.Header().Get("Location"), "without an account the login sends you to register")
	assert.Equal(t, "/login", get(t, h, "/trade", nil).Header().Get("Location"))
	assert.Contains(t, get(t, h, "/register", nil).Body.String(), "Kode setup")

	ok := url.Values{"setup_code": {code}, "email": {" Me@Example.com "}, "password": {"my-own-pass"}, "password_confirm": {"my-own-pass"}}
	for name, bad := range map[string]url.Values{
		"wrong code": {"setup_code": {"0000000000000000"}, "email": ok["email"], "password": ok["password"], "password_confirm": ok["password"]},
		"bad email":  {"setup_code": {code}, "email": {"not-an-email"}, "password": ok["password"], "password_confirm": ok["password"]},
		"short":      {"setup_code": {code}, "email": ok["email"], "password": {"short"}, "password_confirm": {"short"}},
		"no match":   {"setup_code": {code}, "email": ok["email"], "password": ok["password"], "password_confirm": {"other-pass"}},
	} {
		w = form(t, h, "/register", bad, nil)
		assert.Contains(t, w.Header().Get("Location"), "/register?error=", name)
	}
	_, exists, _ := h.Owners.Owner(t.Context())
	require.False(t, exists)

	w = form(t, h, "/register", ok, nil)
	require.Equal(t, "/trade", w.Header().Get("Location"))
	c := w.Result().Cookies()[0]
	assert.Equal(t, http.StatusOK, get(t, h, "/trade", c).Code, "registering logs you in")
	o, exists, _ := h.Owners.Owner(t.Context())
	require.True(t, exists)
	assert.Equal(t, "me@example.com", o.Email)
	assert.NotContains(t, o.Hash, "my-own-pass")

	assert.Equal(t, "/login", get(t, h, "/register", nil).Header().Get("Location"), "registration closes after the first account")
	w = form(t, h, "/register", url.Values{"setup_code": {code}, "email": {"x@example.com"}, "password": {"another-pass"}, "password_confirm": {"another-pass"}}, nil)
	assert.Equal(t, "/login", w.Header().Get("Location"))
	o, _, _ = h.Owners.Owner(t.Context())
	assert.Equal(t, "me@example.com", o.Email, "a second registration cannot replace the owner")
	code, _ = h.Setup(t.Context())
	assert.Empty(t, code)
}

func login(t *testing.T, h *HTTP) *http.Cookie {
	t.Helper()
	w := form(t, h, "/login", url.Values{"email": {" Me@Example.com "}, "password": {testPassword}}, nil)
	require.Equal(t, http.StatusSeeOther, w.Code)
	require.Equal(t, "/trade", w.Header().Get("Location"))
	c := w.Result().Cookies()[0]
	require.True(t, c.HttpOnly)
	require.Equal(t, http.SameSiteStrictMode, c.SameSite)
	return c
}

func TestLoginNeedsEmailAndPasswordThenLocksOut(t *testing.T) {
	h, _ := screen(t, "paper")
	req := httptest.NewRequest("GET", "http://localhost/trade", nil)
	req.SetBasicAuth("u", testPassword)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusSeeOther, w.Code, "basic auth is off with an account store")
	assert.Equal(t, "/login", w.Header().Get("Location"))
	assert.Equal(t, "/login", get(t, h, "/register", nil).Header().Get("Location"))

	c := login(t, h)
	for _, bad := range []url.Values{
		{"email": {"other@example.com"}, "password": {testPassword}},
		{"email": {"me@example.com"}, "password": {"wrong"}},
		{"password": {testPassword}},
	} {
		w = form(t, h, "/login", bad, nil)
		assert.Contains(t, w.Header().Get("Location"), "error=", bad)
	}
	assert.False(t, h.sessions.locked(time.Now()), "three failures stay below the limit")
	for range loginFailures {
		form(t, h, "/login", url.Values{"email": {"me@example.com"}, "password": {"wrong"}}, nil)
	}
	assert.True(t, h.sessions.locked(time.Now()), "repeated failures lock the login")

	w = get(t, h, "/trade", c)
	require.Equal(t, http.StatusOK, w.Code, "an existing session survives the lockout")
	body := w.Body.String()
	for _, want := range []string{"Order manual", "a/manual", "data-chart=\"trade\"", "PAPER", "Order book", "Keluar", "/account"} {
		assert.Contains(t, body, want)
	}
}

func TestAccountChangesPasswordAndEndsOtherSessions(t *testing.T) {
	h, _ := screen(t, "paper")
	other := login(t, h)
	c := login(t, h)
	assert.Contains(t, get(t, h, "/account", c).Body.String(), "me@example.com")

	change := func(current, next, confirm string, csrf string) *httptest.ResponseRecorder {
		return form(t, h, "/account", url.Values{"csrf": {csrf}, "current_password": {current}, "password": {next}, "password_confirm": {confirm}}, c)
	}
	assert.Equal(t, http.StatusForbidden, change(testPassword, "brand-new-pass", "brand-new-pass", "bad").Code)
	assert.Contains(t, change("wrong", "brand-new-pass", "brand-new-pass", h.token).Header().Get("Location"), "error=")
	assert.Contains(t, change(testPassword, "brand-new-pass", "different", h.token).Header().Get("Location"), "error=")

	w := change(testPassword, "brand-new-pass", "brand-new-pass", h.token)
	require.Equal(t, "/account?done=1", w.Header().Get("Location"))
	fresh := w.Result().Cookies()[0]
	assert.Equal(t, http.StatusOK, get(t, h, "/account", fresh).Code)
	assert.Equal(t, "/login", get(t, h, "/trade", other).Header().Get("Location"), "other sessions are logged out")

	assert.Contains(t, form(t, h, "/login", url.Values{"email": {"me@example.com"}, "password": {testPassword}}, nil).Header().Get("Location"), "error=")
	assert.Equal(t, "/trade", form(t, h, "/login", url.Values{"email": {"me@example.com"}, "password": {"brand-new-pass"}}, nil).Header().Get("Location"))
}

func TestLiveWalletActionsNeedThePasswordAgain(t *testing.T) {
	h, rt := screen(t, "live")
	c := login(t, h)
	post := func(password string) *httptest.ResponseRecorder {
		return form(t, h, "/control?wallet=test", url.Values{"csrf": {h.token}, "id": {nonce()}, "operation": {"pause"}, "scope": {"a"}, "password": {password}}, c)
	}
	for _, wrong := range []string{"", "nope"} {
		w := post(wrong)
		assert.Contains(t, w.Header().Get("Location"), "password")
		s, _ := rt.Snapshot(t.Context())
		assert.NotEqual(t, "paused", s.Instances["a"].Status)
	}
	w := post(testPassword)
	assert.NotContains(t, w.Header().Get("Location"), "error=")
	s, _ := rt.Snapshot(t.Context())
	assert.Equal(t, "paused", s.Instances["a"].Status)
}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("correct horse")
	require.NoError(t, err)
	assert.True(t, checkPassword(hash, "correct horse"))
	assert.False(t, checkPassword(hash, "correct horsE"))
	assert.False(t, checkPassword("", "correct horse"))
	assert.False(t, checkPassword("pbkdf2-sha256$x$a$b", "correct horse"))
	other, _ := hashPassword("correct horse")
	assert.NotEqual(t, hash, other, "every hash has its own salt")
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
