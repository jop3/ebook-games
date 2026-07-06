package game

import (
	"image"
	"testing"
)

func TestNewGameStartsWithBlackToMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Turn != Black {
		t.Fatalf("Turn = %v, want Black (Black always moves first)", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", s.Phase)
	}
	if s.Board.Count(Black) != Cols*2 || s.Board.Count(White) != Cols*2 {
		t.Fatal("fresh game should start with the standard two-rank setup")
	}
}

func TestPlayRejectsIllegalMoves(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	before := s.Board
	// Sideways is never legal.
	if s.Play(image.Pt(3, Rows-2), image.Pt(4, Rows-2)) {
		t.Fatal("sideways move should be rejected")
	}
	// Wrong side's pawn (White during Black's turn).
	if s.Play(image.Pt(3, 0), image.Pt(3, 1)) {
		t.Fatal("moving the non-active side's pawn should be rejected")
	}
	if s.Board != before || s.Turn != Black {
		t.Fatal("an illegal Play must not mutate board or turn")
	}
}

func TestPlayAdvancesTurnAndAppliesMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if !s.Play(image.Pt(0, Rows-2), image.Pt(0, Rows-3)) {
		t.Fatal("a legal opening move should be accepted")
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White after Black's move")
	}
	if s.Board.At(0, Rows-3) != Black || s.Board.At(0, Rows-2) != Empty {
		t.Fatal("the pawn should have relocated")
	}
	if s.LastCaptured {
		t.Fatal("a straight move should never be recorded as a capture")
	}
}

func TestPlayRecordsCapture(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Board = Board{}
	s.Board.set(3, 3, Black)
	s.Board.set(2, 2, White)
	s.Turn = Black
	if !s.Play(image.Pt(3, 3), image.Pt(2, 2)) {
		t.Fatal("the diagonal capture should be legal")
	}
	if !s.LastCaptured {
		t.Fatal("LastCaptured should be true after a diagonal capture")
	}
	if s.Board.Count(White) != 0 {
		t.Fatal("the captured White pawn should be gone")
	}
}

// --- Win condition 1 via GameState: reaching the goal rank ends the game.

func TestStatePlayWinByReachingGoalRank(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Board = Board{}
	s.Board.set(3, 1, Black) // one step from row 0
	s.Board.set(0, 5, White)
	s.Turn = Black
	if !s.Play(image.Pt(3, 1), image.Pt(3, 0)) {
		t.Fatal("the winning advance should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once a pawn reaches the goal rank")
	}
	if s.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black", s.Winner())
	}
}

// --- Win condition 2 via GameState: eliminating the opponent's last pawn.

func TestStatePlayWinByEliminatingLastPawn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Board = Board{}
	s.Board.set(3, 3, Black)
	s.Board.set(2, 2, White) // White's only pawn
	s.Turn = Black
	if !s.Play(image.Pt(3, 3), image.Pt(2, 2)) {
		t.Fatal("the winning capture should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once White has zero pawns")
	}
	if s.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black", s.Winner())
	}
}

// --- Win condition 3: the side to move has no legal move at all.

func TestStateWinByOpponentHasNoLegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Board = Board{}
	// Black pawn completely boxed in: White pawns dead ahead and both
	// diagonals, so Black's only pawn (about to become the side to move)
	// has zero legal moves — straight is blocked (occupied, and blocking a
	// straight move is legal regardless of who blocks it), diagonals are
	// blocked by White pawns that would need to be diagonally reachable,
	// but here they occupy the straight-ahead cell instead so there is no
	// legal move in ANY direction.
	s.Board.set(3, 3, Black)
	s.Board.set(3, 2, White) // blocks the only straight destination
	// Diagonals (2,2) and (4,2) left empty on purpose: an empty diagonal is
	// illegal for a pawn (diagonal only ever captures), so those add no
	// legal moves either. Black at (3,3) therefore has zero legal moves.
	s.Board.set(0, 0, White) // White needs a legal move of its own for the turn before
	s.Turn = White
	// White moves first here so it becomes Black's move with zero options.
	if !s.Play(image.Pt(0, 0), image.Pt(0, 1)) {
		t.Fatal("setup: White's move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once Black (to move) has no legal move")
	}
	if s.Winner() != White {
		t.Fatalf("Winner() = %v, want White (Black had no legal move)", s.Winner())
	}
}

func TestAITurnOnlyForWhiteUnderOpponentAI(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	if s.AITurn() {
		t.Fatal("Black moves first; AITurn should be false at game start")
	}
	if !s.Play(image.Pt(0, Rows-2), image.Pt(0, Rows-3)) {
		t.Fatal("Black's opening move should be legal")
	}
	if !s.AITurn() {
		t.Fatal("after Black's move, it should be White's (the AI's) turn")
	}
}

func TestStepAIAppliesALegalMoveAndAdvancesTurn(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	s.Play(image.Pt(0, Rows-2), image.Pt(0, Rows-3))
	before := s.Board
	if !s.StepAI() {
		t.Fatal("StepAI should play a move when it is White's turn")
	}
	if s.Board == before {
		t.Fatal("StepAI should have changed the board")
	}
	if s.Turn != Black {
		t.Fatal("turn should pass back to Black after the AI moves")
	}
}
