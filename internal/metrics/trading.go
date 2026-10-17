package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Trading holds the collectors updated by a trading session.
type Trading struct {
	Equity        *prometheus.GaugeVec
	Balance       *prometheus.GaugeVec
	Unrealized    *prometheus.GaugeVec
	Exposure      *prometheus.GaugeVec
	OpenPositions *prometheus.GaugeVec
	RealizedPnL   *prometheus.GaugeVec // cumulative this session, by strategy
	Trades        *prometheus.CounterVec
	Orders        *prometheus.CounterVec
	Fills         *prometheus.CounterVec
	PlaceLatency  *prometheus.HistogramVec
	Notices       *prometheus.CounterVec
	KillSwitch    prometheus.Gauge
	Paused        prometheus.Gauge
	Reconciles    *prometheus.CounterVec
	UsedWeight    *prometheus.GaugeVec
}

// NewTrading registers the trading collectors on reg.
func NewTrading(reg prometheus.Registerer) *Trading {
	acct := []string{"account"}
	t := &Trading{
		Equity:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_equity", Help: "Account equity (balance + unrealized PnL), quote currency."}, acct),
		Balance:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_balance", Help: "Account cash balance incl. realized PnL."}, acct),
		Unrealized:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_unrealized_pnl", Help: "Unrealized PnL of open positions."}, acct),
		Exposure:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_exposure", Help: "Sum of open position notionals."}, acct),
		OpenPositions: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_open_positions", Help: "Number of open positions."}, acct),
		RealizedPnL:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_realized_pnl", Help: "Net realized PnL of closed trades this session."}, []string{"strategy"}),
		Trades:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tradingbot_trades_total", Help: "Closed round trips."}, []string{"strategy", "result", "reason"}),
		Orders:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tradingbot_orders_total", Help: "Order state changes."}, []string{"account", "tag", "status"}),
		Fills:         prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tradingbot_fills_total", Help: "Executions."}, []string{"account", "tag"}),
		PlaceLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "tradingbot_order_place_seconds", Help: "Time for the broker to accept an order.",
			Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"exchange", "result"}),
		Notices:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tradingbot_notices_total", Help: "Engine notices by level."}, []string{"level"}),
		KillSwitch: prometheus.NewGauge(prometheus.GaugeOpts{Name: "tradingbot_kill_switch", Help: "1 when the kill switch is active."}),
		Paused:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "tradingbot_paused", Help: "1 when new entries are paused."}),
		Reconciles: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tradingbot_reconcile_total", Help: "Reconciliations and corrections."}, []string{"result"}),
		UsedWeight: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tradingbot_exchange_used_weight", Help: "Request weight used in the current minute, as reported by the exchange."}, []string{"exchange", "market"}),
	}
	reg.MustRegister(t.Equity, t.Balance, t.Unrealized, t.Exposure, t.OpenPositions, t.RealizedPnL, t.Trades,
		t.Orders, t.Fills, t.PlaceLatency, t.Notices, t.KillSwitch, t.Paused, t.Reconciles, t.UsedWeight)
	return t
}

// ObservePlace records how long a broker took to accept an order.
func (t *Trading) ObservePlace(exchange string, d time.Duration, err error) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	t.PlaceLatency.WithLabelValues(exchange, result).Observe(d.Seconds())
}

func bool01(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// SetFlags updates the kill switch and pause gauges.
func (t *Trading) SetFlags(killed, paused bool) {
	t.KillSwitch.Set(bool01(killed))
	t.Paused.Set(bool01(paused))
}
