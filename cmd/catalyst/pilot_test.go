package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPilotRejectsP2DatabaseAndMissingConfigBeforeNetwork(t *testing.T) {
	for _, dsn := range []string{"", "postgres://bot:fixture@db/tradingbot_research_stage_two", "postgres://catalyst:fixture@db/tradingbot", "postgres://bot:fixture@db/tradingbot_catalyst_paper_v1"} {
		require.Error(t, validatePilotDatabase(dsn))
	}
	require.NoError(t, validatePilotDatabase("postgres://catalyst:fixture@db/tradingbot_catalyst_paper_v1"))
	policy, err := filepath.Abs("../../configs/catalyst.policy.json")
	require.NoError(t, err)
	t.Setenv("TB_CATALYST_DATABASE_URL", "")
	var out bytes.Buffer
	err = run(t.Context(), []string{"-action", "run", "-policy", policy, "-env-file", filepath.Join(t.TempDir(), "missing.env")}, &out)
	require.ErrorContains(t, err, "its own")
	require.Empty(t, out.String())
}
