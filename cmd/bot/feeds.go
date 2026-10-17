package main

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/rizz404/trading-bot/internal/app"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/marketdata"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
)

type feedKey struct {
	exchange string // data name, e.g. "binance" or "binance-testnet"
	market   domain.Market
}

// feedPlan collects which series to stream from which source.
type feedPlan struct {
	cfg     *config.Config
	log     *slog.Logger
	sources map[string]exchange.MarketData // by base exchange name
	byName  map[string]exchange.MarketData // by data name
	series  map[feedKey]map[string]map[domain.Timeframe]bool
	symbols map[feedKey][]string // marketdata symbols, the default for strategies
}

func newFeedPlan(cfg *config.Config, log *slog.Logger) *feedPlan {
	return &feedPlan{
		cfg: cfg, log: log,
		sources: map[string]exchange.MarketData{},
		byName:  map[string]exchange.MarketData{},
		series:  map[feedKey]map[string]map[domain.Timeframe]bool{},
		symbols: map[feedKey][]string{},
	}
}

// source returns the market data client of a base exchange. Live data
// comes from testnet only in testnet mode, so paper trading sees mainnet
// prices.
func (p *feedPlan) source(base string) (exchange.MarketData, error) {
	if src, ok := p.sources[base]; ok {
		return src, nil
	}
	src, err := app.MarketSource(base, p.cfg.App.Mode == config.ModeTestnet, p.log)
	if err != nil {
		return nil, err
	}
	p.sources[base], p.byName[src.Name()] = src, src
	return src, nil
}

func (p *feedPlan) add(exchangeName string, m domain.Market, symbol string, tf domain.Timeframe) {
	k := feedKey{exchangeName, m}
	if p.series[k] == nil {
		p.series[k] = map[string]map[domain.Timeframe]bool{}
	}
	if p.series[k][symbol] == nil {
		p.series[k][symbol] = map[domain.Timeframe]bool{}
	}
	p.series[k][symbol][tf] = true
}

// addMarketData adds the series configured under marketdata.
func (p *feedPlan) addMarketData(ctx context.Context, store *storage.Store) error {
	src, err := p.source(p.cfg.MarketData.Exchange)
	if err != nil {
		return err
	}
	tfs, err := app.Timeframes(p.cfg.MarketData.Timeframes)
	if err != nil {
		return err
	}
	rules := app.UniverseRules(p.cfg.Universe)
	for _, name := range p.cfg.MarketData.Markets {
		m, err := domain.ParseMarket(name)
		if err != nil {
			return err
		}
		symbols, err := app.ResolveSymbols(ctx, src, store, m, p.cfg.MarketData.Symbols, rules, p.log)
		if err != nil {
			return err
		}
		p.symbols[feedKey{src.Name(), m}] = symbols
		for _, sym := range symbols {
			for _, tf := range tfs {
				p.add(src.Name(), m, sym, tf)
			}
		}
	}
	return nil
}

// strategies returns the enabled strategy instances, with exchange names
// adjusted to the mode and symbols defaulting to the marketdata ones.
// Instruments of markets not covered by marketdata are refreshed here.
func (p *feedPlan) strategies(ctx context.Context, store *storage.Store) ([]strategy.Config, error) {
	var out []strategy.Config
	for _, sc := range p.cfg.Strategies {
		if !sc.Enabled {
			continue
		}
		src, err := p.source(sc.Exchange)
		if err != nil {
			return nil, fmt.Errorf("strategy %s: %w", sc.ID, err)
		}
		m, err := domain.ParseMarket(sc.Market)
		if err != nil {
			return nil, err
		}
		k := feedKey{src.Name(), m}
		scfg, err := app.StrategyConfig(sc, p.symbols[k])
		if err != nil {
			return nil, err
		}
		if len(scfg.Symbols) == 0 {
			return nil, fmt.Errorf("strategy %s: no symbols (set strategies[].symbols or include %s in marketdata.markets)", sc.ID, m)
		}
		var renameErr error
		scfg = app.RenameExchanges(scfg, func(base string) string {
			s, err := p.source(base)
			if err != nil {
				renameErr = err
				return base
			}
			return s.Name()
		})
		if renameErr != nil {
			return nil, fmt.Errorf("strategy %s: %w", sc.ID, renameErr)
		}
		// Refresh instruments and validate the symbols of every instrument
		// not already covered by marketdata.
		bySource := map[feedKey][]string{}
		for _, in := range scfg.Instruments() {
			fk := feedKey{in.Exchange, in.Market}
			if !slices.Contains(p.symbols[fk], in.Symbol) {
				bySource[fk] = append(bySource[fk], in.Symbol)
			}
		}
		for fk, symbols := range bySource {
			if _, err := app.ResolveSymbols(ctx, p.byName[fk.exchange], store, fk.market, symbols, app.UniverseRules(p.cfg.Universe), p.log); err != nil {
				return nil, fmt.Errorf("strategy %s: %w", sc.ID, err)
			}
		}
		out = append(out, scfg)
	}
	return out, nil
}

// start launches one feed per exchange and market. Feeds that stop for any
// reason other than shutdown report to errc.
func (p *feedPlan) start(ctx context.Context, store *storage.Store, m *marketdata.Metrics, sess *session, errc chan<- error) *sync.WaitGroup {
	var wg sync.WaitGroup
	for k, bySymbol := range p.series {
		var subs []exchange.KlineSub
		for sym, tfs := range bySymbol {
			for tf := range tfs {
				subs = append(subs, exchange.KlineSub{Symbol: sym, Timeframe: tf})
			}
		}
		slices.SortFunc(subs, func(a, b exchange.KlineSub) int {
			return cmp.Or(cmp.Compare(a.Symbol, b.Symbol), cmp.Compare(a.Timeframe, b.Timeframe))
		})
		feed := &marketdata.Feed{
			Source:          p.byName[k.exchange],
			Store:           store,
			Market:          k.market,
			Subs:            subs,
			InitialBackfill: p.cfg.MarketData.InitialBackfill,
			Log:             p.log,
			Metrics:         m,
		}
		if sess != nil {
			feed.OnCandle = func(c domain.Candle) { sess.onCandle(ctx, c) }
			feed.OnUpdate = func(c domain.Candle) { sess.onUpdate(ctx, c) }
		}
		p.log.Info("starting feed", "source", k.exchange, "market", k.market, "symbols", len(bySymbol), "streams", len(subs))
		wg.Go(func() {
			if err := feed.Run(ctx); err != nil && ctx.Err() == nil {
				select {
				case errc <- fmt.Errorf("%s %s feed: %w", k.exchange, k.market, err):
				default:
				}
			}
		})
	}
	return &wg
}
