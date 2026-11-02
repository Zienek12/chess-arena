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
	games := Schedule(Config{GameCount: 6, ColorOrder: ColorAlternate, PositionOrder: PositionsSequential}, 2, 0)
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
	for _, game := range Schedule(Config{GameCount: 3, ColorOrder: ColorEngineBWhite, PositionOrder: PositionsSequential}, 2, 0) {
		if game.EngineAIsWhite {
			t.Fatalf("engine A should never be white: %+v", game)
		}
	}
}

func TestScheduleShufflesPositionsPerTournament(t *testing.T) {
	config := Config{GameCount: 100, ColorOrder: ColorAlternate, PositionOrder: PositionsRandom}
	first := Schedule(config, 50, ScheduleSeed("tournament-1"))

	// Pairs share a position with colors swapped, and every position is used once.
	seen := map[int]bool{}
	for index := 0; index < len(first); index += 2 {
		white, black := first[index], first[index+1]
		if white.PositionOrdinal != black.PositionOrdinal || !white.EngineAIsWhite || black.EngineAIsWhite {
			t.Fatalf("games %d and %d are not a color-swapped pair: %+v %+v", white.Number, black.Number, white, black)
		}
		if seen[white.PositionOrdinal] {
			t.Fatalf("position %d is used twice", white.PositionOrdinal)
		}
		seen[white.PositionOrdinal] = true
	}
	if len(seen) != 50 {
		t.Fatalf("used %d positions, want 50", len(seen))
	}

	// The first games do not just take the first positions of the set.
	firstFive := map[int]bool{}
	for index := 0; index < 10; index += 2 {
		firstFive[first[index].PositionOrdinal] = true
	}
	if firstFive[0] && firstFive[1] && firstFive[2] && firstFive[3] && firstFive[4] {
		t.Fatalf("random order kept the set order: %+v", first[:10])
	}

	// The same tournament always gets the same schedule; another one differs.
	again := Schedule(config, 50, ScheduleSeed("tournament-1"))
	other := Schedule(config, 50, ScheduleSeed("tournament-2"))
	same, different := true, false
	for index := range first {
		same = same && first[index] == again[index]
		different = different || first[index] != other[index]
	}
	if !same || !different {
		t.Fatalf("schedule must be reproducible per tournament and differ between tournaments")
	}
}

func TestImportNamesPositions(t *testing.T) {
	pgn := "[Event \"Ruy Lopez\"]\n\n1. e4 e5 2. Nf3 Nc6 3. Bb5 *\n\n" +
		"[Event \"?\"]\n[Opening \"Sicilian Defence\"]\n[Variation \"Najdorf\"]\n\n1. e4 c5 *\n\n" +
		"[Event \"?\"]\n\n1. d4 d5 *\n"
	set, err := ImportPGN(pgn)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{set.Positions[0].Name, set.Positions[1].Name, set.Positions[2].Name}
	if names[0] != "Ruy Lopez" || names[1] != "Sicilian Defence, Najdorf" || names[2] != "" {
		t.Fatalf("names = %q", names)
	}

	fenSet, err := ImportFEN("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1 ; King's Pawn\n" +
		"rnbqkbnr/pppppppp/8/8/3P4/8/PPP1PPPP/RNBQKBNR b KQkq - 0 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if fenSet.Positions[0].Name != "King's Pawn" || fenSet.Positions[1].Name != "" {
		t.Fatalf("unexpected FEN names %+v", fenSet.Positions)
	}

	// Names do not change the fingerprint, so a named re-import finds the same set.
	unnamed, err := ImportPGN("[Event \"a\"]\n\n1. e4 e5 2. Nf3 Nc6 3. Bb5 *\n\n[Event \"b\"]\n\n1. e4 c5 *\n\n[Event \"c\"]\n\n1. d4 d5 *\n")
	if err != nil {
		t.Fatal(err)
	}
	if unnamed.Fingerprint != set.Fingerprint {
		t.Fatal("names must not change the fingerprint")
	}
}
