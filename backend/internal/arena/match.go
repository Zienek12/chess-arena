// Package arena plays tournament games between two engines: it enforces the
// clock, validates every move, adjudicates the result and persists progress.
package arena

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chess-arena/arena/internal/rules"
	"chess-arena/arena/internal/uci"
)

// Terminations decided by the arena rather than the rules of chess.
const (
	TerminationTimeForfeit = "time_forfeit"
	TerminationIllegalMove = "illegal_move"
	TerminationEngineError = "engine_error"
)

// Searcher is an engine that can be asked for a move.
type Searcher interface {
	Search(ctx context.Context, position uci.Position, limit uci.Limit, timeout time.Duration) (uci.SearchResult, error)
}

// PlayedMove is a validated move with the engine's telemetry.
type PlayedMove struct {
	Ply      int
	UCI      string
	FENAfter string
	Side     rules.Color
	NextSide rules.Color
	Search   uci.SearchResult
}

// Outcome is a finished game. Detail explains a forfeit, for example the illegal move.
type Outcome struct {
	Game   *rules.Game
	Detail string
}

// Match is one game between two engines.
type Match struct {
	White       Searcher
	Black       Searcher
	StartFEN    string
	Played      []string // moves already played, when resuming
	Limit       uci.Limit
	MoveTimeout time.Duration // includes the margin for process and pipe latency
	OnMove      func(ctx context.Context, move PlayedMove) error
}

// Play runs the game to the end. Engine faults (timeouts, crashes, illegal
// moves) lose the game for that engine; only infrastructure problems, such as
// a failed OnMove or a cancelled ctx, are returned as errors.
func (match Match) Play(ctx context.Context) (Outcome, error) {
	game, err := rules.NewGame(match.StartFEN)
	if err != nil {
		return Outcome{}, err
	}
	for _, move := range match.Played {
		if err := game.Play(move); err != nil {
			return Outcome{}, fmt.Errorf("replay stored moves: %w", err)
		}
	}
	position := uci.Position{}
	if game.StartFEN() != rules.StartingFEN {
		position.FEN = game.StartFEN()
	}

	for !game.Over() {
		side := game.Turn()
		engine := match.White
		if side == rules.Black {
			engine = match.Black
		}
		position.Moves = game.Moves()

		result, err := engine.Search(ctx, position, match.Limit, match.MoveTimeout)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Outcome{}, ctxErr
		}
		if err != nil {
			termination := TerminationEngineError
			if errors.Is(err, uci.ErrTimeout) {
				termination = TerminationTimeForfeit
			}
			game.Forfeit(side, termination)
			return Outcome{Game: game, Detail: err.Error()}, nil
		}
		if err := game.Play(result.BestMove); err != nil {
			game.Forfeit(side, TerminationIllegalMove)
			return Outcome{Game: game, Detail: err.Error()}, nil
		}
		if match.OnMove != nil {
			played := PlayedMove{
				Ply:      len(game.Moves()),
				UCI:      result.BestMove,
				FENAfter: game.FEN(),
				Side:     side,
				NextSide: side.Opposite(),
				Search:   result,
			}
			if err := match.OnMove(ctx, played); err != nil {
				return Outcome{}, err
			}
		}
	}
	return Outcome{Game: game}, nil
}
