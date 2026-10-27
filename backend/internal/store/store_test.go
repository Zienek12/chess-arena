package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"chess-arena/arena/internal/database"
	"chess-arena/arena/internal/database/dbtest"
	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/tournament"
	"chess-arena/arena/internal/uci"
)

const lease = time.Minute

const testPositions = rules.StartingFEN + "\n7k/8/8/8/8/8/4K3/R7 b - - 0 1\n"

type fixture struct {
	store   *Store
	engineA Engine
	engineB Engine
	set     PositionSetSummary
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	engineA, err := store.CreateEngine(ctx, NewEngine{Name: "engine", Version: "v1", Image: "engine:v1"})
	if err != nil {
		t.Fatal(err)
	}
	engineB, err := store.CreateEngine(ctx, NewEngine{Name: "engine", Version: "v2", Image: "engine:v2",
		UCIOptions: map[string]string{"Hash": "16"}})
	if err != nil {
		t.Fatal(err)
	}
	positions, err := tournament.ImportFEN(testPositions)
	if err != nil {
		t.Fatal(err)
	}
	set, created, err := store.SavePositionSet(ctx, "test", positions)
	if err != nil || !created {
		t.Fatalf("save position set: created=%v err=%v", created, err)
	}
	return fixture{store: store, engineA: engineA, engineB: engineB, set: set}
}

func (f fixture) createTournament(t *testing.T, games int) Tournament {
	t.Helper()
	movetime := 100
	created, err := f.store.CreateTournament(context.Background(), tournament.Config{
		Name:          "test",
		EngineAID:     f.engineA.ID,
		EngineBID:     f.engineB.ID,
		PositionSetID: f.set.ID,
		GameCount:     games,
		Limit:         tournament.SearchLimit{MoveTimeMs: &movetime},
	})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.store.CreateEngine(ctx, NewEngine{Name: "engine", Version: "v1", Image: "x"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	positions, _ := tournament.ImportFEN(testPositions)
	again, created, err := f.store.SavePositionSet(ctx, "duplicate", positions)
	if err != nil || created || again.ID != f.set.ID {
		t.Fatalf("expected existing set, got %+v created=%v err=%v", again, created, err)
	}
	engines, err := f.store.ListEngines(ctx)
	if err != nil || len(engines) != 2 || engines[1].UCIOptions["Hash"] != "16" {
		t.Fatalf("unexpected engines %+v, %v", engines, err)
	}
}

func TestCreateTournamentSchedulesGames(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.createTournament(t, 4)
	if created.Status != tournament.StatusQueued || *created.Limit.MoveTimeMs != 100 || created.CPUs != 1 {
		t.Fatalf("unexpected tournament %+v", created)
	}
	games, err := f.store.ListGames(ctx, created.ID)
	if err != nil || len(games) != 4 {
		t.Fatalf("games: %+v, %v", games, err)
	}
	if games[0].WhiteEngineID != f.engineA.ID || games[1].WhiteEngineID != f.engineB.ID {
		t.Fatalf("colors do not alternate: %+v", games[:2])
	}

	_, err = f.store.CreateTournament(ctx, tournament.Config{
		Name: "bad", EngineAID: f.engineA.ID, EngineBID: "00000000-0000-0000-0000-000000000000",
		PositionSetID: f.set.ID, GameCount: 1, Limit: created.Limit,
	})
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("expected invalid reference, got %v", err)
	}
}

func TestGameLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.createTournament(t, 1)

	game, err := f.store.ClaimGame(ctx, "worker-1", lease)
	if err != nil || game == nil {
		t.Fatalf("claim: %+v, %v", game, err)
	}
	if game.White.ID != f.engineA.ID || game.StartFEN != rules.StartingFEN || game.Attempt != 1 ||
		*game.Limit.MoveTimeMs != 100 || game.MemoryMB != 256 {
		t.Fatalf("unexpected claimed game %+v", game)
	}
	if other, err := f.store.ClaimGame(ctx, "worker-2", lease); err != nil || other != nil {
		t.Fatalf("a leased game must not be claimed twice: %+v, %v", other, err)
	}

	score := 25
	move := MoveRecord{
		GameID: game.ID, TournamentID: game.TournamentID, GameNumber: game.GameNumber,
		Ply: 1, UCI: "e2e4", FENAfter: "after", NextSide: "b", EngineID: game.White.ID,
		Info:      uci.Info{Depth: 3, ScoreCP: &score, Nodes: 5_000_000_000, PV: []string{"e2e4", "e7e5"}},
		ThinkTime: 120 * time.Millisecond,
	}
	if err := f.store.RecordMove(ctx, "worker-1", lease, move); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RecordMove(ctx, "worker-1", lease, move); err != nil {
		t.Fatalf("repeating a move must be a no-op: %v", err)
	}
	move.UCI = "d2d4"
	if err := f.store.RecordMove(ctx, "worker-1", lease, move); !errors.Is(err, ErrConflictingMove) {
		t.Fatalf("expected conflicting move, got %v", err)
	}
	if err := f.store.RecordMove(ctx, "worker-2", lease, move); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("expected lease lost, got %v", err)
	}

	if err := f.store.FinishGame(ctx, "worker-1", GameResult{
		GameID: game.ID, TournamentID: game.TournamentID, Result: rules.WhiteWins,
		Termination: "time_forfeit", WinnerEngineID: game.White.ID, PGN: "1. e4 1-0",
	}); err != nil {
		t.Fatal(err)
	}

	detail, err := f.store.GetTournament(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != tournament.StatusCompleted || detail.Progress.Completed != 1 ||
		detail.Score.EngineAWins != 1 || detail.Score.EngineAPoints != 1 {
		t.Fatalf("unexpected tournament detail %+v", detail)
	}
	gameDetail, err := f.store.GetGame(ctx, game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gameDetail.PGN != "1. e4 1-0" || gameDetail.Termination != "time_forfeit" || len(gameDetail.Moves) != 1 {
		t.Fatalf("unexpected game detail %+v", gameDetail)
	}
	telemetry := gameDetail.Moves[0]
	if telemetry.Info.Nodes != 5_000_000_000 || *telemetry.Info.ScoreCP != 25 || len(telemetry.Info.PV) != 2 ||
		*telemetry.ThinkTimeMs != 120 {
		t.Fatalf("telemetry not stored: %+v", telemetry)
	}
}

func TestExpiredLeaseIsResumedWithPlayedMoves(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.createTournament(t, 1)

	game, _ := f.store.ClaimGame(ctx, "crashed-worker", time.Millisecond)
	if err := f.store.RecordMove(ctx, "crashed-worker", time.Millisecond, MoveRecord{
		GameID: game.ID, TournamentID: game.TournamentID, GameNumber: game.GameNumber,
		Ply: 1, UCI: "e2e4", FENAfter: "after", NextSide: "b", EngineID: game.White.ID,
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	resumed, err := f.store.ClaimGame(ctx, "worker-2", lease)
	if err != nil || resumed == nil {
		t.Fatalf("expected expired game to be claimed: %v", err)
	}
	if resumed.ID != game.ID || resumed.Attempt != 2 || len(resumed.Moves) != 1 || resumed.Moves[0] != "e2e4" {
		t.Fatalf("unexpected resumed game %+v", resumed)
	}
}

func TestReleaseAndCancel(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.createTournament(t, 2)

	game, _ := f.store.ClaimGame(ctx, "worker-1", lease)
	failure := GameFailure{Stage: "start", Code: "engine_start", Message: "boom", Retryable: true}
	if err := f.store.ReleaseGame(ctx, "worker-1", game, failure, true); err != nil {
		t.Fatal(err)
	}
	again, _ := f.store.ClaimGame(ctx, "worker-1", lease)
	if again == nil || again.ID != game.ID || again.Attempt != 2 {
		t.Fatalf("requeued game should be claimed again: %+v", again)
	}

	if err := f.store.CancelTournament(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RecordMove(ctx, "worker-1", lease, MoveRecord{GameID: again.ID, Ply: 1, UCI: "e2e4"}); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("cancelled game must lose its lease, got %v", err)
	}
	if err := f.store.CancelTournament(ctx, created.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict for finished tournament, got %v", err)
	}
	if _, err := f.store.GetTournament(ctx, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if next, _ := f.store.ClaimGame(ctx, "worker-1", lease); next != nil {
		t.Fatalf("cancelled tournament must not hand out games: %+v", next)
	}
}
