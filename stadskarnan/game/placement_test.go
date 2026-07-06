package game

import "testing"

// TestLegalPlacementsEmptyBoardDomino checks the count of legal placements
// for a straight domino on a fully empty 10x10 board: each of its 2
// orientations (bounding box 2x1 or 1x2) has 9*10 = 90 valid origins, for
// 180 total.
func TestLegalPlacementsEmptyBoardDomino(t *testing.T) {
	b := NewBoard()
	domino := PieceDef{Name: "test", Cells: []Offset{{0, 0}, {1, 0}}}
	got := len(LegalPlacementsForPiece(&b, domino))
	want := 2 * 9 * 10
	if got != want {
		t.Fatalf("legal domino placements on empty board = %d, want %d", got, want)
	}
}

// TestLegalPlacementsMonominoFillsBoard checks a single square has exactly
// Size*Size legal placements on an empty board (every cell, one orientation).
func TestLegalPlacementsMonominoFillsBoard(t *testing.T) {
	b := NewBoard()
	got := len(LegalPlacementsForPiece(&b, Pieces[0]))
	if got != Size*Size {
		t.Fatalf("legal monomino placements = %d, want %d", got, Size*Size)
	}
}

// TestLegalPlacementsExcludeOccupiedCells checks that a cell occupied by any
// owner (Black/White/Cathedral) removes every placement that would cover it.
func TestLegalPlacementsExcludeOccupiedCells(t *testing.T) {
	b := NewBoard()
	b.Owner[5][5] = Black
	for _, pl := range LegalPlacementsForPiece(&b, Pieces[0]) {
		for _, c := range pl.Cells {
			if c.X == 5 && c.Y == 5 {
				t.Fatalf("placement %v illegally covers the occupied cell (5,5)", pl)
			}
		}
	}
	got := len(LegalPlacementsForPiece(&b, Pieces[0]))
	if got != Size*Size-1 {
		t.Fatalf("legal monomino placements with one occupied cell = %d, want %d", got, Size*Size-1)
	}
}

// TestLegalPlacementsExcludeSealedCells checks Sealed cells are excluded even
// though they remain Empty (this is the mechanism that makes a sealed region
// permanently unplaceable).
func TestLegalPlacementsExcludeSealedCells(t *testing.T) {
	b := NewBoard()
	b.Sealed[3][3] = true
	for _, pl := range LegalPlacementsForPiece(&b, Pieces[0]) {
		if pl.Anchor.X == 3 && pl.Anchor.Y == 3 {
			t.Fatalf("placement at sealed cell (3,3) should not be legal")
		}
	}
	got := len(LegalPlacementsForPiece(&b, Pieces[0]))
	if got != Size*Size-1 {
		t.Fatalf("legal monomino placements with one sealed cell = %d, want %d", got, Size*Size-1)
	}
}

// TestHasAnyLegalPlacementFullBoard checks a completely full board reports
// no legal placement for any piece.
func TestHasAnyLegalPlacementFullBoard(t *testing.T) {
	b := NewBoard()
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.Owner[y][x] = Black
		}
	}
	hand := NewHand()
	if b.HasAnyLegalPlacement(&hand) {
		t.Fatal("a full board must have no legal placement")
	}
}

// TestHasAnyLegalPlacementEmptyHand checks a side with no available pieces
// (all placed/used) has no legal placement even on an empty board.
func TestHasAnyLegalPlacementEmptyHand(t *testing.T) {
	b := NewBoard()
	var hand Hand // all false
	if b.HasAnyLegalPlacement(&hand) {
		t.Fatal("an empty hand must have no legal placement")
	}
}

// TestHasAnyLegalPlacementFindsRemainingRoom checks that a single 1x1 gap in
// an otherwise full board is found by a monomino (but not by a piece too big
// to fit).
func TestHasAnyLegalPlacementFindsRemainingRoom(t *testing.T) {
	b := NewBoard()
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if x == 4 && y == 4 {
				continue
			}
			b.Owner[y][x] = Black
		}
	}
	monoHand := NewHand()
	for i := range monoHand {
		monoHand[i] = false
	}
	monoHand[0] = true // Stuga, a monomino
	if !b.HasAnyLegalPlacement(&monoHand) {
		t.Fatal("a single free cell should be placeable by a monomino")
	}

	bigHand := NewHand()
	for i := range bigHand {
		bigHand[i] = false
	}
	bigHand[8] = true // Allé, a straight tetromino: needs 4 in a row
	if b.HasAnyLegalPlacement(&bigHand) {
		t.Fatal("a single free cell must not fit a 4-square piece")
	}
}

// TestLegalCathedralPlacementsCount checks the neutral Cathedral (a 3x3
// bounding box cross) has 8*8 legal anchor placements on an empty board (one
// orientation, since the cross is fully symmetric).
func TestLegalCathedralPlacementsCount(t *testing.T) {
	b := NewBoard()
	got := len(LegalCathedralPlacements(&b))
	want := (Size - 2) * (Size - 2)
	if got != want {
		t.Fatalf("legal Cathedral placements = %d, want %d", got, want)
	}
}
