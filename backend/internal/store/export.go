package store

import (
	"context"
	"fmt"
	"strings"
)

// Table is tabular export data: a header and rows of text cells.
type Table struct {
	Header []string
	Rows   [][]string
}

// sideStats aggregates one side's moves of game "g"; side is "white" or "black".
func sideStats(side string) string {
	return fmt.Sprintf(`
		(SELECT concat_ws(',',
			coalesce(round(avg(m.depth), 2)::text, ''),
			coalesce(round(avg(m.nodes))::text, ''),
			coalesce(round(avg(m.nps))::text, ''),
			coalesce(round(avg(m.think_time_ms))::text, ''),
			coalesce(round(avg(ma.accuracy)::numeric, 2)::text, ''),
			coalesce(round(avg(ma.cp_loss), 1)::text, ''),
			count(*) FILTER (WHERE ma.classification = 'blunder')::text)
		 FROM game_moves m
		 LEFT JOIN move_analyses ma ON ma.game_id = m.game_id AND ma.ply = m.ply
		 WHERE m.game_id = g.id AND m.engine_id = g.%s_engine_id)`, side)
}

var sideColumns = []string{"avg_depth", "avg_nodes", "avg_nps", "avg_think_ms", "accuracy", "acpl", "blunders"}

// ExportGames returns one row per game of a tournament, with each side's
// average telemetry and move quality.
func (store *Store) ExportGames(ctx context.Context, tournamentID string) (Table, error) {
	header := []string{"game", "white", "black", "white_image_id", "black_image_id", "white_uci_name", "black_uci_name",
		"position", "opening", "start_fen", "status", "result", "termination", "plies", "started_at", "finished_at"}
	for _, side := range []string{"white", "black"} {
		for _, column := range sideColumns {
			header = append(header, side+"_"+column)
		}
	}
	rows, err := store.pool.Query(ctx, `
		SELECT g.game_number::text, w.name || ' ' || w.version, b.name || ' ' || b.version,
			coalesce(g.white_image_id, ''), coalesce(g.black_image_id, ''),
			coalesce(g.white_uci_name, ''), coalesce(g.black_uci_name, ''),
			(p.ordinal + 1)::text, coalesce(p.name, ''), p.fen, g.status, coalesce(g.result, ''),
			coalesce(r.termination, ''), g.ply::text,
			coalesce(to_char(g.started_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), ''),
			coalesce(to_char(g.finished_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), ''),
			`+sideStats("white")+`, `+sideStats("black")+`
		FROM games g
		JOIN engines w ON w.id = g.white_engine_id
		JOIN engines b ON b.id = g.black_engine_id
		JOIN positions p ON p.id = g.position_id
		LEFT JOIN game_results r ON r.game_id = g.id
		WHERE g.tournament_id = $1
		ORDER BY g.game_number
	`, tournamentID)
	if err != nil {
		return Table{}, fmt.Errorf("export games: %w", err)
	}
	defer rows.Close()
	table := Table{Header: header, Rows: [][]string{}}
	for rows.Next() {
		cells := make([]string, 16)
		var white, black string
		targets := make([]any, 0, 18)
		for index := range cells {
			targets = append(targets, &cells[index])
		}
		if err := rows.Scan(append(targets, &white, &black)...); err != nil {
			return Table{}, fmt.Errorf("scan exported game: %w", err)
		}
		cells = append(cells, strings.Split(white, ",")...)
		table.Rows = append(table.Rows, append(cells, strings.Split(black, ",")...))
	}
	return table, rows.Err()
}

// ExportMoves returns one row per move of a tournament, with the engine's
// telemetry and the analysis engine's verdict.
func (store *Store) ExportMoves(ctx context.Context, tournamentID string) (Table, error) {
	header := []string{"game", "ply", "side", "engine", "move", "depth", "seldepth", "score_cp", "score_mate",
		"nodes", "nps", "engine_time_ms", "think_time_ms", "pv", "sf_best_move", "sf_eval_before_cp",
		"sf_eval_after_cp", "sf_mate_after", "cp_loss", "accuracy", "classification"}
	rows, err := store.pool.Query(ctx, `
		SELECT g.game_number::text, m.ply::text,
			CASE WHEN m.engine_id = g.white_engine_id THEN 'w' ELSE 'b' END,
			e.name || ' ' || e.version, m.uci,
			coalesce(m.depth::text, ''), coalesce(m.seldepth::text, ''),
			coalesce(m.score_cp::text, ''), coalesce(m.score_mate::text, ''),
			coalesce(m.nodes::text, ''), coalesce(m.nps::text, ''), coalesce(m.engine_time_ms::text, ''),
			coalesce(m.think_time_ms::text, ''), coalesce(m.pv, ''),
			coalesce(ma.best_move, ''), coalesce(ma.before_cp::text, ''), coalesce(ma.after_cp::text, ''),
			coalesce(ma.after_mate::text, ''), coalesce(ma.cp_loss::text, ''),
			coalesce(round(ma.accuracy::numeric, 2)::text, ''), coalesce(ma.classification, '')
		FROM game_moves m
		JOIN games g ON g.id = m.game_id
		JOIN engines e ON e.id = m.engine_id
		LEFT JOIN move_analyses ma ON ma.game_id = m.game_id AND ma.ply = m.ply
		WHERE g.tournament_id = $1
		ORDER BY g.game_number, m.ply
	`, tournamentID)
	if err != nil {
		return Table{}, fmt.Errorf("export moves: %w", err)
	}
	defer rows.Close()
	table := Table{Header: header, Rows: [][]string{}}
	for rows.Next() {
		cells := make([]string, len(header))
		targets := make([]any, len(cells))
		for index := range cells {
			targets[index] = &cells[index]
		}
		if err := rows.Scan(targets...); err != nil {
			return Table{}, fmt.Errorf("scan exported move: %w", err)
		}
		table.Rows = append(table.Rows, cells)
	}
	return table, rows.Err()
}

// TournamentPGN returns the PGN of every finished game, in schedule order.
func (store *Store) TournamentPGN(ctx context.Context, tournamentID string) (string, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT r.pgn FROM games g JOIN game_results r ON r.game_id = g.id
		WHERE g.tournament_id = $1 AND r.pgn IS NOT NULL
		ORDER BY g.game_number
	`, tournamentID)
	if err != nil {
		return "", fmt.Errorf("export PGN: %w", err)
	}
	defer rows.Close()
	var games []string
	for rows.Next() {
		var pgn string
		if err := rows.Scan(&pgn); err != nil {
			return "", fmt.Errorf("scan PGN: %w", err)
		}
		games = append(games, strings.TrimSpace(pgn))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(games) == 0 {
		return "", nil
	}
	return strings.Join(games, "\n\n") + "\n", nil
}
