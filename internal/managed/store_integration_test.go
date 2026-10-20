//go:build integration

package managed

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/rizz404/trading-bot/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestPGWalletJournalRecoveryAndLease(t *testing.T) {
	u := os.Getenv("TB_TEST_MANAGED_DATABASE_URL")
	if u == "" {
		t.Skip("set TB_TEST_MANAGED_DATABASE_URL to an isolated migrated *_test database")
	}
	pool, err := storage.Open(t.Context(), u, 8)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var db string
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT current_database()").Scan(&db))
	require.Contains(t, db, "_test")
	m, err := storage.NewMigrator(pool)
	require.NoError(t, err)
	require.NoError(t, m.Up(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil))))
	require.NoError(t, m.Close())
	sp := spec()
	sp.ID = "integration-" + nonce()
	sp.Exchange = sp.ID
	r, err := Init(t.Context(), pool, sp)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM managed_journal WHERE wallet_id=$1", r.Key)
		_, _ = pool.Exec(context.Background(), "DELETE FROM managed_wallets WHERE id=$1", r.Key)
	})
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for n := 0; n < 40; n++ {
		wg.Go(func() {
			err := r.Update(t.Context(), nonce(), "transfer", nil, func(s *State) error { return s.Transfer("reserve", "a", d("5")) })
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	require.Equal(t, 20, success)
	require.NoError(t, r.Update(t.Context(), "idempotent", "transfer", "same", func(s *State) error { return s.Transfer("a", "b", d("10")) }))
	require.NoError(t, r.Update(t.Context(), "idempotent", "transfer", "same", func(*State) error { panic("must not run") }))
	require.Error(t, r.Update(t.Context(), "idempotent", "transfer", "different", func(*State) error { return nil }))
	loaded, err := Init(t.Context(), pool, sp)
	require.NoError(t, err)
	s, err := loaded.Load(t.Context())
	require.NoError(t, err)
	require.Equal(t, "1000", s.Equity().String())
	require.Equal(t, "590", s.Instances["a"].Cash.String())
	var count int
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT count(*) FROM managed_journal WHERE wallet_id=$1", r.Key).Scan(&count))
	require.Equal(t, 21, count)
	lease, err := Acquire(t.Context(), pool, r.Key)
	require.NoError(t, err)
	other, err := Acquire(t.Context(), pool, r.Key)
	require.Error(t, err)
	require.Nil(t, other)
	require.NoError(t, lease.Check(t.Context()))
	_, err = lease.conn.Exec(t.Context(), "SELECT pg_advisory_unlock_all()")
	require.NoError(t, err)
	require.Error(t, lease.Check(t.Context()))
	lease.Close(t.Context())
}
