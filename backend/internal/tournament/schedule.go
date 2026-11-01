package tournament

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
func Schedule(config Config, positionCount int) []ScheduledGame {
	games := make([]ScheduledGame, 0, config.GameCount)
	for index := 0; index < config.GameCount; index++ {
		game := ScheduledGame{Number: index + 1}
		switch config.ColorOrder {
		case ColorAlternate:
			game.PositionOrdinal = (index / 2) % positionCount
			game.EngineAIsWhite = index%2 == 0
		case ColorEngineAWhite:
			game.PositionOrdinal = index % positionCount
			game.EngineAIsWhite = true
		case ColorEngineBWhite:
			game.PositionOrdinal = index % positionCount
		}
		games = append(games, game)
	}
	return games
}
