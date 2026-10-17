// Package app wires configuration into concrete components shared by the
// commands in cmd/.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/exchange/binance"
	"github.com/rizz404/trading-bot/internal/marketdata"
	"github.com/rizz404/trading-bot/internal/storage"
)

// MarketSource returns the market data client named in the config.
func MarketSource(cfg *config.Config, testnet bool, log *slog.Logger) (exchange.MarketData, error) {
	switch cfg.MarketData.Exchange {
	case "binance":
		return binance.New(binance.Options{Testnet: testnet, Log: log}), nil
	default:
		return nil, fmt.Errorf("unsupported market data exchange %q", cfg.MarketData.Exchange)
	}
}

// UniverseRules converts the universe config.
func UniverseRules(u config.UniverseConfig) marketdata.UniverseRules {
	return marketdata.UniverseRules{
		Quote:          u.Quote,
		TopN:           u.TopN,
		MinQuoteVolume: decimal.NewFromFloat(u.MinQuoteVolume),
		MaxSpreadBps:   u.MaxSpreadBps,
		ExcludeBases:   u.ExcludeBases,
	}
}

// ResolveSymbols refreshes the stored instruments of market m and returns
// the symbols to trade: the fixed list if given, otherwise the universe.
// Fixed symbols are checked against the exchange's instrument list.
func ResolveSymbols(ctx context.Context, src exchange.MarketData, store *storage.Store, m domain.Market,
	fixed []string, rules marketdata.UniverseRules, log *slog.Logger,
) ([]string, error) {
	ins, err := src.Instruments(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("instruments: %w", err)
	}
	if err := store.UpsertInstruments(ctx, ins); err != nil {
		return nil, err
	}

	if len(fixed) > 0 {
		known := make(map[string]domain.Instrument, len(ins))
		for _, in := range ins {
			known[in.Symbol] = in
		}
		for _, s := range fixed {
			in, ok := known[s]
			if !ok {
				return nil, fmt.Errorf("%s %s: unknown symbol on %s", m, s, src.Name())
			}
			if !in.Tradable {
				log.Warn("symbol is not trading", "market", m, "symbol", s, "status", in.Status)
			}
		}
		return fixed, nil
	}

	tickers, err := src.Tickers(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("tickers: %w", err)
	}
	picks := marketdata.SelectUniverse(ins, tickers, rules)
	if len(picks) == 0 {
		return nil, fmt.Errorf("%s: no symbol passed the universe rules", m)
	}
	for i, p := range picks {
		log.Debug("universe pick", "market", m, "rank", i+1, "symbol", p.Symbol,
			"quote_volume_m", p.QuoteVolume.Shift(-6).StringFixed(1), "spread_bps", fmt.Sprintf("%.2f", p.SpreadBps))
	}
	log.Info("universe selected", "market", m, "symbols", marketdata.Symbols(picks))
	return marketdata.Symbols(picks), nil
}

// Subscriptions is the cross product of symbols and timeframes.
func Subscriptions(symbols []string, tfs []domain.Timeframe) []exchange.KlineSub {
	subs := make([]exchange.KlineSub, 0, len(symbols)*len(tfs))
	for _, s := range symbols {
		for _, tf := range tfs {
			subs = append(subs, exchange.KlineSub{Symbol: s, Timeframe: tf})
		}
	}
	return subs
}

// Timeframes parses the configured timeframes (already validated).
func Timeframes(names []string) ([]domain.Timeframe, error) {
	out := make([]domain.Timeframe, len(names))
	for i, n := range names {
		tf, err := domain.ParseTimeframe(n)
		if err != nil {
			return nil, err
		}
		out[i] = tf
	}
	return out, nil
}
