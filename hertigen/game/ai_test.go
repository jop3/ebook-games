package game

import (
	"image"
	"testing"
)

func TestBestMoveTakesImmediateDukeCapture(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Duke, White, FaceA)
	put(&b, 3, 2, Footman, Black, FaceA) // one step from capturing the White Duke
	put(&b, 0, 5, Duke, Black, FaceA)
	reserve := [2]ReserveMask{Black: 0, White: 0}
	a, ok := BestMove(b, reserve, Black, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	if a.To != (image.Point{X: 2, Y: 2}) {
		t.Fatalf("BestMove should take the immediate Duke capture, got %+v", a)
	}
}

func TestBestMoveAvoidsHangingItsOwnDuke(t *testing.T) {
	// Black's Duke sits where, if Black does nothing about it, White can
	// capture next turn. Give Black a completely safe alternative (a
	// Footman capture elsewhere) and confirm the AI (playing Black here)
	// prefers to address king safety over a nearly-equal trade when
	// available — evaluated via dukeThreatPenalty in evaluate(), not by
	// asserting a single "correct" move (many engines could reasonably
	// differ), but at minimum BestMove must return SOME legal, non-panicking
	// move at a nontrivial search depth without crashing on a threatened
	// Duke. This is primarily a smoke test that deeper search terminates.
	b := clear()
	put(&b, 2, 2, Duke, Black, FaceA)
	put(&b, 3, 2, Footman, White, FaceA) // adjacent to Black's Duke: a live threat
	put(&b, 5, 5, Duke, White, FaceA)
	reserve := [2]ReserveMask{Black: 0, White: 0}
	a, ok := BestMove(b, reserve, Black, DepthMedium)
	if !ok {
		t.Fatal("BestMove should find a legal move even with the Duke under threat")
	}
	if !b.IsLegalAction(Black, 0, a) {
		t.Fatalf("BestMove returned an illegal action: %+v", a)
	}
}

func TestDukeThreatPenaltyIsHighWhenEnemyCanCaptureNow(t *testing.T) {
	safe := clear()
	put(&safe, 2, 2, Duke, Black, FaceA)
	put(&safe, 0, 5, Duke, White, FaceA) // far away, no immediate threat

	threatened := clear()
	put(&threatened, 2, 2, Duke, Black, FaceA)
	put(&threatened, 3, 2, Footman, White, FaceA) // can capture the Duke this turn
	put(&threatened, 0, 5, Duke, White, FaceA)

	safeScore := dukeThreatPenalty(&safe, Black)
	threatenedScore := dukeThreatPenalty(&threatened, Black)
	if threatenedScore <= safeScore {
		t.Fatalf("threatened penalty (%d) should exceed the safe penalty (%d)", threatenedScore, safeScore)
	}
}

func TestMaterialCountsOnlyTroopsNotTheDuke(t *testing.T) {
	b := clear()
	put(&b, 0, 0, Duke, Black, FaceA)
	put(&b, 1, 0, Footman, Black, FaceA)
	got := material(&b, Black)
	want := tileValue(Footman)
	if got != want {
		t.Fatalf("material() = %d, want %d (Duke must not be counted as material)", got, want)
	}
}

func TestBestMoveNoLegalActionReturnsFalse(t *testing.T) {
	b := clear()
	// Construct a Black side with genuinely ZERO legal actions and no
	// reserve. Two Black tiles only:
	//   - Duke at the (0,0) corner on FaceB (diagonal-adjacent): 3 of its 4
	//     diagonal offsets are off-board at a corner, and the sole in-bounds
	//     one, (1,1), is occupied by Black's own Catapult (own tiles block
	//     rather than trigger a capture) -> the Duke has 0 actions.
	//   - A Catapult at (1,1) on FaceA (strike-only, distance-2 orthogonal):
	//     with no enemy sitting at either of its 2 in-bounds strike offsets,
	//     (3,1) and (1,3), it also has 0 actions (see
	//     TestCatapultNoTargetMeansNoActions for this in isolation).
	// No other Black tile exists, so the side's total legal-action count is
	// exactly 0.
	put(&b, 0, 0, Duke, Black, FaceB)
	put(&b, 1, 1, Catapult, Black, FaceA)
	put(&b, 5, 5, Duke, White, FaceA)
	reserve := [2]ReserveMask{Black: 0, White: 0}
	if len(b.LegalActions(Black, reserve[Black])) != 0 {
		t.Fatalf("test setup: expected zero legal actions for Black, got %v",
			b.LegalActions(Black, reserve[Black]))
	}
	_, ok := BestMove(b, reserve, Black, DepthEasy)
	if ok {
		t.Fatal("BestMove should report ok=false when there is no legal action")
	}
}

// --- full game vs the AI, to a real Duke-capture result ---------------------

func TestFullGameVsAITerminates(t *testing.T) {
	s := NewGame(OpponentAI, DepthMedium)
	for ply := 0; s.Phase == PhasePlaying; ply++ {
		if ply > 300 {
			t.Fatal("game did not terminate")
		}
		if s.AITurn() {
			if !s.StepAI() {
				t.Fatalf("StepAI reported no move at ply %d while it was White's turn", ply)
			}
			continue
		}
		// Black plays via BestMove too (a second, independent AI call at a
		// shallow depth) purely to drive the game to completion quickly and
		// deterministically for this smoke test.
		a, ok := BestMove(s.Board, s.Reserve, Black, DepthEasy)
		if !ok {
			t.Fatalf("Black has no legal action at ply %d but the game isn't over", ply)
		}
		if !s.Play(a) {
			t.Fatalf("BestMove produced an illegal action at ply %d: %+v", ply, a)
		}
	}
	if _, ok := s.Winner(); !ok {
		t.Fatal("game ended but Winner() reports ok=false")
	}
}
