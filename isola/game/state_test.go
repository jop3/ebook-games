package game

import (
	"image"
	"testing"
)

func TestNewGameStartsInStepMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Phase != PhasePlaying {
		t.Fatal("a new game should be PhasePlaying")
	}
	if s.Step != StepMove {
		t.Fatal("a new game should start awaiting a pawn move, not a removal")
	}
	if s.Turn != Black {
		t.Fatal("Black should move first")
	}
}

func TestPlayMoveThenRemovalCompletesATurn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	from := s.Board.PawnPos(Black)
	to := s.Board.LegalMoves(Black)[0]

	if !s.PlayMove(to) {
		t.Fatal("a legal move should be accepted")
	}
	if s.Step != StepRemove {
		t.Fatal("after moving, Step should advance to StepRemove")
	}
	if s.Turn != Black {
		t.Fatal("the turn must not pass to White until the removal half is also played")
	}
	if s.Board.PawnPos(Black) != to {
		t.Fatal("the pawn should already show at its new square during StepRemove")
	}

	// Removing the just-landed-on square must be rejected.
	if s.PlayRemoval(to) {
		t.Fatal("removing the mover's own new position must be rejected")
	}
	if s.Step != StepRemove {
		t.Fatal("a rejected removal must not advance Step")
	}

	// Removing the just-vacated square must be accepted.
	if !s.PlayRemoval(from) {
		t.Fatal("removing the just-vacated square should be legal")
	}
	if s.Step != StepMove {
		t.Fatal("after a legal removal, Step should reset to StepMove for the next side")
	}
	if s.Turn != White {
		t.Fatal("turn should now belong to White")
	}
	if s.Board.IsPresent(from.X, from.Y) {
		t.Fatal("the vacated square should now be missing")
	}
	if !s.HasLast || s.LastFrom != from || s.LastTo != to || s.LastRemoved != from {
		t.Fatalf("Last* fields should record the completed turn: got From=%v To=%v Removed=%v HasLast=%v",
			s.LastFrom, s.LastTo, s.LastRemoved, s.HasLast)
	}
}

func TestPlayMoveRejectsIllegalDestination(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	from := s.Board.PawnPos(Black)
	bogus := image.Pt((from.X+3)%Size, (from.Y+2)%Size) // knight-ish offset, not a queen line in general
	// Guard: ensure the chosen offset is not coincidentally a legal queen move.
	for _, d := range s.Board.LegalMoves(Black) {
		if d == bogus {
			t.Skip("bogus destination happened to be legal; adjust test fixture")
		}
	}
	if s.PlayMove(bogus) {
		t.Fatal("an illegal destination must be rejected")
	}
	if s.Step != StepMove {
		t.Fatal("a rejected move must not advance Step")
	}
}

func TestPlayRemovalBeforeMoveIsRejected(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.PlayRemoval(image.Pt(0, 0)) {
		t.Fatal("PlayRemoval must be rejected while Step is StepMove")
	}
}

func TestNoInputAfterGameEnds(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	trapped, at := winnerTestBoard()
	s.Board = trapped
	s.Turn = Black // Black is stuck: GameOver(Black) is already true
	s.Phase = PhaseDone
	_ = at

	if s.PlayMove(image.Pt(1, 1)) {
		t.Fatal("no move should be accepted once Phase is PhaseDone")
	}
	if s.Winner() != White {
		t.Fatalf("Winner() = %v, want White (Black to move, Black stuck)", s.Winner())
	}
}

// --- WIN: a full sequence of turns ending when a side is trapped -----------

func TestGameEndsWhenSideToMoveIsTrapped(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	// Force a near-trapped position directly, then play the final
	// move+removal through the real state machine and confirm the game
	// ends and reports the correct winner.
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.Present[y][x] = true
		}
	}
	b.WhitePawn = image.Pt(0, 0)
	b.BlackPawn = image.Pt(7, 7) // Black about to move and trap White
	// White's only destinations from (0,0) are along (1,0), (0,1), and the
	// (1,1) diagonal. Block the first two entirely, and truncate the
	// diagonal to a single step, leaving exactly (1,1) — Black's
	// move+removal will move somewhere irrelevant and then remove (1,1),
	// trapping White.
	b.Present[0][1] = false // (1,0) missing
	b.Present[1][0] = false // (0,1) missing
	b.Present[2][2] = false // (2,2) missing: truncates the diagonal to just (1,1)
	s.Board = b
	s.Turn = Black
	s.Step = StepMove

	if got := s.Board.LegalMoves(White); len(got) != 1 || got[0] != image.Pt(1, 1) {
		t.Fatalf("setup: White should have exactly one destination (1,1), got %v", got)
	}

	dest := s.Board.LegalMoves(Black)[0] // any legal Black move
	if !s.PlayMove(dest) {
		t.Fatal("Black's setup move should be legal")
	}
	if !s.PlayRemoval(image.Pt(1, 1)) {
		t.Fatal("removing White's last destination should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("the game should be over: White (to move) has zero legal moves")
	}
	if s.Turn != White {
		t.Fatal("Turn should have advanced to White (the trapped side) before Phase flipped to Done")
	}
	if w := s.Winner(); w != Black {
		t.Fatalf("Winner() = %v, want Black", w)
	}
}

func TestStepAIPlaysAFullTurn(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	// Black (human) moves first; AI is White.
	from := s.Board.PawnPos(Black) // capture BEFORE moving
	dest := s.Board.LegalMoves(Black)[0]
	if !s.PlayMove(dest) {
		t.Fatal("Black's move should be legal")
	}
	if !s.PlayRemoval(from) {
		t.Fatal("removing Black's own vacated square should be legal")
	}
	if !s.AITurn() {
		t.Fatal("it should now be White's (the AI's) turn")
	}
	beforeWhite := s.Board.PawnPos(White)
	beforeTotal := s.Board.TotalPresent()
	if !s.StepAI() {
		t.Fatal("StepAI should play a move")
	}
	if s.AITurn() {
		t.Fatal("after StepAI, it should no longer be the AI's turn")
	}
	if s.Board.PawnPos(White) == beforeWhite {
		t.Fatal("the AI should have moved its pawn")
	}
	if s.Board.TotalPresent() != beforeTotal-1 {
		t.Fatal("the AI's turn should have removed exactly one tile")
	}
	if s.Turn != Black {
		t.Fatal("turn should pass back to Black after the AI's turn")
	}
}
