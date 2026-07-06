package game

import (
	"image"
	"testing"
)

// clearBoard returns an empty board (helper for constructing test positions).
func clearBoard() Board {
	var b Board
	return b
}

func placeShape(b *Board, side Cell, orient int, anchor image.Point) {
	for _, off := range LOrientations[orient] {
		b.set(anchor.X+off.X, anchor.Y+off.Y, side)
	}
}

func TestLegalLPlacementsFromStart(t *testing.T) {
	b := NewBoard()
	moves := LegalLPlacements(b, Black)
	if len(moves) == 0 {
		t.Fatal("Black should have legal placements from the starting position")
	}
	// Every returned placement must actually be applicable: land only on
	// currently-empty-or-own cells, stay in bounds, and differ from the
	// current position.
	cur := currentLCells(&b, Black)
	for _, m := range moves {
		if m.Cells == cur {
			t.Errorf("a returned placement must differ from the current position: %v", m)
		}
		for _, c := range m.Cells {
			if c.X < 0 || c.X >= Size || c.Y < 0 || c.Y >= Size {
				t.Errorf("placement cell %v out of bounds", c)
			}
			v := b.At(c.X, c.Y)
			if v != Empty && v != Black {
				t.Errorf("placement %v overlaps a non-Black, non-empty cell %v (%v)", m, c, v)
			}
		}
	}
}

// TestMustMoveWhenAnyLegalPlacementExists is the spec's headline gotcha: a
// side with at least one legal L-placement must never be reported as having
// none (which would falsely end the game).
func TestMustMoveWhenAnyLegalPlacementExists(t *testing.T) {
	b := NewBoard()
	for _, side := range []Cell{Black, White} {
		if len(LegalLPlacements(b, side)) == 0 {
			t.Fatalf("%v should have legal placements on the starting board", side)
		}
		if _, over := Winner(b, side); over {
			t.Fatalf("%v should not be reported as having lost on the starting board", side)
		}
	}
}

// TestNoLegalPlacementWhenFullyBlocked constructs a position where Black's
// only possible L cells are entirely fenced off by other pieces, and checks
// LegalLPlacements correctly reports zero and Winner reports Black has lost.
func TestNoLegalPlacementWhenFullyBlocked(t *testing.T) {
	b := clearBoard()
	// Fill every cell with White except a small isolated pocket that can't
	// possibly fit any of the 8 L orientations (a single free cell and a
	// diagonal free cell, never orthogonally adjacent in an L shape).
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.set(x, y, White)
		}
	}
	// Give Black a nominal single square as its "L" isn't valid, but for
	// this test we only care about LegalLPlacements(b, Black) - Black
	// doesn't need to already occupy 4 cells for the legality/placement
	// scan itself (currentLCells just returns fewer than 4 points, which
	// simply won't match any candidate's 4-cell set).
	b.set(0, 0, Black)
	// Leave (0,0) as Black's sole existing cell (already set above); every
	// other cell is White, so there is no room for any 4-cell L shape.
	if got := LegalLPlacements(b, Black); len(got) != 0 {
		t.Fatalf("expected no legal placements when the board is fully blocked, got %v", got)
	}
	winner, over := Winner(b, Black)
	if !over || winner != White {
		t.Fatalf("Winner(b, Black) = (%v, %v), want (White, true)", winner, over)
	}
}

func TestApplyLPlacementLiftsOldCellsAndOccupiesNew(t *testing.T) {
	b := NewBoard()
	moves := LegalLPlacements(b, Black)
	pl := moves[0]
	nb := ApplyLPlacement(b, Black, pl)
	if nb.Count(Black) != 4 {
		t.Fatalf("after placement Black should still have exactly 4 cells, got %d", nb.Count(Black))
	}
	for _, c := range pl.Cells {
		if nb.At(c.X, c.Y) != Black {
			t.Errorf("new cell %v should hold Black after placement", c)
		}
	}
	oldCells := currentLCells(&b, Black)
	for _, c := range oldCells {
		// Old cell should be Black only if it's also one of the new cells.
		stillNew := false
		for _, nc := range pl.Cells {
			if nc == c {
				stillNew = true
			}
		}
		if !stillNew && nb.At(c.X, c.Y) == Black {
			t.Errorf("old cell %v should have been vacated (lifted before placing)", c)
		}
	}
	// White's pieces and the neutral pieces must be untouched by moving
	// Black's L.
	if nb.Count(White) != b.Count(White) {
		t.Error("White's piece count should be unaffected by Black's L move")
	}
	if nb.Count(Neutral) != b.Count(Neutral) {
		t.Error("neutral piece count should be unaffected by Black's L move")
	}
}

// TestLPlacementCannotOverlapOpponentOrNeutral builds a position where the
// only "geometrically possible" placements for Black at some anchor are
// blocked by White or a neutral piece, and checks they're excluded.
func TestLPlacementCannotOverlapOpponentOrNeutral(t *testing.T) {
	b := clearBoard()
	// Black currently at a harmless spot.
	placeShape(&b, Black, 0, image.Pt(0, 0)) // covers (0,0)(1,0)(2,0)(2,1)
	// White fully occupies the rest of row y=2 and y=3 except one cell, and
	// a neutral sits at (3,0), so no full alternative orientation fits
	// without overlapping something.
	b.set(3, 0, Neutral)
	b.set(0, 1, White)
	b.set(1, 1, White)
	b.set(0, 2, White)
	b.set(1, 2, White)
	b.set(3, 1, White)
	b.set(3, 2, White)
	b.set(0, 3, White)
	b.set(1, 3, White)
	b.set(2, 3, White)
	b.set(3, 3, White)
	// Remaining empty cells besides Black's own 4: (1,0) is Black already;
	// truly empty cells left: (2,2) only.
	for _, m := range LegalLPlacements(b, Black) {
		for _, c := range m.Cells {
			v := b.At(c.X, c.Y)
			if v == White || v == Neutral {
				t.Fatalf("placement %v illegally overlaps a %v piece at %v", m, v, c)
			}
		}
	}
}

func TestLegalNeutralMoves(t *testing.T) {
	b := NewBoard()
	moves := LegalNeutralMoves(b)
	if len(moves) == 0 {
		t.Fatal("expected some legal neutral moves on the starting board")
	}
	emptyCount := b.Count(Empty)
	neutralCount := b.Count(Neutral)
	if want := emptyCount * neutralCount; len(moves) != want {
		t.Fatalf("got %d neutral moves, want %d (every neutral piece x every empty cell)", len(moves), want)
	}
	for _, m := range moves {
		if b.At(m.From.X, m.From.Y) != Neutral {
			t.Errorf("move %v: From must currently hold Neutral", m)
		}
		if b.At(m.To.X, m.To.Y) != Empty {
			t.Errorf("move %v: To must currently be Empty", m)
		}
	}
}

func TestApplyNeutralMove(t *testing.T) {
	b := NewBoard()
	nm := LegalNeutralMoves(b)[0]
	nb := ApplyNeutralMove(b, nm)
	if nb.At(nm.From.X, nm.From.Y) != Empty {
		t.Error("origin cell should become Empty")
	}
	if nb.At(nm.To.X, nm.To.Y) != Neutral {
		t.Error("destination cell should become Neutral")
	}
	if nb.Count(Neutral) != 2 || nb.Count(Black) != 4 || nb.Count(White) != 4 {
		t.Error("piece counts must be preserved by a neutral move")
	}
}
