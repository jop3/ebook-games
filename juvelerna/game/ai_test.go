package game

import "testing"

func TestLegalActionsNonEmptyAtStart(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 3)
	actions := gs.LegalActions()
	if len(actions) == 0 {
		t.Fatal("a fresh game should always have legal actions")
	}
	// A fresh game has no affordable buys yet (no tokens), so every action
	// should be a take-3, take-2, or reserve.
	for _, a := range actions {
		if a.Kind == ActionBuyTableau || a.Kind == ActionBuyReserved {
			t.Fatalf("no buy should be legal with zero tokens, got %+v", a)
		}
	}
}

func TestBestActionReturnsLegalAction(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 9)
	for _, diff := range []int{DepthEasy, DepthMedium, DepthHard} {
		act, ok := BestAction(gs, diff)
		if !ok {
			t.Fatalf("diff %d: expected a legal action", diff)
		}
		clone := gs.Clone()
		if !clone.Apply(act) {
			t.Fatalf("diff %d: BestAction returned an illegal action %+v", diff, act)
		}
	}
}

func TestStepAIPlaysAndAdvances(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthEasy, 11)
	gs.Turn = 1
	before := gs.Players[1]
	if !gs.StepAI() {
		t.Fatal("StepAI should make a move on the AI's turn")
	}
	if boardEqual(gs.Players[1], before) && gs.Phase == PhasePlaying {
		t.Fatal("StepAI should have changed player 1's state (or moved to a sub-phase)")
	}
}

func TestStepAINoopWhenNotAITurn(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthEasy, 11)
	if gs.StepAI() {
		t.Fatal("StepAI must be a no-op on the human's turn")
	}
}

// boardEqual compares two PlayerStates for equality by value (they contain
// slices, so plain == doesn't work).
func boardEqual(a, b PlayerState) bool {
	if a.Tokens != b.Tokens || a.Gold != b.Gold || a.Bonuses != b.Bonuses || a.Prestige != b.Prestige {
		return false
	}
	if len(a.Cards) != len(b.Cards) || len(a.Reserved) != len(b.Reserved) || len(a.Nobles) != len(b.Nobles) {
		return false
	}
	return true
}

// TestFullAIVsAIGameReachesAWinner drives a complete AI-vs-AI game purely
// through the game package's own API (no UI) as a sanity net on top of the
// UI-level play_test.go: the AI must be able to play itself to a real
// PhaseDone, including navigating noble choices and forced discards, within
// a generous iteration budget.
func TestFullAIVsAIGameReachesAWinner(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		gs := NewGameSeeded(ModeAI, DepthMedium, seed)
		gs.Mode = ModeAI // both sides driven by StepAI below regardless of Mode's human/AI meaning
		finished := false
		for i := 0; i < 20000 && !finished; i++ {
			switch gs.Phase {
			case PhasePlaying:
				act, ok := BestAction(gs, DepthMedium)
				if !ok {
					t.Fatalf("seed %d: no legal action at iter %d (turn=%d)", seed, i, gs.Turn)
				}
				if !gs.Apply(act) {
					t.Fatalf("seed %d: BestAction produced an illegal action %+v at iter %d", seed, act, i)
				}
			case PhaseNobleChoice:
				if !gs.ChooseNoble(0) {
					t.Fatalf("seed %d: ChooseNoble(0) failed while pending %v", seed, gs.PendingNobles)
				}
			case PhaseDiscard:
				if !aiDiscardOnce(gs) {
					t.Fatalf("seed %d: aiDiscardOnce failed with DiscardNeeded=%d", seed, gs.DiscardNeeded)
				}
			case PhaseDone:
				finished = true
			}
		}
		if !finished {
			t.Fatalf("seed %d: game did not reach PhaseDone within the iteration budget", seed)
		}
		if gs.Players[0].Prestige < PrestigeToWin && gs.Players[1].Prestige < PrestigeToWin {
			t.Fatalf("seed %d: game ended but neither player reached %d prestige (%d vs %d)",
				seed, PrestigeToWin, gs.Players[0].Prestige, gs.Players[1].Prestige)
		}
		w := gs.Winner()
		if w != 0 && w != 1 && w != -1 {
			t.Fatalf("seed %d: invalid winner index %d", seed, w)
		}
		wantWinner := FinalWinner(summaryOf(gs.Players[0]), summaryOf(gs.Players[1]))
		if w != wantWinner {
			t.Fatalf("seed %d: GameState winner %d disagrees with independent FinalWinner %d", seed, w, wantWinner)
		}
	}
}
