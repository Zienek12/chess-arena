package tournament

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/corentings/chess/v2"

	"chess-arena/arena/internal/rules"
)

// PositionSourceType is the format a position set was imported from.
type PositionSourceType string

const (
	PositionSourceFEN PositionSourceType = "fen"
	PositionSourcePGN PositionSourceType = "pgn"
)

// Position is one starting position inside a set.
type Position struct {
	Ordinal int
	FEN     string
	Hash    string
}

// PositionSet is an ordered list of starting positions. Its fingerprint is
// deterministic, so importing the same positions twice yields the same set.
type PositionSet struct {
	Fingerprint string
	SourceType  PositionSourceType
	Positions   []Position
}

// ImportFEN reads one FEN per non-empty line; lines starting with # are comments.
func ImportFEN(source string) (PositionSet, error) {
	fens := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(source))
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fen, err := CanonicalFEN(text)
		if err != nil {
			return PositionSet{}, fmt.Errorf("invalid FEN on line %d: %w", line, err)
		}
		if _, err := rules.NewGame(fen); err != nil {
			return PositionSet{}, fmt.Errorf("invalid FEN on line %d: %w", line, err)
		}
		fens = append(fens, fen)
	}
	if err := scanner.Err(); err != nil {
		return PositionSet{}, err
	}
	return newPositionSet(PositionSourceFEN, fens)
}

// ImportPGN takes the final position of every game in source. A game with only
// a FEN tag yields that FEN; a game with moves (an opening line) yields the
// position after the moves.
func ImportPGN(source string) (PositionSet, error) {
	fens := make([]string, 0)
	scanner := chess.NewScanner(strings.NewReader(source))
	for scanner.HasNext() {
		game, err := scanner.ParseNext()
		if err != nil {
			return PositionSet{}, fmt.Errorf("invalid PGN game %d: %w", len(fens)+1, err)
		}
		fen, err := CanonicalFEN(game.FEN())
		if err != nil {
			return PositionSet{}, fmt.Errorf("invalid PGN game %d: %w", len(fens)+1, err)
		}
		fens = append(fens, fen)
	}
	return newPositionSet(PositionSourcePGN, fens)
}

func newPositionSet(sourceType PositionSourceType, fens []string) (PositionSet, error) {
	if len(fens) == 0 {
		return PositionSet{}, errors.New("position source contains no positions")
	}
	positions := make([]Position, 0, len(fens))
	seen := make(map[string]bool, len(fens))
	for _, fen := range fens {
		hash := hashText(fen)
		if seen[hash] {
			continue
		}
		seen[hash] = true
		positions = append(positions, Position{Ordinal: len(positions), FEN: fen, Hash: hash})
	}
	canonical := make([]string, 0, len(positions))
	for _, position := range positions {
		canonical = append(canonical, position.FEN)
	}
	return PositionSet{
		Fingerprint: hashText(strings.Join(canonical, "\n")),
		SourceType:  sourceType,
		Positions:   positions,
	}, nil
}

// CanonicalFEN checks the FEN structure and normalizes whitespace.
func CanonicalFEN(value string) (string, error) {
	fields := strings.Fields(value)
	if len(fields) != 6 {
		return "", errors.New("FEN must contain six fields")
	}
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != 8 {
		return "", errors.New("piece placement must contain eight ranks")
	}
	whiteKing, blackKing := 0, 0
	for _, rank := range ranks {
		files := 0
		for _, character := range rank {
			switch {
			case character >= '1' && character <= '8':
				files += int(character - '0')
			case strings.ContainsRune("PNBRQKpnbrqk", character):
				files++
				if character == 'K' {
					whiteKing++
				}
				if character == 'k' {
					blackKing++
				}
			default:
				return "", fmt.Errorf("invalid piece character %q", character)
			}
		}
		if files != 8 {
			return "", errors.New("each FEN rank must contain eight files")
		}
	}
	if whiteKing != 1 || blackKing != 1 {
		return "", errors.New("FEN must contain exactly one king per side")
	}
	if fields[1] != "w" && fields[1] != "b" {
		return "", errors.New("invalid side to move")
	}
	if fields[2] != "-" {
		seen := make(map[rune]bool)
		for _, character := range fields[2] {
			if !strings.ContainsRune("KQkq", character) || seen[character] {
				return "", errors.New("invalid castling rights")
			}
			seen[character] = true
		}
	}
	if fields[3] != "-" && (len(fields[3]) != 2 || fields[3][0] < 'a' || fields[3][0] > 'h' || (fields[3][1] != '3' && fields[3][1] != '6')) {
		return "", errors.New("invalid en passant square")
	}
	for index, name := range []string{"halfmove clock", "fullmove number"} {
		value, err := strconv.Atoi(fields[index+4])
		if err != nil || value < 0 || (index == 1 && value == 0) {
			return "", fmt.Errorf("invalid %s", name)
		}
	}
	return strings.Join(fields, " "), nil
}

func hashText(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

// IdempotencyKey identifies one event in a game, so a retried write is recognized as a duplicate.
func IdempotencyKey(tournamentID string, gameNumber int, ply int, value string) string {
	return hashText(fmt.Sprintf("%s:%d:%d:%s", tournamentID, gameNumber, ply, value))
}
