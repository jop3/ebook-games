package game

import "testing"

// --- Nigiri + Wasabi ----------------------------------------------------

func TestScoreNigiriPlain(t *testing.T) {
	// Hand-computed: egg(1) + salmon(2) + squid(3) = 6, no wasabi involved.
	got := scoreNigiriCat([]Card{NigiriEgg(), NigiriSalmon(), NigiriSquid()})
	if got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
}

func TestScoreNigiriWasabiTriples(t *testing.T) {
	// Wasabi before Squid: 3*3=9. Hand-computed total: 9.
	got := scoreNigiriCat([]Card{Wasabi(), NigiriSquid()})
	if got != 9 {
		t.Fatalf("got %d, want 9 (wasabi tripling squid)", got)
	}
	// Egg and Salmon tripled too, per spec: Egg 3, Salmon 6.
	got = scoreNigiriCat([]Card{Wasabi(), NigiriEgg()})
	if got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
	got = scoreNigiriCat([]Card{Wasabi(), NigiriSalmon()})
	if got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
}

func TestScoreNigiriWasabiMustPrecedeItsNigiri(t *testing.T) {
	// Wasabi played AFTER the nigiri must NOT retroactively triple it: the
	// squid scores plain (3), and the trailing unused wasabi contributes 0.
	// Hand-computed total: 3.
	got := scoreNigiriCat([]Card{NigiriSquid(), Wasabi()})
	if got != 3 {
		t.Fatalf("got %d, want 3 (wasabi after nigiri must not triple it)", got)
	}
}

func TestScoreNigiriWasabiBanksAcrossGap(t *testing.T) {
	// Wasabi, then an unrelated Tempura, then Squid: the wasabi should still
	// be "banked" across the intervening card. Hand-computed: 3*3=9.
	got := scoreNigiriCat([]Card{Wasabi(), Tempura(), NigiriSquid()})
	if got != 9 {
		t.Fatalf("got %d, want 9 (wasabi banks across an unrelated card)", got)
	}
}

func TestScoreNigiriTwoWasabiTwoNigiri(t *testing.T) {
	// Two banked wasabi, two nigiri played later: both get tripled.
	// Hand-computed: egg*3 + salmon*3 = 3 + 6 = 9.
	got := scoreNigiriCat([]Card{Wasabi(), Wasabi(), NigiriEgg(), NigiriSalmon()})
	if got != 9 {
		t.Fatalf("got %d, want 9 (two banked wasabi each triple one nigiri)", got)
	}
}

func TestScoreNigiriExtraWasabiUnused(t *testing.T) {
	// A wasabi with no nigiri ever following it scores nothing itself, and
	// doesn't affect anything else. Hand-computed: squid(3) + 0 = 3.
	got := scoreNigiriCat([]Card{NigiriSquid(), Wasabi()})
	if got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
}

// --- Tempura -------------------------------------------------------------

func TestScoreTempuraPairs(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 0},  // odd leftover scores 0
		{2, 5},  // one pair
		{3, 5},  // one pair + odd leftover
		{4, 10}, // two pairs
		{5, 10}, // two pairs + odd leftover
	}
	for _, c := range cases {
		tableau := make([]Card, c.n)
		for i := range tableau {
			tableau[i] = Tempura()
		}
		got := scoreTempuraCat(tableau)
		if got != c.want {
			t.Errorf("tempura(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

// --- Sashimi ---------------------------------------------------------------

func TestScoreSashimiTriples(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 0},  // incomplete
		{2, 0},  // incomplete
		{3, 10}, // one complete set
		{4, 10}, // one complete set + incomplete leftover
		{5, 10}, // one complete set + incomplete leftover
		{6, 20}, // two complete sets
	}
	for _, c := range cases {
		tableau := make([]Card, c.n)
		for i := range tableau {
			tableau[i] = Sashimi()
		}
		got := scoreSashimiCat(tableau)
		if got != c.want {
			t.Errorf("sashimi(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

// --- Dumplings ---------------------------------------------------------------

func TestScoreDumplingAllTiers(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 1},
		{2, 3},
		{3, 6},
		{4, 10},
		{5, 15},
		{6, 15}, // 5+ all score the same as 5
		{9, 15},
	}
	for _, c := range cases {
		tableau := make([]Card, c.n)
		for i := range tableau {
			tableau[i] = Dumpling()
		}
		got := scoreDumplingCat(tableau)
		if got != c.want {
			t.Errorf("dumpling(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

// --- Maki majority ---------------------------------------------------------

func TestScoreMakiMajoritySimple(t *testing.T) {
	// P0: 3 icons, P1: 1 icon, P2: 0. Hand-computed: P0=6, P1=3, P2=0.
	tableaus := [][]Card{
		{Maki3()},
		{Maki1()},
		{Tempura()},
	}
	got := ScoreMaki(tableaus)
	want := []int{6, 3, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestScoreMakiTiedForFirstSplitsAndDropsSecond(t *testing.T) {
	// P0 and P1 tied for the lead at 3 icons each; P2 has 1. Standard Sushi
	// Go rule: a tie for first splits the 6 (3 each) and NO second place is
	// awarded at all that round — P2 scores nothing despite having more
	// maki than nobody else.
	tableaus := [][]Card{
		{Maki3()},
		{Maki3()},
		{Maki1()},
	}
	got := ScoreMaki(tableaus)
	want := []int{3, 3, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestScoreMakiTiedForSecondSplitsDown(t *testing.T) {
	// P0 leads outright at 5. P1 and P2 tie for second at 2 each: 3/2 = 1
	// each (split rounded down), remainder discarded.
	tableaus := [][]Card{
		{Maki3(), Maki2()},
		{Maki2()},
		{Maki2()},
	}
	got := ScoreMaki(tableaus)
	want := []int{6, 1, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestScoreMakiAllTiedNoSecondPlacePossible(t *testing.T) {
	// Every player has the same nonzero count: everyone splits the 6, and
	// there is no lower rank to award (nobody has fewer icons than the max).
	tableaus := [][]Card{{Maki2()}, {Maki2()}, {Maki2()}}
	got := ScoreMaki(tableaus)
	for i, v := range got {
		if v != 2 { // 6/3 = 2 each
			t.Fatalf("player %d got %d, want 2 (all tied, 6/3=2 each)", i, v)
		}
	}
}

func TestScoreMakiNobodyPlayedAny(t *testing.T) {
	tableaus := [][]Card{{Tempura()}, {Sashimi()}}
	got := ScoreMaki(tableaus)
	for i, v := range got {
		if v != 0 {
			t.Fatalf("player %d got %d, want 0", i, v)
		}
	}
}

func TestScoreMakiTwoPlayerDropsSecondPlace(t *testing.T) {
	// Judgment call: in a 2-player game, only the leader scores (6); the
	// trailing player gets nothing for maki even though in a 3+ player game
	// a lone "second place" would normally be worth 3.
	tableaus := [][]Card{
		{Maki3()},
		{Maki1()},
	}
	got := ScoreMaki(tableaus)
	want := []int{6, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (2p drops the 2nd-place maki award)", got, want)
		}
	}
}

// --- Pudding -----------------------------------------------------------

func TestScorePuddingThreePlayers(t *testing.T) {
	// P0=6, P1=2, P2=0 puddings across the game. Most=P0 (+6), fewest=P2 (-6).
	got := ScorePudding([]int{6, 2, 0})
	want := []int{6, 0, -6}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestScorePuddingTieForMostAndFewest(t *testing.T) {
	// P0 & P1 tie for most (4 each): split 6 -> 3 each. P2 & P3 tie for
	// fewest (1 each): split -6 -> -3 each.
	got := ScorePudding([]int{4, 4, 1, 1})
	want := []int{3, 3, -3, -3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestScorePuddingTwoPlayerNoMinus(t *testing.T) {
	// Judgment call (explicit in the spec): with exactly 2 players, the
	// trailing player does NOT lose 6 — only the leader gains.
	got := ScorePudding([]int{5, 1})
	want := []int{6, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (2p: no -6 penalty)", got, want)
		}
	}
}

func TestScorePuddingAllTiedNoAward(t *testing.T) {
	// Everyone has the same Pudding count: "most" and "fewest" are the same
	// people, so nobody gains or loses anything (also exercises the 2p case
	// where a plain tie must not still hand out a split +6).
	got := ScorePudding([]int{3, 3, 3})
	for i, v := range got {
		if v != 0 {
			t.Fatalf("player %d got %d, want 0 (fully tied)", i, v)
		}
	}
	got2 := ScorePudding([]int{2, 2})
	for i, v := range got2 {
		if v != 0 {
			t.Fatalf("2p player %d got %d, want 0 (fully tied)", i, v)
		}
	}
}

// --- ScoreRound combines everything, incl. Maki -----------------------------

func TestScoreRoundCombinesAllCategories(t *testing.T) {
	// P0: tempura pair(5) + sashimi triple(10) + 3 dumplings(6) +
	// wasabi->squid(9) + maki3(part of majority) = 5+10+6+9 = 30 plus maki.
	// P1: nothing scoring except a couple of maki icons.
	p0 := []Card{Tempura(), Tempura(), Sashimi(), Sashimi(), Sashimi(),
		Dumpling(), Dumpling(), Dumpling(), Wasabi(), NigiriSquid(), Maki3()}
	p1 := []Card{Maki1()}
	p2 := []Card{Tempura()} // 3rd player so the maki 2nd-place award is in play
	scores := ScoreRound([][]Card{p0, p1, p2})
	if scores[0].Tempura != 5 || scores[0].Sashimi != 10 || scores[0].Dumpling != 6 || scores[0].Nigiri != 9 {
		t.Fatalf("p0 breakdown wrong: %+v", scores[0])
	}
	if scores[0].Maki != 6 || scores[1].Maki != 3 {
		t.Fatalf("maki majority wrong: p0=%d p1=%d", scores[0].Maki, scores[1].Maki)
	}
	wantTotal := 5 + 10 + 6 + 9 + 6
	if scores[0].Total() != wantTotal {
		t.Fatalf("p0 total = %d, want %d", scores[0].Total(), wantTotal)
	}
}
