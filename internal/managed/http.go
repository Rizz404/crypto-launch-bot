package managed

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/rizz404/crypto-launch-bot/internal/marketview"
	"github.com/rizz404/crypto-launch-bot/internal/risk"
	"github.com/rizz404/crypto-launch-bot/internal/ui"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type HTTP struct {
	RateLine func() string
	// NavLinks are extra header links, for surfaces that embed the dashboard.
	NavLinks []ui.NavItem
	Wallets  map[string]*Runtime
	Password string
	Money    func(decimal.Decimal) string
	// Owners enables the account login (/register, /login, /account); nil
	// keeps basic auth with Password.
	Owners    Owners
	setupMu   sync.Mutex
	setupCode string
	// Markets returns the market hub of a wallet mode (paper and live share mainnet).
	Markets func(mode string) *marketview.Hub
	// Fiat converts amounts for the USD/IDR line; nil shows USDT only.
	Fiat Fiat
	// StartPrice feeds the buy-and-hold comparison; nil hides it.
	StartPrice StartPrice
	starts     sync.Map // exchange|symbol|unix -> decimal.Decimal; start prices never change
	// Restart stops the process so the supervisor starts it again, which
	// activates strategies added from the dashboard; nil hides the action.
	Restart  func()
	token    string
	sessions sessions
}

// view is the page data shared by the dashboard, trading and account pages.
func (h *HTTP) view(wallet string, wallets []string, tab string, state *State, q *http.Request) view {
	v := view{Wallet: wallet, Wallets: wallets, Tab: tab, State: state, Token: h.token, Error: q.URL.Query().Get("error"), OK: q.URL.Query().Get("ok"), money: h.Money, fiat: h.Fiat, links: h.NavLinks, Trade: h.Markets != nil, Auth: h.Owners != nil}
	if rt := h.Wallets[wallet]; rt != nil && state != nil && h.Restart != nil {
		v.Inactive = rt.Inactive(state)
	}
	if h.StartPrice != nil && state != nil {
		// Candles are stored under the network name (app.ExchangeName).
		exchange := state.Spec.Exchange
		if state.Spec.Mode == "testnet" {
			exchange += "-testnet"
		}
		v.startPrice = func(symbol string, at time.Time) (decimal.Decimal, bool) {
			key := exchange + "|" + symbol + "|" + strconv.FormatInt(at.Unix(), 10)
			if p, ok := h.starts.Load(key); ok {
				return p.(decimal.Decimal), true
			}
			p, ok := h.StartPrice(q.Context(), exchange, symbol, at)
			if ok {
				h.starts.Store(key, p)
			}
			return p, ok
		}
	}
	return v
}

func NewHTTP(wallets map[string]*Runtime, password string, money func(decimal.Decimal) string) *HTTP {
	h := &HTTP{Wallets: wallets, Password: password, Money: money, token: nonce()}
	if h.Money == nil {
		h.Money = usdtText
	}
	return h
}
func (h *HTTP) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	// Scripts, styles and XHR come from this origin only; there is no inline code or eval.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	if strings.HasPrefix(q.URL.Path, "/static/") && q.Method == http.MethodGet {
		ui.Handler().ServeHTTP(w, q)
		return
	}
	switch q.URL.Path {
	case "/login":
		h.login(w, q)
		return
	case "/register":
		h.register(w, q)
		return
	case "/logout":
		h.logout(w, q)
		return
	}
	if !h.authorized(q) {
		switch {
		case h.Owners == nil:
			w.Header().Set("WWW-Authenticate", `Basic realm="Crypto launch bot"`)
			http.Error(w, "Autentikasi diperlukan", http.StatusUnauthorized)
		case q.Method == http.MethodGet && !strings.HasPrefix(q.URL.Path, "/api/") && q.URL.Path != "/stream":
			http.Redirect(w, q, "/login", http.StatusSeeOther)
		default:
			http.Error(w, "Autentikasi diperlukan", http.StatusUnauthorized)
		}
		return
	}
	if q.URL.Path == "/account" {
		h.account(w, q)
		return
	}
	wallets := sortedWallets(h.Wallets)
	wallet := q.URL.Query().Get("wallet")
	if wallet == "" && len(wallets) > 0 {
		wallet = wallets[0]
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
				formError(w, q, wallet, err)
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
		st, err := rt.Snapshot(q.Context())
		if err != nil {
			http.Error(w, "Status akun tidak tersedia", http.StatusServiceUnavailable)
			return
		}
		confirm := q.Header.Get("X-Confirm-Password")
		if confirm == "" {
			confirm = q.FormValue("password")
		}
		if !h.liveConfirmed(q.Context(), st.Spec.Mode, confirm) {
			err = errors.New("akun live: masukkan password yang benar untuk konfirmasi")
			if strings.HasPrefix(q.Header.Get("Content-Type"), "application/json") {
				http.Error(w, err.Error(), http.StatusForbidden)
			} else {
				formError(w, q, wallet, err)
			}
			return
		}
		if c.Operation == "restart" {
			if h.Restart == nil || !c.Confirm {
				formError(w, q, wallet, errors.New("restart bot belum dikonfirmasi"))
				return
			}
			// Answer first: the shutdown closes this server too.
			http.Redirect(w, q, backURL(q, wallet)+"&ok="+url.QueryEscape("Bot sedang restart. Muat ulang halaman ini dalam ±30 detik."), http.StatusSeeOther)
			go h.Restart()
			return
		}
		if err := rt.Command(q.Context(), c); err != nil {
			if strings.HasPrefix(q.Header.Get("Content-Type"), "application/json") {
				http.Error(w, err.Error(), http.StatusConflict)
			} else {
				formError(w, q, wallet, err)
			}
			return
		}
		http.Redirect(w, q, backURL(q, wallet)+"&ok="+url.QueryEscape(doneMessage(c)), http.StatusSeeOther)
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
	switch q.URL.Path {
	case "/trade", "/api/candles", "/stream":
		hub := h.hub(state.Spec.Mode)
		if hub == nil {
			http.Error(w, "Data pasar tidak tersedia untuk akun ini", http.StatusServiceUnavailable)
			return
		}
		tv := h.tradeView(q, wallet, wallets, state, hub)
		switch q.URL.Path {
		case "/trade":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = Trade(tv).Render(q.Context(), w)
		case "/api/candles":
			h.candles(w, q, tv, hub)
		default:
			h.stream(w, q, rt, tv, hub)
		}
		return
	}
	if q.URL.Path != "/" {
		http.NotFound(w, q)
		return
	}
	tab := q.URL.Query().Get("tab")
	if !slices.ContainsFunc(tabs, func(t ui.NavItem) bool { return t.Key == tab }) {
		tab = tabs[0].Key
	}
	v := h.view(wallet, wallets, tab, state, q)
	if h.RateLine != nil {
		v.Rate = h.RateLine()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = Dashboard(v).Render(q.Context(), w)
}

// formError returns a rejected browser action to the dashboard, where the
// reason is shown, instead of a bare text page. JSON clients keep status codes.
// doneMessage is the toast after a successful action.
func doneMessage(c Command) string {
	scope := c.Scope
	if scope == "account" {
		scope = "seluruh akun"
	}
	switch c.Operation {
	case "pause":
		return "Entry baru dijeda: " + scope
	case "resume":
		return "Dilanjutkan: " + scope
	case "cancel":
		return "Order pending dibatalkan: " + scope
	case "flatten":
		return "Posisi sedang ditutup: " + scope
	case "finish":
		return "Eksperimen sedang diakhiri: " + scope
	case "kill":
		return "Penghentian darurat aktif: " + scope
	case "reset":
		return "Batas risiko direset: " + scope
	case "archive":
		return "Diarsipkan: " + scope
	case "limits":
		return "Batas risiko disimpan: " + scope
	case "create", "clone":
		return "Instance dibuat. Pindahkan modal ke instance ini sebelum menambah strategi."
	case "transfer":
		return "Modal " + idNumber(c.Amount, 2) + " USDT dipindahkan dari " + c.From + " ke " + c.To
	case "bind", "edit_binding":
		return "Strategi disimpan. Tekan \"Aktifkan strategi baru\" supaya mulai berjalan."
	case "rename", "notify":
		return "Pengaturan instance disimpan"
	case "place_order":
		return "Order dikirim"
	case "cancel_order":
		return "Pembatalan order dikirim"
	case "close_position":
		return "Penutupan posisi dikirim"
	}
	return "Aksi berhasil"
}

func formError(w http.ResponseWriter, q *http.Request, wallet string, err error) {
	http.Redirect(w, q, backURL(q, wallet)+"&error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
}

// backURL returns to the dashboard tab the action was submitted from.
func backURL(q *http.Request, wallet string) string {
	if q.FormValue("view") == "trade" {
		return "/trade?wallet=" + url.QueryEscape(wallet) + "&symbol=" + url.QueryEscape(q.FormValue("symbol")) + "&tf=" + url.QueryEscape(q.FormValue("tf"))
	}
	u := "/?wallet=" + url.QueryEscape(wallet)
	if tab := q.URL.Query().Get("tab"); slices.ContainsFunc(tabs, func(t ui.NavItem) bool { return t.Key == tab }) {
		u += "&tab=" + tab
	}
	return u
}

// formFloat parses a decimal form field such as a price or an order size.
func formFloat(q *http.Request, name string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(q.FormValue(name)), 64)
}

func formCommand(q *http.Request) (Command, error) {
	c := Command{ID: q.FormValue("id"), Operation: q.FormValue("operation"), Scope: q.FormValue("scope"), From: q.FormValue("from"), To: q.FormValue("to"), Name: q.FormValue("name"), Policy: q.FormValue("policy"), Order: q.FormValue("order"), Confirm: q.FormValue("confirm") == "yes"}
	if c.ID == "" {
		return c, errors.New("ID aksi diperlukan")
	}
	if manualOperations[c.Operation] {
		c.Scope, c.Symbol, c.Side, c.OrderType = q.FormValue("scope"), q.FormValue("symbol"), q.FormValue("side"), q.FormValue("order_type")
		for _, f := range []struct {
			name   string
			target *decimal.Decimal
		}{{"quantity", &c.Quantity}, {"price", &c.Price}, {"stop_loss", &c.StopLoss}, {"take_profit", &c.TakeProfit}} {
			if v := strings.TrimSpace(q.FormValue(f.name)); v != "" {
				d, err := decimal.NewFromString(v)
				if err != nil {
					return c, errors.New("angka order tidak valid: " + f.name)
				}
				*f.target = d
			}
		}
		if c.OrderType == "market" {
			c.Price = decimal.Zero
		}
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
		case "pullback":
			// Research stage three parameters, shortlisted by the 29 October cost replay.
			name = "trendspot"
			for k, v := range map[string]any{"mode": "pullback", "entry_period": 55, "exit_period": 20, "trend_period": 200,
				"rsi_period": 14, "rsi_entry": 40, "rsi_exit": 65, "atr_period": 14, "stop_atr": 2.5, "target_atr": 5, "trail_atr": 0} {
				params[k] = v
			}
		case "dca", "grid":
			quote, err := formFloat(q, "quote")
			if err != nil || quote <= 0 {
				return c, errors.New("nominal per order (USDT) wajib diisi untuk DCA dan grid")
			}
			if name == "dca" {
				params["interval"], params["quote_amount"], params["max_buys"], params["take_profit"] = tf, quote, 5, 0.03
				break
			}
			lower, err1 := formFloat(q, "lower")
			upper, err2 := formFloat(q, "upper")
			levels, err3 := strconv.Atoi(strings.TrimSpace(q.FormValue("levels")))
			if err1 != nil || err2 != nil || err3 != nil {
				return c, errors.New("grid butuh batas bawah, batas atas dan jumlah level")
			}
			params["lower"], params["upper"], params["levels"], params["quote_per_level"] = lower, upper, levels, quote
		case ManualStrategy:
			params = nil
		default:
			return c, errors.New("template strategi tidak tersedia")
		}
		// Several pairs share one binding (and its instance cash); grid allows only one.
		symbols := strings.FieldsFunc(strings.ToUpper(q.FormValue("symbol")), func(r rune) bool { return r == ',' || r == ' ' })
		c.Binding = &BindingSpec{ID: q.FormValue("binding_id"), Name: name, Symbols: symbols, Params: params, Cap: c.Amount}
	}
	return c, nil
}
