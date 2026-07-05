package game

import "testing"

func TestAllPointsCount(t *testing.T) {
	// A full hexagon of cube-radius R has 3R^2+3R+1 cells (unlike YINSH's
	// truncated 85-point board, Six's v1 field keeps every one of them).
	want := 3*Radius*Radius + 3*Radius + 1
	got := AllPoints()
	if len(got) != want {
		t.Fatalf("AllPoints() = %d points, want %d (3*%d^2+3*%d+1)", len(got), want, Radius, Radius)
	}
	for _, p := range got {
		if p.X+p.Y+p.Z != 0 {
			t.Fatalf("point %v does not satisfy x+y+z=0", p)
		}
		if !InBoard(p) {
			t.Fatalf("AllPoints returned an out-of-board point %v", p)
		}
	}
}

func TestInBoardBoundary(t *testing.T) {
	if !InBoard(Hex{Radius, -Radius, 0}) {
		t.Fatal("a point exactly at radius Radius should be in-board")
	}
	if InBoard(Hex{Radius + 1, -Radius - 1, 0}) {
		t.Fatal("a point at radius Radius+1 should be out of board")
	}
	if InBoard(Hex{1, 1, 1}) {
		t.Fatal("a point violating x+y+z=0 must never be in-board")
	}
}

func TestNeighborsCenterHasSix(t *testing.T) {
	n := Neighbors(Hex{0, 0, 0})
	if len(n) != 6 {
		t.Fatalf("center should have 6 neighbours, got %d: %v", len(n), n)
	}
}

func TestNeighborsCornerHasThree(t *testing.T) {
	// A true corner of the hexagon (two coords simultaneously at the max
	// radius) only has 3 in-board neighbours.
	corner := Hex{Radius, -Radius, 0}
	n := Neighbors(corner)
	if len(n) != 3 {
		t.Fatalf("corner %v should have 3 neighbours, got %d: %v", corner, len(n), n)
	}
}

func TestOnEdge(t *testing.T) {
	if !OnEdge(Hex{Radius, 0, -Radius}) {
		t.Fatal("a radius-Radius point should be OnEdge")
	}
	if OnEdge(Hex{0, 0, 0}) {
		t.Fatal("the center should not be OnEdge")
	}
	if OnEdge(Hex{Radius - 1, 1 - Radius, 0}) {
		t.Fatal("a point strictly inside the board should not be OnEdge")
	}
}

// rotate60, applied 6 times, must return to the identity (verified via
// shapes.go's triangle-orientation construction, but checked directly here
// too since it underpins every shape-orientation test).
func TestRotate60SixCycle(t *testing.T) {
	p := Hex{2, -1, -1}
	cur := p
	for i := 0; i < 6; i++ {
		cur = rotate60(cur)
		if cur.X+cur.Y+cur.Z != 0 {
			t.Fatalf("rotate60 step %d broke the cube invariant: %v", i, cur)
		}
	}
	if cur != p {
		t.Fatalf("rotate60 applied 6 times should be identity, got %v want %v", cur, p)
	}
	// Also confirm it cycles Directions onto Directions (so the rotation
	// is a genuine symmetry of this hex grid, not just of the cube
	// constraint).
	for _, d := range Directions {
		r := rotate60(d)
		found := false
		for _, d2 := range Directions {
			if r == d2 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("rotate60(%v) = %v is not one of the 6 Directions", d, r)
		}
	}
}
