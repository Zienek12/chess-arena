package main

import (
	"errors"
	"testing"
)

func TestImportFENIsDeterministic(t *testing.T) {
	source := "# opening positions\n" + StartingFEN + "\n7k/8/8/8/8/8/4K3/8 b - - 0 1\n"
	first, err := ImportFEN(source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ImportFEN(source)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint || first.ID != first.Fingerprint {
		t.Fatalf("position set is not deterministic: %+v %+v", first, second)
	}
	if len(first.Positions) != 2 || first.Positions[0].Ordinal != 0 || first.Positions[1].Ordinal != 1 {
		t.Fatalf("unexpected positions: %+v", first.Positions)
	}
}

func TestImportPGNReadsFENTags(t *testing.T) {
	pgn := `[Event "Test"]
[SetUp "1"]
[FEN "7k/8/8/8/8/8/4K3/8 w - - 0 1"]

1. Ka2 *
`
	set, err := ImportPGN(pgn)
	if err != nil {
		t.Fatal(err)
	}
	if set.SourceType != PositionSourcePGN || len(set.Positions) != 1 {
		t.Fatalf("unexpected PGN import: %+v", set)
	}
}

func TestImportPGNWithoutFENUsesStartingPosition(t *testing.T) {
	set, err := ImportPGN(`[Event "Starting position"]

1. e4 e5 *
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Positions) != 1 || set.Positions[0].FEN != StartingFEN {
		t.Fatalf("unexpected default position: %+v", set.Positions)
	}
}

func TestTournamentConfigValidation(t *testing.T) {
	depth := 8
	config := TournamentConfig{
		Name:          "baseline",
		EngineAID:     "engine-a",
		EngineBID:     "engine-b",
		PositionSetID: "set-a",
		Depth:         &depth,
	}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	if config.Normalize().GameCount != 1000 {
		t.Fatal("default game count should be 1000")
	}

	nodes := int64(10000)
	config.Depth = nil
	config.Nodes = &nodes
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}

	config.Depth = &depth
	if err := config.Validate(); err == nil {
		t.Fatal("depth and nodes should be mutually exclusive")
	}
}

func TestIdempotentGameStore(t *testing.T) {
	store := NewIdempotentGameStore()
	first := GameSnapshot{
		TournamentID:   "tournament-a",
		GameNumber:     1,
		CurrentFEN:     StartingFEN,
		Version:        1,
		IdempotencyKey: "event-1",
	}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(GameSnapshot{
		TournamentID:   "tournament-a",
		GameNumber:     1,
		CurrentFEN:     StartingFEN,
		Version:        0,
		IdempotencyKey: "old-event",
	}); !errors.Is(err, ErrStaleGameState) {
		t.Fatalf("expected stale state error, got %v", err)
	}
	stored, ok := store.Get("tournament-a", 1)
	if !ok || stored.IdempotencyKey != "event-1" {
		t.Fatalf("unexpected stored game: %+v", stored)
	}
}
