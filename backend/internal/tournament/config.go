// Package tournament holds the arena's domain model: tournament configuration,
// imported starting positions, and the schedule of games.
package tournament

import (
	"errors"
	"fmt"
	"strings"
)

// ColorOrder decides which engine plays white in each game.
type ColorOrder string

const (
	ColorAlternate    ColorOrder = "alternate"
	ColorEngineAWhite ColorOrder = "engine_a_white"
	ColorEngineBWhite ColorOrder = "engine_b_white"
)

// Status is the tournament lifecycle state.
type Status string

const (
	StatusCreated   Status = "created"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Defaults applied by Normalize.
const (
	DefaultGameCount = 1000
	DefaultCPUs      = 1.0
	DefaultMemoryMB  = 256
	MinMemoryMB      = 16
)

// SearchLimit is how long an engine may think per move. Exactly one field must be set.
type SearchLimit struct {
	MoveTimeMs *int   `json:"movetime_ms,omitempty"`
	Depth      *int   `json:"depth,omitempty"`
	Nodes      *int64 `json:"nodes,omitempty"`
}

// Config is the user-provided definition of a tournament.
type Config struct {
	Name          string      `json:"name"`
	EngineAID     string      `json:"engine_a_id"`
	EngineBID     string      `json:"engine_b_id"`
	PositionSetID string      `json:"position_set_id"`
	GameCount     int         `json:"game_count"`
	Limit         SearchLimit `json:"limit"`
	ColorOrder    ColorOrder  `json:"color_order"`
	CPUs          float64     `json:"cpus"`
	MemoryMB      int         `json:"memory_mb"`
}

// Normalize fills in defaults for omitted fields.
func (config Config) Normalize() Config {
	config.Name = strings.TrimSpace(config.Name)
	if config.GameCount == 0 {
		config.GameCount = DefaultGameCount
	}
	if config.ColorOrder == "" {
		config.ColorOrder = ColorAlternate
	}
	if config.CPUs == 0 {
		config.CPUs = DefaultCPUs
	}
	if config.MemoryMB == 0 {
		config.MemoryMB = DefaultMemoryMB
	}
	return config
}

// Validate checks a normalized configuration.
func (config Config) Validate() error {
	if config.Name == "" {
		return errors.New("tournament name is required")
	}
	if config.EngineAID == "" || config.EngineBID == "" || config.EngineAID == config.EngineBID {
		return errors.New("tournament requires two different engines")
	}
	if config.PositionSetID == "" {
		return errors.New("position set is required")
	}
	if config.GameCount < 1 {
		return errors.New("game count must be positive")
	}
	if err := config.Limit.Validate(); err != nil {
		return err
	}
	switch config.ColorOrder {
	case ColorAlternate, ColorEngineAWhite, ColorEngineBWhite:
	default:
		return fmt.Errorf("invalid color order %q", config.ColorOrder)
	}
	if config.CPUs <= 0 {
		return errors.New("cpus must be positive")
	}
	if config.MemoryMB < MinMemoryMB {
		return fmt.Errorf("memory must be at least %d MB", MinMemoryMB)
	}
	return nil
}

// Validate checks that exactly one positive limit is set.
func (limit SearchLimit) Validate() error {
	set := 0
	if limit.MoveTimeMs != nil {
		set++
		if *limit.MoveTimeMs < 1 {
			return errors.New("movetime must be positive")
		}
	}
	if limit.Depth != nil {
		set++
		if *limit.Depth < 1 {
			return errors.New("depth must be positive")
		}
	}
	if limit.Nodes != nil {
		set++
		if *limit.Nodes < 1 {
			return errors.New("nodes must be positive")
		}
	}
	if set != 1 {
		return errors.New("exactly one of movetime_ms, depth or nodes must be configured")
	}
	return nil
}
