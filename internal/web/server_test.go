package web

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type liveStub struct {
	Live
	paused, killed bool
	kills, resets  int
}

func (*liveStub) Mode() string                   { return "paper" }
func (*liveStub) Accounts() []*portfolio.Account { return nil }
func (l *liveStub) Paused() bool                 { return l.paused }
func (l *liveStub) Halted() (bool, string)       { return l.killed, "test" }
func (l *liveStub) SetPaused(v bool)             { l.paused = v }
func (l *liveStub) Kill(context.Context, string) { l.killed = true; l.kills++ }
func (l *liveStub) ResetKillSwitch()             { l.killed = false; l.resets++ }

func TestDashboardAuthSecurityHeadersAndNoSession(t *testing.T) {
	h := New(nil, nil, "test-password", slog.Default()).Handler()
	for _, tc := range []struct {
		password string
		status   int
	}{{"", 401}, {"wrong", 401}, {"test-password", 200}} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.password != "" {
			r.SetBasicAuth("bot", tc.password)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assert.Equal(t, tc.status, w.Code)
		if tc.status == 200 {
			assert.Contains(t, w.Body.String(), "No trading session")
			assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
			assert.Contains(t, w.Header().Get("Content-Security-Policy"), "script-src 'self'")
		} else {
			assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Basic")
		}
	}
	h = New(nil, nil, "", slog.Default()).Handler()
	for _, path := range []string{"/", "/partials/status", "/partials/positions", "/partials/orders", "/static/app.js"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, 200, w.Code, path)
	}
}

func TestDashboardActionsRequireHXAndRespectKillSwitch(t *testing.T) {
	l := &liveStub{}
	h := New(nil, l, "", slog.Default()).Handler()
	do := func(action, hx string) int {
		r := httptest.NewRequest(http.MethodPost, "/actions/"+action, nil)
		r.Header.Set("HX-Request", hx)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	assert.Equal(t, 403, do("kill", ""))
	assert.Zero(t, l.kills)
	assert.Equal(t, 200, do("pause", "true"))
	assert.True(t, l.paused)
	assert.Equal(t, 200, do("resume", "true"))
	assert.False(t, l.paused)
	assert.Equal(t, 200, do("kill", "true"))
	l.paused = true
	assert.Equal(t, 200, do("resume", "true"))
	assert.True(t, l.paused, "cannot resume while killed")
	assert.Equal(t, 200, do("reset", "true"))
	assert.False(t, l.killed)
	assert.Equal(t, 1, l.resets)
	assert.Equal(t, 404, do("unknown", "true"))
}

func TestParseSeriesRejectsInvalidInput(t *testing.T) {
	for _, v := range []string{"", "binance:futures:BTCUSDT", "binance:margin:BTCUSDT:1h", "binance:futures:BTCUSDT:nope"} {
		_, err := parseSeries(v)
		assert.Error(t, err, v)
	}
	k, err := parseSeries("bybit:futures:BTCUSDT:1h")
	assert.NoError(t, err)
	assert.True(t, strings.Contains(chartHref(k, 12), "run=12"))
}

func TestCSPAllowsOnlyPinnedChartAttributionStyle(t *testing.T) {
	js, err := staticFS.ReadFile("static/lightweight-charts.js")
	require.NoError(t, err)
	match := regexp.MustCompile(`this\.qv\.innerText=("(?:[^"\\]|\\.)*")`).FindSubmatch(js)
	require.Len(t, match, 2, "review CSP when upgrading the vendored chart library")
	var css string
	require.NoError(t, json.Unmarshal(match[1], &css))
	hash := sha256.Sum256([]byte(css))
	w := httptest.NewRecorder()
	New(nil, nil, "", slog.Default()).Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "'sha256-"+base64.StdEncoding.EncodeToString(hash[:])+"'")
	assert.NotContains(t, csp, "unsafe-inline")
	assert.NotContains(t, csp, "unsafe-eval")
}
