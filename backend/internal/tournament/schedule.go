package tournament

import (
	"hash/fnv"
	"math/rand/v2"
)

// ScheduledGame is one game of a tournament before it is stored.
type ScheduledGame struct {
	Number          int
	PositionOrdinal int
	EngineAIsWhite  bool
}

// Schedule assigns a starting position and colors to every game.
//
// With alternating colors, consecutive games form pairs that share a starting
// position with colors swapped, so neither engine benefits from a lucky opening.
//
// Positions are used in the order given by PositionOrder. A random order is
// derived from seed, so the same tournament always gets the same schedule,
// while different tournaments sample different positions: a short tournament,
// or one stopped early by an SPRT, does not only play the first positions of
// a set grouped by opening.
func Schedule(config Config, positionCount int, seed uint64) []ScheduledGame {
	order := make([]int, positionCount)
	for index := range order {
		order[index] = index
	}
	if config.PositionOrder == PositionsRandom {
		random := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
		random.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}

	games := make([]ScheduledGame, 0, config.GameCount)
	for index := 0; index < config.GameCount; index++ {
		game := ScheduledGame{Number: index + 1}
		switch config.ColorOrder {
		case ColorAlternate:
			game.PositionOrdinal = order[(index/2)%positionCount]
			game.EngineAIsWhite = index%2 == 0
		case ColorEngineAWhite:
			game.PositionOrdinal = order[index%positionCount]
			game.EngineAIsWhite = true
		case ColorEngineBWhite:
			game.PositionOrdinal = order[index%positionCount]
		}
		games = append(games, game)
	}
	return games
}

// ScheduleSeed derives a schedule seed from a tournament ID.
func ScheduleSeed(tournamentID string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(tournamentID))
	return hash.Sum64()
}
