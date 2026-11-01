// Package uci drives a chess engine over the Universal Chess Interface.
// Every call has a deadline, so a hung engine cannot block the arena.
package uci

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrorPrefix marks an engine-reported error. Engines may send "info string"
// at any time for logging; only lines with this prefix are errors.
const ErrorPrefix = "info string error "

var (
	// ErrTimeout means the engine did not answer before the deadline.
	ErrTimeout = errors.New("engine did not respond in time")
	// ErrExited means the engine closed its output, usually because it crashed.
	ErrExited = errors.New("engine exited")
)

// EngineError is an error the engine reported with ErrorPrefix.
type EngineError struct {
	Message string
}

func (err *EngineError) Error() string {
	return "engine: " + err.Message
}

// Position is the position sent with the "position" command.
type Position struct {
	FEN   string // empty means the standard starting position
	Moves []string
}

func (position Position) command() string {
	command := "position startpos"
	if position.FEN != "" {
		command = "position fen " + position.FEN
	}
	if len(position.Moves) > 0 {
		command += " moves " + strings.Join(position.Moves, " ")
	}
	return command
}

// Limit is the argument of the "go" command. At most one field should be set;
// with none set the engine decides on its own.
type Limit struct {
	MoveTime time.Duration
	Depth    int
	Nodes    int64
}

func (limit Limit) command() string {
	switch {
	case limit.MoveTime > 0:
		return "go movetime " + strconv.FormatInt(limit.MoveTime.Milliseconds(), 10)
	case limit.Depth > 0:
		return "go depth " + strconv.Itoa(limit.Depth)
	case limit.Nodes > 0:
		return "go nodes " + strconv.FormatInt(limit.Nodes, 10)
	}
	return "go"
}

// SearchResult is the engine's answer to "go".
type SearchResult struct {
	BestMove string
	Info     Info          // last info line with search data
	Elapsed  time.Duration // wall-clock time measured by the arena
}

// Identity is what the engine reports in response to "uci".
type Identity struct {
	Name   string
	Author string
}

// Client is a UCI session over a pair of pipes.
type Client struct {
	stdin     io.WriteCloser
	lines     chan string
	done      chan struct{}
	stop      func() error
	closeOnce sync.Once
	closeErr  error
}

// NewClient starts reading engine output. stop is called by Close to release
// the engine process; it may be nil.
func NewClient(stdin io.WriteCloser, stdout io.Reader, stop func() error) *Client {
	client := &Client{stdin: stdin, lines: make(chan string, 256), done: make(chan struct{}), stop: stop}
	go client.readLines(stdout)
	return client
}

func (client *Client) readLines(stdout io.Reader) {
	defer close(client.lines)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case client.lines <- strings.TrimSpace(scanner.Text()):
		case <-client.done:
			return
		}
	}
}

func (client *Client) send(command string) error {
	if _, err := io.WriteString(client.stdin, command+"\n"); err != nil {
		return fmt.Errorf("send %q: %w", firstWord(command), err)
	}
	return nil
}

// readUntil passes engine lines to handle until it returns done, the engine
// exits, the timeout passes, or ctx is cancelled.
func (client *Client) readUntil(ctx context.Context, timeout time.Duration, handle func(line string) (done bool, err error)) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrTimeout
		case line, ok := <-client.lines:
			if !ok {
				return ErrExited
			}
			done, err := handle(line)
			if err != nil || done {
				return err
			}
		}
	}
}

// Handshake performs "uci", applies options and waits for "readyok".
func (client *Client) Handshake(ctx context.Context, options map[string]string, timeout time.Duration) (Identity, error) {
	var identity Identity
	if err := client.send("uci"); err != nil {
		return identity, err
	}
	err := client.readUntil(ctx, timeout, func(line string) (bool, error) {
		switch {
		case strings.HasPrefix(line, "id name "):
			identity.Name = strings.TrimPrefix(line, "id name ")
		case strings.HasPrefix(line, "id author "):
			identity.Author = strings.TrimPrefix(line, "id author ")
		case strings.HasPrefix(line, ErrorPrefix):
			return false, &EngineError{Message: strings.TrimPrefix(line, ErrorPrefix)}
		}
		return line == "uciok", nil
	})
	if err != nil {
		return identity, fmt.Errorf("uci handshake: %w", err)
	}
	for name, value := range options {
		if err := client.send(fmt.Sprintf("setoption name %s value %s", name, value)); err != nil {
			return identity, err
		}
	}
	return identity, client.Ready(ctx, timeout)
}

// Ready sends "isready" and waits for "readyok". An engine error reported in
// the meantime, for example about a previous "position", is returned.
func (client *Client) Ready(ctx context.Context, timeout time.Duration) error {
	if err := client.send("isready"); err != nil {
		return err
	}
	var reported error
	err := client.readUntil(ctx, timeout, func(line string) (bool, error) {
		if strings.HasPrefix(line, ErrorPrefix) {
			reported = &EngineError{Message: strings.TrimPrefix(line, ErrorPrefix)}
		}
		return line == "readyok", nil
	})
	if err != nil {
		return fmt.Errorf("isready: %w", err)
	}
	return reported
}

// NewGame tells the engine a new game starts.
func (client *Client) NewGame(ctx context.Context, timeout time.Duration) error {
	if err := client.send("ucinewgame"); err != nil {
		return err
	}
	return client.Ready(ctx, timeout)
}

// SetPosition sends a position and confirms the engine accepted it.
func (client *Client) SetPosition(ctx context.Context, position Position, timeout time.Duration) error {
	if err := client.send(position.command()); err != nil {
		return err
	}
	return client.Ready(ctx, timeout)
}

// Search sends the position and "go", then waits for "bestmove".
func (client *Client) Search(ctx context.Context, position Position, limit Limit, timeout time.Duration) (SearchResult, error) {
	var result SearchResult
	if err := client.send(position.command()); err != nil {
		return result, err
	}
	started := time.Now()
	if err := client.send(limit.command()); err != nil {
		return result, err
	}
	err := client.readUntil(ctx, timeout, func(line string) (bool, error) {
		if strings.HasPrefix(line, ErrorPrefix) {
			return false, &EngineError{Message: strings.TrimPrefix(line, ErrorPrefix)}
		}
		if info, ok := ParseInfo(line); ok && info.hasSearchData() {
			result.Info = info
			return false, nil
		}
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "bestmove" {
			result.BestMove = fields[1]
			return true, nil
		}
		return false, nil
	})
	result.Elapsed = time.Since(started)
	if err != nil {
		return result, fmt.Errorf("search: %w", err)
	}
	return result, nil
}

// Close asks the engine to quit and releases the process.
func (client *Client) Close() error {
	client.closeOnce.Do(func() {
		close(client.done)
		_ = client.send("quit")
		_ = client.stdin.Close()
		if client.stop != nil {
			client.closeErr = client.stop()
		}
	})
	return client.closeErr
}

func firstWord(command string) string {
	if word, _, found := strings.Cut(command, " "); found {
		return word
	}
	return command
}
