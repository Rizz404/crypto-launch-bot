package managed

import (
	"strings"
	"time"
)

// DefaultRetention keeps one week of settled detail in the hot state. The
// state is rewritten on every ledger transaction, so its size is bounded by
// moving older detail to the append-only archive; aggregates stay in place.
const DefaultRetention = 7 * 24 * time.Hour

// ArchiveRecord is detail moved out of the hot state. Repositories persist it
// in the same transaction that removes it from the state.
type ArchiveRecord struct {
	Kind    string // order, execution, audit
	Key     string
	At      time.Time
	Payload any
}

type archivedOrder struct {
	ID    string
	Order *Order
	Seen  []string
}

// Compact moves settled orders, executions and audit entries older than
// keep to the archive and drops expired idempotency keys. Anything an
// unsettled order, an OCO leg or the durable inbox still needs is kept.
func (s *State) Compact(now time.Time, keep time.Duration) int {
	cutoff := now.Add(-keep)
	busy := map[string]bool{}
	openGroups := map[string]bool{}
	for id, o := range s.Orders {
		if o.State != "terminal" || o.Deferred != nil {
			busy[id] = true
			if o.Group != "" {
				openGroups[o.Group] = true
			}
		}
	}
	for _, ev := range s.Pending {
		busy[ev.Order.ClientID] = true
		busy[ev.Fill.OrderClientID] = true
	}
	for key := range s.PendingFees {
		busy[strings.SplitN(key, ":", 2)[0]] = true
	}
	moved := 0
	for id, o := range s.Orders {
		if busy[id] || (o.Group != "" && openGroups[o.Group]) {
			continue
		}
		if o.SettledAt.IsZero() {
			// Settled before SettledAt existed: start its retention now.
			o.SettledAt = now
			continue
		}
		if !o.SettledAt.Before(cutoff) {
			continue
		}
		rec := archivedOrder{ID: id, Order: o}
		for key := range s.Seen {
			if strings.HasPrefix(key, id+":") {
				rec.Seen = append(rec.Seen, key)
				delete(s.Seen, key)
			}
		}
		s.archive = append(s.archive, ArchiveRecord{Kind: "order", Key: id, At: o.SettledAt, Payload: rec})
		delete(s.Orders, id)
		moved++
	}
	executions := s.Executions[:0]
	for _, e := range s.Executions {
		if e.Fill.Time.Before(cutoff) && !busy[e.Fill.OrderClientID] {
			s.archive = append(s.archive, ArchiveRecord{Kind: "execution", Key: e.Fill.OrderClientID + ":" + e.Fill.TradeID, At: e.Fill.Time, Payload: e})
			moved++
			continue
		}
		executions = append(executions, e)
	}
	s.Executions = executions
	history := s.History[:0]
	for _, a := range s.History {
		if a.At.Before(cutoff) {
			s.archive = append(s.archive, ArchiveRecord{Kind: "audit", Key: a.At.UTC().Format(time.RFC3339Nano) + "/" + Hash(a)[:16], At: a.At, Payload: a})
			moved++
			continue
		}
		history = append(history, a)
	}
	s.History = history
	if s.RequestAt == nil {
		s.RequestAt = map[string]time.Time{}
	}
	for id := range s.Requests {
		at, ok := s.RequestAt[id]
		if !ok {
			s.RequestAt[id] = now
			continue
		}
		if at.Before(cutoff) {
			delete(s.Requests, id)
			delete(s.RequestAt, id)
		}
	}
	return moved
}

// takeArchive hands compacted records to the repository exactly once.
func (s *State) takeArchive() []ArchiveRecord {
	out := s.archive
	s.archive = nil
	return out
}
