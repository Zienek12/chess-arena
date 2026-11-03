package store

import "testing"

func TestDeclaredElo(t *testing.T) {
	cases := []struct {
		options map[string]string
		elo     int
		ok      bool
	}{
		{map[string]string{"UCI_LimitStrength": "true", "UCI_Elo": "1500"}, 1500, true},
		{map[string]string{"uci_limitstrength": "True", "uci_elo": " 1800 "}, 1800, true},
		{map[string]string{"UCI_Elo": "1500"}, 0, false},
		{map[string]string{"UCI_LimitStrength": "false", "UCI_Elo": "1500"}, 0, false},
		{map[string]string{"UCI_LimitStrength": "true", "UCI_Elo": "strong"}, 0, false},
		{nil, 0, false},
	}
	for _, test := range cases {
		elo, ok := DeclaredElo(test.options)
		if ok != test.ok || (ok && elo != test.elo) {
			t.Errorf("DeclaredElo(%v) = %d, %v", test.options, elo, ok)
		}
	}
}

func TestAnchorRatingsShiftOntoTheAnchorsScale(t *testing.T) {
	leaderboard := []LeaderboardEntry{
		{Engine: Engine{Name: "ours"}, Rating: 100},
		{Engine: Engine{Name: "sf1500", UCIOptions: map[string]string{"UCI_LimitStrength": "true", "UCI_Elo": "1500"}}, Rating: -50},
		{Engine: Engine{Name: "sf1800", UCIOptions: map[string]string{"UCI_LimitStrength": "true", "UCI_Elo": "1800"}}, Rating: 230},
	}
	anchorRatings(leaderboard)
	// Offsets 1550 and 1570 average to 1560.
	if *leaderboard[0].Elo != 1660 || *leaderboard[1].Elo != 1510 || *leaderboard[2].Elo != 1790 {
		t.Fatalf("unexpected Elo %v %v %v", *leaderboard[0].Elo, *leaderboard[1].Elo, *leaderboard[2].Elo)
	}
	if leaderboard[0].AnchorElo != nil || *leaderboard[2].AnchorElo != 1800 {
		t.Fatal("only anchors carry a declared Elo")
	}

	relative := []LeaderboardEntry{{Rating: 10}, {Rating: -10}}
	anchorRatings(relative)
	if relative[0].Elo != nil {
		t.Fatal("without anchors there is no absolute Elo")
	}
}
