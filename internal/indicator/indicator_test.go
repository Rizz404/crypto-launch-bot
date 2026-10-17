package indicator

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// These short sequences use independently calculated arithmetic/Wilder
// recurrences, so expected values don't come from the implementation.
func TestMovingAveragesAndBollinger(t *testing.T) {
	sma, ema, bands := NewSMA(3), NewEMA(3), NewBollinger(3, 2)
	assert.Zero(t, sma.Value())
	for i, v := range []float64{1, 2, 3, 6, 4} {
		sma.Update(v)
		ema.Update(v)
		bands.Update(v)
		assert.Equal(t, i >= 2, sma.Ready())
		assert.Equal(t, i >= 2, ema.Ready())
		assert.Equal(t, i >= 2, bands.Ready())
		if i == 2 {
			assert.Equal(t, 2.0, ema.Value(), "EMA seed is the first SMA")
		}
	}
	assert.InDelta(t, 13.0/3, sma.Value(), 1e-12)
	assert.Equal(t, 4.0, ema.Value()) // 2 -> 4 -> 4, alpha=1/2
	assert.InDelta(t, 13.0/3, bands.Middle(), 1e-12)
	assert.InDelta(t, 13.0/3+2*math.Sqrt(14.0/9), bands.Upper(), 1e-12)
	assert.InDelta(t, 13.0/3-2*math.Sqrt(14.0/9), bands.Lower(), 1e-12)
}

func TestRSIWilderAndZeroLoss(t *testing.T) {
	rsi := NewRSI(3)
	for i, v := range []float64{10, 12, 11, 14} {
		rsi.Update(v)
		assert.Equal(t, i == 3, rsi.Ready())
	}
	assert.InDelta(t, 100-100.0/6, rsi.Value(), 1e-12) // gain=5/3, loss=1/3
	assert.InDelta(t, 100-100.0/(1+10.0/11), rsi.Update(11), 1e-12)
	for _, tc := range []struct {
		values []float64
		want   float64
	}{{[]float64{1, 2, 3, 4}, 100}, {[]float64{4, 3, 2, 1}, 0}, {[]float64{4, 4, 4, 4}, 50}} {
		r := NewRSI(3)
		for _, v := range tc.values {
			r.Update(v)
		}
		assert.Equal(t, tc.want, r.Value())
	}
}

func TestATRIncludesGapsAndWilderSmoothing(t *testing.T) {
	a := NewATR(3)
	for i, bar := range [][3]float64{{12, 9, 10}, {14, 11, 13}, {12, 8, 9}} {
		a.Update(bar[0], bar[1], bar[2])
		assert.Equal(t, i == 2, a.Ready())
	}
	assert.Equal(t, 4.0, a.Value())            // true ranges 3,4,5
	assert.Equal(t, 5.0, a.Update(16, 14, 15)) // gap TR=7, (4*2+7)/3
}

func TestMACDAndCross(t *testing.T) {
	m := NewMACD(2, 3, 2)
	for i, v := range []float64{1, 2, 3, 6, 4} {
		m.Update(v)
		assert.Equal(t, i >= 3, m.Ready())
		if i == 3 {
			assert.InDelta(t, 2.0/3, m.Signal(), 1e-12)
		}
	}
	assert.InDelta(t, 5.0/18, m.Line(), 1e-12)
	assert.InDelta(t, 11.0/27, m.Signal(), 1e-12)
	assert.InDelta(t, -7.0/54, m.Histogram(), 1e-12)
	var cross Cross
	for i, pair := range [][2]float64{{1, 2}, {2, 2}, {3, 2}, {4, 2}, {1, 2}} {
		assert.Equal(t, []int{0, 0, 1, 0, -1}[i], cross.Update(pair[0], pair[1]))
	}
}
