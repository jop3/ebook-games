package game

import "testing"

func TestWinnerMoreBlack(t *testing.T) {
	var b Board
	b.set(0, 0, Black)
	b.set(1, 0, Black)
	b.set(2, 0, White)
	if got := Winner(&b); got != Black {
		t.Fatalf("Winner = %v, want Black (2 vs 1)", got)
	}
}

func TestWinnerMoreWhite(t *testing.T) {
	var b Board
	b.set(0, 0, White)
	b.set(1, 0, White)
	b.set(2, 0, Black)
	if got := Winner(&b); got != White {
		t.Fatalf("Winner = %v, want White (2 vs 1)", got)
	}
}

// --- GOTCHA: piece-count tiebreak (a tie is possible before the board is
// full, unlike the board-full case where 49 -- an odd number -- can never
// split evenly) ---------------------------------------------------------

func TestWinnerTie(t *testing.T) {
	var b Board
	b.set(0, 0, Black)
	b.set(1, 0, White)
	if got := Winner(&b); got != Empty {
		t.Fatalf("Winner = %v, want Empty (tie) for equal counts", got)
	}
}

func TestWinnerEmptyBoardIsTie(t *testing.T) {
	var b Board
	if got := Winner(&b); got != Empty {
		t.Fatalf("Winner of an empty board = %v, want Empty", got)
	}
}
