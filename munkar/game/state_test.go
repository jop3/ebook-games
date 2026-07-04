package game

import (
	"image"
	"testing"
)

func TestNewGameStartsBlackUnconstrained(t *testing.T) {
	s := NewGameSeeded(ModeHotseat, 0, 1)
	if s.Turn != Black {
		t.Fatal("Black should move first")
	}
	if s.HasLast {
		t.Fatal("HasLast should be false before any move")
	}
	if len(s.LegalMoves()) != Size*Size {
		t.Fatalf("first move should allow all %d cells, got %d", Size*Size, len(s.LegalMoves()))
	}
}

func TestPlayAdvancesTurnAndAppliesForcing(t *testing.T) {
	s := NewGameSeeded(ModeHotseat, 0, 1)
	// Force a known orientation at (2,2) so the test doesn't depend on the
	// random tile shuffle.
	s.Board.Line[2][2] = OrientH
	if !s.Play(image.Pt(2, 2)) {
		t.Fatal("first move anywhere should be legal")
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White")
	}
	if !s.HasLast || s.Last != (image.Pt(2, 2)) {
		t.Fatal("Last should now be the played cell")
	}
	forced := s.LegalMoves()
	for _, p := range forced {
		if p.Y != 2 {
			t.Errorf("White should be forced onto row 2 (H), got %v", p)
		}
	}
}

func TestPlayRejectsIllegalCell(t *testing.T) {
	s := NewGameSeeded(ModeHotseat, 0, 1)
	s.Board.Line[0][0] = OrientH
	s.Play(image.Pt(0, 0)) // Black plays (0,0), forces White onto row 0
	before := s.Board
	beforeTurn := s.Turn
	// (0,1) is off row 0 — illegal while forced.
	if s.legal(image.Pt(1, 1)) {
		t.Fatal("setup: (1,1) should not be a legal forced cell")
	}
	if s.Play(image.Pt(1, 1)) {
		t.Fatal("playing off the forced line should be rejected")
	}
	if s.Board != before || s.Turn != beforeTurn {
		t.Fatal("a rejected move must not mutate board or turn")
	}
}

func TestPlayEndsGameOnFive(t *testing.T) {
	s := NewGameSeeded(ModeHotseat, 0, 1)
	for x := 0; x < 4; x++ {
		s.Board.Ring[3][x] = Black
	}
	s.Turn = Black
	s.HasLast = false // simulate an unconstrained move to complete the five
	if !s.Play(image.Pt(4, 3)) {
		t.Fatal("the winning placement should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done immediately on five-in-a-row")
	}
	if s.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black", s.Winner())
	}
}

func TestPlayEndsGameOnFullBoardTiebreak(t *testing.T) {
	s := NewGameSeeded(ModeHotseat, 0, 1)
	// The same pre-verified layout as TestTiebreakWinnerLargerGroupWins
	// (Black's largest group 13 vs White's 8, no five-in-a-row anywhere),
	// but with one White cell — (1,2) — left empty so a real Play() call
	// completes the board exactly into that layout. (1,2) was specifically
	// chosen because refilling it captures nothing, so the final board is
	// exactly the pre-verified layout rather than a further-modified one.
	s.Board = boardFromRows([Size]string{
		"B.BBBB",
		"...B..",
		"..BBBB",
		".BB.B.",
		".B...B",
		"B.BB.B",
	})
	s.Board.Ring[2][1] = Empty // the one cell left to fill; was White
	s.Turn = White
	s.HasLast = false
	if Five(&s.Board, Black) || Five(&s.Board, White) {
		t.Fatal("setup: no five should exist yet")
	}
	if !s.Play(image.Pt(1, 2)) {
		t.Fatal("filling the last cell should be legal")
	}
	if len(s.LastFlips) != 0 {
		t.Fatalf("this particular refill should capture nothing, got %v", s.LastFlips)
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once the board fills")
	}
	if s.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black (larger connected group)", s.Winner())
	}
}

func TestStepAIRespectsForcedLine(t *testing.T) {
	s := NewGameSeeded(ModeAI, DepthEasy, 1)
	s.Board.Line[0][0] = OrientV // column 0
	if !s.Play(image.Pt(0, 0)) {
		t.Fatal("Black's opening move should be legal")
	}
	if !s.AITurn() {
		t.Fatal("it should now be White's (the AI's) turn")
	}
	if !s.StepAI() {
		t.Fatal("StepAI should make a move")
	}
	if s.Last.X != 0 {
		t.Fatalf("the AI should have been forced onto column 0, played %v", s.Last)
	}
}
