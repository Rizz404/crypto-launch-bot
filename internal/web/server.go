// Package web serves the dashboard: live session state, charts with trade
// markers and saved backtests. It is rendered with templ and HTMX.
package web

import (
	"cmp"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/rizz404/trading-bot/internal/web/views"
)

//go:embed static
var staticFS embed.FS

// Live is the running trading session, if any.
type Live interface {
	Mode() string
	RunID() int64
	Accounts() []*portfolio.Account
	OpenOrders() []domain.Order
	Strategies() []strategy.Config
	LastPrice(exchange string, m domain.Market, symbol string) (decimal.Decimal, bool)
	Halted() (bool, string)
	Paused() bool
	SetPaused(bool)
	Kill(ctx context.Context, reason string)
	ResetKillSwitch()
}

type Server struct {
	store    *storage.Store
	live     Live // nil when the bot only records market data
	password string
	log      *slog.Logger
}

// New returns a dashboard server. With an empty password there is no
// authentication; callers must then bind to localhost only.
func New(store *storage.Store, live Live, password string, log *slog.Logger) *Server {
	return &Server{store: store, live: live, password: password, log: log.With("component", "web")}
}

// Handler returns the HTTP handler with authentication applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	mux.HandleFunc("GET /{$}", s.overview)
	mux.HandleFunc("GET /partials/status", s.partialStatus)
	mux.HandleFunc("GET /partials/positions", s.partialPositions)
	mux.HandleFunc("GET /partials/orders", s.partialOrders)
	mux.HandleFunc("GET /partials/trades", s.partialTrades)
	mux.HandleFunc("POST /actions/{action}", s.action)
	mux.HandleFunc("GET /api/equity", s.apiEquity)
	mux.HandleFunc("GET /api/candles", s.apiCandles)
	mux.HandleFunc("GET /chart", s.chart)
	mux.HandleFunc("GET /backtests", s.backtests)
	mux.HandleFunc("GET /backtests/{id}", s.backtest)
	return s.auth(securityHeaders(mux))
}

func (s *Server) auth(next http.Handler) http.Handler {
	if s.password == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(s.password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="trading-bot", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		// lightweight-charts 5.2.1 injects a fixed attribution stylesheet.
		// Allow that exact style; keep scripts and other inline styles blocked.
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'sha256-3pRED1tOXas1FXFoPb9TGCjmYe9XQsmO9OV23khV2nY='; script-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		s.log.Warn("render failed", "path", r.URL.Path, "err", err)
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Warn("encode json", "err", err)
	}
}

// --- live overview ---

func (s *Server) status() views.Status {
	if s.live == nil {
		return views.Status{}
	}
	st := views.Status{Mode: s.live.Mode(), Live: true, Paused: s.live.Paused()}
	st.Killed, st.KillReason = s.live.Halted()
	var eq, unreal, expo decimal.Decimal
	for _, a := range s.live.Accounts() {
		snap := a.Snapshot()
		eq, unreal, expo = eq.Add(snap.Equity), unreal.Add(snap.UnrealizedPnL), expo.Add(snap.Exposure)
		st.OpenPositions += snap.OpenPositions
		st.Accounts = append(st.Accounts, views.AccountRow{
			Key: a.Key.String(), Balance: money(snap.Balance), Equity: money(snap.Equity),
			Unrealized: money(snap.UnrealizedPnL), FreeMargin: money(snap.FreeMargin()), Positions: snap.OpenPositions,
		})
	}
	st.Equity, st.Unrealized, st.Exposure = money(eq), money(unreal), money(expo)
	st.UnrealizedPos = !unreal.IsNegative()
	return st
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, views.Overview(s.status()))
}

func (s *Server) partialStatus(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, views.StatusPanel(s.status()))
}

func (s *Server) partialPositions(w http.ResponseWriter, r *http.Request) {
	if s.live == nil {
		s.render(w, r, views.Empty("No trading session."))
		return
	}
	var rows []views.PositionRow
	for _, a := range s.live.Accounts() {
		for _, p := range a.Positions() {
			mark, ok := s.live.LastPrice(p.Exchange, p.Market, p.Symbol)
			if !ok {
				mark = p.EntryPrice
			}
			u := p.UnrealizedPnL(mark)
			rows = append(rows, views.PositionRow{
				Account: a.Key.String(), Symbol: p.Symbol, Side: string(p.Side), Qty: p.Quantity.String(),
				Entry: price(p.EntryPrice), Mark: price(mark), UPnL: money(u), UPnLPositive: !u.IsNegative(),
				Strategy: p.Strategy, Opened: ts(p.OpenedAt),
				ChartHref: chartHref(domain.SeriesKey{Exchange: p.Exchange, Market: p.Market, Symbol: p.Symbol, Timeframe: "1h"}, 0),
			})
		}
	}
	s.render(w, r, views.Positions(rows))
}

func (s *Server) partialOrders(w http.ResponseWriter, r *http.Request) {
	if s.live == nil {
		s.render(w, r, views.Empty("No trading session."))
		return
	}
	orders := s.live.OpenOrders()
	slices.SortFunc(orders, func(a, b domain.Order) int { return a.CreatedAt.Compare(b.CreatedAt) })
	rows := make([]views.OrderRow, len(orders))
	for i, o := range orders {
		p := o.Price
		if o.Type.IsConditional() {
			p = o.StopPrice
		}
		rows[i] = views.OrderRow{
			Symbol: o.Symbol, Side: string(o.Side), Type: string(o.Type), Tag: o.Tag, Qty: o.Quantity.String(),
			Price: price(p), Strategy: o.Strategy, Status: string(o.Status), Created: ts(o.CreatedAt),
		}
	}
	s.render(w, r, views.Orders(rows))
}

func (s *Server) partialTrades(w http.ResponseWriter, r *http.Request) {
	trades, err := s.store.RecentTrades(r.Context(), 50)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, views.Trades(tradeRows(trades, 0)))
}

func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	// HTMX always sends this header; a cross-site form cannot (CSRF guard).
	if r.Header.Get("HX-Request") != "true" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if s.live == nil {
		http.Error(w, "no trading session", http.StatusConflict)
		return
	}
	action := r.PathValue("action")
	s.log.Warn("dashboard action", "action", action, "remote", r.RemoteAddr)
	switch action {
	case "pause":
		s.live.SetPaused(true)
	case "resume":
		if killed, _ := s.live.Halted(); !killed {
			s.live.SetPaused(false)
		}
	case "kill":
		s.live.Kill(r.Context(), "web dashboard")
	case "reset":
		s.live.ResetKillSwitch()
	default:
		http.NotFound(w, r)
		return
	}
	s.render(w, r, views.StatusPanel(s.status()))
}

// --- charts ---

type point struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

func (s *Server) apiEquity(w http.ResponseWriter, r *http.Request) {
	runID, _ := strconv.ParseInt(r.URL.Query().Get("run"), 10, 64)
	if runID == 0 && s.live != nil {
		runID = s.live.RunID()
	}
	pts, err := s.store.EquitySeries(r.Context(), runID, time.Time{})
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]point, 0, len(pts))
	for _, p := range pts {
		v, _ := p.Equity.Float64()
		out = append(out, point{Time: p.Time.Unix(), Value: v})
	}
	s.writeJSON(w, out)
}

type candleJSON struct {
	Time  int64   `json:"time"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

type markerJSON struct {
	Time     int64  `json:"time"`
	Position string `json:"position"`
	Shape    string `json:"shape"`
	Color    string `json:"color"` // pos | neg | accent, resolved to theme colors in app.js
	Text     string `json:"text"`
}

const maxBars = 10_000

func parseSeries(v string) (domain.SeriesKey, error) {
	parts := strings.Split(v, ":")
	if len(parts) != 4 {
		return domain.SeriesKey{}, fmt.Errorf("series must be exchange:market:symbol:timeframe")
	}
	m, err := domain.ParseMarket(parts[1])
	if err != nil {
		return domain.SeriesKey{}, err
	}
	tf, err := domain.ParseTimeframe(parts[3])
	if err != nil {
		return domain.SeriesKey{}, err
	}
	return domain.SeriesKey{Exchange: parts[0], Market: m, Symbol: parts[2], Timeframe: tf}, nil
}

func chartHref(k domain.SeriesKey, runID int64) string {
	h := "/chart?series=" + k.String()
	if runID > 0 {
		h += "&run=" + strconv.FormatInt(runID, 10)
	}
	return h
}

func (s *Server) apiCandles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	key, err := parseSeries(q.Get("series"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runID, _ := strconv.ParseInt(q.Get("run"), 10, 64)
	to := time.Now()
	from := to.Add(-500 * key.Timeframe.Duration())
	if runID > 0 {
		run, err := s.store.GetRun(r.Context(), runID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if run != nil && run.Mode == "backtest" {
			from = run.StartedAt
			if t, ok := run.Params["to"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
					to = parsed
				}
			}
		}
	}
	if from.Before(to.Add(-maxBars * key.Timeframe.Duration())) {
		from = to.Add(-maxBars * key.Timeframe.Duration())
	}

	cs, err := s.store.Candles(r.Context(), key, from, to)
	if err != nil {
		s.fail(w, err)
		return
	}
	trades, err := s.store.SymbolTrades(r.Context(), key.Exchange, key.Market, key.Symbol, from, to, runID)
	if err != nil {
		s.fail(w, err)
		return
	}

	out := struct {
		Candles []candleJSON `json:"candles"`
		Markers []markerJSON `json:"markers"`
	}{Candles: make([]candleJSON, len(cs)), Markers: []markerJSON{}}
	for i, c := range cs {
		f := func(d decimal.Decimal) float64 { v, _ := d.Float64(); return v }
		out.Candles[i] = candleJSON{Time: c.OpenTime.Unix(), Open: f(c.Open), High: f(c.High), Low: f(c.Low), Close: f(c.Close)}
	}
	// Markers must sit on bar times.
	bar := func(t time.Time) int64 { return key.Timeframe.Truncate(t).Unix() }
	for _, t := range trades {
		entry := markerJSON{Time: bar(t.OpenedAt), Color: "accent", Text: "L " + t.EntryPrice.String()}
		if t.Side == domain.Long {
			entry.Position, entry.Shape = "belowBar", "arrowUp"
		} else {
			entry.Position, entry.Shape, entry.Text = "aboveBar", "arrowDown", "S "+t.EntryPrice.String()
		}
		exit := markerJSON{Time: bar(t.ClosedAt), Position: "aboveBar", Shape: "circle", Color: "neg",
			Text: fmt.Sprintf("%s %.2f", t.ExitReason, t.NetPnL)}
		if t.NetPnL > 0 {
			exit.Color = "pos"
		}
		if t.Side == domain.Short {
			exit.Position = "belowBar"
		}
		out.Markers = append(out.Markers, entry, exit)
	}
	slices.SortStableFunc(out.Markers, func(a, b markerJSON) int { return cmp.Compare(a.Time, b.Time) })
	s.writeJSON(w, out)
}

func (s *Server) chart(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.SeriesList(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	page := views.ChartPage{}
	for _, k := range list {
		page.Series = append(page.Series, views.Link{Label: k.String(), Href: k.String()})
	}
	selected := r.URL.Query().Get("series")
	if selected == "" && len(list) > 0 {
		selected = list[0].String()
	}
	if selected == "" {
		page.Note = "No candles recorded in the last 7 days. Run the bot or datafetch first."
		s.render(w, r, views.Chart(page))
		return
	}
	key, err := parseSeries(selected)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !slices.ContainsFunc(page.Series, func(l views.Link) bool { return l.Href == selected }) {
		page.Series = append([]views.Link{{Label: selected, Href: selected}}, page.Series...)
	}
	runID, _ := strconv.ParseInt(r.URL.Query().Get("run"), 10, 64)
	page.Selected = selected
	page.Src = "/api/candles?series=" + key.String()
	if runID > 0 {
		page.Src += "&run=" + strconv.FormatInt(runID, 10)
		page.Note = fmt.Sprintf("Showing trades of run #%d.", runID)
	}
	s.render(w, r, views.Chart(page))
}

// --- backtests ---

func (s *Server) backtests(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.Runs(r.Context(), "backtest", 200)
	if err != nil {
		s.fail(w, err)
		return
	}
	rows := make([]views.RunRow, len(runs))
	for i, run := range runs {
		rows[i] = runRow(run)
	}
	s.render(w, r, views.Backtests(rows))
}

func (s *Server) backtest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	run, err := s.store.GetRun(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if run == nil || run.Mode != "backtest" {
		http.NotFound(w, r)
		return
	}
	trades, err := s.store.RunTrades(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	m := summary(run.Summary)
	params, _ := json.MarshalIndent(run.Params, "", "  ")
	d := views.RunDetail{
		Run:       runRow(*run),
		Params:    string(params),
		Trades:    tradeRows(trades, id),
		EquitySrc: "/api/equity?run=" + strconv.FormatInt(id, 10),
		Metrics: [][2]string{
			{"Return", pct(m["TotalReturn"])}, {"CAGR", pct(m["CAGR"])}, {"Max drawdown", pct(m["MaxDrawdown"])},
			{"Sharpe", num(m["Sharpe"])}, {"Sortino", num(m["Sortino"])}, {"Trades", num0(m["Trades"])},
			{"Win rate", pct(m["WinRate"])}, {"Profit factor", num(m["ProfitFactor"])}, {"Avg trade", num(m["AvgTrade"])},
			{"Fees", num(m["TotalFees"])}, {"Funding", num(m["TotalFunding"])},
			{"Equity", num(m["InitialEquity"]) + " → " + num(m["FinalEquity"])},
		},
	}
	tf := "1h"
	if p, ok := run.Params["params"].(map[string]any); ok {
		if v, ok := p["timeframe"].(string); ok {
			tf = v
		}
	}
	for _, sym := range run.Symbols {
		k := domain.SeriesKey{Exchange: run.Exchange, Market: run.Market, Symbol: sym, Timeframe: domain.Timeframe(tf)}
		d.Charts = append(d.Charts, views.Link{Label: sym + " " + tf, Href: chartHref(k, id)})
	}
	s.render(w, r, views.Backtest(d))
}

func runRow(run storage.Run) views.RunRow {
	m := summary(run.Summary)
	seg, _ := run.Params["segment"].(string)
	period := run.StartedAt.Format(time.DateOnly)
	if t, ok := run.Params["to"].(string); ok && len(t) >= 10 {
		period += " → " + t[:10]
	}
	ret, _ := m["TotalReturn"].(float64)
	return views.RunRow{
		ID: run.ID, Href: "/backtests/" + strconv.FormatInt(run.ID, 10), Instance: run.InstanceID,
		Symbols: strings.Join(run.Symbols, ", "), Segment: seg, Period: period, Started: ts(run.StartedAt),
		Return: pct(m["TotalReturn"]), MaxDD: pct(m["MaxDrawdown"]), Sharpe: num(m["Sharpe"]),
		Trades: num0(m["Trades"]), WinRate: pct(m["WinRate"]), ProfitFactor: num(m["ProfitFactor"]), Positive: ret >= 0,
	}
}

func tradeRows(trades []storage.TradeRow, runID int64) []views.TradeRow {
	rows := make([]views.TradeRow, len(trades))
	for i, t := range trades {
		rows[i] = views.TradeRow{
			Closed: ts(t.ClosedAt), Strategy: t.Strategy, Symbol: t.Symbol, Side: string(t.Side), Qty: t.Quantity.String(),
			Entry: price(t.EntryPrice), Exit: price(t.ExitPrice), Net: fmt.Sprintf("%.2f", t.NetPnL), Fees: money(t.Fees),
			Reason: t.ExitReason, Win: t.NetPnL > 0,
			ChartHref: chartHref(domain.SeriesKey{Exchange: t.Exchange, Market: t.Market, Symbol: t.Symbol, Timeframe: "1h"}, runID),
		}
	}
	return rows
}

func summary(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func money(d decimal.Decimal) string { return d.StringFixed(2) }

func price(d decimal.Decimal) string {
	if d.Abs().LessThan(decimal.NewFromInt(10)) {
		return d.StringFixed(6)
	}
	return d.StringFixed(2)
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04")
}

func pct(v any) string {
	f, ok := v.(float64)
	if !ok {
		return "–"
	}
	return fmt.Sprintf("%.2f%%", f*100)
}

func num(v any) string {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) {
		return "–"
	}
	return fmt.Sprintf("%.2f", f)
}

func num0(v any) string {
	f, ok := v.(float64)
	if !ok {
		return "–"
	}
	return strconv.Itoa(int(f))
}
