package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Eval is an evaluation from White's point of view. CP is always set; a mate
// counts as ±MateCP, so CP also gives the sign when Mate is 0 (checkmate on
// the board).
type Eval struct {
	CP   int  `json:"cp"`
	Mate *int `json:"mate,omitempty"`
}

// MateCP is the centipawn value of a forced mate.
const MateCP = 10_000

// MoveAnalysis is the analysis engine's verdict on one played move.
type MoveAnalysis struct {
	Ply      int      `json:"ply"`
	BestMove string   `json:"best_move,omitempty"`
	BestPV   []string `json:"best_pv,omitempty"`
	Depth    int      `json:"depth"`
	// EvalBefore is the position the mover faced, EvalAfter the one the move produced.
	EvalBefore Eval `json:"eval_before"`
	EvalAfter  Eval `json:"eval_after"`
	// CPLoss is how much worse the move is than the best one, capped at 1000.
	CPLoss int `json:"cp_loss"`
	// Accuracy is 0–100, from the drop in winning chances (as on Lichess).
	Accuracy float64 `json:"accuracy"`
	// Classification is best, good, inaccuracy, mistake or blunder.
	Classification string `json:"classification"`
}

// GameAnalysis describes the analysis of a game.
type GameAnalysis struct {
	Status string `json:"status"` // running, completed or failed
	Engine string `json:"engine,omitempty"`
	// ImageID is the Docker image ID of the analysis engine.
	ImageID string `json:"image_id,omitempty"`
	Nodes   int64  `json:"nodes,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AnalysisJob is a finished game leased for analysis.
type AnalysisJob struct {
	GameID       string
	TournamentID string
	Attempt      int
	StartFEN     string
	Moves        []string
}

// ClaimAnalysis leases the most recently finished game that has not been
// analyzed, or whose analysis lease expired. It returns nil when there is none.
func (store *Store) ClaimAnalysis(ctx context.Context, owner string, lease time.Duration) (*AnalysisJob, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin analysis claim: %w", err)
	}
	defer tx.Rollback(ctx)

	var job AnalysisJob
	err = tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT g.id
			FROM games g
			LEFT JOIN game_analyses a ON a.game_id = g.id
			WHERE g.status = 'completed'
			  AND (a.game_id IS NULL OR (a.status = 'running' AND a.lease_expires_at < now()))
			ORDER BY g.finished_at DESC
			LIMIT 1
			FOR UPDATE OF g SKIP LOCKED
		)
		INSERT INTO game_analyses (game_id, status, lease_owner, lease_expires_at)
		SELECT id, 'running', $1, now() + $2 * interval '1 millisecond' FROM candidate
		ON CONFLICT (game_id) DO UPDATE
		SET lease_owner = EXCLUDED.lease_owner, lease_expires_at = EXCLUDED.lease_expires_at,
		    attempt = game_analyses.attempt + 1, updated_at = now()
		RETURNING game_id, attempt
	`, owner, lease.Milliseconds()).Scan(&job.GameID, &job.Attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim analysis: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT g.tournament_id, p.fen FROM games g JOIN positions p ON p.id = g.position_id WHERE g.id = $1
	`, job.GameID).Scan(&job.TournamentID, &job.StartFEN); err != nil {
		return nil, fmt.Errorf("read game to analyze: %w", err)
	}
	rows, err := tx.Query(ctx, "SELECT uci FROM game_moves WHERE game_id = $1 ORDER BY ply", job.GameID)
	if err != nil {
		return nil, fmt.Errorf("read moves to analyze: %w", err)
	}
	if job.Moves, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		return nil, fmt.Errorf("scan moves to analyze: %w", err)
	}
	if err := notify(ctx, tx, Event{Type: EventAnalysis, TournamentID: job.TournamentID, GameID: job.GameID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit analysis claim: %w", err)
	}
	return &job, nil
}

// SaveAnalysis stores the analysis of every move and completes the job.
func (store *Store) SaveAnalysis(ctx context.Context, owner string, job *AnalysisJob, engine EngineBuild, nodes int64, moves []MoveAnalysis) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin analysis save: %w", err)
	}
	defer tx.Rollback(ctx)

	updated, err := tx.Exec(ctx, `
		UPDATE game_analyses
		SET status = 'completed', engine = $3, image_id = NULLIF($5, ''), nodes = $4, error = NULL,
		    lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
		WHERE game_id = $1 AND status = 'running' AND lease_owner = $2
	`, job.GameID, owner, engine.UCIName, nodes, engine.ImageID)
	if err != nil {
		return fmt.Errorf("complete analysis: %w", err)
	}
	if updated.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	if _, err := tx.Exec(ctx, "DELETE FROM move_analyses WHERE game_id = $1", job.GameID); err != nil {
		return fmt.Errorf("clear move analyses: %w", err)
	}
	batch := &pgx.Batch{}
	for _, move := range moves {
		batch.Queue(`
			INSERT INTO move_analyses (game_id, ply, best_move, best_pv, depth, before_cp, before_mate,
				after_cp, after_mate, cp_loss, accuracy, classification)
			VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11, $12)
		`, job.GameID, move.Ply, move.BestMove, strings.Join(move.BestPV, " "), move.Depth,
			move.EvalBefore.CP, move.EvalBefore.Mate, move.EvalAfter.CP, move.EvalAfter.Mate,
			move.CPLoss, move.Accuracy, move.Classification)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert move analyses: %w", err)
	}
	if err := notify(ctx, tx, Event{Type: EventAnalysis, TournamentID: job.TournamentID, GameID: job.GameID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FailAnalysis records why a game could not be analyzed. With retry the job
// becomes claimable again after retryAfter; otherwise it is marked failed.
func (store *Store) FailAnalysis(ctx context.Context, owner string, job *AnalysisJob, message string, retry bool, retryAfter time.Duration) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin analysis failure: %w", err)
	}
	defer tx.Rollback(ctx)
	status := "failed"
	if retry {
		status = "running"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE game_analyses
		SET status = $3, error = $4, lease_owner = NULL,
		    lease_expires_at = CASE WHEN $3 = 'running' THEN now() + $5 * interval '1 millisecond' END,
		    updated_at = now()
		WHERE game_id = $1 AND lease_owner = $2
	`, job.GameID, owner, status, message, retryAfter.Milliseconds()); err != nil {
		return fmt.Errorf("record analysis failure: %w", err)
	}
	if err := notify(ctx, tx, Event{Type: EventAnalysis, TournamentID: job.TournamentID, GameID: job.GameID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReleaseAnalysis gives a job back without counting the attempt, because the
// analysis engine itself could not start. It becomes claimable after retryAfter.
func (store *Store) ReleaseAnalysis(ctx context.Context, owner string, job *AnalysisJob, message string, retryAfter time.Duration) error {
	if _, err := store.pool.Exec(ctx, `
		UPDATE game_analyses
		SET attempt = attempt - 1, error = $3, lease_owner = NULL,
		    lease_expires_at = now() + $4 * interval '1 millisecond', updated_at = now()
		WHERE game_id = $1 AND lease_owner = $2 AND status = 'running'
	`, job.GameID, owner, message, retryAfter.Milliseconds()); err != nil {
		return fmt.Errorf("release analysis: %w", err)
	}
	return nil
}

// gameAnalysis reads the analysis of a game; both results are nil before it starts.
func (store *Store) gameAnalysis(ctx context.Context, gameID string) (*GameAnalysis, map[int]*MoveAnalysis, error) {
	var analysis GameAnalysis
	var engine, imageID, message *string
	var nodes *int64
	err := store.pool.QueryRow(ctx, "SELECT status, engine, image_id, nodes, error FROM game_analyses WHERE game_id = $1", gameID).
		Scan(&analysis.Status, &engine, &imageID, &nodes, &message)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read game analysis: %w", err)
	}
	if engine != nil {
		analysis.Engine = *engine
	}
	if imageID != nil {
		analysis.ImageID = *imageID
	}
	if nodes != nil {
		analysis.Nodes = *nodes
	}
	if message != nil {
		analysis.Error = *message
	}

	rows, err := store.pool.Query(ctx, `
		SELECT ply, COALESCE(best_move, ''), COALESCE(best_pv, ''), depth, before_cp, before_mate,
			after_cp, after_mate, cp_loss, accuracy, classification
		FROM move_analyses WHERE game_id = $1
	`, gameID)
	if err != nil {
		return nil, nil, fmt.Errorf("read move analyses: %w", err)
	}
	defer rows.Close()
	moves := map[int]*MoveAnalysis{}
	for rows.Next() {
		var move MoveAnalysis
		var pv string
		if err := rows.Scan(&move.Ply, &move.BestMove, &pv, &move.Depth, &move.EvalBefore.CP, &move.EvalBefore.Mate,
			&move.EvalAfter.CP, &move.EvalAfter.Mate, &move.CPLoss, &move.Accuracy, &move.Classification); err != nil {
			return nil, nil, fmt.Errorf("scan move analysis: %w", err)
		}
		move.BestPV = strings.Fields(pv)
		moves[move.Ply] = &move
	}
	return &analysis, moves, rows.Err()
}
