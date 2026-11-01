// Package rules validates moves and detects the end of a game. The arena never
// trusts an engine to report illegal moves or game results on its own.
package rules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/corentings/chess/v2"
)

// StartingFEN is the standard initial position.
const StartingFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// Results in PGN notation.
const (
	WhiteWins  = "1-0"
	BlackWins  = "0-1"
	Draw       = "1/2-1/2"
	Unfinished = "*"
)

// Terminations decided by the rules of chess.
const (
	TerminationCheckmate            = "checkmate"
	TerminationStalemate            = "stalemate"
	TerminationThreefoldRepetition  = "threefold_repetition"
	TerminationFivefoldRepetition   = "fivefold_repetition"
	TerminationFiftyMoveRule        = "fifty_move_rule"
	TerminationSeventyFiveMoveRule  = "seventy_five_move_rule"
	TerminationInsufficientMaterial = "insufficient_material"
)

// ErrIllegalMove is returned when a move is malformed or not legal in the current position.
var ErrIllegalMove = errors.New("illegal move")

// Color is the side to move: "w" or "b".
type Color string

const (
	White Color = "w"
	Black Color = "b"
)

// Opposite returns the other side.
func (c Color) Opposite() Color {
	if c == White {
		return Black
	}
	return White
}

// Game is a chess game started from an arbitrary position.
type Game struct {
	game     *chess.Game
	startFEN string
	moves    []string
	forfeit  string
}

// NewGame starts a game from fen; an empty fen means the standard starting position.
func NewGame(fen string) (*Game, error) {
	if fen == "" {
		fen = StartingFEN
	}
	option, err := chess.FEN(fen)
	if err != nil {
		return nil, fmt.Errorf("invalid FEN %q: %w", fen, err)
	}
	return &Game{game: chess.NewGame(option), startFEN: fen}, nil
}

// Play applies a move in UCI notation, then claims threefold repetition and
// the fifty-move rule automatically, as an arbiter would for engines.
func (g *Game) Play(move string) error {
	if g.Over() {
		return fmt.Errorf("%w %s: game is already over", ErrIllegalMove, move)
	}
	if err := g.game.PushNotationMove(move, chess.UCINotation{}, nil); err != nil {
		return fmt.Errorf("%w %s: %v", ErrIllegalMove, move, err)
	}
	g.moves = append(g.moves, move)
	if g.game.Outcome() != chess.NoOutcome {
		return nil
	}
	for _, method := range g.game.EligibleDraws() {
		if method == chess.ThreefoldRepetition || method == chess.FiftyMoveRule {
			return g.game.Draw(method)
		}
	}
	return nil
}

// Forfeit ends the game as a loss for color, for example after a time forfeit.
func (g *Game) Forfeit(color Color, termination string) {
	if g.Over() {
		return
	}
	if color == White {
		g.game.Resign(chess.White)
	} else {
		g.game.Resign(chess.Black)
	}
	g.forfeit = termination
}

// Over reports whether the game has a result.
func (g *Game) Over() bool {
	return g.game.Outcome() != chess.NoOutcome
}

// Result returns the PGN result, "*" while the game is in progress.
func (g *Game) Result() string {
	return string(g.game.Outcome())
}

// Termination describes why the game ended, or "" while it is in progress.
func (g *Game) Termination() string {
	if g.forfeit != "" {
		return g.forfeit
	}
	switch g.game.Method() {
	case chess.Checkmate:
		return TerminationCheckmate
	case chess.Stalemate:
		return TerminationStalemate
	case chess.ThreefoldRepetition:
		return TerminationThreefoldRepetition
	case chess.FivefoldRepetition:
		return TerminationFivefoldRepetition
	case chess.FiftyMoveRule:
		return TerminationFiftyMoveRule
	case chess.SeventyFiveMoveRule:
		return TerminationSeventyFiveMoveRule
	case chess.InsufficientMaterial:
		return TerminationInsufficientMaterial
	}
	return ""
}

// Turn returns the side to move.
func (g *Game) Turn() Color {
	if g.game.Position().Turn() == chess.White {
		return White
	}
	return Black
}

// FEN returns the current position.
func (g *Game) FEN() string {
	return g.game.FEN()
}

// StartFEN returns the position the game started from.
func (g *Game) StartFEN() string {
	return g.startFEN
}

// Moves returns the moves played so far in UCI notation.
func (g *Game) Moves() []string {
	return append([]string(nil), g.moves...)
}

// PGN renders the game with the given tags. Result, SetUp and FEN tags are added automatically.
func (g *Game) PGN(tags map[string]string) string {
	for key, value := range tags {
		g.game.AddTagPair(key, value)
	}
	g.game.AddTagPair("Result", g.Result())
	if g.startFEN != StartingFEN {
		g.game.AddTagPair("SetUp", "1")
		g.game.AddTagPair("FEN", g.startFEN)
	}
	return strings.TrimSpace(g.game.String()) + "\n"
}
