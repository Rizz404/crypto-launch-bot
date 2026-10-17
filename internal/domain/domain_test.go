package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestInstrumentRounding(t *testing.T) {
	in := Instrument{Symbol: "BTCUSDT", TickSize: d("0.10"), StepSize: d("0.001"), MinQty: d("0.001"), MinNotional: d("50")}

	assert.Equal(t, "81513.6", in.RoundPrice(d("81513.69")).String())
	assert.Equal(t, "81513.6", in.RoundPrice(d("81513.6")).String())
	assert.Equal(t, "0.012", in.RoundQty(d("0.0129")).String())
	assert.Equal(t, "0", in.RoundQty(d("0.0009")).String())

	assert.NoError(t, in.CheckOrder(d("81513.6"), d("0.001")))
	assert.ErrorContains(t, in.CheckOrder(d("81513.6"), d("0.0005")), "below min")
	assert.ErrorContains(t, in.CheckOrder(d("40000"), d("0.001")), "notional 40")

	zero := Instrument{}
	assert.Equal(t, "1.2345", zero.RoundQty(d("1.2345")).String(), "no step means no rounding")
}

func TestTickerSpread(t *testing.T) {
	bps, ok := Ticker{Bid: d("99.95"), Ask: d("100.05")}.SpreadBps()
	require.True(t, ok)
	assert.InDelta(t, 10.0, bps, 1e-9)

	_, ok = Ticker{Bid: d("0"), Ask: d("1")}.SpreadBps()
	assert.False(t, ok)
}

func TestTimeframe(t *testing.T) {
	tf, err := ParseTimeframe("4h")
	require.NoError(t, err)
	assert.Equal(t, 4*time.Hour, tf.Duration())

	_, err = ParseTimeframe("1M")
	assert.Error(t, err)

	ts := time.Date(2026, 10, 17, 13, 47, 5, 0, time.UTC) // Saturday
	assert.Equal(t, time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC), Timeframe("4h").Truncate(ts))
	assert.Equal(t, time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), Timeframe("1d").Truncate(ts))
	assert.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Timeframe("1w").Truncate(ts), "Monday")
	// Binance 3d bars opened on 2026-10-14 and 2026-10-17.
	assert.Equal(t, time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), Timeframe("3d").Truncate(ts))
	assert.Equal(t, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Timeframe("3d").Truncate(ts.Add(-24*time.Hour)))

	c := Candle{Timeframe: "15m", OpenTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	assert.Equal(t, time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC), c.CloseTime())
}
