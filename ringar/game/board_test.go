package game

import "testing"

func contains(pts []Point, p Point) bool {
	for _, q := range pts {
		if q == p {
			return true
		}
	}
	return false
}

// TestRingMoveZeroMarkers: the trivial case — a ring may slide any distance
// over a stretch of empty points, with nothing to jump.
func TestRingMoveZeroMarkers(t *testing.T) {
	b := NewBoard()
	from := Point{0, 0, 0}
	b.Rings[from] = Black
	moves := RingMoves(b, from)
	// Along axis A direction (1,-1,0): (1,-1,0),(2,-2,0),(3,-3,0),(4,-4,0),(5,-5,... wait
	// (5,-5,0) is a corner and excluded, so this ray should stop at (4,-4,0).
	want := []Point{{1, -1, 0}, {2, -2, 0}, {3, -3, 0}, {4, -4, 0}}
	for _, p := range want {
		if !contains(moves, p) {
			t.Errorf("expected empty-stretch destination %v, got %v", p, moves)
		}
	}
	if contains(moves, Point{5, -5, 0}) {
		t.Errorf("corner point must never be a legal destination: %v", moves)
	}
}

// TestRingMoveJumpOneMarker: exactly one marker jumped, landing on the very
// next empty point — not "any" empty point further along.
func TestRingMoveJumpOneMarker(t *testing.T) {
	b := NewBoard()
	from := Point{-3, 0, 3}
	to := Point{-2, 0, 2}     // marker here
	beyond := Point{-1, 0, 1} // must land here
	further := Point{0, 0, 0}
	b.Rings[from] = Black
	b.Markers[to] = White
	moves := RingMoves(b, from)
	if contains(moves, to) {
		t.Fatalf("must never land ON the jumped marker: %v", moves)
	}
	if !contains(moves, beyond) {
		t.Fatalf("must land on the first empty point after the jumped marker, got %v", moves)
	}
	if contains(moves, further) {
		t.Fatalf("must NOT continue past the landing point in the same ray: %v", moves)
	}
}

// TestRingMoveJumpSeveralMarkers: several consecutive markers jumped in one
// move, landing exactly on the first empty point after all of them.
func TestRingMoveJumpSeveralMarkers(t *testing.T) {
	b := NewBoard()
	from := Point{-4, 0, 4}
	b.Rings[from] = Black
	// direction (1,0,-1): -3,0,3 / -2,0,2 / -1,0,1 all markers, then 0,0,0 empty.
	b.Markers[Point{-3, 0, 3}] = White
	b.Markers[Point{-2, 0, 2}] = Black
	b.Markers[Point{-1, 0, 1}] = White
	landing := Point{0, 0, 0}
	moves := RingMoves(b, from)
	if !contains(moves, landing) {
		t.Fatalf("must land right after jumping all 3 markers, got %v", moves)
	}
	for _, mid := range []Point{{-3, 0, 3}, {-2, 0, 2}, {-1, 0, 1}} {
		if contains(moves, mid) {
			t.Fatalf("must not be able to land mid-jump on %v", mid)
		}
	}
}

// TestRingMoveBlockedByRing: a ring (either color) blocks the ray outright —
// the ring cannot be jumped, landed on, or seen past.
func TestRingMoveBlockedByRing(t *testing.T) {
	b := NewBoard()
	from := Point{-4, 4, 0}
	blocker := Point{-3, 3, 0}
	beyond := Point{-2, 2, 0}
	b.Rings[from] = Black
	b.Rings[blocker] = White
	moves := RingMoves(b, from)
	if contains(moves, blocker) {
		t.Fatalf("must never land on a ring: %v", moves)
	}
	if contains(moves, beyond) {
		t.Fatalf("a ring must block the entire ray, including points beyond it: %v", moves)
	}

	// Same, but the ring sits AFTER a run of markers that would otherwise be
	// jumpable: the ring still blocks, the jump fails entirely.
	b2 := NewBoard()
	from2 := Point{-4, 0, 4}
	b2.Rings[from2] = Black
	b2.Markers[Point{-3, 0, 3}] = White
	b2.Rings[Point{-2, 0, 2}] = White // blocks right after the marker
	moves2 := RingMoves(b2, from2)
	if contains(moves2, Point{-2, 0, 2}) || contains(moves2, Point{-1, 0, 1}) {
		t.Fatalf("a ring immediately after a jumped marker must still block the ray: %v", moves2)
	}
}

// TestApplyRingMoveFlipsExactlyJumpedMarkers verifies ApplyRingMove drops a
// marker at `from`, flips only the markers strictly between from and to, and
// moves the ring — nothing else on the board changes.
func TestApplyRingMoveFlipsExactlyJumpedMarkers(t *testing.T) {
	b := NewBoard()
	from := Point{-4, 0, 4}
	to := Point{0, 0, 0}
	b.Rings[from] = Black
	b.Markers[Point{-3, 0, 3}] = White
	b.Markers[Point{-2, 0, 2}] = Black
	b.Markers[Point{-1, 0, 1}] = White
	// An unrelated marker elsewhere must be untouched.
	untouched := Point{5, -4, -1}
	b.Markers[untouched] = White

	flipped := ApplyRingMove(b, from, to)
	if len(flipped) != 3 {
		t.Fatalf("expected 3 flipped markers, got %d: %v", len(flipped), flipped)
	}
	if b.Markers[Point{-3, 0, 3}] != Black || b.Markers[Point{-2, 0, 2}] != White || b.Markers[Point{-1, 0, 1}] != Black {
		t.Fatalf("jumped markers were not all flipped: %v %v %v",
			b.Markers[Point{-3, 0, 3}], b.Markers[Point{-2, 0, 2}], b.Markers[Point{-1, 0, 1}])
	}
	if b.Markers[from] != Black {
		t.Fatalf("a marker of the mover's color must be dropped at `from`, got %v", b.Markers[from])
	}
	if b.HasRing(from) {
		t.Fatalf("the ring must have left `from`")
	}
	if b.Rings[to] != Black {
		t.Fatalf("the ring must now be at `to`")
	}
	if b.Markers[untouched] != White {
		t.Fatalf("an unrelated marker must not be touched by the move")
	}
}
