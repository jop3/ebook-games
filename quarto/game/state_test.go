package game

import "testing"

func TestNewGameInitialState(t *testing.T) {
	s := NewGame(ModeHotseat, 0)
	if len(s.Pool) != NumPieces {
		t.Fatalf("expected %d pieces in pool, got %d", NumPieces, len(s.Pool))
	}
	if s.Step != StepGive {
		t.Fatalf("expected the very first action to be StepGive (nothing to place yet)")
	}
	if s.Turn != 0 {
		t.Fatalf("expected player 0 to start")
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("expected PhasePlaying at start")
	}
}

func TestGiveThenPlaceCycle(t *testing.T) {
	s := NewGame(ModeHotseat, 0)
	if !s.GivePiece(5) {
		t.Fatalf("expected giving piece 5 to succeed")
	}
	if s.Turn != 1 {
		t.Fatalf("expected turn to pass to player 1 after a give")
	}
	if s.Step != StepPlace {
		t.Fatalf("expected StepPlace after a give")
	}
	if s.ActivePiece != 5 {
		t.Fatalf("expected ActivePiece to be the given piece")
	}
	if len(s.Pool) != NumPieces-1 {
		t.Fatalf("expected pool to shrink by one")
	}
	if !s.PlacePiece(0, 0) {
		t.Fatalf("expected placement to succeed")
	}
	if s.Step != StepGive {
		t.Fatalf("expected StepGive to follow a non-winning placement")
	}
	if s.Turn != 1 {
		t.Fatalf("turn should not change on a place (only on a give)")
	}
}

func TestGiveRejectsUnavailablePiece(t *testing.T) {
	s := NewGame(ModeHotseat, 0)
	s.GivePiece(0)
	// piece 0 is no longer in the pool
	if s.GivePiece(0) {
		t.Fatalf("expected giving an already-used piece to fail")
	}
}

func TestPlaceRejectsWrongStep(t *testing.T) {
	s := NewGame(ModeHotseat, 0)
	// Step is StepGive; placing should fail.
	if s.PlacePiece(0, 0) {
		t.Fatalf("expected placement to fail before any piece has been given")
	}
}

func TestWinDetectedOnPlace(t *testing.T) {
	s := NewGame(ModeHotseat, 0)
	// Manually set up a near-win: 3 tall pieces on row 0, hand player a 4th tall piece.
	s.Board.Place(0, 0, AttrTall)
	s.Board.Place(1, 0, AttrTall|AttrDark)
	s.Board.Place(2, 0, AttrTall|AttrSquare)
	// Remove the pieces we used from the pool to keep state consistent.
	for _, p := range []Piece{AttrTall, AttrTall | AttrDark, AttrTall | AttrSquare} {
		s.removeFromPool(p)
	}
	winPiece := Piece(AttrTall | AttrSolid)
	s.removeFromPool(winPiece)
	s.ActivePiece = winPiece
	s.Step = StepPlace
	if !s.PlacePiece(3, 0) {
		t.Fatalf("expected the winning placement to be accepted")
	}
	if s.Phase != PhaseWon {
		t.Fatalf("expected PhaseWon after completing a shared line")
	}
}

func TestDrawWhenBoardFullNoWin(t *testing.T) {
	s := NewGame(ModeHotseat, 0)
	// A known Quarto no-win-anywhere full board (each row/col/diagonal has at
	// least one attribute split 2-2), values chosen and verified below by the
	// HasWin check before AND after the final placement.
	pattern := [Size][Size]Piece{
		{0b0010, 0b1100, 0b1101, 0b0100},
		{0b1110, 0b0101, 0b0000, 0b1011},
		{0b1000, 0b1111, 0b0001, 0b0110},
		{0b0111, 0b0011, 0b1010, -1}, // last cell filled below via the state machine
	}
	last := Piece(0b1001)
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if pattern[y][x] == -1 {
				continue
			}
			s.Board.Place(x, y, pattern[y][x])
			s.removeFromPool(pattern[y][x])
		}
	}
	if s.Board.HasWin() {
		t.Fatalf("test setup should not already have a win before the final placement")
	}
	s.removeFromPool(last)
	s.ActivePiece = last
	s.Step = StepPlace
	s.PlacePiece(3, 3)
	if s.Phase == PhaseWon {
		t.Fatalf("pattern produced an unexpected win on the final placement; fixture needs different values")
	}
	if s.Phase != PhaseDraw {
		t.Fatalf("expected PhaseDraw once the board is full without a win, got %v", s.Phase)
	}
}

func TestAITurnDetection(t *testing.T) {
	s := NewGame(ModeAI, 2)
	if s.AITurn() {
		t.Fatalf("player 0 (human) starts; AITurn should be false")
	}
	s.GivePiece(0)
	if !s.AITurn() {
		t.Fatalf("after player 0 gives, it's player 1 (AI)'s turn to place")
	}
}

func TestStepAIPlaysAndGives(t *testing.T) {
	s := NewGame(ModeAI, 1)
	s.GivePiece(0) // human gives piece 0 to the AI
	if !s.StepAI() {
		t.Fatalf("expected AI to place")
	}
	if s.Step != StepGive {
		t.Fatalf("expected AI's placement to advance to StepGive")
	}
	if !s.StepAI() {
		t.Fatalf("expected AI to give a piece")
	}
	if s.Step != StepPlace || s.Turn != 0 {
		t.Fatalf("expected the give to hand the turn back to the human")
	}
}
