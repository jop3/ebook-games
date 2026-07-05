package game

import (
	"image"
	"testing"
)

// --- GOTCHA: win condition (a) — capturing the enemy King ------------------

func TestWinnerByKingCapture(t *testing.T) {
	b := emptyBoard()
	b[2][2] = &Piece{Kind: Triangle, Side: Black}
	b[1][1] = &Piece{Kind: King, Side: White}
	b[2][0] = &Piece{Kind: King, Side: Black} // irrelevant, far away (a neutral row, not on anyone's goal rank)
	if w, ok := Winner(&b); ok {
		t.Fatalf("Winner() = %v/%v before the capturing move, want no winner yet", w, ok)
	}
	nb, captured := b.Apply(Move{From: image.Pt(2, 2), To: image.Pt(1, 1)})
	if captured == nil || captured.Kind != King {
		t.Fatal("setup: the move should have captured White's King")
	}
	w, ok := Winner(&nb)
	if !ok || w != Black {
		t.Fatalf("Winner() = %v/%v, want Black (captured the enemy King)", w, ok)
	}
}

// --- GOTCHA: win condition (b) — King reaches the far edge ------------------

func TestWinnerByKingReachingFarEdge(t *testing.T) {
	b := emptyBoard()
	// Black's King, still in its starting diagonal (Ortho=false) mode, one
	// diagonal step from Black's goal rank (Rows-1=5).
	b[4][1] = &Piece{Kind: King, Side: Black}
	b[2][3] = &Piece{Kind: King, Side: White} // irrelevant, a neutral row (not on anyone's goal rank)
	if w, ok := Winner(&b); ok {
		t.Fatalf("Winner() = %v/%v before reaching the edge, want no winner yet", w, ok)
	}
	m := Move{From: image.Pt(1, 4), To: image.Pt(2, 5)}
	if !b.IsLegalMove(Black, m) {
		t.Fatal("setup: the diagonal step onto the goal rank should be legal")
	}
	nb, captured := b.Apply(m)
	if captured != nil {
		t.Fatal("setup: this move should not capture anything")
	}
	w, ok := Winner(&nb)
	if !ok || w != Black {
		t.Fatalf("Winner() = %v/%v, want Black (King reached the far edge)", w, ok)
	}
}

func TestWinnerByKingReachingFarEdgeWhiteSide(t *testing.T) {
	b := emptyBoard()
	b[0][2] = &Piece{Kind: King, Side: White} // White's goal rank is y=0
	b[2][0] = &Piece{Kind: King, Side: Black} // irrelevant, a neutral row (not on anyone's goal rank)
	w, ok := Winner(&b)
	if !ok || w != White {
		t.Fatalf("Winner() = %v/%v, want White (already on its goal rank)", w, ok)
	}
}

func TestNoWinnerMidGame(t *testing.T) {
	b := NewBoard()
	if w, ok := Winner(&b); ok {
		t.Fatalf("Winner() = %v/%v on the starting position, want no winner", w, ok)
	}
}
