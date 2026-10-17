package marketdata

import "github.com/prometheus/client_golang/prometheus"

// Metrics are the Prometheus collectors of the market data feed.
type Metrics struct {
	closedCandles *prometheus.CounterVec
	backfilled    *prometheus.CounterVec
	connects      *prometheus.CounterVec
	lastClose     *prometheus.GaugeVec
}

// NewMetrics registers the feed collectors on reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		closedCandles: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tradingbot_marketdata_closed_candles_total",
			Help: "Closed candles received from the live stream.",
		}, []string{"exchange", "market", "timeframe"}),
		backfilled: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tradingbot_marketdata_backfilled_candles_total",
			Help: "Candles fetched through REST to fill gaps.",
		}, []string{"exchange", "market", "timeframe"}),
		connects: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tradingbot_marketdata_stream_connects_total",
			Help: "Successful stream (re)connections.",
		}, []string{"exchange", "market"}),
		lastClose: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tradingbot_marketdata_last_candle_close_timestamp_seconds",
			Help: "Close time of the newest closed candle per series; alert when it falls behind.",
		}, []string{"exchange", "market", "symbol", "timeframe"}),
	}
	reg.MustRegister(m.closedCandles, m.backfilled, m.connects, m.lastClose)
	return m
}
