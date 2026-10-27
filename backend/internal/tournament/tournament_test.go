package tournament

import (
	"testing"

	"chess-arena/arena/internal/rules"
)

func TestImportFENIsDeterministic(t *testing.T) {
	source := "# opening positions\n" + rules.StartingFEN + "\n7k/8/8/8/8/8/4K3/R7 b - - 0 1\n"
	first, err := ImportFEN(source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ImportFEN(source)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("position set is not deterministic: %+v %+v", first, second)
	}
	if len(first.Positions) != 2 || first.Positions[0].Ordinal != 0 || first.Positions[1].Ordinal != 1 {
		t.Fatalf("unexpected positions: %+v", first.Positions)
	}
}

func TestImportFENReportsLineNumber(t *testing.T) {
	_, err := ImportFEN("# comment\n\nnot a fen\n")
	if err == nil || err.Error()[:20] != "invalid FEN on line " {
		t.Fatalf("expected line number in error, got %v", err)
	}
}

func TestImportPGNUsesPositionAfterMoves(t *testing.T) {
	pgn := `[Event "Opening 1"]

1. e4 e5 *

[Event "Endgame"]
[SetUp "1"]
[FEN "7k/8/8/8/8/8/4K3/R7 w - - 0 1"]

*
`
	set, err := ImportPGN(pgn)
	if err != nil {
		t.Fatal(err)
	}
	if set.SourceType != PositionSourcePGN || len(set.Positions) != 2 {
		t.Fatalf("unexpected PGN import: %+v", set)
	}
	if want := "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2"; set.Positions[0].FEN != want {
		t.Fatalf("got %s, want %s", set.Positions[0].FEN, want)
	}
	if want := "7k/8/8/8/8/8/4K3/R7 w - - 0 1"; set.Positions[1].FEN != want {
		t.Fatalf("got %s, want %s", set.Positions[1].FEN, want)
	}
}

func TestConfigValidation(t *testing.T) {
	movetime := 1000
	config := Config{
		Name:          "baseline",
		EngineAID:     "engine-a",
		EngineBID:     "engine-b",
		PositionSetID: "set-a",
		Limit:         SearchLimit{MoveTimeMs: &movetime},
	}.Normalize()
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	if config.GameCount != DefaultGameCount || config.CPUs != DefaultCPUs || config.MemoryMB != DefaultMemoryMB {
		t.Fatalf("defaults not applied: %+v", config)
	}

	depth := 8
	config.Limit.Depth = &depth
	if err := config.Validate(); err == nil {
		t.Fatal("movetime and depth should be mutually exclusive")
	}

	config.Limit = SearchLimit{}
	if err := config.Validate(); err == nil {
		t.Fatal("a search limit is required")
	}
}

func TestScheduleAlternatesColorsPerPosition(t *testing.T) {
	games := Schedule(Config{GameCount: 6, ColorOrder: ColorAlternate}, 2)
	want := []ScheduledGame{
		{1, 0, true}, {2, 0, false},
		{3, 1, true}, {4, 1, false},
		{5, 0, true}, {6, 0, false},
	}
	for index := range want {
		if games[index] != want[index] {
			t.Fatalf("game %d: got %+v, want %+v", index+1, games[index], want[index])
		}
	}
}

func TestScheduleFixedColor(t *testing.T) {
	for _, game := range Schedule(Config{GameCount: 3, ColorOrder: ColorEngineBWhite}, 2) {
		if game.EngineAIsWhite {
			t.Fatalf("engine A should never be white: %+v", game)
		}
	}
}
