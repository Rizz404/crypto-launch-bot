package managed

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// TestDashboardPreview serves fixtures only. It never starts a runtime/feed,
// connects a database, loads .env, or creates exchange clients.
func TestDashboardPreview(t *testing.T) {
	addr := os.Getenv("TB_DASHBOARD_PREVIEW")
	if addr == "" {
		t.Skip("set TB_DASHBOARD_PREVIEW=127.0.0.1:18082 for fixture UI preview")
	}
	repo, err := NewMemory(spec())
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, repo.Update(t.Context(), "fixture", "fixture", nil, func(s *State) error {
		s.StartedAt = now.Add(-24 * time.Hour)
		s.Mark("ETHUSDT", d("100"))
		s.PriceAt["ETHUSDT"] = now
		for n := 0; n < 24; n++ {
			s.Instances["a"].Cash = d("500").Add(d("0.5").Mul(decimal.NewFromInt(int64(n))))
			s.Sample(now.Add(time.Duration(n-24) * time.Hour))
		}
		s.Instances["a"].Cash = d("480")
		s.Holding("a/one", "ETHUSDT").Quantity = d("0.2")
		s.Holding("a/one", "ETHUSDT").Cost = d("20")
		s.Mark("ETHUSDT", d("105"))
		s.Sample(now)
		s.Observe(now)
		s.Bindings["a/one"].LastNotice = "Menunggu candle dan sinyal strategi berikutnya"
		return nil
	}))
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), nil, nil)
	require.NoError(t, err)
	srv := &http.Server{Addr: addr, Handler: NewHTTP(map[string]*Runtime{"fixture-paper": rt}, "", nil), ReadHeaderTimeout: 5 * time.Second}
	t.Cleanup(func() { _ = srv.Close() })
	t.Log("fixture dashboard: http://" + addr)
	require.ErrorIs(t, srv.ListenAndServe(), http.ErrServerClosed)
}
