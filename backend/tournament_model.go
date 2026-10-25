package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const StartingFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type PositionSourceType string

const (
	PositionSourceFEN PositionSourceType = "fen"
	PositionSourcePGN PositionSourceType = "pgn"
)

type EngineDefinition struct {
	ID         string
	Name       string
	Version    string
	Image      string
	UCIOptions map[string]string
}

type Position struct {
	Ordinal int
	FEN     string
	Hash    string
}

type PositionSet struct {
	ID          string
	Fingerprint string
	SourceType  PositionSourceType
	SourceName  string
	Positions   []Position
}

type TournamentConfig struct {
	ID            string
	Name          string
	EngineAID     string
	EngineBID     string
	PositionSetID string
	GameCount     int
	Depth         *int
	Nodes         *int64
	ColorOrder    string
	Status        string
}

type GameSnapshot struct {
	TournamentID   string
	GameNumber     int
	PositionID     string
	WhiteEngineID  string
	BlackEngineID  string
	CurrentFEN     string
	Ply            int
	Status         string
	Result         string
	NextSide       string
	Attempt        int
	Version        int64
	IdempotencyKey string
}

type GameMove struct {
	GameID         string
	Ply            int
	UCI            string
	FENAfter       string
	EngineID       string
	IdempotencyKey string
}

var ErrStaleGameState = errors.New("game state version is stale")

func (config TournamentConfig) Normalize() TournamentConfig {
	if config.GameCount == 0 {
		config.GameCount = 1000
	}
	if config.ColorOrder == "" {
		config.ColorOrder = "alternate"
	}
	if config.Status == "" {
		config.Status = "created"
	}
	return config
}

func (config TournamentConfig) Validate() error {
	config = config.Normalize()
	if strings.TrimSpace(config.Name) == "" {
		return errors.New("tournament name is required")
	}
	if config.EngineAID == "" || config.EngineBID == "" || config.EngineAID == config.EngineBID {
		return errors.New("tournament requires two different engines")
	}
	if config.PositionSetID == "" {
		return errors.New("position set is required")
	}
	if config.GameCount < 1 {
		return errors.New("game count must be positive")
	}
	if (config.Depth == nil) == (config.Nodes == nil) {
		return errors.New("exactly one of depth or nodes must be configured")
	}
	if config.Depth != nil && *config.Depth < 1 {
		return errors.New("depth must be positive")
	}
	if config.Nodes != nil && *config.Nodes < 1 {
		return errors.New("nodes must be positive")
	}
	if config.ColorOrder != "alternate" && config.ColorOrder != "engine_a_white" && config.ColorOrder != "engine_b_white" {
		return fmt.Errorf("invalid color order %q", config.ColorOrder)
	}
	return nil
}

func ImportFEN(source string) (PositionSet, error) {
	positions := make([]Position, 0)
	scanner := bufio.NewScanner(strings.NewReader(source))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fen, err := CanonicalFEN(line)
		if err != nil {
			return PositionSet{}, fmt.Errorf("invalid FEN on line %d: %w", len(positions)+1, err)
		}
		positions = append(positions, Position{Ordinal: len(positions), FEN: fen, Hash: hashText(fen)})
	}
	if err := scanner.Err(); err != nil {
		return PositionSet{}, err
	}
	return newPositionSet(PositionSourceFEN, positions)
}

func ImportPGN(source string) (PositionSet, error) {
	positions := make([]Position, 0)
	scanner := bufio.NewScanner(strings.NewReader(source))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[FEN ") {
			continue
		}
		fen, ok := pgnTagValue(line, "FEN")
		if !ok {
			return PositionSet{}, fmt.Errorf("invalid FEN tag: %s", line)
		}
		canonical, err := CanonicalFEN(fen)
		if err != nil {
			return PositionSet{}, fmt.Errorf("invalid PGN FEN tag: %w", err)
		}
		positions = append(positions, Position{Ordinal: len(positions), FEN: canonical, Hash: hashText(canonical)})
	}
	if err := scanner.Err(); err != nil {
		return PositionSet{}, err
	}
	if len(positions) == 0 {
		positions = append(positions, Position{Ordinal: 0, FEN: StartingFEN, Hash: hashText(StartingFEN)})
	}
	return newPositionSet(PositionSourcePGN, positions)
}

func newPositionSet(sourceType PositionSourceType, positions []Position) (PositionSet, error) {
	if len(positions) == 0 {
		return PositionSet{}, errors.New("position source contains no positions")
	}
	canonical := make([]string, 0, len(positions))
	for _, position := range positions {
		canonical = append(canonical, position.FEN)
	}
	fingerprint := hashText(strings.Join(canonical, "\n"))
	return PositionSet{
		ID:          fingerprint,
		Fingerprint: fingerprint,
		SourceType:  sourceType,
		Positions:   positions,
	}, nil
}

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

func pgnTagValue(line string, tag string) (string, bool) {
	prefix := "[" + tag + " \""
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "\"]") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(line, prefix), "\"]"), true
}

func hashText(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func IdempotencyKey(tournamentID string, gameNumber int, ply int, value string) string {
	return hashText(fmt.Sprintf("%s:%d:%d:%s", tournamentID, gameNumber, ply, value))
}

type IdempotentGameStore struct {
	games map[string]GameSnapshot
}

func NewIdempotentGameStore() *IdempotentGameStore {
	return &IdempotentGameStore{games: make(map[string]GameSnapshot)}
}

func (store *IdempotentGameStore) Save(snapshot GameSnapshot) error {
	key := fmt.Sprintf("%s:%d", snapshot.TournamentID, snapshot.GameNumber)
	previous, exists := store.games[key]
	if exists && snapshot.Version < previous.Version {
		return ErrStaleGameState
	}
	if exists && snapshot.Version == previous.Version {
		if snapshot.IdempotencyKey != previous.IdempotencyKey {
			return fmt.Errorf("conflicting game state at version %d", snapshot.Version)
		}
		return nil
	}
	store.games[key] = snapshot
	return nil
}

func (store *IdempotentGameStore) Get(tournamentID string, gameNumber int) (GameSnapshot, bool) {
	snapshot, ok := store.games[fmt.Sprintf("%s:%d", tournamentID, gameNumber)]
	return snapshot, ok
}

func SortedEngineVersions(engines []EngineDefinition) []string {
	versions := make([]string, 0, len(engines))
	for _, engine := range engines {
		versions = append(versions, engine.Version)
	}
	sort.Strings(versions)
	return versions
}
