package arena

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/sandbox"
	"chess-arena/arena/internal/store"
)

// fakeLauncher starts in-memory UCI engines that answer "go" with scripted
// moves, keyed by image name.
type fakeLauncher struct {
	scripts  map[string][]string
	startErr error
	specs    []sandbox.Spec
}

func (launcher *fakeLauncher) Start(_ context.Context, spec sandbox.Spec) (*sandbox.Process, error) {
	if launcher.startErr != nil {
		return nil, launcher.startErr
	}
	launcher.specs = append(launcher.specs, spec)
	moves := append([]string(nil), launcher.scripts[spec.Image]...)
	commandReader, commandWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	go func() {
		defer outputWriter.Close()
		scanner := bufio.NewScanner(commandReader)
		for scanner.Scan() {
			var reply string
			switch command := scanner.Text(); {
			case command == "uci":
				reply = "id name fake\nuciok\n"
			case command == "isready":
				reply = "readyok\n"
			case strings.HasPrefix(command, "go"):
				reply = "info depth 1 score cp 0 nodes 20 time 1 pv " + moves[0] + "\nbestmove " + moves[0] + "\n"
				moves = moves[1:]
			case command == "quit":
				return
			}
			if _, err := io.WriteString(outputWriter, reply); err != nil {
				return
			}
		}
	}()
	return sandbox.NewProcess(commandWriter, outputReader, func() error { return commandReader.Close() }), nil
}

type fakeStore struct {
	mu       sync.Mutex
	game     *store.ClaimedGame
	moves    []store.MoveRecord
	result   *store.GameResult
	failure  *store.GameFailure
	requeued bool
}

func (fake *fakeStore) ClaimGame(context.Context, string, time.Duration) (*store.ClaimedGame, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	game := fake.game
	fake.game = nil
	return game, nil
}

func (fake *fakeStore) RecordMove(_ context.Context, _ string, _ time.Duration, move store.MoveRecord) error {
	fake.moves = append(fake.moves, move)
	return nil
}

func (fake *fakeStore) FinishGame(_ context.Context, _ string, result store.GameResult) error {
	fake.result = &result
	return nil
}

func (fake *fakeStore) ReleaseGame(_ context.Context, _ string, _ *store.ClaimedGame, failure store.GameFailure, requeue bool) error {
	fake.failure = &failure
	fake.requeued = requeue
	return nil
}

func claimedGame() *store.ClaimedGame {
	movetime := 100
	game := &store.ClaimedGame{
		ID: "game-1", TournamentID: "tournament-1", TournamentName: "Test cup", GameNumber: 1, Attempt: 1,
		StartFEN: rules.StartingFEN,
		White:    store.Engine{ID: "white-id", Name: "white", Version: "v1", Image: "white:v1"},
		Black:    store.Engine{ID: "black-id", Name: "black", Version: "v1", Image: "black:v1"},
		CPUs:     1, MemoryMB: 128,
	}
	game.Limit.MoveTimeMs = &movetime
	return game
}

func newTestWorker(fake *fakeStore, launcher *fakeLauncher) *Worker {
	return NewWorker(Config{ID: "test-worker"}, fake, launcher, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestWorkerPlaysAndStoresGame(t *testing.T) {
	fake := &fakeStore{game: claimedGame()}
	launcher := &fakeLauncher{scripts: map[string][]string{
		"white:v1": {"f2f3", "g2g4"},
		"black:v1": {"e7e5", "d8h4"},
	}}
	played, err := newTestWorker(fake, launcher).RunOnce(context.Background())
	if err != nil || !played {
		t.Fatalf("played=%v err=%v", played, err)
	}
	if fake.result == nil || fake.result.Result != rules.BlackWins || fake.result.WinnerEngineID != "black-id" {
		t.Fatalf("unexpected result %+v", fake.result)
	}
	if !strings.Contains(fake.result.PGN, `[White "white v1"]`) || !strings.Contains(fake.result.PGN, "2. g4 Qh4#") {
		t.Fatalf("unexpected PGN:\n%s", fake.result.PGN)
	}
	if len(fake.moves) != 4 || fake.moves[1].EngineID != "black-id" || fake.moves[1].Info.Nodes != 20 {
		t.Fatalf("unexpected moves %+v", fake.moves)
	}
	if spec := launcher.specs[0]; spec.CPUs != 1 || spec.MemoryMB != 128 {
		t.Fatalf("resource limits not passed to the sandbox: %+v", spec)
	}
}

func TestWorkerRequeuesWhenEngineCannotStart(t *testing.T) {
	fake := &fakeStore{game: claimedGame()}
	launcher := &fakeLauncher{startErr: errors.New("image not found")}
	_, err := newTestWorker(fake, launcher).RunOnce(context.Background())
	if err == nil || fake.failure == nil || fake.failure.Code != "white_engine" || !fake.requeued {
		t.Fatalf("expected requeued start failure, got err=%v failure=%+v", err, fake.failure)
	}
}

func TestWorkerFailsGameAfterMaxAttempts(t *testing.T) {
	game := claimedGame()
	game.Attempt = 4
	fake := &fakeStore{game: game}
	if _, err := newTestWorker(fake, &fakeLauncher{}).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.failure == nil || fake.failure.Code != "max_attempts" || fake.requeued {
		t.Fatalf("expected failed game, got %+v requeued=%v", fake.failure, fake.requeued)
	}
}
