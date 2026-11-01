package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"chess-arena/arena/internal/uci"
)

// GameSummary is one row of a tournament's game list.
type GameSummary struct {
	ID            string     `json:"id"`
	TournamentID  string     `json:"tournament_id"`
	GameNumber    int        `json:"game_number"`
	Status        string     `json:"status"`
	Result        string     `json:"result,omitempty"`
	Termination   string     `json:"termination,omitempty"`
	WhiteEngineID string     `json:"white_engine_id"`
	BlackEngineID string     `json:"black_engine_id"`
	Ply           int        `json:"ply"`
	Attempt       int        `json:"attempt"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

// MoveDetail is a stored move with its telemetry.
type MoveDetail struct {
	Ply         int      `json:"ply"`
	UCI         string   `json:"uci"`
	FENAfter    string   `json:"fen_after"`
	EngineID    string   `json:"engine_id"`
	Info        uci.Info `json:"info"`
	ThinkTimeMs *int     `json:"think_time_ms,omitempty"`
}

// GameDetail is a game with its moves, result and PGN.
type GameDetail struct {
	GameSummary
	StartFEN   string       `json:"start_fen"`
	CurrentFEN string       `json:"current_fen"`
	Moves      []MoveDetail `json:"moves"`
	PGN        string       `json:"pgn,omitempty"`
}

const gameSummaryColumns = `g.id, g.tournament_id, g.game_number, g.status, COALESCE(g.result, ''),
	COALESCE(r.termination, ''), g.white_engine_id, g.black_engine_id, g.ply, g.attempt, g.started_at, g.finished_at`

func scanGameSummary(row pgx.Row, extra ...any) (GameSummary, error) {
	var game GameSummary
	targets := []any{&game.ID, &game.TournamentID, &game.GameNumber, &game.Status, &game.Result,
		&game.Termination, &game.WhiteEngineID, &game.BlackEngineID, &game.Ply, &game.Attempt,
		&game.StartedAt, &game.FinishedAt}
	err := row.Scan(append(targets, extra...)...)
	return game, err
}

// ListGames returns the games of a tournament in schedule order.
func (store *Store) ListGames(ctx context.Context, tournamentID string) ([]GameSummary, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT `+gameSummaryColumns+`
		FROM games g LEFT JOIN game_results r ON r.game_id = g.id
		WHERE g.tournament_id = $1
		ORDER BY g.game_number
	`, tournamentID)
	if isInvalidUUID(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()
	games := []GameSummary{}
	for rows.Next() {
		game, err := scanGameSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		if isInvalidUUID(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return games, nil
}

// GetGame returns a game with its moves and, once finished, its PGN.
func (store *Store) GetGame(ctx context.Context, id string) (GameDetail, error) {
	var detail GameDetail
	var pgn *string
	summary, err := scanGameSummary(store.pool.QueryRow(ctx, `
		SELECT `+gameSummaryColumns+`, p.fen, g.current_fen, r.pgn
		FROM games g
		JOIN positions p ON p.id = g.position_id
		LEFT JOIN game_results r ON r.game_id = g.id
		WHERE g.id = $1
	`, id), &detail.StartFEN, &detail.CurrentFEN, &pgn)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return detail, ErrNotFound
	}
	if err != nil {
		return detail, fmt.Errorf("read game: %w", err)
	}
	detail.GameSummary = summary
	if pgn != nil {
		detail.PGN = *pgn
	}

	rows, err := store.pool.Query(ctx, `
		SELECT ply, uci, fen_after, engine_id, depth, seldepth, score_cp, score_mate, nodes, nps,
			engine_time_ms, pv, think_time_ms
		FROM game_moves WHERE game_id = $1 ORDER BY ply
	`, id)
	if err != nil {
		return detail, fmt.Errorf("read moves: %w", err)
	}
	defer rows.Close()
	detail.Moves = []MoveDetail{}
	for rows.Next() {
		var move MoveDetail
		var depth, seldepth *int
		var nodes, nps, engineTime *int64
		var pv *string
		if err := rows.Scan(&move.Ply, &move.UCI, &move.FENAfter, &move.EngineID, &depth, &seldepth,
			&move.Info.ScoreCP, &move.Info.ScoreMate, &nodes, &nps, &engineTime, &pv, &move.ThinkTimeMs); err != nil {
			return detail, fmt.Errorf("scan move: %w", err)
		}
		move.Info.Depth = valueOrZero(depth)
		move.Info.SelDepth = valueOrZero(seldepth)
		move.Info.Nodes = valueOrZero(nodes)
		move.Info.NPS = valueOrZero(nps)
		move.Info.TimeMs = valueOrZero(engineTime)
		if pv != nil {
			move.Info.PV = strings.Fields(*pv)
		}
		detail.Moves = append(detail.Moves, move)
	}
	return detail, rows.Err()
}

func valueOrZero[T int | int64](value *T) T {
	if value == nil {
		return 0
	}
	return *value
}
