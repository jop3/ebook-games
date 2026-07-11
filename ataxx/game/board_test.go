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
	if n := b.Count(Black); n != 2 {
		t.Fatalf("Black count = %d, want 2", n)
	}
	if n := b.Count(White); n != 2 {
		t.Fatalf("White count = %d, want 2", n)
	}
	if b.At(0, 0) != Black || b.At(Size-1, Size-1) != Black {
		t.Fatalf("Black should start at (0,0) and (%d,%d)", Size-1, Size-1)
	}
	if b.At(Size-1, 0) != White || b.At(0, Size-1) != White {
		t.Fatalf("White should start at (%d,0) and (0,%d)", Size-1, Size-1)
	}
	if n := b.Count(Empty); n != Size*Size-4 {
		t.Fatalf("Empty count = %d, want %d", n, Size*Size-4)
	}
}

func TestCloneDestinationsMiddleOfBoard(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	dests := b.CloneDestinations(image.Pt(3, 3))
	if len(dests) != 8 {
		t.Fatalf("clone destinations from the center of an empty board = %d, want 8: %v", len(dests), dests)
	}
}

func TestCloneDestinationsCorner(t *testing.T) {
	b := emptyBoard()
	b.set(0, 0, Black)
	dests := b.CloneDestinations(image.Pt(0, 0))
	if len(dests) != 3 {
		t.Fatalf("clone destinations from a corner = %d, want 3: %v", len(dests), dests)
	}
}

func TestJumpDestinationsMiddleOfBoard(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	dests := b.JumpDestinations(image.Pt(3, 3))
	if len(dests) != 16 {
		t.Fatalf("jump destinations from the center of an empty board = %d, want 16: %v", len(dests), dests)
	}
	// Sanity: the 16-cell ring must include diagonal-knight-like offsets
	// like (+2,+1), not just the 8 pure straight/diagonal double-distance
	// cells -- otherwise this is only 8, a common under-implementation.
	want := image.Pt(5, 4) // (3+2, 3+1)
	found := false
	for _, d := range dests {
		if d == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("jump destinations %v missing the knight-like offset %v", dests, want)
	}
}

func TestJumpDestinationsCorner(t *testing.T) {
	b := emptyBoard()
	b.set(0, 0, Black)
	dests := b.JumpDestinations(image.Pt(0, 0))
	// From (0,0), the only in-bounds Chebyshev-distance-2 cells are
	// (2,0),(2,1),(2,2),(1,2),(0,2): 5 cells.
	if len(dests) != 5 {
		t.Fatalf("jump destinations from a corner = %d, want 5: %v", len(dests), dests)
	}
}

func TestDestinationsExcludeOccupiedCells(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(4, 3, White) // blocks one clone destination
	b.set(5, 3, Black) // blocks one jump destination
	clones := b.CloneDestinations(image.Pt(3, 3))
	for _, d := range clones {
		if d == (image.Point{X: 4, Y: 3}) {
			t.Fatal("an occupied cell must not be listed as a clone destination")
		}
	}
	if len(clones) != 7 {
		t.Fatalf("clone destinations = %d, want 7 (8 minus the occupied one)", len(clones))
	}
	jumps := b.JumpDestinations(image.Pt(3, 3))
	for _, d := range jumps {
		if d == (image.Point{X: 5, Y: 3}) {
			t.Fatal("an occupied cell must not be listed as a jump destination")
		}
	}
	if len(jumps) != 15 {
		t.Fatalf("jump destinations = %d, want 15 (16 minus the occupied one)", len(jumps))
	}
}

func TestIsLegalMoveDistances(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	cases := []struct {
		to   image.Point
		want bool
		name string
	}{
		{image.Pt(4, 4), true, "distance-1 diagonal clone"},
		{image.Pt(3, 4), true, "distance-1 orthogonal clone"},
		{image.Pt(5, 5), true, "distance-2 diagonal jump"},
		{image.Pt(3, 5), true, "distance-2 orthogonal jump"},
		{image.Pt(5, 4), true, "distance-2 knight-like jump"},
		{image.Pt(6, 6), false, "distance-3 is not a legal move"},
		{image.Pt(3, 3), false, "moving onto the source is not legal"},
	}
	for _, c := range cases {
		m := Move{From: image.Pt(3, 3), To: c.to}
		if got := b.IsLegalMove(Black, m); got != c.want {
			t.Errorf("%s: IsLegalMove(%v) = %v, want %v", c.name, m, got, c.want)
		}
	}
}

func TestIsLegalMoveRejectsWrongSideOrOccupiedDestination(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(4, 4, White)
	if b.IsLegalMove(White, Move{From: image.Pt(3, 3), To: image.Pt(4, 3)}) {
		t.Fatal("White should not be able to move Black's man")
	}
	if b.IsLegalMove(Black, Move{From: image.Pt(3, 3), To: image.Pt(4, 4)}) {
		t.Fatal("moving onto an occupied cell should be illegal")
	}
}

func TestLegalMovesCountsCloneAndJump(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	moves := b.LegalMoves(Black)
	if len(moves) != 8+16 {
		t.Fatalf("legal moves from the center of an empty board = %d, want %d", len(moves), 8+16)
	}
	clones, jumps := 0, 0
	for _, m := range moves {
		if m.IsClone() {
			clones++
		}
		if m.IsJump() {
			jumps++
		}
	}
	if clones != 8 || jumps != 16 {
		t.Fatalf("clone/jump split = %d/%d, want 8/16", clones, jumps)
	}
}

func TestLegalMovesOnlyForRequestedSide(t *testing.T) {
	b := NewBoard()
	blackMoves := b.LegalMoves(Black)
	for _, m := range blackMoves {
		if b.At(m.From.X, m.From.Y) != Black {
			t.Fatalf("LegalMoves(Black) returned a move from a non-Black cell: %v", m)
		}
	}
	whiteMoves := b.LegalMoves(White)
	for _, m := range whiteMoves {
		if b.At(m.From.X, m.From.Y) != White {
			t.Fatalf("LegalMoves(White) returned a move from a non-White cell: %v", m)
		}
	}
}

func TestIsFull(t *testing.T) {
	var b Board
	if b.IsFull() {
		t.Fatal("an empty board must not report full")
	}
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.set(x, y, Black)
		}
	}
	if !b.IsFull() {
		t.Fatal("a completely filled board must report full")
	}
}
