package game

import (
	"image"
	"testing"
)

func TestNewGameInitialState(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Turn != Black {
		t.Errorf("Turn = %v, want Black (Black starts)", s.Turn)
	}
	if s.Step != StepPlaceL {
		t.Errorf("Step = %v, want StepPlaceL", s.Step)
	}
	if s.Phase != PhasePlaying {
		t.Errorf("Phase = %v, want PhasePlaying", s.Phase)
	}
}

func TestPlaceLThenStepAdvancesToNeutralOptional(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	moves := LegalLPlacements(s.Board, Black)
	pl := moves[0]
	if !s.PlaceL(pl) {
		t.Fatal("a legal placement should be accepted")
	}
	if s.Step != StepNeutralOptional {
		t.Fatalf("Step = %v, want StepNeutralOptional after placing L", s.Step)
	}
	if s.Turn != Black {
		t.Fatal("turn must not pass until the neutral step is resolved (skip or move)")
	}
	for _, c := range pl.Cells {
		if s.Board.At(c.X, c.Y) != Black {
			t.Errorf("cell %v should now hold Black", c)
		}
	}
}

func TestPlaceLRejectsIllegalPlacement(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	before := s.Board
	// An illegal placement: Black's own current position re-offered (must
	// differ from the old position).
	cur := currentLCells(&s.Board, Black)
	bogus := Placement{Cells: cur}
	if s.PlaceL(bogus) {
		t.Fatal("placing the piece back at its own current position must be rejected")
	}
	if s.Board != before {
		t.Fatal("a rejected placement must not mutate the board")
	}
	if s.Step != StepPlaceL {
		t.Fatal("a rejected placement must not advance Step")
	}
}

func TestPlaceLRejectsWhenNotYourStep(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	pl := LegalLPlacements(s.Board, Black)[0]
	if !s.PlaceL(pl) {
		t.Fatal("setup: first placement should succeed")
	}
	// Now in StepNeutralOptional; a second PlaceL call must be rejected.
	another := LegalLPlacements(s.Board, Black)
	if len(another) > 0 && s.PlaceL(another[0]) {
		t.Fatal("PlaceL must be rejected once already in the neutral-optional step")
	}
}

func TestSkipNeutralEndsTurnWithoutChangingNeutrals(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	pl := LegalLPlacements(s.Board, Black)[0]
	s.PlaceL(pl)
	beforeNeutral := s.Board.occupiedCells(Neutral)
	if !s.SkipNeutral() {
		t.Fatal("SkipNeutral should be legal right after PlaceL")
	}
	afterNeutral := s.Board.occupiedCells(Neutral)
	if len(beforeNeutral) != len(afterNeutral) {
		t.Fatal("neutral piece count changed across a skip")
	}
	for i := range beforeNeutral {
		if beforeNeutral[i] != afterNeutral[i] {
			t.Fatal("skipping the neutral move must not move any neutral piece")
		}
	}
	if s.Turn != White {
		t.Fatalf("Turn = %v, want White after Black's full turn", s.Turn)
	}
	if s.Step != StepPlaceL {
		t.Fatal("Step must reset to StepPlaceL for the next player")
	}
}

func TestMoveNeutralEndsTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	pl := LegalLPlacements(s.Board, Black)[0]
	s.PlaceL(pl)
	nm := LegalNeutralMoves(s.Board)[0]
	if !s.MoveNeutral(nm) {
		t.Fatal("a legal neutral move should be accepted")
	}
	if s.Board.At(nm.To.X, nm.To.Y) != Neutral {
		t.Error("destination cell should hold Neutral")
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White after the neutral move")
	}
}

func TestMoveNeutralRejectedBeforePlaceL(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	nm := LegalNeutralMoves(s.Board)[0]
	if s.MoveNeutral(nm) {
		t.Fatal("MoveNeutral must be rejected before the mandatory L placement is made")
	}
	if s.Turn != Black || s.Step != StepPlaceL {
		t.Fatal("state must be unchanged by a rejected neutral move")
	}
}

func TestMoveNeutralRejectsIllegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	pl := LegalLPlacements(s.Board, Black)[0]
	s.PlaceL(pl)
	before := s.Board
	bad := NeutralMove{From: image.Pt(0, 0), To: image.Pt(0, 0)} // (0,0) isn't Neutral
	if s.MoveNeutral(bad) {
		t.Fatal("an illegal neutral move must be rejected")
	}
	if s.Board != before {
		t.Fatal("a rejected neutral move must not mutate the board")
	}
}

// TestWinConditionNoLegalPlacementLosesImmediately constructs a position
// where, after Black's full turn, White has zero legal L-placements, and
// checks the game ends immediately in Black's favor without White moving.
func TestWinConditionNoLegalPlacementLosesImmediately(t *testing.T) {
	// Build a position where White's L is fully boxed in: every other cell
	// on the board belongs to Black, so White has no legal placement
	// anywhere (any candidate either reuses its own current 4 cells,
	// excluded as "no change", or requires a Black-occupied cell).
	var b2 Board
	placeShape(&b2, Black, 1, image.Pt(0, 0)) // some Black L
	placeShape(&b2, White, 0, image.Pt(1, 2)) // White's boxed-in L: (1,2)(2,2)(3,2)(3,3)
	// Fill every remaining cell with Neutral-like blockers using Black so
	// White truly has nowhere to go. We only have 2 real neutral pieces, so
	// use Black for the rest (doesn't matter who owns the blocking cells
	// for this legality check, only that they're non-empty/non-White).
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b2.At(x, y) == Empty {
				b2.set(x, y, Black)
			}
		}
	}
	if len(LegalLPlacements(b2, White)) != 0 {
		t.Fatal("test setup bug: White should have zero legal placements on b2")
	}
	s2 := NewGame(OpponentHotseat, 0)
	s2.Board = b2
	s2.Turn = Black
	s2.Step = StepNeutralOptional // pretend Black just placed its L
	if !s2.SkipNeutral() {
		t.Fatal("SkipNeutral should succeed to end Black's turn")
	}
	if s2.Phase != PhaseDone {
		t.Fatal("Phase should be Done: White has no legal L-placement on its turn")
	}
	if s2.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black", s2.Winner())
	}
}

func TestAITurnAndStepAI(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	if s.AITurn() {
		t.Fatal("AITurn should be false while it's Black's (the human's) turn")
	}
	pl := LegalLPlacements(s.Board, Black)[0]
	s.PlaceL(pl)
	s.SkipNeutral()
	if s.Turn != White {
		t.Fatal("setup: turn should now be White's (the AI's)")
	}
	if !s.AITurn() {
		t.Fatal("AITurn should be true on White's turn in OpponentAI mode")
	}
	before := s.Board
	if !s.StepAI() {
		t.Fatal("StepAI should make a move")
	}
	if s.Board == before && s.Phase == PhasePlaying {
		t.Fatal("StepAI should have changed the board")
	}
	if s.Turn != Black && s.Phase == PhasePlaying {
		t.Fatal("after the AI's full turn, play should return to Black")
	}
}

func TestHotseatAITurnAlwaysFalse(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.AITurn() {
		t.Fatal("hotseat games never have an AI turn")
	}
	pl := LegalLPlacements(s.Board, Black)[0]
	s.PlaceL(pl)
	s.SkipNeutral()
	if s.AITurn() {
		t.Fatal("hotseat games never have an AI turn, even on White's turn")
	}
}
