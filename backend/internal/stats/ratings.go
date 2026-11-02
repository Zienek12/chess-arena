package stats

import "math"

// Pairing is the result of all games between two players.
type Pairing struct {
	A, B    string
	Games   int
	APoints float64 // points scored by A; B scored Games - APoints
}

// priorGames are virtual draws every player plays against an average player.
// They keep ratings finite for players who won or lost every game, and pull
// players with few games towards the average.
const priorGames = 2

// Ratings estimates an Elo rating for every player from pairwise results
// with the Bradley-Terry model, draws counting as half a win. Ratings are
// relative: their mean is 0.
func Ratings(pairings []Pairing) map[string]float64 {
	points := map[string]float64{}
	opponents := map[string]map[string]int{}
	addGames := func(player, opponent string, games int) {
		if opponents[player] == nil {
			opponents[player] = map[string]int{}
		}
		opponents[player][opponent] += games
	}
	for _, pairing := range pairings {
		if pairing.Games == 0 || pairing.A == pairing.B {
			continue
		}
		points[pairing.A] += pairing.APoints
		points[pairing.B] += float64(pairing.Games) - pairing.APoints
		addGames(pairing.A, pairing.B, pairing.Games)
		addGames(pairing.B, pairing.A, pairing.Games)
	}

	// Strengths γ = 10^(elo/400), found with Hunter's minorization-maximization
	// iterations; the virtual average player has strength 1.
	strength := map[string]float64{}
	for player := range opponents {
		strength[player] = 1
	}
	for iteration := 0; iteration < 10_000; iteration++ {
		next := make(map[string]float64, len(strength))
		change := 0.0
		for player, games := range opponents {
			own := strength[player]
			denominator := priorGames / (own + 1)
			for opponent, count := range games {
				denominator += float64(count) / (own + strength[opponent])
			}
			next[player] = (points[player] + priorGames/2.0) / denominator
			change = math.Max(change, math.Abs(math.Log(next[player]/own)))
		}
		strength = next
		if change < 1e-9 {
			break
		}
	}

	ratings := make(map[string]float64, len(strength))
	mean := 0.0
	for player, value := range strength {
		ratings[player] = 400 * math.Log10(value)
		mean += ratings[player]
	}
	if len(ratings) > 0 {
		mean /= float64(len(ratings))
	}
	for player := range ratings {
		ratings[player] -= mean
	}
	return ratings
}
