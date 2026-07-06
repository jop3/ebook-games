package game

import (
	"image"
	"testing"
)

func TestCanPlaceWallOpenBoard(t *testing.T) {
	b := NewBoard()
	if !CanPlaceWall(&b, Wall{X: 3, Y: 3, Orient: Horizontal}) {
		t.Fatal("an isolated wall on an open board should be legal")
	}
}

func TestCanPlaceWallOutOfBounds(t *testing.T) {
	b := NewBoard()
	cases := []Wall{
		{X: -1, Y: 0, Orient: Horizontal},
		{X: 0, Y: -1, Orient: Vertical},
		{X: WallGrid, Y: 0, Orient: Horizontal},
		{X: 0, Y: WallGrid, Orient: Vertical},
	}
	for _, w := range cases {
		if CanPlaceWall(&b, w) {
			t.Fatalf("out-of-bounds wall %v must be rejected", w)
		}
	}
}

func TestCanPlaceWallRejectsExactDuplicate(t *testing.T) {
	b := NewBoard()
	w := Wall{X: 3, Y: 3, Orient: Horizontal}
	b.place(w)
	if CanPlaceWall(&b, w) {
		t.Fatal("placing the exact same wall twice must be rejected")
	}
}

func TestCanPlaceWallRejectsAdjacentOverlap(t *testing.T) {
	b := NewBoard()
	b.place(Wall{X: 3, Y: 3, Orient: Horizontal})
	if CanPlaceWall(&b, Wall{X: 4, Y: 3, Orient: Horizontal}) {
		t.Fatal("a horizontal wall sharing an endpoint with an existing one must be rejected")
	}
	if CanPlaceWall(&b, Wall{X: 2, Y: 3, Orient: Horizontal}) {
		t.Fatal("a horizontal wall sharing the other endpoint must also be rejected")
	}
	// Not adjacent: legal.
	if !CanPlaceWall(&b, Wall{X: 5, Y: 3, Orient: Horizontal}) {
		t.Fatal("a non-overlapping horizontal wall on the same row should remain legal")
	}
}

func TestCanPlaceWallRejectsAdjacentOverlapVertical(t *testing.T) {
	b := NewBoard()
	b.place(Wall{X: 3, Y: 3, Orient: Vertical})
	if CanPlaceWall(&b, Wall{X: 3, Y: 4, Orient: Vertical}) {
		t.Fatal("a vertical wall sharing an endpoint with an existing one must be rejected")
	}
	if CanPlaceWall(&b, Wall{X: 3, Y: 2, Orient: Vertical}) {
		t.Fatal("a vertical wall sharing the other endpoint must also be rejected")
	}
}

func TestCanPlaceWallRejectsCrossing(t *testing.T) {
	b := NewBoard()
	b.place(Wall{X: 3, Y: 3, Orient: Horizontal})
	if CanPlaceWall(&b, Wall{X: 3, Y: 3, Orient: Vertical}) {
		t.Fatal("a vertical wall crossing a horizontal wall at the same anchor must be rejected")
	}

	b2 := NewBoard()
	b2.place(Wall{X: 5, Y: 5, Orient: Vertical})
	if CanPlaceWall(&b2, Wall{X: 5, Y: 5, Orient: Horizontal}) {
		t.Fatal("a horizontal wall crossing a vertical wall at the same anchor must be rejected")
	}
}

// TestCanPlaceWallRejectsTrappingP1 builds a small pocket around P1's pawn
// using 3 independent (non-overlapping, non-crossing) wall placements, then
// checks that closing the 4th and final side — which would leave P1 with
// zero path to its goal row — is rejected, even though that placement is
// perfectly legal in isolation (no overlap/crossing) on an otherwise open
// board.
func TestCanPlaceWallRejectsTrappingP1(t *testing.T) {
	b := NewBoard()
	b.Pawns[P1] = image.Pt(4, 4)
	b.Pawns[P2] = image.Pt(4, 0) // clear path to its own goal (row 8)

	top := Wall{X: 3, Y: 3, Orient: Horizontal}    // blocks (4,3)-(4,4)
	bottom := Wall{X: 4, Y: 4, Orient: Horizontal}  // blocks (4,4)-(4,5)
	left := Wall{X: 3, Y: 4, Orient: Vertical}      // blocks (3,4)-(4,4)
	right := Wall{X: 4, Y: 3, Orient: Vertical}     // blocks (4,4)-(5,4)

	for _, w := range []Wall{top, bottom, left} {
		if !CanPlaceWall(&b, w) {
			t.Fatalf("setup wall %v should still be legal (a path remains via the 4th side)", w)
		}
		b.place(w)
	}
	// Sanity: with 3 of 4 sides closed, P1 still has exactly one path out.
	if _, ok := BFSDistance(&b, b.Pawns[P1], GoalRow(P1)); !ok {
		t.Fatal("setup is wrong: P1 should still have a path through the open 4th side")
	}

	if CanPlaceWall(&b, right) {
		t.Fatal("closing the last side must be rejected: it would leave P1 with zero path to its goal")
	}
}

// TestCanPlaceWallRejectsTrappingP2 is the mirror image of the P1 case,
// confirming the dual (both-sides) check isn't accidentally one-sided.
func TestCanPlaceWallRejectsTrappingP2(t *testing.T) {
	b := NewBoard()
	b.Pawns[P2] = image.Pt(4, 4)
	b.Pawns[P1] = image.Pt(4, 8) // clear path to its own goal (row 0)

	top := Wall{X: 3, Y: 3, Orient: Horizontal}
	bottom := Wall{X: 4, Y: 4, Orient: Horizontal}
	left := Wall{X: 3, Y: 4, Orient: Vertical}
	right := Wall{X: 4, Y: 3, Orient: Vertical}

	for _, w := range []Wall{top, left, right} {
		if !CanPlaceWall(&b, w) {
			t.Fatalf("setup wall %v should still be legal", w)
		}
		b.place(w)
	}
	if CanPlaceWall(&b, bottom) {
		t.Fatal("closing the last side must be rejected: it would leave P2 with zero path to its goal")
	}
}

func TestCanPlaceWallAllowsNarrowingWithoutFullyBlocking(t *testing.T) {
	b := NewBoard()
	b.Pawns[P1] = image.Pt(4, 4)
	b.Pawns[P2] = image.Pt(4, 0)
	// Only 3 of 4 sides closed: a path remains (through "right"), so this
	// narrowing wall must remain legal even though it's the last one before
	// the trap.
	top := Wall{X: 3, Y: 3, Orient: Horizontal}
	left := Wall{X: 3, Y: 4, Orient: Vertical}
	if !CanPlaceWall(&b, top) {
		t.Fatal("top wall should be legal")
	}
	b.place(top)
	if !CanPlaceWall(&b, left) {
		t.Fatal("left wall should still be legal: a path remains via bottom/right")
	}
}

func TestBFSDistanceOpenBoard(t *testing.T) {
	b := NewBoard()
	d, ok := BFSDistance(&b, b.Pawns[P1], GoalRow(P1))
	if !ok || d != Size-1 {
		t.Fatalf("BFSDistance(P1 start) = %d,%v want %d,true", d, ok, Size-1)
	}
}

func TestBFSDistanceNoPath(t *testing.T) {
	var b Board
	b.Pawns[P1] = image.Pt(4, 4)
	// Fully enclose (4,4) with 4 non-conflicting walls.
	b.place(Wall{X: 3, Y: 3, Orient: Horizontal})
	b.place(Wall{X: 4, Y: 4, Orient: Horizontal})
	b.place(Wall{X: 3, Y: 4, Orient: Vertical})
	b.place(Wall{X: 4, Y: 3, Orient: Vertical})
	if _, ok := BFSDistance(&b, b.Pawns[P1], GoalRow(P1)); ok {
		t.Fatal("a fully enclosed pawn should have no path")
	}
}

func TestShortestPathReturnsWalkableRoute(t *testing.T) {
	b := NewBoard()
	path, ok := ShortestPath(&b, b.Pawns[P1], GoalRow(P1))
	if !ok {
		t.Fatal("shortest path should exist on an open board")
	}
	if path[0] != b.Pawns[P1] {
		t.Fatalf("path must start at the pawn, got %v", path[0])
	}
	if path[len(path)-1].Y != GoalRow(P1) {
		t.Fatalf("path must end on the goal row, got %v", path[len(path)-1])
	}
	for i := 0; i+1 < len(path); i++ {
		if b.wallBetween(path[i], path[i+1]) {
			t.Fatalf("path step %v -> %v is wall-blocked", path[i], path[i+1])
		}
	}
	if len(path)-1 != Size-1 {
		t.Fatalf("path length %d, want the direct %d-step distance", len(path)-1, Size-1)
	}
}
