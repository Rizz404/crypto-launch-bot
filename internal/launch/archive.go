package launch

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
)

func history(archive catalyst.Archive) ([]Batch, error) {
	ids, err := archive.IDs("launch-batches")
	if err != nil {
		return nil, err
	}
	var batches []Batch
	for _, id := range ids {
		var b Batch
		if err = archive.Read("launch-batches", id, &b); err != nil {
			return nil, err
		}
		if b.Version != Version || !slices.Contains([]string{"presale", "dex", "cex"}, b.Kind) || b.At.IsZero() {
			return nil, errors.New("invalid launch batch version or kind")
		}
		batches = append(batches, b)
	}
	slices.SortFunc(batches, func(a, b Batch) int { return a.At.Compare(b.At) })
	return batches, nil
}

// Collect performs one bounded read-only pass. Archive writes only store
// research records; no database, Telegram, AI or trading credentials are used.
func Collect(ctx context.Context, archive catalyst.Archive, kind, spider string, limit int) (Report, error) {
	if !slices.Contains([]string{"all", "presale", "dex", "cex"}, kind) || limit < 1 || limit > 10 {
		return Report{}, errors.New("source must be all/presale/dex/cex; limit must be 1–10")
	}
	batches, err := history(archive)
	if err != nil {
		return Report{}, err
	}
	scanned, latest := map[string]time.Time{}, map[string]time.Time{}
	var nextPool *uint64
	for _, b := range batches {
		latest[b.Kind] = b.At
		for _, symbol := range b.Scanned {
			scanned[symbol] = b.At
		}
		if b.Kind == "dex" && b.NextPoolIndex != nil {
			nextPool = b.NextPoolIndex
		}
	}
	client := NewClient(archive)
	for _, source := range []string{"presale", "dex", "cex"} {
		if kind != "all" && kind != source {
			continue
		}
		if !latest[source].IsZero() && time.Since(latest[source]) < 5*time.Minute {
			continue
		}
		var batch Batch
		switch source {
		case "presale":
			batch = client.discoverPresales(ctx, spider)
		case "dex":
			batch = client.discoverDEX(ctx, limit, nextPool)
		case "cex":
			batch = client.discoverCEX(ctx, limit, scanned)
		}
		if _, err = archive.Put("launch-batches", batch); err != nil {
			return Report{}, err
		}
	}
	return ReadReport(archive, time.Now().UTC())
}

func ReadReport(archive catalyst.Archive, now time.Time) (Report, error) {
	batches, err := history(archive)
	if err != nil {
		return Report{}, err
	}
	evidenceAt := map[string]time.Time{}
	check := func(id string) error {
		var raw rawRecord
		if err := archive.Read("launch-raw", id, &raw); err == nil {
			evidenceAt[id] = raw.At
			return nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		var web catalyst.Evidence
		if err := archive.Read("launch-web", id, &web); err != nil {
			return err
		}
		evidenceAt[id] = web.FirstSeenAt
		return nil
	}
	r, err := summarize(batches, now, check)
	if err != nil {
		return r, err
	}
	for i := range r.Reviews {
		for _, id := range r.Reviews[i].Candidate.EvidenceIDs {
			at := evidenceAt[id]
			if at.IsZero() || at.After(now) || now.Sub(at) > EvidenceTTL {
				if r.Reviews[i].Status != "rejected" {
					r.Reviews[i].Status = "held"
				}
				r.Reviews[i].Reasons = append(r.Reviews[i].Reasons, "referenced_evidence_stale_or_future")
				break
			}
		}
	}
	return r, nil
}

func summarize(batches []Batch, now time.Time, evidence func(string) error) (Report, error) {
	r := Report{Version: Version, At: now, ResearchOnly: true, Sources: map[string]Batch{}}
	for _, kind := range []string{"presale", "dex", "cex"} {
		r.Sources[kind] = Batch{Version: Version, Kind: kind, Gaps: []string{"source_not_collected"}}
	}
	latest, first, conflicts := map[string]Candidate{}, map[string]time.Time{}, map[string]bool{}
	for _, b := range batches {
		if b.At.After(now) {
			return r, errors.New("future batch timestamp")
		}
		for _, id := range b.EvidenceIDs {
			if id != "" {
				if err := evidence(id); err != nil {
					return r, err
				}
			}
		}
		if old, ok := r.Sources[b.Kind]; !ok || b.At.After(old.At) {
			r.Sources[b.Kind] = b
		}
		for _, candidate := range b.Candidates {
			canonical := candidate
			canonical.EvidenceIDs = slices.Clone(candidate.EvidenceIDs)
			canonical.Seal()
			if canonical.ID != candidate.ID || canonical.Revision != candidate.Revision || candidate.Kind != b.Kind {
				return r, errors.New("invalid candidate identity or revision")
			}
			if old, ok := latest[candidate.ID]; ok {
				if old.Asset.Key() != candidate.Asset.Key() && old.Asset.Key() != "" && candidate.Asset.Key() != "" {
					conflicts[candidate.ID] = true
				}
				if candidate.ObservedAt.Equal(old.ObservedAt) && candidate.Revision != old.Revision {
					conflicts[candidate.ID] = true
				}
			}
			if t, ok := first[candidate.ID]; !ok || candidate.ObservedAt.Before(t) {
				first[candidate.ID] = candidate.ObservedAt
			}
			if old, ok := latest[candidate.ID]; !ok || candidate.ObservedAt.After(old.ObservedAt) {
				latest[candidate.ID] = candidate
			}
		}
	}
	checked := map[string]bool{}
	for _, candidate := range latest {
		candidate.FirstSeenAt = first[candidate.ID]
		for _, id := range candidate.EvidenceIDs {
			if !checked[id] {
				if err := evidence(id); err != nil {
					return r, err
				}
				checked[id] = true
			}
		}
		source := r.Sources[candidate.Kind]
		gaps := slices.Clone(source.Gaps)
		if source.At.After(candidate.ObservedAt) && len(source.Candidates) == 0 {
			gaps = append(gaps, "candidate_not_reobserved")
		}
		if conflicts[candidate.ID] {
			gaps = append(gaps, "candidate_identity_or_revision_conflict")
		}
		r.Reviews = append(r.Reviews, Verify(candidate, now, gaps))
	}
	for kind, b := range r.Sources {
		b.Candidates = nil
		r.Sources[kind] = b
	}
	slices.SortFunc(r.Reviews, func(a, b Review) int {
		if n := cmp.Compare(a.Candidate.Kind, b.Candidate.Kind); n != 0 {
			return n
		}
		return strings.Compare(a.Candidate.ID, b.Candidate.ID)
	})
	return r, nil
}
