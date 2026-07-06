package game

import (
	"image"
	"testing"
)

func TestNewGameStartingState(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Turn != Black {
		t.Fatalf("Black should move first, got %v", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatal("fresh game should be PhasePlaying")
	}
	if s.Board.Count(Black) != 3 || s.Board.Count(White) != 3 {
		t.Fatalf("expected 3 tiles per side, got B=%d W=%d", s.Board.Count(Black), s.Board.Count(White))
	}
	if len(s.Reserve[Black].Types()) != 4 || len(s.Reserve[White].Types()) != 4 {
		t.Fatalf("expected 4 reserve types per side, got B=%d W=%d",
			len(s.Reserve[Black].Types()), len(s.Reserve[White].Types()))
	}
}

func TestPlayRejectsIllegalAction(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	before := s.Board
	beforeTurn := s.Turn
	// Nothing at (0,0) at the start; From there is empty, this must be
	// rejected.
	bogus := Action{Kind: ActRelocate, From: image.Pt(0, 0), To: image.Pt(0, 1)}
	if s.Play(bogus) {
		t.Fatal("an action from an empty square must be rejected")
	}
	if s.Board != before || s.Turn != beforeTurn {
		t.Fatal("a rejected action must not mutate board or turn")
	}
}

func TestPlayAdvancesTurnAndReturnsTrueOnLegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	legal := s.LegalActions()
	if len(legal) == 0 {
		t.Fatal("Black should have legal actions at the start")
	}
	// Pick a plain relocate (not a recruit) for a clean before/after check.
	var mv Action
	found := false
	for _, a := range legal {
		if a.Kind == ActRelocate {
			mv = a
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected at least one ActRelocate among Black's opening actions")
	}
	if !s.Play(mv) {
		t.Fatalf("legal action %+v was rejected", mv)
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White after Black's move")
	}
}

// --- recruit: newly placed tile can act only from ITS NEXT turn ------------
//
// Design choice (documented per the spec's explicit "unverified, decide and
// document" instruction): Recruit consumes the whole turn — it is an
// alternative to moving, not a placement bundled with a bonus action — so a
// freshly recruited tile sits out the turn it arrives on and is available
// like any other own tile starting on the recruiting side's NEXT turn.

func TestRecruitedTileCannotActTheSameTurnItArrives(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Duke, Black, FaceA)
	put(&b, 0, 5, Duke, White, FaceA)
	s := &GameState{
		Board:    b,
		Reserve:  [2]ReserveMask{Black: NewReserve(), White: NewReserve()},
		Turn:     Black,
		Opponent: OpponentHotseat,
		Phase:    PhasePlaying,
	}
	recruit := Action{Kind: ActRecruit, To: image.Pt(3, 2), Recruit: Knight}
	if !s.Play(recruit) {
		t.Fatal("recruiting should be a legal action")
	}
	// The turn already passed to White: the recruit action itself was
	// Black's whole turn, so the new Knight never got a chance to act this
	// turn regardless of what it could do.
	if s.Turn != White {
		t.Fatal("Recruit should consume the whole turn and pass to the opponent")
	}
	// Play White's own turn, then confirm it's Black's turn again and the
	// recruited Knight now has ordinary legal actions like any other tile.
	whiteLegal := s.LegalActions()
	if len(whiteLegal) == 0 {
		t.Fatal("White should have a legal action")
	}
	s.Play(whiteLegal[0])
	if s.Turn != Black {
		t.Fatal("expected it to be Black's turn again")
	}
	knightActs := s.Board.ActionsFrom(image.Pt(3, 2))
	if len(knightActs) == 0 {
		t.Fatal("the recruited Knight should have ordinary legal actions on Black's next turn")
	}
}

// --- winner reporting --------------------------------------------------------

func TestWinnerFalseWhilePlaying(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if _, ok := s.Winner(); ok {
		t.Fatal("Winner should report ok=false while the game is still playing")
	}
}

func TestStepAIPlaysAMove(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	legal := s.LegalActions()
	// Advance Black once so it becomes White's (the AI's) turn.
	var mv Action
	for _, a := range legal {
		if a.Kind == ActRelocate {
			mv = a
			break
		}
	}
	if !s.Play(mv) {
		t.Fatal("Black's opening move should be legal")
	}
	if !s.AITurn() {
		t.Fatal("it should now be the AI's (White's) turn")
	}
	before := s.Board
	if !s.StepAI() {
		t.Fatal("StepAI should report it made a move")
	}
	if s.Board == before {
		t.Fatal("StepAI should have changed the board")
	}
	if s.Turn != Black {
		t.Fatal("turn should be back to Black after the AI moves")
	}
}
