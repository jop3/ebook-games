package game

import "testing"

func TestWinnerNoWinnerYet(t *testing.T) {
	b := NewBoard()
	if w := Winner(&b); w != Empty {
		t.Fatalf("Winner on the starting position = %v, want Empty", w)
	}
}

// --- Win condition 1: a pawn reaches the opponent's home (back) rank.

func TestWinnerBlackReachesRow0(t *testing.T) {
	var b Board
	b.set(3, 0, Black) // on White's back rank
	b.set(0, 4, White) // White still has a pawn elsewhere (not on row Rows-1)
	if w := Winner(&b); w != Black {
		t.Fatalf("Winner = %v, want Black (reached row 0)", w)
	}
}

func TestWinnerWhiteReachesLastRow(t *testing.T) {
	var b Board
	b.set(3, Rows-1, White) // on Black's back rank
	b.set(0, 3, Black)      // Black still has a pawn elsewhere (not on row 0)
	if w := Winner(&b); w != White {
		t.Fatalf("Winner = %v, want White (reached row %d)", w, Rows-1)
	}
}

// A pawn merely sitting on its OWN home rank (never left) is not a win for
// anyone — only reaching the OPPONENT's back rank counts.
func TestWinnerOwnHomeRankIsNotAWin(t *testing.T) {
	var b Board
	b.set(3, Rows-1, Black) // Black's own back rank
	b.set(3, 0, White)      // White's own back rank
	if w := Winner(&b); w != Empty {
		t.Fatalf("Winner = %v, want Empty (both sides merely sitting on their own home rank)", w)
	}
}

// --- Win condition 2: the opponent has zero pawns.

func TestWinnerOpponentZeroPawns(t *testing.T) {
	var b Board
	b.set(3, 3, Black) // sole survivor; no White pawns anywhere
	if w := Winner(&b); w != Black {
		t.Fatalf("Winner = %v, want Black (White has zero pawns)", w)
	}

	var b2 Board
	b2.set(3, 3, White)
	if w := Winner(&b2); w != White {
		t.Fatalf("Winner = %v, want White (Black has zero pawns)", w)
	}
}

// An empty board (neither side has any pawns) must not spuriously declare a
// winner via the zero-pawns check firing for both sides at once — Black's
// check is evaluated first, so this pins down that behavior explicitly.
func TestWinnerBothZeroPawnsIsDefined(t *testing.T) {
	var b Board
	w := Winner(&b)
	if w != Black {
		t.Fatalf("Winner on an all-empty board = %v, want Black (White-zero-pawns checked first)", w)
	}
}

// --- Win condition 3 (turn-dependent: no legal move) is exercised via
// GameState in state_test.go, since Winner() itself is turn-agnostic.
