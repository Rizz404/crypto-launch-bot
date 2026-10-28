package catalyst

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func scheduledEvidence(t *testing.T, body string) Evidence {
	t.Helper()
	source := testPolicy().Sources[0]
	docs, text, err := extractDocuments(source, "text/html", "<main>"+body+"</main>", fixtureNow)
	require.NoError(t, err)
	return Evidence{ID: "primary-fixture", SourceID: source.ID, URL: source.URL, FirstSeenAt: fixtureNow, Body: text, Documents: docs}
}

func TestEventAttestationIndependentFromModel(t *testing.T) {
	p := testPolicy()
	e := scheduledEvidence(t, "The upgrade is scheduled for 2026-10-24T12:00:00Z. Read the release notes.")
	events := DiscoverEvents(p, []Evidence{e})
	require.Len(t, events, 1)
	c := events[0]
	a := Analysis{Action: "PROPOSE_ENTRY", Symbol: c.Symbol, EventKey: c.Key, EventType: c.Type, EventAt: c.At, EventDateText: c.DateText, Thesis: "fixture thesis", Counterargument: "priced in", Invalidation: "withdrawal", HoldRationale: "review release", Reason: "scheduled", Citations: []Citation{{EvidenceID: e.ID, Quote: c.Quote}}}
	archive := Archive{Dir: t.TempDir()}
	v, err := archive.VerifyEvent(p, a, []Evidence{e}, fixtureNow)
	require.NoError(t, err)
	require.True(t, v.Verified)
	require.Equal(t, Hash(a), v.Revision)
	for name, change := range map[string]func(*Analysis){
		"invented key":         func(a *Analysis) { a.EventKey = "model-invented" },
		"different asset":      func(a *Analysis) { a.Symbol = "SOLUSDT" },
		"invented time":        func(a *Analysis) { a.EventAt = a.EventAt.Add(time.Hour) },
		"different event type": func(a *Analysis) { a.EventType = "listing" },
		"not an entry":         func(a *Analysis) { a.Action = "WATCH" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := a
			change(&bad)
			_, err := archive.VerifyEvent(p, bad, []Evidence{e}, fixtureNow)
			require.Error(t, err)
		})
	}
	corrected := scheduledEvidence(t, "The upgrade is scheduled for 2026-10-25T12:00:00Z.")
	changed := DiscoverEvents(p, []Evidence{corrected})
	require.Len(t, changed, 1)
	require.Equal(t, c.Key, changed[0].Key, "schedule correction must not invent a new event identity")
}

func TestEventDiscoveryRefusesAmbiguityAndUnscopedSources(t *testing.T) {
	for _, body := range []string{
		"The upgrade is scheduled for 2026-10-24.",
		"The upgrade is not scheduled for 2026-10-24T12:00:00Z.",
		"The upgrade is tentatively scheduled for 2026-10-24T12:00:00Z.",
		"The upgrade scheduled for 2026-10-24T12:00:00Z is canceled.",
		"Published 2026-10-24T12:00:00Z. An upgrade might be scheduled.",
		"The upgrade is scheduled for 2026-10-24T12:00:00Z or 2026-10-25T12:00:00Z.",
		"The upgrade is scheduled for 2026-02-30T12:00:00Z.",
		"The upgrade will not activate as scheduled on 2026-10-24T12:00:00Z.",
		// Real Ethereum Foundation wording for a testnet schedule.
		"The Glamsterdam network upgrade is scheduled to activate on Hoodi at epoch 132,352, slot 4,235,264 (October 26, 2026, 17:42:48 UTC).",
		"The Glamsterdam upgrade on Hoodi, previously scheduled for October 26, 2026 at 17:42:48 UTC (epoch 132,352), has been postponed.",
	} {
		require.Empty(t, DiscoverEvents(testPolicy(), []Evidence{scheduledEvidence(t, body)}), body)
	}
	for _, body := range []string{
		"The network upgrade is scheduled to activate on mainnet at epoch 140,000 (October 26, 2026, 17:42:48 UTC).",
		"The upgrade is scheduled for October 26, 2026 at 17:42:48 UTC.",
		"The upgrade is scheduled for 2026-10-26 17:42:48 UTC.",
	} {
		events := DiscoverEvents(testPolicy(), []Evidence{scheduledEvidence(t, body)})
		require.Len(t, events, 1, body)
		require.Equal(t, time.Date(2026, 10, 26, 17, 42, 48, 0, time.UTC), events[0].At, body)
	}
	p := testPolicy()
	e := scheduledEvidence(t, "The upgrade is scheduled for October 24, 2026 at 12:00 UTC.")
	require.Len(t, DiscoverEvents(p, []Evidence{e}), 1)
	p.Sources[0].Primary = false
	require.Empty(t, DiscoverEvents(p, []Evidence{e}))
	p.Sources[0].Primary = true
	p.Sources[0].Symbols = []string{"ETHUSDT", "SOLUSDT"}
	require.Empty(t, DiscoverEvents(p, []Evidence{e}), "an aggregator cannot prove asset identity")
}

func TestDailyInferenceBudgetPersistsAndSerializes(t *testing.T) {
	a := Archive{Dir: t.TempDir()}
	now := time.Date(2026, 10, 28, 16, 59, 0, 0, time.UTC)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 40 {
		wg.Go(func() {
			if a.reserveInference(now) == nil {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	require.Equal(t, int32(12), accepted.Load())
	reopened := Archive{Dir: a.Dir}
	require.ErrorContains(t, reopened.reserveInference(now), "budget exhausted")
	require.NoError(t, reopened.reserveInference(now.Add(time.Minute)), "budget resets at WIB midnight, not UTC midnight")
}

func TestAvailableEvidenceRetainsSourceGaps(t *testing.T) {
	p := testPolicy()
	a := Archive{Dir: t.TempDir()}
	e := scheduledEvidence(t, "The upgrade is scheduled for 2026-10-24T12:00:00Z.")
	_, err := a.Put("evidence", e)
	require.NoError(t, err)
	_, err = a.Put("observations", Observation{SourceID: p.Sources[0].ID, Evidence: e.ID, Status: "ok", At: time.Now().UTC()})
	require.NoError(t, err)
	p.Sources = append(p.Sources, Source{ID: "missing", URL: "https://other.example/", Symbols: []string{"ETHUSDT"}})
	got, gaps, err := a.AvailableEvidence(p)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, []string{"missing"}, gaps)
	_, err = a.LatestEvidence(p)
	require.Error(t, err, "strict legacy callers retain their all-source contract")
	_, err = a.Put("observations", Observation{SourceID: p.Sources[0].ID, Status: "failed", At: time.Now().UTC()})
	require.NoError(t, err)
	got, gaps, err = a.AvailableEvidence(p)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Len(t, gaps, 2)
}

func TestEventDatesWithUnicodeContext(t *testing.T) {
	body := strings.Repeat("日本語", 30) + " The upgrade is scheduled for 2026-10-24T12:00:00Z. " + strings.Repeat("日本語", 30)
	events := DiscoverEvents(testPolicy(), []Evidence{scheduledEvidence(t, body)})
	require.Len(t, events, 1, fmt.Sprint(events))
	require.Contains(t, body, events[0].Quote)
}

func TestEventAliasesAndPostEventReceipt(t *testing.T) {
	p := testPolicy()
	archive := Archive{Dir: t.TempDir()}
	e := scheduledEvidence(t, "The upgrade is scheduled for 2026-10-24T12:00:00Z.")
	original, err := archive.Events(p, []Evidence{e})
	require.NoError(t, err)
	require.Len(t, original, 1)
	copy := e
	copy.ID = "another-evidence"
	copy.Documents = append([]Document(nil), e.Documents...)
	copy.Documents[0].ID = "another-document"
	copy.Documents[0].URL += "/followup"
	copy.FirstSeenAt = original[0].At.Add(time.Hour)
	copy.Documents[0].FirstSeenAt = copy.FirstSeenAt
	aliases, err := archive.Events(p, []Evidence{copy})
	require.NoError(t, err)
	require.Len(t, aliases, 1)
	require.Equal(t, original[0].Key, aliases[0].Key, "same asset/time across articles is the same consumed event")
	require.Equal(t, original[0].FirstSeenAt, aliases[0].FirstSeenAt, "repeating an old schedule is not post-event evidence")
	c := aliases[0]
	a := Analysis{Action: "HOLD", Symbol: c.Symbol, EventKey: c.Key, EventType: c.Type, EventAt: c.At, EventDateText: c.DateText, Thesis: "same", Counterargument: "priced in", Invalidation: "cancel", HoldRationale: "wait", Reason: "fixture", Citations: []Citation{{EvidenceID: copy.ID, Quote: c.Quote}}}
	_, err = archive.VerifyEvent(p, a, []Evidence{copy}, copy.FirstSeenAt)
	require.Error(t, err, "a re-fetched pre-event quote cannot justify post-event hold")
}

func TestArchiveQuotaPreservesExistingEvidence(t *testing.T) {
	a := Archive{Dir: t.TempDir(), MaxBytes: maxArchiveBytes + 1024}
	value := strings.Repeat("a", 5<<20)
	id, err := a.Put("records", value)
	require.NoError(t, err)
	_, err = a.Put("records", "different"+value)
	require.ErrorContains(t, err, "quota reached")
	again, err := a.Put("records", value)
	require.NoError(t, err, "an existing content-addressed record consumes no extra quota")
	require.Equal(t, id, again)
	var restored string
	require.NoError(t, a.Read("records", id, &restored))
	require.Equal(t, value, restored)
}

func TestWatchMayCiteOtherAssetsButEntryNeedsSameAsset(t *testing.T) {
	p := testPolicy()
	p.Symbols = append(p.Symbols, "DOGEUSDT")
	e := scheduledEvidence(t, "The upgrade is scheduled for 2026-10-24T12:00:00Z.")
	watch := Analysis{Action: "WATCH", Symbol: "DOGEUSDT", Reason: "no DOGE catalyst", Citations: []Citation{{EvidenceID: e.ID, Quote: "The upgrade is scheduled for 2026-10-24T12:00:00Z."}}}
	require.NoError(t, p.ValidateAnalysis(watch, []Evidence{e}, fixtureNow))
	entry := watch
	entry.Action, entry.EventKey, entry.EventType, entry.EventAt, entry.EventDateText = "PROPOSE_ENTRY", "eth-upgrade", "upgrade", time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), "2026-10-24T12:00:00Z"
	entry.Thesis, entry.Counterargument, entry.Invalidation, entry.HoldRationale = "t", "c", "i", "h"
	require.ErrorContains(t, p.ValidateAnalysis(entry, []Evidence{e}, fixtureNow), "primary evidence")
	entry.Symbol = "ETHUSDT"
	require.NoError(t, p.ValidateAnalysis(entry, []Evidence{e}, fixtureNow))
}
