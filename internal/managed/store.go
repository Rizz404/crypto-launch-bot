package managed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository's transaction boundary includes the state and immutable journal.
// Implementations must discard all changes when fn returns an error.
type Repository interface {
	Load(context.Context) (*State, error)
	Update(context.Context, string, string, any, func(*State) error) error
}
type PGStore struct {
	Pool *pgxpool.Pool
	Key  string
}

func Init(ctx context.Context, pool *pgxpool.Pool, spec Spec) (*PGStore, error) {
	state, err := NewState(spec)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	_, err = pool.Exec(ctx, `INSERT INTO managed_wallets(id,exchange,mode,market,quote,state) VALUES($1,$2,$3,'spot',$4,$5) ON CONFLICT(id) DO NOTHING`, spec.Key(), spec.Exchange, spec.Mode, spec.Quote, raw)
	if err != nil {
		return nil, err
	}
	r := &PGStore{Pool: pool, Key: spec.Key()}
	existing, err := r.Load(ctx)
	if err != nil {
		return nil, err
	}
	if existing.ConfigHash != Hash(spec) {
		return nil, errors.New("managed configuration changed: use audited revision or update stored configuration")
	}
	return r, nil
}
func (r *PGStore) Load(ctx context.Context) (*State, error) {
	var raw []byte
	if err := r.Pool.QueryRow(ctx, `SELECT state FROM managed_wallets WHERE id=$1`, r.Key).Scan(&raw); err != nil {
		return nil, err
	}
	var s State
	err := json.Unmarshal(raw, &s)
	return &s, err
}
func (r *PGStore) Update(ctx context.Context, id, op string, payload any, fn func(*State) error) error {
	if id == "" {
		return errors.New("request id required")
	}
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var raw []byte
	var revision int64
	if err = tx.QueryRow(ctx, `SELECT state,revision FROM managed_wallets WHERE id=$1 FOR UPDATE`, r.Key).Scan(&raw, &revision); err != nil {
		return err
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		return err
	}
	hash := Hash(struct {
		Operation string
		Payload   any
	}{op, payload})
	if old, ok := s.Requests[id]; ok {
		if old != hash {
			return errors.New("idempotency key reused with different payload")
		}
		return tx.Commit(ctx)
	}
	if err = fn(&s); err != nil {
		return err
	}
	s.Requests[id] = hash
	raw, err = json.Marshal(s)
	if err != nil {
		return err
	}
	p, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO managed_journal(wallet_id,sequence,operation,request_id,payload) VALUES($1,$2,$3,$4,$5)`, r.Key, revision+1, op, id, p); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE managed_wallets SET state=$2,revision=revision+1,updated_at=now() WHERE id=$1`, r.Key, raw); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MemoryStore provides the same copy-on-commit contract for offline backtests.
type MemoryStore struct {
	mu    sync.Mutex
	state *State
}

func NewMemory(spec Spec) (*MemoryStore, error) {
	s, err := NewState(spec)
	return &MemoryStore{state: s}, err
}
func clone(s *State) *State {
	raw, _ := json.Marshal(s)
	var out State
	_ = json.Unmarshal(raw, &out)
	return &out
}
func (r *MemoryStore) Load(context.Context) (*State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		return nil, errors.New("uninitialized state")
	}
	return clone(r.state), nil
}
func (r *MemoryStore) Update(_ context.Context, id, op string, payload any, fn func(*State) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == "" {
		return errors.New("request id required")
	}
	s := clone(r.state)
	hash := Hash(struct {
		Operation string
		Payload   any
	}{op, payload})
	if old, ok := s.Requests[id]; ok {
		if old != hash {
			return fmt.Errorf("idempotency conflict")
		}
		return nil
	}
	if err := fn(s); err != nil {
		return err
	}
	s.Requests[id] = hash
	r.state = s
	return nil
}
