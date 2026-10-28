package catalyst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange/binance"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type pilotFixture struct {
	pilot *Pilot
	repo  managed.Repository
	spec  managed.Spec
	ins   []domain.Instrument
	clock func() time.Time
}

// newPilotFixture wires public-source, Binance and model fixtures; Runtime is built but not started.
func newPilotFixture(t *testing.T, repository func(managed.Spec) (managed.Repository, error), model roundTrip) pilotFixture {
	t.Helper()
	p := testPolicy()
	p.Version = "fixture-" + Hash([]string{t.Name(), time.Now().String()})[:16]
	archive := Archive{Dir: t.TempDir()}
	inbox := filepath.Join(archive.Dir, "inbox")
	collector, err := NewCollector(p, archive)
	require.NoError(t, err)
	date := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	collector.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/robots.txt" {
			return response(404, "text/plain", ""), nil
		}
		return response(200, "text/html", "<main>The upgrade is scheduled for "+date+".</main>"), nil
	})}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/exchangeInfo":
			_, _ = fmt.Fprint(w, `{"symbols":[{"symbol":"ETHUSDT","status":"TRADING","baseAsset":"ETH","quoteAsset":"USDT","filters":[{"filterType":"PRICE_FILTER","tickSize":"0.01"},{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001"},{"filterType":"NOTIONAL","minNotional":"5"}]}]}`)
		case "/api/v3/ticker/24hr":
			_, _ = fmt.Fprint(w, `[{"symbol":"ETHUSDT","quoteVolume":"20000000"}]`)
		case "/bapi/asset/v2/public/asset-service/product/get-products":
			_, _ = fmt.Fprint(w, `{"data":[{"b":"ETH","tags":[]}]}`)
		case "/api/v3/ticker/bookTicker":
			_, _ = fmt.Fprint(w, `{"symbol":"ETHUSDT","bidPrice":"99.99","askPrice":"100.01","bidQty":"1000","askQty":"1000"}`)
		case "/api/v3/klines":
			var rows [][]any
			for _, c := range atrCandles(time.Now()) {
				rows = append(rows, []any{c.OpenTime.UnixMilli(), "100", "101", "99", "100", "1", c.CloseTime().UnixMilli() - 1, "100", 1})
			}
			_ = json.NewEncoder(w).Encode(rows)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	market := NewMarket(binance.New(binance.Options{Endpoints: &binance.Endpoints{SpotREST: server.URL, Web: server.URL}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	ins, err := market.Refresh(t.Context(), p, time.Now())
	require.NoError(t, err)
	analyst, err := NewAnalyst("https://example.org/v1", "fixture", "fixture-key")
	require.NoError(t, err)
	analyst.client = &http.Client{Transport: model}
	spec, err := PaperSpec(p, inbox)
	require.NoError(t, err)
	repo, err := repository(spec)
	require.NoError(t, err)
	var offset time.Duration
	clock := func() time.Time { return time.Now().UTC().Add(offset) }
	rt, err := NewPaper(t.Context(), repo, p, inbox, ins, clock)
	require.NoError(t, err)
	return pilotFixture{pilot: &Pilot{Policy: p, Archive: archive, Collector: collector, Market: market, Analyst: analyst, Runtime: rt, Inbox: inbox, CheckLease: func(context.Context) error { return nil }}, repo: repo, spec: spec, ins: ins, clock: clock}
}

func exercisePilotLifecycle(t *testing.T, repository func(managed.Spec) (managed.Repository, error)) (*Pilot, managed.Repository) {
	t.Helper()
	calls := 0
	f := newPilotFixture(t, repository, func(r *http.Request) (*http.Response, error) {
		calls++
		var request struct{ Messages []struct{ Content string } }
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return nil, err
		}
		var packet struct {
			Research ResearchContext `json:"research"`
		}
		if err := json.Unmarshal([]byte(request.Messages[1].Content), &packet); err != nil {
			return nil, err
		}
		if len(packet.Research.Events) != 1 || len(packet.Research.Market) != 1 {
			return nil, fmt.Errorf("missing event/market context")
		}
		c := packet.Research.Events[0]
		a := Analysis{Action: "PROPOSE_ENTRY", Symbol: c.Symbol, EventKey: c.Key, EventType: c.Type, EventAt: c.At, EventDateText: c.DateText, Thesis: "fixture scheduled upgrade", Counterargument: "priced in", Invalidation: "withdrawal", HoldRationale: "review outcome", Reason: "fixture primary schedule", Citations: []Citation{{EvidenceID: c.EvidenceID, Quote: c.Quote}}}
		if len(packet.Research.Positions) > 0 {
			a.Action = "HOLD"
		}
		raw, _ := json.Marshal(a)
		body, _ := json.Marshal(map[string]any{"model": "fixture-served", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(raw)}}}})
		return response(200, "application/json", string(body)), nil
	})
	pilot, repo, spec, p, inbox, ins, clock := f.pilot, f.repo, f.spec, f.pilot.Policy, f.pilot.Inbox, f.ins, f.clock
	require.NoError(t, pilot.Runtime.Start(t.Context()))
	t.Cleanup(func() { _ = pilot.Runtime.End(context.Background(), "stopped") })
	require.NoError(t, pilot.Cycle(t.Context()))
	require.Equal(t, 1, calls)
	state, err := repo.Load(t.Context())
	require.NoError(t, err)
	require.Empty(t, state.Executions, "analysis places an intent, never a fill at an old quote")
	record := func(bid, ask, qty string) {
		t.Helper()
		require.NoError(t, pilot.Runtime.RecordBook(t.Context(), domain.BookQuote{Exchange: "binance", Market: domain.Spot, Symbol: "ETHUSDT", Bid: decimal.RequireFromString(bid), Ask: decimal.RequireFromString(ask), BidQty: decimal.RequireFromString(qty), AskQty: decimal.RequireFromString(qty), At: clock()}))
	}
	record("99.99", "100.01", "1000")
	state, err = repo.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, state.Executions, 1)
	require.True(t, state.Executions[0].Fill.Price.GreaterThan(decimal.RequireFromString("100.01")))
	require.NoError(t, pilot.Cycle(t.Context()), "a held position receives a distinct HOLD review")
	require.Equal(t, 2, calls)
	require.NoError(t, pilot.Cycle(t.Context()), "unchanged evidence/holding must not spend another call")
	require.Equal(t, 2, calls)
	require.NoError(t, pilot.Runtime.End(t.Context(), "stopped"))
	repo, err = repository(spec)
	require.NoError(t, err)
	pilot.Runtime, err = NewPaper(t.Context(), repo, p, inbox, ins, clock)
	require.NoError(t, err)
	require.NoError(t, pilot.Runtime.Start(t.Context()))
	record("95", "95.01", "1")
	state, err = repo.Load(t.Context())
	require.NoError(t, err)
	require.Len(t, state.Executions, 2, "a shallow bid produces a partial exit")
	require.True(t, state.Inventory("").IsPositive(), "partial liquidation is not flat")
	record("101", "101.01", "1000")
	state, err = repo.Load(t.Context())
	require.NoError(t, err)
	require.False(t, state.Inventory("").IsPositive(), "a triggered close finishes even after a rebound")
	reserved, _ := state.Reserved("", "", "")
	require.True(t, reserved.IsZero())
	require.False(t, state.Halted)
	require.Empty(t, state.Attention)
	require.NoError(t, pilot.Runtime.Tick(t.Context(), clock()))
	record("99.99", "100.01", "1000")
	state, err = repo.Load(t.Context())
	require.NoError(t, err)
	require.True(t, state.Inventory("").IsZero(), "the consumed event must not reopen after exit")
	return pilot, repo
}

func TestPilotPublicContextToPaperEntryHoldPartialExitAndRestart(t *testing.T) {
	var repo *managed.MemoryStore
	exercisePilotLifecycle(t, func(spec managed.Spec) (managed.Repository, error) {
		if repo == nil {
			var err error
			repo, err = managed.NewMemory(spec)
			return repo, err
		}
		return repo, nil
	})
}

func memoryRepository(spec managed.Spec) (managed.Repository, error) { return managed.NewMemory(spec) }

func TestPilotWorkerKeepsFeedAndTimersWhileModelHangs(t *testing.T) {
	modelStarted := make(chan struct{}, 1)
	f := newPilotFixture(t, memoryRepository, func(r *http.Request) (*http.Response, error) {
		modelStarted <- struct{}{}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- f.pilot.Run(ctx) }()
	select {
	case <-modelStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("research never reached the model")
	}
	require.Eventually(t, func() bool { return f.pilot.Status().Healthy(f.pilot.Policy, time.Now()) }, 10*time.Second, 100*time.Millisecond)
	tick := f.pilot.Status().TickAt
	require.Eventually(t, func() bool {
		return f.pilot.Status().TickAt.After(tick) && f.pilot.Status().Healthy(f.pilot.Policy, time.Now())
	}, 5*time.Second, 100*time.Millisecond, "a hung model must not stall timers or quotes")
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	state, err := f.repo.Load(t.Context())
	require.NoError(t, err)
	require.Empty(t, state.Executions, "no fill without a completed review")
}

func TestPilotWorkerStopsWhenLeaseIsLost(t *testing.T) {
	f := newPilotFixture(t, memoryRepository, func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	var checks atomic.Int32
	f.pilot.CheckLease = func(context.Context) error {
		if checks.Add(1) > 3 {
			return errors.New("lease lost")
		}
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- f.pilot.Run(t.Context()) }()
	select {
	case err := <-done:
		require.ErrorContains(t, err, "ownership")
	case <-time.After(15 * time.Second):
		t.Fatal("worker kept running without its lease")
	}
}

func TestPilotResearchAuthAndHealth(t *testing.T) {
	p := &Pilot{Policy: testPolicy(), Archive: Archive{Dir: t.TempDir()}}
	h := p.HTTP("fixture-password-123")
	for _, path := range []string{"/research", "/research.json"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 401, w.Code)
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.SetBasicAuth("pilot", "fixture-password-123")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, 200, w.Code)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, 503, w.Code)
	p.status = PilotStatus{TickAt: time.Now(), Prices: map[string]time.Time{"ETHUSDT": time.Now()}}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "fixture-password")
	require.True(t, strings.Contains(w.Body.String(), "research_ready_is_separate"))
}

func TestResearchPageEscapesUntrustedSourceText(t *testing.T) {
	var out strings.Builder
	data := struct {
		Status PilotStatus
		Report Report
	}{PilotStatus{Reason: `<img src=x onerror=alert(1)>`}, Report{Documents: []DocumentSummary{{URL: "javascript:alert(1)", Title: "<script>alert(1)</script>"}}}}
	require.NoError(t, researchPage.Execute(&out, data))
	page := out.String()
	for _, raw := range []string{"<script>alert", "<img src=x", `href="javascript:`} {
		require.NotContains(t, page, raw)
	}
}
