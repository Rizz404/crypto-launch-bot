// Package backtest replays stored candles through the engine with the
// simulated broker and reports performance metrics.
package backtest

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/engine"
	"github.com/rizz404/trading-bot/internal/portfolio"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
)

// CandleSource loads stored candles. storage.Store implements it.
type CandleSource interface {
	Candles(ctx context.Context, k domain.SeriesKey, from, to time.Time) ([]domain.Candle, error)
}

type Config struct {
	Strategy       strategy.Config
	From, To       time.Time
	InitialBalance decimal.Decimal // per account (exchange + market)
	Fees           map[domain.Market]broker.Fees
	SlippageBps    float64
	// FundingRate is charged on futures positions every 8 hours (00, 08,
	// 16 UTC); historical funding is not stored, so a constant is used.
	FundingRate decimal.Decimal
	// ExecTimeframe, if set and smaller than the strategy timeframes, is
	// used to match orders (e.g. 1m for realistic intrabar stop fills).
	ExecTimeframe domain.Timeframe
	Risk          risk.Config
	Instruments   []domain.Instrument
	Log           *slog.Logger
}

// EquityPoint is the account equity at a moment.
type EquityPoint struct {
	Time   time.Time
	Equity decimal.Decimal
}

type Result struct {
	Config        Config
	Metrics       Metrics
	Equity        []EquityPoint
	Trades        []domain.Trade
	OpenPositions []domain.Position
	Candles       int
}

const fundingInterval = 8 * time.Hour

// Run executes one backtest.
func Run(ctx context.Context, cfg Config, src CandleSource) (*Result, error) {
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	sc := cfg.Strategy
	if sc.Exchange == "" || len(sc.Symbols) == 0 {
		return nil, fmt.Errorf("backtest: strategy needs an exchange and symbols")
	}

	var now time.Time
	clock := func() time.Time { return now }

	var trades []domain.Trade
	onTrade := func(t domain.Trade) { trades = append(trades, t) }
	book := portfolio.NewBook()
	eng := engine.New(engine.Config{Book: book, Risk: risk.NewManager(cfg.Risk, log), Log: log, Clock: clock, IDSeed: 0})

	// One simulated broker per exchange and one account per exchange and
	// market, each starting with InitialBalance.
	sims := map[string]*broker.Sim{}
	for _, in := range sc.Instruments() {
		key := portfolio.AccountKey{Exchange: in.Exchange, Market: in.Market}
		if book.Account(key) == nil {
			book.Add(portfolio.NewAccount(key, cfg.InitialBalance, onTrade))
		}
		if sims[in.Exchange] == nil {
			sims[in.Exchange] = broker.NewSim(broker.SimConfig{Exchange: in.Exchange, Fees: cfg.Fees, SlippageBps: cfg.SlippageBps, Clock: clock})
			eng.AddBroker(sims[in.Exchange])
		}
	}
	eng.SetInstruments(cfg.Instruments)
	if err := eng.AddStrategy(sc); err != nil {
		return nil, err
	}

	// Load every timeframe the strategy needs plus the execution timeframe.
	tfs := eng.Timeframes()
	execTF := smallest(tfs)
	if cfg.ExecTimeframe != "" && cfg.ExecTimeframe.Duration() < execTF.Duration() {
		execTF = cfg.ExecTimeframe
		tfs = append(tfs, execTF)
	}
	var candles []domain.Candle
	for _, in := range sc.Instruments() {
		for _, tf := range tfs {
			key := domain.SeriesKey{Exchange: in.Exchange, Market: in.Market, Symbol: in.Symbol, Timeframe: tf}
			cs, err := src.Candles(ctx, key, cfg.From, cfg.To)
			if err != nil {
				return nil, fmt.Errorf("load %s: %w", key, err)
			}
			if len(cs) == 0 {
				return nil, fmt.Errorf("no candles for %s in range; run datafetch first", key)
			}
			candles = append(candles, cs...)
		}
	}
	// Time order; at equal close time the execution timeframe goes first so
	// fills happen before strategies see the larger bar.
	slices.SortFunc(candles, func(a, b domain.Candle) int {
		return cmp.Or(
			a.CloseTime().Compare(b.CloseTime()),
			cmp.Compare(a.Timeframe.Duration(), b.Timeframe.Duration()),
			cmp.Compare(a.Symbol, b.Symbol),
		)
	})
	log.Info("backtest loaded", "candles", len(candles), "exec_tf", execTF, "timeframes", tfs)

	res := &Result{Config: cfg, Candles: len(candles)}
	for i, c := range candles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.Timeframe == execTF {
			now = c.OpenTime
			sims[c.Exchange].OnCandle(c)
		}
		now = c.CloseTime()
		if c.Timeframe == execTF && c.Market == domain.Futures && !cfg.FundingRate.IsZero() &&
			now.Truncate(fundingInterval).Equal(now) {
			acct := book.Account(portfolio.AccountKey{Exchange: c.Exchange, Market: c.Market})
			acct.Mark(c.Symbol, c.Close)
			acct.ApplyFunding(c.Symbol, cfg.FundingRate, now)
		}
		eng.OnCandle(ctx, c)
		if c.Timeframe == execTF {
			eng.Tick(ctx, now) // timer-driven strategies (DCA, funding)
		}

		// Sample only after every symbol/account/timeframe at this close
		// time has updated its ledger. Sampling the first event loses the
		// other accounts' mark-to-market PnL, including the final bar.
		if i+1 == len(candles) || !candles[i+1].CloseTime().Equal(now) {
			res.Equity = append(res.Equity, EquityPoint{Time: now, Equity: book.Total().Equity})
		}
	}

	res.Trades = trades
	for _, a := range book.Accounts() {
		res.OpenPositions = append(res.OpenPositions, a.Positions()...)
	}
	initial := cfg.InitialBalance.Mul(decimal.NewFromInt(int64(len(book.Accounts()))))
	res.Metrics = Compute(initial, res.Equity, trades)
	return res, nil
}

func smallest(tfs []domain.Timeframe) domain.Timeframe {
	best := tfs[0]
	for _, tf := range tfs[1:] {
		if tf.Duration() < best.Duration() {
			best = tf
		}
	}
	return best
}
