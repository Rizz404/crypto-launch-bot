package fx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRatesFromCoinGeckoWithFrankfurterFallback(t *testing.T) {
	gecko, ecb := http.StatusOK, http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gecko":
			w.WriteHeader(gecko)
			_, _ = w.Write([]byte(`{"tether":{"usd":0.99905,"idr":18035.86}}`))
		case "/ecb":
			w.WriteHeader(ecb)
			_, _ = w.Write([]byte(`{"amount":1.0,"base":"USD","date":"2026-10-16","rates":{"IDR":18031}}`))
		}
	}))
	defer srv.Close()
	at := time.Date(2026, 10, 18, 17, 0, 0, 0, time.UTC)
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.CoinGecko, s.Frankfurter, s.Clock = srv.URL+"/gecko", srv.URL+"/ecb", func() time.Time { return at }
	ctx := context.Background()

	_, ok := s.Rates()
	assert.False(t, ok, "no rates before the first fetch")
	require.NoError(t, s.Refresh(ctx))
	r, ok := s.Rates()
	require.True(t, ok)
	assert.Equal(t, "0.99905", r.USD.String())
	assert.Equal(t, "18035.86", r.IDR.String())
	assert.Equal(t, "CoinGecko", r.Source)
	assert.Equal(t, at, r.At)

	gecko = http.StatusTooManyRequests
	require.NoError(t, s.Refresh(ctx))
	r, _ = s.Rates()
	assert.Equal(t, "1", r.USD.String(), "the ECB fallback assumes 1 USDT = 1 USD")
	assert.Equal(t, "18031", r.IDR.String())

	ecb = http.StatusBadGateway
	require.Error(t, s.Refresh(ctx))
	r, ok = s.Rates()
	assert.True(t, ok, "the last good rates stay")
	assert.Equal(t, "18031", r.IDR.String())
}
