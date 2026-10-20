package managed

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/risk"
	_ "github.com/rizz404/trading-bot/internal/strategy/all"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func generous() risk.Config {
	c := risk.DefaultConfig()
	c.MaxPositionPct = 1
	c.MaxExposurePct = 1
	c.MaxOpenPositions = 10
	c.MaxLeverage = 1
	return c
}
func spec() Spec {
	return Spec{ID: "test", Exchange: "sim", Mode: "paper", Quote: "USDT", Capital: d("1000"), Risk: generous(), Instances: []InstanceSpec{
		{ID: "a", Name: "A", Allocation: d("500"), Risk: generous(), Strategies: []BindingSpec{{ID: "one", Name: "dca", Symbols: []string{"ETHUSDT"}, Risk: generous(), Params: map[string]any{"quote_amount": 20, "max_buys": 2, "interval": "1h", "take_profit": 0.03}}}},
		{ID: "b", Name: "B", Allocation: d("400"), Risk: generous(), Strategies: []BindingSpec{{ID: "two", Name: "dca", Symbols: []string{"ETHUSDT"}, Risk: generous(), Params: map[string]any{"quote_amount": 30, "max_buys": 2, "interval": "1h", "take_profit": 0.03}}}},
	}}
}
func order(id string, side domain.Side, qty, price string) domain.Order {
	return domain.Order{ClientID: id, Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Side: side, Type: domain.OrderLimit, Quantity: d(qty), Price: d(price), Status: domain.OrderNew}
}
func fill(o domain.Order, id, qty, price string) domain.Fill {
	return domain.Fill{OrderClientID: o.ClientID, TradeID: id, Exchange: o.Exchange, Market: o.Market, Symbol: o.Symbol, Side: o.Side, Quantity: d(qty), Price: d(price), Time: time.Now()}
}
func TestTransferAtomicConcurrentIdempotent(t *testing.T) {
	r, err := NewMemory(spec())
	require.NoError(t, err)
	var wg sync.WaitGroup
	for n := 0; n < 50; n++ {
		wg.Go(func() {
			_ = r.Update(t.Context(), nonce(), "transfer", nil, func(s *State) error { return s.Transfer("reserve", "a", d("5")) })
		})
	}
	wg.Wait()
	s, err := r.Load(t.Context())
	require.NoError(t, err)
	require.True(t, s.Reserve.IsZero())
	require.Equal(t, "600", s.Instances["a"].Cash.String())
	require.Equal(t, "1000", s.Equity().String())
	err = r.Update(t.Context(), "same", "transfer", "payload", func(s *State) error { return s.Transfer("a", "b", d("10")) })
	require.NoError(t, err)
	require.NoError(t, r.Update(t.Context(), "same", "transfer", "payload", func(*State) error { return errors.New("must not run twice") }))
	require.Error(t, r.Update(t.Context(), "same", "transfer", "different", func(*State) error { return nil }))
	require.Error(t, r.Update(t.Context(), nonce(), "rollback", nil, func(s *State) error { s.Reserve = d("9999"); return errors.New("failure") }))
	s, _ = r.Load(t.Context())
	require.Equal(t, "1000", s.Equity().String())
}
func TestReservationsOwnershipOCOAndReplay(t *testing.T) {
	s, err := NewState(spec())
	require.NoError(t, err)
	s.Mark("ETHUSDT", d("100"))
	o := order("buy", domain.Buy, "3", "100")
	require.NoError(t, s.ReserveOrder("a/one", o, "", d("0.001")))
	require.Error(t, s.ReserveOrder("a/one", order("too", domain.Buy, "3", "100"), "", d("0")))
	f := fill(o, "1", "1", "100")
	f.Fee = d("0.1")
	ok, err := s.Fill(f)
	require.NoError(t, err)
	require.True(t, ok)
	q, _ := s.Reserved("a", "", "")
	require.Equal(t, "200.2", q.String())
	f = fill(o, "2", "1", "100")
	f.Fee = d("0.1")
	_, err = s.Fill(f)
	require.NoError(t, err)
	q, _ = s.Reserved("a", "", "")
	require.Equal(t, "100.1", q.String(), "second partial must not release excess reservation")
	_, err = s.Fill(f)
	require.NoError(t, err)
	require.Equal(t, "2", s.Holding("a/one", "ETHUSDT").Quantity.String())
	require.Error(t, s.ReserveOrder("b/two", order("steal", domain.Sell, "1", "100"), "", d("0")))
	stop, target := order("stop", domain.Sell, "2", "95"), order("target", domain.Sell, "2", "105")
	require.NoError(t, s.ReserveOrder("a/one", stop, "oco", d("0")))
	require.NoError(t, s.ReserveOrder("a/one", target, "oco", d("0")))
	_, reserved := s.Reserved("", "a/one", "ETHUSDT")
	require.Equal(t, "2", reserved.String(), "OCO only reserves owned inventory once")
	stop.Status = domain.OrderCanceled
	s.UpdateOrder(stop)
	_, reserved = s.Reserved("", "a/one", "ETHUSDT")
	require.Equal(t, "2", reserved.String(), "other leg stays reserved until confirmation")
	target.Status = domain.OrderCanceled
	s.UpdateOrder(target)
	_, reserved = s.Reserved("", "a/one", "ETHUSDT")
	require.True(t, reserved.IsZero())
}
func TestBaseAndThirdAssetFees(t *testing.T) {
	s, _ := NewState(spec())
	s.Mark("ETHUSDT", d("100"))
	o := order("base", domain.Buy, "1", "100")
	require.NoError(t, s.ReserveOrder("a/one", o, "", d("0.01")))
	f := fill(o, "base1", "1", "100")
	f.BaseFee = d("0.001")
	f.Fee = d("0.1")
	f.FeeAsset = "ETH"
	_, err := s.Fill(f)
	require.NoError(t, err)
	require.Equal(t, "0.999", s.Holding("a/one", "ETHUSDT").Quantity.String())
	require.Equal(t, "400", s.Instances["a"].Cash.String())
	o = order("bnb", domain.Buy, "1", "100")
	require.NoError(t, s.ReserveOrder("a/one", o, "", d("0.01")))
	f = fill(o, "bnb1", "1", "100")
	f.FeeAsset = "BNB"
	f.Commission = d("0.001")
	_, err = s.Fill(f)
	require.NoError(t, err)
	require.NotEmpty(t, s.FeeIssue)
	s.Mark("BNBUSDT", d("500"))
	require.Equal(t, "299.5", s.Instances["a"].Cash.String())
	require.Equal(t, "0.5", s.FeeQuote.String())
	require.Empty(t, s.PendingFees)
	require.Empty(t, s.FeeIssue)
}
func TestRuntimeSharedCapitalSameSymbolRestartAndScopedPause(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	r, err := NewMemory(spec())
	require.NoError(t, err)
	ins := []domain.Instrument{{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", TickSize: d("0.01"), StepSize: d("0.001"), MinQty: d("0.001"), MinNotional: d("5")}}
	makeRuntime := func() *Runtime {
		sim := broker.NewSim(broker.SimConfig{Exchange: "sim", Clock: func() time.Time { return now }})
		rt, err := NewRuntime(t.Context(), r, sim, ins, d("0.001"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
		require.NoError(t, err)
		require.NoError(t, rt.Start(t.Context()))
		rt.AllowGaps = true // This test deliberately advances the timer clock by one hour.
		return rt
	}
	rt := makeRuntime()
	bar := func() domain.Candle {
		return domain.Candle{Exchange: "sim", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Add(-time.Minute), Open: d("100"), High: d("100"), Low: d("100"), Close: d("100"), Closed: true}
	}
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	require.NoError(t, rt.Tick(t.Context(), now))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	s, err := rt.Snapshot(t.Context())
	require.NoError(t, err)
	require.Equal(t, "0.2", s.Holding("a/one", "ETHUSDT").Quantity.String())
	require.Equal(t, "0.3", s.Holding("b/two", "ETHUSDT").Quantity.String())
	require.Equal(t, "1000", s.Equity().String())
	require.NoError(t, rt.Command(t.Context(), Command{ID: "pause", Operation: "pause", Scope: "a"}))
	rt = makeRuntime()
	now = now.Add(time.Hour)
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	require.NoError(t, rt.Tick(t.Context(), now))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	s, _ = rt.Snapshot(t.Context())
	require.Equal(t, "0.2", s.Holding("a/one", "ETHUSDT").Quantity.String())
	require.Equal(t, "0.6", s.Holding("b/two", "ETHUSDT").Quantity.String())
	require.Equal(t, "1000", s.Equity().String())
	require.NoError(t, rt.Command(t.Context(), Command{ID: "close-a", Operation: "flatten", Scope: "a", Confirm: true}))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(t.Context(), bar(), true))
	s, _ = rt.Snapshot(t.Context())
	require.True(t, s.Holding("a/one", "ETHUSDT").Quantity.IsZero())
	require.Equal(t, "0.6", s.Holding("b/two", "ETHUSDT").Quantity.String())
}
func TestHTTPAuthCSRFAndEscaping(t *testing.T) {
	repo, _ := NewMemory(spec())
	now := time.Now()
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), nil, d("0"), func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	h := NewHTTP(map[string]*Runtime{"test": rt}, "pass", nil)
	get := httptest.NewRequest("GET", "http://localhost/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, get)
	require.Equal(t, 401, w.Code)
	get.SetBasicAuth("user", "pass")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, get)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "Portofolio bot")
	require.NotContains(t, w.Header().Get("Content-Security-Policy"), "unsafe-inline")
	req := httptest.NewRequest("POST", "http://localhost/control", strings.NewReader(url.Values{"operation": {"pause"}, "scope": {"a"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("user", "pass")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	require.NoError(t, repo.Update(t.Context(), "xss", "rename", nil, func(s *State) error { s.Instances["a"].Name = "<script>alert(1)</script>"; return nil }))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, get)
	require.NotContains(t, w.Body.String(), "<script>")
	require.Contains(t, w.Body.String(), "&lt;script&gt;")
}

func TestDashboardChartAndTimes(t *testing.T) {
	at := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	// One hour, then a nine-hour gap: the middle point must sit at 1/10 of the width.
	path := chartPath([]EquitySample{{At: at, Account: d("100")}, {At: at.Add(time.Hour), Account: d("110")}, {At: at.Add(10 * time.Hour), Account: d("105")}})
	require.Equal(t, "M10.00 180.00 L108.00 20.00 L990.00 100.00 ", path)
	require.Equal(t, "belum ada", timestamp(time.Time{}))
	require.Equal(t, "2026-10-20 07:00:00 UTC", timestamp(at.Add(7*time.Hour+123).In(time.FixedZone("WIB", 7*3600))))

	repo, _ := NewMemory(spec())
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), nil, d("0"), func() time.Time { return at }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, repo.Update(t.Context(), "samples", "samples", nil, func(s *State) error {
		s.Sample(at)
		s.Sample(at.Add(time.Hour))
		return nil
	}))
	w := httptest.NewRecorder()
	NewHTTP(map[string]*Runtime{"test": rt}, "", nil).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/", nil))
	body := w.Body.String()
	require.Contains(t, body, "Reconciliation exchange terakhir: belum ada")
	require.Contains(t, body, "2026-10-20 00:00:00 UTC → 2026-10-20 01:00:00 UTC")
	require.NotContains(t, body, "0001-01-01")
	require.NotContains(t, body, "+0000 UTC")
}

func TestDashboardFormRejectionAndRate(t *testing.T) {
	repo, _ := NewMemory(spec())
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), nil, d("0"), time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	h := NewHTTP(map[string]*Runtime{"test": rt}, "", nil)
	h.RateLine = func() string { return "Kurs 1 USDT ≈ Rp16.000" }
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/", nil))
	require.Contains(t, w.Body.String(), "Kurs 1 USDT ≈ Rp16.000")

	req := httptest.NewRequest("POST", "http://localhost/control?wallet=test", strings.NewReader(url.Values{"csrf": {h.token}, "id": {"x"}, "operation": {"flatten"}, "scope": {"a"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, 303, w.Code)
	location := w.Header().Get("Location")
	require.Contains(t, location, "error=")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+location, nil))
	require.Contains(t, w.Body.String(), `role="alert">Aksi ditolak: explicit confirmation required`)
	s, err := rt.Snapshot(t.Context())
	require.NoError(t, err)
	require.Equal(t, "running", string(s.Instances["a"].Status))
}
