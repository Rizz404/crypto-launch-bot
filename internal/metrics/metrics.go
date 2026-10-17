// Package metrics exposes Prometheus metrics and a health endpoint.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRegistry returns a registry with Go runtime, process and build info
// collectors already registered.
func NewRegistry(version, mode string) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	info := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "tradingbot_build_info",
		Help:        "Build and runtime information about the trading bot.",
		ConstLabels: prometheus.Labels{"version": version, "mode": mode},
	})
	info.Set(1)
	reg.MustRegister(info)
	return reg
}

// HealthFunc reports whether the bot's dependencies are healthy.
type HealthFunc func(ctx context.Context) error

// NewServer serves /metrics and /healthz on addr.
func NewServer(addr string, reg *prometheus.Registry, health HealthFunc) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := health(ctx); err != nil {
			http.Error(w, "unhealthy: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
