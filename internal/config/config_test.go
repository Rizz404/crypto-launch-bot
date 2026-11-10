package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

const baseYAML = `
app:
  mode: %s
database:
  url: postgres://bot:bot@localhost:5432/tradingbot
exchanges:
  binance:
    enabled: true
    testnet: %t
`

func TestManagedExamplesRemainPaperAndSingleCapital(t *testing.T) {
	for _, name := range []string{"managed.example.yaml", "managed.trading.example.yaml", "managed.eth-paper.yaml", "managed.research-stage-one.yaml", "managed.research-stage-two.yaml", "managed.research-stage-three.yaml"} {
		cfg, err := load(filepath.Join("..", "..", "configs", name), []string{"TB_DATABASE__URL=postgres://unused@localhost/unused"})
		require.NoError(t, err)
		require.Equal(t, ModePaper, cfg.App.Mode)
		require.False(t, cfg.App.AllowLive)
		require.Len(t, cfg.Wallets, 1)
		require.Empty(t, cfg.Strategies)
	}
}

func TestResearchStageOneUsesBoundedSharedCapital(t *testing.T) {
	cfg, err := load(filepath.Join("..", "..", "configs", "managed.research-stage-one.yaml"), []string{"TB_DATABASE__URL=postgres://unused@localhost/unused"})
	require.NoError(t, err)
	require.False(t, cfg.Telegram.Enabled)
	w := cfg.Wallets[0]
	require.Len(t, w.Instances, 3)
	remaining := w.Capital
	for _, i := range w.Instances {
		remaining = remaining.Sub(i.Allocation)
		require.Len(t, i.Strategies, 3)
		require.Equal(t, 1, i.Risk.MaxOpenPositions)
		for _, b := range i.Strategies {
			require.Equal(t, "trendspot", b.Name)
			require.Equal(t, 1, b.Risk.MaxLeverage)
		}
	}
	require.True(t, remaining.IsZero(), "allocations must use one wallet capital without duplication")
	require.Equal(t, []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}, cfg.MarketData.Symbols)
}

func TestResearchStageTwoUsesComparableRiskAndExplicitCapital(t *testing.T) {
	cfg, err := load(filepath.Join("..", "..", "configs", "managed.research-stage-two.yaml"), []string{"TB_DATABASE__URL=postgres://unused@localhost/unused"})
	require.NoError(t, err)
	require.False(t, cfg.Telegram.Enabled)
	w := cfg.Wallets[0]
	require.Equal(t, "research-stage-two", w.ID)
	require.Equal(t, "747.83957456237531", w.Capital.String())
	require.Len(t, w.Instances, 6)
	require.Equal(t, 6, w.Risk.MaxOpenPositions)
	remaining := w.Capital
	for _, i := range w.Instances {
		remaining = remaining.Sub(i.Allocation)
		require.Len(t, i.Strategies, 3)
		require.Equal(t, 1, i.Risk.MaxOpenPositions)
		for _, b := range i.Strategies {
			require.Equal(t, i.Risk, b.Risk)
			require.Equal(t, 1, b.Risk.MaxLeverage)
		}
	}
	require.True(t, remaining.IsZero(), "every allocation must come from the explicit virtual capital")
	for _, i := range w.Instances[3:] {
		require.True(t, i.Allocation.Equal(w.Instances[1].Allocation))
		require.Equal(t, w.Instances[1].Risk, i.Risk)
		for _, b := range i.Strategies {
			require.Equal(t, "emacross", b.Name)
			require.Equal(t, false, b.Params["allow_short"])
		}
	}
	require.Equal(t, []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}, cfg.MarketData.Symbols)
	require.Equal(t, []string{"1m", "4h"}, cfg.MarketData.Timeframes)
}

func TestResearchStageThreeKeepsIndependentComparableCases(t *testing.T) {
	cfg, err := load(filepath.Join("..", "..", "configs", "managed.research-stage-three.yaml"), []string{"TB_DATABASE__URL=postgres://unused@localhost/unused"})
	require.NoError(t, err)
	old, err := load(filepath.Join("..", "..", "configs", "managed.research-stage-two.yaml"), []string{"TB_DATABASE__URL=postgres://unused@localhost/unused"})
	require.NoError(t, err)
	require.False(t, cfg.Telegram.Enabled)
	w := cfg.Wallets[0]
	require.Equal(t, "research-stage-three", w.ID)
	require.Equal(t, "664.74628849988916", w.Capital.String())
	require.Len(t, w.Instances, 6)
	remaining := w.Capital
	cases := map[string]bool{}
	for _, i := range w.Instances {
		require.Equal(t, "110.79104808331486", i.Allocation.String())
		remaining = remaining.Sub(i.Allocation)
		require.Len(t, i.Strategies, 1)
		require.Equal(t, w.Instances[0].Risk, i.Risk)
		require.Equal(t, 1, i.Risk.MaxOpenPositions)
		require.Equal(t, 0.25, i.Risk.MaxExposurePct)
		b := i.Strategies[0]
		require.Equal(t, i.Risk, b.Risk)
		require.Len(t, b.Symbols, 1)
		require.Contains(t, []string{"BTCUSDT", "ETHUSDT"}, b.Symbols[0])
		key := fmt.Sprint(b.Params["timeframe"], ":", b.Symbols[0])
		require.False(t, cases[key], "duplicate case %s", key)
		cases[key] = true
		var baseline map[string]any
		for _, oldInstance := range old.Wallets[0].Instances {
			if oldInstance.ID == "ema-trend-1m" && b.Name == "emacross" || oldInstance.ID == "tren-2000k" && b.Name == "trendspot" {
				if b.Name == "trendspot" {
					baseline = oldInstance.Strategies[2].Params
				} else {
					baseline = oldInstance.Strategies[0].Params
				}
			}
		}
		require.NotNil(t, baseline)
		for k, v := range baseline {
			if k != "timeframe" {
				require.Equal(t, v, b.Params[k], "parameter %s in %s", k, i.ID)
			}
		}
		require.Len(t, b.Params, len(baseline))
	}
	require.True(t, remaining.IsZero())
	require.Equal(t, []string{"1m", "15m", "1h", "4h"}, cfg.MarketData.Timeframes)
}

func TestLoadDefaultsAndEnvOverride(t *testing.T) {
	path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, "paper", true))

	cfg, err := load(path, []string{
		"TB_APP__LOG_LEVEL=debug",
		"TB_EXCHANGES__BINANCE__API_KEY=key123",
		"UNRELATED=1",
	})
	require.NoError(t, err)

	assert.Equal(t, ModePaper, cfg.App.Mode)
	assert.Equal(t, "debug", cfg.App.LogLevel)
	assert.Equal(t, "json", cfg.App.LogFormat, "default kept")
	assert.Equal(t, int32(10), cfg.Database.MaxConns, "default kept")
	assert.Equal(t, "127.0.0.1:9100", cfg.Metrics.Listen, "default kept")
	assert.Equal(t, []string{"bStocks"}, cfg.Universe.ExcludeTags, "tokenized stocks are excluded by default")
	assert.True(t, cfg.Telegram.FX, "USD/IDR conversion in messages is on by default")
	assert.True(t, cfg.Exchanges["binance"].Testnet)
	assert.Equal(t, "key123", cfg.Exchanges["binance"].APIKey.Reveal())
	assert.Equal(t, []string{"binance"}, cfg.EnabledExchanges())
	assert.Equal(t, []string{"futures"}, cfg.MarketData.Markets, "default kept")
	assert.Equal(t, 20, cfg.Universe.TopN, "default kept")
	assert.Contains(t, cfg.Universe.ExcludeBases, "USDC")
}

func TestValidateMarketData(t *testing.T) {
	path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, "paper", true)+`
marketdata:
  markets: [spot, margin]
  timeframes: [1m, 1M]
`)
	_, err := load(path, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown market "margin"`)
	assert.Contains(t, err.Error(), `unsupported timeframe "1M"`)
}

func TestLoadRejectsSecretsInFile(t *testing.T) {
	path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, "paper", true)+"    api_secret: oops\n")

	_, err := load(path, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TB_EXCHANGES__BINANCE__API_SECRET")
}

func TestValidateModeGuards(t *testing.T) {
	creds := []string{"TB_EXCHANGES__BINANCE__API_KEY=k", "TB_EXCHANGES__BINANCE__API_SECRET=s"}
	tests := []struct {
		name    string
		mode    string
		testnet bool
		environ []string
		wantErr string
	}{
		{"paper without creds", "paper", false, nil, ""},
		{"testnet ok", "testnet", true, creds, ""},
		{"testnet mode on mainnet", "testnet", false, creds, "testnet must be true"},
		{"live mode on testnet", "live", true, creds, "testnet must be false"},
		{"live ok", "live", false, append(creds, "TB_APP__ALLOW_LIVE=true"), ""},
		{"live without allow_live", "live", false, creds, "allow_live"},
		{"testnet without creds", "testnet", true, nil, "needs API credentials"},
		{"unknown mode", "yolo", true, nil, "app.mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, tt.mode, tt.testnet))
			_, err := load(path, tt.environ)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateRequiresDatabaseURL(t *testing.T) {
	path := writeFile(t, "config.yaml", "app:\n  mode: paper\n")
	_, err := load(path, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TB_DATABASE__URL")
}

func TestSecretNeverPrints(t *testing.T) {
	s := Secret("hunter2")
	assert.Equal(t, "***", s.String())
	assert.Equal(t, "***", fmt.Sprint(s))
	assert.Equal(t, "***", s.LogValue().String())
	assert.Equal(t, "", Secret("").String())
}

func TestReadDotEnv(t *testing.T) {
	path := writeFile(t, ".env", "# comment\n\nexport TB_A=1\nTB_B=\"two words\"\nTB_C='x=y'\n")
	got, err := ReadDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"TB_A=1", "TB_B=two words", "TB_C=x=y"}, got)

	got, err = ReadDotEnv(filepath.Join(t.TempDir(), "missing.env"))
	require.NoError(t, err)
	assert.Nil(t, got)

	_, err = ReadDotEnv(writeFile(t, "bad.env", "NOEQUALS\n"))
	assert.Error(t, err)
}

func TestListOverridesReplaceDefaults(t *testing.T) {
	path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, "paper", true)+`
marketdata:
  timeframes: [4h]
universe:
  exclude_bases: [DOGE]
`)
	cfg, err := load(path, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"4h"}, cfg.MarketData.Timeframes)
	assert.Equal(t, []string{"DOGE"}, cfg.Universe.ExcludeBases)
	assert.Equal(t, "USDC", DefaultExcludeBases[0], "package default not mutated")
}

func TestStrategyParamsAndFeeOverridesKeepDefaults(t *testing.T) {
	path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, "paper", true)+`
strategies:
  - id: sample
    name: emacross
    exchange: binance
    market: futures
    symbols: [BTCUSDT]
    params: {fast: 5, timeframe: 4h}
backtest:
  fees:
    futures: {maker: 0.0001}
`)
	cfg, err := load(path, nil)
	require.NoError(t, err)
	sc, ok := cfg.Strategy("sample")
	require.True(t, ok)
	assert.Equal(t, "4h", sc.Params["timeframe"])
	assert.EqualValues(t, 5, sc.Params["fast"])
	assert.Equal(t, 0.0001, cfg.Backtest.Fees["futures"].Maker)
	assert.Equal(t, 0.0005, cfg.Backtest.Fees["futures"].Taker)
	assert.Equal(t, 0.001, cfg.Backtest.Fees["spot"].Taker)
}

func TestWebRequiresPasswordOffLoopbackAndRejectsYAMLSecrets(t *testing.T) {
	for _, listen := range []string{"0.0.0.0:8080", "[::]:8080"} {
		path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, "paper", true)+fmt.Sprintf("web:\n  enabled: true\n  listen: %q\n", listen))
		_, err := load(path, nil)
		assert.ErrorContains(t, err, "TB_WEB__PASSWORD")
		_, err = load(path, []string{"TB_WEB__PASSWORD=test-password"})
		assert.NoError(t, err)
	}
	for _, secret := range []string{"web:\n  password: unsafe\n", "telegram:\n  token: unsafe\n", "    api_key: unsafe\n"} {
		path := writeFile(t, "config.yaml", fmt.Sprintf(baseYAML, "paper", true)+secret)
		_, err := load(path, nil)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "unsafe")
	}
}

func TestWalletModesRunTogetherWithSeparateCredentials(t *testing.T) {
	const yaml = `
app: {mode: paper, allow_live: %t}
database: {url: postgres://bot:bot@localhost:5432/tradingbot}
web: {enabled: %t, listen: '127.0.0.1:18080'}
exchanges:
  binance: {enabled: true, testnet: false}
marketdata: {exchange: binance, markets: [spot], symbols: [BTCUSDT], timeframes: [1m]}
wallets:
  - {id: paper, exchange: binance, quote: USDT, capital: '100'}
  - {id: test, mode: testnet, exchange: binance, quote: USDT, capital: '100'}
  - {id: real, mode: live, allow_live: %t, exchange: binance, quote: USDT, capital: '10'}
`
	testnet := []string{"TB_EXCHANGES__BINANCE__TESTNET_API_KEY=tk", "TB_EXCHANGES__BINANCE__TESTNET_API_SECRET=ts"}
	live := []string{"TB_EXCHANGES__BINANCE__API_KEY=lk", "TB_EXCHANGES__BINANCE__API_SECRET=ls"}
	tests := []struct {
		name               string
		appLive, walletLiv bool
		web                bool
		environ            []string
		wantErr            string
	}{
		{"all modes", true, true, false, append(slices.Clone(testnet), live...), ""},
		{"live wallet not confirmed", true, false, false, append(slices.Clone(testnet), live...), "allow_live"},
		{"app not confirmed", false, true, false, append(slices.Clone(testnet), live...), "allow_live"},
		{"no testnet key", true, true, false, live, "needs testnet API credentials"},
		{"no live key", true, true, false, testnet, "needs live API credentials"},
		{"live on web uses the account login, not env", true, true, true, append(slices.Clone(testnet), live...), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := load(writeFile(t, "config.yaml", fmt.Sprintf(yaml, tt.appLive, tt.web, tt.walletLiv)), tt.environ)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{"paper", "testnet", "live"}, []string{cfg.Wallets[0].Mode, cfg.Wallets[1].Mode, cfg.Wallets[2].Mode})
			key, _ := cfg.Exchanges["binance"].Credentials(true)
			assert.Equal(t, "tk", key.Reveal())
			key, _ = cfg.Exchanges["binance"].Credentials(false)
			assert.Equal(t, "lk", key.Reveal())
		})
	}
}

func TestCredentialsNeverCrossNetworks(t *testing.T) {
	mainnet := ExchangeConfig{APIKey: "main", APISecret: "s"}
	key, _ := mainnet.Credentials(true)
	assert.Empty(t, key, "a mainnet key must not be sent to testnet")
	legacy := ExchangeConfig{Testnet: true, APIKey: "test", APISecret: "s"}
	key, _ = legacy.Credentials(false)
	assert.Empty(t, key, "a testnet key must not be used for live orders")
	key, _ = legacy.Credentials(true)
	assert.Equal(t, "test", key.Reveal())
}
