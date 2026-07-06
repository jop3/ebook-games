package game

import "testing"

func TestWinnerKingEscapedToAnyCorner(t *testing.T) {
	for _, c := range cornerCells {
		b := emptyBoard()
		b.set(c.X, c.Y, King)
		w, reason, ok := Winner(&b)
		if !ok || w != SideDefender || reason != ReasonKingEscaped {
			t.Fatalf("corner %v: Winner = %v,%v,%v, want SideDefender/ReasonKingEscaped/true", c, w, reason, ok)
		}
	}
}

func TestWinnerKingNotYetEscapedFromNonCornerSquare(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, King) // not a corner
	if _, _, ok := Winner(&b); ok {
		t.Fatal("the king sitting on a non-corner square must not be a win")
	}
	b2 := NewBoard() // king on the throne, the starting position
	if _, _, ok := Winner(&b2); ok {
		t.Fatal("the starting position must not already be a win")
	}
}

func TestWinnerKingCapturedMeansAttackersWin(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, Defender) // king absent from the board entirely
	w, reason, ok := Winner(&b)
	if !ok || w != SideAttacker || reason != ReasonKingCaptured {
		t.Fatalf("Winner = %v,%v,%v, want SideAttacker/ReasonKingCaptured/true when the king is gone", w, reason, ok)
	}
}
