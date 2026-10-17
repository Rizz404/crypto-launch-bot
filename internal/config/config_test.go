package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	got, err := readDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"TB_A=1", "TB_B=two words", "TB_C=x=y"}, got)

	got, err = readDotEnv(filepath.Join(t.TempDir(), "missing.env"))
	require.NoError(t, err)
	assert.Nil(t, got)

	_, err = readDotEnv(writeFile(t, "bad.env", "NOEQUALS\n"))
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
