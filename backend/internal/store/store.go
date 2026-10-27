// Package store persists engines, position sets, tournaments and games in PostgreSQL.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound means the requested row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means the row already exists or is in a state that forbids the change.
	ErrConflict = errors.New("conflict")
	// ErrInvalidReference means a referenced engine or position set does not exist.
	ErrInvalidReference = errors.New("referenced engine or position set does not exist")
	// ErrLeaseLost means another worker took over the game, or it was cancelled.
	ErrLeaseLost = errors.New("game lease lost")
	// ErrConflictingMove means a different move is already stored at this ply.
	ErrConflictingMove = errors.New("a different move already exists at this ply")
)

// Store is the PostgreSQL repository.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a store backed by pool. Migrations must already be applied.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func isUniqueViolation(err error) bool {
	return pgErrorCode(err) == "23505"
}

func isForeignKeyViolation(err error) bool {
	return pgErrorCode(err) == "23503"
}

// isInvalidUUID reports whether PostgreSQL rejected an ID that is not a UUID,
// which for lookups means the row cannot exist.
func isInvalidUUID(err error) bool {
	return pgErrorCode(err) == "22P02"
}
