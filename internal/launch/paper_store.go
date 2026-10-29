package launch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"syscall"

	"github.com/rizz404/trading-bot/internal/catalyst"
)

type paperRecord struct {
	Version    string    `json:"version"`
	RunID      string    `json:"run_id"`
	PolicyHash string    `json:"policy_hash"`
	Sequence   uint64    `json:"sequence"`
	Previous   string    `json:"previous"`
	Step       PaperStep `json:"step"`
}

var paperRunID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// PaperStore replays an immutable journal. No financial exchange I/O occurs.
// Its separate process lock spans read/reduce/append; Archive.Put retains its
// existing quota, checksum, exclusive-write and fsync guarantees.
type PaperStore struct {
	Archive catalyst.Archive
	RunID   string
	Policy  PaperPolicy
}

func (store PaperStore) Load() (PaperState, error) {
	if err := store.Policy.Validate(); err != nil {
		return PaperState{}, err
	}
	if !paperRunID.MatchString(store.RunID) {
		return PaperState{}, errors.New("invalid paper run ID")
	}
	s := newPaperState(store.RunID, store.Policy)
	ids, err := store.Archive.IDs("paper-journal")
	if err != nil {
		return s, err
	}
	type item struct {
		ID     string
		Record paperRecord
	}
	records := make([]item, 0, len(ids))
	for _, id := range ids {
		var r paperRecord
		if err = store.Archive.Read("paper-journal", id, &r); err != nil {
			return s, err
		}
		records = append(records, item{id, r})
	}
	slices.SortFunc(records, func(a, b item) int {
		if a.Record.Sequence < b.Record.Sequence {
			return -1
		}
		if a.Record.Sequence > b.Record.Sequence {
			return 1
		}
		return 0
	})
	for _, item := range records {
		r := item.Record
		if r.Version != PaperJournalVersion || r.RunID != store.RunID || r.PolicyHash != s.PolicyHash || r.Sequence != s.Sequence+1 || r.Previous != s.LastHash {
			return s, errors.New("paper journal has a gap/fork, foreign run or policy mismatch")
		}
		if _, seen := s.Requests[r.Step.ID]; seen {
			return s, errors.New("duplicate request in paper journal")
		}
		if err = s.apply(r.Step, store.Policy); err != nil {
			return s, err
		}
		s.Requests[r.Step.ID] = catalyst.Hash(r.Step)
		s.Sequence = r.Sequence
		s.LastHash = item.ID
	}
	return s, nil
}

func (store PaperStore) Step(ctx context.Context, step PaperStep) (PaperState, error) {
	if err := ctx.Err(); err != nil {
		return PaperState{}, err
	}
	if err := os.MkdirAll(store.Archive.Dir, 0700); err != nil {
		return PaperState{}, err
	}
	f, err := os.OpenFile(filepath.Join(store.Archive.Dir, ".paper-transaction-lock"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return PaperState{}, err
	}
	defer func() { _ = f.Close() }()
	// Nonblocking: another worker retries through the coordinator, not while
	// holding a partially modified state or an unbounded shell wait.
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return PaperState{}, errors.New("paper journal is busy")
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	s, err := store.Load()
	if err != nil {
		return s, err
	}
	requestHash := catalyst.Hash(step)
	if old, seen := s.Requests[step.ID]; seen {
		if old != requestHash {
			return s, errors.New("paper request ID reused with different payload")
		}
		return s, nil
	}
	if err = s.apply(step, store.Policy); err != nil {
		return PaperState{}, err
	}
	r := paperRecord{Version: PaperJournalVersion, RunID: store.RunID, PolicyHash: s.PolicyHash, Sequence: s.Sequence + 1, Previous: s.LastHash, Step: step}
	if err = ctx.Err(); err != nil {
		return PaperState{}, err
	}
	id, err := store.Archive.Put("paper-journal", r)
	if err != nil {
		return PaperState{}, err
	}
	s.Sequence = r.Sequence
	s.LastHash = id
	s.Requests[step.ID] = requestHash
	return s, nil
}

func ReplayPaper(ctx context.Context, archive catalyst.Archive, fixture PaperFixture) (PaperState, error) {
	if fixture.Version != PaperFixtureVersion || !fixture.FixtureOnly || len(fixture.Steps) == 0 || len(fixture.Steps) > 1000 {
		return PaperState{}, errors.New("bounded synthetic paper fixture required")
	}
	store := PaperStore{Archive: archive, RunID: fixture.RunID, Policy: fixture.Policy}
	var state PaperState
	for _, step := range fixture.Steps {
		var err error
		state, err = store.Step(ctx, step)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}
