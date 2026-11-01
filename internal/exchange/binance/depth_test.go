package binance

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/exchange"
)

func TestDepthSnapshotAndMarketMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/depth", r.URL.Path)
		require.Equal(t, "1000", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"lastUpdateId":160,"bids":[["0.0024","10"]],"asks":[["0.0026","100"]]}`))
	}))
	defer srv.Close()
	c := testClient(srv)
	snap, err := c.Depth(t.Context(), "BNBBTC", 1000)
	require.NoError(t, err)
	assert.EqualValues(t, 160, snap.LastUpdateID)
	assert.Equal(t, "0.0024", snap.Bids[0].Price.String())

	ev, ok, err := c.parseMarketMsg([]byte(`{"stream":"bnbbtc@depth@100ms","data":{"e":"depthUpdate","E":1,"s":"BNBBTC","U":157,"u":160,"b":[["0.0024","10"]],"a":[["0.0026","0"]]}}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, exchange.MarketDepth, ev.Kind)
	assert.EqualValues(t, 157, ev.Depth.FirstID)
	assert.EqualValues(t, 160, ev.Depth.FinalID)
	assert.True(t, ev.Depth.Asks[0].Qty.IsZero())

	ev, ok, err = c.parseMarketMsg([]byte(`{"stream":"bnbbtc@aggTrade","data":{"e":"aggTrade","E":2,"s":"BNBBTC","a":12345,"p":"0.001","q":"100","f":100,"l":105,"T":1672515782136,"m":true,"M":true}}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "0.001", ev.Trade.Price.String())
	assert.Equal(t, "100", ev.Trade.Qty.String(), "q must not be overwritten by Q")
	assert.True(t, ev.Trade.BuyerMaker)

	ev, ok, err = c.parseMarketMsg([]byte(`{"stream":"bnbbtc@ticker","data":{"e":"24hrTicker","E":3,"s":"BNBBTC","p":"0.0015","P":"250.00","c":"0.0025","Q":"10","h":"0.0025","l":"0.0010","v":"10000","q":"18","o":"0.0010"}}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "250", ev.Ticker.ChangePct.String())
	assert.Equal(t, "0.0015", ev.Ticker.Change.String())
	assert.Equal(t, "18", ev.Ticker.QuoteVol.String())
	assert.Equal(t, "0.001", ev.Ticker.Low.String())
}
