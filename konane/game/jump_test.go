package game

import (
	"image"
	"testing"
)

// emptyBoard returns a board with every cell cleared, for constructing exact
// test positions instead of using the fully-packed starting layout.
func emptyBoard() Board {
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.set(x, y, Empty)
		}
	}
	return b
}

// --- LegalJumpsFrom: the exact requirements for a legal jump ---------------

func TestLegalJumpRequiresEnemyThenEmpty(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black) // mover at (3,3)
	b.set(3, 4, White) // enemy immediately below
	// (3,5) already Empty: legal landing.

	js := b.LegalJumpsFrom(image.Pt(3, 3), Black)
	if len(js) != 1 {
		t.Fatalf("expected exactly 1 legal jump, got %d: %v", len(js), js)
	}
	want := Jump{From: image.Pt(3, 3), Over: image.Pt(3, 4), To: image.Pt(3, 5)}
	if js[0] != want {
		t.Fatalf("jump = %+v, want %+v", js[0], want)
	}
}

func TestLegalJumpRejectsFriendlyAdjacent(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(3, 4, Black) // FRIENDLY, not enemy — must not be jumpable

	js := b.LegalJumpsFrom(image.Pt(3, 3), Black)
	for _, j := range js {
		if j.To == (image.Point{X: 3, Y: 5}) {
			t.Fatalf("jumping over a friendly stone must not be legal, got %+v", j)
		}
	}
}

func TestLegalJumpRejectsOccupiedLanding(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(3, 4, White)
	b.set(3, 5, Black) // landing cell occupied — must block the jump

	js := b.LegalJumpsFrom(image.Pt(3, 3), Black)
	for _, j := range js {
		if j.To == (image.Point{X: 3, Y: 5}) {
			t.Fatalf("jump onto an occupied cell must not be legal, got %+v", j)
		}
	}
}

func TestLegalJumpRejectsEmptyAdjacent(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	// (3,4) left Empty: nothing to jump over.

	js := b.LegalJumpsFrom(image.Pt(3, 3), Black)
	if len(js) != 0 {
		t.Fatalf("no adjacent enemy: expected 0 jumps, got %v", js)
	}
}

func TestLegalJumpRejectsOffBoard(t *testing.T) {
	// Mover on the rightmost column; the only rightward jump would land at
	// x==Size (off-board), so it must not appear, even though there is an
	// enemy stone immediately to the right.
	b := emptyBoard()
	b.set(Size-1, 3, Black) // (x=Size-1, y=3): last column, nothing beyond it

	js := b.LegalJumpsFrom(image.Pt(Size-1, 3), Black)
	if len(js) != 0 {
		t.Fatalf("mover at the right edge with nothing beyond it should have 0 jumps, got %v", js)
	}

	// A more direct off-board check: an enemy stone one column left of the
	// edge (in bounds), with the landing cell beyond the mover being
	// off-board, must not be legal.
	b2 := emptyBoard()
	b2.set(Size-2, 3, Black) // one in from the edge
	b2.set(Size-1, 3, White) // enemy on the last column
	js2 := b2.LegalJumpsFrom(image.Pt(Size-2, 3), Black)
	for _, j := range js2 {
		if j.To.X >= Size {
			t.Fatalf("jump off the right edge should not be legal: %+v", j)
		}
	}
	if len(js2) != 0 {
		t.Fatalf("landing beyond the right edge is off-board; expected 0 jumps, got %v", js2)
	}

	// Same check at the top edge (y=0): jumping upward off-board.
	b3 := emptyBoard()
	b3.set(3, 1, Black) // one row in from the top
	b3.set(3, 0, White) // enemy on the top row; landing above it is off-board
	js3 := b3.LegalJumpsFrom(image.Pt(3, 1), Black)
	if len(js3) != 0 {
		t.Fatalf("jump off the top edge should not be legal, got %v", js3)
	}
}

func TestLegalJumpsFromWrongOwnerIsEmpty(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, White) // NOT Black
	b.set(3, 4, White)
	if js := b.LegalJumpsFrom(image.Pt(3, 3), Black); len(js) != 0 {
		t.Fatalf("querying Black jumps from a White-occupied cell must return none, got %v", js)
	}
}

// --- Apply -------------------------------------------------------------------

func TestApplyMovesAndCaptures(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(3, 4, White)
	j := Jump{From: image.Pt(3, 3), Over: image.Pt(3, 4), To: image.Pt(3, 5)}
	nb := b.Apply(j, Black)
	if nb.At(3, 3) != Empty {
		t.Fatal("origin cell should be vacated")
	}
	if nb.At(3, 4) != Empty {
		t.Fatal("jumped-over enemy stone should be removed")
	}
	if nb.At(3, 5) != Black {
		t.Fatal("mover should now occupy the landing cell")
	}
}

// --- Chain generation: stopping early vs continuing -------------------------

func TestGenerateChainsIncludesStopEarlyAndContinue(t *testing.T) {
	b := emptyBoard()
	// A double-jump setup: Black at (1,1) can jump White at (2,1) landing
	// (3,1), then jump White at (4,1) landing (5,1).
	b.set(1, 1, Black)
	b.set(2, 1, White)
	b.set(4, 1, White)

	chains := GenerateChains(b, Black)
	var sawLen1, sawLen2 bool
	for _, c := range chains {
		if len(c) == 1 && c[0].To == (image.Point{X: 3, Y: 1}) {
			sawLen1 = true
		}
		if len(c) == 2 && c[1].To == (image.Point{X: 5, Y: 1}) {
			sawLen2 = true
		}
	}
	if !sawLen1 {
		t.Errorf("expected a length-1 chain (stopping after the first jump) among %v", chains)
	}
	if !sawLen2 {
		t.Errorf("expected a length-2 chain (continuing into the second jump) among %v", chains)
	}
}

func TestHasAnyJumpTrueAndFalse(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(3, 4, White)
	if !b.HasAnyJump(Black) {
		t.Fatal("Black should have a legal jump")
	}

	isolated := emptyBoard()
	isolated.set(0, 0, White) // no enemy anywhere adjacent: no legal jump
	if isolated.HasAnyJump(White) {
		t.Fatal("an isolated stone with no adjacent enemy should have no legal jump")
	}

	empty := emptyBoard()
	if empty.HasAnyJump(Black) || empty.HasAnyJump(White) {
		t.Fatal("an empty board should have zero legal jumps for either side")
	}
}
