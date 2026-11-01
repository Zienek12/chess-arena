package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"chess-arena/arena/internal/tournament"
)

// PositionSetSummary describes a stored position set without its positions.
type PositionSetSummary struct {
	ID            string    `json:"id"`
	Fingerprint   string    `json:"fingerprint"`
	SourceType    string    `json:"source_type"`
	SourceName    string    `json:"source_name"`
	PositionCount int       `json:"position_count"`
	CreatedAt     time.Time `json:"created_at"`
}

const positionSetColumns = "id, fingerprint, source_type, source_name, position_count, created_at"

func scanPositionSet(row pgx.Row) (PositionSetSummary, error) {
	var set PositionSetSummary
	err := row.Scan(&set.ID, &set.Fingerprint, &set.SourceType, &set.SourceName, &set.PositionCount, &set.CreatedAt)
	return set, err
}

// SavePositionSet stores an imported set. Importing identical positions again
// returns the existing set, and created is false.
func (store *Store) SavePositionSet(ctx context.Context, name string, set tournament.PositionSet) (summary PositionSetSummary, created bool, err error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return summary, false, fmt.Errorf("begin position set transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	summary, err = scanPositionSet(tx.QueryRow(ctx, `
		INSERT INTO position_sets (fingerprint, source_type, source_name, position_count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (fingerprint) DO NOTHING
		RETURNING `+positionSetColumns,
		set.Fingerprint, string(set.SourceType), name, len(set.Positions)))
	if errors.Is(err, pgx.ErrNoRows) {
		summary, err = scanPositionSet(tx.QueryRow(ctx,
			"SELECT "+positionSetColumns+" FROM position_sets WHERE fingerprint = $1", set.Fingerprint))
		if err != nil {
			return summary, false, fmt.Errorf("read existing position set: %w", err)
		}
		return summary, false, nil
	}
	if err != nil {
		return summary, false, fmt.Errorf("insert position set: %w", err)
	}

	ordinals := make([]int32, 0, len(set.Positions))
	fens := make([]string, 0, len(set.Positions))
	hashes := make([]string, 0, len(set.Positions))
	for _, position := range set.Positions {
		ordinals = append(ordinals, int32(position.Ordinal))
		fens = append(fens, position.FEN)
		hashes = append(hashes, position.Hash)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO positions (position_set_id, ordinal, fen, fen_hash)
		SELECT $1, ordinal, fen, fen_hash
		FROM unnest($2::int[], $3::text[], $4::text[]) AS input(ordinal, fen, fen_hash)
	`, summary.ID, ordinals, fens, hashes); err != nil {
		return summary, false, fmt.Errorf("insert positions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return summary, false, fmt.Errorf("commit position set: %w", err)
	}
	return summary, true, nil
}

// ListPositionSets returns all sets, newest first.
func (store *Store) ListPositionSets(ctx context.Context) ([]PositionSetSummary, error) {
	rows, err := store.pool.Query(ctx, "SELECT "+positionSetColumns+" FROM position_sets ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list position sets: %w", err)
	}
	defer rows.Close()
	sets := []PositionSetSummary{}
	for rows.Next() {
		set, err := scanPositionSet(rows)
		if err != nil {
			return nil, fmt.Errorf("scan position set: %w", err)
		}
		sets = append(sets, set)
	}
	return sets, rows.Err()
}
