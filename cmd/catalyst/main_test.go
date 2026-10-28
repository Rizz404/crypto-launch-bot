package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/stretchr/testify/require"
)

func TestReportIsDefaultAndHasNoSideEffects(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "unused")
	policy, err := filepath.Abs("../../configs/catalyst.policy.json")
	require.NoError(t, err)
	var out bytes.Buffer
	require.NoError(t, run(t.Context(), []string{"-policy", policy, "-archive", archive}, &out))
	var report catalyst.Report
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))
	require.True(t, report.ShadowOnly)
	require.Nil(t, report.CostUSD)
	_, err = os.Stat(archive)
	require.True(t, os.IsNotExist(err))
}

func TestCollectRequiresSpiderWithoutHTTPFallback(t *testing.T) {
	policy, err := filepath.Abs("../../configs/catalyst.policy.json")
	require.NoError(t, err)
	dir := t.TempDir()
	archive := filepath.Join(dir, "unused")
	var out bytes.Buffer
	err = run(t.Context(), []string{"-policy", policy, "-archive", archive, "-action", "collect", "-spider-binary", filepath.Join(dir, "missing-spider")}, &out)
	require.ErrorContains(t, err, "spider is not installed")
	require.Empty(t, out.String())
	_, err = os.Stat(archive)
	require.True(t, os.IsNotExist(err))
}

func TestAnalyzeWithoutCredentialsAndReplayWithoutFixtureFailClosed(t *testing.T) {
	for _, key := range []string{"TB_CATALYST_API_KEY", "TB_CATALYST_MODEL", "TB_CATALYST_BASE_URL"} {
		t.Setenv(key, "")
	}
	policy, err := filepath.Abs("../../configs/catalyst.policy.json")
	require.NoError(t, err)
	var out bytes.Buffer
	require.Error(t, run(t.Context(), []string{"-policy", policy, "-action", "analyze"}, &out))
	require.Empty(t, out.String())
	require.Error(t, run(t.Context(), []string{"-action", "replay"}, &out))
	require.NoError(t, run(t.Context(), []string{"-h"}, &out))
}

func TestDedicatedEnvAndProcessPrecedence(t *testing.T) {
	// This test only constructs the adapter; it makes no HTTP request.
	path := filepath.Join(t.TempDir(), ".env.catalyst")
	require.NoError(t, os.WriteFile(path, []byte("TB_CATALYST_BASE_URL=https://route-gateway.fbe.co.id/v1\nTB_CATALYST_MODEL=fixture-model\nTB_CATALYST_API_KEY='fixture-key'\n"), 0600))
	for _, key := range []string{"TB_CATALYST_BASE_URL", "TB_CATALYST_MODEL", "TB_CATALYST_API_KEY"} {
		value, present := os.LookupEnv(key)
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		})
		require.NoError(t, os.Unsetenv(key))
	}
	a, err := loadAnalyst(path)
	require.NoError(t, err)
	require.Equal(t, "https://route-gateway.fbe.co.id/v1", a.BaseURL)
	require.Equal(t, "fixture-model", a.Model)
	t.Setenv("TB_CATALYST_MODEL", "process-model")
	a, err = loadAnalyst(path)
	require.NoError(t, err)
	require.Equal(t, "process-model", a.Model)
	t.Setenv("TB_CATALYST_API_KEY", "")
	_, err = loadAnalyst(path)
	require.Error(t, err, "explicit empty process key overrides file key")
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(contents), "fixture-key", "env file is not rewritten")
}
