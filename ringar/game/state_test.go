package game

import "testing"

// TestPlacementPhase drives all 10 placements and checks the turn order
// (White first, per the spec) and the phase transition into PhasePlaying.
func TestPlacementPhase(t *testing.T) {
	s := NewGame(OpponentHotseat)
	if s.Phase != PhasePlacement || s.Turn != White {
		t.Fatalf("new game should start in placement, White to place; got phase=%v turn=%v", s.Phase, s.Turn)
	}
	pts := AllPoints()
	wantTurn := White
	for i := 0; i < 10; i++ {
		if s.Turn != wantTurn {
			t.Fatalf("placement %d: turn = %v, want %v", i, s.Turn, wantTurn)
		}
		if !s.PlaceRing(pts[i]) {
			t.Fatalf("placement %d at %v should be legal", i, pts[i])
		}
		wantTurn = wantTurn.Opponent()
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("after 10 placements phase should be Playing, got %v", s.Phase)
	}
	if s.Board.RingCount(Black) != 5 || s.Board.RingCount(White) != 5 {
		t.Fatalf("each side should have exactly 5 rings, got B=%d W=%d", s.Board.RingCount(Black), s.Board.RingCount(White))
	}
	// White placed first (per spec) and the phases share one alternating
	// turn counter, so White also makes the first ring-move.
	if s.Turn != White {
		t.Fatalf("White should move first after placement, got %v", s.Turn)
	}
	// Placing on an occupied point is illegal.
	if s.PlaceRing(pts[0]) {
		t.Fatalf("re-placing on an occupied point must be rejected")
	}
}

// TestRingRemovalReducesMobilityIsExplicit checks the ring-removal API
// itself: RemoveRingChoice requires an explicit choice of ring (any of the
// claiming side's own rings), is rejected for a non-owned point, and after
// resolving hands the turn to the other side when no further rows remain.
func TestRingRemovalIsExplicitChoice(t *testing.T) {
	s := freshPlayingGame()
	line := lineOf(AxisA, 5)
	for _, p := range line {
		s.Board.Markers[p] = Black
	}
	s.Turn = Black
	s.beginResolution(Black)
	if s.Phase != PhaseRowPending || s.PendingSide != Black {
		t.Fatalf("completed row should enter RowPending for Black, got phase=%v side=%v", s.Phase, s.PendingSide)
	}
	if len(s.PendingWindow) != 5 {
		t.Fatalf("a run of exactly 5 should auto-fix the window, got %v", s.PendingWindow)
	}

	otherRing := Point{5, -4, -1} // an arbitrary point away from the row
	s.Board.Rings[otherRing] = Black
	// Choosing a ring NOT owned by the claiming side must be rejected.
	someWhiteRing := Point{-5, 4, 1}
	s.Board.Rings[someWhiteRing] = White
	if s.RemoveRingChoice(someWhiteRing) {
		t.Fatalf("must not be able to remove an opponent's ring to claim your own row")
	}
	if s.Removed[Black] != 0 {
		t.Fatalf("a rejected removal must not change the score")
	}

	if !s.RemoveRingChoice(otherRing) {
		t.Fatalf("removing any of the claiming side's own rings must be accepted")
	}
	if s.Removed[Black] != 1 {
		t.Fatalf("Removed[Black] = %d, want 1", s.Removed[Black])
	}
	for _, p := range line {
		if s.Board.HasMarker(p) {
			t.Fatalf("row markers must be cleared after resolution")
		}
	}
	if s.Board.HasRing(otherRing) {
		t.Fatalf("the chosen ring must be removed from the board")
	}
	if s.Phase != PhasePlaying || s.Turn != White {
		t.Fatalf("after resolving the only pending row, turn should pass to White; got phase=%v turn=%v", s.Phase, s.Turn)
	}
}

// TestBothSidesRowMoverResolvesFirst is a defensive/robustness test for a
// position that cannot arise from a single legal move in this engine (a
// move only ever changes markers to the MOVER's color, so in normal play
// only the mover can complete a fresh row) but which the spec explicitly
// asks to define and test the tie-break for: if both sides have a
// completed row present at once, the mover resolves theirs first.
func TestBothSidesRowMoverResolvesFirst(t *testing.T) {
	s := freshPlayingGame()
	blackLine := lineOf(AxisA, 5)
	whiteLine := lineOf(AxisB, 5)
	for _, p := range blackLine {
		s.Board.Markers[p] = Black
	}
	for _, p := range whiteLine {
		// Make sure we didn't accidentally reuse a Black point.
		if s.Board.Markers[p] == Black {
			t.Fatal("test setup: axis lines overlap, pick different fixtures")
		}
		s.Board.Markers[p] = White
	}

	s.beginResolution(Black) // Black is the mover
	if s.Phase != PhaseRowPending || s.PendingSide != Black {
		t.Fatalf("mover (Black) must resolve their own row first, got side=%v", s.PendingSide)
	}
	// Resolve Black's row.
	ring := Point{5, -4, -1}
	s.Board.Rings[ring] = Black
	if !s.RemoveRingChoice(ring) {
		t.Fatal("Black's row resolution should succeed")
	}
	// Now White's (pre-existing) row should be offered.
	if s.Phase != PhaseRowPending || s.PendingSide != White {
		t.Fatalf("after the mover's row is resolved, the opponent's completed row must be offered next; got phase=%v side=%v", s.Phase, s.PendingSide)
	}
	whiteRing := Point{-5, 4, 1}
	s.Board.Rings[whiteRing] = White
	if !s.RemoveRingChoice(whiteRing) {
		t.Fatal("White's row resolution should succeed")
	}
	if s.Phase != PhasePlaying || s.Turn != White {
		t.Fatalf("after both rows resolve, turn should pass to the mover's opponent (White); got phase=%v turn=%v", s.Phase, s.Turn)
	}
}

// TestThreeRingsWin: removing a 3rd ring ends the game with that side as
// the winner, regardless of who moves next.
func TestThreeRingsWin(t *testing.T) {
	s := freshPlayingGame()
	s.Removed[Black] = 2
	line := lineOf(AxisC, 5)
	for _, p := range line {
		s.Board.Markers[p] = Black
	}
	ring := Point{5, -4, -1}
	s.Board.Rings[ring] = Black
	s.beginResolution(Black)
	if !s.RemoveRingChoice(ring) {
		t.Fatal("winning row resolution should succeed")
	}
	if s.Phase != PhaseDone {
		t.Fatalf("Phase should be Done at 3 removed rings, got %v", s.Phase)
	}
	if s.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black", s.Winner())
	}
}

// TestPlayAppliesMoveAndEntersRowPending exercises the normal Play() path:
// a legal ring move through Play both applies the move (matching a direct
// ApplyRingMove) and, when it completes a row, enters PhaseRowPending
// instead of silently auto-resolving.
func TestPlayAppliesMoveAndEntersRowPending(t *testing.T) {
	s := freshPlayingGame()
	from := Point{-4, 0, 4}
	to := Point{0, 0, 0}
	s.Board.Rings[from] = Black
	s.Board.Markers[Point{-3, 0, 3}] = White
	s.Board.Markers[Point{-2, 0, 2}] = Black
	s.Board.Markers[Point{-1, 0, 1}] = White
	s.Turn = Black

	if !s.Play(from, to) {
		t.Fatal("legal move should be accepted by Play")
	}
	if s.Board.Rings[to] != Black || s.Board.HasRing(from) {
		t.Fatal("Play should have moved the ring")
	}
	if len(s.LastFlipped) != 3 {
		t.Fatalf("LastFlipped = %v, want 3 points", s.LastFlipped)
	}

	// An illegal move must be rejected without changing state.
	before := *s.Board
	if s.Play(Point{100, 100, 100}, Point{0, 0, 0}) {
		t.Fatal("an illegal move must be rejected")
	}
	_ = before
}

func freshPlayingGame() *GameState {
	s := NewGame(OpponentHotseat)
	s.Phase = PhasePlaying
	s.placed = 10
	return s
}
