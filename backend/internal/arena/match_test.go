package arena

import (
	"context"
	"errors"
	"testing"
	"time"

	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/uci"
)

// scripted plays its moves in order and records the positions it was given.
type scripted struct {
	moves     []string
	err       error
	positions []uci.Position
}

func (engine *scripted) Search(_ context.Context, position uci.Position, _ uci.Limit, _ time.Duration) (uci.SearchResult, error) {
	engine.positions = append(engine.positions, position)
	if engine.err != nil {
		return uci.SearchResult{}, engine.err
	}
	move := engine.moves[0]
	engine.moves = engine.moves[1:]
	return uci.SearchResult{BestMove: move, Info: uci.Info{Depth: 1}}, nil
}

func TestMatchPlaysToCheckmate(t *testing.T) {
	white := &scripted{moves: []string{"f2f3", "g2g4"}}
	black := &scripted{moves: []string{"e7e5", "d8h4"}}
	var recorded []PlayedMove
	outcome, err := Match{
		White: white, Black: black,
		OnMove: func(_ context.Context, move PlayedMove) error {
			recorded = append(recorded, move)
			return nil
		},
	}.Play(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Game.Result() != rules.BlackWins || outcome.Game.Termination() != rules.TerminationCheckmate {
		t.Fatalf("got %s by %s", outcome.Game.Result(), outcome.Game.Termination())
	}
	if len(recorded) != 4 || recorded[3].Ply != 4 || recorded[3].Side != rules.Black || recorded[3].NextSide != rules.White {
		t.Fatalf("unexpected recorded moves %+v", recorded)
	}
	if got := black.positions[1].Moves; len(got) != 3 || got[2] != "g2g4" {
		t.Fatalf("engine got wrong move history %v", got)
	}
}

func TestMatchForfeitsIllegalMove(t *testing.T) {
	outcome, err := Match{
		White: &scripted{moves: []string{"e2e5"}},
		Black: &scripted{},
	}.Play(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Game.Result() != rules.BlackWins || outcome.Game.Termination() != TerminationIllegalMove {
		t.Fatalf("got %s by %s", outcome.Game.Result(), outcome.Game.Termination())
	}
}

func TestMatchForfeitsTimeoutAndCrash(t *testing.T) {
	cases := map[string]error{
		TerminationTimeForfeit: uci.ErrTimeout,
		TerminationEngineError: uci.ErrExited,
	}
	for termination, engineErr := range cases {
		outcome, err := Match{
			White: &scripted{moves: []string{"e2e4"}},
			Black: &scripted{err: engineErr},
		}.Play(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Game.Result() != rules.WhiteWins || outcome.Game.Termination() != termination {
			t.Fatalf("%v: got %s by %s", engineErr, outcome.Game.Result(), outcome.Game.Termination())
		}
	}
}

func TestMatchResumesFromPlayedMovesAndCustomFEN(t *testing.T) {
	start := "7k/8/8/8/8/8/4K3/R7 w - - 0 1"
	white := &scripted{moves: []string{"e2e3"}}
	black := &scripted{err: uci.ErrTimeout}
	_, err := Match{
		White: white, Black: black,
		StartFEN: start,
		Played:   []string{"a1a2", "h8g8"},
	}.Play(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	position := white.positions[0]
	if position.FEN != start || len(position.Moves) != 2 {
		t.Fatalf("resumed game sent wrong position %+v", position)
	}
}

func TestMatchReturnsPersistenceErrors(t *testing.T) {
	failure := errors.New("database down")
	_, err := Match{
		White:  &scripted{moves: []string{"e2e4"}},
		Black:  &scripted{},
		OnMove: func(context.Context, PlayedMove) error { return failure },
	}.Play(context.Background())
	if !errors.Is(err, failure) {
		t.Fatalf("expected persistence error, got %v", err)
	}
}
