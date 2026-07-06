package game

import (
	"image"
	"testing"
)

// TestRemainingSquaresAndWinner checks the win condition directly: whoever
// has fewer total unplaced squares in hand wins; equal totals tie.
func TestRemainingSquaresAndWinner(t *testing.T) {
	s := NewGame(OpponentHotseat)
	full := s.Hand(Black).RemainingSquares()
	if full != s.Hand(White).RemainingSquares() {
		t.Fatalf("both hands should start equal, got %d vs %d", full, s.Hand(White).RemainingSquares())
	}
	if full == 0 {
		t.Fatal("a fresh hand should not be empty")
	}

	// Empty hand -> Black.
	*s.Hand(Black) = Hand{}
	if s.Winner() != Black {
		t.Fatalf("Black with 0 remaining should win, got %v", s.Winner())
	}

	// Restore equal hands -> tie.
	*s.Hand(Black) = NewHand()
	if s.Winner() != Empty {
		t.Fatalf("equal hands should tie, got %v", s.Winner())
	}

	// White with strictly fewer remaining squares should win.
	wh := NewHand()
	wh[8] = false // remove Allé (a 4-square piece) from White's hand
	s.Hands[1] = wh
	if s.Winner() != White {
		t.Fatalf("White with fewer remaining squares should win, got %v", s.Winner())
	}
}

// TestPlaceCathedralThenTurnOrder checks the opening sequence: the game
// starts in PhaseCathedral with Black "to move" (placing the neutral
// piece); once placed, it's PhasePlaying with White to move first (Black
// used its first turn on the neutral piece).
func TestPlaceCathedralThenTurnOrder(t *testing.T) {
	s := NewGame(OpponentHotseat)
	if s.Phase != PhaseCathedral {
		t.Fatalf("game should start in PhaseCathedral, got %v", s.Phase)
	}

	placements := LegalCathedralPlacements(&s.Board)
	if len(placements) == 0 {
		t.Fatal("an empty board should offer Cathedral placements")
	}
	anchor := placements[0].Anchor

	if !s.PlaceCathedral(anchor) {
		t.Fatal("a legal Cathedral placement should succeed")
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase should be PhasePlaying after the Cathedral is placed, got %v", s.Phase)
	}
	if s.Turn != White {
		t.Fatalf("White should move first after Black places the Cathedral, got %v", s.Turn)
	}
	if s.Board.Count(Cathedral) != CathedralShape.Size() {
		t.Fatalf("Cathedral cell count = %d, want %d", s.Board.Count(Cathedral), CathedralShape.Size())
	}

	// A second attempt (already placed) must fail.
	if s.PlaceCathedral(anchor) {
		t.Fatal("PlaceCathedral must fail once already placed")
	}
}

// TestPlaceRejectsWrongTurnAndUnavailablePiece checks basic input guards.
func TestPlaceRejectsWrongTurnAndUnavailablePiece(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.PlaceCathedral(LegalCathedralPlacements(&s.Board)[0].Anchor)
	// Turn is now White.
	placements := LegalPlacementsForOrientation(&s.Board, 0, 0)
	if len(placements) == 0 {
		t.Fatal("setup: monomino should have legal placements")
	}
	anchor := placements[0].Anchor

	if s.Place(Black, 0, 0, anchor) {
		t.Fatal("Black must not be able to place out of turn")
	}
	if !s.Place(White, 0, 0, anchor) {
		t.Fatal("White's legal placement should succeed")
	}
	if s.Hand(White)[0] {
		t.Fatal("piece 0 should now be marked unavailable in White's hand")
	}
	// White (now placed piece 0) tries to place the SAME piece id again while
	// it's Black's turn anyway (double guard: turn AND availability).
	if s.Place(White, 0, 0, image.Pt(0, 0)) {
		t.Fatal("White must not place again out of turn")
	}
}

// TestPlaceAppliesEnclosureAndReturnsCapturedPieceToHand exercises Place end
// to end: constructing a position one move from a capture, then checking the
// captured piece actually becomes available again in the opponent's hand.
func TestPlaceAppliesEnclosureAndReturnsCapturedPieceToHand(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Phase = PhasePlaying
	s.Turn = Black

	// White's monomino (piece 0) sits at (5,5), walled on 3 sides by Black;
	// Black's move below completes the 4th side and should capture it.
	place(&s.Board, 5, 5, White, 0)
	*s.Hand(White) = Hand{} // White has already placed everything except...
	s.Hand(White)[0] = false
	place(&s.Board, 4, 5, Black, 1)
	place(&s.Board, 6, 5, Black, 2)
	place(&s.Board, 5, 4, Black, 3)
	// Black still has a monomino (piece id 1, "Koja") available to complete
	// the south wall at (5,6).
	*s.Hand(Black) = Hand{}
	s.Hand(Black)[1] = true

	if !s.Place(Black, 1, 0, image.Pt(5, 6)) {
		t.Fatal("Black's enclosing placement should be legal")
	}
	if s.Board.At(5, 5) != Empty {
		t.Fatal("White's trapped piece should have been captured")
	}
	if !s.Hand(White)[0] {
		t.Fatal("captured piece should be returned to White's hand as available")
	}
	if len(s.LastCaptured) != 1 || s.LastCaptured[0] != (image.Point{X: 5, Y: 5}) {
		t.Fatalf("LastCaptured = %v, want [(5,5)]", s.LastCaptured)
	}
}

// TestAdvancePassesStuckSideAndEndsGameWhenNeitherCanMove checks the pass
// and end-of-game logic: a side with zero legal placements is skipped (their
// opponent goes again) and the game ends only once NEITHER side has a legal
// placement.
func TestAdvancePassesStuckSideAndEndsGameWhenNeitherCanMove(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Phase = PhasePlaying
	s.Turn = Black

	// Fill the whole board with Black except one free cell, so White (which
	// only owns pieces bigger than 1 square) cannot move, but Black (who
	// still holds a monomino) can.
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if x == 0 && y == 0 {
				continue
			}
			s.Board.Owner[y][x] = Black
		}
	}
	*s.Hand(White) = Hand{}
	s.Hand(White)[8] = true // Allé: a straight tetromino, cannot fit anywhere
	*s.Hand(Black) = Hand{}
	s.Hand(Black)[0] = true // Stuga: a monomino, fits at (0,0)

	if !s.Place(Black, 0, 0, image.Pt(0, 0)) {
		t.Fatal("Black's placement into the last free cell should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatalf("with the board now full and neither side able to place, Phase should be Done, got %v", s.Phase)
	}
}

// TestAdvanceSkipsStuckSideButKeepsGameGoing checks a stuck side is skipped
// (not ending the game) while the OTHER side still has legal placements.
func TestAdvanceSkipsStuckSideButKeepsGameGoing(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Phase = PhasePlaying
	s.Turn = Black

	// Leave plenty of empty room, but give White only a piece that cannot
	// possibly fit: bigger than the board's remaining space is unnecessary —
	// instead, just empty White's hand entirely (simplest possible "stuck").
	*s.Hand(White) = Hand{}
	*s.Hand(Black) = NewHand()

	placements := LegalPlacementsForOrientation(&s.Board, 0, 0)
	if !s.Place(Black, 0, 0, placements[0].Anchor) {
		t.Fatal("Black's placement should be legal")
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("game should continue (Black still has pieces and room), got Phase=%v", s.Phase)
	}
	if s.Turn != Black {
		t.Fatalf("White has nothing to place, so it should stay Black's turn, got %v", s.Turn)
	}
}
