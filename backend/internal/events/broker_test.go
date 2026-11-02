package events

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"chess-arena/arena/internal/database"
	"chess-arena/arena/internal/database/dbtest"
	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/store"
	"chess-arena/arena/internal/tournament"
)

func TestPublishSkipsSlowSubscribers(t *testing.T) {
	broker := NewBroker(slog.New(slog.DiscardHandler))
	events, unsubscribe := broker.Subscribe()
	for range subscriberBuffer + 10 {
		broker.Publish(store.Event{Type: store.EventMove})
	}
	if len(events) != subscriberBuffer {
		t.Fatalf("buffered %d events", len(events))
	}
	unsubscribe()
	unsubscribe()
	broker.Publish(store.Event{Type: store.EventMove})
}

func TestStoreChangesReachSubscribers(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	broker := NewBroker(slog.New(slog.DiscardHandler))
	events, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	go broker.Listen(ctx, pool)

	repository := store.New(pool)
	engineA, _ := repository.CreateEngine(ctx, store.NewEngine{Name: "a", Version: "1", Image: "a:1"})
	engineB, _ := repository.CreateEngine(ctx, store.NewEngine{Name: "b", Version: "1", Image: "b:1"})
	positions, _ := tournament.ImportFEN(rules.StartingFEN)
	set, _, err := repository.SavePositionSet(ctx, "start", positions)
	if err != nil {
		t.Fatal(err)
	}
	movetime := 100
	config := tournament.Config{Name: "live", EngineAID: engineA.ID, EngineBID: engineB.ID, PositionSetID: set.ID,
		GameCount: 1, Limit: tournament.SearchLimit{MoveTimeMs: &movetime}}

	// LISTEN starts asynchronously; create tournaments until one is seen.
	deadline := time.After(10 * time.Second)
	for {
		created, err := repository.CreateTournament(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		select {
		case event := <-events:
			if event.Type != store.EventTournament || event.TournamentID == "" {
				t.Fatalf("unexpected event %+v", event)
			}
			if event.TournamentID == created.ID {
				return
			}
		case <-time.After(200 * time.Millisecond):
		case <-deadline:
			t.Fatal("no event received")
		}
	}
}
