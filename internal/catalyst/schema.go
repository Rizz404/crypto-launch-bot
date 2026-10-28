package catalyst

// Request constrained output upstream; local validation remains authoritative.
var analysisResponseFormat = map[string]any{
	"type": "json_schema",
	"json_schema": map[string]any{
		"name": "catalyst_analysis", "strict": true,
		"schema": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"action", "symbol", "event_key", "event_type", "event_at", "event_date_text", "thesis", "counterargument", "invalidation", "hold_rationale", "reason", "citations"},
			"properties": map[string]any{
				"action":          map[string]any{"type": "string", "enum": []string{"REJECT", "WATCH", "PROPOSE_ENTRY", "HOLD", "PROPOSE_EXIT"}},
				"symbol":          map[string]string{"type": "string"},
				"event_key":       map[string]string{"type": "string"},
				"event_type":      map[string]string{"type": "string"},
				"event_at":        map[string]string{"type": "string"},
				"event_date_text": map[string]string{"type": "string"},
				"thesis":          map[string]string{"type": "string"},
				"counterargument": map[string]string{"type": "string"},
				"invalidation":    map[string]string{"type": "string"},
				"hold_rationale":  map[string]string{"type": "string"},
				"reason":          map[string]string{"type": "string"},
				"citations": map[string]any{
					"type": "array", "minItems": 1, "maxItems": 8,
					"items": map[string]any{
						"type": "object", "additionalProperties": false,
						"required":   []string{"evidence_id", "quote"},
						"properties": map[string]any{"evidence_id": map[string]string{"type": "string"}, "quote": map[string]string{"type": "string"}},
					},
				},
			},
		},
	},
}
