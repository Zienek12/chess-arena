package analysis

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
	"chess-arena/arena/internal/uci"
)

// Store is the persistence the worker needs.
type Store interface {
	ClaimAnalysis(ctx context.Context, owner string, lease time.Duration) (*store.AnalysisJob, error)
	SaveAnalysis(ctx context.Context, owner string, job *store.AnalysisJob, engine store.EngineBuild, nodes int64, moves []store.MoveAnalysis) error
	FailAnalysis(ctx context.Context, owner string, job *store.AnalysisJob, message string, retry bool, retryAfter time.Duration) error
	ReleaseAnalysis(ctx context.Context, owner string, job *store.AnalysisJob, message string, retryAfter time.Duration) error
}

// ErrEngineUnavailable means the analysis engine could not be started; the
// game is returned to the queue without counting the attempt.
var ErrEngineUnavailable = errors.New("analysis engine unavailable")

// Config tunes a worker. Zero values are replaced by defaults.
type Config struct {
	ID            string // lease owner; must be unique per worker
	Image         string // Docker image of the analysis engine
	Nodes         int64  // search budget per position
	HashMB        int
	MemoryMB      int
	PollInterval  time.Duration
	Lease         time.Duration
	StartTimeout  time.Duration
	SearchTimeout time.Duration // per position
	RetryAfter    time.Duration
	MaxAttempts   int
}

func (config Config) withDefaults() Config {
	if config.Nodes == 0 {
		config.Nodes = 200_000
	}
	if config.HashMB == 0 {
		config.HashMB = 64
	}
	if config.MemoryMB == 0 {
		config.MemoryMB = 256
	}
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.Lease == 0 {
		config.Lease = 30 * time.Minute
	}
	if config.StartTimeout == 0 {
		config.StartTimeout = 2 * time.Minute
	}
	if config.SearchTimeout == 0 {
		config.SearchTimeout = time.Minute
	}
	if config.RetryAfter == 0 {
		config.RetryAfter = time.Minute
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}
	return config
}

// Worker analyzes finished games one at a time. It keeps the engine running
// while there is work and stops it when the queue is empty.
type Worker struct {
	config   Config
	store    Store
	launcher sandbox.Launcher
	logger   *slog.Logger
	engine   *uci.Client
	build    store.EngineBuild
}

// NewWorker returns an analysis worker.
func NewWorker(config Config, store Store, launcher sandbox.Launcher, logger *slog.Logger) *Worker {
	return &Worker{config: config.withDefaults(), store: store, launcher: launcher,
		logger: logger.With("analysis_worker", config.ID)}
}

// Run analyzes games until ctx is cancelled.
func (worker *Worker) Run(ctx context.Context) {
	defer worker.closeEngine()
	for ctx.Err() == nil {
		analyzed, err := worker.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			worker.logger.Error("analysis failed", "error", err)
		}
		if analyzed && err == nil {
			continue
		}
		worker.closeEngine()
		wait := worker.config.PollInterval
		if errors.Is(err, ErrEngineUnavailable) {
			wait = worker.config.RetryAfter
		}
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
}

// RunOnce claims and analyzes one game. It returns false when there is none.
func (worker *Worker) RunOnce(ctx context.Context) (bool, error) {
	job, err := worker.store.ClaimAnalysis(ctx, worker.config.ID, worker.config.Lease)
	if err != nil || job == nil {
		return false, err
	}
	logger := worker.logger.With("game", job.GameID, "attempt", job.Attempt)
	if job.Attempt > worker.config.MaxAttempts {
		return true, worker.store.FailAnalysis(ctx, worker.config.ID, job,
			fmt.Sprintf("analysis failed %d times", job.Attempt-1), false, 0)
	}

	engine, err := worker.ensureEngine(ctx)
	if err != nil {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		releaseErr := worker.store.ReleaseAnalysis(releaseCtx, worker.config.ID, job, err.Error(), worker.config.RetryAfter)
		return true, errors.Join(fmt.Errorf("%w: %w", ErrEngineUnavailable, err), releaseErr)
	}

	started := time.Now()
	moves, err := worker.analyze(ctx, engine, job)
	if err == nil {
		err = worker.store.SaveAnalysis(ctx, worker.config.ID, job, worker.build, worker.config.Nodes, moves)
		if err == nil {
			logger.Info("game analyzed", "moves", len(moves), "elapsed", time.Since(started).Round(time.Second))
		}
		return true, err
	}
	if errors.Is(err, store.ErrLeaseLost) {
		return true, nil
	}
	// The engine may be in a bad state; start a fresh one for the next game.
	worker.closeEngine()
	failCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	retry := ctx.Err() != nil || job.Attempt < worker.config.MaxAttempts
	if failErr := worker.store.FailAnalysis(failCtx, worker.config.ID, job, err.Error(), retry, worker.config.RetryAfter); failErr != nil {
		return true, errors.Join(err, failErr)
	}
	return true, err
}

// analyze evaluates every position of the game and judges its moves.
func (worker *Worker) analyze(ctx context.Context, engine *uci.Client, job *store.AnalysisJob) ([]store.MoveAnalysis, error) {
	game, err := rules.NewGame(job.StartFEN)
	if err != nil {
		return nil, fmt.Errorf("start position: %w", err)
	}
	whiteStarts := game.Turn() == rules.White
	for _, move := range job.Moves {
		if err := game.Play(move); err != nil {
			return nil, fmt.Errorf("replay %s: %w", move, err)
		}
	}
	if err := engine.NewGame(ctx, worker.config.StartTimeout); err != nil {
		return nil, err
	}

	positions := make([]PositionEval, 0, len(job.Moves)+1)
	for played := 0; played <= len(job.Moves); played++ {
		// A game that ended on the board needs no search in its final position.
		if played == len(job.Moves) && game.Over() && game.Termination() != "" {
			if game.Termination() == rules.TerminationCheckmate {
				positions = append(positions, Checkmated())
			} else {
				positions = append(positions, PositionEval{})
			}
			break
		}
		result, err := engine.Search(ctx, uci.Position{FEN: job.StartFEN, Moves: job.Moves[:played]},
			uci.Limit{Nodes: worker.config.Nodes}, worker.config.SearchTimeout)
		if err != nil {
			return nil, fmt.Errorf("position after %d moves: %w", played, err)
		}
		positions = append(positions, evalFromSearch(result))
	}
	return Moves(positions, job.Moves, whiteStarts), nil
}

func evalFromSearch(result uci.SearchResult) PositionEval {
	eval := PositionEval{Mate: result.Info.ScoreMate, PV: result.Info.PV, Depth: result.Info.Depth}
	if result.Info.ScoreCP != nil {
		eval.CP = *result.Info.ScoreCP
	}
	if result.BestMove != "(none)" && result.BestMove != "0000" {
		eval.BestMove = result.BestMove
	}
	return eval
}

func (worker *Worker) ensureEngine(ctx context.Context) (*uci.Client, error) {
	if worker.engine != nil {
		return worker.engine, nil
	}
	process, err := worker.launcher.Start(ctx, sandbox.Spec{Image: worker.config.Image, CPUs: 1, MemoryMB: worker.config.MemoryMB})
	if err != nil {
		return nil, fmt.Errorf("start analysis engine: %w", err)
	}
	client := uci.NewClient(process.Stdin, process.Stdout, process.Stop)
	identity, err := client.Handshake(ctx, map[string]string{
		"Threads": "1",
		"Hash":    strconv.Itoa(worker.config.HashMB),
	}, worker.config.StartTimeout)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("analysis engine handshake: %w", err)
	}
	worker.engine = client
	worker.build = store.EngineBuild{ImageID: process.ImageID, UCIName: identity.Name}
	worker.logger.Info("analysis engine started", "engine", identity.Name)
	return client, nil
}

func (worker *Worker) closeEngine() {
	if worker.engine != nil {
		_ = worker.engine.Close()
		worker.engine = nil
	}
}
