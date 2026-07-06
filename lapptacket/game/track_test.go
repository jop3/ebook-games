package game

import "testing"

// TestNextActorTrailingMarkerRule is the single most important turn-order
// detail in the whole game (per the spec's own gotcha list): whichever
// marker is furthest BEHIND always acts next — never strict alternation —
// and a tie favors player 0 ("player 1" in 1-based rules text).
func TestNextActorTrailingMarkerRule(t *testing.T) {
	cases := []struct {
		m0, m1 int
		want   int
	}{
		{0, 0, 0},   // tied at the start -> player 0
		{5, 5, 0},   // tied mid-game -> player 0
		{3, 7, 0},   // player 0 behind
		{7, 3, 1},   // player 1 behind
		{53, 40, 1}, // player 0 finished, player 1 still behind
		{40, 53, 0}, // player 1 finished, player 0 still behind
		{53, 53, 0}, // both finished (tie) -> player 0 (game should be over)
	}
	for _, c := range cases {
		if got := NextActor(c.m0, c.m1); got != c.want {
			t.Errorf("NextActor(%d,%d) = %d, want %d", c.m0, c.m1, got, c.want)
		}
	}
}

// TestNextActorInterleavedSequence walks a hand-picked sequence of marker
// moves (as if from a mix of BuyPatch and Advance actions) and checks that
// the actor alternates according to the trailing rule, NOT strict
// alternation — sometimes the same player acts twice in a row, exactly
// because buying a cheap/fast patch can still leave you behind.
func TestNextActorInterleavedSequence(t *testing.T) {
	// (m0, m1, wantActor) at each step, in order.
	steps := []struct {
		m0, m1, want int
	}{
		{0, 0, 0},   // start tied -> P0
		{2, 0, 1},   // P0 moved to 2 (bought a time-cost-2 patch) -> now P1 behind
		{2, 6, 0},   // P1 advanced hard past P0 -> now P0 behind again
		{2, 6, 0},   // P0 still behind after some no-op check
		{9, 6, 1},   // P0 jumped ahead of P1 -> P1 behind
		{9, 9, 0},   // P1 caught up exactly -> tie -> P0
		{9, 10, 0},  // P1 nudged 1 ahead -> P0 still behind
		{10, 10, 0}, // tie again -> P0
	}
	for i, s := range steps {
		if got := NextActor(s.m0, s.m1); got != s.want {
			t.Errorf("step %d: NextActor(%d,%d) = %d, want %d", i, s.m0, s.m1, got, s.want)
		}
	}
}

func TestCrossedCountLandingExactlyOn(t *testing.T) {
	positions := []int{4, 9, 15}
	if n := crossedCount(3, 4, positions); n != 1 {
		t.Fatalf("landing exactly on 4: crossedCount = %d, want 1", n)
	}
}

// TestCrossedCountMultiSquareJumpCrossesWithoutLanding is the explicit
// gotcha from the spec: "passing" an income square must be detected via the
// marker's start->end range for the move, not just its final resting cell.
func TestCrossedCountMultiSquareJumpCrossesWithoutLanding(t *testing.T) {
	positions := []int{4, 9, 15}
	// A jump from 7 to 12 (e.g. a patch with TimeCost=5) passes OVER the
	// income square at 9 without ever landing exactly on it.
	if n := crossedCount(7, 12, positions); n != 1 {
		t.Fatalf("jump 7->12 over income square 9: crossedCount = %d, want 1", n)
	}
	// A big jump can cross more than one marked square at once.
	if n := crossedCount(2, 16, positions); n != 3 {
		t.Fatalf("jump 2->16 over 4,9,15: crossedCount = %d, want 3", n)
	}
	// No movement, or movement entirely before/after all marks, crosses none.
	if n := crossedCount(5, 5, positions); n != 0 {
		t.Fatalf("zero-length move: crossedCount = %d, want 0", n)
	}
	if n := crossedCount(16, 20, positions); n != 0 {
		t.Fatalf("move entirely past all marks: crossedCount = %d, want 0", n)
	}
}

func TestCrossedSpecialIndicesSkipsAlreadyClaimed(t *testing.T) {
	positions := []int{7, 13, 20}
	claimed := []bool{false, false, false}

	idx := crossedSpecialIndices(5, 10, positions, claimed) // crosses 7
	if len(idx) != 1 || idx[0] != 0 {
		t.Fatalf("crossedSpecialIndices(5,10) = %v, want [0]", idx)
	}
	claimed[0] = true

	// A second player crossing the same range must not re-claim index 0.
	idx = crossedSpecialIndices(5, 10, positions, claimed)
	if len(idx) != 0 {
		t.Fatalf("expected already-claimed special patch to be skipped, got %v", idx)
	}

	// A jump crossing two at once, one already claimed, returns only the
	// unclaimed one.
	idx = crossedSpecialIndices(6, 21, positions, claimed) // crosses 7(claimed),13,20
	if len(idx) != 2 || idx[0] != 1 || idx[1] != 2 {
		t.Fatalf("crossedSpecialIndices(6,21) = %v, want [1 2]", idx)
	}
}

func TestClampTrack(t *testing.T) {
	if got := clampTrack(60); got != TrackEnd {
		t.Fatalf("clampTrack(60) = %d, want %d", got, TrackEnd)
	}
	if got := clampTrack(-3); got != 0 {
		t.Fatalf("clampTrack(-3) = %d, want 0", got)
	}
	if got := clampTrack(30); got != 30 {
		t.Fatalf("clampTrack(30) = %d, want 30", got)
	}
}

func TestIncomeAndSpecialSquaresDisjointAndSorted(t *testing.T) {
	seen := map[int]bool{}
	for _, p := range IncomeSquares {
		seen[p] = true
	}
	for _, p := range SpecialPatchPositions {
		if seen[p] {
			t.Fatalf("position %d appears in both IncomeSquares and SpecialPatchPositions", p)
		}
	}
	for i := 1; i < len(IncomeSquares); i++ {
		if IncomeSquares[i] <= IncomeSquares[i-1] {
			t.Fatalf("IncomeSquares not strictly sorted at %d", i)
		}
	}
	for i := 1; i < len(SpecialPatchPositions); i++ {
		if SpecialPatchPositions[i] <= SpecialPatchPositions[i-1] {
			t.Fatalf("SpecialPatchPositions not strictly sorted at %d", i)
		}
	}
	if IncomeSquares[len(IncomeSquares)-1] != TrackEnd {
		t.Fatalf("expected the final income square to be at TrackEnd (%d)", TrackEnd)
	}
}
