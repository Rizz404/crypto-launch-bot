package bybit

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSetLeveragePropagatesNetworkFailure(t *testing.T) {
	want := errors.New("offline")
	c := New(Options{HTTPClient: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, want })}})
	tr := NewTrader(c, "fake-key", "fake-secret", []domain.Market{domain.Futures})
	assert.ErrorIs(t, tr.SetLeverage(context.Background(), domain.Futures, "BTCUSDT", 2), want)
}

func TestKlineWebsocketConfirmAndTriggerDirection(t *testing.T) {
	c := New(Options{Testnet: true})
	cs := c.parseKlineMsg(domain.Futures, []byte(`{"topic":"kline.60.BTCUSDT","data":[{"start":1767225600000,"interval":"60","open":"100","high":"110","low":"90","close":"105","volume":"2","turnover":"210","confirm":true}]}`))
	require.Len(t, cs, 1)
	assert.True(t, cs[0].Closed)
	assert.Equal(t, domain.Timeframe("1h"), cs[0].Timeframe)
	assert.Equal(t, "BTCUSDT", cs[0].Symbol)
	assert.Empty(t, c.parseKlineMsg(domain.Futures, []byte(`{"op":"pong"}`)))
	for _, tc := range []struct {
		typ       domain.OrderType
		side      domain.Side
		direction int
	}{
		{domain.OrderStopMarket, domain.Buy, 1}, {domain.OrderStopMarket, domain.Sell, 2},
		{domain.OrderTakeProfit, domain.Buy, 2}, {domain.OrderTakeProfit, domain.Sell, 1},
	} {
		assert.Equal(t, tc.direction, triggerDirection(domain.Order{Type: tc.typ, Side: tc.side}))
	}
	assert.Equal(t, domain.OrderCanceled, status("Deactivated"))
	assert.Equal(t, domain.OrderCanceled, status("PartiallyFilledCanceled"))
}
