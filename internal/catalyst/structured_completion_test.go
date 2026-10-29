package catalyst

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStructuredCompletionUsesCallerSchemaWithoutTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model          string              `json:"model"`
			Messages       []map[string]string `json:"messages"`
			MaxTokens      int                 `json:"max_tokens"`
			ResponseFormat map[string]any      `json:"response_format"`
			Tools          json.RawMessage     `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if request.MaxTokens != 2048 || request.Messages[0]["content"] != "custom launch prompt" || request.ResponseFormat["type"] != "json_schema" || len(request.Tools) != 0 {
			t.Error("caller schema/prompt/token cap not preserved")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served","choices":[{"finish_reason":"stop","message":{"content":"{}"}}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`))
	}))
	defer server.Close()
	a, err := NewAnalyst(server.URL+"/v1", "mock", "synthetic-key")
	if err != nil {
		t.Fatal(err)
	}
	raw, model, _, _, err := a.StructuredCompletion(t.Context(), 2048, "custom launch prompt", map[string]any{"type": "json_schema"}, []byte(`{}`))
	if err != nil || raw != "{}" || model != "served" {
		t.Fatalf("invalid response %q %q %v", raw, model, err)
	}
}
