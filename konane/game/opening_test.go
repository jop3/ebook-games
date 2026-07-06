package game

import (
	"image"
	"testing"
)

// TestOpeningExactTwoStepRemoval walks the opening phase exactly as the rules
// describe it: Black removes one of the two center stones, then White removes
// one of its own stones orthogonally adjacent to the resulting gap, and only
// then does normal jump-play begin with Black to move.
func TestOpeningExactTwoStepRemoval(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Phase != PhaseOpeningBlackRemove {
		t.Fatalf("a fresh game should start in PhaseOpeningBlackRemove, got %v", s.Phase)
	}

	opts := CenterRemovalOptions()
	if len(opts) != 2 {
		t.Fatalf("expected exactly 2 center removal options, got %d", len(opts))
	}
	for _, p := range opts {
		if s.Board.At(p.X, p.Y) != Black {
			t.Fatalf("center removal option %v should hold a Black stone before the opening, got %v", p, s.Board.At(p.X, p.Y))
		}
	}

	// A non-center cell must be rejected outright.
	if s.RemoveOpeningBlack(image.Pt(0, 0)) {
		t.Fatal("removing a non-center stone should be illegal during the opening")
	}
	if s.Phase != PhaseOpeningBlackRemove {
		t.Fatal("an illegal opening removal must not advance the phase")
	}

	gap := opts[0]
	if !s.RemoveOpeningBlack(gap) {
		t.Fatalf("removing center stone %v should be legal", gap)
	}
	if s.Board.At(gap.X, gap.Y) != Empty {
		t.Fatal("the chosen center stone should now be empty")
	}
	if s.Phase != PhaseOpeningWhiteRemove {
		t.Fatalf("phase should advance to PhaseOpeningWhiteRemove, got %v", s.Phase)
	}

	// White's options must be exactly its own stones orthogonally adjacent to
	// the gap — every one of them, since a fresh checkerboard guarantees all 4
	// orthogonal neighbors of a Black cell are White.
	whiteOpts := s.OpeningWhiteOptions()
	if len(whiteOpts) != 4 {
		t.Fatalf("expected 4 White neighbors of an interior gap, got %d: %v", len(whiteOpts), whiteOpts)
	}
	for _, p := range whiteOpts {
		dx, dy := p.X-gap.X, p.Y-gap.Y
		orth := (dx == 0 && (dy == 1 || dy == -1)) || (dy == 0 && (dx == 1 || dx == -1))
		if !orth {
			t.Fatalf("White option %v is not orthogonally adjacent to the gap %v", p, gap)
		}
		if s.Board.At(p.X, p.Y) != White {
			t.Fatalf("White option %v does not hold a White stone", p)
		}
	}

	// A stone that is not adjacent to the gap must be rejected.
	if s.RemoveOpeningWhite(image.Pt(0, 0)) {
		t.Fatal("removing a stone not adjacent to the gap should be illegal")
	}
	// A Black stone (even if somehow adjacent) must be rejected — White can
	// only remove its own stone.
	blackAdjacent := image.Pt(gap.X, gap.Y) // the gap itself: not White at all
	if s.RemoveOpeningWhite(blackAdjacent) {
		t.Fatal("the gap itself (empty) must be rejected as a White removal target")
	}

	choice := whiteOpts[0]
	if !s.RemoveOpeningWhite(choice) {
		t.Fatalf("removing adjacent White stone %v should be legal", choice)
	}
	if s.Board.At(choice.X, choice.Y) != Empty {
		t.Fatal("the chosen White stone should now be empty")
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("phase should advance to PhasePlaying once the opening completes, got %v", s.Phase)
	}
	if s.Turn != Black {
		t.Fatalf("Black should move first after the opening, got Turn=%v", s.Turn)
	}

	// Exactly 2 stones should have left the board (one Black, one White);
	// everything else stays exactly as NewBoard() set it up.
	fresh := NewBoard()
	diffs := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if s.Board.At(x, y) != fresh.At(x, y) {
				diffs++
			}
		}
	}
	if diffs != 2 {
		t.Fatalf("opening should change exactly 2 cells, changed %d", diffs)
	}
}

// TestOpeningRemovalOrderEnforced checks that the two opening steps cannot be
// taken out of order or skipped.
func TestOpeningRemovalOrderEnforced(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	opts := CenterRemovalOptions()

	// White cannot remove before Black has opened the gap.
	if s.RemoveOpeningWhite(image.Pt(opts[0].X+1, opts[0].Y)) {
		t.Fatal("White's opening removal must be illegal before Black has moved")
	}
	if s.Phase != PhaseOpeningBlackRemove {
		t.Fatal("phase must remain PhaseOpeningBlackRemove")
	}

	if !s.RemoveOpeningBlack(opts[0]) {
		t.Fatal("setup: Black's opening removal should succeed")
	}

	// A normal jump attempt during PhaseOpeningWhiteRemove must be rejected —
	// there is no jump-play until the opening fully completes.
	if s.StartJump(image.Pt(0, 1), image.Pt(0, 3)) {
		t.Fatal("StartJump must be illegal while still in the opening phase")
	}
}
