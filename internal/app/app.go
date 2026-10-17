// Package app wires configuration into concrete components shared by the
// commands in cmd/.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/config"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/exchange/binance"
	"github.com/rizz404/trading-bot/internal/exchange/bybit"
	"github.com/rizz404/trading-bot/internal/exchange/okx"
	"github.com/rizz404/trading-bot/internal/marketdata"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/rizz404/trading-bot/internal/strategy"
)

// MarketSource returns the public market data client of an exchange.
func MarketSource(name string, testnet bool, log *slog.Logger) (exchange.MarketData, error) {
	switch name {
	case "binance":
		return binance.New(binance.Options{Testnet: testnet, Log: log}), nil
	case "bybit":
		return bybit.New(bybit.Options{Testnet: testnet, Log: log}), nil
	case "okx":
		return okx.New(okx.Options{Testnet: testnet, Log: log}), nil
	default:
		return nil, fmt.Errorf("unsupported exchange %q", name)
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

// StrategyConfig converts a configured strategy instance. Symbols default
// to fallback when the instance lists none.
func StrategyConfig(sc config.StrategyConfig, fallback []string) (strategy.Config, error) {
	m, err := domain.ParseMarket(sc.Market)
	if err != nil {
		return strategy.Config{}, err
	}
	symbols := sc.Symbols
	if len(symbols) == 0 {
		symbols = fallback
	}
	return strategy.Config{
		ID: sc.ID, Name: sc.Name, Exchange: sc.Exchange, Market: m,
		Symbols: symbols, Leverage: sc.Leverage, Params: sc.Params,
	}, nil
}

// Fees converts the configured fee table.
func Fees(fees map[string]broker.Fees) map[domain.Market]broker.Fees {
	out := map[domain.Market]broker.Fees{}
	for k, v := range fees {
		out[domain.Market(k)] = v
	}
	return out
}

// ExchangeName is the name used for an exchange's data, accounts and orders
// in a mode: testnet data and orders never mix with mainnet ones.
func ExchangeName(exchange string, mode config.Mode) string {
	if mode == config.ModeTestnet {
		return exchange + "-testnet"
	}
	return exchange
}

// LiveBroker returns the trading client of an exchange for testnet or live
// mode, using the credentials from the config.
func LiveBroker(cfg *config.Config, name string, markets []domain.Market, log *slog.Logger) (broker.Live, error) {
	ex, ok := cfg.Exchanges[name]
	if !ok || !ex.Enabled {
		return nil, fmt.Errorf("exchange %s is not enabled in the config", name)
	}
	testnet := cfg.App.Mode == config.ModeTestnet
	switch name {
	case "binance":
		c := binance.New(binance.Options{Testnet: testnet, Log: log})
		return binance.NewTrader(c, ex.APIKey.Reveal(), ex.APISecret.Reveal(), markets), nil
	case "bybit":
		c := bybit.New(bybit.Options{Testnet: testnet, Log: log})
		return bybit.NewTrader(c, ex.APIKey.Reveal(), ex.APISecret.Reveal(), markets), nil
	case "okx":
		c := okx.New(okx.Options{Testnet: testnet, Log: log})
		return okx.NewTrader(c, ex.APIKey.Reveal(), ex.APISecret.Reveal(), ex.Passphrase.Reveal(), markets), nil
	default:
		return nil, fmt.Errorf("live trading on %s is not supported", name)
	}
}
