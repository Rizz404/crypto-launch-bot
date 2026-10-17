package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTradingMetricsFlagsAndLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewTrading(reg)
	m.SetFlags(true, false)
	m.ObservePlace("sim", 100*time.Millisecond, nil)
	m.ObservePlace("sim", 200*time.Millisecond, errors.New("reject"))
	families, err := reg.Gather()
	require.NoError(t, err)
	var histograms int
	for _, f := range families {
		switch f.GetName() {
		case "tradingbot_kill_switch":
			assert.Equal(t, 1.0, f.Metric[0].Gauge.GetValue())
		case "tradingbot_paused":
			assert.Zero(t, f.Metric[0].Gauge.GetValue())
		case "tradingbot_order_place_seconds":
			for _, series := range f.Metric {
				histograms++
				assert.Equal(t, uint64(1), series.Histogram.GetSampleCount())
			}
		}
	}
	assert.Equal(t, 2, histograms, "success and error are separate latency series")
}
