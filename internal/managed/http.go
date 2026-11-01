package managed

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/rizz404/trading-bot/internal/marketview"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/ui"
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
	// NavLinks are extra header links, for surfaces that embed the dashboard.
	NavLinks []ui.NavItem
	Wallets  map[string]*Runtime
	Password string
	Money    func(decimal.Decimal) string
	// TOTP is the base32 secret of the login and live-order codes; empty keeps basic auth.
	TOTP string
	// Markets returns the market hub of a wallet mode (paper and live share mainnet).
	Markets  func(mode string) *marketview.Hub
	token    string
	sessions sessions
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
	case "/logout":
		h.logout(w, q)
		return
	}
	if !h.authorized(q) {
		switch {
		case h.TOTP == "":
			w.Header().Set("WWW-Authenticate", `Basic realm="Trading bot"`)
			http.Error(w, "Autentikasi diperlukan", http.StatusUnauthorized)
		case q.Method == http.MethodGet && !strings.HasPrefix(q.URL.Path, "/api/") && q.URL.Path != "/stream":
			http.Redirect(w, q, "/login", http.StatusSeeOther)
		default:
			http.Error(w, "Autentikasi diperlukan", http.StatusUnauthorized)
		}
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
		code := q.Header.Get("X-TOTP")
		if code == "" {
			code = q.FormValue("totp")
		}
		if !h.liveConfirmed(st.Spec.Mode, code) {
			err = errors.New("akun live: kode authenticator (TOTP) baru diperlukan")
			if strings.HasPrefix(q.Header.Get("Content-Type"), "application/json") {
				http.Error(w, err.Error(), http.StatusForbidden)
			} else {
				formError(w, q, wallet, err)
			}
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
		http.Redirect(w, q, backURL(q, wallet), http.StatusSeeOther)
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
	links := h.NavLinks
	if h.Markets != nil {
		links = append([]ui.NavItem{{Key: "trade", Label: "Trading", Href: "/trade?wallet=" + url.QueryEscape(wallet)}}, links...)
	}
	v := view{Wallet: wallet, Wallets: wallets, Tab: tab, State: state, Token: h.token, Error: q.URL.Query().Get("error"), money: h.Money, links: links}
	if h.RateLine != nil {
		v.Rate = h.RateLine()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = Dashboard(v).Render(q.Context(), w)
}

// formError returns a rejected browser action to the dashboard, where the
// reason is shown, instead of a bare text page. JSON clients keep status codes.
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
		case ManualStrategy:
			params = nil
		default:
			return c, errors.New("template strategi tidak tersedia")
		}
		c.Binding = &BindingSpec{ID: q.FormValue("binding_id"), Name: name, Symbols: []string{q.FormValue("symbol")}, Params: params, Cap: c.Amount}
	}
	return c, nil
}
