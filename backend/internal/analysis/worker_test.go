package analysis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/sandbox"
	"chess-arena/arena/internal/store"
)

// scriptedEngine answers each "go" with the next scripted score and best move.
type scriptedEngine struct {
	replies  []string // "score cp 30|e2e4"
	searches int
	starts   int
}

func (engine *scriptedEngine) Start(_ context.Context, _ sandbox.Spec) (*sandbox.Process, error) {
	engine.starts++
	commandReader, commandWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	go func() {
		defer outputWriter.Close()
		scanner := bufio.NewScanner(commandReader)
		for scanner.Scan() {
			var reply string
			switch command := scanner.Text(); {
			case command == "uci":
				reply = "id name Fakefish 1\nuciok\n"
			case command == "isready":
				reply = "readyok\n"
			case strings.HasPrefix(command, "go"):
				score, best, _ := strings.Cut(engine.replies[engine.searches], "|")
				engine.searches++
				reply = fmt.Sprintf("info depth 12 multipv 1 %s wdl 500 400 100 nodes 1000 pv %s\nbestmove %s\n", score, best, best)
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

type recordingStore struct {
	jobs     []*store.AnalysisJob
	saved    []store.MoveAnalysis
	engine   string
	failed   string
	released string
}

func (s *recordingStore) ReleaseAnalysis(_ context.Context, _ string, _ *store.AnalysisJob, message string, _ time.Duration) error {
	s.released = message
	return nil
}

func (s *recordingStore) ClaimAnalysis(context.Context, string, time.Duration) (*store.AnalysisJob, error) {
	if len(s.jobs) == 0 {
		return nil, nil
	}
	job := s.jobs[0]
	s.jobs = s.jobs[1:]
	return job, nil
}

func (s *recordingStore) SaveAnalysis(_ context.Context, _ string, _ *store.AnalysisJob, engine string, _ int64, moves []store.MoveAnalysis) error {
	s.engine, s.saved = engine, moves
	return nil
}

func (s *recordingStore) FailAnalysis(_ context.Context, _ string, _ *store.AnalysisJob, message string, _ bool, _ time.Duration) error {
	s.failed = message
	return nil
}

func TestWorkerAnalyzesFoolsMate(t *testing.T) {
	engine := &scriptedEngine{replies: []string{
		"score cp 30|e2e4",
		"score cp 10|e7e5",
		"score cp 150|e2e4", // after f3 e5 White should not play g4
		"score mate 1|d8h4",
	}}
	records := &recordingStore{jobs: []*store.AnalysisJob{{
		GameID: "game", StartFEN: rules.StartingFEN, Moves: []string{"f2f3", "e7e5", "g2g4", "d8h4"}, Attempt: 1,
	}}}
	worker := NewWorker(Config{ID: "analysis-1", Image: "fakefish"}, records, engine, slog.New(slog.DiscardHandler))
	defer worker.closeEngine()

	analyzed, err := worker.RunOnce(context.Background())
	if !analyzed || err != nil {
		t.Fatalf("RunOnce = %v, %v (failure %q)", analyzed, err, records.failed)
	}
	// The final, checkmated position is not searched.
	if engine.searches != 4 || records.engine != "Fakefish 1" || len(records.saved) != 4 {
		t.Fatalf("searches=%d engine=%q saved=%+v", engine.searches, records.engine, records.saved)
	}
	g4, mate := records.saved[2], records.saved[3]
	if g4.Classification != "blunder" || g4.BestMove != "e2e4" || g4.EvalAfter.CP != -store.MateCP+1 {
		t.Fatalf("g2g4 allows mate in one: %+v", g4)
	}
	if mate.Classification != "best" || *mate.EvalAfter.Mate != 0 || mate.EvalAfter.CP != -store.MateCP {
		t.Fatalf("d8h4 is mate: %+v", mate)
	}

	// The engine keeps running for the next game.
	if again, err := worker.RunOnce(context.Background()); again || err != nil || engine.starts != 1 {
		t.Fatalf("an empty queue must not start another engine: %v %v starts=%d", again, err, engine.starts)
	}
}

type brokenLauncher struct{}

func (brokenLauncher) Start(context.Context, sandbox.Spec) (*sandbox.Process, error) {
	return nil, errors.New("pull access denied")
}

func TestWorkerReleasesJobWhenEngineCannotStart(t *testing.T) {
	records := &recordingStore{jobs: []*store.AnalysisJob{{GameID: "game", StartFEN: rules.StartingFEN, Attempt: 1}}}
	worker := NewWorker(Config{ID: "analysis-1", Image: "missing"}, records, brokenLauncher{}, slog.New(slog.DiscardHandler))
	analyzed, err := worker.RunOnce(context.Background())
	if !analyzed || !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("RunOnce = %v, %v", analyzed, err)
	}
	if !strings.Contains(records.released, "pull access denied") || records.failed != "" {
		t.Fatalf("the job must be released, not failed: released=%q failed=%q", records.released, records.failed)
	}
}
