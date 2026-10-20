// portfolio-backtest runs several strategies against one shared spot capital
// pool. CSVs are merged chronologically, never evaluated as independent budgets.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/strategy"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
	"github.com/shopspring/decimal"
)

type input struct {
	file     *os.File
	reader   *csv.Reader
	symbol   string
	next     *domain.Candle
	previous time.Time
	rows     int
}

func (i *input) advance(allowGaps bool) error {
	for {
		row, err := i.reader.Read()
		i.rows++
		if errors.Is(err, io.EOF) {
			i.next = nil
			return nil
		}
		if err != nil {
			return err
		}
		if len(row) < 6 {
			return errors.New("CSV needs open_time,open,high,low,close,volume")
		}
		stamp, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			if i.rows == 1 && row[0] == "open_time" {
				continue
			}
			return errors.New("invalid candle timestamp")
		}
		if stamp > 1e14 {
			stamp /= 1e6
		} else if stamp > 1e11 {
			stamp /= 1e3
		}
		at := time.Unix(stamp, 0).UTC()
		if !i.previous.IsZero() && (!at.After(i.previous) || (!allowGaps && !at.Equal(i.previous.Add(time.Minute)))) {
			return fmt.Errorf("non-contiguous or unordered candles for %s at %s", i.symbol, at)
		}
		i.previous = at
		values := make([]decimal.Decimal, 5)
		for n := range values {
			values[n], err = decimal.NewFromString(row[n+1])
			if err != nil {
				return err
			}
		}
		if !values[0].IsPositive() || !values[2].IsPositive() || !values[3].IsPositive() || values[1].LessThan(decimal.Max(values[0], values[3])) || values[2].GreaterThan(decimal.Min(values[0], values[3])) || values[4].IsNegative() || at.Second() != 0 {
			return errors.New("invalid OHLCV candle")
		}
		i.next = &domain.Candle{Exchange: "binance", Market: domain.Spot, Symbol: i.symbol, Timeframe: "1m", OpenTime: at, Open: values[0], High: values[1], Low: values[2], Close: values[3], Volume: values[4], Closed: true}
		return nil
	}
}

type aggregate struct {
	bar   domain.Candle
	count int
}

func (a *aggregate) push(c domain.Candle, tf domain.Timeframe) (domain.Candle, bool) {
	start := tf.Truncate(c.OpenTime)
	if a.count == 0 || !a.bar.OpenTime.Equal(start) {
		a.bar = c
		a.bar.OpenTime = start
		a.bar.Timeframe = tf
		a.count = 0
	}
	if a.count > 0 {
		a.bar.High = decimal.Max(a.bar.High, c.High)
		a.bar.Low = decimal.Min(a.bar.Low, c.Low)
		a.bar.Volume = a.bar.Volume.Add(c.Volume)
	}
	a.bar.Close = c.Close
	a.count++
	complete := c.CloseTime().Equal(start.Add(tf.Duration())) && a.count == int(tf.Duration()/time.Minute)
	if complete {
		return a.bar, true
	}
	return domain.Candle{}, false
}
func run() error {
	fs := flag.NewFlagSet("portfolio-backtest", flag.ContinueOnError)
	configuration := fs.String("config", "configs/managed.example.yaml", "managed configuration")
	data := fs.String("data", "", "CSV directory: SYMBOL-1m.csv and instruments.json")
	fromText := fs.String("from", "", "inclusive UTC date YYYY-MM-DD")
	toText := fs.String("to", "", "exclusive UTC date YYYY-MM-DD")
	out := fs.String("out", "", "JSON report path; defaults to stdout")
	fee := fs.Float64("fee", 0.001, "commission per side")
	slip := fs.Float64("slippage-bps", 2, "market slippage")
	allowGaps := fs.Bool("allow-gaps", false, "explicitly permit missing minutes; incomplete aggregate bars are dropped")
	wallet := fs.String("wallet", "", "wallet id, defaults to first")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *data == "" || *fee < 0 || *fee > 0.1 || *slip < 0 {
		return errors.New("valid data directory and nonnegative costs required")
	}
	from, err := time.Parse("2006-01-02", *fromText)
	if err != nil {
		return err
	}
	to, err := time.Parse("2006-01-02", *toText)
	if err != nil || !to.After(from) {
		return errors.New("invalid end date")
	}
	// Validation only; no database connection, exchange client, secrets or .env.
	cfg, err := config.Load(*configuration, "/dev/null")
	if err != nil {
		return err
	}
	if len(cfg.Wallets) == 0 {
		return errors.New("wallet definition required")
	}
	spec := cfg.Wallets[0]
	if *wallet != "" {
		found := false
		for _, s := range cfg.Wallets {
			if s.ID == *wallet {
				spec = s
				found = true
			}
		}
		if !found {
			return errors.New("wallet not found")
		}
	}
	spec.Mode = "backtest"
	spec.Exchange = "binance"
	repo, err := managed.NewMemory(spec)
	if err != nil {
		return err
	}
	var ins []domain.Instrument
	raw, err := os.ReadFile(filepath.Join(*data, "instruments.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &ins); err != nil {
		return err
	}
	for n := range ins {
		ins[n].Exchange = "binance"
		ins[n].Market = domain.Spot
	}
	now := from
	sim := broker.NewSim(broker.SimConfig{Exchange: "binance", Fees: map[domain.Market]broker.Fees{domain.Spot: {Maker: *fee, Taker: *fee}}, SlippageBps: *slip, Clock: func() time.Time { return now }})
	rt, err := managed.NewRuntime(context.Background(), repo, sim, ins, decimal.NewFromFloat(*fee+*slip/10000), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		return err
	}
	rt.AllowGaps = *allowGaps
	started := false
	symbols := map[string]bool{}
	tfs := map[domain.Timeframe]bool{}
	for _, i := range spec.Instances {
		for _, b := range i.Strategies {
			st, err := strategy.New(b.Name)
			if err != nil {
				return err
			}
			if err = st.Init(nil, b.Config("binance", i.ID+"/"+b.ID)); err != nil {
				return err
			}
			for _, sym := range b.Symbols {
				symbols[sym] = true
			}
			for _, tf := range st.Timeframes() {
				tfs[tf] = true
			}
		}
	}
	sources := map[string]string{}
	sourceHash := func(path string) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		h := sha256.New()
		if _, err = io.Copy(h, f); err != nil {
			return err
		}
		sources[filepath.Base(path)] = hex.EncodeToString(h.Sum(nil))
		return nil
	}
	if err = sourceHash(filepath.Join(*data, "instruments.json")); err != nil {
		return err
	}
	var inputs []*input
	for symbol := range symbols {
		path := filepath.Join(*data, symbol+"-1m.csv")
		if err = sourceHash(path); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		i := &input{file: f, reader: csv.NewReader(f), symbol: symbol}
		if err = i.advance(*allowGaps); err != nil {
			return err
		}
		inputs = append(inputs, i)
	}
	slices.SortFunc(inputs, func(a, b *input) int {
		if a.symbol < b.symbol {
			return -1
		}
		if a.symbol > b.symbol {
			return 1
		}
		return 0
	})
	aggregators := map[string]*aggregate{}
	first, last := map[string]decimal.Decimal{}, map[string]decimal.Decimal{}
	peak := spec.Capital
	mdd := decimal.Zero
	bars := 0
	orderedTF := make([]domain.Timeframe, 0, len(tfs))
	for tf := range tfs {
		if tf != "1m" {
			orderedTF = append(orderedTF, tf)
		}
	}
	slices.SortFunc(orderedTF, func(a, b domain.Timeframe) int {
		if a.Duration() < b.Duration() {
			return -1
		}
		return 1
	})
	for {
		var selected *input
		for _, i := range inputs {
			if i.next != nil && (selected == nil || i.next.OpenTime.Before(selected.next.OpenTime)) {
				selected = i
			}
		}
		if selected == nil {
			break
		}
		c := *selected.next
		if err = selected.advance(*allowGaps); err != nil {
			return err
		}
		if c.OpenTime.Before(from) {
			rt.Warmup(c)
			for _, tf := range orderedTF {
				key := c.Symbol + string(tf)
				a := aggregators[key]
				if a == nil {
					a = &aggregate{}
					aggregators[key] = a
				}
				if bar, ok := a.push(c, tf); ok {
					rt.Warmup(bar)
				}
			}
			continue
		}
		if !c.OpenTime.Before(to) {
			selected.next = nil
			continue
		}
		now = c.CloseTime()
		if !started {
			if err = rt.Start(context.Background()); err != nil {
				return err
			}
			started = true
		}
		if _, ok := first[c.Symbol]; !ok {
			first[c.Symbol] = c.Open
		}
		last[c.Symbol] = c.Close
		if err = rt.Record(context.Background(), c, true); err != nil {
			return err
		}
		for _, tf := range orderedTF {
			key := c.Symbol + string(tf)
			a := aggregators[key]
			if a == nil {
				a = &aggregate{}
				aggregators[key] = a
			}
			if bar, ok := a.push(c, tf); ok {
				if err = rt.Record(context.Background(), bar, true); err != nil {
					return err
				}
			}
		}
		if err = rt.Tick(context.Background(), now); err != nil {
			return err
		}
		bars++
		state, err := rt.Snapshot(context.Background())
		if err != nil {
			return err
		}
		eq := state.Equity()
		peak = decimal.Max(peak, eq)
		if peak.IsPositive() {
			mdd = decimal.Max(mdd, peak.Sub(eq).Div(peak))
		}
	}
	if bars == 0 {
		return errors.New("no candles in requested period")
	}
	if err = rt.End(context.Background(), "completed"); err != nil {
		return err
	}
	state, err := rt.Snapshot(context.Background())
	if err != nil {
		return err
	}
	fees := decimal.Zero
	turnover := decimal.Zero
	for _, b := range state.Bindings {
		fees = fees.Add(b.Fees)
	}
	for _, o := range state.Orders {
		turnover = turnover.Add(o.Order.FilledQty.Mul(o.Order.AvgPrice))
	}
	benchmark := decimal.Zero
	for symbol, price := range first {
		benchmark = benchmark.Add(last[symbol].Div(price).Mul(decimal.NewFromFloat((1 - *fee) * (1 - *fee) / (1 + *slip/10000) * (1 - *slip/10000))))
	}
	benchmark = benchmark.Div(decimal.NewFromInt(int64(len(first))))
	exitCost := state.Inventory("").Mul(decimal.NewFromFloat(*fee + *slip/10000))
	report := map[string]any{"mode": "offline shared-capital backtest", "from": from, "to_exclusive": to, "bars": bars, "capital": spec.Capital, "final_equity": state.Equity(), "return": state.Equity().Div(spec.Capital).Sub(decimal.NewFromInt(1)), "max_drawdown": mdd, "fees": fees, "turnover": turnover, "terminal_exit_cost_estimate": exitCost, "conservative_final_equity": state.Equity().Sub(exitCost), "cash_benchmark_return": 0, "equal_weight_buy_hold_return": benchmark.Sub(decimal.NewFromInt(1)), "fee_per_side": *fee, "slippage_bps": *slip, "configuration_sha256": managed.Hash(spec), "data_sha256": sources, "state": state, "limitations": []string{"Signals and balances share one funded wallet; instances are not independent Rp2m simulations.", "OHLC fills omit order-book queue, latency and market impact.", "No taxes, IDR FX movement or funding; open inventory includes an explicit estimated terminal exit cost.", "Dates used in past selection are not an untouched holdout."}}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if *out != "" {
		return os.WriteFile(*out, append(encoded, '\n'), 0600)
	}
	fmt.Println(string(encoded))
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
