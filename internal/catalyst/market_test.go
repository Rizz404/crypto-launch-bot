package catalyst

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange/binance"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func atrCandles(now time.Time) []domain.Candle {
	var out []domain.Candle
	for n := 15; n > 0; n-- {
		out = append(out, domain.Candle{Exchange: "binance", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "15m", OpenTime: now.Truncate(15 * time.Minute).Add(-time.Duration(n) * 15 * time.Minute), Open: decimal.NewFromInt(100), High: decimal.NewFromInt(101), Low: decimal.NewFromInt(99), Close: decimal.NewFromInt(100), Closed: true})
	}
	return out
}

func TestQuoteATRRequiresConsecutiveClosedFreshBars(t *testing.T) {
	now := fixtureNow.Add(5 * time.Minute)
	q, err := quoteATR(atrCandles(now), "ETHUSDT", now)
	require.NoError(t, err)
	require.Equal(t, "2", q.ATR.String())
	require.Equal(t, "100", q.EntryAnchor.String())
	for name, mutate := range map[string]func([]domain.Candle){
		"gap":          func(c []domain.Candle) { c[5].OpenTime = c[5].OpenTime.Add(15 * time.Minute) },
		"open":         func(c []domain.Candle) { c[14].Closed = false },
		"symbol":       func(c []domain.Candle) { c[14].Symbol = "DOGEUSDT" },
		"invalid OHLC": func(c []domain.Candle) { c[14].Low = decimal.NewFromInt(110) },
		"future":       func(c []domain.Candle) { c[14].OpenTime = now.Add(15 * time.Minute) },
	} {
		t.Run(name, func(t *testing.T) {
			candles := atrCandles(now)
			mutate(candles)
			_, err := quoteATR(candles, "ETHUSDT", now)
			require.Error(t, err)
		})
	}
	_, err = quoteATR(atrCandles(now), "ETHUSDT", now.Add(15*time.Minute))
	require.ErrorContains(t, err, "stale")
}

func TestMarketBuildsPublicSnapshotAndCachesSlowInputs(t *testing.T) {
	var books, klines, info int
	wrongSymbol := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/bapi/asset/v2/public/asset-service/product/get-products":
			_, _ = fmt.Fprint(w, `{"data":[{"b":"ETH","tags":[]}]}`)
		case "/api/v3/exchangeInfo":
			info++
			_, _ = fmt.Fprint(w, `{"symbols":[{"symbol":"ETHUSDT","status":"TRADING","baseAsset":"ETH","quoteAsset":"USDT","filters":[{"filterType":"PRICE_FILTER","tickSize":"0.01"},{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001"},{"filterType":"NOTIONAL","minNotional":"5"}]}]}`)
		case "/api/v3/ticker/24hr":
			_, _ = fmt.Fprint(w, `[{"symbol":"ETHUSDT","quoteVolume":"20000000"}]`)
		case "/api/v3/klines":
			klines++
			var rows [][]any
			for _, c := range atrCandles(time.Now().UTC()) {
				rows = append(rows, []any{c.OpenTime.UnixMilli(), "100", "101", "99", "100", "10", c.CloseTime().UnixMilli() - 1, "1000", 1})
			}
			_ = json.NewEncoder(w).Encode(rows)
		case "/api/v3/ticker/bookTicker":
			books++
			symbol := "ETHUSDT"
			if wrongSymbol {
				symbol = "DOGEUSDT"
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"symbol": symbol, "bidPrice": "100", "askPrice": "100.01", "bidQty": "1000", "askQty": "1000"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	m := NewMarket(binance.New(binance.Options{Endpoints: &binance.Endpoints{SpotREST: s.URL, Web: s.URL}}))
	p := testPolicy()
	for range 2 {
		q, err := m.Snapshot(t.Context(), p, "ETHUSDT")
		require.NoError(t, err)
		require.Equal(t, "2", q.ATR.String())
		require.Equal(t, "1000", q.BidQty.String())
		require.Equal(t, "20000000", q.QuoteVolume.String())
	}
	require.Equal(t, 1, info)
	require.Equal(t, 1, klines)
	require.Equal(t, 2, books, "the price/depth must be refreshed for each decision")
	wrongSymbol = true
	_, err := m.Snapshot(t.Context(), p, "ETHUSDT")
	require.ErrorContains(t, err, "invalid public spot quote")
	_, err = m.Snapshot(t.Context(), p, "NOTCONFIGUREDUSDT")
	require.ErrorContains(t, err, "outside policy")
}
