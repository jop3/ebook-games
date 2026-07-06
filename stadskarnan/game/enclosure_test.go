package game

import "testing"

// place puts owner c with piece id pid at (x,y) directly on the board,
// bypassing legality — used to build specific test positions.
func place(b *Board, x, y int, c Cell, pid int8) {
	b.Owner[y][x] = c
	b.PieceID[y][x] = pid
}

// TestEnclosureEdgeTouchingNeverCaptured is the classic Cathedral bug this
// package is written to avoid: a region that reaches the board edge must
// never be treated as enclosed, even if every OTHER side of it is walled by
// one color.
func TestEnclosureEdgeTouchingNeverCaptured(t *testing.T) {
	b := NewBoard()
	// White monomino sits on the left edge (x=0); Black walls it on the
	// other 3 (in-bounds) sides. The 4th side is the board boundary itself,
	// not a wall — this region must stay open.
	place(&b, 0, 5, White, 0)
	place(&b, 1, 5, Black, 0)
	place(&b, 0, 4, Black, 1)
	place(&b, 0, 6, Black, 2)

	sealed, captured := Enclosure(&b)

	if len(captured) != 0 {
		t.Fatalf("edge-touching region must never be captured, got %v", captured)
	}
	if b.At(0, 5) != White {
		t.Fatal("the White man on the edge must remain on the board")
	}
	if b.IsSealed(0, 5) {
		t.Fatal("an edge-touching region must not be sealed")
	}
	for _, p := range sealed {
		if p.X == 0 && p.Y == 5 {
			t.Fatal("sealed list must not include the edge-touching cell")
		}
	}
}

// TestEnclosureCapturesOpponentInInteriorPocket checks the core capture
// rule: a non-edge-touching region whose entire border is one color,
// containing an opponent piece, captures that piece and seals the region.
func TestEnclosureCapturesOpponentInInteriorPocket(t *testing.T) {
	b := NewBoard()
	place(&b, 5, 5, White, 3) // trapped piece, piece id 3 in White's hand
	place(&b, 4, 5, Black, 0)
	place(&b, 6, 5, Black, 1)
	place(&b, 5, 4, Black, 2)
	place(&b, 5, 6, Black, 3)

	sealed, captured := Enclosure(&b)

	if len(captured) != 1 {
		t.Fatalf("expected exactly 1 captured piece, got %d: %v", len(captured), captured)
	}
	cp := captured[0]
	if cp.Owner != White || cp.PieceID != 3 {
		t.Fatalf("captured piece = %+v, want Owner=White PieceID=3", cp)
	}
	if len(cp.Cells) != 1 || cp.Cells[0].X != 5 || cp.Cells[0].Y != 5 {
		t.Fatalf("captured cells = %v, want [(5,5)]", cp.Cells)
	}
	if b.At(5, 5) != Empty {
		t.Fatal("the captured cell must now be Empty")
	}
	if !b.IsSealed(5, 5) {
		t.Fatal("the captured (now empty) cell must be permanently sealed")
	}
	foundSealed := false
	for _, p := range sealed {
		if p.X == 5 && p.Y == 5 {
			foundSealed = true
		}
	}
	if !foundSealed {
		t.Fatal("sealed list should include the captured cell")
	}
}

// TestEnclosureSealsEmptyPocketWithoutCapture checks the "no opponent
// pieces inside" branch: a landlocked, purely empty region enclosed by one
// color is sealed (no further placement there, ever) but nothing is
// captured (there is nothing to capture).
func TestEnclosureSealsEmptyPocketWithoutCapture(t *testing.T) {
	b := NewBoard()
	place(&b, 4, 5, Black, 0)
	place(&b, 6, 5, Black, 1)
	place(&b, 5, 4, Black, 2)
	place(&b, 5, 6, Black, 3)
	// (5,5) itself is left Empty — nothing to capture.

	sealed, captured := Enclosure(&b)

	if len(captured) != 0 {
		t.Fatalf("an empty enclosed pocket must capture nothing, got %v", captured)
	}
	if !b.IsSealed(5, 5) {
		t.Fatal("an empty enclosed pocket must still be sealed")
	}
	found := false
	for _, p := range sealed {
		if p.X == 5 && p.Y == 5 {
			found = true
		}
	}
	if !found {
		t.Fatal("sealed list should include the pocket cell")
	}

	// And the seal actually blocks future placement.
	b2 := b
	if len(LegalPlacementsForPiece(&b2, Pieces[0])) == 0 {
		t.Fatal("setup sanity: board should still have room elsewhere")
	}
	for _, pl := range LegalPlacementsForPiece(&b2, Pieces[0]) {
		if pl.Anchor.X == 5 && pl.Anchor.Y == 5 {
			t.Fatal("a sealed cell must never accept a future placement")
		}
	}
}

// TestEnclosureMixedBorderStaysOpen checks that a pocket bordered by BOTH
// colors (not uniformly one color) is not enclosed by either: the flood
// fill must pass straight through the lone opposite-color cell and keep
// going, in this setup reaching the rest of the (otherwise empty, open)
// board — i.e. it must not be captured or sealed.
func TestEnclosureMixedBorderStaysOpen(t *testing.T) {
	b := NewBoard()
	place(&b, 4, 5, Black, 0)
	place(&b, 6, 5, Black, 1)
	place(&b, 5, 4, Black, 2)
	place(&b, 5, 6, White, 0) // breaks the uniform Black border
	// (5,5) Empty; rest of the board Empty (open on all sides via (5,6)'s
	// other neighbors).

	sealed, captured := Enclosure(&b)

	if len(captured) != 0 {
		t.Fatalf("a mixed-color border must never capture, got %v", captured)
	}
	if b.IsSealed(5, 5) {
		t.Fatal("a mixed-color-bordered pocket that reaches open board must not be sealed")
	}
	for _, p := range sealed {
		if p.X == 5 && p.Y == 5 {
			t.Fatal("(5,5) must not appear in the sealed list")
		}
	}
}

// TestEnclosureByCathedralWall checks the Cathedral counts as part of a
// uniform border alongside a single color, per the rules ("bordered only by
// one color's pieces and/or the Cathedral").
func TestEnclosureByCathedralWall(t *testing.T) {
	b := NewBoard()
	place(&b, 5, 5, White, 7)
	place(&b, 4, 5, Black, 0)
	place(&b, 6, 5, Black, 1)
	place(&b, 5, 4, Black, 2)
	b.Owner[6][5] = Cathedral // south wall is the Cathedral instead of Black
	b.PieceID[6][5] = -1

	_, captured := Enclosure(&b)

	if len(captured) != 1 || captured[0].Owner != White || captured[0].PieceID != 7 {
		t.Fatalf("Cathedral should complete the enclosing wall and trigger capture, got %v", captured)
	}
}

// TestEnclosureBothColorsIndependently checks two separate pockets — one
// walled by Black trapping a White piece, one walled by White trapping a
// Black piece — are each resolved correctly and don't interfere with each
// other.
func TestEnclosureBothColorsIndependently(t *testing.T) {
	b := NewBoard()
	// Black-walled pocket trapping White, near (2,2).
	place(&b, 2, 2, White, 0)
	place(&b, 1, 2, Black, 0)
	place(&b, 3, 2, Black, 1)
	place(&b, 2, 1, Black, 2)
	place(&b, 2, 3, Black, 3)

	// White-walled pocket trapping Black, near (7,7).
	place(&b, 7, 7, Black, 4)
	place(&b, 6, 7, White, 1)
	place(&b, 8, 7, White, 2)
	place(&b, 7, 6, White, 3)
	place(&b, 7, 8, White, 4)

	_, captured := Enclosure(&b)

	if len(captured) != 2 {
		t.Fatalf("expected 2 captures, got %d: %v", len(captured), captured)
	}
	var gotWhite, gotBlack bool
	for _, cp := range captured {
		if cp.Owner == White && cp.PieceID == 0 {
			gotWhite = true
		}
		if cp.Owner == Black && cp.PieceID == 4 {
			gotBlack = true
		}
	}
	if !gotWhite || !gotBlack {
		t.Fatalf("expected one White and one Black capture, got %v", captured)
	}
	if b.At(2, 2) != Empty || b.At(7, 7) != Empty {
		t.Fatal("both trapped pieces should have been removed from the board")
	}
}
