// Package stats turns game results into Elo estimates, the likelihood of
// superiority and sequential probability ratio tests (SPRT).
//
// Results are modelled as a trinomial distribution of wins, draws and losses,
// and Elo uses the logistic model: a score s corresponds to
// -400·log10(1/s - 1) Elo.
package stats

import "math"

// z95 is the two-sided 95% quantile of the normal distribution.
const z95 = 1.959964

// Summary describes a match from one player's point of view.
type Summary struct {
	Games  int     `json:"games"`
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
	Score  float64 `json:"score"`
	// Elo and EloError (half the 95% confidence interval) are omitted while
	// they are infinite, which happens when every game was won or lost.
	Elo      *float64 `json:"elo,omitempty"`
	EloError *float64 `json:"elo_error,omitempty"`
	// LOS is the likelihood of superiority: the probability that the player
	// is stronger, judged from wins and losses only.
	LOS float64 `json:"los"`
}

// EloFromScore converts an expected score in (0, 1) to an Elo difference.
func EloFromScore(score float64) float64 {
	return -400 * math.Log10(1/score-1)
}

// ScoreFromElo converts an Elo difference to the expected score.
func ScoreFromElo(elo float64) float64 {
	return 1 / (1 + math.Pow(10, -elo/400))
}

// Summarize computes the score, Elo with its 95% error and LOS.
func Summarize(wins, draws, losses int) Summary {
	summary := Summary{Games: wins + draws + losses, Wins: wins, Draws: draws, Losses: losses, LOS: 0.5}
	if summary.Games == 0 {
		return summary
	}
	n := float64(summary.Games)
	score, variance := scoreAndVariance(float64(wins), float64(draws), float64(losses))
	summary.Score = score
	if score > 0 && score < 1 {
		elo := EloFromScore(score)
		summary.Elo = &elo
		margin := z95 * math.Sqrt(variance/n)
		if low, high := score-margin, score+margin; low > 0 && high < 1 {
			eloError := (EloFromScore(high) - EloFromScore(low)) / 2
			summary.EloError = &eloError
		}
	}
	if decisive := float64(wins + losses); decisive > 0 {
		summary.LOS = 0.5 * (1 + math.Erf(float64(wins-losses)/math.Sqrt(2*decisive)))
	}
	return summary
}

// scoreAndVariance returns the mean score per game and its per-game variance.
func scoreAndVariance(wins, draws, losses float64) (float64, float64) {
	n := wins + draws + losses
	w, d, l := wins/n, draws/n, losses/n
	score := w + d/2
	variance := w*math.Pow(1-score, 2) + d*math.Pow(0.5-score, 2) + l*math.Pow(score, 2)
	return score, variance
}

// sprtPrior is added to every outcome count before computing the LLR. Without
// it a one-sided run (all wins) has no variance and could never be decided;
// with many games its effect vanishes.
const sprtPrior = 0.5

// SPRT is a sequential test of H0: elo = Elo0 against H1: elo = Elo1, with
// false positive rate Alpha and false negative rate Beta.
type SPRT struct {
	Elo0  float64 `json:"elo0"`
	Elo1  float64 `json:"elo1"`
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
}

// SPRT outcomes.
const (
	AcceptH0 = "H0" // the player is about Elo0 or weaker: the change failed
	AcceptH1 = "H1" // the player is about Elo1 or stronger: the change passed
)

// SPRTStatus is the state of a test after some games.
type SPRTStatus struct {
	SPRT
	LLR   float64 `json:"llr"`
	Lower float64 `json:"lower_bound"`
	Upper float64 `json:"upper_bound"`
	// Result is AcceptH0, AcceptH1, or empty while the test continues.
	Result string `json:"result,omitempty"`
}

// Bounds are the log-likelihood ratios at which the test accepts H0 (lower)
// or H1 (upper).
func (test SPRT) Bounds() (float64, float64) {
	return math.Log(test.Beta / (1 - test.Alpha)), math.Log((1 - test.Beta) / test.Alpha)
}

// Evaluate computes the log-likelihood ratio with the generalized SPRT
// approximation used by Fishtest and decides once it crosses a bound.
func (test SPRT) Evaluate(wins, draws, losses int) SPRTStatus {
	status := SPRTStatus{SPRT: test}
	status.Lower, status.Upper = test.Bounds()
	if wins+draws+losses == 0 {
		return status
	}
	w, d, l := float64(wins)+sprtPrior, float64(draws)+sprtPrior, float64(losses)+sprtPrior
	n := w + d + l
	score, variance := scoreAndVariance(w, d, l)
	s0, s1 := ScoreFromElo(test.Elo0), ScoreFromElo(test.Elo1)
	status.LLR = n * (s1 - s0) * (2*score - s0 - s1) / (2 * variance)
	switch {
	case status.LLR >= status.Upper:
		status.Result = AcceptH1
	case status.LLR <= status.Lower:
		status.Result = AcceptH0
	}
	return status
}
