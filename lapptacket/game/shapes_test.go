package game

import (
	"image"
	"testing"
)

func TestOrientationsDedupesSymmetricShapes(t *testing.T) {
	// A 2x2 square has full 4-fold + reflective symmetry: only 1 distinct
	// orientation.
	if got := len(Orientations(shO4)); got != 1 {
		t.Fatalf("square orientations = %d, want 1", got)
	}
	// A straight domino has 2 distinct orientations (horizontal/vertical);
	// reflection doesn't add more.
	if got := len(Orientations(shDomino)); got != 2 {
		t.Fatalf("domino orientations = %d, want 2", got)
	}
	// An L-tromino (a 2x2 square minus one corner) is symmetric under
	// reflection — each mirrored form coincides with one of the 4
	// rotations — so it has only 4 distinct orientations, not 8.
	if got := len(Orientations(shL3)); got != 4 {
		t.Fatalf("L-tromino orientations = %d, want 4", got)
	}
}

func TestAllPatchesAreConnectedPolyominoes(t *testing.T) {
	for _, p := range Patches {
		if !isConnected(p.Cells) {
			t.Errorf("patch %d (%s) is not a connected polyomino: %v", p.ID, p.Name, p.Cells)
		}
	}
}

// isConnected does a flood fill over the offsets (treated as a set of unit
// cells) and checks every cell is reachable from the first.
func isConnected(cells []Offset) bool {
	if len(cells) == 0 {
		return true
	}
	set := map[Offset]bool{}
	for _, c := range cells {
		set[c] = true
	}
	seen := map[Offset]bool{cells[0]: true}
	stack := []Offset{cells[0]}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range []Offset{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := Offset{cur[0] + d[0], cur[1] + d[1]}
			if set[n] && !seen[n] {
				seen[n] = true
				stack = append(stack, n)
			}
		}
	}
	return len(seen) == len(cells)
}

func TestCanPlaceAtRespectsBoundsAndOccupancy(t *testing.T) {
	var b Board
	oriented := Orientations(shO4)[0] // 2x2 square, normalized to {(0,0),(1,0),(0,1),(1,1)}

	// Fits in the corner.
	cells, ok := canPlaceAt(&b, oriented, image.Pt(0, 0))
	if !ok || len(cells) != 4 {
		t.Fatalf("expected the square to fit at (0,0), got ok=%v cells=%v", ok, cells)
	}
	b.Place(cells)

	// Now overlapping the same corner must fail.
	if _, ok := canPlaceAt(&b, oriented, image.Pt(0, 0)); ok {
		t.Fatal("expected overlap at (0,0) to be rejected")
	}

	// Off the bottom-right edge must fail (board is 9x9, indices 0..8).
	if _, ok := canPlaceAt(&b, oriented, image.Pt(8, 8)); ok {
		t.Fatal("expected placement running off the board edge to be rejected")
	}

	// Elsewhere still fits.
	if _, ok := canPlaceAt(&b, oriented, image.Pt(4, 4)); !ok {
		t.Fatal("expected the square to fit at (4,4)")
	}
}

func TestHasLegalPlacementOnFullBoard(t *testing.T) {
	var b Board
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			b.Filled[y][x] = true
		}
	}
	if HasLegalPlacement(&b, Patches[0]) {
		t.Fatal("expected no legal placement on a completely full board")
	}
}
