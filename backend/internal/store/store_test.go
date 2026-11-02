package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"chess-arena/arena/internal/database"
	"chess-arena/arena/internal/database/dbtest"
	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/stats"
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
		// Games start from the set's first position, which the tests rely on.
		PositionOrder: tournament.PositionsSequential,
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

	// Without a position order, positions are shuffled per tournament and stored as such.
	shuffled, err := f.store.CreateTournament(ctx, tournament.Config{
		Name: "shuffled", EngineAID: f.engineA.ID, EngineBID: f.engineB.ID,
		PositionSetID: f.set.ID, GameCount: 4, Limit: created.Limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := f.store.GetTournament(ctx, shuffled.ID)
	if err != nil || listed.PositionOrder != tournament.PositionsRandom {
		t.Fatalf("position order not stored: %+v, %v", listed.Tournament, err)
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
	if engine := detail.Telemetry[0]; engine.EngineID != f.engineA.ID || engine.Moves != 1 ||
		engine.MovesWithInfo != 1 || engine.MaxDepth != 3 || engine.TotalNodes != 5_000_000_000 ||
		engine.AverageThinkTime != 120 {
		t.Fatalf("unexpected engine A telemetry %+v", engine)
	}
	if engine := detail.Telemetry[1]; engine.EngineID != f.engineB.ID || engine.Moves != 0 {
		t.Fatalf("unexpected engine B telemetry %+v", engine)
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

func TestSPRTStopsTournamentOnceDecided(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	movetime := 100
	// These hypotheses are decided by the third win, not the second.
	created, err := f.store.CreateTournament(ctx, tournament.Config{
		Name: "sprt", EngineAID: f.engineA.ID, EngineBID: f.engineB.ID, PositionSetID: f.set.ID,
		GameCount: 20, Limit: tournament.SearchLimit{MoveTimeMs: &movetime},
		SPRT: &stats.SPRT{Elo0: 0, Elo1: 100, Alpha: 0.2, Beta: 0.2},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range []string{"win", "win", "win"} {
		game, err := f.store.ClaimGame(ctx, "worker-1", lease)
		if err != nil || game == nil {
			t.Fatalf("claim: %+v, %v", game, err)
		}
		result, winner := rules.Draw, ""
		if outcome == "win" {
			result, winner = rules.WhiteWins, game.White.ID
			if game.Black.ID == f.engineA.ID {
				result, winner = rules.BlackWins, game.Black.ID
			}
		}
		if err := f.store.FinishGame(ctx, "worker-1", GameResult{GameID: game.ID, TournamentID: game.TournamentID,
			Result: result, Termination: "checkmate", WinnerEngineID: winner}); err != nil {
			t.Fatal(err)
		}
	}

	detail, err := f.store.GetTournament(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != tournament.StatusCompleted || detail.SPRTResult != stats.AcceptH1 ||
		detail.SPRTStatus == nil || detail.SPRTStatus.Result != stats.AcceptH1 {
		t.Fatalf("SPRT did not stop the tournament: %+v %+v", detail.Tournament, detail.SPRTStatus)
	}
	if detail.Progress.Completed != 3 || detail.Progress.Cancelled != 17 || detail.Stats.Wins != 3 || detail.Stats.Draws != 0 {
		t.Fatalf("unexpected progress %+v and stats %+v", detail.Progress, detail.Stats)
	}
	if game, err := f.store.ClaimGame(ctx, "worker-1", lease); err != nil || game != nil {
		t.Fatalf("no game may be claimed after the SPRT decided: %+v, %v", game, err)
	}
}

func TestAnalysisAndLeaderboard(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	created := f.createTournament(t, 1)
	if job, err := f.store.ClaimAnalysis(ctx, "analysis-1", lease); err != nil || job != nil {
		t.Fatalf("unfinished games must not be analyzed: %+v, %v", job, err)
	}

	game, err := f.store.ClaimGame(ctx, "worker-1", lease)
	if err != nil || game == nil {
		t.Fatalf("claim: %+v, %v", game, err)
	}
	for ply, uci := range []string{"e2e4", "e7e5"} {
		engineID := game.White.ID
		if ply == 1 {
			engineID = game.Black.ID
		}
		if err := f.store.RecordMove(ctx, "worker-1", lease, MoveRecord{GameID: game.ID, TournamentID: game.TournamentID,
			GameNumber: game.GameNumber, Ply: ply + 1, UCI: uci, FENAfter: "fen", NextSide: "w", EngineID: engineID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.store.FinishGame(ctx, "worker-1", GameResult{GameID: game.ID, TournamentID: game.TournamentID,
		Result: rules.WhiteWins, Termination: "time_forfeit", WinnerEngineID: game.White.ID}); err != nil {
		t.Fatal(err)
	}

	job, err := f.store.ClaimAnalysis(ctx, "analysis-1", lease)
	if err != nil || job == nil || job.GameID != game.ID || len(job.Moves) != 2 || job.StartFEN != rules.StartingFEN {
		t.Fatalf("claim analysis: %+v, %v", job, err)
	}
	if again, err := f.store.ClaimAnalysis(ctx, "analysis-2", lease); err != nil || again != nil {
		t.Fatalf("a leased analysis must not be claimed twice: %+v, %v", again, err)
	}
	mate := 3
	moves := []MoveAnalysis{
		{Ply: 1, BestMove: "e2e4", BestPV: []string{"e2e4", "e7e5"}, Depth: 18, EvalBefore: Eval{CP: 30}, EvalAfter: Eval{CP: 25},
			CPLoss: 5, Accuracy: 99, Classification: "best"},
		{Ply: 2, BestMove: "c7c5", Depth: 18, EvalBefore: Eval{CP: 25}, EvalAfter: Eval{CP: MateCP - 3, Mate: &mate},
			CPLoss: 1000, Accuracy: 1, Classification: "blunder"},
	}
	if err := f.store.SaveAnalysis(ctx, "analysis-2", job, "Stockfish", 1000, moves); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("only the lease owner may save: %v", err)
	}
	if err := f.store.SaveAnalysis(ctx, "analysis-1", job, "Stockfish 17.1", 1000, moves); err != nil {
		t.Fatal(err)
	}

	detail, err := f.store.GetGame(ctx, game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Analysis == nil || detail.Analysis.Status != "completed" || detail.Analysis.Engine != "Stockfish 17.1" {
		t.Fatalf("unexpected game analysis %+v", detail.Analysis)
	}
	second := detail.Moves[1].Analysis
	if second == nil || second.Classification != "blunder" || *second.EvalAfter.Mate != 3 || len(detail.Moves[0].Analysis.BestPV) != 2 {
		t.Fatalf("unexpected move analysis %+v", second)
	}

	tournamentDetail, err := f.store.GetTournament(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if black := tournamentDetail.Telemetry[1]; black.AnalyzedMoves != 1 || black.Blunders != 1 || black.AverageCPLoss != 1000 {
		t.Fatalf("unexpected move quality %+v", black.MoveQuality)
	}

	leaderboard, err := f.store.Leaderboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(leaderboard) != 2 || leaderboard[0].ID != game.White.ID || leaderboard[0].Wins != 1 ||
		leaderboard[0].Rating <= 0 || leaderboard[1].Losses != 1 || leaderboard[0].Accuracy != 99 {
		t.Fatalf("unexpected leaderboard %+v", leaderboard)
	}
}

func TestPositionNamesAreStoredAndFilledIn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	unnamed, err := tournament.ImportPGN("[Event \"?\"]\n\n1. e4 e5 *\n\n[Event \"?\"]\n\n1. d4 d5 *\n")
	if err != nil {
		t.Fatal(err)
	}
	set, created, err := f.store.SavePositionSet(ctx, "openings", unnamed)
	if err != nil || !created {
		t.Fatalf("save: %v %v", created, err)
	}
	named, err := tournament.ImportPGN("[Event \"Open Game\"]\n\n1. e4 e5 *\n\n[Event \"Closed Game\"]\n\n1. d4 d5 *\n")
	if err != nil {
		t.Fatal(err)
	}
	again, created, err := f.store.SavePositionSet(ctx, "openings", named)
	if err != nil || created || again.ID != set.ID {
		t.Fatalf("re-import must return the existing set: %+v %v %v", again, created, err)
	}

	movetime := 100
	created2, err := f.store.CreateTournament(ctx, tournament.Config{
		Name: "named", EngineAID: f.engineA.ID, EngineBID: f.engineB.ID, PositionSetID: set.ID,
		GameCount: 4, Limit: tournament.SearchLimit{MoveTimeMs: &movetime}, PositionOrder: tournament.PositionsSequential,
	})
	if err != nil {
		t.Fatal(err)
	}
	games, err := f.store.ListGames(ctx, created2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if games[0].Opening != "Open Game" || games[0].PositionNumber != 1 || games[2].Opening != "Closed Game" || games[2].PositionNumber != 2 {
		t.Fatalf("unexpected openings %+v", games)
	}
	detail, err := f.store.GetGame(ctx, games[3].ID)
	if err != nil || detail.Opening != "Closed Game" {
		t.Fatalf("game detail opening: %q %v", detail.Opening, err)
	}
}
