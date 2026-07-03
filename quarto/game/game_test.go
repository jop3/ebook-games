package game

import "testing"

func TestShareOneWin(t *testing.T) {
	b := NewBoard()
	// Four pieces all Tall (bit0=1), varying other bits.
	b.Place(0, 0, AttrTall)
	b.Place(1, 0, AttrTall|AttrDark)
	b.Place(2, 0, AttrTall|AttrSquare)
	b.Place(3, 0, AttrTall|AttrSolid)
	if !b.HasWin() {
		t.Fatalf("expected a share-one (all Tall) win on row 0")
	}
	ln, ok := b.WinningLine()
	if !ok {
		t.Fatalf("expected WinningLine to report a line")
	}
	if ln != [4]int{0, 1, 2, 3} {
		t.Fatalf("expected row 0 indices, got %v", ln)
	}
}

func TestShareZeroWin(t *testing.T) {
	b := NewBoard()
	// Four pieces all Short (bit0=0) but otherwise all different — this is
	// the classic "easy to forget" win: p0&p1&p2&p3 == 0 yet they still win
	// because they all share the SAME zero bit.
	b.Place(0, 0, 0) // short, light, round, hollow
	b.Place(0, 1, AttrDark)
	b.Place(0, 2, AttrSquare)
	b.Place(0, 3, AttrSolid)
	if !b.HasWin() {
		t.Fatalf("expected a share-zero (all Short) win on column 0")
	}
	// Sanity: the classic AND is indeed zero, confirming this exercises the
	// share-zero path and not share-one.
	p0, p1, p2, p3 := b.At(0, 0), b.At(0, 1), b.At(0, 2), b.At(0, 3)
	if p0&p1&p2&p3 != 0 {
		t.Fatalf("test setup error: pieces should not share a 1-bit")
	}
}

func TestNoWinPartialLine(t *testing.T) {
	b := NewBoard()
	b.Place(0, 0, AttrTall)
	b.Place(1, 0, AttrTall)
	b.Place(2, 0, AttrTall)
	// row 0 incomplete
	if b.HasWin() {
		t.Fatalf("incomplete line should not win")
	}
}

func TestNoWinNoSharedAttribute(t *testing.T) {
	b := NewBoard()
	// Construct 4 pieces on a row such that no attribute is constant.
	// tall/dark/square/solid = 1111
	b.Place(0, 0, AttrTall|AttrDark|AttrSquare|AttrSolid)
	// short/light/round/hollow = 0000
	b.Place(1, 0, 0)
	// mixed: tall/light/round/hollow = 0001
	b.Place(2, 0, AttrTall)
	// mixed: short/dark/round/hollow = 0010
	b.Place(3, 0, AttrDark)
	if b.HasWin() {
		t.Fatalf("expected no win: no attribute is constant across all four")
	}
}

func TestDiagonalWin(t *testing.T) {
	b := NewBoard()
	b.Place(0, 0, AttrSolid)
	b.Place(1, 1, AttrSolid|AttrTall)
	b.Place(2, 2, AttrSolid|AttrDark)
	b.Place(3, 3, AttrSolid|AttrSquare)
	if !b.HasWin() {
		t.Fatalf("expected main-diagonal share-one (all Solid) win")
	}
}

func TestAntiDiagonalWin(t *testing.T) {
	b := NewBoard()
	b.Place(3, 0, AttrSquare)
	b.Place(2, 1, AttrSquare|AttrTall)
	b.Place(1, 2, AttrSquare|AttrDark)
	b.Place(0, 3, AttrSquare|AttrSolid)
	if !b.HasWin() {
		t.Fatalf("expected anti-diagonal share-one (all Square) win")
	}
}

func TestFullBoardDraw(t *testing.T) {
	b := NewBoard()
	if b.Full() {
		t.Fatalf("empty board should not be full")
	}
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.Place(x, y, Piece(n%NumPieces))
			n++
		}
	}
	if !b.Full() {
		t.Fatalf("expected board to be full after filling all cells")
	}
}

func TestPlaceRejectsOccupiedOrOOB(t *testing.T) {
	b := NewBoard()
	if !b.Place(0, 0, 3) {
		t.Fatalf("expected first placement to succeed")
	}
	if b.Place(0, 0, 4) {
		t.Fatalf("expected placement on occupied cell to fail")
	}
	if b.Place(-1, 0, 5) {
		t.Fatalf("expected out-of-bounds placement to fail")
	}
	if b.Place(0, 0, -1) {
		t.Fatalf("expected placing NoPiece via Place to fail (invalid piece range)")
	}
}
