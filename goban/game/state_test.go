package game

import (
	"image"
	"testing"
)

func TestNewGameDefaults(t *testing.T) {
	s := NewGame(9, OpponentHotseat, DefaultKomi)
	if s.Turn != Black {
		t.Fatal("Black moves first")
	}
	if s.Phase != PhasePlaying {
		t.Fatal("a new game should be in PhasePlaying")
	}
	if s.Board.Size() != 9 {
		t.Fatalf("board size = %d, want 9", s.Board.Size())
	}
}

func TestPlayAlternatesTurn(t *testing.T) {
	s := NewGame(9, OpponentHotseat, DefaultKomi)
	if !s.Play(4, 4) {
		t.Fatal("opening move should be legal")
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White after Black's move")
	}
	if s.Board.At(image.Pt(4, 4)) != Black {
		t.Fatal("the stone should be on the board")
	}
}

func TestPlayRejectsOccupiedAndSuicide(t *testing.T) {
	s := NewGame(9, OpponentHotseat, DefaultKomi)
	s.Play(4, 4)
	if s.Play(4, 4) {
		t.Fatal("playing on an occupied point must be rejected")
	}
	if s.Turn != White {
		t.Fatal("a rejected move must not change the turn")
	}
}

func TestKoViaGameState(t *testing.T) {
	// Reuse the classic ko shape, this time driving it through GameState.Play
	// so priorBoard bookkeeping is exercised end to end.
	s := NewGame(9, OpponentHotseat, 0)
	s.Board.Set(image.Pt(2, 0), Black)
	s.Board.Set(image.Pt(3, 1), Black)
	s.Board.Set(image.Pt(2, 2), Black)
	s.Board.Set(image.Pt(1, 1), Black)
	s.Board.Set(image.Pt(1, 0), White)
	s.Board.Set(image.Pt(0, 1), White)
	s.Board.Set(image.Pt(1, 2), White)
	s.Turn = White

	if !s.Play(2, 1) {
		t.Fatal("White's capturing move should be legal")
	}
	if s.Board.At(image.Pt(1, 1)) != Empty {
		t.Fatal("the captured Black stone should be removed")
	}
	if s.Turn != Black {
		t.Fatal("turn should now be Black's")
	}

	// Black's immediate recapture must be forbidden by ko.
	if s.Play(1, 1) {
		t.Fatal("immediate ko recapture must be rejected by GameState.Play")
	}
	if s.Turn != Black {
		t.Fatal("a rejected move must not advance the turn")
	}

	// Black plays elsewhere, White plays elsewhere, and NOW the recapture at
	// (1,1) is legal again.
	if !s.Play(7, 7) {
		t.Fatal("Black's elsewhere move should be legal")
	}
	if !s.Play(7, 8) {
		t.Fatal("White's elsewhere move should be legal")
	}
	if !s.Play(1, 1) {
		t.Fatal("the ko should have lifted after an intervening move elsewhere")
	}
	if s.Board.At(image.Pt(2, 1)) != Empty {
		t.Fatal("the recapture should have removed White's stone at (2,1)")
	}
}

func TestPassTwiceEntersMarking(t *testing.T) {
	s := NewGame(9, OpponentHotseat, DefaultKomi)
	if !s.Pass() {
		t.Fatal("a pass should be accepted")
	}
	if s.Phase != PhasePlaying {
		t.Fatal("a single pass must not end the game")
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White after Black passes")
	}
	if !s.Pass() {
		t.Fatal("the second pass should be accepted")
	}
	if s.Phase != PhaseMarking {
		t.Fatal("two consecutive passes should enter the mark-dead phase")
	}
	if s.Dead == nil {
		t.Fatal("Dead set should be initialized entering PhaseMarking")
	}
}

func TestPlayResetsConsecutivePasses(t *testing.T) {
	s := NewGame(9, OpponentHotseat, DefaultKomi)
	s.Pass()
	if !s.Play(4, 4) {
		t.Fatal("a move should be legal after a single pass")
	}
	if s.ConsecutivePasses != 0 {
		t.Fatal("a real move should reset the pass counter")
	}
	s.Pass()
	if s.Phase != PhasePlaying {
		t.Fatal("a single pass after a real move must not end the game")
	}
}

func TestToggleDeadAndFinishMarking(t *testing.T) {
	s := NewGame(9, OpponentHotseat, 0)
	for y := 0; y < 9; y++ {
		s.Board.Set(image.Pt(3, y), Black)
		s.Board.Set(image.Pt(5, y), White) // bounds the board on the right
	}
	s.Board.Set(image.Pt(1, 4), White) // a doomed lone stone in Black's area
	s.Phase = PhaseMarking
	s.Dead = map[image.Point]bool{}

	if !s.ToggleDead(1, 4) {
		t.Fatal("tapping a stone during marking should toggle it")
	}
	if !s.Dead[image.Pt(1, 4)] {
		t.Fatal("the tapped stone should now be marked dead")
	}
	// Tap again: toggles back to alive.
	s.ToggleDead(1, 4)
	if s.Dead[image.Pt(1, 4)] {
		t.Fatal("tapping a dead group again should mark it alive")
	}
	s.ToggleDead(1, 4) // mark dead again for the final score below

	s.FinishMarking()
	if s.Phase != PhaseDone {
		t.Fatal("FinishMarking should move to PhaseDone")
	}
	if s.BlackScore != 9+27 {
		t.Fatalf("Black score with the dead stone removed = %v, want %v", s.BlackScore, 9+27)
	}
	// The two walls are otherwise a mirror image with komi 0, so once the
	// dead stone is correctly resolved to Black's territory this is an exact
	// tie — which itself confirms the dead-marking pipeline worked, since
	// leaving it un-marked would have made the region dame and cost Black
	// the whole left territory (see TestAreaScoreDeadStonesCountForCapturer).
	if s.WhiteScore != 9+27 {
		t.Fatalf("White score = %v, want %v", s.WhiteScore, 9+27)
	}
	if s.Winner() != Empty {
		t.Fatalf("expected an exact tie, got winner %v (black=%v white=%v)", s.Winner(), s.BlackScore, s.WhiteScore)
	}
}

func TestAITurnOnlyForOpponentAI(t *testing.T) {
	hot := NewGame(9, OpponentHotseat, DefaultKomi)
	if hot.AITurn() {
		t.Fatal("hotseat games never have an AI turn")
	}
	ai := NewGame(9, OpponentAI, DefaultKomi)
	if ai.AITurn() {
		t.Fatal("Black (human) moves first even in AI mode")
	}
	ai.Play(4, 4)
	if !ai.AITurn() {
		t.Fatal("after Black's move in AI mode, it should be White (the AI)'s turn")
	}
}

func TestStepAIMirrorsHumanPass(t *testing.T) {
	s := NewGame(9, OpponentAI, DefaultKomi)
	s.Pass() // Black (human) passes; turn -> White (AI)
	if !s.AITurn() {
		t.Fatal("setup: should be the AI's turn")
	}
	if !s.StepAI() {
		t.Fatal("StepAI should act (by passing) when the human just passed")
	}
	if s.Phase != PhaseMarking {
		t.Fatal("the AI mirroring the human's pass should end play via double pass")
	}
}

func TestStepAIPlaysWhenNotMirroring(t *testing.T) {
	s := NewGame(9, OpponentAI, DefaultKomi)
	s.Play(4, 4) // Black's real move; turn -> White (AI), LastPass is false
	if !s.StepAI() {
		t.Fatal("StepAI should play a real move")
	}
	if s.Turn != Black {
		t.Fatal("after the AI's move, turn should return to Black")
	}
	if s.Board.Count(White) != 1 {
		t.Fatal("the AI should have placed exactly one White stone")
	}
}

// --- Full scripted 9x9 game reaching a plausible final score ----------------
//
// Black and White each build a solid wall (column 3 for Black, column 5 for
// White), leaving column 4 as a neutral corridor that borders both colors.
// This is a real, rule-legal sequence of alternating Play() calls (no direct
// board surgery) ending in two passes and a mark-dead phase with nothing
// marked dead, so it exercises the entire pipeline: alternating placement,
// turn bookkeeping, double-pass end, and final area scoring.
func TestFullGameScriptedFinalScore(t *testing.T) {
	s := NewGame(9, OpponentHotseat, DefaultKomi)
	for y := 0; y < 9; y++ {
		if !s.Play(3, y) { // Black
			t.Fatalf("Black's move at (3,%d) should be legal", y)
		}
		if !s.Play(5, y) { // White
			t.Fatalf("White's move at (5,%d) should be legal", y)
		}
	}
	if s.Board.Count(Black) != 9 || s.Board.Count(White) != 9 {
		t.Fatalf("expected 9 stones each, got black=%d white=%d", s.Board.Count(Black), s.Board.Count(White))
	}
	if s.Turn != Black {
		t.Fatal("after 18 alternating moves, it should be Black's turn again")
	}

	if !s.Pass() {
		t.Fatal("Black's pass should be accepted")
	}
	if !s.Pass() {
		t.Fatal("White's pass should be accepted")
	}
	if s.Phase != PhaseMarking {
		t.Fatal("two passes should enter the mark-dead phase")
	}
	s.FinishMarking() // nothing marked dead

	if s.Phase != PhaseDone {
		t.Fatal("FinishMarking should conclude the game")
	}
	// Black: 9 stones + columns 0-2 (27 points), all bordering only Black.
	wantBlack := 9.0 + 27.0
	// White: 9 stones + columns 6-8 (27 points) + komi.
	wantWhite := 9.0 + 27.0 + DefaultKomi
	if s.BlackScore != wantBlack {
		t.Fatalf("BlackScore = %v, want %v", s.BlackScore, wantBlack)
	}
	if s.WhiteScore != wantWhite {
		t.Fatalf("WhiteScore = %v, want %v", s.WhiteScore, wantWhite)
	}
	if s.Winner() != White {
		t.Fatalf("White should win by komi (%v vs %v), got winner %v", s.WhiteScore, s.BlackScore, s.Winner())
	}
}
