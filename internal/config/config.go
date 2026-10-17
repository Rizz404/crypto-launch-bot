// Package config loads bot configuration from defaults, a YAML file, an
// optional .env file and the process environment (in that precedence order).
//
// Secrets (API keys) are accepted only from the environment, never from YAML.
// Environment keys use the TB_ prefix and "__" as the nesting separator:
//
//	TB_DATABASE__URL                -> database.url
//	TB_EXCHANGES__BINANCE__API_KEY  -> exchanges.binance.api_key
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/risk"
)

const envPrefix = "TB_"

// Mode is the execution mode of the bot.
type Mode string

const (
	ModeBacktest Mode = "backtest"
	ModePaper    Mode = "paper"
	ModeTestnet  Mode = "testnet"
	ModeLive     Mode = "live"
)

func (m Mode) valid() bool {
	switch m {
	case ModeBacktest, ModePaper, ModeTestnet, ModeLive:
		return true
	}
	return false
}

// Secret is a string that never prints its value.
type Secret string

func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return "***"
}

// LogValue keeps secrets out of slog output.
func (s Secret) LogValue() slog.Value { return slog.StringValue(s.String()) }

// Reveal returns the raw secret value.
func (s Secret) Reveal() string { return string(s) }

type Config struct {
	App        AppConfig                 `koanf:"app"`
	Database   DatabaseConfig            `koanf:"database"`
	Metrics    MetricsConfig             `koanf:"metrics"`
	Exchanges  map[string]ExchangeConfig `koanf:"exchanges"`
	MarketData MarketDataConfig          `koanf:"marketdata"`
	Universe   UniverseConfig            `koanf:"universe"`
	Strategies []StrategyConfig          `koanf:"strategies"`
	Risk       risk.Config               `koanf:"risk"`
	Backtest   BacktestConfig            `koanf:"backtest"`
	Paper      PaperConfig               `koanf:"paper"`
	Telegram   TelegramConfig            `koanf:"telegram"`
}

// TelegramConfig configures notifications and remote commands.
type TelegramConfig struct {
	Enabled bool   `koanf:"enabled"`
	Token   Secret `koanf:"token"` // only via TB_TELEGRAM__TOKEN
	// ChatIDs may receive notifications and run commands.
	ChatIDs []int64 `koanf:"chat_ids"`
	// NotifyLevel is the minimum engine notice level forwarded (info, warn, error).
	NotifyLevel  string `koanf:"notify_level"`
	DailySummary bool   `koanf:"daily_summary"`
}

// PaperConfig configures simulated trading on live data. Fees and slippage
// come from the backtest section.
type PaperConfig struct {
	InitialBalance float64 `koanf:"initial_balance"` // per account (exchange + market)
	// Resume restores balances, positions and open orders of the previous
	// paper session instead of starting fresh.
	Resume bool `koanf:"resume"`
}

// StrategyConfig configures one strategy instance.
type StrategyConfig struct {
	ID       string         `koanf:"id"`
	Name     string         `koanf:"name"`
	Enabled  bool           `koanf:"enabled"`
	Exchange string         `koanf:"exchange"`
	Market   string         `koanf:"market"`
	Symbols  []string       `koanf:"symbols"` // empty: the marketdata symbols/universe
	Leverage int            `koanf:"leverage"`
	Params   map[string]any `koanf:"params"`
}

// BacktestConfig holds simulation costs.
type BacktestConfig struct {
	InitialBalance float64                `koanf:"initial_balance"`
	SlippageBps    float64                `koanf:"slippage_bps"`
	FundingRate    float64                `koanf:"funding_rate"` // per 8h
	ExecTimeframe  string                 `koanf:"exec_timeframe"`
	Fees           map[string]broker.Fees `koanf:"fees"` // by market
}

type AppConfig struct {
	Mode Mode `koanf:"mode"`
	// AllowLive must be true to run in live (mainnet) mode: a deliberate
	// second switch so real orders are never sent by accident.
	AllowLive bool   `koanf:"allow_live"`
	LogLevel  string `koanf:"log_level"`
	LogFormat string `koanf:"log_format"`
}

type DatabaseConfig struct {
	URL      Secret `koanf:"url"`
	MaxConns int32  `koanf:"max_conns"`
}

type MetricsConfig struct {
	Enabled bool   `koanf:"enabled"`
	Listen  string `koanf:"listen"`
}

type ExchangeConfig struct {
	Enabled    bool   `koanf:"enabled"`
	Testnet    bool   `koanf:"testnet"`
	APIKey     Secret `koanf:"api_key"`
	APISecret  Secret `koanf:"api_secret"`
	Passphrase Secret `koanf:"passphrase"` // OKX only
}

// MarketDataConfig selects which live candles the bot records.
type MarketDataConfig struct {
	Exchange   string   `koanf:"exchange"`
	Markets    []string `koanf:"markets"`
	Timeframes []string `koanf:"timeframes"`
	// Symbols fixes the symbol list; when empty the universe rules pick them.
	Symbols []string `koanf:"symbols"`
	// InitialBackfill is how many candles to fetch per series with no history.
	InitialBackfill int `koanf:"initial_backfill"`
}

// UniverseConfig ranks symbols by liquidity. See marketdata.UniverseRules.
type UniverseConfig struct {
	Quote          string   `koanf:"quote"`
	TopN           int      `koanf:"top_n"`
	MinQuoteVolume float64  `koanf:"min_quote_volume"`
	MaxSpreadBps   float64  `koanf:"max_spread_bps"`
	ExcludeBases   []string `koanf:"exclude_bases"`
}

// secretKeys may only be set through the environment.
var secretKeys = []string{"api_key", "api_secret", "passphrase"}

// DefaultExcludeBases are assets that are pegged or wrapped, and therefore
// not worth trading against USDT.
var DefaultExcludeBases = []string{
	"USDC", "FDUSD", "TUSD", "USDP", "DAI", "BUSD", "USD1", "USDE", "PYUSD", "RLUSD", "BFUSD", "XUSD", "U",
	"EUR", "EURI", "AEUR", "GBP", "TRY", "BRL",
	"PAXG", "XAUT", "WBTC", "WBETH", "BETH",
}

func defaults() Config {
	return Config{
		App: AppConfig{
			Mode:      ModePaper,
			LogLevel:  "info",
			LogFormat: "json",
		},
		Database: DatabaseConfig{MaxConns: 10},
		Metrics:  MetricsConfig{Enabled: true, Listen: "127.0.0.1:9100"},
		MarketData: MarketDataConfig{
			Exchange:        "binance",
			Markets:         []string{"futures"},
			Timeframes:      []string{"1m", "1h"},
			InitialBackfill: 1000,
		},
		Risk:     risk.DefaultConfig(),
		Paper:    PaperConfig{InitialBalance: 1000, Resume: true},
		Telegram: TelegramConfig{NotifyLevel: "warn", DailySummary: true},
		Backtest: BacktestConfig{
			InitialBalance: 1000,
			SlippageBps:    2,
			FundingRate:    0.0001,
			Fees: map[string]broker.Fees{
				"spot":    {Maker: 0.001, Taker: 0.001},
				"futures": {Maker: 0.0002, Taker: 0.0005},
			},
		},
		Universe: UniverseConfig{
			Quote:          "USDT",
			TopN:           20,
			MinQuoteVolume: 20_000_000,
			MaxSpreadBps:   10,
			ExcludeBases:   slices.Clone(DefaultExcludeBases),
		},
	}
}

// Load reads the YAML file at path, then overlays envFile (skipped if it
// does not exist) and the process environment, and validates the result.
func Load(path, envFile string) (*Config, error) {
	dotenv, err := readDotEnv(envFile)
	if err != nil {
		return nil, err
	}
	// Process environment wins over .env because it comes last.
	return load(path, append(dotenv, os.Environ()...))
}

func load(path string, environ []string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	if err := checkNoSecretsInFile(k); err != nil {
		return nil, err
	}

	envProvider := env.Provider(".", env.Opt{
		Prefix:        envPrefix,
		EnvironFunc:   func() []string { return environ },
		TransformFunc: envKey,
	})
	if err := k.Load(envProvider, nil); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	cfg := defaults()
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

// envKey maps TB_EXCHANGES__BINANCE__API_KEY to exchanges.binance.api_key.
func envKey(k, v string) (string, any) {
	k = strings.ToLower(strings.TrimPrefix(k, envPrefix))
	return strings.ReplaceAll(k, "__", "."), v
}

func checkNoSecretsInFile(k *koanf.Koanf) error {
	var errs []error
	for _, name := range k.MapKeys("exchanges") {
		for _, s := range secretKeys {
			key := "exchanges." + name + "." + s
			if k.String(key) != "" {
				env := envPrefix + strings.ToUpper(strings.ReplaceAll(key, ".", "__"))
				errs = append(errs, fmt.Errorf("%s must not be set in the config file, use %s", key, env))
			}
		}
	}
	if k.String("telegram.token") != "" {
		errs = append(errs, errors.New("telegram.token must not be set in the config file, use TB_TELEGRAM__TOKEN"))
	}
	return errors.Join(errs...)
}

// Validate checks the configuration for invalid or unsafe combinations.
func (c *Config) Validate() error {
	var errs []error
	if !c.App.Mode.valid() {
		errs = append(errs, fmt.Errorf("app.mode %q must be one of backtest, paper, testnet, live", c.App.Mode))
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(c.App.LogLevel)); err != nil {
		errs = append(errs, fmt.Errorf("app.log_level %q is invalid", c.App.LogLevel))
	}
	if c.App.Mode == ModeLive && !c.App.AllowLive {
		errs = append(errs, errors.New("live mode sends real orders: set app.allow_live: true to confirm"))
	}
	if c.App.LogFormat != "json" && c.App.LogFormat != "text" {
		errs = append(errs, fmt.Errorf("app.log_format %q must be json or text", c.App.LogFormat))
	}
	if c.Database.URL == "" {
		errs = append(errs, errors.New("database.url is required (set TB_DATABASE__URL)"))
	}
	if c.Database.MaxConns < 1 {
		errs = append(errs, errors.New("database.max_conns must be >= 1"))
	}
	if c.Metrics.Enabled && c.Metrics.Listen == "" {
		errs = append(errs, errors.New("metrics.listen is required when metrics are enabled"))
	}

	if c.Telegram.Enabled {
		if c.Telegram.Token == "" {
			errs = append(errs, errors.New("telegram.enabled needs a token (set TB_TELEGRAM__TOKEN)"))
		}
		if len(c.Telegram.ChatIDs) == 0 {
			errs = append(errs, errors.New("telegram.chat_ids must list at least one chat"))
		}
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(c.Telegram.NotifyLevel)); err != nil {
			errs = append(errs, fmt.Errorf("telegram.notify_level %q is invalid", c.Telegram.NotifyLevel))
		}
	}
	if c.MarketData.Exchange != "binance" {
		errs = append(errs, fmt.Errorf("marketdata.exchange %q is not supported yet (only binance)", c.MarketData.Exchange))
	}
	if len(c.MarketData.Markets) == 0 {
		errs = append(errs, errors.New("marketdata.markets must not be empty"))
	}
	for _, m := range c.MarketData.Markets {
		if _, err := domain.ParseMarket(m); err != nil {
			errs = append(errs, fmt.Errorf("marketdata.markets: %w", err))
		}
	}
	if len(c.MarketData.Timeframes) == 0 {
		errs = append(errs, errors.New("marketdata.timeframes must not be empty"))
	}
	for _, tf := range c.MarketData.Timeframes {
		if _, err := domain.ParseTimeframe(tf); err != nil {
			errs = append(errs, fmt.Errorf("marketdata.timeframes: %w", err))
		}
	}
	if c.MarketData.InitialBackfill < 1 {
		errs = append(errs, errors.New("marketdata.initial_backfill must be >= 1"))
	}
	if len(c.MarketData.Symbols) == 0 && (c.Universe.Quote == "" || c.Universe.TopN < 1) {
		errs = append(errs, errors.New("universe.quote and universe.top_n are required when marketdata.symbols is empty"))
	}

	ids := map[string]bool{}
	for i, sc := range c.Strategies {
		switch {
		case sc.ID == "" || sc.Name == "":
			errs = append(errs, fmt.Errorf("strategies[%d]: id and name are required", i))
		case ids[sc.ID]:
			errs = append(errs, fmt.Errorf("strategies[%d]: duplicate id %q", i, sc.ID))
		}
		ids[sc.ID] = true
		if sc.Exchange == "" {
			errs = append(errs, fmt.Errorf("strategies[%d] (%s): exchange is required", i, sc.ID))
		}
		if _, err := domain.ParseMarket(sc.Market); err != nil {
			errs = append(errs, fmt.Errorf("strategies[%d] (%s): %w", i, sc.ID, err))
		}
	}
	if c.Risk.MaxLeverage < 1 {
		errs = append(errs, errors.New("risk.max_leverage must be >= 1"))
	}
	if c.Risk.RiskPerTrade <= 0 || c.Risk.RiskPerTrade > 0.1 {
		errs = append(errs, errors.New("risk.risk_per_trade must be in (0, 0.1]"))
	}
	if c.Backtest.ExecTimeframe != "" {
		if _, err := domain.ParseTimeframe(c.Backtest.ExecTimeframe); err != nil {
			errs = append(errs, fmt.Errorf("backtest.exec_timeframe: %w", err))
		}
	}

	for name, ex := range c.Exchanges {
		if !ex.Enabled {
			continue
		}
		// Guard against sending real orders by accident, and against a
		// "live" run silently trading on testnet.
		switch {
		case c.App.Mode == ModeTestnet && !ex.Testnet:
			errs = append(errs, fmt.Errorf("exchanges.%s.testnet must be true in testnet mode", name))
		case c.App.Mode == ModeLive && ex.Testnet:
			errs = append(errs, fmt.Errorf("exchanges.%s.testnet must be false in live mode", name))
		}
		if (c.App.Mode == ModeTestnet || c.App.Mode == ModeLive) && (ex.APIKey == "" || ex.APISecret == "") {
			errs = append(errs, fmt.Errorf("exchanges.%s needs API credentials in %s mode", name, c.App.Mode))
		}
	}
	return errors.Join(errs...)
}

// Strategy returns the strategy config with the given id.
func (c *Config) Strategy(id string) (StrategyConfig, bool) {
	for _, sc := range c.Strategies {
		if sc.ID == id {
			return sc, true
		}
	}
	return StrategyConfig{}, false
}

// EnabledExchanges returns the names of enabled exchanges.
func (c *Config) EnabledExchanges() []string {
	var names []string
	for name, ex := range c.Exchanges {
		if ex.Enabled {
			names = append(names, name)
		}
	}
	return names
}
