package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// EventsChannel is the PostgreSQL NOTIFY channel that announces changes, so
// every backend instance can push them to its clients.
const EventsChannel = "arena_events"

// Event types.
const (
	EventMove       = "move"       // a move was stored
	EventGame       = "game"       // a game changed status
	EventTournament = "tournament" // a tournament was created or changed status
	EventAnalysis   = "analysis"   // a game's analysis changed
)

// Event says what changed. Clients reload the affected resources.
type Event struct {
	Type         string `json:"type"`
	TournamentID string `json:"tournament_id,omitempty"`
	GameID       string `json:"game_id,omitempty"`
}

// notify queues event; PostgreSQL delivers it when tx commits.
func notify(ctx context.Context, tx pgx.Tx, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "SELECT pg_notify($1, $2)", EventsChannel, string(payload)); err != nil {
		return fmt.Errorf("notify %s: %w", event.Type, err)
	}
	return nil
}
