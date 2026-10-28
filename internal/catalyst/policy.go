// Package catalyst collects auditable evidence and validates AI research.
// It has no exchange trading credentials; execution is limited to Sim orders.
package catalyst

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const MaxDocumentBytes = 1 << 20
const MaxAnalysisBytes = 64 << 10

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
var symbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,20}USDT$`)

type Source struct {
	ID      string   `json:"id"`
	URL     string   `json:"url"`
	Primary bool     `json:"primary"`
	Symbols []string `json:"symbols"`
	// ArticlePattern selects same-host article paths linked from the seed;
	// Spider then scrapes those pages instead of an unordered crawl.
	ArticlePattern string `json:"article_pattern,omitempty"`
}

// Policy is versioned input, not something the analyst may change.
type Policy struct {
	Version     string   `json:"version"`
	Sources     []Source `json:"sources"`
	Symbols     []string `json:"symbols"`
	MemeSymbols []string `json:"meme_symbols"`
	MaxCalls    int      `json:"max_calls_per_run"`
	MaxTokens   int      `json:"max_output_tokens"`
	ReviewHours int      `json:"review_hours"`
	DecisionTTL int      `json:"decision_ttl_seconds"`
	EventDays   int      `json:"event_horizon_days"`
	PollSeconds int      `json:"poll_seconds"`
}

func (p Policy) Validate() error {
	if !identifier.MatchString(p.Version) || len(p.Sources) == 0 || len(p.Sources) > 20 || len(p.Symbols) == 0 || len(p.Symbols) > 20 {
		return errors.New("policy needs a version, 1–20 sources and 1–20 symbols")
	}
	if p.MaxCalls < 1 || p.MaxCalls > 20 || p.MaxTokens < 256 || p.MaxTokens > 8192 || p.ReviewHours < 1 || p.ReviewHours > 24 || p.DecisionTTL < 1 || p.DecisionTTL > 300 || p.EventDays < 1 || p.EventDays > 30 || p.PollSeconds < 60 {
		return errors.New("policy budgets, expiry or polling interval out of bounds")
	}
	seen := map[string]bool{}
	for _, sym := range p.Symbols {
		if !symbolPattern.MatchString(sym) || seen[sym] {
			return errors.New("invalid or duplicate policy symbol")
		}
		seen[sym] = true
	}
	for _, sym := range p.MemeSymbols {
		if !slices.Contains(p.Symbols, sym) {
			return errors.New("meme symbol outside policy")
		}
	}
	ids, urls := map[string]bool{}, map[string]bool{}
	for _, s := range p.Sources {
		u, err := publicURL(s.URL)
		if err != nil || !identifier.MatchString(s.ID) || ids[s.ID] || urls[s.URL] || len(s.Symbols) == 0 {
			return errors.New("invalid or duplicate source")
		}
		if u.Hostname() == "" {
			return errors.New("source hostname missing")
		}
		if _, err = regexp.Compile(s.ArticlePattern); err != nil || len(s.ArticlePattern) > 200 {
			return errors.New("invalid source article pattern")
		}
		ids[s.ID], urls[s.URL] = true, true
		for _, sym := range s.Symbols {
			if !slices.Contains(p.Symbols, sym) {
				return errors.New("source symbol outside policy")
			}
		}
	}
	return nil
}

func Decode(r io.Reader, out any, limit int64) error {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if int64(len(b)) > limit {
		return errors.New("JSON exceeds size limit")
	}
	keys := json.NewDecoder(bytes.NewReader(b))
	keys.UseNumber()
	if err := uniqueKeys(keys, 0); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid JSON or unknown field")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON content")
	}
	return nil
}

func uniqueKeys(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds limit")
	}
	t, err := d.Token()
	if err != nil {
		return errors.New("invalid JSON")
	}
	if t == json.Delim('{') {
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return errors.New("invalid or duplicate JSON key")
			}
			seen[name] = true
			if err := uniqueKeys(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	} else if t == json.Delim('[') {
		for d.More() {
			if err := uniqueKeys(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

func Hash(v any) string {
	b, _ := json.Marshal(v)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func publicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.Fragment != "" || u.RawQuery != "" || (u.Port() != "" && u.Port() != "443") {
		return nil, errors.New("source must be HTTPS without credentials, query or fragment")
	}
	return u, nil
}

type Evidence struct {
	ID                string     `json:"id"`
	SourceID          string     `json:"source_id"`
	URL               string     `json:"url"`
	BodyHash          string     `json:"body_hash"`
	ContentType       string     `json:"content_type"`
	FirstSeenAt       time.Time  `json:"first_seen_at"`
	Body              string     `json:"body"`
	Raw               string     `json:"raw,omitempty"`
	ExtractionVersion string     `json:"extraction_version,omitempty"`
	Documents         []Document `json:"documents,omitempty"`
}

type Citation struct {
	EvidenceID string `json:"evidence_id"`
	Quote      string `json:"quote"`
}

// Analysis remains a proposal, even when all structural checks succeed.
// Primary quotes establish provenance; they do not prove economic causality.
type Analysis struct {
	Action          string     `json:"action"`
	Symbol          string     `json:"symbol"`
	EventKey        string     `json:"event_key"`
	EventType       string     `json:"event_type"`
	EventAt         time.Time  `json:"event_at"`
	EventDateText   string     `json:"event_date_text"`
	Thesis          string     `json:"thesis"`
	Counterargument string     `json:"counterargument"`
	Invalidation    string     `json:"invalidation"`
	HoldRationale   string     `json:"hold_rationale"`
	Reason          string     `json:"reason"`
	Citations       []Citation `json:"citations"`
}

func (p Policy) ValidateAnalysis(a Analysis, evidence []Evidence, now time.Time) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if !slices.Contains([]string{"REJECT", "WATCH", "PROPOSE_ENTRY", "HOLD", "PROPOSE_EXIT"}, a.Action) || !slices.Contains(p.Symbols, a.Symbol) || strings.TrimSpace(a.Reason) == "" {
		return errors.New("invalid action, symbol or reason")
	}
	if len(a.Citations) == 0 || len(a.Citations) > 8 {
		return errors.New("analysis needs 1–8 citations")
	}
	primary := false
	dateFound := false
	for _, c := range a.Citations {
		found := false
		for _, e := range evidence {
			if e.ID != c.EvidenceID {
				continue
			}
			if e.FirstSeenAt.IsZero() || e.FirstSeenAt.After(now) || c.Quote == "" || len(c.Quote) > 2000 || !strings.Contains(e.Body, c.Quote) {
				return errors.New("citation does not match archived evidence")
			}
			for _, s := range p.Sources {
				// WATCH may cite any policy source; only same-asset sources can support entry/hold.
				if s.ID != e.SourceID || s.URL != e.URL {
					continue
				}
				found = true
				if s.Primary && slices.Contains(s.Symbols, a.Symbol) {
					primary = true
					dateFound = dateFound || (a.EventDateText != "" && strings.Contains(c.Quote, a.EventDateText))
				}
			}
		}
		if !found {
			return errors.New("citation source outside policy")
		}
	}
	if a.Action == "PROPOSE_ENTRY" || a.Action == "HOLD" {
		if !primary || !identifier.MatchString(a.EventKey) || !slices.Contains([]string{"upgrade", "product", "tokenomics", "listing", "meme_catalyst"}, a.EventType) || a.Thesis == "" || a.Counterargument == "" || a.Invalidation == "" || a.HoldRationale == "" {
			return errors.New("entry/hold needs primary evidence and a complete thesis")
		}
		if a.EventAt.IsZero() || a.EventAt.After(now.AddDate(0, 0, p.EventDays)) || !dateFound {
			return errors.New("event date not supported by primary quote or outside horizon")
		}
		if a.Action == "PROPOSE_ENTRY" && !a.EventAt.After(now) {
			return errors.New("new pre-event entry requires a future event")
		}
	}
	return nil
}
