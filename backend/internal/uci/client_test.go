package uci

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// fakeEngine answers each command line with the lines returned by respond.
func fakeEngine(t *testing.T, respond func(command string) []string) *Client {
	t.Helper()
	commandReader, commandWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	go func() {
		defer outputWriter.Close()
		scanner := bufio.NewScanner(commandReader)
		for scanner.Scan() {
			for _, line := range respond(scanner.Text()) {
				if _, err := io.WriteString(outputWriter, line+"\n"); err != nil {
					return
				}
			}
		}
	}()
	client := NewClient(commandWriter, outputReader, nil)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func standardResponses(command string) []string {
	switch {
	case command == "uci":
		return []string{"id name fake", "id author tests", "option name Hash type spin", "uciok"}
	case command == "isready":
		return []string{"info string NNUE evaluation enabled", "readyok"}
	case strings.HasPrefix(command, "go"):
		return []string{
			"info depth 1 score cp 13 nodes 20 nps 1000 time 1 pv e2e4",
			"info currmove e2e4 currmovenumber 1",
			"info depth 2 seldepth 3 score cp 30 nodes 400 nps 2000 time 2 pv e2e4 e7e5",
			"bestmove e2e4 ponder e7e5",
		}
	}
	return nil
}

func TestHandshakeIgnoresPlainInfoStrings(t *testing.T) {
	client := fakeEngine(t, standardResponses)
	identity, err := client.Handshake(context.Background(), map[string]string{"Hash": "16"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "fake" || identity.Author != "tests" {
		t.Fatalf("unexpected identity %+v", identity)
	}
}

func TestSearchKeepsLastInfoWithSearchData(t *testing.T) {
	client := fakeEngine(t, standardResponses)
	result, err := client.Search(context.Background(), Position{}, Limit{MoveTime: 100 * time.Millisecond}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.BestMove != "e2e4" {
		t.Fatalf("bestmove = %q", result.BestMove)
	}
	if result.Info.Depth != 2 || *result.Info.ScoreCP != 30 || result.Info.Nodes != 400 ||
		strings.Join(result.Info.PV, " ") != "e2e4 e7e5" {
		t.Fatalf("unexpected info %+v", result.Info)
	}
}

func TestSearchTimesOut(t *testing.T) {
	client := fakeEngine(t, func(string) []string { return nil })
	_, err := client.Search(context.Background(), Position{}, Limit{}, 50*time.Millisecond)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
}

func TestSetPositionReturnsEngineError(t *testing.T) {
	client := fakeEngine(t, func(command string) []string {
		if strings.HasPrefix(command, "position") {
			return []string{"info string error illegal move e2e5"}
		}
		return standardResponses(command)
	})
	err := client.SetPosition(context.Background(), Position{Moves: []string{"e2e5"}}, time.Second)
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Message != "illegal move e2e5" {
		t.Fatalf("expected engine error, got %v", err)
	}
}

func TestDetectsExitedEngine(t *testing.T) {
	commandReader, commandWriter := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, commandReader) }()
	client := NewClient(commandWriter, strings.NewReader("id name crashes\n"), nil)
	defer client.Close()
	if _, err := client.Handshake(context.Background(), nil, time.Second); !errors.Is(err, ErrExited) {
		t.Fatalf("expected exited engine, got %v", err)
	}
}

func TestCommands(t *testing.T) {
	cases := map[string]string{
		Position{}.command(): "position startpos",
		Position{Moves: []string{"e2e4", "e7e5"}}.command():      "position startpos moves e2e4 e7e5",
		Position{FEN: "7k/8/8/8/8/8/4K3/R7 w - - 0 1"}.command(): "position fen 7k/8/8/8/8/8/4K3/R7 w - - 0 1",
		Limit{MoveTime: 1500 * time.Millisecond}.command():       "go movetime 1500",
		Limit{Depth: 6}.command():                                "go depth 6",
		Limit{Nodes: 10000}.command():                            "go nodes 10000",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestParseInfo(t *testing.T) {
	info, ok := ParseInfo("info depth 0 score mate -3")
	if !ok || info.ScoreMate == nil || *info.ScoreMate != -3 || info.ScoreCP != nil {
		t.Fatalf("unexpected info %+v", info)
	}
	if _, ok := ParseInfo("info string depth 5"); ok {
		t.Fatal("info string must not be parsed as search data")
	}
	info, _ = ParseInfo("info depth 4 score cp 12 lowerbound nodes 99 pv g1f3")
	if *info.ScoreCP != 12 || info.Nodes != 99 || info.PV[0] != "g1f3" {
		t.Fatalf("bound flag broke parsing: %+v", info)
	}
}
