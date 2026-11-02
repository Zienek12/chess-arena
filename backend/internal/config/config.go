// Package config reads backend settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the backend settings.
type Config struct {
	HTTPAddr      string        // HTTP_ADDR
	DatabaseURL   string        // DATABASE_URL; the arena is disabled without it
	CORSOrigins   []string      // CORS_ORIGINS, comma separated
	Workers       int           // ARENA_WORKERS; games played in parallel, 0 disables
	WorkerID      string        // ARENA_WORKER_ID; prefix of lease owners
	MoveMargin    time.Duration // ARENA_MOVE_MARGIN_MS; tolerance on top of movetime
	SearchTimeout time.Duration // ARENA_SEARCH_TIMEOUT_MS; per move for depth and node limits
	EngineNetwork string        // ARENA_ENGINE_NETWORK; Docker network for engines
	APIToken      string        // ARENA_API_TOKEN; required for writes when set
	AllowedImages []string      // ARENA_ALLOWED_IMAGES; image repositories engines may use

	AnalysisWorkers int    // ANALYSIS_WORKERS; games analyzed in parallel, 0 disables
	AnalysisImage   string // ANALYSIS_ENGINE_IMAGE; Docker image of the analysis engine
	AnalysisNodes   int64  // ANALYSIS_NODES; search budget per analyzed position
}

// DefaultAllowedImages are this project's engine and Stockfish images.
const DefaultAllowedImages = "zienek/chess-arena-engine,chess-arena-engine,chess-arena-stockfish"

// Load reads the configuration, applying defaults for unset variables.
func Load() (Config, error) {
	hostname, _ := os.Hostname()
	config := Config{
		HTTPAddr:      envString("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		CORSOrigins:   envList("CORS_ORIGINS", "http://localhost:3000"),
		WorkerID:      envString("ARENA_WORKER_ID", hostname),
		EngineNetwork: envString("ARENA_ENGINE_NETWORK", "none"),
		APIToken:      strings.TrimSpace(os.Getenv("ARENA_API_TOKEN")),
		AllowedImages: envList("ARENA_ALLOWED_IMAGES", DefaultAllowedImages),
		AnalysisImage: envString("ANALYSIS_ENGINE_IMAGE", "chess-arena-stockfish:local"),
	}
	var err error
	if config.Workers, err = envInt("ARENA_WORKERS", 1); err != nil {
		return config, err
	}
	margin, err := envInt("ARENA_MOVE_MARGIN_MS", 1000)
	if err != nil {
		return config, err
	}
	config.MoveMargin = time.Duration(margin) * time.Millisecond
	timeout, err := envInt("ARENA_SEARCH_TIMEOUT_MS", 60_000)
	if err != nil {
		return config, err
	}
	config.SearchTimeout = time.Duration(timeout) * time.Millisecond
	if config.Workers < 0 || margin < 0 || timeout <= 0 {
		return config, fmt.Errorf("ARENA_WORKERS and ARENA_MOVE_MARGIN_MS must not be negative, ARENA_SEARCH_TIMEOUT_MS must be positive")
	}
	if config.AnalysisWorkers, err = envInt("ANALYSIS_WORKERS", 1); err != nil {
		return config, err
	}
	nodes, err := envInt("ANALYSIS_NODES", 200_000)
	if err != nil {
		return config, err
	}
	config.AnalysisNodes = int64(nodes)
	if config.AnalysisWorkers < 0 || config.AnalysisNodes <= 0 {
		return config, fmt.Errorf("ANALYSIS_WORKERS must not be negative and ANALYSIS_NODES must be positive")
	}
	return config, nil
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envList(name, fallback string) []string {
	values := []string{}
	for _, value := range strings.Split(envString(name, fallback), ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func envInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return parsed, nil
}
