package catalyst

import (
	"cmp"
	"slices"
	"time"
)

type ReviewSummary struct {
	ID, Action, Symbol, Reason, Error string
	RequestedModel, ServedModel       string
	At, ExpiresAt                     time.Time
	Valid                             bool
	LatencyMS                         int64
	InputTokens, OutputTokens         int
}

type DocumentSummary struct {
	SourceID, EvidenceID, ID, Revision, URL, Kind, Title string
	PublishedText, UpdatedText                           string
	FirstSeenAt                                          time.Time
}

type Report struct {
	ShadowOnly                bool
	PolicyHash                string
	Sources                   []Observation
	Reviews                   []ReviewSummary
	Documents                 []DocumentSummary
	InputTokens, OutputTokens int
	InvalidReviews            int
	CostUSD                   *float64
}

func (a Archive) Report(p Policy) (Report, error) {
	r := Report{ShadowOnly: true, PolicyHash: Hash(p)}
	ids, err := a.IDs("observations")
	if err != nil {
		return r, err
	}
	latest := map[string]Observation{}
	for _, id := range ids {
		var obs Observation
		if err = a.Read("observations", id, &obs); err != nil {
			return r, err
		}
		if old, ok := latest[obs.SourceID]; !ok || obs.At.After(old.At) {
			latest[obs.SourceID] = obs
		}
	}
	for _, source := range p.Sources {
		obs, ok := latest[source.ID]
		if !ok {
			obs = Observation{SourceID: source.ID, Status: "missing"}
		}
		r.Sources = append(r.Sources, obs)
	}
	ids, err = a.IDs("evidence")
	if err != nil {
		return r, err
	}
	seen := map[string]bool{}
	for _, id := range ids {
		var evidence Evidence
		if err = a.Read("evidence", id, &evidence); err != nil {
			return r, err
		}
		obs := latest[evidence.SourceID]
		if obs.Status != "ok" || obs.Evidence != evidence.ID {
			continue
		}
		for _, source := range p.Sources {
			if source.ID != evidence.SourceID || source.URL != evidence.URL {
				continue
			}
			for _, d := range evidence.Documents {
				if seen[d.ID] {
					continue
				}
				seen[d.ID] = true
				r.Documents = append(r.Documents, DocumentSummary{SourceID: source.ID, EvidenceID: evidence.ID, ID: d.ID, Revision: d.Revision, URL: d.URL, Kind: d.Kind, Title: d.Title, PublishedText: d.PublishedText, UpdatedText: d.UpdatedText, FirstSeenAt: d.FirstSeenAt})
			}
		}
	}
	ids, err = a.IDs("reviews")
	if err != nil {
		return r, err
	}
	for _, id := range ids {
		var review Review
		if err = a.Read("reviews", id, &review); err != nil {
			return r, err
		}
		if review.PolicyHash != r.PolicyHash {
			continue
		}
		r.Reviews = append(r.Reviews, ReviewSummary{ID: review.ID, Action: review.Analysis.Action, Symbol: review.Analysis.Symbol, Reason: review.Analysis.Reason, Error: review.Error, RequestedModel: review.RequestedModel, ServedModel: review.ServedModel, At: review.At, ExpiresAt: review.ExpiresAt, Valid: review.Valid, LatencyMS: review.LatencyMS, InputTokens: review.InputTokens, OutputTokens: review.OutputTokens})
		r.InputTokens += review.InputTokens
		r.OutputTokens += review.OutputTokens
		if !review.Valid {
			r.InvalidReviews++
		}
	}
	slices.SortFunc(r.Reviews, func(a, b ReviewSummary) int { return cmp.Compare(a.At.UnixNano(), b.At.UnixNano()) })
	return r, nil
}
