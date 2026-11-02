package stats

import (
	"math"
	"testing"
)

func near(got, want, tolerance float64) bool {
	return math.Abs(got-want) <= tolerance
}

func TestEloAndScoreAreInverse(t *testing.T) {
	for _, elo := range []float64{-400, -100, 0, 35, 200} {
		if got := EloFromScore(ScoreFromElo(elo)); !near(got, elo, 1e-9) {
			t.Fatalf("EloFromScore(ScoreFromElo(%v)) = %v", elo, got)
		}
	}
	if !near(ScoreFromElo(400), 10.0/11, 1e-12) {
		t.Fatal("400 Elo must mean a 10:1 score ratio")
	}
}

func TestSummarize(t *testing.T) {
	// 60 wins, 80 draws, 60 losses: an even match.
	even := Summarize(60, 80, 60)
	if even.Score != 0.5 || *even.Elo != 0 || even.LOS != 0.5 {
		t.Fatalf("unexpected even summary %+v", even)
	}
	// 0.6 score per game with variance 0.24 over 100 games.
	strong := Summarize(60, 0, 40)
	if !near(*strong.Elo, 70.44, 0.01) || !near(*strong.EloError, 70.7, 0.5) {
		t.Fatalf("unexpected Elo %v ± %v", *strong.Elo, *strong.EloError)
	}
	if !near(strong.LOS, 0.9772, 0.0001) {
		t.Fatalf("LOS = %v", strong.LOS)
	}
	perfect := Summarize(10, 0, 0)
	if perfect.Elo != nil || perfect.EloError != nil || perfect.Score != 1 {
		t.Fatalf("a perfect score has no finite Elo: %+v", perfect)
	}
	if empty := Summarize(0, 0, 0); empty.Games != 0 || empty.Elo != nil || empty.LOS != 0.5 {
		t.Fatalf("unexpected empty summary %+v", empty)
	}
}

func TestSPRT(t *testing.T) {
	test := SPRT{Elo0: 0, Elo1: 10, Alpha: 0.05, Beta: 0.05}
	lower, upper := test.Bounds()
	if !near(lower, -2.944, 0.001) || !near(upper, 2.944, 0.001) {
		t.Fatalf("bounds = %v, %v", lower, upper)
	}
	if status := test.Evaluate(0, 0, 0); status.LLR != 0 || status.Result != "" {
		t.Fatalf("no games must not decide: %+v", status)
	}
	if status := test.Evaluate(3, 0, 0); status.Result != "" || status.LLR <= 0 {
		t.Fatalf("three wins are too few to decide a small difference: %+v", status)
	}
	// Winning every game decides quickly when the hypotheses are far apart.
	wide := SPRT{Elo0: 0, Elo1: 200, Alpha: 0.1, Beta: 0.1}
	if status := wide.Evaluate(5, 0, 0); status.Result != AcceptH1 {
		t.Fatalf("a perfect score must pass eventually: %+v", status)
	}
	if status := wide.Evaluate(0, 0, 5); status.Result != AcceptH0 {
		t.Fatalf("losing every game must fail: %+v", status)
	}
	if status := test.Evaluate(600, 400, 400); status.Result != AcceptH1 {
		t.Fatalf("a clearly stronger player must pass: %+v", status)
	}
	if status := test.Evaluate(400, 400, 600); status.Result != AcceptH0 {
		t.Fatalf("a clearly weaker player must fail: %+v", status)
	}
	if status := test.Evaluate(52, 100, 48); status.Result != "" || status.LLR <= 0 {
		t.Fatalf("a small edge needs more games: %+v", status)
	}
}

func TestRatings(t *testing.T) {
	ratings := Ratings([]Pairing{
		{A: "strong", B: "middle", Games: 100, APoints: 64},
		{A: "middle", B: "weak", Games: 100, APoints: 64},
		{A: "perfect", B: "weak", Games: 10, APoints: 10},
	})
	if len(ratings) != 4 {
		t.Fatalf("ratings = %v", ratings)
	}
	if !(ratings["strong"] > ratings["middle"] && ratings["middle"] > ratings["weak"]) {
		t.Fatalf("ratings are not ordered: %v", ratings)
	}
	// 64% is about 100 Elo; the prior pulls each gap slightly towards zero.
	if gap := ratings["strong"] - ratings["middle"]; gap < 85 || gap > 105 {
		t.Fatalf("gap = %v", gap)
	}
	if math.IsInf(ratings["perfect"], 0) || ratings["perfect"] <= ratings["weak"] {
		t.Fatalf("a perfect score must give a finite, higher rating: %v", ratings)
	}
	sum := 0.0
	for _, rating := range ratings {
		sum += rating
	}
	if !near(sum, 0, 1e-6) {
		t.Fatalf("ratings must average to zero: %v", ratings)
	}
}
