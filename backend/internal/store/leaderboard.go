package store

import (
	"context"
	"fmt"
	"sort"

	"chess-arena/arena/internal/stats"
)

// LeaderboardEntry is one engine's record across every tournament.
type LeaderboardEntry struct {
	Engine
	// Rating is relative Elo from all completed games; ratings average to 0.
	Rating float64 `json:"rating"`
	Games  int     `json:"games"`
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
	Points float64 `json:"points"`
	// AverageDepth and AverageNPS cover moves that reported them.
	AverageDepth float64 `json:"average_depth"`
	AverageNPS   float64 `json:"average_nps"`
	MoveQuality
}

// Leaderboard rates every engine that finished a game, best first.
func (store *Store) Leaderboard(ctx context.Context) ([]LeaderboardEntry, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT white_engine_id, black_engine_id, result, count(*)
		FROM games
		WHERE status = 'completed' AND result IN ('1-0', '0-1', '1/2-1/2')
		GROUP BY white_engine_id, black_engine_id, result
	`)
	if err != nil {
		return nil, fmt.Errorf("read results: %w", err)
	}
	defer rows.Close()
	entries := map[string]*LeaderboardEntry{}
	entry := func(id string) *LeaderboardEntry {
		if entries[id] == nil {
			entries[id] = &LeaderboardEntry{}
		}
		return entries[id]
	}
	var pairings []stats.Pairing
	for rows.Next() {
		var white, black, result string
		var games int
		if err := rows.Scan(&white, &black, &result, &games); err != nil {
			return nil, fmt.Errorf("scan results: %w", err)
		}
		whitePoints := map[string]float64{"1-0": 1, "1/2-1/2": 0.5, "0-1": 0}[result]
		pairings = append(pairings, stats.Pairing{A: white, B: black, Games: games, APoints: whitePoints * float64(games)})
		for _, side := range []struct {
			id     string
			points float64
		}{{white, whitePoints}, {black, 1 - whitePoints}} {
			record := entry(side.id)
			record.Games += games
			record.Points += side.points * float64(games)
			switch side.points {
			case 1:
				record.Wins += games
			case 0.5:
				record.Draws += games
			default:
				record.Losses += games
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []LeaderboardEntry{}, nil
	}
	ratings := stats.Ratings(pairings)

	engines, err := store.ListEngines(ctx)
	if err != nil {
		return nil, err
	}
	for _, engine := range engines {
		if record := entries[engine.ID]; record != nil {
			record.Engine = engine
			record.Rating = ratings[engine.ID]
		}
	}

	rows, err = store.pool.Query(ctx, `
		SELECT m.engine_id, coalesce(avg(m.depth), 0)::float8, coalesce(avg(m.nps), 0)::float8,`+moveQualityColumns+`
		FROM game_moves m
		JOIN games g ON g.id = m.game_id AND g.status = 'completed'
		LEFT JOIN move_analyses ma ON ma.game_id = m.game_id AND ma.ply = m.ply
		GROUP BY m.engine_id
	`)
	if err != nil {
		return nil, fmt.Errorf("read engine moves: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var depth, nps float64
		var quality MoveQuality
		if err := rows.Scan(append([]any{&id, &depth, &nps}, quality.targets()...)...); err != nil {
			return nil, fmt.Errorf("scan engine moves: %w", err)
		}
		if record := entries[id]; record != nil {
			record.AverageDepth, record.AverageNPS, record.MoveQuality = depth, nps, quality
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	leaderboard := make([]LeaderboardEntry, 0, len(entries))
	for _, record := range entries {
		leaderboard = append(leaderboard, *record)
	}
	sort.Slice(leaderboard, func(i, j int) bool {
		if leaderboard[i].Rating != leaderboard[j].Rating {
			return leaderboard[i].Rating > leaderboard[j].Rating
		}
		return leaderboard[i].DisplayName() < leaderboard[j].DisplayName()
	})
	return leaderboard, nil
}
