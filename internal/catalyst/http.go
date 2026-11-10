package catalyst

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/ui"
)

// HTTP reuses the existing managed dashboard/control and protects research
// views with the same required password. It starts no Telegram poller.
func (p *Pilot) HTTP(password string) http.Handler {
	ledger := managed.NewHTTP(map[string]*managed.Runtime{"catalyst": p.Runtime}, password, nil)
	ledger.NavLinks = []ui.NavItem{{Key: "research", Label: "Riset AI", Href: "/research"}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if password == "" {
			http.Error(w, "Password dashboard wajib diisi", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path != "/research" && r.URL.Path != "/research.json" && r.URL.Path != "/healthz" {
			ledger.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		if r.Method != http.MethodGet {
			http.Error(w, "Metode tidak tersedia", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/healthz" {
			if !p.Status().Healthy(p.Policy, time.Now()) {
				http.Error(w, "runtime/feed unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"runtime_feed":"ok","research_ready_is_separate":true}`))
			return
		}
		_, pass, ok := r.BasicAuth()
		if password == "" || !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Catalyst paper"`)
			http.Error(w, "Autentikasi diperlukan", http.StatusUnauthorized)
			return
		}
		report, err := p.Archive.Report(p.Policy)
		if err != nil {
			http.Error(w, "Arsip riset tidak tersedia", http.StatusServiceUnavailable)
			return
		}
		status := p.Status()
		// A restart must not hide the last archived decision during inference dedup.
		if status.DecisionID == "" && len(report.Reviews) > 0 {
			last := report.Reviews[len(report.Reviews)-1]
			status.DecisionID, status.Action, status.Symbol, status.Reason = last.ID, last.Action, last.Symbol, last.Reason
			if status.Error == "" {
				status.Error = last.Error
			}
		}
		data := researchData{status, report}
		if r.URL.Path == "/research.json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(data)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Research(data).Render(r.Context(), w)
	})
}
