package game

import (
	"image"
	"testing"
)

// --- GOTCHA: a side can NEVER generate/play a move in the other orientation -

func TestLegalMovesVOnlyVertical(t *testing.T) {
	b := NewBoard(SizeStandard)
	moves := b.LegalMoves(V)
	if len(moves) == 0 {
		t.Fatal("V should have legal moves on an empty board")
	}
	for _, m := range moves {
		if m.A.X != m.B.X {
			t.Fatalf("V move %v is not vertical (columns differ)", m)
		}
		if abs(m.A.Y-m.B.Y) != 1 {
			t.Fatalf("V move %v cells are not adjacent rows", m)
		}
	}
}

func TestLegalMovesHOnlyHorizontal(t *testing.T) {
	b := NewBoard(SizeStandard)
	moves := b.LegalMoves(H)
	if len(moves) == 0 {
		t.Fatal("H should have legal moves on an empty board")
	}
	for _, m := range moves {
		if m.A.Y != m.B.Y {
			t.Fatalf("H move %v is not horizontal (rows differ)", m)
		}
		if abs(m.A.X-m.B.X) != 1 {
			t.Fatalf("H move %v cells are not adjacent columns", m)
		}
	}
}

func TestIsLegalMoveRejectsWrongOrientation(t *testing.T) {
	b := NewBoard(SizeStandard)
	// A horizontal pair is illegal for V, and a vertical pair is illegal for H.
	horiz := Move{A: image.Pt(2, 2), B: image.Pt(3, 2)}
	vert := Move{A: image.Pt(2, 2), B: image.Pt(2, 3)}
	if b.IsLegalMove(V, horiz) {
		t.Fatal("V must never be allowed to place a horizontal domino")
	}
	if b.IsLegalMove(H, vert) {
		t.Fatal("H must never be allowed to place a vertical domino")
	}
	if !b.IsLegalMove(V, vert) {
		t.Fatal("V should be able to place this vertical domino")
	}
	if !b.IsLegalMove(H, horiz) {
		t.Fatal("H should be able to place this horizontal domino")
	}
}

func TestIsLegalMoveRejectsOccupiedOrOutOfBounds(t *testing.T) {
	b := NewBoard(SizeStandard)
	b = b.Apply(Move{A: image.Pt(0, 0), B: image.Pt(0, 1)})
	if b.IsLegalMove(V, Move{A: image.Pt(0, 0), B: image.Pt(0, 1)}) {
		t.Fatal("must reject a move onto already-occupied cells")
	}
	// Off the bottom-right edge.
	edge := Move{A: image.Pt(SizeStandard-1, SizeStandard-1), B: image.Pt(SizeStandard, SizeStandard-1)}
	if b.IsLegalMove(H, edge) {
		t.Fatal("must reject a move with a cell off the board")
	}
	// Non-adjacent cells, same column (not a valid vertical pair).
	farV := Move{A: image.Pt(3, 0), B: image.Pt(3, 2)}
	if b.IsLegalMove(V, farV) {
		t.Fatal("must reject two non-adjacent cells as a vertical move")
	}
}

func TestApplyMarksBothCellsOccupied(t *testing.T) {
	b := NewBoard(SizeStandard)
	m := Move{A: image.Pt(4, 4), B: image.Pt(4, 5)}
	nb := b.Apply(m)
	if !b.Empty(4, 4) || !b.Empty(4, 5) {
		t.Fatal("Apply must not mutate the receiver board")
	}
	if nb.Empty(4, 4) || nb.Empty(4, 5) {
		t.Fatal("Apply must occupy both of the move's cells")
	}
	if nb.EmptyCount() != b.EmptyCount()-2 {
		t.Fatalf("EmptyCount should drop by exactly 2, got %d -> %d", b.EmptyCount(), nb.EmptyCount())
	}
}

// --- PartnersFrom: the "ghost second cell" the UI highlights ----------------

func TestPartnersFromEmptyBoardInterior(t *testing.T) {
	b := NewBoard(SizeStandard)
	p := image.Pt(4, 4)
	vPartners := b.PartnersFrom(V, p)
	if len(vPartners) != 2 {
		t.Fatalf("interior cell should have 2 vertical partner candidates, got %v", vPartners)
	}
	hPartners := b.PartnersFrom(H, p)
	if len(hPartners) != 2 {
		t.Fatalf("interior cell should have 2 horizontal partner candidates, got %v", hPartners)
	}
}

func TestPartnersFromCornerHasExactlyOne(t *testing.T) {
	b := NewBoard(SizeStandard)
	p := image.Pt(0, 0) // top-left corner
	vPartners := b.PartnersFrom(V, p)
	if len(vPartners) != 1 || vPartners[0] != image.Pt(0, 1) {
		t.Fatalf("corner cell should have exactly one vertical partner (below), got %v", vPartners)
	}
	hPartners := b.PartnersFrom(H, p)
	if len(hPartners) != 1 || hPartners[0] != image.Pt(1, 0) {
		t.Fatalf("corner cell should have exactly one horizontal partner (right), got %v", hPartners)
	}
}

func TestPartnersFromOccupiedCellIsEmpty(t *testing.T) {
	b := NewBoard(SizeStandard)
	b = b.Apply(Move{A: image.Pt(4, 4), B: image.Pt(4, 5)})
	if got := b.PartnersFrom(V, image.Pt(4, 4)); got != nil {
		t.Fatalf("an occupied cell has no partners, got %v", got)
	}
}

func TestMakeMoveCanonicalOrder(t *testing.T) {
	m1 := MakeMove(image.Pt(3, 5), image.Pt(3, 4))
	if m1.A != image.Pt(3, 4) || m1.B != image.Pt(3, 5) {
		t.Fatalf("MakeMove should order vertically by row: got %v", m1)
	}
	m2 := MakeMove(image.Pt(5, 2), image.Pt(4, 2))
	if m2.A != image.Pt(4, 2) || m2.B != image.Pt(5, 2) {
		t.Fatalf("MakeMove should order horizontally by column: got %v", m2)
	}
}

// --- HAND-VERIFIED small-board endgame positions ----------------------------

// A 1x2 board (1 column wide, 2 rows tall): V can fill it in one move; H can
// never place at all (needs 2 columns). If V is to move, V has exactly the
// one legal move, and it fills the board.
func Test1x2BoardOnlyVCanMove(t *testing.T) {
	// Board is always square in this package, so model "a single open column,
	// 2 rows tall" as a 2x2 board with its right column pre-filled.
	b := BoardFromRows([][]bool{
		{false, true}, // row 0: col 0 empty, col 1 occupied (walls off the right column)
		{false, true}, // row 1: col 0 empty, col 1 occupied
	})
	vMoves := b.LegalMoves(V)
	hMoves := b.LegalMoves(H)
	if len(hMoves) != 0 {
		t.Fatalf("H should have no legal move in a single open column, got %v", hMoves)
	}
	if len(vMoves) != 1 {
		t.Fatalf("V should have exactly 1 legal move (fill the open column), got %v", vMoves)
	}
	nb := b.Apply(vMoves[0])
	if nb.EmptyCount() != 0 {
		t.Fatalf("filling the last open column should leave the board full, got %d empty", nb.EmptyCount())
	}
	if nb.HasMove(V) || nb.HasMove(H) {
		t.Fatal("a full board must have no legal move for either side")
	}
}

// Symmetric case: a single open ROW, 2 cells wide — only H can move.
func Test1x2BoardOnlyHCanMove(t *testing.T) {
	b := BoardFromRows([][]bool{
		{false, false}, // row 0: both cells open
		{true, true},   // row 1: fully occupied
	})
	vMoves := b.LegalMoves(V)
	hMoves := b.LegalMoves(H)
	if len(vMoves) != 0 {
		t.Fatalf("V should have no legal move in a single open row, got %v", vMoves)
	}
	if len(hMoves) != 1 {
		t.Fatalf("H should have exactly 1 legal move (fill the open row), got %v", hMoves)
	}
}

// On an empty 2x2 board, whichever column V fills, the remaining two empty
// cells sit in a single column — vertically, not horizontally, adjacent — so
// H (to move next) has no legal move and loses. Hand-verified: V wins a 2x2
// game outright with either of its two possible opening moves.
func Test2x2BoardVAlwaysWinsAfterOneMove(t *testing.T) {
	b := NewBoard(2)
	for _, m := range b.LegalMoves(V) {
		nb := b.Apply(m)
		if nb.HasMove(H) {
			t.Fatalf("after V plays %v on a 2x2 board, H should have no legal move left, but does", m)
		}
	}
}

// A 1x1 board: neither side can ever place (no room for any domino at all).
func Test1x1BoardNeitherCanMove(t *testing.T) {
	b := NewBoard(1)
	if b.HasMove(V) || b.HasMove(H) {
		t.Fatal("a 1x1 board has no legal move for either side")
	}
}
