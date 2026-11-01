package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"chess-arena/arena/internal/tournament"
)

// Tournament is a stored tournament with its configuration.
type Tournament struct {
	ID string `json:"id"`
	tournament.Config
	Status    tournament.Status `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Progress counts games by status.
type Progress struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

// Score is the head-to-head result from engine A's point of view.
type Score struct {
	EngineAWins   int     `json:"engine_a_wins"`
	EngineBWins   int     `json:"engine_b_wins"`
	Draws         int     `json:"draws"`
	EngineAPoints float64 `json:"engine_a_points"`
	EngineBPoints float64 `json:"engine_b_points"`
}

// TournamentDetail is a tournament with its progress and score.
type TournamentDetail struct {
	Tournament
	Progress Progress `json:"progress"`
	Score    Score    `json:"score"`
}

const tournamentColumns = `id, name, engine_a_id, engine_b_id, position_set_id, game_count,
	movetime_ms, depth, nodes, color_order, cpu_limit, memory_limit_mb, status, created_at, updated_at`

func scanTournament(row pgx.Row) (Tournament, error) {
	var t Tournament
	var depth *int32
	var movetime *int32
	err := row.Scan(&t.ID, &t.Name, &t.EngineAID, &t.EngineBID, &t.PositionSetID, &t.GameCount,
		&movetime, &depth, &t.Limit.Nodes, &t.ColorOrder, &t.CPUs, &t.MemoryMB, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if movetime != nil {
		value := int(*movetime)
		t.Limit.MoveTimeMs = &value
	}
	if depth != nil {
		value := int(*depth)
		t.Limit.Depth = &value
	}
	return t, err
}

// CreateTournament validates config, stores the tournament and schedules all
// of its games. The tournament is queued, so workers start playing it at once.
func (store *Store) CreateTournament(ctx context.Context, config tournament.Config) (Tournament, error) {
	config = config.Normalize()
	if err := config.Validate(); err != nil {
		return Tournament{}, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Tournament{}, fmt.Errorf("begin tournament transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	positionIDs, positionFENs, err := loadPositions(ctx, tx, config.PositionSetID)
	if err != nil {
		return Tournament{}, err
	}

	created, err := scanTournament(tx.QueryRow(ctx, `
		INSERT INTO tournaments (name, engine_a_id, engine_b_id, position_set_id, game_count,
			movetime_ms, depth, nodes, color_order, cpu_limit, memory_limit_mb, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING `+tournamentColumns,
		config.Name, config.EngineAID, config.EngineBID, config.PositionSetID, config.GameCount,
		config.Limit.MoveTimeMs, config.Limit.Depth, config.Limit.Nodes, string(config.ColorOrder),
		config.CPUs, config.MemoryMB, string(tournament.StatusQueued)))
	if isForeignKeyViolation(err) || isInvalidUUID(err) {
		return Tournament{}, ErrInvalidReference
	}
	if err != nil {
		return Tournament{}, fmt.Errorf("insert tournament: %w", err)
	}

	schedule := tournament.Schedule(config, len(positionIDs))
	numbers := make([]int32, 0, len(schedule))
	positions := make([]string, 0, len(schedule))
	whites := make([]string, 0, len(schedule))
	blacks := make([]string, 0, len(schedule))
	fens := make([]string, 0, len(schedule))
	sides := make([]string, 0, len(schedule))
	keys := make([]string, 0, len(schedule))
	for _, game := range schedule {
		white, black := config.EngineAID, config.EngineBID
		if !game.EngineAIsWhite {
			white, black = black, white
		}
		fen := positionFENs[game.PositionOrdinal]
		numbers = append(numbers, int32(game.Number))
		positions = append(positions, positionIDs[game.PositionOrdinal])
		whites = append(whites, white)
		blacks = append(blacks, black)
		fens = append(fens, fen)
		sides = append(sides, strings.Fields(fen)[1])
		keys = append(keys, tournament.IdempotencyKey(created.ID, game.Number, 0, "scheduled"))
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO games (tournament_id, game_number, position_id, white_engine_id, black_engine_id,
			current_fen, next_side, idempotency_key)
		SELECT $1, number, position_id::uuid, white::uuid, black::uuid, fen, side, key
		FROM unnest($2::int[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[], $8::text[])
			AS input(number, position_id, white, black, fen, side, key)
	`, created.ID, numbers, positions, whites, blacks, fens, sides, keys); err != nil {
		return Tournament{}, fmt.Errorf("schedule games: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Tournament{}, fmt.Errorf("commit tournament: %w", err)
	}
	return created, nil
}

// loadPositions returns position IDs and FENs indexed by ordinal.
func loadPositions(ctx context.Context, tx pgx.Tx, positionSetID string) ([]string, []string, error) {
	rows, err := tx.Query(ctx, "SELECT id, fen FROM positions WHERE position_set_id = $1 ORDER BY ordinal", positionSetID)
	if isInvalidUUID(err) {
		return nil, nil, ErrInvalidReference
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load positions: %w", err)
	}
	defer rows.Close()
	var ids, fens []string
	for rows.Next() {
		var id, fen string
		if err := rows.Scan(&id, &fen); err != nil {
			return nil, nil, fmt.Errorf("scan position: %w", err)
		}
		ids = append(ids, id)
		fens = append(fens, fen)
	}
	if err := rows.Err(); err != nil {
		if isInvalidUUID(err) {
			return nil, nil, ErrInvalidReference
		}
		return nil, nil, err
	}
	if len(ids) == 0 {
		return nil, nil, ErrInvalidReference
	}
	return ids, fens, nil
}

// ListTournaments returns all tournaments, newest first.
func (store *Store) ListTournaments(ctx context.Context) ([]Tournament, error) {
	rows, err := store.pool.Query(ctx, "SELECT "+tournamentColumns+" FROM tournaments ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}
	defer rows.Close()
	tournaments := []Tournament{}
	for rows.Next() {
		t, err := scanTournament(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tournament: %w", err)
		}
		tournaments = append(tournaments, t)
	}
	return tournaments, rows.Err()
}

// GetTournament returns a tournament with its progress and score.
func (store *Store) GetTournament(ctx context.Context, id string) (TournamentDetail, error) {
	var detail TournamentDetail
	t, err := scanTournament(store.pool.QueryRow(ctx, "SELECT "+tournamentColumns+" FROM tournaments WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return detail, ErrNotFound
	}
	if err != nil {
		return detail, fmt.Errorf("read tournament: %w", err)
	}
	detail.Tournament = t

	progress := &detail.Progress
	score := &detail.Score
	err = store.pool.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE status = 'queued'),
			count(*) FILTER (WHERE status = 'running'),
			count(*) FILTER (WHERE status = 'completed'),
			count(*) FILTER (WHERE status = 'failed'),
			count(*) FILTER (WHERE status = 'cancelled'),
			count(*) FILTER (WHERE (result = '1-0' AND white_engine_id = $2) OR (result = '0-1' AND black_engine_id = $2)),
			count(*) FILTER (WHERE (result = '1-0' AND white_engine_id = $3) OR (result = '0-1' AND black_engine_id = $3)),
			count(*) FILTER (WHERE result = '1/2-1/2')
		FROM games
		WHERE tournament_id = $1
	`, id, t.EngineAID, t.EngineBID).Scan(&progress.Total, &progress.Queued, &progress.Running,
		&progress.Completed, &progress.Failed, &progress.Cancelled,
		&score.EngineAWins, &score.EngineBWins, &score.Draws)
	if err != nil {
		return detail, fmt.Errorf("read tournament progress: %w", err)
	}
	score.EngineAPoints = float64(score.EngineAWins) + float64(score.Draws)/2
	score.EngineBPoints = float64(score.EngineBWins) + float64(score.Draws)/2
	return detail, nil
}

// CancelTournament stops a tournament. Queued games are cancelled, and running
// games lose their lease, so their workers stop at the next move.
func (store *Store) CancelTournament(ctx context.Context, id string) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin cancel transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE tournaments SET status = 'cancelled', updated_at = now()
		WHERE id = $1 AND status IN ('created', 'queued', 'running', 'paused')
	`, id)
	if isInvalidUUID(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("cancel tournament: %w", err)
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM tournaments WHERE id = $1)", id).Scan(&exists); err != nil {
			return fmt.Errorf("check tournament: %w", err)
		}
		if !exists {
			return ErrNotFound
		}
		return fmt.Errorf("tournament already finished: %w", ErrConflict)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE games
		SET status = 'cancelled', lease_owner = NULL, lease_expires_at = NULL, finished_at = now(), updated_at = now()
		WHERE tournament_id = $1 AND status IN ('queued', 'running')
	`, id); err != nil {
		return fmt.Errorf("cancel games: %w", err)
	}
	return tx.Commit(ctx)
}

// completeTournamentIfDone marks a tournament completed once no game is left to play.
func completeTournamentIfDone(ctx context.Context, tx pgx.Tx, tournamentID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE tournaments SET status = 'completed', updated_at = now()
		WHERE id = $1
		  AND status IN ('queued', 'running')
		  AND NOT EXISTS (
			SELECT 1 FROM games WHERE tournament_id = $1 AND status IN ('queued', 'running'))
	`, tournamentID)
	if err != nil {
		return fmt.Errorf("complete tournament: %w", err)
	}
	return nil
}
