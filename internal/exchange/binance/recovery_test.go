package binance

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSpotRecoveryUsesTradeIdentityAndActualCommissions(t *testing.T) {
	response := `[{"symbol":"ETHUSDT","id":101,"orderId":42,"price":"100","qty":"1","commission":"0.001","commissionAsset":"ETH","time":1,"isBuyer":true},{"symbol":"ETHUSDT","id":102,"orderId":42,"price":"102","qty":"1","commission":"0.0002","commissionAsset":"BNB","time":2,"isBuyer":true}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/myTrades", r.URL.Path)
		require.Equal(t, "42", r.URL.Query().Get("orderId"))
		require.NotEmpty(t, r.URL.Query().Get("signature"))
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()
	tr := NewTrader(testClient(srv), "fixture", "fixture", []domain.Market{domain.Spot})
	known := domain.Order{ClientID: "entry", Exchange: "binance", Market: domain.Spot, Symbol: "ETHUSDT", Side: domain.Buy, Quantity: decimal.NewFromInt(2)}
	final := known
	final.ExchangeID = "42"
	final.FilledQty = known.Quantity
	fills, err := tr.spotExecutions(t.Context(), known, final)
	require.NoError(t, err)
	require.Len(t, fills, 2)
	require.Equal(t, "101", fills[0].TradeID)
	require.Equal(t, "0.001", fills[0].BaseFee.String())
	require.Equal(t, "0.1", fills[0].Fee.String())
	require.Equal(t, "0.0002", fills[1].Commission.String())
	require.Equal(t, "BNB", fills[1].FeeAsset)
	known.FilledQty = decimal.NewFromInt(1)
	fills, err = tr.spotExecutions(t.Context(), known, final)
	require.NoError(t, err)
	require.Len(t, fills, 1)
	require.Equal(t, "102", fills[0].TradeID)
	known.FilledQty = decimal.RequireFromString("0.5")
	_, err = tr.spotExecutions(t.Context(), known, final)
	require.ErrorContains(t, err, "splits")
	response = `[{"symbol":"ETHUSDT","id":101,"orderId":999,"price":"100","qty":"2","commission":"0","commissionAsset":"USDT","isBuyer":true}]`
	_, err = tr.spotExecutions(t.Context(), known, final)
	require.ErrorContains(t, err, "identity")
}
