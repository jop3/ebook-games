package game

import (
	"image"
	"testing"
)

func TestNewGameStartsAtStepMoveBlackToMove(t *testing.T) {
	s := NewGame(OpponentHotseat)
	if s.Turn != Black {
		t.Fatal("Black should move first")
	}
	if s.Step != StepMove {
		t.Fatal("a fresh game should start awaiting a queen move, not a shot")
	}
	if s.Phase != PhasePlaying {
		t.Fatal("a fresh game should be PhasePlaying")
	}
}

// TestTwoPhaseTurnOrder is the central gotcha the spec calls out: the shot
// must be legal FROM THE NEW SQUARE, not the old one, and a shot cannot be
// attempted before the move that makes it possible.
func TestTwoPhaseTurnOrder(t *testing.T) {
	s := NewGame(OpponentHotseat)

	// Shooting before moving must be rejected outright (Step is StepMove).
	if s.Shoot(image.Pt(3, 5)) {
		t.Fatal("Shoot must be rejected before the move half of the turn has happened")
	}
	if s.Step != StepMove || s.Turn != Black {
		t.Fatal("a rejected Shoot must not change Step or Turn")
	}

	// A queen move from (3,0) to (3,5) is legal on the starting board.
	from, to := image.Pt(3, 0), image.Pt(3, 5)
	if !s.MoveQueen(from, to) {
		t.Fatal("the setup move should be legal")
	}
	if s.Step != StepShoot {
		t.Fatal("after a legal move, Step must advance to StepShoot")
	}
	if s.Pending != to {
		t.Fatalf("Pending = %v, want %v (the queen's new square)", s.Pending, to)
	}
	if s.Turn != Black {
		t.Fatal("Turn must not change until the shoot half also completes")
	}

	// Moving again (a second queen move) must be rejected: we're mid-turn.
	if s.MoveQueen(image.Pt(6, 0), image.Pt(6, 5)) {
		t.Fatal("a second MoveQueen must be rejected while a shot is pending")
	}

	// The shot must come from the NEW square (3,5), not the OLD one (3,0):
	// a destination that is only reachable from the old square (e.g. requires
	// a ray direction impossible from (3,5)) must be rejected.
	// From (3,0) one could reach (0,0) horizontally (impossible from (3,5),
	// which is not on the same rank/file/diagonal as (0,0): dx=-3, dy=-5).
	if s.Shoot(image.Pt(0, 0)) {
		t.Fatal("a shot destination reachable only from the OLD square must be rejected: shots come from the queen's new square")
	}
	if s.Step != StepShoot {
		t.Fatal("a rejected Shoot must not advance the turn")
	}

	// A legal shot from the new square (3,5) completes the turn.
	if !s.Shoot(image.Pt(3, 8)) {
		t.Fatal("a legal shot from the new square should be accepted")
	}
	if s.Step != StepMove {
		t.Fatal("after a completed turn, Step should reset to StepMove")
	}
	if s.Turn != White {
		t.Fatal("after Black's completed turn, it should be White's turn")
	}
	if s.Board.At(3, 8) != Burned {
		t.Fatal("the shot square should now be Burned")
	}
	if s.Board.At(3, 5) != QueenBlack {
		t.Fatal("Black's queen should remain at its new square")
	}
	if s.Board.At(3, 0) != Empty {
		t.Fatal("the vacated origin square should remain Empty")
	}
}

func TestMoveQueenRejectsIllegalMoves(t *testing.T) {
	s := NewGame(OpponentHotseat)
	if s.MoveQueen(image.Pt(3, 9), image.Pt(3, 5)) {
		t.Fatal("moving a White queen on Black's turn must be rejected")
	}
	if s.MoveQueen(image.Pt(3, 0), image.Pt(4, 2)) {
		t.Fatal("a non-queen-line move must be rejected")
	}
	if s.Step != StepMove || s.Turn != Black {
		t.Fatal("rejected moves must not change Step or Turn")
	}
}

func TestCompleteTurnEndsGameWhenOpponentBoxedIn(t *testing.T) {
	s := NewGame(OpponentHotseat)
	// Build an artificial near-endgame: Black's queen at (4,4) is about to
	// move to (5,4) and shoot back to (4,4); White's only queen, at (0,0),
	// is fully boxed in by burned squares already, so completing Black's
	// turn should immediately end the game with Black as the winner.
	var b Board
	b.set(4, 4, QueenBlack)
	b.set(0, 0, QueenWhite)
	b.set(1, 0, Burned)
	b.set(0, 1, Burned)
	b.set(1, 1, Burned)
	s.Board = b
	s.Turn = Black
	s.Step = StepMove

	if !s.MoveQueen(image.Pt(4, 4), image.Pt(5, 4)) {
		t.Fatal("setup move should be legal")
	}
	if !s.Shoot(image.Pt(4, 4)) {
		t.Fatal("setup shot should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("the game should end the instant White (to move next) has no legal move")
	}
	if s.Winner() != QueenBlack {
		t.Fatalf("Winner() = %v, want QueenBlack", s.Winner())
	}
}

func TestNoActionsAcceptedAfterGameOver(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Phase = PhaseDone
	if s.MoveQueen(image.Pt(3, 0), image.Pt(3, 5)) {
		t.Fatal("MoveQueen must be rejected once the game is over")
	}
	s.Step = StepShoot
	if s.Shoot(image.Pt(3, 5)) {
		t.Fatal("Shoot must be rejected once the game is over")
	}
}

func TestAITurnOnlyForOpponentAI(t *testing.T) {
	hotseat := NewGame(OpponentHotseat)
	hotseat.Turn = White
	if hotseat.AITurn() {
		t.Fatal("hotseat mode must never report AITurn")
	}

	vsAI := NewGame(OpponentAI)
	if vsAI.AITurn() {
		t.Fatal("it's Black's (the human's) move first; AITurn must be false")
	}
	vsAI.Turn = White
	if !vsAI.AITurn() {
		t.Fatal("with OpponentAI and White to move, AITurn must be true")
	}
}

// TestStepAIPlaysOneHalfPerCall checks the aiPend-across-two-frames pattern
// used by main.go: StepAI performs the move half first (Step becomes
// StepShoot, Turn is still White/the AI), then, on a second call, the
// already-decided shot (Turn flips to Black).
func TestStepAIPlaysOneHalfPerCall(t *testing.T) {
	s := NewGame(OpponentAI)
	s.Turn = White // pretend it's already the AI's turn
	before := s.Board

	if !s.StepAI() {
		t.Fatal("StepAI should play the move half")
	}
	if s.Step != StepShoot {
		t.Fatal("after the move half, Step should be StepShoot")
	}
	if s.Turn != White {
		t.Fatal("Turn must not change until the shoot half also completes")
	}
	if s.Board == before {
		t.Fatal("the move half should have changed the board (queen relocated)")
	}
	mid := s.Board

	if !s.StepAI() {
		t.Fatal("StepAI should play the shoot half")
	}
	if s.Step != StepMove {
		t.Fatal("after the shoot half, Step should reset to StepMove")
	}
	if s.Turn != Black {
		t.Fatal("after White's completed turn, it should be Black's turn")
	}
	if s.Board == mid {
		t.Fatal("the shoot half should have burned a square")
	}
	if !s.HasLastBurned {
		t.Fatal("HasLastBurned should be set after a completed turn")
	}
}
