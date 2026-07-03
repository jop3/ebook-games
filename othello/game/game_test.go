package game

import "testing"

func TestStartPosition(t *testing.T) {
	b := NewBoard()
	if b.Count(Black) != 2 || b.Count(White) != 2 {
		t.Fatalf("start should be 2-2, got %d-%d", b.Count(Black), b.Count(White))
	}
	if b.At(3, 3) != White || b.At(4, 4) != White || b.At(3, 4) != Black || b.At(4, 3) != Black {
		t.Fatal("wrong starting layout")
	}
}

func TestOpeningLegalMoves(t *testing.T) {
	b := NewBoard()
	moves := b.LegalMoves(Black)
	// Standard opening: Black has exactly four legal moves.
	if len(moves) != 4 {
		t.Fatalf("expected 4 opening moves for Black, got %d: %v", len(moves), moves)
	}
	want := map[[2]int]bool{{3, 2}: true, {2, 3}: true, {5, 4}: true, {4, 5}: true}
	for _, m := range moves {
		if !want[m] {
			t.Errorf("unexpected legal move %v", m)
		}
	}
}

func TestApplyFlips(t *testing.T) {
	b := NewBoard()
	// Black plays (3,2): should flip the White at (3,3).
	if !b.Apply(3, 2, Black) {
		t.Fatal("expected (3,2) to be legal for Black")
	}
	if b.At(3, 3) != Black {
		t.Error("White at (3,3) should have flipped to Black")
	}
	if b.At(3, 2) != Black {
		t.Error("placed disc should be Black")
	}
	if b.Count(Black) != 4 || b.Count(White) != 1 {
		t.Errorf("after flip expected 4-1, got %d-%d", b.Count(Black), b.Count(White))
	}
}

func TestIllegalMoves(t *testing.T) {
	b := NewBoard()
	if b.Apply(0, 0, Black) {
		t.Error("corner (0,0) should be illegal at start")
	}
	if b.Apply(3, 3, Black) {
		t.Error("occupied cell should be illegal")
	}
	// A move that brackets nothing is illegal.
	if b.LegalMove(2, 2, Black) {
		t.Error("(2,2) brackets nothing, should be illegal")
	}
}

func TestMultiDirectionFlip(t *testing.T) {
	// Construct a position where one move flips in two directions.
	var b Board
	b.set(4, 4, Black)
	b.set(3, 4, White)
	b.set(2, 4, Black) // horizontal bracket to the left of (5,4)? build carefully
	// Place W between two B horizontally and vertically around target (4,3).
	b = Board{}
	b.set(4, 3, Empty)
	// horizontal: B W [target] -> place at right
	b.set(2, 3, Black)
	b.set(3, 3, White)
	// vertical: B W [target]
	b.set(4, 1, Black)
	b.set(4, 2, White)
	// target (4,3): to the left along row 3 we have (3,3)=W,(2,3)=B -> flips (3,3)
	// upward along col 4 we have (4,2)=W,(4,1)=B -> flips (4,2)
	fl := b.flips(4, 3, Black)
	if len(fl) != 2 {
		t.Fatalf("expected 2 flips from two directions, got %d", len(fl))
	}
}
