package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Engine is a registered engine version and the Docker image that runs it.
type Engine struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Image      string            `json:"image"`
	UCIOptions map[string]string `json:"uci_options"`
	CreatedAt  time.Time         `json:"created_at"`
}

// NewEngine is the input for CreateEngine.
type NewEngine struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Image      string            `json:"image"`
	UCIOptions map[string]string `json:"uci_options"`
}

// Validate checks required fields.
func (engine NewEngine) Validate() error {
	if strings.TrimSpace(engine.Name) == "" || strings.TrimSpace(engine.Version) == "" {
		return errors.New("engine name and version are required")
	}
	if strings.TrimSpace(engine.Image) == "" {
		return errors.New("engine image is required")
	}
	return nil
}

// DisplayName is "name version", used in PGN headers.
func (engine Engine) DisplayName() string {
	return engine.Name + " " + engine.Version
}

const engineColumns = "id, name, version, image, uci_options, created_at"

func scanEngine(row pgx.Row) (Engine, error) {
	var engine Engine
	err := row.Scan(&engine.ID, &engine.Name, &engine.Version, &engine.Image, &engine.UCIOptions, &engine.CreatedAt)
	if engine.UCIOptions == nil {
		engine.UCIOptions = map[string]string{}
	}
	return engine, err
}

// CreateEngine registers an engine. A duplicate name and version returns ErrConflict.
func (store *Store) CreateEngine(ctx context.Context, input NewEngine) (Engine, error) {
	if input.UCIOptions == nil {
		input.UCIOptions = map[string]string{}
	}
	engine, err := scanEngine(store.pool.QueryRow(ctx, `
		INSERT INTO engines (name, version, image, uci_options)
		VALUES ($1, $2, $3, $4)
		RETURNING `+engineColumns,
		strings.TrimSpace(input.Name), strings.TrimSpace(input.Version), strings.TrimSpace(input.Image), input.UCIOptions))
	if isUniqueViolation(err) {
		return Engine{}, fmt.Errorf("engine %s %s already exists: %w", input.Name, input.Version, ErrConflict)
	}
	if err != nil {
		return Engine{}, fmt.Errorf("insert engine: %w", err)
	}
	return engine, nil
}

// ListEngines returns all engines ordered by name and version.
func (store *Store) ListEngines(ctx context.Context) ([]Engine, error) {
	rows, err := store.pool.Query(ctx, "SELECT "+engineColumns+" FROM engines ORDER BY name, version")
	if err != nil {
		return nil, fmt.Errorf("list engines: %w", err)
	}
	defer rows.Close()
	engines := []Engine{}
	for rows.Next() {
		engine, err := scanEngine(rows)
		if err != nil {
			return nil, fmt.Errorf("scan engine: %w", err)
		}
		engines = append(engines, engine)
	}
	return engines, rows.Err()
}

func getEngine(ctx context.Context, query pgx.Tx, id string) (Engine, error) {
	engine, err := scanEngine(query.QueryRow(ctx, "SELECT "+engineColumns+" FROM engines WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Engine{}, ErrNotFound
	}
	return engine, err
}
