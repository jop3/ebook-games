package game

import (
	"image"
	"testing"
)

func TestWinnerNoneAtStart(t *testing.T) {
	b := NewBoard()
	if _, ok := Winner(&b); ok {
		t.Fatal("nobody should have won at the starting position")
	}
}

func TestWinnerP1ReachesGoalRow(t *testing.T) {
	b := NewBoard()
	b.Pawns[P1] = image.Pt(2, 0) // any cell of row 0, not just the center
	w, ok := Winner(&b)
	if !ok || w != P1 {
		t.Fatalf("Winner = %v,%v want P1,true", w, ok)
	}
}

func TestWinnerP2ReachesGoalRow(t *testing.T) {
	b := NewBoard()
	b.Pawns[P2] = image.Pt(7, Size-1) // any cell of the bottom row
	w, ok := Winner(&b)
	if !ok || w != P2 {
		t.Fatalf("Winner = %v,%v want P2,true", w, ok)
	}
}

func TestWinnerNotTriggeredByStartRow(t *testing.T) {
	b := NewBoard() // both pawns sit on their OWN start rows, not their goal rows
	if _, ok := Winner(&b); ok {
		t.Fatal("starting on your own edge must not itself be a win")
	}
}
