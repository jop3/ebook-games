package game

import (
	"image"
	"testing"
)

func TestNewGameSetup(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Turn != Black {
		t.Fatalf("Turn = %v, want Black to start", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", s.Phase)
	}
	if s.Board.Count(Black) != Cols || s.Board.Count(White) != Cols {
		t.Fatalf("starting counts wrong: black=%d white=%d", s.Board.Count(Black), s.Board.Count(White))
	}
	if s.AITurn() {
		t.Fatal("hotseat is never the AI's turn")
	}
}

func TestPlayLegalMoveAdvancesTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
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
	if s.Board.At(m.To.X, m.To.Y) == nil || s.Board.At(m.To.X, m.To.Y).Side != Black {
		t.Fatal("Black's piece should now be at the destination")
	}
}

func TestPlayRejectsIllegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	// Moving onto your own back-rank neighbor is always illegal (occupied by
	// your own piece).
	from := image.Pt(0, 0)
	to := image.Pt(1, 0)
	if s.Play(from, to) {
		t.Fatal("moving onto your own piece must be rejected")
	}
	if s.Turn != Black {
		t.Fatal("an illegal move must not change the turn")
	}
	// Moving from an empty square, or moving the opponent's piece, must also
	// be rejected.
	if s.Play(image.Pt(0, 2), image.Pt(0, 3)) {
		t.Fatal("moving from an empty square must be rejected")
	}
	if s.Play(image.Pt(0, Rows-1), image.Pt(0, Rows-2)) {
		t.Fatal("Black may not move a White piece")
	}
}

func TestAITurnAndStepAI(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	if s.AITurn() {
		t.Fatal("should not be the AI's turn before Black (human) has moved")
	}
	legal := s.Board.LegalMoves(Black)
	m := legal[0]
	s.Play(m.From, m.To)
	if !s.AITurn() {
		t.Fatal("should be the AI's (White's) turn after Black moves")
	}
	if !s.StepAI() {
		t.Fatal("StepAI should have played a move")
	}
	if s.AITurn() {
		t.Fatal("turn should have passed back to Black after StepAI")
	}
}

// --- WIN: King capture, driven through GameState.Play -----------------------

func TestGameStateWinByKingCapture(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	for y := range s.Board {
		for x := range s.Board[y] {
			s.Board[y][x] = nil
		}
	}
	s.Board[2][2] = &Piece{Kind: Triangle, Side: Black}
	s.Board[1][1] = &Piece{Kind: King, Side: White}
	s.Board[2][0] = &Piece{Kind: King, Side: Black} // neutral row, not on anyone's goal rank
	s.Turn = Black
	if !s.Play(image.Pt(2, 2), image.Pt(1, 1)) {
		t.Fatal("the King-capturing move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done immediately once White's King is captured")
	}
	if w, ok := s.Winner(); !ok || w != Black {
		t.Fatalf("Winner() = %v/%v, want Black", w, ok)
	}
	if s.LastCaptured == nil || *s.LastCaptured != (image.Point{X: 1, Y: 1}) {
		t.Fatalf("LastCaptured = %v, want (1,1)", s.LastCaptured)
	}
}

// --- WIN: King reaches the far edge, driven through GameState.Play --------

func TestGameStateWinByKingReachingFarEdge(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	for y := range s.Board {
		for x := range s.Board[y] {
			s.Board[y][x] = nil
		}
	}
	s.Board[4][1] = &Piece{Kind: King, Side: Black} // diagonal mode, one step from y=5
	s.Board[2][3] = &Piece{Kind: King, Side: White} // neutral row, not on anyone's goal rank
	s.Turn = Black
	if !s.Play(image.Pt(1, 4), image.Pt(2, 5)) {
		t.Fatal("the goal-rank move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done immediately once Black's King reaches the far edge")
	}
	if w, ok := s.Winner(); !ok || w != Black {
		t.Fatalf("Winner() = %v/%v, want Black", w, ok)
	}
}
