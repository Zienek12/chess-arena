package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"chess-arena/arena/internal/api"
	"chess-arena/arena/internal/arena"
	"chess-arena/arena/internal/config"
	"chess-arena/arena/internal/database"
	"chess-arena/arena/internal/play"
	"chess-arena/arena/internal/sandbox"
	"chess-arena/arena/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("backend stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	registry, err := play.LoadRegistry()
	if err != nil {
		return err
	}
	docker := sandbox.NewDockerLauncher()
	docker.Network = cfg.EngineNetwork

	mux := http.NewServeMux()
	mux.Handle("/ws", play.NewHandler(registry, docker, cfg.CORSOrigins, logger))

	var workers sync.WaitGroup
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL is not set; tournament API and arena workers are disabled")
	} else {
		pool, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		if err := database.Migrate(ctx, pool); err != nil {
			return err
		}
		logger.Info("database schema is up to date")

		repository := store.New(pool)
		api.NewServer(repository, logger).Register(mux)
		for index := 1; index <= cfg.Workers; index++ {
			worker := arena.NewWorker(arena.Config{
				ID:            fmt.Sprintf("%s-%d", cfg.WorkerID, index),
				MoveMargin:    cfg.MoveMargin,
				SearchTimeout: cfg.SearchTimeout,
			}, repository, docker, logger)
			workers.Add(1)
			go func() {
				defer workers.Done()
				worker.Run(ctx)
			}()
		}
		logger.Info("arena workers started", "count", cfg.Workers)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.WithCORS(mux, cfg.CORSOrigins),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	// Workers requeue their current game when ctx is cancelled.
	workers.Wait()
	return nil
}
