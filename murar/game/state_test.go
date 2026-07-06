package game

import (
	"image"
	"testing"
)

func TestGameStatePlayMoveLegalAdvancesTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Turn != P1 {
		t.Fatal("P1 should move first")
	}
	if !s.PlayMove(image.Pt(4, 7)) {
		t.Fatal("stepping forward should be legal")
	}
	if s.Turn != P2 {
		t.Fatal("turn should pass to P2")
	}
	if s.Board.Pawns[P1] != image.Pt(4, 7) {
		t.Fatalf("P1 pawn = %v, want (4,7)", s.Board.Pawns[P1])
	}
}

func TestGameStatePlayMoveIllegalRejected(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.PlayMove(image.Pt(4, 6)) {
		t.Fatal("a 2-cell plain move must be rejected")
	}
	if s.Turn != P1 {
		t.Fatal("an illegal move must not change the turn")
	}
	if s.Board.Pawns[P1] != image.Pt(4, 8) {
		t.Fatal("an illegal move must not move the pawn")
	}
}

func TestGameStatePlaceWallLegalDecrementsAndAdvances(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	before := s.Board.WallsLeft[P1]
	if !s.PlaceWall(Wall{X: 3, Y: 3, Orient: Horizontal}) {
		t.Fatal("a legal wall placement should succeed")
	}
	if s.Board.WallsLeft[P1] != before-1 {
		t.Fatalf("WallsLeft[P1] = %d, want %d", s.Board.WallsLeft[P1], before-1)
	}
	if s.Turn != P2 {
		t.Fatal("turn should pass to P2 after a wall placement")
	}
	if s.LastWall == nil || *s.LastWall != (Wall{X: 3, Y: 3, Orient: Horizontal}) {
		t.Fatalf("LastWall = %v, want the wall just placed", s.LastWall)
	}
}

func TestGameStatePlaceWallRejectsWhenNoWallsLeft(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Board.WallsLeft[P1] = 0
	if s.PlaceWall(Wall{X: 3, Y: 3, Orient: Horizontal}) {
		t.Fatal("placing a wall with none left must be rejected")
	}
}

func TestGameStatePlaceWallRejectsIllegalPlacement(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	w := Wall{X: 3, Y: 3, Orient: Horizontal}
	s.Board.place(w)
	s.Board.WallsLeft[P1]-- // keep the board consistent with the direct placement
	before := s.Board.WallsLeft[P1]
	if s.PlaceWall(w) {
		t.Fatal("an overlapping wall placement must be rejected")
	}
	if s.Board.WallsLeft[P1] != before {
		t.Fatal("a rejected wall placement must not consume a wall")
	}
	if s.Turn != P1 {
		t.Fatal("a rejected wall placement must not advance the turn")
	}
}

func TestGameStateWinEndsGame(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Board.Pawns[P1] = image.Pt(4, 1)
	s.Board.Pawns[P2] = image.Pt(0, 4) // move P2 off (4,0), P1's intended destination
	if !s.PlayMove(image.Pt(4, 0)) {
		t.Fatal("the winning move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once P1 reaches row 0")
	}
	if w, ok := s.Winner(); !ok || w != P1 {
		t.Fatalf("Winner() = %v,%v want P1,true", w, ok)
	}
}

func TestGameStateAITurnAndStepAI(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	if s.AITurn() {
		t.Fatal("P1 (human) moves first; it should not be the AI's turn yet")
	}
	if !s.PlayMove(image.Pt(4, 7)) {
		t.Fatal("P1's opening move should be legal")
	}
	if !s.AITurn() {
		t.Fatal("it should now be P2's (the AI's) turn")
	}
	before := s.Board
	if !s.StepAI() {
		t.Fatal("StepAI should make a move")
	}
	if s.Board == before {
		t.Fatal("StepAI should have changed the board")
	}
	if s.AITurn() {
		t.Fatal("after the AI moves, turn should pass back to the human")
	}
}

func TestGameStateNoInputAfterGameEnds(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Board.Pawns[P1] = image.Pt(4, 1)
	s.Board.Pawns[P2] = image.Pt(0, 4) // move P2 off (4,0), P1's intended destination
	if !s.PlayMove(image.Pt(4, 0)) {
		t.Fatal("setup: P1's winning move should be legal")
	} // P1 wins
	if s.PlayMove(image.Pt(4, 1)) {
		t.Fatal("no move should be accepted once the game is done")
	}
	if s.PlaceWall(Wall{X: 0, Y: 0, Orient: Horizontal}) {
		t.Fatal("no wall placement should be accepted once the game is done")
	}
}
