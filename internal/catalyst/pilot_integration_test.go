//go:build integration

package catalyst

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestPilotPostgresRestartAndLease(t *testing.T) {
	dsn := os.Getenv("TB_TEST_CATALYST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TB_TEST_CATALYST_DATABASE_URL to an isolated *_test database")
	}
	pool, err := storage.Open(t.Context(), dsn, 5)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var db string
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT current_database()").Scan(&db))
	require.True(t, strings.HasSuffix(db, "_test"), "never run the fixture on a production database")
	m, err := storage.NewMigrator(pool)
	require.NoError(t, err)
	require.NoError(t, m.Up(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil))))
	require.NoError(t, m.Close())
	var existing int
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT count(*) FROM managed_wallets WHERE exchange='binance' AND mode='paper'").Scan(&existing))
	require.Zero(t, existing, "test needs an empty isolated Binance paper namespace")
	key := ""
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, "DELETE FROM managed_archive WHERE wallet_id=$1", key)
		_, _ = pool.Exec(ctx, "DELETE FROM managed_journal WHERE wallet_id=$1", key)
		_, _ = pool.Exec(ctx, "DELETE FROM managed_wallets WHERE id=$1", key)
	})
	pilot, repo := exercisePilotLifecycle(t, func(spec managed.Spec) (managed.Repository, error) {
		key = spec.Key()
		return managed.Init(t.Context(), pool, spec)
	})
	state, err := repo.Load(t.Context())
	require.NoError(t, err)
	require.True(t, state.Inventory("").IsZero())
	var fills int
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT count(*) FROM managed_journal WHERE wallet_id=$1 AND operation='execution'", key).Scan(&fills))
	require.GreaterOrEqual(t, fills, 3)
	require.GreaterOrEqual(t, len(state.Executions), 3)
	lease, err := managed.Acquire(t.Context(), pool, key)
	require.NoError(t, err)
	defer lease.Close(context.Background())
	other, err := managed.Acquire(t.Context(), pool, key)
	require.Error(t, err)
	require.Nil(t, other)
	require.NoError(t, lease.Check(t.Context()))
	require.NoError(t, pilot.Runtime.End(t.Context(), "stopped"))
}
