package managed

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/rizz404/trading-bot/internal/risk"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

type HTTP struct {
	RateLine func() string
	Wallets  map[string]*Runtime
	Password string
	Money    func(decimal.Decimal) string
	token    string
}

func NewHTTP(wallets map[string]*Runtime, password string, money func(decimal.Decimal) string) *HTTP {
	h := &HTTP{Wallets: wallets, Password: password, Money: money, token: nonce()}
	if h.Money == nil {
		h.Money = func(v decimal.Decimal) string { return v.StringFixed(4) + " USDT" }
	}
	return h
}
func (h *HTTP) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	if h.Password != "" {
		_, pass, ok := q.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(h.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Trading bot"`)
			http.Error(w, "Autentikasi diperlukan", http.StatusUnauthorized)
			return
		}
	}
	if q.URL.Path == "/managed.css" && q.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write([]byte(dashboardCSS))
		return
	}
	wallet := q.URL.Query().Get("wallet")
	if wallet == "" {
		ids := make([]string, 0, len(h.Wallets))
		for id := range h.Wallets {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		if len(ids) > 0 {
			wallet = ids[0]
		}
	}
	rt := h.Wallets[wallet]
	if rt == nil {
		http.Error(w, "Akun tidak ditemukan", http.StatusNotFound)
		return
	}
	if q.Method == http.MethodPost {
		if q.URL.Path != "/control" {
			http.NotFound(w, q)
			return
		}
		q.Body = http.MaxBytesReader(w, q.Body, 65536)
		var c Command
		if strings.HasPrefix(q.Header.Get("Content-Type"), "application/json") {
			if subtle.ConstantTimeCompare([]byte(q.Header.Get("X-CSRF-Token")), []byte(h.token)) != 1 {
				http.Error(w, "Token aksi tidak valid", http.StatusForbidden)
				return
			}
			dec := json.NewDecoder(q.Body)
			dec.DisallowUnknownFields()
			if dec.Decode(&c) != nil || dec.Decode(new(any)) != io.EOF {
				http.Error(w, "Perintah tidak valid", http.StatusBadRequest)
				return
			}
		} else {
			if q.ParseForm() != nil {
				http.Error(w, "Form tidak valid", http.StatusBadRequest)
				return
			}
			if subtle.ConstantTimeCompare([]byte(q.FormValue("csrf")), []byte(h.token)) != 1 {
				http.Error(w, "Token aksi tidak valid", http.StatusForbidden)
				return
			}
			var err error
			c, err = formCommand(q)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if origin := q.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != q.Host {
				http.Error(w, "Origin ditolak", http.StatusForbidden)
				return
			}
		}
		if err := rt.Command(q.Context(), c); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Redirect(w, q, "/?wallet="+url.QueryEscape(wallet), http.StatusSeeOther)
		return
	}
	if q.Method != http.MethodGet {
		http.Error(w, "Method tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	state, err := rt.Snapshot(q.Context())
	if err != nil {
		http.Error(w, "Status akun tidak tersedia", http.StatusServiceUnavailable)
		return
	}
	if q.URL.Path == "/api/state" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
		return
	}
	if q.URL.Path != "/" {
		http.NotFound(w, q)
		return
	}
	data := struct {
		Wallet                      string
		Wallets                     map[string]*Runtime
		State                       *State
		Token, Request, Error, Rate string
	}{wallet, h.Wallets, state, h.token, nonce(), q.URL.Query().Get("error"), ""}
	if h.RateLine != nil {
		data.Rate = h.RateLine()
	}
	tmpl := template.Must(template.New("dashboard").Funcs(template.FuncMap{"money": h.Money, "dd": drawdown, "chart": chartPath, "unrealized": state.Unrealized,
		"pnl":         func() decimal.Decimal { return state.Equity().Sub(state.Spec.Capital) },
		"instancePnL": func(id string) decimal.Decimal { return state.InstanceEquity(id).Sub(state.Instances[id].Funded) },
		"limits": func(scope string) any {
			c := state.Spec.Risk
			cap := decimal.Zero
			if i := state.Instances[scope]; i != nil {
				c = i.Risk
			}
			if b := state.Bindings[scope]; b != nil {
				c = b.Spec.Risk
				cap = b.Spec.Cap
			}
			c = effectiveRisk(c)
			return struct {
				Scope                                     string
				Risk, Position, Exposure, Daily, Drawdown float64
				Cap                                       decimal.Decimal
			}{scope, c.RiskPerTrade * 100, c.MaxPositionPct * 100, c.MaxExposurePct * 100, c.DailyLossLimit * 100, c.MaxDrawdown * 100, cap}
		}, "equity": state.InstanceEquity, "free": state.FreeCash, "accountEquity": state.Equity, "holdingValue": func(p *Holding) decimal.Decimal { return p.Quantity.Mul(state.Prices[p.Symbol]) }, "inc": func(s string) string { return s }, "dict": func(v ...string) map[string]string {
			m := map[string]string{}
			for i := 0; i+1 < len(v); i += 2 {
				m[v[i]] = v[i+1]
			}
			return m
		}, "csrf": func() string { return h.token }, "rid": nonce, "wallet": func() string { return wallet }}).Parse(dashboardHTML))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}
func formCommand(q *http.Request) (Command, error) {
	c := Command{ID: q.FormValue("id"), Operation: q.FormValue("operation"), Scope: q.FormValue("scope"), From: q.FormValue("from"), To: q.FormValue("to"), Name: q.FormValue("name"), Policy: q.FormValue("policy"), Confirm: q.FormValue("confirm") == "yes"}
	if c.ID == "" {
		return c, errors.New("ID aksi diperlukan")
	}
	if v := q.FormValue("amount"); v != "" {
		d, err := decimal.NewFromString(v)
		if err != nil {
			return c, errors.New("nominal tidak valid")
		}
		c.Amount = d
	}
	if c.Operation == "limits" {
		c.Risk = new(risk.Config)
		*c.Risk = effectiveRisk(risk.Config{})
		c.Risk.MaxLeverage = 1
		fields := []struct {
			name   string
			target *float64
		}{{"risk_pct", &c.Risk.RiskPerTrade}, {"position_pct", &c.Risk.MaxPositionPct}, {"exposure_pct", &c.Risk.MaxExposurePct}, {"daily_pct", &c.Risk.DailyLossLimit}, {"drawdown_pct", &c.Risk.MaxDrawdown}}
		for _, f := range fields {
			v, err := strconv.ParseFloat(q.FormValue(f.name), 64)
			if err != nil {
				return c, errors.New("batas risiko tidak valid")
			}
			*f.target = v / 100
		}
	}
	if c.Operation == "create" || c.Operation == "clone" {
		c.Instance = &InstanceSpec{ID: q.FormValue("instance_id"), Name: q.FormValue("name")}
	}
	if c.Operation == "bind" || c.Operation == "edit_binding" {
		tf := q.FormValue("timeframe")
		if tf == "" {
			tf = "4h"
		}
		name := q.FormValue("strategy")
		params := map[string]any{"timeframe": tf}
		switch name {
		case "momentum":
			name = "trendspot"
			params["mode"] = "momentum"
			params["entry_period"] = 84
			params["trend_period"] = 100
			params["momentum_min"] = 0.05
		case "emacross":
			params["allow_short"] = false
		default:
			return c, errors.New("template strategi tidak tersedia")
		}
		c.Binding = &BindingSpec{ID: q.FormValue("binding_id"), Name: name, Symbols: []string{q.FormValue("symbol")}, Params: params, Cap: c.Amount}
	}
	return c, nil
}

const dashboardCSS = `:root{color-scheme:light dark;font:15px system-ui;background:#f5f7fa;color:#172534}body{margin:0}main{max-width:1200px;margin:auto;padding:24px}h1{font-size:28px}h2{font-size:20px}nav,.cards,.actions{display:flex;gap:12px;flex-wrap:wrap}.card,article,details{background:white;border:1px solid #d5dde6;border-radius:12px;padding:18px;margin:14px 0}.card{flex:1;min-width:160px}strong{display:block}svg{width:100%;height:180px}svg path{fill:none;stroke:#3299d6;stroke-width:3}table{width:100%;border-collapse:collapse}td,th{text-align:left;padding:10px;border-bottom:1px solid #d5dde6}form{display:flex;align-items:end;gap:8px;flex-wrap:wrap;margin:12px 0}input,select,button{font:inherit;padding:9px;border-radius:6px;border:1px solid #a4b2c1;max-width:100%;box-sizing:border-box}button{cursor:pointer;background:#135e96;color:white}label{display:grid;gap:4px}.scroll{overflow:auto}.warning{background:#fff0ca;padding:14px;border-radius:8px}small{color:#506477}code{overflow-wrap:anywhere}@media(prefers-color-scheme:dark){:root{background:#101a25;color:#edf3fa}.card,article,details{background:#182635;border-color:#36495e}small{color:#b1c4d8}.warning{background:#463b19}}@media(max-width:600px){main{padding:12px}h1{font-size:23px}td,th{padding:7px}form>*{max-width:100%}}`
const dashboardHTML = `<!doctype html><html lang="id"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Portofolio bot</title><link rel="stylesheet" href="/managed.css"><main>
<h1>Portofolio bot</h1><nav>{{range $id,$rt:=.Wallets}}<a href="/?wallet={{$id}}">{{$id}}</a>{{end}}<a href="/api/state?wallet={{.Wallet}}">Ekspor status</a></nav>
<p>Akun {{.Wallet}} · {{.State.Spec.Mode}} · {{.State.Spec.Exchange}}</p>{{if .State.Attention}}<p class="warning">Perlu diperiksa: {{.State.Attention}}. Entry baru ditahan.</p>{{end}}
{{if .State.Halted}}<p class="warning">Batas risiko akun aktif: {{.State.Reason}}</p>{{end}}
<div class="cards"><div class="card">Nilai akun bot<strong>{{money accountEquity}}</strong></div><div class="card">Cadangan belum dialokasikan<strong>{{money .State.Reserve}}</strong></div><div class="card">Mulai<strong>{{.State.StartedAt}}</strong></div></div>
<div class="card">PnL net akun<strong>{{money pnl}}</strong>Drawdown saat ini: {{dd accountEquity .State.Peak}}<br><small>Grafik equity per jam, maksimal 30 hari. Perubahan alokasi internal tidak menambah nilai akun.</small>{{if gt (len .State.Samples) 1}}<svg viewBox="0 0 1000 200" role="img" aria-label="Grafik nilai akun bot"><path d="{{chart .State.Samples}}"></path></svg>{{else}}<p>Grafik tersedia setelah dua sampel jam berbeda.</p>{{end}}</div>
<details><summary>Harga dan kesehatan</summary><p>Reconciliation exchange terakhir: {{.State.ReconciledAt}}</p>{{range $symbol,$price:=.State.Prices}}<p>{{$symbol}}: {{$price}} · {{index $.State.PriceAt $symbol}}</p>{{end}}<p>Perubahan harga terakhir juga menunjukkan apakah feed masih masuk. Penundaan feed atau selisih saldo menahan entry.</p></details>
{{template "limits" (limits "account")}}
<details><summary>Tambah instance atau salin template</summary><p>Instance baru dimulai tanpa modal dan dijeda. Transfer dana sebelum menjalankannya. Strategi baru diaktifkan pada startup berikutnya agar feed dan indikator siap.</p><form method="post" action="/control?wallet={{.Wallet}}"><input type="hidden" name="csrf" value="{{.Token}}"><input type="hidden" name="id" value="{{rid}}"><select name="operation"><option value="create">Buat baru</option><option value="clone">Salin konfigurasi</option></select><label>ID baru<input name="instance_id" required></label><label>Nama<input name="name" required></label><label>Sumber template<select name="scope">{{range $id,$i:=.State.Instances}}<option>{{$id}}</option>{{end}}</select></label><button>Simpan</button></form></details>
<details><summary>Pindahkan modal bebas</summary><p>Dana dalam order dan nilai koin tidak bisa dipindahkan sebagai saldo tunai.</p><form method="post" action="/control?wallet={{.Wallet}}"><input type="hidden" name="csrf" value="{{.Token}}"><input type="hidden" name="id" value="{{rid}}"><input type="hidden" name="operation" value="transfer"><label>Dari<select name="from"><option value="reserve">Cadangan</option>{{range $id,$i:=.State.Instances}}<option>{{$id}}</option>{{end}}</select></label><label>Ke<select name="to"><option value="reserve">Cadangan</option>{{range $id,$i:=.State.Instances}}<option>{{$id}}</option>{{end}}</select></label><label>USDT<input name="amount" inputmode="decimal" required></label><button>Transfer</button></form></details>
{{template "control" (dict "scope" "account" "label" "Seluruh akun bot")}}
{{range $id,$i:=.State.Instances}}<article><h2>{{$i.Name}} <small>{{$id}} · {{$i.Status}}</small></h2><div class="cards"><div>Modal neto<strong>{{money $i.Funded}}</strong></div><div>Saldo bebas<strong>{{money (free $id)}}</strong></div><div>Nilai instance<strong>{{money (equity $id)}}</strong></div></div>{{if $i.Halted}}<p class="warning">Batas risiko instance aktif</p>{{end}}
<p>PnL net {{money (instancePnL $id)}} · drawdown {{dd (equity $id) $i.Peak}}</p>
{{template "control" (dict "scope" $id "label" "Instance ini")}}{{template "limits" (limits $id)}}
<details><summary>Ubah nama / notifikasi</summary><form method="post" action="/control?wallet={{wallet}}"><input type="hidden" name="csrf" value="{{csrf}}"><input type="hidden" name="id" value="{{rid}}"><input type="hidden" name="scope" value="{{$id}}"><select name="operation"><option value="rename">Nama</option><option value="notify">Notifikasi: info/warn/error/off</option></select><input name="name" value="{{$i.Name}}" required><button>Simpan</button></form></details>
<div class="scroll"><table><thead><tr><th>Strategi</th><th>Status / alasan</th><th>PnL terealisasi</th><th>Komisi</th></tr></thead><tbody>{{range $owner,$b:=$.State.Bindings}}{{if eq $b.InstanceID $id}}<tr><td>{{$b.Spec.Name}} · {{$b.Spec.Symbols}}<br><small>{{$owner}}</small></td><td>{{$b.Status}}<br>{{$b.Reason}}<br><small>{{if $b.LastNotice}}{{$b.LastNotice}}{{else}}Menunggu sinyal atau jadwal berikutnya; saldo, minimum order, dan risiko tetap diperiksa.{{end}}</small></td><td>{{money $b.Realized}}<br><small>Belum terealisasi: {{money (unrealized $owner)}}</small></td><td>{{money $b.Fees}}</td></tr><tr><td colspan="4">{{template "control" (dict "scope" $owner "label" "Strategi ini")}}{{template "limits" (limits $owner)}}</td></tr>{{end}}{{end}}</tbody></table></div>
<details><summary>Tambah strategi (jeda instance dahulu)</summary><form method="post" action="/control?wallet={{wallet}}"><input type="hidden" name="csrf" value="{{csrf}}"><input type="hidden" name="id" value="{{rid}}"><input type="hidden" name="operation" value="bind"><input type="hidden" name="scope" value="{{$id}}"><label>ID strategi<input name="binding_id" required></label><label>Strategi<select name="strategy"><option value="momentum">Momentum mengikuti tren</option><option value="emacross">EMA cross</option></select></label><label>Pair<input name="symbol" placeholder="ETHUSDT" required></label><label>Candle<select name="timeframe"><option>4h</option><option>1d</option><option>1h</option></select></label><label>Batas modal opsional (USDT)<input name="amount" value="0"></label><button>Simpan konfigurasi</button></form></details></article>{{end}}
<details open><summary>Inventori per strategi</summary><div class="scroll"><table><tr><th>Pemilik</th><th>Pair</th><th>Kuantitas</th><th>Biaya pokok</th><th>Nilai saat ini</th></tr>{{range .State.Holdings}}{{if .Quantity.IsPositive}}<tr><td>{{.Owner}}</td><td>{{.Symbol}}</td><td>{{.Quantity}}</td><td>{{money .Cost}}</td><td>{{money (holdingValue .)}}</td></tr>{{end}}{{end}}</table></div></details>
<details><summary>Order dan reservasi</summary><div class="scroll"><table><tr><th>Pemilik</th><th>Pair / sisi</th><th>Status</th><th>Dana dipesan</th><th>Koin dipesan</th></tr>{{range .State.Orders}}<tr><td>{{.Owner}}</td><td>{{.Order.Symbol}} {{.Order.Side}}</td><td>{{.State}}</td><td>{{money .QuoteReserved}}</td><td>{{.BaseReserved}}</td></tr>{{end}}</table></div></details>
<details><summary>Eksekusi dan komisi aktual</summary><p>Komisi aset ketiga ditampilkan dalam aset asli. Nilai quote final masuk total fee strategi setelah harga aset komisi tersedia.</p><div class="scroll"><table><tr><th>Waktu / pemilik</th><th>Pair / sisi</th><th>Kuantitas / harga</th><th>Komisi asli</th></tr>{{range .State.Executions}}<tr><td>{{.Fill.Time}}<br>{{.Owner}}</td><td>{{.Fill.Symbol}} {{.Fill.Side}}</td><td>{{.Fill.Quantity}} / {{.Fill.Price}}</td><td>{{.Fill.Commission}} {{.Fill.FeeAsset}}<br>Setara quote: {{money .Fill.Fee}}</td></tr>{{end}}</table></div></details>
<details><summary>Sesi dan versi konfigurasi</summary>{{range .State.Runs}}<p>{{.Mode}} · {{.Status}} · {{.StartedAt}} → {{.EndedAt}}<br><small>{{.ID}} · <code>{{.ConfigurationHash}}</code></small></p>{{end}}</details>
<details><summary>Riwayat perubahan</summary>{{range .State.History}}<p><small>{{.At}}</small> · {{.Kind}} · {{.Scope}} · {{.Message}}</p>{{end}}</details>
<p><small>Nilai koin berubah mengikuti harga. {{.Rate}}<br>Order hanya selesai setelah status exchange dikonfirmasi. Stop untuk resume tetap mempertahankan state; akhiri eksperimen adalah aksi terpisah.</small></p></main>
{{define "control"}}<form method="post" action="/control?wallet={{wallet}}"><input type="hidden" name="csrf" value="{{csrf}}"><input type="hidden" name="id" value="{{rid}}"><input type="hidden" name="scope" value="{{.scope}}"><label>{{.label}}<select name="operation"><option value="pause">Jeda entry baru</option><option value="resume">Lanjutkan</option><option value="cancel">Batalkan order pending</option><option value="flatten">Tutup posisi</option><option value="finish">Akhiri eksperimen</option><option value="kill">Penghentian darurat</option><option value="reset">Reset batas risiko</option><option value="archive">Arsipkan jika sudah kosong</option></select></label><label>Inventori saat akhir<select name="policy"><option value="retain">Pertahankan koin</option><option value="flatten">Jual koin milik scope ini</option></select></label><label><span>Konfirmasi aksi dana</span><select name="confirm"><option value="no">Belum</option><option value="yes">Ya, scope dan dampak sudah benar</option></select></label><button>Terapkan</button></form>{{end}}
{{define "limits"}}<details><summary>Batas risiko · {{.Scope}}</summary><form method="post" action="/control?wallet={{wallet}}"><input type="hidden" name="csrf" value="{{csrf}}"><input type="hidden" name="id" value="{{rid}}"><input type="hidden" name="operation" value="limits"><input type="hidden" name="scope" value="{{.Scope}}"><label>Risiko per entry (%)<input name="risk_pct" value="{{.Risk}}" required></label><label>Batas per koin (%)<input name="position_pct" value="{{.Position}}" required></label><label>Total exposure (%)<input name="exposure_pct" value="{{.Exposure}}" required></label><label>Rugi harian (%)<input name="daily_pct" value="{{.Daily}}" required></label><label>Drawdown (%)<input name="drawdown_pct" value="{{.Drawdown}}" required></label><label>Batas modal strategi (USDT, 0=pool)<input name="amount" value="{{.Cap}}"></label><label>Konfirmasi<select name="confirm"><option value="no">Belum</option><option value="yes">Ya, terapkan batas ini</option></select></label><button>Simpan batas</button></form><small>Persentase posisi/exposure memakai equity instance untuk strategi. Batas parent tetap berlaku; 0 menonaktifkan batas yang dipilih. Batas modal hanya berlaku pada scope strategi.</small></details>{{end}}
`
