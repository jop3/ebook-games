package game

import (
	"image"
	"testing"
)

func TestNewBoardIsLegal(t *testing.T) {
	b := NewBoard()
	if n := b.Count(Black); n != 4 {
		t.Fatalf("Black has %d cells, want 4", n)
	}
	if n := b.Count(White); n != 4 {
		t.Fatalf("White has %d cells, want 4", n)
	}
	if n := b.Count(Neutral); n != 2 {
		t.Fatalf("Neutral has %d cells, want 2", n)
	}
	if n := b.Count(Empty); n != 6 {
		t.Fatalf("Empty has %d cells, want 6 (16-4-4-2)", n)
	}

	// Both L-piece cell sets must exactly match one of the 8 valid
	// orientations (translated) - i.e. the starting layout isn't an
	// arbitrary/invalid tetromino shape.
	for _, side := range []Cell{Black, White} {
		cells := currentLCells(&b, side)
		if !isValidLShape(cells) {
			t.Errorf("%v's starting cells %v do not form a valid L-tetromino orientation", side, cells)
		}
	}
}

// isValidLShape reports whether cells, once normalized (shifted so its
// bounding box starts at 0,0) and sorted, matches one of the 8 orientations.
func isValidLShape(cells [4]image.Point) bool {
	pts := make([]image.Point, 4)
	copy(pts, cells[:])
	norm := normalizeShape(pts)
	var key [4]image.Point
	copy(key[:], norm)
	for _, shape := range LOrientations {
		var k [4]image.Point
		copy(k[:], shape)
		if k == key {
			return true
		}
	}
	return false
}

// TestNewBoardHasNoImmediateForcedWin guards the starting position's
// balance: neither side should be able to trap the other in a single
// opening full move (mandatory L-placement + optional neutral move). An
// earlier, more wall-hugging candidate starting layout failed exactly this
// check during development (Black had a first move that immediately left
// White with zero legal L-placements) and was replaced.
func TestNewBoardHasNoImmediateForcedWin(t *testing.T) {
	b := NewBoard()
	for _, fm := range GenerateFullMoves(b, Black) {
		if len(LegalLPlacements(fm.Result, White)) == 0 {
			t.Fatalf("Black has a first move %+v that immediately traps White - starting position is unbalanced", fm.L)
		}
	}
	for _, fm := range GenerateFullMoves(b, White) {
		if len(LegalLPlacements(fm.Result, Black)) == 0 {
			t.Fatalf("White has a first move %+v that immediately traps Black - starting position is unbalanced", fm.L)
		}
	}
}

func TestBoardAtOutOfBounds(t *testing.T) {
	b := NewBoard()
	if c := b.At(-1, 0); c != Empty {
		t.Errorf("At(-1,0) = %v, want Empty", c)
	}
	if c := b.At(4, 0); c != Empty {
		t.Errorf("At(4,0) = %v, want Empty", c)
	}
	if c := b.At(0, 99); c != Empty {
		t.Errorf("At(0,99) = %v, want Empty", c)
	}
}

func TestCellOpponent(t *testing.T) {
	if Black.Opponent() != White {
		t.Error("Black.Opponent() should be White")
	}
	if White.Opponent() != Black {
		t.Error("White.Opponent() should be Black")
	}
	if Empty.Opponent() != Empty {
		t.Error("Empty.Opponent() should be Empty (not meaningful, but shouldn't panic/crash)")
	}
}
