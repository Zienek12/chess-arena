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
}

// Load reads the configuration, applying defaults for unset variables.
func Load() (Config, error) {
	hostname, _ := os.Hostname()
	config := Config{
		HTTPAddr:      envString("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		CORSOrigins:   envList("CORS_ORIGINS", "http://localhost:3000"),
		WorkerID:      envString("ARENA_WORKER_ID", hostname),
		EngineNetwork: envString("ARENA_ENGINE_NETWORK", "none"),
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
