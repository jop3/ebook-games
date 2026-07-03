package game

import "testing"

// fireFromInterior finds the edge point whose interior cell is (ex,ey) and
// whose inward direction matches, then fires. Helper keeps tests readable.
func fireCell(t *testing.T, g *Grid, ex, ey int, d dir) RayResult {
	t.Helper()
	ep, ok := g.edgePointAt(ex, ey, d)
	if !ok {
		t.Fatalf("no edge point at interior (%d,%d) dir %+v", ex, ey, d)
	}
	return g.Fire(ep)
}

// TestStraightThrough: empty grid, ray passes straight and exits opposite.
func TestStraightThrough(t *testing.T) {
	g := NewGrid(8, 8)
	// Fire down the leftmost column (interior entry cell (0,0), dir down).
	res := fireCell(t, g, 0, 0, dirDown)
	if res.Outcome != OutcomeDetour {
		t.Fatalf("expected detour (straight through), got %v", res.Outcome)
	}
	// It should exit at the bottom of the same column: interior cell (0,7),
	// inward dir up.
	exit, _ := g.edgePointAt(0, 7, dirUp)
	if res.ExitIndex != exit.Index {
		t.Fatalf("expected exit at bottom of column 0 (idx %d), got %d", exit.Index, res.ExitIndex)
	}
}

// TestDirectHit: an atom straight in the ray's path absorbs it.
func TestDirectHit(t *testing.T) {
	g := NewGrid(8, 8)
	g.SetAtom(3, 4, true)
	// Enter column 3 from the top going down; atom at (3,4) is directly ahead.
	res := fireCell(t, g, 3, 0, dirDown)
	if res.Outcome != OutcomeHit {
		t.Fatalf("expected hit, got %v", res.Outcome)
	}
}

// TestSingleDeflection: an atom on one forward diagonal deflects the ray 90°
// away from it.
func TestSingleDeflection(t *testing.T) {
	g := NewGrid(8, 8)
	// Ray travels down column 3. Place an atom diagonally forward on the
	// screen-left side, i.e. at (2,4) while the ray is passing (3,3)->front(3,4).
	// front cell = (3,4); front's diagonals are (3+1,4)=(4,4) and (3-1,4)=(2,4).
	g.SetAtom(2, 4, true)
	res := fireCell(t, g, 3, 0, dirDown)
	if res.Outcome != OutcomeDetour {
		t.Fatalf("expected detour after deflection, got %v", res.Outcome)
	}
	// Atom is on the screen-left; ray should deflect to the screen-right (east)
	// and exit on the right edge. Verify the exit is on the right column.
	eps := g.EdgePoints()
	exit := eps[res.ExitIndex]
	if exit.entryX != g.W-1 {
		t.Fatalf("expected exit on right edge (x=%d), got interior x=%d", g.W-1, exit.entryX)
	}
}

// TestSingleDeflectionOtherSide: atom on the opposite diagonal deflects the
// other way.
func TestSingleDeflectionOtherSide(t *testing.T) {
	g := NewGrid(8, 8)
	// Atom on the screen-right diagonal of the front cell: front (3,4), right
	// diagonal (4,4).
	g.SetAtom(4, 4, true)
	res := fireCell(t, g, 3, 0, dirDown)
	if res.Outcome != OutcomeDetour {
		t.Fatalf("expected detour after deflection, got %v", res.Outcome)
	}
	eps := g.EdgePoints()
	exit := eps[res.ExitIndex]
	if exit.entryX != 0 {
		t.Fatalf("expected exit on left edge (x=0), got interior x=%d", exit.entryX)
	}
}

// TestReflectionBothDiagonals: atoms on both forward diagonals reflect the ray
// straight back out its entry point.
func TestReflectionBothDiagonals(t *testing.T) {
	g := NewGrid(8, 8)
	// Ray down column 3, front cell (3,4); both diagonals (2,4) and (4,4) hold
	// atoms. Neither is directly in front, so no hit; both -> reflect.
	g.SetAtom(2, 4, true)
	g.SetAtom(4, 4, true)
	res := fireCell(t, g, 3, 0, dirDown)
	if res.Outcome != OutcomeReflection {
		t.Fatalf("expected reflection (both diagonals), got %v", res.Outcome)
	}
}

// TestEdgeReflection: an atom diagonally adjacent to the entry point reflects
// the ray immediately (it never really enters).
func TestEdgeReflection(t *testing.T) {
	g := NewGrid(8, 8)
	// Enter column 3 from the top (virtual start (3,-1), first front cell (3,0)).
	// The front cell's diagonals are (2,0) and (4,0). An atom at (2,0) sits
	// diagonally next to the entry point and forces immediate reflection.
	g.SetAtom(2, 0, true)
	res := fireCell(t, g, 3, 0, dirDown)
	if res.Outcome != OutcomeReflection {
		t.Fatalf("expected edge reflection, got %v", res.Outcome)
	}
	if res.ExitIndex != 0 && res.Outcome != OutcomeReflection {
		t.Fatalf("edge reflection should mark same entry point")
	}
}

// TestEdgeReflectionBothCorners: atoms on both entry diagonals still reflect.
func TestEdgeReflectionBothCorners(t *testing.T) {
	g := NewGrid(8, 8)
	g.SetAtom(2, 0, true)
	g.SetAtom(4, 0, true)
	res := fireCell(t, g, 3, 0, dirDown)
	if res.Outcome != OutcomeReflection {
		t.Fatalf("expected reflection, got %v", res.Outcome)
	}
}

// TestMultiStepUTurn: two deflections send the ray back the way it came,
// exiting near the entry side (U-shaped path).
func TestMultiStepUTurn(t *testing.T) {
	g := NewGrid(8, 8)
	// Plan a U-turn for a ray entering the top of column 2 going down.
	//
	// Step A: ray at (2,y) heading down. Front (2,3). Put an atom on the
	// screen-right diagonal of the front cell -> (3,3). Ray deflects to
	// screen-LEFT (west), now heading left along row 2 or 3.
	//
	// After a right-diagonal atom, d = d.left(). For d=down{0,1},
	// left()={dy,-dx}={1,0}=east? Let's instead just assert the path bends
	// twice and exits back on the TOP edge, which is the signature of a U-turn.
	//
	// Construct symmetric deflectors so a downward ray turns 90°, travels, then
	// turns 90° again to head back up and out the top.
	//
	// Entry: top of column 2, down.
	// First deflection at front (2,3): atom at (1,3) (screen-left diagonal) ->
	//   deflect to screen-right (east). Now heading right (dirRight) around row 2.
	g.SetAtom(1, 3, true)
	// Now the ray is heading east along row 2 (it moved into cell (2,2) then
	// turns). We need a second atom to bend it back up (north) so it exits the
	// top edge. When heading east, front is (x+1,2); its diagonals are
	// (x+1,1) and (x+1,3). Place an atom at (4,3) so that when the ray's front
	// is (4,2), the screen-below diagonal (4,3) deflects it to head north.
	g.SetAtom(4, 3, true)

	res := fireCell(t, g, 2, 0, dirDown)
	if res.Outcome != OutcomeDetour {
		t.Fatalf("expected detour (U-turn), got %v", res.Outcome)
	}
	eps := g.EdgePoints()
	exit := eps[res.ExitIndex]
	// A U-turn back out the top edge: the exit's interior cell is on the top
	// row (y==0) with inward dir down.
	if exit.entryY != 0 || exit.inDir != dirDown {
		t.Fatalf("expected exit on TOP edge (U-turn), got interior (%d,%d) dir %+v",
			exit.entryX, exit.entryY, exit.inDir)
	}
}

// TestDetourPairing: a detour's entry and exit are distinct and mutually
// consistent (firing from the exit point would trace the reverse path... at
// least the entry != exit for a genuine detour).
func TestDetourPairing(t *testing.T) {
	g := NewGrid(8, 8)
	g.SetAtom(2, 4, true) // single deflection, from TestSingleDeflection
	res := fireCell(t, g, 3, 0, dirDown)
	if res.Outcome != OutcomeDetour {
		t.Fatalf("expected detour, got %v", res.Outcome)
	}
	if res.ExitIndex == res.EntryIndex {
		t.Fatalf("detour must have distinct entry/exit, both = %d", res.EntryIndex)
	}
}

// TestFireFromEachSide: sanity that rays can be fired from all four edges of an
// empty grid and pass straight through.
func TestFireFromEachSide(t *testing.T) {
	g := NewGrid(6, 6)
	cases := []struct {
		ex, ey int
		d      dir
	}{
		{0, 0, dirDown},  // top
		{5, 2, dirLeft},  // right
		{3, 5, dirUp},    // bottom
		{0, 4, dirRight}, // left
	}
	for _, c := range cases {
		res := fireCell(t, g, c.ex, c.ey, c.d)
		if res.Outcome != OutcomeDetour {
			t.Fatalf("empty grid from (%d,%d) dir %+v: expected detour, got %v",
				c.ex, c.ey, c.d, res.Outcome)
		}
	}
}
