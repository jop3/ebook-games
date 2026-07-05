package game

import (
	"image"
	"testing"
)

func TestNewGameSetup(t *testing.T) {
	s := NewGame(OpponentHotseat, ModeCapture, 0)
	if s.Turn != Black {
		t.Fatalf("Turn = %v, want Black to start", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", s.Phase)
	}
	if s.Board.Count(Black) != 9 || s.Board.Count(White) != 9 {
		t.Fatalf("starting counts wrong: black=%d white=%d", s.Board.Count(Black), s.Board.Count(White))
	}
	if s.AITurn() {
		t.Fatal("hotseat is never the AI's turn")
	}
}

func TestPlayLegalMoveAdvancesTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, ModeCapture, 0)
	legal := s.Board.LegalMoves(Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}
	m := legal[0]
	if !s.Play(m.From, m.To) {
		t.Fatalf("Play(%v) should have succeeded", m)
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White after Black moves")
	}
	if s.Board.At(m.To.X, m.To.Y) != Black {
		t.Fatal("Black's man should now be at the destination")
	}
}

func TestPlayRejectsIllegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, ModeCapture, 0)
	// (4,4) is empty and unreachable in one rook move from any Black man's
	// starting square along a clear path is actually reachable (column 4 is
	// open); use a definitely-illegal move instead: moving to a cell already
	// occupied by another Black man.
	from := image.Pt(0, Size-1)
	to := image.Pt(1, Size-1) // occupied by another Black man
	if s.Play(from, to) {
		t.Fatal("moving onto your own man must be rejected")
	}
	if s.Turn != Black {
		t.Fatal("an illegal move must not change the turn")
	}
	// Also reject moving a piece that isn't there / isn't yours.
	if s.Play(image.Pt(4, 4), image.Pt(4, 0)) {
		t.Fatal("moving from an empty square must be rejected")
	}
	if s.Play(image.Pt(0, 0), image.Pt(0, 4)) {
		t.Fatal("Black may not move a White man")
	}
}

func TestAITurnAndStepAI(t *testing.T) {
	s := NewGame(OpponentAI, ModeCapture, DepthEasy)
	if s.AITurn() {
		t.Fatal("should not be the AI's turn before Black (human) has moved")
	}
	legal := s.Board.LegalMoves(Black)
	m := legal[0]
	s.Play(m.From, m.To)
	if !s.AITurn() {
		t.Fatal("should be the AI's (White's) turn after Black moves")
	}
	beforeCount := s.Board.Count(White) + s.Board.Count(Black)
	if !s.StepAI() {
		t.Fatal("StepAI should have played a move")
	}
	if s.AITurn() {
		t.Fatal("turn should have passed back to Black after StepAI")
	}
	if got := s.Board.Count(White) + s.Board.Count(Black); got > beforeCount {
		t.Fatalf("total man count should never increase: got %d, had %d", got, beforeCount)
	}
}

func TestWinEndsGameImmediatelyWithoutAdvancingTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, ModeCapture, 0)
	for i := range s.Board {
		for j := range s.Board[i] {
			s.Board[i][j] = Empty
		}
	}
	// Black to move; the move captures White's last-but-one man, reducing
	// White to a single man and winning immediately.
	s.Board.set(4, 0, Black)
	s.Board.set(3, 4, White)
	s.Board.set(2, 4, Black)
	s.Board.set(4, 8, White) // White's sole survivor
	s.Turn = Black
	if !s.Play(image.Pt(4, 0), image.Pt(4, 4)) {
		t.Fatal("the winning move should be legal and applied")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done immediately once White is reduced to 1 man")
	}
	if w := s.Winner(); w != Black {
		t.Fatalf("Winner() = %v, want Black", w)
	}
	if len(s.LastCaptured) != 1 {
		t.Fatalf("LastCaptured = %v, want exactly the 1 captured White man", s.LastCaptured)
	}
}

func TestFiveInRowGameEndsOnQualifyingLine(t *testing.T) {
	s := NewGame(OpponentHotseat, ModeFiveInRow, 0)
	for i := range s.Board {
		for j := range s.Board[i] {
			s.Board[i][j] = Empty
		}
	}
	// 4 Black men already in a row on a neutral rank; the 5th, played this
	// turn, completes the line and should end the game for Black.
	s.Board.set(0, 4, Black)
	s.Board.set(1, 4, Black)
	s.Board.set(2, 4, Black)
	s.Board.set(3, 4, Black)
	s.Board.set(4, 0, Black) // will slide down to complete the line at (4,4)
	s.Turn = Black
	if !s.Play(image.Pt(4, 0), image.Pt(4, 4)) {
		t.Fatal("the completing move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once the 5-in-a-row line is completed")
	}
	if w := s.Winner(); w != Black {
		t.Fatalf("Winner() = %v, want Black (Fem i rad)", w)
	}
}
