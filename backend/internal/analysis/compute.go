// Package analysis reviews finished games with a reference engine such as
// Stockfish: it evaluates every position, then judges each played move by how
// much it lost compared to the engine's best move.
package analysis

import (
	"math"

	"chess-arena/arena/internal/store"
)

// maxCPLoss caps the loss of one move, so a single blunder in a lost
// position does not dominate the average.
const maxCPLoss = 1000

// Winning-chance drops, in percentage points, that make a move an
// inaccuracy, a mistake or a blunder (Lichess uses the same thresholds).
const (
	inaccuracyDrop = 5
	mistakeDrop    = 10
	blunderDrop    = 15
)

// PositionEval is the engine's verdict on one position, from the point of
// view of the side to move.
type PositionEval struct {
	CP       int
	Mate     *int // moves to mate; 0 means the side to move is checkmated
	BestMove string
	PV       []string
	Depth    int
}

// Checkmated is the evaluation of a position whose side to move is mated.
func Checkmated() PositionEval {
	zero := 0
	return PositionEval{Mate: &zero}
}

// centipawns turns an evaluation into centipawns, with mates as ±MateCP:
// a nearer mate is better for the winner.
func (eval PositionEval) centipawns() int {
	if eval.Mate == nil {
		return eval.CP
	}
	switch mate := *eval.Mate; {
	case mate > 0:
		return store.MateCP - mate
	case mate < 0:
		return -store.MateCP - mate
	default:
		return -store.MateCP
	}
}

// fromWhite converts an evaluation of the side to move to White's view.
func (eval PositionEval) fromWhite(whiteToMove bool) store.Eval {
	sign := 1
	if !whiteToMove {
		sign = -1
	}
	result := store.Eval{CP: sign * eval.centipawns()}
	if eval.Mate != nil {
		mate := sign * *eval.Mate
		result.Mate = &mate
	}
	return result
}

// WinPercent is the chance to win in percent for a centipawn evaluation,
// using the Lichess model fitted on rated games.
func WinPercent(cp int) float64 {
	return 50 + 50*(2/(1+math.Exp(-0.00368208*float64(cp)))-1)
}

// MoveAccuracy is the accuracy in percent of a move that changed the
// mover's winning chances from before to after (Lichess formula).
func MoveAccuracy(before, after float64) float64 {
	if after >= before {
		return 100
	}
	accuracy := 103.1668*math.Exp(-0.04354*(before-after)) - 3.1669 + 1
	return math.Max(0, math.Min(100, accuracy))
}

func clamp(value, limit int) int {
	return max(-limit, min(limit, value))
}

// Moves judges every played move. positions holds one evaluation per
// position: the start, then the position after each move, so it has one
// more entry than moves.
func Moves(positions []PositionEval, moves []string, whiteStarts bool) []store.MoveAnalysis {
	result := make([]store.MoveAnalysis, 0, len(moves))
	whiteToMove := whiteStarts
	for index, move := range moves {
		before, after := positions[index], positions[index+1]
		// Both from the mover's point of view.
		beforeCP := before.centipawns()
		afterCP := -after.centipawns()
		beforeChance, afterChance := WinPercent(beforeCP), WinPercent(afterCP)

		analysis := store.MoveAnalysis{
			Ply:        index + 1,
			BestMove:   before.BestMove,
			BestPV:     before.PV,
			Depth:      before.Depth,
			EvalBefore: before.fromWhite(whiteToMove),
			EvalAfter:  after.fromWhite(!whiteToMove),
			CPLoss:     max(0, clamp(beforeCP, maxCPLoss)-clamp(afterCP, maxCPLoss)),
			Accuracy:   MoveAccuracy(beforeChance, afterChance),
		}
		switch drop := beforeChance - afterChance; {
		case move == before.BestMove:
			analysis.Classification = "best"
		case drop >= blunderDrop:
			analysis.Classification = "blunder"
		case drop >= mistakeDrop:
			analysis.Classification = "mistake"
		case drop >= inaccuracyDrop:
			analysis.Classification = "inaccuracy"
		default:
			analysis.Classification = "good"
		}
		result = append(result, analysis)
		whiteToMove = !whiteToMove
	}
	return result
}
