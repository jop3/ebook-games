package game

import (
	"image"
	"testing"
)

// TestStepAIFullGameReachesEnd drives an entire hot-seat-style game where
// BOTH players are played by the same greedy heuristic (player 1 through
// the exported StepAI, player 0 by applying aiChoose directly), which
// exercises the whole rules engine end to end: BuyPatch, Advance,
// PlaceFreePatch, income crossing, the 7x7 bonus, and game-end detection.
func TestStepAIFullGameReachesEnd(t *testing.T) {
	s := NewGame(OpponentAI)
	steps := 0
	for !s.GameOver() {
		steps++
		if steps > 20000 {
			t.Fatalf("game did not terminate after %d steps (m0=%d m1=%d pending=%v)",
				steps, s.Marker[0], s.Marker[1], s.Pending)
		}
		if s.Pending[0] > 0 {
			pt, ok := findEmptyCell(&s.Boards[0])
			if !ok {
				s.Pending[0] = 0
				s.maybeFinish()
				continue
			}
			if !s.PlaceFreePatch(0, pt) {
				t.Fatalf("step %d: PlaceFreePatch(0, %v) failed", steps, pt)
			}
			continue
		}
		if s.ActingPlayer() == 1 {
			if !s.StepAI() {
				t.Fatalf("step %d: StepAI() returned false while it was player 1's turn", steps)
			}
			continue
		}
		choice := aiChoose(s, 0)
		var ok bool
		if choice.isAdvance {
			ok = s.Advance(0)
		} else {
			ok = s.BuyPatch(0, choice.offset, choice.orientIdx, choice.anchor)
		}
		if !ok {
			t.Fatalf("step %d: player 0 action failed: %+v", steps, choice)
		}
	}

	if s.Marker[0] != TrackEnd || s.Marker[1] != TrackEnd {
		t.Fatalf("game ended with markers not at TrackEnd: %d, %d", s.Marker[0], s.Marker[1])
	}
	if s.Pending[0] != 0 || s.Pending[1] != 0 {
		t.Fatalf("game ended with unresolved pending free patches: %v", s.Pending)
	}
	// Both scores must be computable and the winner (if any) well-formed.
	sc0, sc1 := s.FinalScore(0), s.FinalScore(1)
	w := s.Winner()
	if w < -1 || w > 1 {
		t.Fatalf("Winner() = %d, out of range", w)
	}
	if (sc0 > sc1 && w != 0) || (sc1 > sc0 && w != 1) || (sc0 == sc1 && w != -1) {
		t.Fatalf("Winner()=%d inconsistent with scores %d/%d", w, sc0, sc1)
	}
	t.Logf("full AI-vs-AI game finished in %d steps, scores %d/%d, winner=%d, bonus owner=%d",
		steps, sc0, sc1, w, s.BonusOwner)
}

func TestStepAIReturnsFalseWhenNotAITurn(t *testing.T) {
	s := NewGame(OpponentAI)
	s.Marker[0] = 0
	s.Marker[1] = 5 // player 0 is trailing -> it's player 0's turn, not the AI's (player 1)
	if s.StepAI() {
		t.Fatal("expected StepAI to do nothing when it is not player 1's turn")
	}
}

func TestStepAIResolvesPendingBeforeNormalAction(t *testing.T) {
	s := NewGame(OpponentAI)
	s.Pending[1] = 1
	s.Marker[0] = 20
	s.Marker[1] = 0 // ordinarily player 1 would be the trailing actor anyway here
	if !s.StepAI() {
		t.Fatal("expected StepAI to resolve the pending free patch")
	}
	if s.Pending[1] != 0 {
		t.Fatalf("Pending[1] = %d, want 0 after StepAI resolves it", s.Pending[1])
	}
	if s.Boards[1].FilledCount() != 1 {
		t.Fatalf("expected exactly one cell filled by the resolved free patch, got %d", s.Boards[1].FilledCount())
	}
}

func TestFragmentationPenaltyPrefersLessIsolatingPlacement(t *testing.T) {
	var b Board
	// Build a wall so that cell (2,2) is nearly enclosed: filled at
	// (1,2),(3,2),(2,1) — placing a patch cell at (2,3) would seal the
	// last side and make (2,2) a fully dead 4-neighbor pocket.
	b.Filled[2][1] = true
	b.Filled[2][3] = true
	b.Filled[1][2] = true

	sealing := fragmentationPenalty(&b, []image.Point{{X: 2, Y: 3}})
	elsewhere := fragmentationPenalty(&b, []image.Point{{X: 7, Y: 7}})
	if sealing <= elsewhere {
		t.Fatalf("expected sealing the last side of a pocket (%v) to score a higher penalty than an unrelated cell (%v)", sealing, elsewhere)
	}
}
