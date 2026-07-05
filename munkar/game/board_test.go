package game

import (
	"image"
	"testing"
)

func TestRotateOrientIsAnInvolutionPairSwap(t *testing.T) {
	// Rotating 90 degrees swaps H<->V and D1<->D2; rotating twice more
	// (180 total, applied as two 90s) returns to the original.
	for _, o := range []Orient{OrientH, OrientV, OrientD1, OrientD2} {
		if got := rotateOrient(rotateOrient(o)); got != o {
			t.Errorf("rotateOrient twice should return to %v, got %v", o, got)
		}
	}
	if rotateOrient(OrientH) != OrientV || rotateOrient(OrientV) != OrientH {
		t.Error("H and V should swap under a 90 degree rotation")
	}
	if rotateOrient(OrientD1) != OrientD2 || rotateOrient(OrientD2) != OrientD1 {
		t.Error("the two diagonals should swap under a 90 degree rotation")
	}
}

func TestRotateTileFourTimesIsIdentity(t *testing.T) {
	t4 := baseTile
	for i := 0; i < 4; i++ {
		t4 = rotateTile(t4)
	}
	if t4 != baseTile {
		t.Fatalf("rotating the tile 4 times should return the original, got %v", t4)
	}
}

// constRand always returns the same Intn result — a trivial deterministic
// stand-in for *math/rand.Rand so board construction can be tested without
// pulling in math/rand here.
type constRand struct{ n int }

func (r constRand) Intn(n int) int { return r.n % n }

func TestNewBoardFillsAllFourQuadrants(t *testing.T) {
	b := NewBoard(constRand{0})
	// Every cell must have been assigned a valid orientation (i.e. every
	// quadrant was actually written, not left at some zero-value gap) and
	// the whole board must start empty.
	seen := map[Orient]int{}
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			seen[b.Line[y][x]]++
			if b.Ring[y][x] != Empty {
				t.Fatalf("new board should start all-empty, (%d,%d) is %v", x, y, b.Ring[y][x])
			}
		}
	}
	// With rotation 0 on every quadrant (constRand{0}), every quadrant is an
	// identical copy of baseTile, so counts should be exactly 4x the base
	// tile's own per-orientation counts.
	base := map[Orient]int{}
	for _, row := range baseTile {
		for _, o := range row {
			base[o]++
		}
	}
	for o, n := range base {
		if seen[o] != n*4 {
			t.Errorf("orientation %v: got %d cells, want %d", o, seen[o], n*4)
		}
	}
}

func TestNewBoardHonorsQuadrantRotation(t *testing.T) {
	// A rotation index of 1 on every quadrant should place rotateTile(baseTile)
	// (not baseTile itself) in each of the four 3x3 blocks.
	b := NewBoard(constRand{1})
	want := rotateTile(baseTile)
	for _, q := range quadrantOrigins {
		for y := 0; y < 3; y++ {
			for x := 0; x < 3; x++ {
				if got := b.Line[q.Y+y][q.X+x]; got != want[y][x] {
					t.Fatalf("quadrant at %v cell (%d,%d): got %v, want %v", q, x, y, got, want[y][x])
				}
			}
		}
	}
}

// --- direction-forcing (ForcedCells) ----------------------------------------

func TestForcedCellsHorizontal(t *testing.T) {
	var b Board
	b.Line[2][3] = OrientH
	b.Ring[2][3] = Black
	b.Ring[2][0] = White // occupied — must be excluded from the forced set
	got := ForcedCells(b, image.Pt(3, 2))
	want := map[image.Point]bool{
		{X: 1, Y: 2}: true, {X: 2, Y: 2}: true,
		{X: 4, Y: 2}: true, {X: 5, Y: 2}: true,
	}
	if len(got) != len(want) {
		t.Fatalf("ForcedCells(H) = %v, want exactly %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected forced cell %v", p)
		}
		if p.Y != 2 {
			t.Errorf("forced cell %v not on row 2", p)
		}
	}
}

func TestForcedCellsVertical(t *testing.T) {
	var b Board
	b.Line[1][4] = OrientV
	b.Ring[1][4] = Black
	got := ForcedCells(b, image.Pt(4, 1))
	if len(got) != 5 {
		t.Fatalf("ForcedCells(V) with only the placed cell occupied = %v, want 5 empty cells", got)
	}
	for _, p := range got {
		if p.X != 4 {
			t.Errorf("forced cell %v not on column 4", p)
		}
	}
}

// TestForcedCellsDiagonalRising covers "╱" (OrientD1): x+y constant.
func TestForcedCellsDiagonalRising(t *testing.T) {
	var b Board
	b.Line[3][2] = OrientD1 // placed at (2,3): x+y=5
	b.Ring[3][2] = Black
	got := ForcedCells(b, image.Pt(2, 3))
	want := map[image.Point]bool{
		{X: 0, Y: 5}: true, {X: 1, Y: 4}: true,
		{X: 3, Y: 2}: true, {X: 4, Y: 1}: true, {X: 5, Y: 0}: true,
	}
	if len(got) != len(want) {
		t.Fatalf("ForcedCells(╱) = %v, want exactly %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected forced cell %v (x+y=%d, want 5)", p, p.X+p.Y)
		}
	}
}

// TestForcedCellsDiagonalFalling covers "╲" (OrientD2): x-y constant.
func TestForcedCellsDiagonalFalling(t *testing.T) {
	var b Board
	b.Line[1][3] = OrientD2 // placed at (3,1): x-y=2
	b.Ring[1][3] = Black
	got := ForcedCells(b, image.Pt(3, 1))
	want := map[image.Point]bool{
		{X: 2, Y: 0}: true, {X: 4, Y: 2}: true,
		{X: 5, Y: 3}: true,
	}
	if len(got) != len(want) {
		t.Fatalf("ForcedCells(╲) = %v, want exactly %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected forced cell %v (x-y=%d, want 2)", p, p.X-p.Y)
		}
	}
}

// GOTCHA: a fully-occupied forced line means "play anywhere" — the
// direction-forcing rule's fallback, checked directly against ForcedCells
// and via the higher-level LegalMoves.
func TestForcedCellsLineFullFallsBackToAnywhere(t *testing.T) {
	var b Board
	for x := 0; x < Size; x++ {
		b.Line[2][x] = OrientH
	}
	// Fill the entire row 2 (both colors, doesn't matter which).
	for x := 0; x < Size; x++ {
		c := Black
		if x%2 == 1 {
			c = White
		}
		b.Ring[2][x] = c
	}
	if got := ForcedCells(b, image.Pt(3, 2)); len(got) != 0 {
		t.Fatalf("a fully-occupied forced line should yield no forced cells, got %v", got)
	}

	// LegalMoves must then fall back to every empty cell elsewhere on the
	// board (not an empty list, and not restricted to row 2). Fill
	// everything outside row 2 except one cell, to make the fallback set
	// easy to check exactly.
	for y := 0; y < Size; y++ {
		if y == 2 {
			continue
		}
		for x := 0; x < Size; x++ {
			b.Ring[y][x] = Black
		}
	}
	b.Ring[5][5] = Empty
	moves := LegalMoves(b, image.Pt(3, 2), true)
	if len(moves) != 1 || moves[0] != (image.Point{X: 5, Y: 5}) {
		t.Fatalf("LegalMoves fallback = %v, want exactly [(5,5)]", moves)
	}
}

// The very first move of a game (hasLast=false) is always unconstrained,
// regardless of board contents.
func TestLegalMovesFirstMoveIsUnconstrained(t *testing.T) {
	var b Board
	b.Ring[0][0] = Black // one occupied cell; rest empty
	moves := LegalMoves(b, image.Pt(0, 0), false)
	if len(moves) != Size*Size-1 {
		t.Fatalf("first move should allow all %d empty cells, got %d", Size*Size-1, len(moves))
	}
}
