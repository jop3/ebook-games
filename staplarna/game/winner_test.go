package game

import "testing"

// TestTypeEliminationNotTotalPieceCount is the classic gotcha the spec calls
// out explicitly: a side with plenty of TOTAL pieces left, but zero of one
// SPECIFIC type, has already lost. This test would pass a buggy
// "total pieces == 0" implementation as easily as a correct one UNLESS it
// specifically constructs a position with many total pieces but one type at
// zero — which is exactly what it does.
func TestTypeEliminationNotTotalPieceCount(t *testing.T) {
	b := NewBoard()
	// Black has plenty of total pieces (10 Tott, 9 Tzarra = 19), but ZERO
	// Tzaar anywhere on the board.
	for i, p := range AllPoints()[:10] {
		_ = i
		b.Stacks[p] = Stack{Owner: Black, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	}
	for _, p := range AllPoints()[10:19] {
		b.Stacks[p] = Stack{Owner: Black, Type: Tzarra, Height: 1, Comp: [3]int{Tzarra: 1}}
	}
	// White has a full, untouched army.
	for _, p := range AllPoints()[19:25] {
		b.Stacks[p] = Stack{Owner: White, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	}

	if total := b.TypeCount(Black, Tott) + b.TypeCount(Black, Tzarra) + b.TypeCount(Black, Tzaar); total != 19 {
		t.Fatalf("setup sanity check failed: Black total = %d, want 19", total)
	}
	if b.TypeCount(Black, Tzaar) != 0 {
		t.Fatalf("setup sanity check failed: Black should have exactly 0 Tzaar, got %d", b.TypeCount(Black, Tzaar))
	}

	loser := EliminatedSide(b)
	if loser != Black {
		t.Fatalf("EliminatedSide = %v, want Black (0 Tzaar, despite 19 total pieces remaining)", loser)
	}
}

// TestEliminationOnAnyOfTheThreeTypes checks all 3 types independently
// trigger the elimination (Tzaar, Tzarra, and Tott each hitting zero must
// each, on their own, end the game for that side).
func TestEliminationOnAnyOfTheThreeTypes(t *testing.T) {
	for _, typ := range AllTypes {
		b := NewBoard()
		pts := AllPoints()
		i := 0
		for _, t2 := range AllTypes {
			if t2 == typ {
				continue // leave this one at zero for Black
			}
			b.Stacks[pts[i]] = Stack{Owner: Black, Type: t2, Height: 1, Comp: func() [3]int { var c [3]int; c[t2] = 1; return c }()}
			i++
		}
		// White untouched.
		b.Stacks[pts[i]] = Stack{Owner: White, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}

		if got := EliminatedSide(b); got != Black {
			t.Errorf("type %v at zero for Black: EliminatedSide = %v, want Black", typ, got)
		}
	}
}

// TestNoWinnerWithFullArmies: a fresh, fully-placed board (both sides at
// their starting 6/9/15) has no eliminated side.
func TestNoWinnerWithFullArmies(t *testing.T) {
	b := NewBoard()
	pts := AllPoints()
	i := 0
	place := func(side Side) {
		for _, typ := range AllTypes {
			for n := 0; n < StartCount(typ); n++ {
				var c [3]int
				c[typ] = 1
				b.Stacks[pts[i]] = Stack{Owner: side, Type: typ, Height: 1, Comp: c}
				i++
			}
		}
	}
	place(Black)
	place(White)
	if len(pts) < i {
		t.Fatalf("not enough board cells (%d) to place both full armies (%d)", len(pts), i)
	}
	if got := EliminatedSide(b); got != None {
		t.Fatalf("EliminatedSide with both full armies = %v, want None", got)
	}
}

// TestEliminatedSideIsPureBoardScan: EliminatedSide doesn't care about turn
// order or game phase — it's a pure function of the board.
func TestEliminatedSideIsPureBoardScan(t *testing.T) {
	b := NewBoard()
	if got := EliminatedSide(b); got != None {
		t.Fatalf("an empty board should report None (not a false elimination), got %v", got)
	}
}
