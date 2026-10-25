package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_tournament_model.sql
var tournamentMigration []byte

var ErrConflictingMove = errors.New("a different move already exists at this ply")

type TournamentStore struct {
	pool *pgxpool.Pool
}

func OpenTournamentStore(ctx context.Context, databaseURL string) (*TournamentStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &TournamentStore{pool: pool}, nil
}

func (store *TournamentStore) Close() {
	store.pool.Close()
}

func (store *TournamentStore) ApplyMigrations(ctx context.Context) error {
	if _, err := store.pool.Exec(ctx, string(tournamentMigration)); err != nil {
		return fmt.Errorf("apply tournament migrations: %w", err)
	}
	return nil
}

func (store *TournamentStore) SaveGameState(ctx context.Context, snapshot GameSnapshot) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin game state transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var currentVersion int64
	var currentKey string
	err = tx.QueryRow(ctx, `
		SELECT state_version, idempotency_key
		FROM games
		WHERE tournament_id = $1 AND game_number = $2
		FOR UPDATE
	`, snapshot.TournamentID, snapshot.GameNumber).Scan(&currentVersion, &currentKey)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read current game state: %w", err)
	}
	if err == nil {
		if currentKey == snapshot.IdempotencyKey {
			return tx.Commit(ctx)
		}
		if snapshot.Version <= currentVersion {
			return ErrStaleGameState
		}
		_, err = tx.Exec(ctx, `
			UPDATE games
			SET current_fen = $1,
			    ply = $2,
			    status = $3,
			    result = NULLIF($4, ''),
			    next_side = $5,
			    attempt = $6,
			    state_version = $7,
			    idempotency_key = $8,
			    updated_at = now()
			WHERE tournament_id = $9 AND game_number = $10
		`, snapshot.CurrentFEN, snapshot.Ply, snapshot.Status, snapshot.Result,
			snapshot.NextSide, snapshot.Attempt, snapshot.Version, snapshot.IdempotencyKey,
			snapshot.TournamentID, snapshot.GameNumber)
		if err != nil {
			return fmt.Errorf("update game state: %w", err)
		}
		return tx.Commit(ctx)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO games (
			tournament_id, game_number, position_id, white_engine_id, black_engine_id,
			current_fen, ply, status, result, next_side, attempt, state_version, idempotency_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10, $11, $12, $13)
	`, snapshot.TournamentID, snapshot.GameNumber, snapshot.PositionID,
		snapshot.WhiteEngineID, snapshot.BlackEngineID, snapshot.CurrentFEN,
		snapshot.Ply, snapshot.Status, snapshot.Result, snapshot.NextSide,
		snapshot.Attempt, snapshot.Version, snapshot.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("insert game state: %w", err)
	}
	return tx.Commit(ctx)
}

func (store *TournamentStore) AppendGameMove(ctx context.Context, move GameMove) error {
	result, err := store.pool.Exec(ctx, `
		INSERT INTO game_moves (game_id, ply, uci, fen_after, engine_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_id, ply) DO NOTHING
	`, move.GameID, move.Ply, move.UCI, move.FENAfter, move.EngineID, move.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("insert game move: %w", err)
	}
	if result.RowsAffected() > 0 {
		return nil
	}

	var existingKey string
	err = store.pool.QueryRow(ctx, `
		SELECT idempotency_key FROM game_moves WHERE game_id = $1 AND ply = $2
	`, move.GameID, move.Ply).Scan(&existingKey)
	if err != nil {
		return fmt.Errorf("read existing game move: %w", err)
	}
	if existingKey == move.IdempotencyKey {
		return nil
	}
	return ErrConflictingMove
}

func (store *TournamentStore) RecordGameError(ctx context.Context, gameID, stage, code, message string, retryable bool) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO game_errors (game_id, stage, code, message, retryable)
		VALUES ($1, $2, $3, $4, $5)
	`, gameID, stage, code, message, retryable)
	if err != nil {
		return fmt.Errorf("insert game error: %w", err)
	}
	return nil
}
