// Command backtest-year compares existing spot strategies on verified archive CSVs.
// Run download.py first, then: go run ./scripts/backtest-year -data /tmp/trading-bot-year
// It uses the existing backtest engine and has no exchange client or trading credentials.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/rizz404/trading-bot/internal/backtest"
	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
	"github.com/shopspring/decimal"
)

type source map[domain.Timeframe][]domain.Candle

func (s source) Candles(_ context.Context, k domain.SeriesKey, from, to time.Time) ([]domain.Candle, error) {
	cs := s[k.Timeframe]
	if len(cs) == 0 {
		return nil, fmt.Errorf("missing timeframe %s", k.Timeframe)
	}
	if cs[0].OpenTime.Before(from) || !cs[len(cs)-1].OpenTime.Before(to) {
		return nil, fmt.Errorf("unexpected date range")
	}
	return cs, nil
}

type issues struct {
	Counts  map[string]int
	Samples []string
}

func (h *issues) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelWarn }
func (h *issues) Handle(_ context.Context, r slog.Record) error {
	h.Counts[r.Message]++
	if len(h.Samples) < 8 {
		s := r.Message
		r.Attrs(func(a slog.Attr) bool { s += " " + a.String(); return true })
		h.Samples = append(h.Samples, s)
	}
	return nil
}
func (h *issues) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *issues) WithGroup(_ string) slog.Handler      { return h }

type row struct {
	Symbol, Strategy, Status, Reason string
	Implementation                   string
	Params                           map[string]any    `json:",omitempty"`
	Metrics                          *backtest.Metrics `json:",omitempty"`
	FinalIDR, ProfitIDR, Seconds     float64
	OpenPositions, Candles           int
	Issues                           *issues `json:",omitempty"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func dec(s string) decimal.Decimal { v, e := decimal.NewFromString(s); must(e); return v }
func load(path, symbol string) source {
	f, e := os.Open(path)
	must(e)
	defer func() { must(f.Close()) }()
	r := csv.NewReader(f)
	cs := make([]domain.Candle, 0, 525600)
	for {
		v, e := r.Read()
		if e == io.EOF {
			break
		}
		must(e)
		if len(v) != 8 {
			panic("bad CSV")
		}
		sec, e := strconv.ParseInt(v[0], 10, 64)
		must(e)
		tr, e := strconv.ParseInt(v[7], 10, 64)
		must(e)
		c := domain.Candle{Exchange: "binance", Market: domain.Spot, Symbol: symbol, Timeframe: "1m", OpenTime: time.Unix(sec, 0).UTC(), Open: dec(v[1]), High: dec(v[2]), Low: dec(v[3]), Close: dec(v[4]), Volume: dec(v[5]), QuoteVolume: dec(v[6]), Trades: tr, Closed: true}
		if len(cs) > 0 && !c.OpenTime.Equal(cs[len(cs)-1].OpenTime.Add(time.Minute)) {
			panic("non-contiguous candles")
		}
		cs = append(cs, c)
	}
	if len(cs) != 525600 {
		panic(fmt.Sprintf("expected full year: %s has %d", symbol, len(cs)))
	}
	return source{"1m": cs, "1h": aggregate(cs, "1h", 60), "4h": aggregate(cs, "4h", 240), "1d": aggregate(cs, "1d", 1440)}
}

func aggregate(cs []domain.Candle, tf domain.Timeframe, n int) []domain.Candle {
	if len(cs)%n != 0 {
		panic("incomplete aggregate candle")
	}
	hs := make([]domain.Candle, 0, len(cs)/n)
	for i := 0; i < len(cs); i += n {
		a := cs[i]
		a.Timeframe = tf
		if !a.OpenTime.Equal(tf.Truncate(a.OpenTime)) {
			panic("unaligned aggregate candle")
		}
		for j := i + 1; j < i+n; j++ {
			c := cs[j]
			a.High = decimal.Max(a.High, c.High)
			a.Low = decimal.Min(a.Low, c.Low)
			a.Close = c.Close
			a.Volume = a.Volume.Add(c.Volume)
			a.QuoteVolume = a.QuoteVolume.Add(c.QuoteVolume)
			a.Trades += c.Trades
		}
		hs = append(hs, a)
	}
	return hs
}

type candidate struct {
	ID, Name string
	Params   map[string]any
}

// These five hypotheses are fixed before running; no search or per-coin tuning.
func candidates() []candidate {
	return []candidate{
		{"ema4h-trend", "emacross", map[string]any{"timeframe": "4h", "fast": 20, "slow": 50, "trend_ema": 200, "rsi_period": 14, "rsi_long_max": 70, "rsi_short_min": 30, "atr_period": 14, "stop_atr": 3, "target_atr": 0, "trail_atr": 3, "allow_short": false, "exit_on_cross": true}},
		{"ema1d-trend", "emacross", map[string]any{"timeframe": "1d", "fast": 20, "slow": 50, "trend_ema": 100, "rsi_period": 14, "rsi_long_max": 70, "rsi_short_min": 30, "atr_period": 14, "stop_atr": 3, "target_atr": 0, "trail_atr": 3, "allow_short": false, "exit_on_cross": true}},
		{"breakout1d", "trendspot", map[string]any{"mode": "breakout", "timeframe": "1d", "trend_period": 100, "entry_period": 55, "exit_period": 20, "atr_period": 14, "stop_atr": 3, "target_atr": 0, "trail_atr": 3}},
		{"momentum1d", "trendspot", map[string]any{"mode": "momentum", "timeframe": "1d", "trend_period": 100, "entry_period": 90, "exit_period": 20, "atr_period": 14, "stop_atr": 3, "target_atr": 0, "trail_atr": 3, "momentum_min": 0.05}},
		{"pullback4h", "trendspot", map[string]any{"mode": "pullback", "timeframe": "4h", "trend_period": 200, "entry_period": 55, "exit_period": 20, "atr_period": 14, "rsi_period": 14, "rsi_entry": 40, "rsi_exit": 65, "stop_atr": 2.5, "target_atr": 5, "trail_atr": 0}},
	}
}

func main() {
	data := flag.String("data", "/tmp/trading-bot-year", "verified archive CSV directory")
	out := flag.String("out", "docs/testing/backtest-2026-10-19", "report directory")
	suite := flag.String("suite", "baseline", "baseline or candidates")
	declare := flag.Bool("declare", false, "write candidate specification and exit before simulations")
	job := flag.String("job", "", "execute one queued research job from JSON")
	result := flag.String("result", "", "write the queued job result to JSON")
	flag.Parse()
	if *job != "" {
		must(runJob(*job, *result))
		return
	}
	if *suite != "baseline" && *suite != "candidates" {
		panic("unknown suite")
	}
	if *suite == "candidates" && *out == "docs/testing/backtest-2026-10-19" {
		*out = "docs/testing/candidates-2026-10-19"
	}
	must(os.MkdirAll(*out, 0755))
	spec, err := json.MarshalIndent(candidates(), "", "  ")
	must(err)
	if *declare {
		must(os.WriteFile(filepath.Join(*out, "candidate-spec.json"), spec, 0644))
		return
	}
	if *suite == "candidates" {
		locked, e := os.ReadFile(filepath.Join(*out, "candidate-spec.json"))
		must(e)
		if string(locked) != string(spec) {
			panic("candidate specification changed after declaration")
		}
	}
	start := time.Date(2025, 10, 19, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)
	const fx = 18052.0
	balance := decimal.NewFromInt(2000000).Div(decimal.NewFromFloat(fx))
	var info struct {
		Symbols []struct {
			Symbol, BaseAsset, QuoteAsset, Status string
			Filters                               []map[string]json.RawMessage
		}
	}
	b, e := os.ReadFile(filepath.Join(*data, "instruments.json"))
	must(e)
	must(json.Unmarshal(b, &info))
	instruments := map[string]domain.Instrument{}
	for _, s := range info.Symbols {
		in := domain.Instrument{Exchange: "binance", Market: domain.Spot, Symbol: s.Symbol, Base: s.BaseAsset, Quote: s.QuoteAsset, Status: s.Status, Tradable: s.Status == "TRADING", ContractSize: decimal.NewFromInt(1), MaxLeverage: 1}
		for _, f := range s.Filters {
			get := func(k string) string { var v string; must(json.Unmarshal(f[k], &v)); return v }
			switch get("filterType") {
			case "PRICE_FILTER":
				in.TickSize = dec(get("tickSize"))
			case "LOT_SIZE":
				in.StepSize = dec(get("stepSize"))
				in.MinQty = dec(get("minQty"))
			case "MIN_NOTIONAL", "NOTIONAL":
				in.MinNotional = dec(get("minNotional"))
			}
		}
		if !in.TickSize.IsPositive() || !in.StepSize.IsPositive() || !in.MinNotional.IsPositive() {
			panic("missing rules")
		}
		instruments[s.Symbol] = in
	}
	var rows []row
	var benchmarks []map[string]any
	allStart := time.Now()
	save := func() {
		v := map[string]any{"from": start, "to_exclusive": end, "initial_idr_each": 2000000, "usd_idr_fixed": fx, "initial_usdt_each": balance, "market": "Binance Spot", "assumptions": []string{"USDT = USD; fixed conversion at BI USD midpoint 2026-10-19 (18142.26 + 17961.74)/2; IDR FX fluctuations excluded", "Spot fees maker/taker 0.1%; market/stop slippage 2 bps; no tax or withdrawal fees", "Current public exchange rules, not historical snapshots; no parameter optimization on test period", "EMA 1h signals with 1m execution; grid/DCA/MM 1m; cold-start indicator warmup within tested year", "Default risk: 1% per trade, daily kill 5%, peak drawdown kill 20%, close on kill; no automatic reset", "Grid fixed initial-open +/-15%, 10 levels, 10 USDT/level, stop 3% below lower boundary; does not restart after stop", "OHLC limit fills omit queue position, liquidity and latency; MM/grid results are approximate and potentially optimistic", "Final equity includes open inventory marked at final close; closing fees on remaining inventory not deducted", "Fees metric includes closed trades only; entry fees on open inventory affect equity but may not appear in metric", "Five coins selected from current top ten, not a historical-universe study; selection/survivorship bias applies"}, "sources": map[string]string{"prices": "https://data.binance.vision/", "instrument_rules": "https://data-api.binance.vision/api/v3/exchangeInfo", "fx": "https://www.bi.go.id/en/statistik/informasi-kurs/transaksi-bi/transaksi-bi/default.aspx", "ranking": "https://coinucation.com/coins"}, "results": rows, "buy_and_hold_net_estimate": benchmarks, "elapsed_seconds": time.Since(allStart).Seconds()}
		v["suite"] = *suite
		if *suite == "candidates" {
			v["candidate_spec_sha256"] = fmt.Sprintf("%x", sha256.Sum256(spec))
			v["candidates"] = candidates()
			v["assumptions"] = []string{"Same dates, symbols, initial capital, fee/slippage model and risk limits as baseline; no leverage or parameter search", "Signals use fully closed 4h/1d bars aggregated from the same 1m CSV; fills still use every 1m bar", "Cold start within tested year; 200x4h/100x1d trend warmup holds cash before signals; no pre-period warmup", "Five candidates declared before running and shared across all five coins; this is exploratory on a previously inspected year, NOT untouched out-of-sample evidence", "Spot maker/taker 0.1%, market/stop slippage 2 bps; USDT=USD; fixed Rp18052/USD; no taxes", "Default risk unchanged: 1% per trade, max position 25% of equity, daily kill 5%, peak drawdown kill 20%, close on kill, no automatic reset", "Current instrument rules approximate historical rules; no order-book queue, latency or liquidity simulation", "Final equity marks open inventory at last close; unclosed inventory has no final liquidation fee deducted; fee metric covers closed trades only", "Zero or very few trades is insufficient evidence of profitability; benchmark cash earns 0%", "Results must be validated on untouched dates and paper trading before any live decision"}
		}
		b, e := json.MarshalIndent(v, "", "  ")
		must(e)
		must(os.WriteFile(filepath.Join(*out, "results.json"), b, 0644))
	}
	for _, sym := range []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "XRPUSDT", "SOLUSDT"} {
		t := time.Now()
		src := load(filepath.Join(*data, sym+".csv"), sym)
		fmt.Printf("loaded %s in %.2fs\n", sym, time.Since(t).Seconds())
		first, _ := src["1m"][0].Open.Float64()
		last, _ := src["1m"][len(src["1m"])-1].Close.Float64()
		hold := (last * (1 - 0.0002) * (1 - 0.001)) / (first * (1 + 0.0002) * (1 + 0.001))
		benchmarks = append(benchmarks, map[string]any{"symbol": sym, "return_pct": (hold - 1) * 100, "final_idr": 2000000 * hold})
		params := map[string]map[string]any{
			"emacross": {"timeframe": "1h", "fast": 12, "slow": 26, "allow_short": false},
			"grid":     {"timeframe": "1m", "lower": first * 0.85, "upper": first * 1.15, "levels": 10, "quote_per_level": 10.0, "stop_below": first * 0.85 * 0.97},
			"dca":      {"timeframe": "1m", "interval": "24h", "quote_amount": 10.0, "max_buys": 10, "dip_pct": 0.02, "take_profit": 0.03},
			"mm":       {"spread_bps": 20.0, "order_quote": 10.0, "max_inventory_quote": 50.0, "skew_bps": 10.0, "requote_bps": 5.0, "min_requote": "5s"},
		}
		cases := []candidate{}
		for _, name := range []string{"emacross", "grid", "dca", "mm"} {
			cases = append(cases, candidate{name, name, params[name]})
		}
		if *suite == "candidates" {
			cases = candidates()
		}
		for _, test := range cases {
			name := test.ID
			t := time.Now()
			is := &issues{Counts: map[string]int{}}
			sc := strategy.Config{ID: name + "-" + sym, Name: test.Name, Exchange: "binance", Market: domain.Spot, Symbols: []string{sym}, Leverage: 1, Params: test.Params}
			res, err := backtest.Run(context.Background(), backtest.Config{Strategy: sc, From: start, To: end, InitialBalance: balance, Fees: map[domain.Market]broker.Fees{domain.Spot: {Maker: 0.001, Taker: 0.001}}, SlippageBps: 2, ExecTimeframe: "1m", Risk: risk.DefaultConfig(), Instruments: []domain.Instrument{instruments[sym]}, Log: slog.New(is)}, src)
			r := row{Symbol: sym, Strategy: name, Implementation: test.Name, Status: "completed", Params: test.Params, Seconds: time.Since(t).Seconds(), Issues: is}
			if err != nil {
				r.Status = "failed"
				r.Reason = err.Error()
			} else {
				r.Metrics = &res.Metrics
				r.FinalIDR = res.Metrics.FinalEquity * fx
				r.ProfitIDR = r.FinalIDR - 2000000
				r.OpenPositions = len(res.OpenPositions)
				r.Candles = res.Candles
				// Keep hourly equity in exported CSV; matching still uses every minute.
				var hourly []backtest.EquityPoint
				for i, p := range res.Equity {
					if p.Time.Minute() == 0 || i == len(res.Equity)-1 {
						hourly = append(hourly, p)
					}
				}
				res.Equity = hourly
				must(backtest.WriteCSV(*out, sc.ID, res))
				fmt.Printf("%s %s: return %.2f%% final Rp%.0f trades %d took %.2fs\n", sym, name, res.Metrics.TotalReturn*100, r.FinalIDR, res.Metrics.Trades, r.Seconds)
			}
			rows = append(rows, r)
			save()
			runtime.GC()
		}
		if *suite == "baseline" {
			for _, name := range []string{"funding", "xspread"} {
				reason := "Historical funding is not supplied to strategy context by backtest.Run; no meaningful funding test is possible with this runner"
				if name == "xspread" {
					reason = "Requires synchronized futures history and margin on two exchanges; public Bybit API connection reset in this environment; Binance Spot data cannot substitute"
				}
				rows = append(rows, row{Symbol: sym, Strategy: name, Status: "unsupported", Reason: reason})
			}
		}
		save()
	}
	fmt.Printf("all simulations took %.2fs (excluding archive download)\n", time.Since(allStart).Seconds())
}
