package managed

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"
)

// Lease uses a dedicated PostgreSQL session. Losing that connection invalidates
// ownership: it must never be transparently replaced by another pool connection.
type Lease struct {
	mu   sync.Mutex
	conn *pgxpool.Conn
	key  string
}

func Acquire(ctx context.Context, pool *pgxpool.Pool, key string) (*Lease, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var ok bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, key).Scan(&ok); err != nil || !ok {
		conn.Release()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("another process owns this wallet")
	}
	return &Lease{conn: conn, key: key}, nil
}
func (l *Lease) Check(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil || l.conn.Conn().IsClosed() {
		return errors.New("wallet ownership connection lost")
	}
	var ok bool
	err := l.conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory')`).Scan(&ok)
	if err != nil || !ok {
		return errors.New("wallet ownership lost")
	}
	return nil
}
func (l *Lease) Close(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn != nil {
		_, _ = l.conn.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, l.key)
		l.conn.Release()
		l.conn = nil
	}
}
