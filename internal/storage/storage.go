// Package storage owns the PostgreSQL/TimescaleDB connection and migrations.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/rizz404/trading-bot/migrations"
)

// ConnectWait is how long Open keeps retrying a database that is not
// accepting connections yet (it starts alongside the bot after a reboot).
var ConnectWait = time.Minute

// Open creates a connection pool and verifies it with a ping, waiting up to
// ConnectWait for a database that refuses connections. Other errors, such
// as a wrong password, fail at once.
func Open(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// pgx errors may echo the URL, which can contain the password.
		return nil, fmt.Errorf("parse database url: invalid format")
	}
	cfg.MaxConns = maxConns
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	deadline := time.Now().Add(ConnectWait)
	for delay := 250 * time.Millisecond; ; delay = min(delay*2, 5*time.Second) {
		err := pool.Ping(ctx)
		if err == nil {
			return pool, nil
		}
		var connErr *pgconn.ConnectError
		var pgErr *pgconn.PgError
		retry := errors.As(err, &connErr) && !errors.As(err, &pgErr)
		if !retry || time.Now().Add(delay).After(deadline) {
			pool.Close()
			return nil, fmt.Errorf("ping database: %w", err)
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, fmt.Errorf("ping database: %w", ctx.Err())
		case <-time.After(delay):
		}
	}
}

// Migrator applies the embedded goose migrations.
type Migrator struct {
	provider *goose.Provider
	closer   io.Closer
}

// NewMigrator wraps pool in a database/sql handle for goose.
func NewMigrator(pool *pgxpool.Pool) (*Migrator, error) {
	db := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("goose provider: %w", err)
	}
	return &Migrator{provider: p, closer: db}, nil
}

func (m *Migrator) Close() error { return m.closer.Close() }

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context, log *slog.Logger) error {
	res, err := m.provider.Up(ctx)
	for _, r := range res {
		log.Info("migration applied", "version", r.Source.Version, "file", r.Source.Path, "took", r.Duration)
	}
	return err
}

// Down rolls back the most recent migration.
func (m *Migrator) Down(ctx context.Context, log *slog.Logger) error {
	r, err := m.provider.Down(ctx)
	if r != nil {
		log.Info("migration rolled back", "version", r.Source.Version, "file", r.Source.Path)
	}
	return err
}

// Status logs the state of every known migration.
func (m *Migrator) Status(ctx context.Context, log *slog.Logger) error {
	st, err := m.provider.Status(ctx)
	if err != nil {
		return err
	}
	for _, s := range st {
		log.Info("migration", "version", s.Source.Version, "file", s.Source.Path, "state", s.State, "applied_at", s.AppliedAt)
	}
	return nil
}

// Pending reports how many migrations have not been applied yet.
func (m *Migrator) Pending(ctx context.Context) (int, error) {
	st, err := m.provider.Status(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range st {
		if s.State == goose.StatePending {
			n++
		}
	}
	return n, nil
}
