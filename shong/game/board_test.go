package game

import (
	"image"
	"testing"
)

func emptyBoard() Board {
	var b Board
	return b
}

func TestNewBoardStartingPosition(t *testing.T) {
	b := NewBoard()
	wantCols := [Cols]Kind{Ex, Triangle, Square, King}
	for x := 0; x < Cols; x++ {
		bp := b.At(x, 0)
		if bp == nil || bp.Side != Black || bp.Kind != wantCols[x] {
			t.Fatalf("(%d,0) = %+v, want Black %v", x, bp, wantCols[x])
		}
		if bp.Moved {
			t.Fatalf("(%d,0) starts already Moved", x)
		}
		wp := b.At(x, Rows-1)
		if wp == nil || wp.Side != White || wp.Kind != wantCols[x] {
			t.Fatalf("(%d,%d) = %+v, want White %v", x, Rows-1, wp, wantCols[x])
		}
	}
	if b.Count(Black) != Cols || b.Count(White) != Cols {
		t.Fatalf("counts = %d/%d, want %d/%d", b.Count(Black), b.Count(White), Cols, Cols)
	}
	for y := 1; y < Rows-1; y++ {
		for x := 0; x < Cols; x++ {
			if b.At(x, y) != nil {
				t.Fatalf("(%d,%d) should be empty in the starting position", x, y)
			}
		}
	}
}

// --- direction sets ----------------------------------------------------------

func TestTriangleMovesDiagonalOnly(t *testing.T) {
	b := emptyBoard()
	b[3][1] = &Piece{Kind: Triangle, Side: Black}
	dests := b.DestinationsFrom(image.Pt(1, 3))
	want := map[image.Point]bool{
		{X: 2, Y: 4}: true, {X: 0, Y: 4}: true, {X: 2, Y: 2}: true, {X: 0, Y: 2}: true,
	}
	if len(dests) != len(want) {
		t.Fatalf("Triangle destinations = %v, want exactly the 4 diagonal neighbors", dests)
	}
	for _, d := range dests {
		if !want[d] {
			t.Fatalf("Triangle moved non-diagonally to %v", d)
		}
	}
}

func TestSquareMovesOrthogonalOnly(t *testing.T) {
	b := emptyBoard()
	b[3][1] = &Piece{Kind: Square, Side: Black}
	dests := b.DestinationsFrom(image.Pt(1, 3))
	want := map[image.Point]bool{
		{X: 2, Y: 3}: true, {X: 0, Y: 3}: true, {X: 1, Y: 4}: true, {X: 1, Y: 2}: true,
	}
	if len(dests) != len(want) {
		t.Fatalf("Square destinations = %v, want exactly the 4 orthogonal neighbors", dests)
	}
	for _, d := range dests {
		if !want[d] {
			t.Fatalf("Square moved non-orthogonally to %v", d)
		}
	}
}

func TestExMovesAllEightDirections(t *testing.T) {
	b := emptyBoard()
	b[3][1] = &Piece{Kind: Ex, Side: Black}
	dests := b.DestinationsFrom(image.Pt(1, 3))
	if len(dests) != 8 {
		t.Fatalf("X destinations = %v (%d), want all 8 neighbors", dests, len(dests))
	}
}

// --- GOTCHA: exact distance (short=1, long=2, never "up to") ----------------

func TestShortMoveMustBeExactlyOne(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Square, Side: Black} // not yet Moved: short (distance 1)
	if !b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(2, 1)}) {
		t.Fatal("an unmoved piece's distance-1 move should be legal")
	}
	if b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(2, 0)}) {
		t.Fatal("an unmoved (short) piece must not be able to move 2 squares")
	}
}

func TestLongMoveMustBeExactlyTwoNotOne(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Square, Side: Black, Moved: true} // long: distance must be exactly 2
	if b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(2, 1)}) {
		t.Fatal("a long piece must NOT be able to stop after only 1 square")
	}
	if !b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(2, 0)}) {
		t.Fatal("a long piece's clear distance-2 move should be legal")
	}
}

func TestLongMoveMiddleSquareMustBeClear(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Square, Side: Black, Moved: true}
	b[1][2] = &Piece{Kind: Triangle, Side: White} // sits on the middle square of the long move
	if b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(2, 0)}) {
		t.Fatal("a long move must be illegal if its middle square is occupied (by either side)")
	}
	// The blocker itself (adjacent, distance 1) is not reachable either since
	// this piece is long (must move exactly 2), not short.
	if b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(2, 1)}) {
		t.Fatal("a long piece cannot capture at distance 1")
	}
}

func TestLongMoveMiddleSquareBlockedByOwnPiece(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Square, Side: Black, Moved: true}
	b[1][2] = &Piece{Kind: Triangle, Side: Black} // own piece blocks the middle square
	if b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(2, 0)}) {
		t.Fatal("a long move must be illegal if blocked by the mover's own piece")
	}
}

// --- GOTCHA: no jumping, for both short and long moves ----------------------

func TestShortMoveCannotLandOnOwnPiece(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Triangle, Side: Black}
	b[1][1] = &Piece{Kind: Square, Side: Black}
	if b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(1, 1)}) {
		t.Fatal("landing on the mover's own piece must be illegal")
	}
}

// --- GOTCHA: displacement capture on landing --------------------------------

func TestDisplacementCaptureOnLanding(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Triangle, Side: Black}
	b[1][1] = &Piece{Kind: Square, Side: White}
	if !b.IsLegalMove(Black, Move{From: image.Pt(2, 2), To: image.Pt(1, 1)}) {
		t.Fatal("landing on an enemy piece should be a legal capturing move")
	}
	nb, captured := b.Apply(Move{From: image.Pt(2, 2), To: image.Pt(1, 1)})
	if captured == nil || captured.Side != White || captured.Kind != Square {
		t.Fatalf("Apply should report the captured White Square, got %+v", captured)
	}
	if nb.At(1, 1) == nil || nb.At(1, 1).Side != Black || nb.At(1, 1).Kind != Triangle {
		t.Fatal("the mover should now occupy the landing square")
	}
	if nb.At(2, 2) != nil {
		t.Fatal("the origin square should now be empty")
	}
	if nb.Count(White) != 0 {
		t.Fatal("the captured White piece should be gone from the board")
	}
}

func TestLongMoveDisplacementCapture(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Square, Side: Black, Moved: true}
	b[0][2] = &Piece{Kind: Triangle, Side: White}
	nb, captured := b.Apply(Move{From: image.Pt(2, 2), To: image.Pt(2, 0)})
	if captured == nil || captured.Side != White {
		t.Fatal("a long move landing on an enemy piece should capture it")
	}
	if nb.At(2, 0).Side != Black {
		t.Fatal("the mover should now occupy the landing square")
	}
}

// --- move-marking: a piece becomes long only after its own first move ------

func TestPieceBecomesLongAfterItsFirstMove(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Triangle, Side: Black}
	nb, _ := b.Apply(Move{From: image.Pt(2, 2), To: image.Pt(1, 1)})
	p := nb.At(1, 1)
	if p == nil || !p.Moved {
		t.Fatal("a piece must be marked Moved after its first move")
	}
	// Now it must move exactly 2, never 1.
	if nb.IsLegalMove(Black, Move{From: image.Pt(1, 1), To: image.Pt(2, 2)}) {
		t.Fatal("the now-long piece must not be able to move only 1 square")
	}
	if !nb.IsLegalMove(Black, Move{From: image.Pt(1, 1), To: image.Pt(3, 3)}) {
		t.Fatal("the now-long piece should be able to move exactly 2 squares")
	}
}

// --- GOTCHA: King always moves exactly 1, regardless of Moved --------------

func TestKingAlwaysMovesExactlyOne(t *testing.T) {
	b := emptyBoard()
	b[2][1] = &Piece{Kind: King, Side: Black, Moved: true} // Moved=true must not matter for King
	dests := b.DestinationsFrom(image.Pt(1, 2))
	for _, d := range dests {
		dx, dy := absInt(d.X-1), absInt(d.Y-2)
		if dx > 1 || dy > 1 {
			t.Fatalf("King destination %v is farther than 1 step away from (1,2)", d)
		}
	}
	if len(dests) == 0 {
		t.Fatal("King should have some legal moves on an empty board")
	}
}

// --- GOTCHA: King's alternating move-set is per-King state, flips only when
// the King itself moves (not every ply) --------------------------------------

func TestKingMoveSetFlipsOnlyWhenTheKingMoves(t *testing.T) {
	b := emptyBoard()
	b[2][1] = &Piece{Kind: King, Side: Black}   // (x=1,y=2): Ortho=false, starts diagonal
	b[0][3] = &Piece{Kind: Square, Side: White} // (x=3,y=0): an unrelated White piece

	king := b.At(1, 2)
	if king == nil || king.Ortho {
		t.Fatal("setup: King should start in diagonal (Ortho=false) mode")
	}
	dests := b.DestinationsFrom(image.Pt(1, 2))
	for _, d := range dests {
		if d.X == 1 || d.Y == 2 {
			t.Fatalf("King in diagonal mode moved orthogonally to %v", d)
		}
	}

	// Some OTHER piece (White's) moves; the Black King's own piece struct is
	// untouched, so its move-set must not have changed.
	afterOther, _ := b.Apply(Move{From: image.Pt(3, 0), To: image.Pt(3, 1)})
	kingStill := afterOther.At(1, 2)
	if kingStill == nil || kingStill.Ortho {
		t.Fatal("the King's move-set must not flip just because a DIFFERENT piece moved")
	}

	// Now the King itself moves: its mode must flip to orthogonal.
	afterKingMove, _ := b.Apply(Move{From: image.Pt(1, 2), To: image.Pt(2, 3)})
	king2 := afterKingMove.At(2, 3)
	if king2 == nil || !king2.Ortho {
		t.Fatal("the King's move-set must flip to orthogonal after it moves")
	}
	dests2 := afterKingMove.DestinationsFrom(image.Pt(2, 3))
	for _, d := range dests2 {
		if d.X != 2 && d.Y != 3 {
			t.Fatalf("King in orthogonal mode moved diagonally to %v", d)
		}
	}

	// And it must NOT flip again just because another ply passes without the
	// King moving.
	other := afterKingMove
	other[0][0] = &Piece{Kind: Triangle, Side: White} // (x=0,y=0)
	afterOtherMove, _ := other.Apply(Move{From: image.Pt(0, 0), To: image.Pt(1, 1)})
	king3 := afterOtherMove.At(2, 3)
	if king3 == nil || !king3.Ortho {
		t.Fatal("the King's move-set must stay orthogonal when a DIFFERENT piece moves")
	}
}

// --- Apply produces independent board copies (no shared-pointer aliasing) --

func TestApplyProducesIndependentBoards(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Triangle, Side: Black}
	b1, _ := b.Apply(Move{From: image.Pt(2, 2), To: image.Pt(1, 1)})
	b2, _ := b.Apply(Move{From: image.Pt(2, 2), To: image.Pt(3, 1)})
	if b1.At(1, 1) == nil || b1.At(3, 1) != nil {
		t.Fatal("b1 should reflect only its own move")
	}
	if b2.At(3, 1) == nil || b2.At(1, 1) != nil {
		t.Fatal("b2 should reflect only its own move")
	}
	if b.At(2, 2) == nil {
		t.Fatal("the original board must be unaffected by Apply")
	}
}
