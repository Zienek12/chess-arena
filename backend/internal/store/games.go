package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"chess-arena/arena/internal/tournament"
	"chess-arena/arena/internal/uci"
)

// ClaimedGame is everything a worker needs to play or resume a game.
type ClaimedGame struct {
	ID             string
	TournamentID   string
	TournamentName string
	GameNumber     int
	Attempt        int
	StartFEN       string
	Moves          []string // moves already played, when resuming
	White          Engine
	Black          Engine
	Limit          tournament.SearchLimit
	CPUs           float64
	MemoryMB       int
}

// MoveRecord is one played move with its telemetry.
type MoveRecord struct {
	GameID       string
	TournamentID string
	GameNumber   int
	Ply          int
	UCI          string
	FENAfter     string
	NextSide     string
	EngineID     string
	Info         uci.Info
	ThinkTime    time.Duration
}

// GameResult is the final outcome of a game.
type GameResult struct {
	GameID         string
	TournamentID   string
	Result         string
	Termination    string
	WinnerEngineID string // empty for a draw
	PGN            string
}

// GameFailure describes why a worker could not finish a game.
type GameFailure struct {
	Stage     string
	Code      string
	Message   string
	Retryable bool
}

// ClaimGame leases the next playable game: a queued game, or a running game
// whose worker's lease expired. It returns nil when there is nothing to play.
func (store *Store) ClaimGame(ctx context.Context, owner string, lease time.Duration) (*ClaimedGame, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var game ClaimedGame
	var positionID, whiteID, blackID string
	err = tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT g.id
			FROM games g
			JOIN tournaments t ON t.id = g.tournament_id
			WHERE t.status IN ('queued', 'running')
			  AND (g.status = 'queued' OR (g.status = 'running' AND g.lease_expires_at < now()))
			ORDER BY t.created_at, g.game_number
			LIMIT 1
			FOR UPDATE OF g SKIP LOCKED
		)
		UPDATE games g
		SET status = 'running',
		    lease_owner = $1,
		    lease_expires_at = now() + $2 * interval '1 millisecond',
		    attempt = g.attempt + 1,
		    started_at = COALESCE(g.started_at, now()),
		    updated_at = now()
		FROM candidate
		WHERE g.id = candidate.id
		RETURNING g.id, g.tournament_id, g.game_number, g.attempt, g.position_id, g.white_engine_id, g.black_engine_id
	`, owner, lease.Milliseconds()).Scan(&game.ID, &game.TournamentID, &game.GameNumber, &game.Attempt,
		&positionID, &whiteID, &blackID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim game: %w", err)
	}

	var depth, movetime *int32
	err = tx.QueryRow(ctx, `
		SELECT t.name, t.movetime_ms, t.depth, t.nodes, t.cpu_limit, t.memory_limit_mb, p.fen
		FROM tournaments t, positions p
		WHERE t.id = $1 AND p.id = $2
	`, game.TournamentID, positionID).Scan(&game.TournamentName, &movetime, &depth, &game.Limit.Nodes,
		&game.CPUs, &game.MemoryMB, &game.StartFEN)
	if err != nil {
		return nil, fmt.Errorf("read claimed game settings: %w", err)
	}
	if movetime != nil {
		value := int(*movetime)
		game.Limit.MoveTimeMs = &value
	}
	if depth != nil {
		value := int(*depth)
		game.Limit.Depth = &value
	}
	if game.White, err = getEngine(ctx, tx, whiteID); err != nil {
		return nil, fmt.Errorf("read white engine: %w", err)
	}
	if game.Black, err = getEngine(ctx, tx, blackID); err != nil {
		return nil, fmt.Errorf("read black engine: %w", err)
	}

	rows, err := tx.Query(ctx, "SELECT uci FROM game_moves WHERE game_id = $1 ORDER BY ply", game.ID)
	if err != nil {
		return nil, fmt.Errorf("read played moves: %w", err)
	}
	game.Moves, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("scan played moves: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE tournaments SET status = 'running', updated_at = now()
		WHERE id = $1 AND status = 'queued'
	`, game.TournamentID); err != nil {
		return nil, fmt.Errorf("start tournament: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim: %w", err)
	}
	return &game, nil
}

// RecordMove stores a move with its telemetry, advances the game state and
// renews the lease. Storing the same move twice is a no-op.
func (store *Store) RecordMove(ctx context.Context, owner string, lease time.Duration, move MoveRecord) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin move transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	key := tournament.IdempotencyKey(move.TournamentID, move.GameNumber, move.Ply, move.UCI)
	result, err := tx.Exec(ctx, `
		UPDATE games
		SET current_fen = $3, ply = $4, next_side = $5, state_version = $8, idempotency_key = $6,
		    lease_expires_at = now() + $7 * interval '1 millisecond', updated_at = now()
		WHERE id = $1 AND status = 'running' AND lease_owner = $2
	`, move.GameID, owner, move.FENAfter, move.Ply, move.NextSide, key, lease.Milliseconds(), int64(move.Ply))
	if err != nil {
		return fmt.Errorf("update game state: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrLeaseLost
	}

	info := move.Info
	var pv *string
	if len(info.PV) > 0 {
		joined := strings.Join(info.PV, " ")
		pv = &joined
	}
	inserted, err := tx.Exec(ctx, `
		INSERT INTO game_moves (game_id, ply, uci, fen_after, engine_id, idempotency_key,
			depth, seldepth, score_cp, score_mate, nodes, nps, engine_time_ms, pv, think_time_ms)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, 0), NULLIF($8, 0), $9, $10, NULLIF($11::bigint, 0), NULLIF($12::bigint, 0), NULLIF($13::bigint, 0), $14, $15)
		ON CONFLICT (game_id, ply) DO NOTHING
	`, move.GameID, move.Ply, move.UCI, move.FENAfter, move.EngineID, key,
		info.Depth, info.SelDepth, info.ScoreCP, info.ScoreMate, info.Nodes, info.NPS, info.TimeMs, pv,
		move.ThinkTime.Milliseconds())
	if err != nil {
		return fmt.Errorf("insert game move: %w", err)
	}
	if inserted.RowsAffected() == 0 {
		var existingKey string
		if err := tx.QueryRow(ctx, "SELECT idempotency_key FROM game_moves WHERE game_id = $1 AND ply = $2",
			move.GameID, move.Ply).Scan(&existingKey); err != nil {
			return fmt.Errorf("read existing game move: %w", err)
		}
		if existingKey != key {
			return ErrConflictingMove
		}
	}
	return tx.Commit(ctx)
}

// FinishGame stores the result and PGN, and completes the tournament when this was its last game.
func (store *Store) FinishGame(ctx context.Context, owner string, result GameResult) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finish transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	updated, err := tx.Exec(ctx, `
		UPDATE games
		SET status = 'completed', result = $3, lease_owner = NULL, lease_expires_at = NULL,
		    finished_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'running' AND lease_owner = $2
	`, result.GameID, owner, result.Result)
	if err != nil {
		return fmt.Errorf("complete game: %w", err)
	}
	if updated.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO game_results (game_id, result, termination, winner_engine_id, pgn)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5)
		ON CONFLICT (game_id) DO NOTHING
	`, result.GameID, result.Result, result.Termination, result.WinnerEngineID, result.PGN); err != nil {
		return fmt.Errorf("insert game result: %w", err)
	}
	if err := completeTournamentIfDone(ctx, tx, result.TournamentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReleaseGame records why a game could not be finished. With requeue the game
// goes back to the queue for another attempt; otherwise it is marked failed.
func (store *Store) ReleaseGame(ctx context.Context, owner string, game *ClaimedGame, failure GameFailure, requeue bool) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin release transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO game_errors (game_id, stage, code, message, retryable)
		VALUES ($1, $2, $3, $4, $5)
	`, game.ID, failure.Stage, failure.Code, failure.Message, failure.Retryable); err != nil {
		return fmt.Errorf("insert game error: %w", err)
	}
	status := "failed"
	if requeue {
		status = "queued"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE games
		SET status = $3, lease_owner = NULL, lease_expires_at = NULL, updated_at = now(),
		    finished_at = CASE WHEN $3 = 'failed' THEN now() ELSE finished_at END
		WHERE id = $1 AND status = 'running' AND lease_owner = $2
	`, game.ID, owner, status); err != nil {
		return fmt.Errorf("release game: %w", err)
	}
	if err := completeTournamentIfDone(ctx, tx, game.TournamentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
