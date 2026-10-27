package rules

import (
	"errors"
	"strings"
	"testing"
)

func playAll(t *testing.T, game *Game, moves ...string) {
	t.Helper()
	for _, move := range moves {
		if err := game.Play(move); err != nil {
			t.Fatalf("play %s: %v", move, err)
		}
	}
}

func TestDetectsCheckmate(t *testing.T) {
	game, err := NewGame("")
	if err != nil {
		t.Fatal(err)
	}
	playAll(t, game, "f2f3", "e7e5", "g2g4", "d8h4")
	if game.Result() != BlackWins || game.Termination() != TerminationCheckmate {
		t.Fatalf("got %s by %s", game.Result(), game.Termination())
	}
}

func TestRejectsIllegalMove(t *testing.T) {
	game, _ := NewGame("")
	if err := game.Play("e2e5"); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected illegal move, got %v", err)
	}
	if err := game.Play("0000"); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected null move to be illegal, got %v", err)
	}
}

func TestClaimsThreefoldRepetition(t *testing.T) {
	game, _ := NewGame("")
	playAll(t, game, "g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8")
	if game.Result() != Draw || game.Termination() != TerminationThreefoldRepetition {
		t.Fatalf("got %s by %s", game.Result(), game.Termination())
	}
}

func TestDetectsStalemate(t *testing.T) {
	game, _ := NewGame("7k/8/6K1/5Q2/8/8/8/8 w - - 0 1")
	playAll(t, game, "f5f7")
	if game.Result() != Draw || game.Termination() != TerminationStalemate {
		t.Fatalf("got %s by %s", game.Result(), game.Termination())
	}
}

func TestForfeitAndPGN(t *testing.T) {
	start := "7k/8/8/8/8/8/4K3/R7 b - - 0 1"
	game, _ := NewGame(start)
	playAll(t, game, "h8g8")
	game.Forfeit(White, "time_forfeit")
	if game.Result() != BlackWins || game.Termination() != "time_forfeit" {
		t.Fatalf("got %s by %s", game.Result(), game.Termination())
	}
	pgn := game.PGN(map[string]string{"White": "a", "Black": "b"})
	for _, want := range []string{`[FEN "` + start + `"]`, `[Result "0-1"]`, "1... Kg8", "0-1\n"} {
		if !strings.Contains(pgn, want) {
			t.Fatalf("PGN missing %q:\n%s", want, pgn)
		}
	}
}
