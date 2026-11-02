package analysis

import (
	"math"
	"testing"

	"chess-arena/arena/internal/store"
)

func mate(moves int) *int { return &moves }

func TestWinPercentAndAccuracy(t *testing.T) {
	if WinPercent(0) != 50 || WinPercent(store.MateCP) < 99.9 || WinPercent(-store.MateCP) > 0.1 {
		t.Fatal("unexpected winning chances")
	}
	if MoveAccuracy(50, 60) != 100 {
		t.Fatal("a move that improves the position is perfectly accurate")
	}
	if got := MoveAccuracy(50, 50); math.Abs(got-100) > 0.01 {
		t.Fatalf("a move that keeps the evaluation is about 100%% accurate, got %v", got)
	}
	if got := MoveAccuracy(90, 10); got > 2 {
		t.Fatalf("throwing away a won position scores about 1%%, got %v", got)
	}
}

func TestMovesFromStartingPosition(t *testing.T) {
	positions := []PositionEval{
		{CP: 30, BestMove: "e2e4", PV: []string{"e2e4", "e7e5"}, Depth: 18}, // White to move, +0.30
		{CP: -35, BestMove: "e7e5", Depth: 18},                              // after e2e4, Black is worse by 0.35
		{CP: 400, BestMove: "d1h5", Depth: 18},                              // after f7f6??, White is +4
	}
	moves := Moves(positions, []string{"e2e4", "f7f6"}, true)
	if len(moves) != 2 {
		t.Fatalf("moves = %+v", moves)
	}
	white, black := moves[0], moves[1]
	if white.Classification != "best" || white.CPLoss != 0 || white.EvalBefore.CP != 30 || white.EvalAfter.CP != 35 {
		t.Fatalf("unexpected analysis of e2e4: %+v", white)
	}
	if black.Ply != 2 || black.BestMove != "e7e5" || black.EvalBefore.CP != 35 || black.EvalAfter.CP != 400 {
		t.Fatalf("evaluations must be from White's view: %+v", black)
	}
	// Black went from -0.35 to -4.00.
	if black.CPLoss != 365 || black.Classification != "blunder" || black.Accuracy > 30 {
		t.Fatalf("unexpected analysis of f7f6: %+v", black)
	}
}

func TestMovesWithMates(t *testing.T) {
	positions := []PositionEval{
		{Mate: mate(1), BestMove: "d8h4"}, // Black to move mates in one
		Checkmated(),                      // White is checkmated
	}
	moves := Moves(positions, []string{"d8h4"}, false)
	move := moves[0]
	if move.Classification != "best" || move.CPLoss != 0 || move.Accuracy != 100 {
		t.Fatalf("delivering mate is perfect: %+v", move)
	}
	if *move.EvalBefore.Mate != -1 || move.EvalBefore.CP >= 0 {
		t.Fatalf("mate for Black is negative from White's view: %+v", move.EvalBefore)
	}
	if *move.EvalAfter.Mate != 0 || move.EvalAfter.CP != -store.MateCP {
		t.Fatalf("checkmated White must be -MateCP: %+v", move.EvalAfter)
	}

	// Missing a mate in one for a merely winning move.
	missed := Moves([]PositionEval{{Mate: mate(1), BestMove: "d8h4"}, {CP: -300}}, []string{"g7g5"}, false)[0]
	if missed.CPLoss != 700 || missed.Classification == "best" {
		t.Fatalf("cp loss is capped at 1000 per side: %+v", missed)
	}
}
