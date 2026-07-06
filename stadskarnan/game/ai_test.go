package game

import (
	"image"
	"testing"
)

// TestBestPlacementReturnsLegalMove checks the AI's chosen placement is
// always actually legal on the current board (this would catch the AI
// picking an orientation/anchor combination that doesn't correspond to a
// real placement).
func TestBestPlacementReturnsLegalMove(t *testing.T) {
	s := NewGame(OpponentAI)
	s.PlaceCathedral(LegalCathedralPlacements(&s.Board)[0].Anchor)
	if s.Turn != White {
		t.Fatal("setup: White should move first")
	}

	pieceID, orientIdx, anchor, ok := BestPlacement(s, White)
	if !ok {
		t.Fatal("White should have a legal placement on a nearly-empty board")
	}
	found := false
	for _, p := range LegalPlacementsForOrientation(&s.Board, pieceID, orientIdx) {
		if p.Anchor == anchor {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("BestPlacement returned (piece %d, orient %d, anchor %v), which is not among the actual legal placements",
			pieceID, orientIdx, anchor)
	}
}

// TestSimulatePlacementRewardsCapture checks the AI's core scoring incentive
// directly: simulating a capturing placement must score strictly higher
// (from the mover's perspective) than simulating an equal-sized placement
// elsewhere that captures nothing — the depth-1 term BestPlacement's search
// is built on. This is checked at the scoring-primitive level rather than by
// asserting a specific end-to-end BestPlacement choice, because with the AI's
// 2-ply lookahead enabled a capture can legitimately hand the opponent a
// piece they can immediately redeploy, so "always take the capture" is not a
// universal truth about the full search — but rewarding the capture at the
// immediate (depth-1) level, which is exactly the game's own win condition
// (fewest unplaced squares), must always hold.
func TestSimulatePlacementRewardsCapture(t *testing.T) {
	s := NewGame(OpponentAI)
	s.Phase = PhasePlaying

	// Black's monomino sits trapped on 3 sides at (5,5); White's own
	// monomino placed at (5,6) completes the capture.
	place(&s.Board, 5, 5, Black, 1)
	place(&s.Board, 4, 5, White, 2)
	place(&s.Board, 6, 5, White, 3)
	place(&s.Board, 5, 4, White, 4)
	*s.Hand(Black) = NewHand()
	s.Hand(Black)[1] = false
	wh := NewHand()
	wh[2], wh[3], wh[4] = false, false, false
	s.Hands[1] = wh

	captureCells := []image.Point{{X: 5, Y: 6}}
	_, capturedHands := simulatePlacement(s.Board, s.Hands, White, 0, captureCells)
	capturedScore := gap(&capturedHands, White)

	// Same size (a monomino) placed somewhere far away that captures
	// nothing.
	plainCells := []image.Point{{X: 0, Y: 0}}
	_, plainHands := simulatePlacement(s.Board, s.Hands, White, 0, plainCells)
	plainScore := gap(&plainHands, White)

	if capturedScore <= plainScore {
		t.Fatalf("capturing placement scored %d, want strictly greater than the non-capturing placement's %d",
			capturedScore, plainScore)
	}
	if !capturedHands[handIndex(Black)][1] {
		t.Fatal("the captured piece should be available again in Black's hand after the simulated capture")
	}
}

// TestBestPlacementNoLegalMoveReturnsFalse checks BestPlacement reports ok
// == false when the side truly cannot place anything.
func TestBestPlacementNoLegalMoveReturnsFalse(t *testing.T) {
	s := NewGame(OpponentAI)
	s.Phase = PhasePlaying
	s.Turn = White
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			s.Board.Owner[y][x] = Black
		}
	}
	_, _, _, ok := BestPlacement(s, White)
	if ok {
		t.Fatal("BestPlacement should report no legal move on a full board")
	}
}

// TestStepAIAppliesAMove checks GameState.StepAI actually advances the game
// (mirrors the aiPend-after-paint pattern used by main.go/ui.go).
func TestStepAIAppliesAMove(t *testing.T) {
	s := NewGame(OpponentAI)
	s.PlaceCathedral(LegalCathedralPlacements(&s.Board)[0].Anchor)
	if !s.AITurn() {
		t.Fatal("setup: it should be the AI's (White's) turn")
	}
	before := s.Board
	if !s.StepAI() {
		t.Fatal("StepAI should apply a move")
	}
	if s.Board == before {
		t.Fatal("the board should have changed after the AI's move")
	}
	if s.AITurn() {
		t.Fatal("after the AI moves, control should return to the human (Black) unless White is somehow stuck immediately")
	}
}
