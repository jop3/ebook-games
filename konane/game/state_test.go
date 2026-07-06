package game

import (
	"image"
	"testing"
)

// clearBoard empties the game's board (bypassing the opening phase) so tests
// can construct exact positions, then puts the game directly into
// PhasePlaying with the given side to move.
func clearBoard(s *GameState, toMove Cell) {
	s.Board = emptyBoard()
	s.Phase = PhasePlaying
	s.Turn = toMove
	s.ChainActive = false
	s.LastCaptured = nil
}

// --- GOTCHA: no legal jump anywhere -> immediate loss -----------------------

func TestNoLegalJumpAnywhereIsImmediateLoss(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	clearBoard(s, Black)
	s.Board.set(0, 0, Black) // a single, isolated Black stone: no enemy to jump

	// beginTurn is exercised via finishTurn/RemoveOpeningWhite in normal play;
	// call it directly here (same package) to check the "no jump -> loss"
	// rule in isolation, independent of how a turn is reached.
	s.beginTurn(Black)

	if s.Phase != PhaseDone {
		t.Fatalf("a side with zero legal jumps anywhere must lose immediately, Phase=%v", s.Phase)
	}
	if s.Winner() != White {
		t.Fatalf("Winner() = %v, want White (Black had no legal jump)", s.Winner())
	}
}

func TestNoLegalJumpAnywhereViaFinishTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	clearBoard(s, Black)
	// Black can make exactly one jump, capturing White's only stone, which
	// then leaves White with nothing at all: White's turn begins with zero
	// stones and hence zero legal jumps -> White loses immediately.
	s.Board.set(3, 3, Black)
	s.Board.set(3, 4, White)

	if !s.StartJump(image.Pt(3, 3), image.Pt(3, 5)) {
		t.Fatal("the only legal jump should be accepted")
	}
	if s.Phase != PhaseDone {
		t.Fatalf("White should have zero stones (and thus zero jumps) after this capture, Phase=%v", s.Phase)
	}
	if s.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black", s.Winner())
	}
}

// --- Chain jumps: both continuing and stopping early ------------------------

func TestChainContinuePastFirstJump(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	clearBoard(s, Black)
	// Black at (1,1) can jump White at (2,1) -> land (3,1); from (3,1) it can
	// jump White at (4,1) -> land (5,1).
	s.Board.set(1, 1, Black)
	s.Board.set(2, 1, White)
	s.Board.set(4, 1, White)

	if !s.StartJump(image.Pt(1, 1), image.Pt(3, 1)) {
		t.Fatal("the first jump should be legal")
	}
	if !s.ChainActive {
		t.Fatal("a further jump is available from (3,1); ChainActive should be true")
	}
	if s.ChainFrom != (image.Point{X: 3, Y: 1}) {
		t.Fatalf("ChainFrom = %v, want (3,1)", s.ChainFrom)
	}
	if s.Turn != Black {
		t.Fatal("turn must not pass to White while a chain is still active")
	}
	if s.Board.At(2, 1) != Empty {
		t.Fatal("the first jumped stone should already be captured")
	}

	// A tap on a non-adjacent, non-chain cell must be rejected without
	// disturbing the chain.
	if s.ContinueJump(image.Pt(0, 0)) {
		t.Fatal("an illegal continuation must be rejected")
	}
	if !s.ChainActive {
		t.Fatal("a rejected continuation must not cancel the chain")
	}

	if !s.ContinueJump(image.Pt(5, 1)) {
		t.Fatal("the second jump should be legal")
	}
	if s.ChainActive {
		t.Fatal("no further jump is available from (5,1); the chain should auto-end")
	}
	if s.Turn != White {
		t.Fatal("turn should now belong to White")
	}
	if len(s.LastCaptured) != 2 {
		t.Fatalf("LastCaptured should list both stones captured this turn, got %v", s.LastCaptured)
	}
}

func TestChainStopEarlyViaKlart(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	clearBoard(s, Black)
	s.Board.set(1, 1, Black)
	s.Board.set(2, 1, White)
	s.Board.set(4, 1, White) // a second jump WOULD be available, but is optional

	if !s.StartJump(image.Pt(1, 1), image.Pt(3, 1)) {
		t.Fatal("the first jump should be legal")
	}
	if !s.ChainActive {
		t.Fatal("expected a chain to be active (a further jump exists)")
	}

	if !s.EndChain() {
		t.Fatal("EndChain (Klart) should succeed while a chain is active")
	}
	if s.ChainActive {
		t.Fatal("ChainActive should be false after EndChain")
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White once the player stops the chain early")
	}
	// The second White stone (at (4,1)) must still be on the board — the
	// chain was stopped before that jump was taken.
	if s.Board.At(4, 1) != White {
		t.Fatal("stopping the chain early must leave the not-yet-jumped stone in place")
	}
}

func TestEndChainWithoutActiveChainIsRejected(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	clearBoard(s, Black)
	s.Board.set(3, 3, Black)
	if s.EndChain() {
		t.Fatal("EndChain must fail when no chain is in progress")
	}
}

// --- StartJump / ContinueJump legality guards -------------------------------

func TestStartJumpRejectsWrongTurnAndIllegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	clearBoard(s, Black)
	s.Board.set(3, 3, Black)
	s.Board.set(4, 4, White) // not adjacent to (3,3): no legal jump between them

	if s.StartJump(image.Pt(4, 4), image.Pt(6, 6)) {
		t.Fatal("StartJump with a White origin while Black is to move must be rejected")
	}
	if s.StartJump(image.Pt(3, 3), image.Pt(5, 5)) {
		t.Fatal("a non-adjacent/non-jump destination must be rejected")
	}
	if s.Turn != Black || s.ChainActive {
		t.Fatal("rejected attempts must not change turn or chain state")
	}
}

func TestStartJumpRejectsWhileChainActive(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	clearBoard(s, Black)
	s.Board.set(1, 1, Black)
	s.Board.set(2, 1, White)
	s.Board.set(4, 1, White)
	s.Board.set(6, 1, Black) // a second Black stone with its own jump available
	s.Board.set(6, 2, White)

	if !s.StartJump(image.Pt(1, 1), image.Pt(3, 1)) {
		t.Fatal("setup: first jump should succeed")
	}
	if !s.ChainActive {
		t.Fatal("setup: expected an active chain")
	}
	// Must not be able to start a fresh jump with a different stone while a
	// chain from another stone is still active.
	if s.StartJump(image.Pt(6, 1), image.Pt(6, 3)) {
		t.Fatal("StartJump must be rejected while a chain is already active")
	}
}

// --- AITurn / StepAI ---------------------------------------------------------

func TestAITurnDuringOpeningWhiteRemove(t *testing.T) {
	s := NewGame(OpponentAI, 0)
	if s.AITurn() {
		t.Fatal("AITurn should be false during Black's own opening removal")
	}
	opts := CenterRemovalOptions()
	if !s.RemoveOpeningBlack(opts[0]) {
		t.Fatal("setup: Black's opening removal should succeed")
	}
	if !s.AITurn() {
		t.Fatal("AITurn should be true for White's opening removal in AI mode")
	}
	if !s.StepAI() {
		t.Fatal("StepAI should perform White's opening removal")
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("phase should advance to PhasePlaying, got %v", s.Phase)
	}
}

func TestAITurnHotseatNeverTrue(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	opts := CenterRemovalOptions()
	s.RemoveOpeningBlack(opts[0])
	if s.AITurn() {
		t.Fatal("AITurn must always be false in hotseat mode")
	}
	s.RemoveOpeningWhite(s.OpeningWhiteOptions()[0])
	if s.AITurn() {
		t.Fatal("AITurn must always be false in hotseat mode, even on White's turn")
	}
}

func TestStepAIPlaysAFullChainMove(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	clearBoard(s, White)
	s.Board.set(3, 3, White)
	s.Board.set(3, 4, Black)
	s.Board.set(6, 6, Black) // keeps Black from being reduced to 0 (irrelevant to the win rule, just realism)

	if !s.AITurn() {
		t.Fatal("it should be the AI's (White's) turn")
	}
	before := s.Board
	if !s.StepAI() {
		t.Fatal("StepAI should play White's only available move")
	}
	if s.Board == before {
		t.Fatal("StepAI should have changed the board")
	}
	if len(s.LastCaptured) == 0 {
		t.Fatal("StepAI's jump should have captured at least one stone")
	}
}
