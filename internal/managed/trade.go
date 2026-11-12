package managed

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/crypto-launch-bot/internal/domain"
	"github.com/rizz404/crypto-launch-bot/internal/marketview"
)

// Trading screen refresh: the market from memory several times a second, the
// ledger (one state load) only every few seconds.
const (
	marketPush  = 300 * time.Millisecond
	accountPoll = 3 * time.Second
	bookDepth   = 14
)

func (h *HTTP) hub(mode string) *marketview.Hub {
	if h.Markets == nil {
		return nil
	}
	return h.Markets(mode)
}

// tradeView is the trading screen of one wallet and symbol.
type tradeView struct {
	view
	Symbol    string
	TF        domain.Timeframe
	Symbols   []string
	Manual    []manualScope
	Positions []positionRow
}

// manualScope is a manual binding that can trade the screen's symbol.
type manualScope struct {
	ID         string
	Cash, Hold decimal.Decimal
}

type positionRow struct {
	Owner, Symbol         string
	Qty, Avg, Mark, Value decimal.Decimal
	PnL                   decimal.Decimal
}

func (h *HTTP) tradeView(q *http.Request, wallet string, wallets []string, s *State, hub *marketview.Hub) tradeView {
	symbols := slices.Clone(hub.Symbols)
	slices.Sort(symbols)
	symbol := q.URL.Query().Get("symbol")
	if !slices.Contains(symbols, symbol) && len(symbols) > 0 {
		symbol = symbols[0]
	}
	tf := domain.Timeframe(q.URL.Query().Get("tf"))
	if !slices.Contains(marketview.ChartTimeframes, tf) {
		tf = "15m"
	}
	v := tradeView{view: h.view(wallet, wallets, "trade", s, q), Symbol: symbol, TF: tf, Symbols: symbols}
	for _, id := range sortedBindings(s) {
		b := s.Bindings[id]
		if b.Spec.Name == ManualStrategy && slices.Contains(b.Spec.Symbols, symbol) {
			v.Manual = append(v.Manual, manualScope{ID: id, Cash: s.FreeCash(b.InstanceID), Hold: s.holding(id, symbol)})
		}
	}
	for _, p := range s.Holdings {
		if !p.Quantity.IsPositive() {
			continue
		}
		mark := s.Prices[p.Symbol]
		r := positionRow{Owner: p.Owner, Symbol: p.Symbol, Qty: p.Quantity, Mark: mark, Value: p.Quantity.Mul(mark)}
		r.Avg = p.Cost.Div(p.Quantity)
		r.PnL = r.Value.Sub(p.Cost)
		v.Positions = append(v.Positions, r)
	}
	slices.SortFunc(v.Positions, func(a, b positionRow) int {
		return cmp.Or(cmp.Compare(a.Symbol, b.Symbol), cmp.Compare(a.Owner, b.Owner))
	})
	return v
}

func (v tradeView) Href(symbol string, tf domain.Timeframe) string {
	return "/trade?wallet=" + url.QueryEscape(v.Wallet) + "&symbol=" + url.QueryEscape(symbol) + "&tf=" + string(tf)
}
func (v tradeView) Live() bool                     { return v.State.Spec.Mode == "live" }
func (v tradeView) Timeframes() []domain.Timeframe { return marketview.ChartTimeframes }
func (v tradeView) StreamURL() string {
	return "/stream?wallet=" + url.QueryEscape(v.Wallet) + "&symbol=" + url.QueryEscape(v.Symbol) + "&tf=" + string(v.TF)
}
func (v tradeView) CandlesURL() string {
	return "/api/candles?wallet=" + url.QueryEscape(v.Wallet) + "&symbol=" + url.QueryEscape(v.Symbol) + "&tf=" + string(v.TF)
}

// OpenOrders are the non-terminal orders, newest first.
func (v tradeView) OpenOrders() []*Order {
	var out []*Order
	for _, o := range v.Orders() {
		if o.State != "terminal" {
			out = append(out, o)
		}
	}
	slices.Reverse(out)
	return out
}

// Cancellable mirrors Engine.Cancel: protective legs are closed with the position.
func (v tradeView) Cancellable(o *Order) bool {
	return o.State == "open" && (o.Order.Tag == domain.TagStrategy || o.Order.Tag == domain.TagEntry)
}

func (v tradeView) Fills() []Execution {
	rows, _ := v.Executions()
	return rows[:min(len(rows), 100)]
}

// Request is a fresh idempotency key for one rendered form.
func (v tradeView) Request() string { return nonce() }

func dec(d decimal.Decimal) string {
	if d.IsZero() {
		return "0"
	}
	return d.Round(8).String()
}

type chartCandle struct {
	Time                   int64           `json:"time"`
	Open, High, Low, Close decimal.Decimal `json:"-"`
}

// MarshalJSON emits numbers, which lightweight-charts requires.
func (c chartCandle) MarshalJSON() ([]byte, error) {
	f := func(d decimal.Decimal) float64 { v, _ := d.Float64(); return v }
	return json.Marshal(map[string]any{"time": c.Time, "open": f(c.Open), "high": f(c.High), "low": f(c.Low), "close": f(c.Close)})
}

type chartMarker struct {
	Time     int64  `json:"time"`
	Position string `json:"position"`
	Shape    string `json:"shape"`
	Color    string `json:"color"`
	Text     string `json:"text"`
}

func toChart(c domain.Candle) chartCandle {
	return chartCandle{Time: c.OpenTime.Unix(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close}
}

// candles returns chart history from the hub (REST once, then the stream)
// and the wallet's own fills on the symbol as markers.
func (h *HTTP) candles(w http.ResponseWriter, q *http.Request, v tradeView, hub *marketview.Hub) {
	bars, err := hub.History(q.Context(), v.Symbol, v.TF, 500)
	if err != nil {
		http.Error(w, "Candle tidak tersedia: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	out := struct {
		Candles []chartCandle `json:"candles"`
		Markers []chartMarker `json:"markers"`
	}{Candles: []chartCandle{}, Markers: []chartMarker{}}
	for _, c := range bars {
		out.Candles = append(out.Candles, toChart(c))
	}
	if len(bars) > 0 {
		first := bars[0].OpenTime
		for _, e := range v.State.Executions {
			if e.Fill.Symbol != v.Symbol || e.Fill.Time.Before(first) {
				continue
			}
			m := chartMarker{Time: e.Fill.Time.Truncate(v.TF.Duration()).Unix(), Position: "belowBar", Shape: "arrowUp", Color: "pos", Text: "B " + dec(e.Fill.Quantity)}
			if e.Fill.Side == domain.Sell {
				m.Position, m.Shape, m.Color, m.Text = "aboveBar", "arrowDown", "neg", "S "+dec(e.Fill.Quantity)
			}
			out.Markers = append(out.Markers, m)
		}
		slices.SortStableFunc(out.Markers, func(a, b chartMarker) int { return cmp.Compare(a.Time, b.Time) })
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type marketPayload struct {
	Synced bool         `json:"synced"`
	Bids   [][2]string  `json:"bids"`
	Asks   [][2]string  `json:"asks"`
	Trades []tapeRow    `json:"trades"`
	Ticker *tickerRow   `json:"ticker,omitempty"`
	Candle *chartCandle `json:"candle,omitempty"`
	Seq    int64        `json:"seq"`
}
type tapeRow struct {
	Price string `json:"p"`
	Qty   string `json:"q"`
	Time  int64  `json:"t"`
	Sell  bool   `json:"s"`
}
type tickerRow struct {
	Last      string `json:"last"`
	ChangePct string `json:"pct"`
	High      string `json:"high"`
	Low       string `json:"low"`
	QuoteVol  string `json:"qv"`
}

// accountVersion changes whenever the screen's account panels would.
func accountVersion(s *State) string {
	orders := make(map[string]string, len(s.Orders))
	for id, o := range s.Orders {
		orders[id] = o.State + dec(o.Order.FilledQty)
	}
	holdings := make(map[string]string, len(s.Holdings))
	for k, p := range s.Holdings {
		holdings[k] = dec(p.Quantity)
	}
	return Hash(struct {
		E, H      int
		O, P      map[string]string
		Halted    bool
		Attention string
	}{len(s.Executions), len(s.History), orders, holdings, s.Halted, s.Attention})
}

// stream pushes market updates (from memory) and account change notices as
// server-sent events. Browsers never cause exchange requests here.
func (h *HTTP) stream(w http.ResponseWriter, q *http.Request, rt *Runtime, v tradeView, hub *marketview.Hub) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming tidak didukung", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no") // nginx must not buffer events
	ctx := q.Context()
	send := func(event string, payload any) bool {
		b, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
		return err == nil
	}
	var since int64
	version := accountVersion(v.State)
	market, account := time.NewTicker(marketPush), time.NewTicker(accountPoll)
	defer market.Stop()
	defer account.Stop()
	// First trades: the latest tape only, not the whole buffer per reconnect.
	if t := hub.View(v.Symbol, v.TF, 0, 0).Trades; len(t) > 30 {
		since = t[len(t)-31].Seq
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-market.C:
			mv := hub.View(v.Symbol, v.TF, bookDepth, since)
			p := marketPayload{Synced: mv.Synced, Bids: [][2]string{}, Asks: [][2]string{}, Trades: []tapeRow{}}
			for _, l := range mv.Bids {
				p.Bids = append(p.Bids, [2]string{dec(l.Price), dec(l.Qty)})
			}
			for _, l := range mv.Asks {
				p.Asks = append(p.Asks, [2]string{dec(l.Price), dec(l.Qty)})
			}
			for _, t := range mv.Trades {
				p.Trades = append(p.Trades, tapeRow{dec(t.Price), dec(t.Qty), t.Time.UnixMilli(), t.BuyerMaker})
				since = t.Seq
			}
			if mv.Ticker.Symbol != "" {
				p.Ticker = &tickerRow{dec(mv.Ticker.Last), mv.Ticker.ChangePct.StringFixed(2), dec(mv.Ticker.High), dec(mv.Ticker.Low), mv.Ticker.QuoteVol.StringFixed(0)}
			}
			if !mv.Candle.OpenTime.IsZero() {
				c := toChart(mv.Candle)
				p.Candle = &c
			}
			p.Seq = since
			if !send("market", p) {
				return
			}
		case <-account.C:
			s, err := rt.Snapshot(ctx)
			if err != nil {
				continue
			}
			if nv := accountVersion(s); nv != version {
				version = nv
				if !send("account", map[string]string{"v": nv}) {
					return
				}
			}
		}
	}
}
