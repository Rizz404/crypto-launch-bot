package catalyst

import (
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/rizz404/trading-bot/internal/managed"
)

var researchPage = template.Must(template.New("research").Parse(`<!doctype html><html lang="id"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Riset P3</title><body><main><h1>Riset katalis · paper</h1><p><a href="/">Dana, posisi, proteksi dan kontrol</a></p><p>Hanya uang virtual. Batas AI 12 panggilan per hari WIB.</p><h2>Operasi</h2><p>Timer terakhir: {{.Status.TickAt}}. Riset terakhir: {{.Status.ResearchAt}}.</p><p>{{.Status.Error}}</p><h2>Keputusan terakhir</h2><p>{{.Status.Action}} {{.Status.Symbol}} — {{.Status.Reason}}</p><h2>Sumber</h2><ul>{{range .Report.Sources}}<li>{{.SourceID}}: {{.Status}} ({{.At}})</li>{{end}}</ul><h2>Dokumen terakhir</h2><ul>{{range .Report.Documents}}<li><a href="{{.URL}}" rel="noopener noreferrer">{{.Title}}</a> · {{.SourceID}} · {{.FirstSeenAt}}</li>{{end}}</ul><p>Data sumber hilang bukan pembatalan event. Keputusan tidak otomatis berarti order diterima atau terisi.</p></main></body></html>`))

// HTTP reuses the existing managed dashboard/control and protects research
// views with the same required password. It starts no Telegram poller.
func (p *Pilot) HTTP(password string) http.Handler {
	ledger := managed.NewHTTP(map[string]*managed.Runtime{"catalyst": p.Runtime}, password, nil)
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
		w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'")
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
		data := struct {
			Status PilotStatus
			Report Report
		}{p.Status(), report}
		if r.URL.Path == "/research.json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(data)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = researchPage.Execute(w, data)
	})
}
