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
	if n := b.Count(Black); n != 9 {
		t.Fatalf("Black count = %d, want 9", n)
	}
	if n := b.Count(White); n != 9 {
		t.Fatalf("White count = %d, want 9", n)
	}
	for x := 0; x < Size; x++ {
		if b.At(x, Size-1) != Black {
			t.Fatalf("(%d,%d) = %v, want Black on the bottom rank", x, Size-1, b.At(x, Size-1))
		}
		if b.At(x, 0) != White {
			t.Fatalf("(%d,%d) = %v, want White on the top rank", x, 0, b.At(x, 0))
		}
	}
	for y := 1; y < Size-1; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != Empty {
				t.Fatalf("(%d,%d) = %v, want Empty in the middle ranks", x, y, b.At(x, y))
			}
		}
	}
}

func TestRookMoveAnyDistance(t *testing.T) {
	b := emptyBoard()
	b.set(4, 4, Black)
	dests := b.DestinationsFrom(image.Pt(4, 4))
	// From an otherwise empty board, a rook at (4,4) reaches every other cell
	// in its row and column: (Size-1)*2 destinations.
	want := (Size - 1) * 2
	if len(dests) != want {
		t.Fatalf("got %d destinations, want %d: %v", len(dests), want, dests)
	}
	if !b.IsLegalMove(Black, Move{From: image.Pt(4, 4), To: image.Pt(4, 0)}) {
		t.Fatal("(4,4)->(4,0) should be legal: full-length vertical rook move")
	}
	if !b.IsLegalMove(Black, Move{From: image.Pt(4, 4), To: image.Pt(8, 4)}) {
		t.Fatal("(4,4)->(8,4) should be legal: full-length horizontal rook move")
	}
}

func TestRookCannotJumpOverAMan(t *testing.T) {
	b := emptyBoard()
	b.set(0, 4, Black)
	b.set(3, 4, White) // blocks the row beyond x=3
	if b.IsLegalMove(Black, Move{From: image.Pt(0, 4), To: image.Pt(3, 4)}) {
		t.Fatal("moving onto an occupied square must be illegal")
	}
	if b.IsLegalMove(Black, Move{From: image.Pt(0, 4), To: image.Pt(5, 4)}) {
		t.Fatal("jumping over the White man at x=3 must be illegal")
	}
	if !b.IsLegalMove(Black, Move{From: image.Pt(0, 4), To: image.Pt(2, 4)}) {
		t.Fatal("moving up to (but not through) the blocker should be legal")
	}
}

func TestDiagonalMoveIsIllegal(t *testing.T) {
	b := emptyBoard()
	b.set(4, 4, Black)
	if b.IsLegalMove(Black, Move{From: image.Pt(4, 4), To: image.Pt(6, 6)}) {
		t.Fatal("a diagonal move must be illegal — men move like rooks, not bishops")
	}
}

func TestCannotMoveOntoOwnOrEnemyMan(t *testing.T) {
	b := emptyBoard()
	b.set(0, 0, Black)
	b.set(0, 3, Black)
	b.set(0, 5, White)
	if b.IsLegalMove(Black, Move{From: image.Pt(0, 0), To: image.Pt(0, 3)}) {
		t.Fatal("moving onto your own man must be illegal")
	}
	if b.IsLegalMove(Black, Move{From: image.Pt(0, 0), To: image.Pt(0, 5)}) {
		t.Fatal("moving onto an enemy man must be illegal")
	}
}

func TestMovingSomeoneElsesManIsIllegal(t *testing.T) {
	b := emptyBoard()
	b.set(4, 4, White)
	if b.IsLegalMove(Black, Move{From: image.Pt(4, 4), To: image.Pt(4, 0)}) {
		t.Fatal("Black cannot move a White man")
	}
}

func TestLegalMovesFromStartingPosition(t *testing.T) {
	b := NewBoard()
	moves := b.LegalMoves(Black)
	// Black's men are all on the bottom rank (y=8), boxed in by White's rank
	// at y=0 only along columns; every column is open from y=7 down to y=1 (7
	// squares) plus sideways moves along the back rank are blocked by
	// neighboring Black men except at the two ends. Just sanity-check it's
	// nonzero and every move starts on a Black man and lands on an empty cell.
	if len(moves) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}
	for _, m := range moves {
		if b.At(m.From.X, m.From.Y) != Black {
			t.Fatalf("move %v does not originate on a Black man", m)
		}
		if b.At(m.To.X, m.To.Y) != Empty {
			t.Fatalf("move %v does not land on an empty cell", m)
		}
		if !b.IsLegalMove(Black, m) {
			t.Fatalf("LegalMoves produced a move IsLegalMove rejects: %v", m)
		}
	}
}
