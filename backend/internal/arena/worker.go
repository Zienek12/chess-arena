package arena

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/sandbox"
	"chess-arena/arena/internal/store"
	"chess-arena/arena/internal/tournament"
	"chess-arena/arena/internal/uci"
)

// Store is the persistence the worker needs.
type Store interface {
	ClaimGame(ctx context.Context, owner string, lease time.Duration) (*store.ClaimedGame, error)
	RecordMove(ctx context.Context, owner string, lease time.Duration, move store.MoveRecord) error
	FinishGame(ctx context.Context, owner string, result store.GameResult) error
	ReleaseGame(ctx context.Context, owner string, game *store.ClaimedGame, failure store.GameFailure, requeue bool) error
}

// Config tunes a worker. Zero values are replaced by defaults.
type Config struct {
	ID            string        // lease owner; must be unique per worker
	PollInterval  time.Duration // wait between claims when the queue is empty
	Lease         time.Duration // renewed after every move
	MoveMargin    time.Duration // allowed overhead on top of movetime
	SearchTimeout time.Duration // per-move limit for depth and node searches
	StartTimeout  time.Duration // container start and handshake limit
	MaxAttempts   int           // claims before a game is marked failed
}

func (config Config) withDefaults() Config {
	if config.PollInterval == 0 {
		config.PollInterval = 2 * time.Second
	}
	if config.Lease == 0 {
		config.Lease = 5 * time.Minute
	}
	if config.MoveMargin == 0 {
		config.MoveMargin = time.Second
	}
	if config.SearchTimeout == 0 {
		config.SearchTimeout = time.Minute
	}
	if config.StartTimeout == 0 {
		// Includes container creation, which takes several seconds on Docker Desktop.
		config.StartTimeout = 2 * time.Minute
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}
	return config
}

// Worker claims queued games and plays them one at a time.
type Worker struct {
	config   Config
	store    Store
	launcher sandbox.Launcher
	logger   *slog.Logger
}

// NewWorker returns a worker. Run several workers to play games in parallel.
func NewWorker(config Config, store Store, launcher sandbox.Launcher, logger *slog.Logger) *Worker {
	return &Worker{config: config.withDefaults(), store: store, launcher: launcher, logger: logger.With("worker", config.ID)}
}

// Run plays games until ctx is cancelled.
func (worker *Worker) Run(ctx context.Context) {
	for ctx.Err() == nil {
		played, err := worker.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			worker.logger.Error("game failed", "error", err)
		}
		if played && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(worker.config.PollInterval):
		}
	}
}

// stageError tells where a game failed, for game_errors.
type stageError struct {
	stage string
	code  string
	err   error
}

func (err *stageError) Error() string { return err.stage + ": " + err.err.Error() }
func (err *stageError) Unwrap() error { return err.err }

// RunOnce claims and plays a single game. It returns false when the queue is empty.
func (worker *Worker) RunOnce(ctx context.Context) (bool, error) {
	game, err := worker.store.ClaimGame(ctx, worker.config.ID, worker.config.Lease)
	if err != nil || game == nil {
		return false, err
	}
	logger := worker.logger.With("tournament", game.TournamentID, "game", game.GameNumber, "attempt", game.Attempt)

	if game.Attempt > worker.config.MaxAttempts {
		failure := store.GameFailure{Stage: "claim", Code: "max_attempts",
			Message: fmt.Sprintf("game failed %d times", game.Attempt-1)}
		return true, worker.store.ReleaseGame(ctx, worker.config.ID, game, failure, false)
	}

	logger.Info("playing game", "white", game.White.DisplayName(), "black", game.Black.DisplayName(), "resumed_moves", len(game.Moves))
	err = worker.play(ctx, game, logger)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, store.ErrLeaseLost):
		logger.Warn("game was taken over or cancelled")
		return true, nil
	}

	// Release with a fresh context: on shutdown ctx is already cancelled.
	releaseCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	failure := store.GameFailure{Stage: "play", Code: "internal", Message: err.Error(), Retryable: true}
	var staged *stageError
	if errors.As(err, &staged) {
		failure.Stage, failure.Code = staged.stage, staged.code
	}
	if ctx.Err() != nil {
		failure.Code = "shutdown"
	}
	requeue := ctx.Err() != nil || game.Attempt < worker.config.MaxAttempts
	if releaseErr := worker.store.ReleaseGame(releaseCtx, worker.config.ID, game, failure, requeue); releaseErr != nil {
		return true, errors.Join(err, releaseErr)
	}
	return true, err
}

func (worker *Worker) play(ctx context.Context, game *store.ClaimedGame, logger *slog.Logger) error {
	white, err := worker.startEngine(ctx, game, game.White)
	if err != nil {
		return &stageError{stage: "start", code: "white_engine", err: err}
	}
	defer white.Close()
	black, err := worker.startEngine(ctx, game, game.Black)
	if err != nil {
		return &stageError{stage: "start", code: "black_engine", err: err}
	}
	defer black.Close()

	limit, timeout := worker.searchLimit(game.Limit)
	match := Match{
		White:       white,
		Black:       black,
		StartFEN:    game.StartFEN,
		Played:      game.Moves,
		Limit:       limit,
		MoveTimeout: timeout,
		OnMove: func(ctx context.Context, move PlayedMove) error {
			engineID := game.White.ID
			if move.Side == rules.Black {
				engineID = game.Black.ID
			}
			return worker.store.RecordMove(ctx, worker.config.ID, worker.config.Lease, store.MoveRecord{
				GameID:       game.ID,
				TournamentID: game.TournamentID,
				GameNumber:   game.GameNumber,
				Ply:          move.Ply,
				UCI:          move.UCI,
				FENAfter:     move.FENAfter,
				NextSide:     string(move.NextSide),
				EngineID:     engineID,
				Info:         move.Search.Info,
				ThinkTime:    move.Search.Elapsed,
			})
		},
	}
	outcome, err := match.Play(ctx)
	if err != nil {
		if errors.Is(err, store.ErrLeaseLost) || ctx.Err() != nil {
			return err
		}
		return &stageError{stage: "play", code: "match", err: err}
	}

	result := outcome.Game.Result()
	winner := ""
	switch result {
	case rules.WhiteWins:
		winner = game.White.ID
	case rules.BlackWins:
		winner = game.Black.ID
	}
	termination := outcome.Game.Termination()
	pgn := outcome.Game.PGN(map[string]string{
		"Event":       game.TournamentName,
		"Site":        "Chess Arena",
		"Round":       strconv.Itoa(game.GameNumber),
		"White":       game.White.DisplayName(),
		"Black":       game.Black.DisplayName(),
		"Termination": termination,
	})
	logger.Info("game finished", "result", result, "termination", termination, "detail", outcome.Detail)
	return worker.store.FinishGame(ctx, worker.config.ID, store.GameResult{
		GameID:         game.ID,
		TournamentID:   game.TournamentID,
		Result:         result,
		Termination:    termination,
		WinnerEngineID: winner,
		PGN:            pgn,
	})
}

// searchLimit converts the tournament limit to a UCI limit and the deadline the
// arena enforces for every move.
func (worker *Worker) searchLimit(limit tournament.SearchLimit) (uci.Limit, time.Duration) {
	switch {
	case limit.MoveTimeMs != nil:
		movetime := time.Duration(*limit.MoveTimeMs) * time.Millisecond
		return uci.Limit{MoveTime: movetime}, movetime + worker.config.MoveMargin
	case limit.Depth != nil:
		return uci.Limit{Depth: *limit.Depth}, worker.config.SearchTimeout
	case limit.Nodes != nil:
		return uci.Limit{Nodes: *limit.Nodes}, worker.config.SearchTimeout
	}
	return uci.Limit{}, worker.config.SearchTimeout
}

func (worker *Worker) startEngine(ctx context.Context, game *store.ClaimedGame, engine store.Engine) (*uci.Client, error) {
	process, err := worker.launcher.Start(ctx, sandbox.Spec{Image: engine.Image, CPUs: game.CPUs, MemoryMB: game.MemoryMB})
	if err != nil {
		return nil, err
	}
	client := uci.NewClient(process.Stdin, process.Stdout, process.Stop)
	if _, err := client.Handshake(ctx, engine.UCIOptions, worker.config.StartTimeout); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("%s: %w", engine.DisplayName(), err)
	}
	if err := client.NewGame(ctx, worker.config.StartTimeout); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("%s: %w", engine.DisplayName(), err)
	}
	return client, nil
}
