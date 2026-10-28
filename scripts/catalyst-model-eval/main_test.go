package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/stretchr/testify/require"
)

func TestSyntheticCasesAndUnsafeEntryGate(t *testing.T) {
	now := time.Now().UTC()
	rows := cases(now)
	require.Len(t, rows, 7)
	for _, c := range rows {
		require.NoError(t, c.Policy.Validate())
		for _, e := range c.Evidence {
			require.True(t, e.FirstSeenAt.Before(now))
		}
	}
	for _, action := range []string{"WATCH", "PROPOSE_ENTRY"} {
		t.Run(action, func(t *testing.T) {
			c := rows[1]
			a := catalyst.Analysis{Action: action, Symbol: "ALPHAUSDT", Reason: "unconfirmed rumor", EventKey: "rumor", EventType: "listing", EventAt: now.Add(time.Hour), EventDateText: "tomorrow", Thesis: "rumor", Counterargument: "unconfirmed", Invalidation: "no official source", HoldRationale: "none", Citations: []catalyst.Citation{{EvidenceID: c.Evidence[0].ID, Quote: c.Evidence[0].Body}}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/v1/chat/completions", r.URL.Path)
				raw, err := json.Marshal(a)
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"model": "fixture-only", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(raw)}}}}))
			}))
			defer server.Close()
			analyst, err := catalyst.NewAnalyst(server.URL+"/v1", "fixture-model", "fixture-key")
			require.NoError(t, err)
			result := evaluate(t.Context(), analyst, catalyst.Archive{Dir: t.TempDir()}, c, now)
			require.Equal(t, action == "WATCH", result.Passed)
		})
	}
}
