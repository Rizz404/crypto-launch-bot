package catalyst

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// EventCandidate comes from a primary document with a configured single-asset
// identity and an explicit UTC schedule, independently of model output.
type EventCandidate struct {
	Key, Symbol, Type, SourceID, EvidenceID, DocumentID, URL, DateText, Quote string
	At, FirstSeenAt                                                           time.Time
}

var eventDate = regexp.MustCompile(`\b(?:\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2})?(?:Z|\+00:00)|\d{4}-\d{2}-\d{2} \d{2}:\d{2}(?::\d{2})? UTC|(?:January|February|March|April|May|June|July|August|September|October|November|December) \d{1,2}, \d{4},? (?:at )?\d{2}:\d{2}(?::\d{2})? UTC)\b`)

func parseEventDate(text string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00", "2006-01-02 15:04 MST", "2006-01-02 15:04:05 MST",
		"January 2, 2006 at 15:04 MST", "January 2, 2006 15:04 MST", "January 2, 2006, 15:04 MST",
		"January 2, 2006 at 15:04:05 MST", "January 2, 2006 15:04:05 MST", "January 2, 2006, 15:04:05 MST"} {
		if t, err := time.Parse(layout, text); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, errors.New("event needs an explicit UTC date and time")
}

// DiscoverEvents deliberately refuses date-only, multi-date, cancellation and
// unscoped documents. Missing verification means WATCH, never an invented date.
func DiscoverEvents(p Policy, evidence []Evidence) []EventCandidate {
	var out []EventCandidate
	for _, e := range evidence {
		for _, source := range p.Sources {
			if !source.Primary || source.ID != e.SourceID || source.URL != e.URL || len(source.Symbols) != 1 {
				continue
			}
			for _, doc := range e.Documents {
				var candidate *EventCandidate
				ambiguous := false
				for _, loc := range eventDate.FindAllStringIndex(doc.Body, 32) {
					date := doc.Body[loc[0]:loc[1]]
					at, err := parseEventDate(date)
					if err != nil {
						continue
					}
					start, end := max(0, loc[0]-160), min(len(doc.Body), loc[1]+160)
					for start < len(doc.Body) && !utf8.RuneStart(doc.Body[start]) {
						start++
					}
					for end < len(doc.Body) && !utf8.RuneStart(doc.Body[end]) {
						end--
					}
					quote := doc.Body[start:end]
					lower := strings.ToLower(quote)
					if slices.ContainsFunc([]string{"cancel", "postpon", "delay", "tba", "published", "updated", "copyright", "rumor", "unconfirmed", "tentative", "not scheduled", "not confirmed", "proposed", "will not", "won't", "no longer"}, func(s string) bool { return strings.Contains(lower, s) }) {
						ambiguous = true
						break
					}
					// Test networks carry no Spot market impact; skip their dates instead of guessing scope.
					if slices.ContainsFunc([]string{"testnet", "devnet", "hoodi", "sepolia", "holesky"}, func(s string) bool { return strings.Contains(lower, s) }) {
						continue
					}
					if !slices.ContainsFunc([]string{"scheduled", "will activate", "activates at", "launches", "will launch", "go live", "activation at", "is set for", "takes place", "listing at"}, func(s string) bool { return strings.Contains(lower, s) }) {
						continue
					}
					kind := ""
					for _, cue := range []struct{ text, kind string }{{"upgrade", "upgrade"}, {"hard fork", "upgrade"}, {"activation", "upgrade"}, {"mainnet", "product"}, {"launch", "product"}, {"listing", "listing"}} {
						if strings.Contains(lower, cue.text) {
							kind = cue.kind
							break
						}
					}
					if kind == "" {
						continue
					}
					if candidate != nil && !candidate.At.Equal(at) {
						ambiguous = true
						break
					}
					candidate = &EventCandidate{Key: Hash([]string{source.ID, doc.ID, source.Symbols[0]})[:32], Symbol: source.Symbols[0], Type: kind, SourceID: source.ID, EvidenceID: e.ID, DocumentID: doc.ID, URL: doc.URL, DateText: date, Quote: quote, At: at, FirstSeenAt: doc.FirstSeenAt}
				}
				if candidate != nil && !ambiguous {
					out = append(out, *candidate)
				}
			}
		}
	}
	return out
}

// Events preserves aliases across primary articles and schedule corrections.
// Conflicting identities are held for review rather than merged after trading.
func (archive Archive) Events(p Policy, evidence []Evidence) ([]EventCandidate, error) {
	ids, err := archive.IDs("events")
	if err != nil {
		return nil, err
	}
	var history []EventCandidate
	for _, id := range ids {
		var old EventCandidate
		if err = archive.Read("events", id, &old); err != nil {
			return nil, err
		}
		history = append(history, old)
	}
	events := DiscoverEvents(p, evidence)
	for n := range events {
		candidate := &events[n]
		canonical := ""
		for _, old := range history {
			if old.Symbol == candidate.Symbol && old.At.Equal(candidate.At) && old.Quote == candidate.Quote && old.FirstSeenAt.Before(candidate.FirstSeenAt) {
				candidate.FirstSeenAt = old.FirstSeenAt
			}
			if old.Symbol == candidate.Symbol && (old.DocumentID == candidate.DocumentID || old.At.Equal(candidate.At)) {
				if canonical != "" && canonical != old.Key {
					return nil, errors.New("conflicting event aliases require review")
				}
				canonical = old.Key
			}
		}
		if canonical != "" {
			candidate.Key = canonical
		}
		if _, err = archive.Put("events", *candidate); err != nil {
			return nil, err
		}
		history = append(history, *candidate)
	}
	return events, nil
}

func (archive Archive) VerifyEvent(p Policy, a Analysis, evidence []Evidence, now time.Time) (VerifiedEvent, error) {
	if a.Action != "PROPOSE_ENTRY" && a.Action != "HOLD" {
		return VerifiedEvent{}, errors.New("only entry or hold proposals can attest an event")
	}
	if err := p.ValidateAnalysis(a, evidence, now); err != nil {
		return VerifiedEvent{}, err
	}
	candidates, err := archive.Events(p, evidence)
	if err != nil {
		return VerifiedEvent{}, err
	}
	for _, candidate := range candidates {
		if a.Action == "HOLD" && !a.EventAt.After(now) && candidate.FirstSeenAt.Before(a.EventAt) {
			continue
		}
		if candidate.Key != a.EventKey || candidate.Symbol != a.Symbol || candidate.Type != a.EventType || !candidate.At.Equal(a.EventAt) || candidate.DateText != a.EventDateText {
			continue
		}
		for _, citation := range a.Citations {
			if citation.EvidenceID == candidate.EvidenceID && strings.Contains(citation.Quote, candidate.DateText) && strings.Contains(candidate.Quote, citation.Quote) {
				return VerifiedEvent{Key: candidate.Key, Symbol: candidate.Symbol, Revision: Hash(a), Verified: true, Meme: slices.Contains(p.MemeSymbols, a.Symbol), ValidUntil: now.Add(time.Duration(p.PollSeconds*2) * time.Second)}, nil
			}
		}
	}
	return VerifiedEvent{}, errors.New("event lacks an unambiguous primary UTC schedule and asset mapping")
}
