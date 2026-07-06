package game

import (
	"image"
	"testing"
)

func TestNewBoardSetup(t *testing.T) {
	b := NewBoard()
	if got := b.Count(Black); got != Cols*2 {
		t.Fatalf("Black count = %d, want %d", got, Cols*2)
	}
	if got := b.Count(White); got != Cols*2 {
		t.Fatalf("White count = %d, want %d", got, Cols*2)
	}
	for x := 0; x < Cols; x++ {
		if b.At(x, Rows-1) != Black || b.At(x, Rows-2) != Black {
			t.Fatalf("Black should fill rows %d,%d; col %d did not", Rows-1, Rows-2, x)
		}
		if b.At(x, 0) != White || b.At(x, 1) != White {
			t.Fatalf("White should fill rows 0,1; col %d did not", x)
		}
	}
	// Middle rows empty on an 8x6 board (rows 2,3).
	for y := 2; y < Rows-2; y++ {
		for x := 0; x < Cols; x++ {
			if b.At(x, y) != Empty {
				t.Fatalf("row %d should be empty, col %d = %v", y, x, b.At(x, y))
			}
		}
	}
}

// --- THE critical semantics test: straight never captures, diagonal always
// captures, for BOTH sides. If someone flipped these (classic chess muscle
// memory bug), every assertion below fails.

func TestBlackStraightNeverCapturesDiagonalAlwaysCaptures(t *testing.T) {
	var b Board // all Empty
	b.set(3, 3, Black)
	b.set(3, 2, White) // straight ahead of Black's pawn (Black forward = dy -1)
	b.set(2, 2, White) // forward-left diagonal
	b.set(4, 2, Black) // forward-right diagonal, but occupied by OWN pawn

	straight := Move{From: image.Pt(3, 3), To: image.Pt(3, 2)}
	diagLeft := Move{From: image.Pt(3, 3), To: image.Pt(2, 2)}
	diagRight := Move{From: image.Pt(3, 3), To: image.Pt(4, 2)}

	if b.IsLegalMove(Black, straight) {
		t.Fatal("straight move onto an enemy pawn must be illegal — straight never captures")
	}
	if !b.IsLegalMove(Black, diagLeft) {
		t.Fatal("diagonal move onto an enemy pawn must be legal — diagonal always captures")
	}
	if b.IsLegalMove(Black, diagRight) {
		t.Fatal("diagonal move onto a square held by the mover's OWN pawn must be illegal")
	}

	moves := b.MovesFrom(image.Pt(3, 3), Black)
	if len(moves) != 1 {
		t.Fatalf("MovesFrom = %v, want exactly the one legal diagonal capture", moves)
	}
	if moves[0].To != image.Pt(2, 2) || !moves[0].Capture {
		t.Fatalf("MovesFrom = %v, want a Capture move to (2,2)", moves[0])
	}

	nb := b.Apply(diagLeft)
	if nb.At(2, 2) != Black {
		t.Fatal("after the diagonal capture, the mover should occupy the destination")
	}
	if nb.At(3, 3) != Empty {
		t.Fatal("after the move, the origin square should be empty")
	}
	if nb.Count(White) != 1 {
		t.Fatalf("the captured White pawn should be removed; White count = %d, want 1", nb.Count(White))
	}
}

func TestWhiteStraightNeverCapturesDiagonalAlwaysCaptures(t *testing.T) {
	// Mirror image of the Black test: White's forward is dy=+1.
	var b Board
	b.set(3, 2, White)
	b.set(3, 3, Black) // straight ahead of White's pawn
	b.set(4, 3, Black) // forward-right diagonal
	b.set(2, 3, White) // forward-left diagonal, occupied by OWN pawn

	straight := Move{From: image.Pt(3, 2), To: image.Pt(3, 3)}
	diagRight := Move{From: image.Pt(3, 2), To: image.Pt(4, 3)}
	diagLeft := Move{From: image.Pt(3, 2), To: image.Pt(2, 3)}

	if b.IsLegalMove(White, straight) {
		t.Fatal("straight move onto an enemy pawn must be illegal — straight never captures")
	}
	if !b.IsLegalMove(White, diagRight) {
		t.Fatal("diagonal move onto an enemy pawn must be legal — diagonal always captures")
	}
	if b.IsLegalMove(White, diagLeft) {
		t.Fatal("diagonal move onto a square held by the mover's OWN pawn must be illegal")
	}

	nb := b.Apply(diagRight)
	if nb.At(4, 3) != White {
		t.Fatal("after the diagonal capture, the mover should occupy the destination")
	}
	if nb.Count(Black) != 1 {
		t.Fatalf("the captured Black pawn should be removed; Black count = %d, want 1", nb.Count(Black))
	}
}

// --- Diagonal onto an EMPTY square must be illegal for both sides — the
// spec's "diagonal is never a plain move onto an empty square."

func TestDiagonalOntoEmptyIsIllegalBothSides(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	b.set(3, 2, White) // White pawn present to confirm we're testing the empty diagonal, not the whole rank

	diagEmpty := Move{From: image.Pt(3, 3), To: image.Pt(4, 2)} // empty square
	if b.IsLegalMove(Black, diagEmpty) {
		t.Fatal("Black diagonal onto an empty square must be illegal")
	}

	var b2 Board
	b2.set(3, 2, White)
	diagEmpty2 := Move{From: image.Pt(3, 2), To: image.Pt(2, 3)}
	if b2.IsLegalMove(White, diagEmpty2) {
		t.Fatal("White diagonal onto an empty square must be illegal")
	}
}

// --- Straight onto an empty square is legal (the only non-capturing move).

func TestStraightOntoEmptyIsLegalBothSides(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	if !b.IsLegalMove(Black, Move{From: image.Pt(3, 3), To: image.Pt(3, 2)}) {
		t.Fatal("Black straight move onto an empty square must be legal")
	}
	nb := b.Apply(Move{From: image.Pt(3, 3), To: image.Pt(3, 2)})
	if nb.At(3, 2) != Black || nb.At(3, 3) != Empty {
		t.Fatal("straight move should relocate the pawn with no capture")
	}

	var w Board
	w.set(2, 1, White)
	if !w.IsLegalMove(White, Move{From: image.Pt(2, 1), To: image.Pt(2, 2)}) {
		t.Fatal("White straight move onto an empty square must be legal")
	}
}

// --- No double-step, no sideways/backward moves, no jumping.

func TestNoDoubleStepOrSidewaysOrBackward(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	illegal := []Move{
		{From: image.Pt(3, 3), To: image.Pt(3, 1)}, // double straight step
		{From: image.Pt(3, 3), To: image.Pt(1, 1)}, // double diagonal step
		{From: image.Pt(3, 3), To: image.Pt(4, 3)}, // sideways
		{From: image.Pt(3, 3), To: image.Pt(3, 4)}, // backward straight (Black forward is -1)
		{From: image.Pt(3, 3), To: image.Pt(2, 4)}, // backward diagonal
		{From: image.Pt(3, 3), To: image.Pt(3, 3)}, // no-op
	}
	for _, m := range illegal {
		if b.IsLegalMove(Black, m) {
			t.Errorf("move %v should be illegal", m)
		}
	}
}

// --- Board edges: a diagonal destination off the board must not panic and
// must simply be absent from MovesFrom.

func TestEdgeColumnDiagonalBoundsChecked(t *testing.T) {
	var b Board
	b.set(0, 3, Black) // leftmost column: no x=-1 diagonal exists
	moves := b.MovesFrom(image.Pt(0, 3), Black)
	for _, m := range moves {
		if m.To.X < 0 || m.To.X >= Cols {
			t.Fatalf("move %v escaped the board", m)
		}
	}
	// Straight-ahead-only expected (no enemy anywhere).
	if len(moves) != 1 || moves[0].To != image.Pt(0, 2) {
		t.Fatalf("moves = %v, want just the straight move to (0,2)", moves)
	}
}

// --- LegalMoves aggregates MovesFrom correctly and only for the given side.

func TestLegalMovesOnlyForGivenSide(t *testing.T) {
	b := NewBoard()
	blackMoves := b.LegalMoves(Black)
	whiteMoves := b.LegalMoves(White)
	// At the opening, each of Black's Cols pawns on the front rank (Rows-2)
	// has exactly one straight move (the back rank Rows-1 pawns are fully
	// blocked by their own front rank); no captures exist yet.
	if len(blackMoves) != Cols {
		t.Fatalf("Black opening move count = %d, want %d", len(blackMoves), Cols)
	}
	if len(whiteMoves) != Cols {
		t.Fatalf("White opening move count = %d, want %d", len(whiteMoves), Cols)
	}
	for _, m := range blackMoves {
		if m.Capture {
			t.Fatal("no captures should exist in the opening position")
		}
	}
}

func TestIsLegalMoveRejectsWrongMoverAndOutOfBounds(t *testing.T) {
	b := NewBoard()
	// From holds no pawn of the claimed side.
	if b.IsLegalMove(Black, Move{From: image.Pt(0, 0), To: image.Pt(0, 1)}) {
		t.Fatal("From must hold a pawn of the claimed side")
	}
	// Out of bounds From/To must not panic and must be illegal.
	if b.IsLegalMove(Black, Move{From: image.Pt(-1, -1), To: image.Pt(0, 0)}) {
		t.Fatal("out-of-bounds From must be illegal")
	}
	if b.IsLegalMove(Black, Move{From: image.Pt(0, Rows-1), To: image.Pt(0, Rows)}) {
		t.Fatal("out-of-bounds To must be illegal")
	}
}
