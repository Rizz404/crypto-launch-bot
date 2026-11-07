package managed

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Owner is the single dashboard account; Hash is a PBKDF2 password hash.
type Owner struct {
	Email string
	Hash  string
}

// Owners stores the dashboard account.
type Owners interface {
	Owner(ctx context.Context) (Owner, bool, error)
	// Register creates the account; false when one already exists.
	Register(ctx context.Context, o Owner) (bool, error)
	SetHash(ctx context.Context, hash string) error
}

// PGOwners keeps the account in the web_owner table.
type PGOwners struct{ Pool *pgxpool.Pool }

func (p PGOwners) Owner(ctx context.Context) (Owner, bool, error) {
	var o Owner
	err := p.Pool.QueryRow(ctx, `SELECT email, password_hash FROM web_owner`).Scan(&o.Email, &o.Hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Owner{}, false, nil
	}
	return o, err == nil, err
}

func (p PGOwners) Register(ctx context.Context, o Owner) (bool, error) {
	tag, err := p.Pool.Exec(ctx, `INSERT INTO web_owner(email, password_hash) VALUES ($1, $2) ON CONFLICT DO NOTHING`, o.Email, o.Hash)
	return err == nil && tag.RowsAffected() == 1, err
}

func (p PGOwners) SetHash(ctx context.Context, hash string) error {
	_, err := p.Pool.Exec(ctx, `UPDATE web_owner SET password_hash = $1, updated_at = now()`, hash)
	return err
}

// MemOwners keeps the account in memory (tests and previews).
type MemOwners struct {
	mu sync.Mutex
	o  *Owner
}

func (m *MemOwners) Owner(context.Context) (Owner, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.o == nil {
		return Owner{}, false, nil
	}
	return *m.o, true, nil
}

func (m *MemOwners) Register(_ context.Context, o Owner) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.o != nil {
		return false, nil
	}
	m.o = &o
	return true, nil
}

func (m *MemOwners) SetHash(_ context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.o == nil {
		return errors.New("no account")
	}
	m.o.Hash = hash
	return nil
}

const minPassword = 8

// passwordIterations follows the OWASP PBKDF2-SHA256 guidance; tests lower it.
var passwordIterations = 600_000

var b64 = base64.RawStdEncoding

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func checkPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	salt, err1 := b64.DecodeString(parts[2])
	want, err2 := b64.DecodeString(parts[3])
	if err != nil || err1 != nil || err2 != nil || iter < 1 || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
