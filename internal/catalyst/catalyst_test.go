package catalyst

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func testPolicy() Policy {
	return Policy{Version: "fixture-v1", Sources: []Source{{ID: "official", URL: "https://example.org/events", Primary: true, Symbols: []string{"ETHUSDT"}}}, Symbols: []string{"ETHUSDT"}, MaxCalls: 1, MaxTokens: 2048, ReviewHours: 6, DecisionTTL: 300, EventDays: 14, PollSeconds: 900}
}

func testInput(now time.Time) Input {
	p := testPolicy()
	e := Evidence{ID: "fixture-evidence", SourceID: "official", URL: p.Sources[0].URL, FirstSeenAt: now.Add(-time.Hour), Body: "Upgrade scheduled for 2026-10-25T12:00:00Z; token utility remains uncertain."}
	a := Analysis{Action: "PROPOSE_ENTRY", Symbol: "ETHUSDT", EventKey: "fixture-upgrade", EventType: "upgrade", EventAt: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC), EventDateText: "2026-10-25T12:00:00Z", Thesis: "synthetic test catalyst", Counterargument: "already anticipated", Invalidation: "cancellation", HoldRationale: "review actual outcome", Reason: "fixture only", Citations: []Citation{{EvidenceID: e.ID, Quote: e.Body}}}
	r := Review{ID: "fixture-review", Valid: true, At: now, ExpiresAt: now.Add(5 * time.Minute), ReviewBefore: now.Add(6 * time.Hour), PolicyHash: Hash(p), Analysis: a}
	i := domain.Instrument{Exchange: "binance", Market: domain.Spot, Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", Tradable: true, TickSize: decimal.RequireFromString("0.01"), StepSize: decimal.RequireFromString("0.001"), MinQty: decimal.RequireFromString("0.001"), MinNotional: decimal.NewFromInt(5)}
	q := Quote{Symbol: a.Symbol, Bid: decimal.RequireFromString("99.99"), Ask: decimal.NewFromInt(100), BidQty: decimal.NewFromInt(1000), AskQty: decimal.NewFromInt(1000), QuoteVolume: decimal.NewFromInt(20000000), ATR: decimal.NewFromInt(1), EntryAnchor: decimal.NewFromInt(100), At: now, ATRAt: now, Instrument: i}
	return Input{Policy: p, Review: r, Evidence: []Evidence{e}, Event: VerifiedEvent{Key: a.EventKey, Symbol: a.Symbol, Revision: Hash(a), Verified: true, ValidUntil: now.Add(6 * time.Hour)}, Quote: q}
}

var fixtureNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func TestStrictJSONAndPolicy(t *testing.T) {
	for _, raw := range []string{`{"version":"a","version":"b"}`, `{"alien":true}`, `{} {}`, `{"max_calls_per_run":NaN}`} {
		var p Policy
		require.Error(t, Decode(strings.NewReader(raw), &p, 1024), raw)
	}
	var v any
	require.Error(t, Decode(strings.NewReader(`{"x":"too long"}`), &v, 4))
	require.NoError(t, testPolicy().Validate())
	p := testPolicy()
	p.Sources[0].URL = "http://127.0.0.1/secret"
	require.Error(t, p.Validate())
	p = testPolicy()
	p.Sources[0].URL = "https://example.org/?token=secret"
	require.Error(t, p.Validate())
}

func TestAnalysisRejectsFabricationAndMissingPrimary(t *testing.T) {
	base := testInput(fixtureNow)
	require.NoError(t, base.Policy.ValidateAnalysis(base.Review.Analysis, base.Evidence, fixtureNow))
	for name, mutate := range map[string]func(*Input){
		"fabricated quote":   func(i *Input) { i.Review.Analysis.Citations[0].Quote = "invented" },
		"wrong symbol":       func(i *Input) { i.Review.Analysis.Symbol = "BTCUSDT" },
		"no primary":         func(i *Input) { i.Policy.Sources[0].Primary = false },
		"future observation": func(i *Input) { i.Evidence[0].FirstSeenAt = fixtureNow.Add(time.Hour) },
		"unsupported date":   func(i *Input) { i.Review.Analysis.EventDateText = "made-up date" },
		"empty thesis":       func(i *Input) { i.Review.Analysis.Invalidation = "" },
	} {
		t.Run(name, func(t *testing.T) {
			i := testInput(fixtureNow)
			mutate(&i)
			require.Error(t, i.Policy.ValidateAnalysis(i.Review.Analysis, i.Evidence, fixtureNow))
		})
	}
}

func TestArchiveConcurrentAndTampering(t *testing.T) {
	a := Archive{Dir: t.TempDir()}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { _, err := a.Put("records", map[string]string{"a": "b"}); require.NoError(t, err) })
	}
	wg.Wait()
	ids, err := a.IDs("records")
	require.NoError(t, err)
	require.Len(t, ids, 1)
	var out map[string]string
	require.NoError(t, a.Read("records", ids[0], &out))
	require.NoError(t, os.WriteFile(filepath.Join(a.Dir, "records", ids[0]+".json"), []byte(`{"a":"tampered"}`), 0600))
	require.Error(t, a.Read("records", ids[0], &out))
	require.Error(t, a.Read("../escape", ids[0], &out))
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, media, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": []string{media}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestCollectorVersionsAndLatestFailure(t *testing.T) {
	p := testPolicy()
	a := Archive{Dir: t.TempDir()}
	c, err := NewCollector(p, a)
	require.NoError(t, err)
	c.minInterval = 0 // Exercise revisions without advancing wall-clock time.
	body := "first"
	status := 200
	c.client = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { return response(status, "text/plain", body), nil })}
	e1, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	e2, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.Equal(t, e1.FirstSeenAt, e2.FirstSeenAt)
	body = "second"
	e3, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.NotEqual(t, e1.ID, e3.ID)
	latest, err := a.LatestEvidence(p)
	require.NoError(t, err)
	require.Equal(t, e3.ID, latest[0].ID)
	body = "first"
	_, err = c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	latest, err = a.LatestEvidence(p)
	require.NoError(t, err)
	require.Equal(t, e1.ID, latest[0].ID)
	status = 429
	_, err = c.Collect(t.Context(), p.Sources[0])
	require.Error(t, err)
	_, err = a.LatestEvidence(p)
	require.Error(t, err, "latest failure cannot reuse previous successful snapshot")
}

func TestCollectorRefusesNonPublicIPs(t *testing.T) {
	for _, addr := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "::1", "fc00::1", "::ffff:127.0.0.1", "192.0.2.1", "64:ff9b::a00:1"} {
		require.False(t, publicIP(net.ParseIP(addr)), addr)
	}
	require.True(t, publicIP(net.ParseIP("1.1.1.1")))
}

func TestAnalystNoToolsAndDurableRequestDedup(t *testing.T) {
	input := testInput(time.Now().UTC())
	input.Review.Analysis.Action = "WATCH"
	analyst, err := NewAnalyst("https://router.example/v1", "fixture/model", "test-not-a-real-key")
	require.NoError(t, err)
	var calls atomic.Int32
	analyst.client = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		var body map[string]any
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		require.NotContains(t, body, "tools")
		require.Equal(t, false, body["stream"])
		format := body["response_format"].(map[string]any)
		require.Equal(t, "json_schema", format["type"])
		schema := format["json_schema"].(map[string]any)
		require.Equal(t, true, schema["strict"])
		definition := schema["schema"].(map[string]any)
		require.Equal(t, false, definition["additionalProperties"])
		require.Contains(t, definition["required"], "hold_rationale")
		raw, _ := json.Marshal(input.Review.Analysis)
		envelope, _ := json.Marshal(map[string]any{"model": "served/model", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": string(raw)}}}, "usage": map[string]int{"prompt_tokens": 20, "completion_tokens": 10}})
		return response(200, "application/json", string(envelope)), nil
	})}
	a := Archive{Dir: t.TempDir()}
	r, err := analyst.Analyze(t.Context(), a, input.Policy, input.Evidence, time.Now().UTC())
	require.NoError(t, err)
	require.True(t, r.Valid)
	require.Equal(t, "served/model", r.ServedModel)
	_, err = analyst.Analyze(t.Context(), a, input.Policy, input.Evidence, time.Now().UTC())
	require.Error(t, err)
	require.Equal(t, int32(1), calls.Load())
	ids, err := a.IDs("reviews")
	require.NoError(t, err)
	var stored Review
	require.NoError(t, a.Read("reviews", ids[0], &stored))
	data, _ := json.Marshal(stored)
	require.NotContains(t, string(data), "test-not-a-real-key")
}

func TestEntryGateRejectsUnsafeBoundaries(t *testing.T) {
	budget := Budget{Equity: decimal.NewFromInt(1000), FreeCash: decimal.NewFromInt(1000)}
	p := DevelopmentEntryPolicy()
	input := testInput(fixtureNow)
	intent, err := p.Entry(input.Policy, input.Review, input.Evidence, input.Event, input.Quote, budget, fixtureNow)
	require.NoError(t, err)
	require.True(t, intent.Quantity.IsPositive())
	require.True(t, intent.Quantity.Mul(input.Quote.Ask).LessThanOrEqual(decimal.NewFromInt(200)))
	for name, mutate := range map[string]func(*Input, *Budget){
		"unverified":       func(i *Input, _ *Budget) { i.Event.Verified = false },
		"revoked":          func(i *Input, _ *Budget) { i.Event.Revoked = true },
		"stale quote":      func(i *Input, _ *Budget) { i.Quote.At = fixtureNow.Add(-6 * time.Second) },
		"future quote":     func(i *Input, _ *Budget) { i.Quote.At = fixtureNow.Add(time.Second) },
		"wide spread":      func(i *Input, _ *Budget) { i.Quote.Bid = decimal.NewFromInt(90) },
		"thin exit":        func(i *Input, _ *Budget) { i.Quote.BidQty = decimal.Zero },
		"halt":             func(_ *Input, b *Budget) { b.Halted = true },
		"full exposure":    func(_ *Input, b *Budget) { b.Exposure = decimal.NewFromInt(400) },
		"wrong venue":      func(i *Input, _ *Budget) { i.Quote.Instrument.Exchange = "other" },
		"expired decision": func(i *Input, _ *Budget) { i.Review.ExpiresAt = fixtureNow },
		"changed policy":   func(i *Input, _ *Budget) { i.Policy.Version = "other" },
		"meme mislabel":    func(i *Input, _ *Budget) { i.Event.Meme = true },
	} {
		t.Run(name, func(t *testing.T) {
			i := testInput(fixtureNow)
			b := budget
			mutate(&i, &b)
			_, err := p.Entry(i.Policy, i.Review, i.Evidence, i.Event, i.Quote, b, fixtureNow)
			require.Error(t, err)
		})
	}
}

func writeInput(t *testing.T, dir string, input Input) {
	t.Helper()
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, input.Quote.Symbol+".json"), raw, 0600))
}

func TestPaperEntryTrailingRestartFullExitAndNoReentry(t *testing.T) {
	ctx := context.Background()
	now := fixtureNow
	input := testInput(now)
	inbox := t.TempDir()
	writeInput(t, inbox, input)
	spec, err := PaperSpec(input.Policy, inbox)
	require.NoError(t, err)
	repo, err := managed.NewMemory(spec)
	require.NoError(t, err)
	clock := func() time.Time { return now }
	rt, err := NewPaper(ctx, repo, input.Policy, inbox, []domain.Instrument{input.Quote.Instrument}, clock)
	require.NoError(t, err)
	require.NoError(t, rt.Start(ctx))
	candle := func(price string) domain.Candle {
		d := decimal.RequireFromString(price)
		return domain.Candle{Exchange: "binance", Market: domain.Spot, Symbol: "ETHUSDT", Timeframe: "1m", OpenTime: now.Truncate(time.Minute), Open: d, High: d, Low: d, Close: d}
	}
	require.NoError(t, rt.Record(ctx, candle("100"), false))
	require.NoError(t, rt.Tick(ctx, now))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(ctx, candle("100"), false))
	state, err := repo.Load(ctx)
	require.NoError(t, err)
	owner := "research/ethusdt"
	require.True(t, state.Holding(owner, "ETHUSDT").Quantity.IsPositive())
	for _, price := range []string{"110", "120"} {
		now = now.Add(time.Second)
		require.NoError(t, rt.Record(ctx, candle(price), false))
	}
	before, err := repo.Load(ctx)
	require.NoError(t, err)
	checkpoint := append([]byte(nil), before.Bindings[owner].Engine...)
	require.NoError(t, rt.End(ctx, "stopped"))
	rt, err = NewPaper(ctx, repo, input.Policy, inbox, []domain.Instrument{input.Quote.Instrument}, clock)
	require.NoError(t, err)
	require.NoError(t, rt.Start(ctx))
	require.NotEmpty(t, checkpoint)
	now = now.Add(time.Second)
	require.NoError(t, rt.Record(ctx, candle("116"), false))
	now = now.Add(time.Minute)
	require.NoError(t, rt.Record(ctx, candle("116"), false))
	require.NoError(t, rt.Tick(ctx, now))
	state, err = repo.Load(ctx)
	require.NoError(t, err)
	require.True(t, state.Holding(owner, "ETHUSDT").Quantity.IsZero())
	require.Len(t, state.Executions, 2)
	now = now.Add(time.Minute)
	require.NoError(t, rt.Tick(ctx, now))
	require.NoError(t, rt.Record(ctx, candle("100"), false))
	state, err = repo.Load(ctx)
	require.NoError(t, err)
	require.Len(t, state.Executions, 2, "same event cannot reopen")
	for _, o := range state.Orders {
		require.Equal(t, "terminal", o.State)
	}
	_, err = NewPaper(ctx, repo, Policy{}, inbox, []domain.Instrument{input.Quote.Instrument}, clock)
	require.Error(t, err)
}

func TestJSONLimitRejectsWholeHugePayload(t *testing.T) {
	var out Analysis
	require.Error(t, Decode(bytes.NewReader([]byte(fmt.Sprintf(`{"reason":%q}`, strings.Repeat("x", 2000)))), &out, 1024))
}

func TestReplayEventRevocationAndThesisExpiry(t *testing.T) {
	for _, reason := range []string{"revoked", "expired"} {
		t.Run(reason, func(t *testing.T) {
			i := testInput(fixtureNow)
			f := Replay{Policy: i.Policy, Instruments: []domain.Instrument{i.Quote.Instrument}, Steps: []ReplayStep{
				{At: fixtureNow, Symbol: "ETHUSDT", Price: decimal.NewFromInt(100), Input: &i},
				{At: fixtureNow.Add(time.Minute), Symbol: "ETHUSDT", Price: decimal.NewFromInt(100)},
			}}
			changed := testInput(fixtureNow)
			at := fixtureNow.Add(2 * time.Minute)
			if reason == "revoked" {
				changed.Event.Revoked = true
			} else {
				at = fixtureNow.Add(6 * time.Hour)
			}
			f.Steps = append(f.Steps, ReplayStep{At: at, Symbol: "ETHUSDT", Price: decimal.NewFromInt(100), Input: &changed}, ReplayStep{At: at.Add(time.Minute), Symbol: "ETHUSDT", Price: decimal.NewFromInt(100)})
			result, err := f.Run(t.Context())
			require.NoError(t, err)
			require.True(t, result.FixtureOnly)
			require.Len(t, result.Executions, 2)
			require.True(t, result.Inventory.IsZero())
			require.True(t, result.Reserved.IsZero())
		})
	}
}

func TestPaperRejectsP2WalletWithoutMutation(t *testing.T) {
	i := testInput(fixtureNow)
	inbox := t.TempDir()
	spec, err := PaperSpec(i.Policy, inbox)
	require.NoError(t, err)
	spec.ID = "research-stage-two"
	repo, err := managed.NewMemory(spec)
	require.NoError(t, err)
	before, err := repo.Load(t.Context())
	require.NoError(t, err)
	_, err = NewPaper(t.Context(), repo, i.Policy, inbox, []domain.Instrument{i.Quote.Instrument}, func() time.Time { return fixtureNow })
	require.Error(t, err)
	after, err := repo.Load(t.Context())
	require.NoError(t, err)
	require.Equal(t, managed.Hash(before), managed.Hash(after))
}

func TestMemeCapIncludesFeesAndCannotExhaustSleeve(t *testing.T) {
	i := testInput(fixtureNow)
	i.Policy.MemeSymbols = []string{"ETHUSDT"}
	i.Review.PolicyHash = Hash(i.Policy)
	i.Event.Meme = true
	b := Budget{Equity: decimal.NewFromInt(1000), FreeCash: decimal.NewFromInt(1000)}
	intent, err := DevelopmentEntryPolicy().Entry(i.Policy, i.Review, i.Evidence, i.Event, i.Quote, b, fixtureNow)
	require.NoError(t, err)
	require.True(t, intent.Quantity.Mul(i.Quote.Ask).Mul(decimal.RequireFromString("1.0012")).LessThanOrEqual(decimal.NewFromInt(20)))
	b.MemeExposure = decimal.NewFromInt(40)
	_, err = DevelopmentEntryPolicy().Entry(i.Policy, i.Review, i.Evidence, i.Event, i.Quote, b, fixtureNow)
	require.Error(t, err)
}

func TestProviderFailuresAreRecordedWithoutEchoingBody(t *testing.T) {
	i := testInput(time.Now().UTC())
	a, err := NewAnalyst("https://router.example/v1", "fixture/model", "private-test-key")
	require.NoError(t, err)
	a.client = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return response(401, "application/json", `{"error":"private-test-key"}`), nil
	})}
	archive := Archive{Dir: t.TempDir()}
	review, err := a.Analyze(t.Context(), archive, i.Policy, i.Evidence, time.Now().UTC())
	require.Error(t, err)
	require.False(t, review.Valid)
	require.NotContains(t, review.Error, "private-test-key")
	require.Empty(t, review.Raw)
}

func TestRobotsRulesAndCollectorPollingBudget(t *testing.T) {
	for _, row := range []struct {
		body, path string
		allowed    bool
	}{
		{"User-agent: *\nDisallow: /private", "/private/data", false},
		{"User-agent: *\nDisallow: /\nAllow: /news", "/news/item", true},
		{"User-agent: *\nDisallow: /news/*/hidden$", "/news/a/hidden", false},
		{"User-agent: other\nDisallow: /", "/news", true},
		{"User-agent: *\nDisallow: /\nUser-agent: trading-bot-catalyst-research\nAllow: /news", "/news", true},
	} {
		require.Equal(t, row.allowed, robotsAllow(row.body, row.path), row.body)
	}
	p := testPolicy()
	c, err := NewCollector(p, Archive{Dir: t.TempDir()})
	require.NoError(t, err)
	var calls int
	c.client = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Path == "/robots.txt" {
			return response(404, "text/plain", ""), nil
		}
		return response(200, "text/plain", "evidence"), nil
	})}
	first, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	cached, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, first.FirstSeenAt, cached.FirstSeenAt)
}

func TestIncompleteAndToolCallingModelResponsesFailClosed(t *testing.T) {
	for _, finish := range []string{"length", "tool_calls"} {
		t.Run(finish, func(t *testing.T) {
			i := testInput(time.Now().UTC())
			a, err := NewAnalyst("https://router.example/v1", "fixture/model", "fixture-key")
			require.NoError(t, err)
			a.client = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
				raw, _ := json.Marshal(map[string]any{"model": "served", "choices": []any{map[string]any{"finish_reason": finish, "message": map[string]any{"content": "{}", "tool_calls": []any{map[string]any{"id": "forbidden"}}}}}})
				return response(200, "application/json", string(raw)), nil
			})}
			review, err := a.Analyze(t.Context(), Archive{Dir: t.TempDir()}, i.Policy, i.Evidence, time.Now().UTC())
			require.Error(t, err)
			require.False(t, review.Valid)
		})
	}
}

func TestHoldAfterEventNeedsNewPrimaryEvidenceAndKeepsProtection(t *testing.T) {
	for _, fresh := range []bool{true, false} {
		t.Run(fmt.Sprint("fresh=", fresh), func(t *testing.T) {
			i := testInput(fixtureNow)
			i.Review.Analysis.EventAt = fixtureNow.Add(time.Hour)
			i.Review.Analysis.EventDateText = i.Review.Analysis.EventAt.Format(time.RFC3339)
			i.Evidence[0].Body = "Upgrade scheduled for " + i.Review.Analysis.EventDateText
			i.Review.Analysis.Citations[0].Quote = i.Evidence[0].Body
			i.Event.Revision = Hash(i.Review.Analysis)
			raw, err := json.Marshal(i)
			require.NoError(t, err)
			var hold Input
			require.NoError(t, json.Unmarshal(raw, &hold))
			at := fixtureNow.Add(2 * time.Hour)
			hold.Review.ID = "fixture-hold"
			hold.Review.At = at
			hold.Review.ExpiresAt = at.Add(5 * time.Minute)
			hold.Review.ReviewBefore = at.Add(6 * time.Hour)
			hold.Review.Analysis.Action = "HOLD"
			hold.Event.Revision = Hash(hold.Review.Analysis)
			hold.Event.ValidUntil = at.Add(6 * time.Hour)
			if fresh {
				hold.Evidence[0].FirstSeenAt = at
				hold.Evidence[0].ID = "updated-fixture-evidence"
				hold.Evidence[0].Body += "; upgrade completed"
				hold.Review.Analysis.Citations[0].EvidenceID = hold.Evidence[0].ID
				hold.Review.Analysis.Citations[0].Quote = hold.Evidence[0].Body
			}
			hold.Event.Revision = Hash(hold.Review.Analysis)
			f := Replay{Policy: i.Policy, Instruments: []domain.Instrument{i.Quote.Instrument}, Steps: []ReplayStep{
				{At: fixtureNow, Symbol: "ETHUSDT", Price: decimal.NewFromInt(100), Input: &i},
				{At: fixtureNow.Add(time.Minute), Symbol: "ETHUSDT", Price: decimal.NewFromInt(100)},
				{At: at, Symbol: "ETHUSDT", Price: decimal.NewFromInt(100), Input: &hold},
				{At: fixtureNow.Add(6*time.Hour + time.Minute), Symbol: "ETHUSDT", Price: decimal.NewFromInt(100)},
				{At: fixtureNow.Add(6*time.Hour + 2*time.Minute), Symbol: "ETHUSDT", Price: decimal.NewFromInt(100)},
			}}
			result, err := f.Run(t.Context())
			require.NoError(t, err)
			require.Equal(t, fresh, result.Inventory.IsPositive())
			if fresh {
				require.Len(t, result.Executions, 1)
			} else {
				require.Len(t, result.Executions, 2)
			}
		})
	}
}

func TestExpiredPendingEntryIsCanceledBeforeFill(t *testing.T) {
	now := fixtureNow.Add(time.Second)
	i := testInput(now)
	i.Policy.DecisionTTL = 1
	i.Review.PolicyHash = Hash(i.Policy)
	i.Review.ExpiresAt = now.Add(time.Second)
	f := Replay{Policy: i.Policy, Instruments: []domain.Instrument{i.Quote.Instrument}, Steps: []ReplayStep{
		{At: now, Symbol: "ETHUSDT", Price: decimal.NewFromInt(100), Input: &i},
		{At: now.Add(2 * time.Second), Symbol: "ETHUSDT", Price: decimal.NewFromInt(100)},
		{At: now.Add(time.Minute), Symbol: "ETHUSDT", Price: decimal.NewFromInt(100)},
	}}
	result, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, result.Executions)
	require.True(t, result.Inventory.IsZero())
	require.True(t, result.Reserved.IsZero())
}
