package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/rizz404/trading-bot/internal/backtest"
	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

type researchJob struct {
	ID, Candidate, Family, Stage, Symbol    string
	From, To, InputTF, InputFile, RulesFile string
	Name                                    string
	Params                                  map[string]any
	Risk                                    risk.Config
	Fee, SlippageBps                        float64
}

type researchResult struct {
	Job                                                                                   researchJob
	Metrics                                                                               backtest.Metrics
	ConservativeFinalUSDT, ConservativeFinalIDR, ConservativeReturn, TerminalExitCostUSDT float64
	OpenPositions, Candles                                                                int
	Seconds                                                                               float64
	Issues                                                                                *issues
}

// Binance's documented spot halt left the 2023-03-24 13:00 UTC hourly
// candle absent. Keep it absent: no invented price or executable candle.
// https://www.binance.com/en/support/announcement/detail/813a31506e9f478ea8c1058b425df87a
func knownSpotHalt(at time.Time, tf domain.Timeframe) bool {
	return tf == "1h" && at.Equal(time.Date(2023, 3, 24, 13, 0, 0, 0, time.UTC))
}

func expectedTradingSamples(from, to time.Time, tf domain.Timeframe) int {
	n := int(to.Sub(from) / tf.Duration())
	halt := time.Date(2023, 3, 24, 13, 0, 0, 0, time.UTC)
	if knownSpotHalt(halt, tf) && !halt.Before(from) && halt.Before(to) {
		n--
	}
	return n
}

// Group by UTC clock buckets rather than a fixed number of available bars.
// A halt can shorten a bucket, but cannot shift all following 4h/day bars.
func aggregateTradingBars(cs []domain.Candle, tf, inputTF domain.Timeframe) ([]domain.Candle, error) {
	var out []domain.Candle
	var count int
	flush := func() error {
		if len(out) == 0 {
			return nil
		}
		last := out[len(out)-1]
		if count != expectedTradingSamples(last.OpenTime, last.CloseTime(), inputTF) {
			return fmt.Errorf("incomplete trading bucket %s", last.OpenTime)
		}
		return nil
	}
	for _, c := range cs {
		at := tf.Truncate(c.OpenTime)
		if len(out) == 0 || !out[len(out)-1].OpenTime.Equal(at) {
			if err := flush(); err != nil {
				return nil, err
			}
			if len(out) > 0 && !out[len(out)-1].CloseTime().Equal(at) {
				return nil, fmt.Errorf("missing aggregate bucket %s", at)
			}
			a := c
			a.Timeframe = tf
			a.OpenTime = at
			out = append(out, a)
			count = 1
		} else {
			a := &out[len(out)-1]
			a.High = decimal.Max(a.High, c.High)
			a.Low = decimal.Min(a.Low, c.Low)
			a.Close = c.Close
			a.Volume = a.Volume.Add(c.Volume)
			a.QuoteVolume = a.QuoteVolume.Add(c.QuoteVolume)
			a.Trades += c.Trades
			count++
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

func rangeSource(path, symbol string, tf domain.Timeframe, from, to time.Time) (source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { must(f.Close()) }()
	r := csv.NewReader(f)
	var cs []domain.Candle
	for {
		v, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if len(v) != 8 {
			return nil, fmt.Errorf("invalid normalized CSV")
		}
		sec, e := strconv.ParseInt(v[0], 10, 64)
		if e != nil {
			return nil, e
		}
		at := time.Unix(sec, 0).UTC()
		if at.Before(from) || !at.Before(to) {
			continue
		}
		if len(cs) > 0 {
			next := cs[len(cs)-1].OpenTime.Add(tf.Duration())
			if !at.Equal(next) {
				if !knownSpotHalt(next, tf) || !at.Equal(next.Add(tf.Duration())) {
					return nil, fmt.Errorf("gap/duplicate at %s", at)
				}
			}
		}
		tr, e := strconv.ParseInt(v[7], 10, 64)
		if e != nil {
			return nil, e
		}
		cs = append(cs, domain.Candle{Exchange: "binance", Market: domain.Spot, Symbol: symbol, Timeframe: tf, OpenTime: at, Open: dec(v[1]), High: dec(v[2]), Low: dec(v[3]), Close: dec(v[4]), Volume: dec(v[5]), QuoteVolume: dec(v[6]), Trades: tr, Closed: true})
	}
	expected := expectedTradingSamples(from, to, tf)
	if len(cs) != expected || len(cs) == 0 || !cs[0].OpenTime.Equal(from) || !cs[len(cs)-1].CloseTime().Equal(to) {
		return nil, fmt.Errorf("incomplete %s %s: %d/%d candles", symbol, tf, len(cs), expected)
	}
	src := source{tf: cs}
	for _, out := range []domain.Timeframe{"1h", "4h", "1d"} {
		if out.Duration() > tf.Duration() {
			bars, e := aggregateTradingBars(cs, out, tf)
			if e != nil {
				return nil, e
			}
			src[out] = bars
		}
	}
	return src, nil
}
func jobInstrument(path, symbol string) (domain.Instrument, error) {
	var info struct {
		Symbols []struct {
			Symbol, BaseAsset, QuoteAsset, Status string
			Filters                               []map[string]json.RawMessage
		}
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return domain.Instrument{}, e
	}
	if e = json.Unmarshal(b, &info); e != nil {
		return domain.Instrument{}, e
	}
	for _, s := range info.Symbols {
		if s.Symbol != symbol {
			continue
		}
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
			return in, fmt.Errorf("incomplete instrument rules")
		}
		return in, nil
	}
	return domain.Instrument{}, fmt.Errorf("instrument not found: %s", symbol)
}
func runJob(input, output string) error {
	if output == "" {
		return fmt.Errorf("-result is required with -job")
	}
	started := time.Now()
	b, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var j researchJob
	if err = json.Unmarshal(b, &j); err != nil {
		return err
	}
	if j.Fee <= 0 || j.Fee > 0.01 || j.SlippageBps < 0 || j.SlippageBps > 100 || j.Risk.MaxLeverage != 1 || j.Risk.MaxPositionPct <= 0 || j.Risk.MaxPositionPct > 0.98 || j.Risk.RiskPerTrade <= 0 || j.Risk.RiskPerTrade > 0.05 || j.Risk.MaxExposurePct <= 0 || j.Risk.MaxExposurePct > 1 || j.Risk.DailyLossLimit <= 0 || j.Risk.DailyLossLimit > 0.10 || j.Risk.MaxDrawdown <= 0 || j.Risk.MaxDrawdown > 0.35 || !j.Risk.CloseOnKill || j.Risk.MaxOpenPositions != 1 {
		return fmt.Errorf("invalid spot research risk/cost settings")
	}
	tf, err := domain.ParseTimeframe(j.InputTF)
	if err != nil {
		return err
	}
	if tf != "1m" && tf != "1h" {
		return fmt.Errorf("only 1m/1h research sources supported")
	}
	from, err := time.Parse(time.DateOnly, j.From)
	if err != nil {
		return err
	}
	to, err := time.Parse(time.DateOnly, j.To)
	if err != nil {
		return err
	}
	if !to.After(from) {
		return fmt.Errorf("empty research period")
	}
	src, err := rangeSource(j.InputFile, j.Symbol, tf, from, to)
	if err != nil {
		return err
	}
	in, err := jobInstrument(j.RulesFile, j.Symbol)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	is := &issues{Counts: map[string]int{}}
	const fx = 18052.0
	balance := decimal.NewFromInt(2000000).Div(decimal.NewFromFloat(fx))
	sc := strategy.Config{ID: j.ID, Name: j.Name, Exchange: "binance", Market: domain.Spot, Symbols: []string{j.Symbol}, Leverage: 1, Params: j.Params}
	res, err := backtest.Run(ctx, backtest.Config{Strategy: sc, From: from, To: to, InitialBalance: balance, Fees: map[domain.Market]broker.Fees{domain.Spot: {Maker: j.Fee, Taker: j.Fee}}, SlippageBps: j.SlippageBps, ExecTimeframe: tf, Risk: j.Risk, Instruments: []domain.Instrument{in}, Log: slog.New(is)}, src)
	if err != nil {
		return err
	}
	// Conservative terminal value: include estimated market-exit fee and adverse slippage on inventory.
	// This adjustment does not fabricate a closed trade or change intraperiod drawdown metrics.
	terminal := 0.0
	last := src[tf][len(src[tf])-1].Close
	for _, p := range res.OpenPositions {
		notional, _ := p.Notional(last).Float64()
		terminal += notional * (j.Fee + j.SlippageBps/10000)
	}
	final := res.Metrics.FinalEquity - terminal
	result := researchResult{Job: j, Metrics: res.Metrics, ConservativeFinalUSDT: final, ConservativeFinalIDR: final * fx, ConservativeReturn: final/res.Metrics.InitialEquity - 1, TerminalExitCostUSDT: terminal, OpenPositions: len(res.OpenPositions), Candles: res.Candles, Seconds: time.Since(started).Seconds(), Issues: is}
	b, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	tmp := output + ".tmp"
	if err = os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, output)
}
