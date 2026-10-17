package binance

import (
	"testing"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpotExecutionCaseSensitiveFieldsAndBaseFee(t *testing.T) {
	tr := NewTrader(New(Options{Testnet: true}), "key", "secret", []domain.Market{domain.Spot})
	var events []broker.Event
	tr.SetHandler(func(ev broker.Event) { events = append(events, ev) })
	tr.handleSpotMessage([]byte(`{"event":{"e":"executionReport","E":1767225600000,"s":"BTCUSDT","c":"tb123","S":"BUY","o":"MARKET","q":"2","Q":"200","p":"0","P":"0","x":"TRADE","X":"FILLED","i":123,"I":999,"l":"2","L":"100","n":"0.002","N":"BTC","T":1767225600001,"t":321,"z":"2","Z":"200","m":false,"M":true,"w":false,"W":1767225600000,"v":0,"V":"NONE"}}`))
	require.Len(t, events, 1)
	assert.Equal(t, "2", events[0].Fill.Quantity.String())
	assert.Equal(t, "100", events[0].Fill.Price.String())
	assert.Equal(t, "321", events[0].Fill.TradeID)
	assert.Equal(t, "0.2", events[0].Fill.Fee.String())
	assert.Equal(t, "2", events[0].Order.Quantity.String())
	assert.Equal(t, domain.OrderFilled, events[0].Order.Status)
	assert.Equal(t, int64(1767225600001), events[0].Fill.Time.UnixMilli())
}
