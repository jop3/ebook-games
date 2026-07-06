package game

import (
	"image"
	"testing"
)

func TestNewGameSetup(t *testing.T) {
	s := NewGame(OpponentHotseat, SizeStandard, 0)
	if s.Turn != V {
		t.Fatalf("Turn = %v, want V to start", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", s.Phase)
	}
	if s.Board.EmptyCount() != SizeStandard*SizeStandard {
		t.Fatalf("fresh board should be entirely empty, got %d empty", s.Board.EmptyCount())
	}
	if s.AITurn() {
		t.Fatal("hotseat is never the AI's turn")
	}
}

func TestPlayLegalMoveAdvancesTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, SizeStandard, 0)
	legal := s.Board.LegalMoves(V)
	if len(legal) == 0 {
		t.Fatal("V should have legal moves at the start")
	}
	m := legal[0]
	if !s.Play(m) {
		t.Fatalf("Play(%v) should have succeeded", m)
	}
	if s.Turn != H {
		t.Fatal("turn should pass to H after V moves")
	}
	if s.Board.Empty(m.A.X, m.A.Y) || s.Board.Empty(m.B.X, m.B.Y) {
		t.Fatal("both of the move's cells should now be occupied")
	}
	if !s.HasLastMove || s.LastMove != [2]image.Point{m.A, m.B} {
		t.Fatalf("LastMove should record the placed domino, got %v", s.LastMove)
	}
}

func TestPlayRejectsIllegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, SizeStandard, 0)
	// V may never place a horizontal domino, even on an empty board.
	horiz := Move{A: image.Pt(2, 2), B: image.Pt(3, 2)}
	if s.Play(horiz) {
		t.Fatal("V must never be allowed a horizontal placement")
	}
	if s.Turn != V {
		t.Fatal("an illegal move must not change the turn")
	}
	// Occupied cell.
	s.Play(Move{A: image.Pt(0, 0), B: image.Pt(0, 1)}) // legal, V's move
	if s.Play(Move{A: image.Pt(0, 1), B: image.Pt(0, 2)}) {
		// (0,1) is now occupied by the previous move.
		t.Fatal("moving onto an occupied cell must be rejected")
	}
}

func TestPlayRejectedAfterGameOver(t *testing.T) {
	s := NewGame(OpponentHotseat, 1, 0) // 1x1: nobody can ever move
	if s.Phase != PhaseDone {
		t.Fatal("a 1x1 board should already be game-over at start (V has no legal move)")
	}
	if w := s.Winner(); w != H {
		t.Fatalf("Winner() = %v, want H (V, to move, could not move)", w)
	}
	if s.Play(Move{A: image.Pt(0, 0), B: image.Pt(0, 0)}) {
		t.Fatal("Play must reject any move once the game is over")
	}
}

// --- GOTCHA: explicit "cannot move loses" check, not an assumption ----------

func TestLossConditionIsCannotMoveNotLastMoved(t *testing.T) {
	// Construct a position where it is H's turn but H (horizontal-only) has
	// no legal placement, even though empty cells remain — every remaining
	// empty cell is vertically isolated (single-column strips). V should be
	// declared the winner, not because V "moved last" in this constructed
	// snapshot (no move was played to reach it at all), but strictly because
	// H, to move, cannot place — this test exercises Winner()/checkTerminal
	// directly against a hand-built board, independent of move history.
	s := NewGame(OpponentHotseat, SizeStandard, 0)
	rows := make([][]bool, SizeStandard)
	for y := range rows {
		row := make([]bool, SizeStandard)
		// Occupy every column except column 0 (fully empty) — only V can use
		// that lone open column; H needs two open columns side by side.
		for x := 1; x < SizeStandard; x++ {
			row[x] = true
		}
		rows[y] = row
	}
	b := BoardFromRows(rows)
	s.Board = b
	s.Turn = H
	s.checkTerminal()
	if s.Phase != PhaseDone {
		t.Fatal("H has zero legal moves here (only a lone open column remains); game must be over")
	}
	if w := s.Winner(); w != V {
		t.Fatalf("Winner() = %v, want V (H could not move, not because V 'moved last')", w)
	}
	// Confirm independently: H truly has no legal move on this board, while V
	// does (still has vertical room in the open column).
	if b.HasMove(H) {
		t.Fatal("test setup invalid: H should have no legal move here")
	}
	if !b.HasMove(V) {
		t.Fatal("test setup invalid: V should still have a legal move here")
	}
}

func TestAITurnAndStepAI(t *testing.T) {
	s := NewGame(OpponentAI, SizeStandard, DepthEasy)
	if s.AITurn() {
		t.Fatal("should not be the AI's turn before V (human) has moved")
	}
	legal := s.Board.LegalMoves(V)
	s.Play(legal[0])
	if !s.AITurn() {
		t.Fatal("should be the AI's (H's) turn after V moves")
	}
	beforeEmpty := s.Board.EmptyCount()
	if !s.StepAI() {
		t.Fatal("StepAI should have played a move")
	}
	if s.AITurn() {
		t.Fatal("turn should have passed back to V after StepAI")
	}
	if s.Turn != V {
		t.Fatalf("Turn = %v, want V after H's StepAI move", s.Turn)
	}
	if got := s.Board.EmptyCount(); got != beforeEmpty-2 {
		t.Fatalf("EmptyCount should drop by exactly 2 after StepAI, got %d -> %d", beforeEmpty, got)
	}
}

func TestFullHotseatGameTerminates(t *testing.T) {
	s := NewGame(OpponentHotseat, SizeSmall, 0)
	for ply := 0; s.Phase == PhasePlaying; ply++ {
		if ply > SizeSmall*SizeSmall {
			t.Fatal("game did not terminate in a sane number of plies")
		}
		moves := s.Board.LegalMoves(s.Turn)
		if len(moves) == 0 {
			t.Fatal("Phase should already be Done if the side to move has no legal move")
		}
		if !s.Play(moves[0]) {
			t.Fatalf("a move drawn from LegalMoves must always be playable: %v", moves[0])
		}
	}
	// Whichever side ended up unable to move lost; sanity-check both are
	// consistent with the final board.
	if s.Board.HasMove(s.Turn) {
		t.Fatal("game ended but the side to move still has a legal move")
	}
}
