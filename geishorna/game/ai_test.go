package game

import (
	"math/rand"
	"testing"
)

// TestAIPlaysLegalFullRound runs the real AI on both seats (by driving the
// human seat through StepAI-equivalent logic) — here we drive the human with a
// trivial policy and the AI with StepAI, then assert a legal, complete round.
func TestAIvsTrivialCompletesRound(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	s := NewGame(rng.Shuffle)
	for guard := 0; s.Phase == PhaseAction; guard++ {
		if guard > 200 {
			t.Fatal("round did not terminate")
		}
		if s.AITurn() {
			if !s.StepAI() {
				t.Fatal("StepAI reported nothing to do while it was the AI's turn")
			}
			continue
		}
		// human seat: trivial policy
		p := HumanIdx
		if s.Pending != nil {
			if s.Pending.Action == Gift {
				_ = s.ResolveGift(p, 0)
			} else {
				_ = s.ResolveCompetition(p, 0)
			}
			continue
		}
		for _, a := range AllActions {
			if !s.Available(p, a) {
				continue
			}
			switch a {
			case Secret:
				_ = s.DoSecret(p, 0)
			case TradeOff:
				_ = s.DoTradeOff(p, []int{0, 1})
			case Gift:
				_ = s.DoGift(p, []int{0, 1, 2})
			case Competition:
				_ = s.DoCompetition(p, []int{0, 1}, []int{2, 3})
			}
			break
		}
	}
	if s.Phase != PhaseRoundEnd {
		t.Fatalf("phase = %v, want round-end", s.Phase)
	}
	if s.Players[AIIdx].usedCount() != NumActions {
		t.Errorf("AI spent %d markers, want %d", s.Players[AIIdx].usedCount(), NumActions)
	}
	if len(s.Players[AIIdx].Hand) != 0 {
		t.Errorf("AI ended with %d cards, want 0", len(s.Players[AIIdx].Hand))
	}
}

// TestAISecretPicksPivotalCard: with the AI one card short of majority on the
// high-charm Pärlan (geisha 0), keeping that card secret should be its best
// Secret pick.
func TestAISecretPicksPivotalCard(t *testing.T) {
	s := NewGame(nil)
	// AI to act.
	s.Turn = AIIdx
	// Contest on geisha 0: even so far; a single card wins it.
	s.Players[AIIdx].Field = [NumGeishas]int{1, 0, 0, 0, 0, 0, 0}
	s.Players[HumanIdx].Field = [NumGeishas]int{1, 0, 0, 0, 0, 0, 0}
	s.Players[AIIdx].Hand = []Card{{Geisha: 6}, {Geisha: 0}, {Geisha: 6}}
	base := s.aiFieldSnapshot()
	idx, _ := s.aiBestSecret(s.Players[AIIdx].Hand, base)
	if s.Players[AIIdx].Hand[idx].Geisha != 0 {
		t.Errorf("AI secreted geisha %d, want the pivotal geisha 0 (Pärlan, charm 5)", s.Players[AIIdx].Hand[idx].Geisha)
	}
}

// TestAIChoosesBetterCompetitionPair: when the human offers two pairs and one
// clearly helps the AI more, the AI takes it.
func TestAIChooserTakesBetterPair(t *testing.T) {
	s := NewGame(nil)
	s.Turn = HumanIdx
	// Set an open competition offer with the AI as chooser: pair 0 = two copies
	// of high-charm geisha 0, pair 1 = two low geishas.
	s.Pending = &Pending{
		Action:  Competition,
		Actor:   HumanIdx,
		Chooser: AIIdx,
		Groups:  [2][]Card{{{Geisha: 0}, {Geisha: 0}}, {{Geisha: 6}, {Geisha: 6}}},
	}
	if got := s.aiChooseCompetition(); got != 0 {
		t.Errorf("AI took pair %d, want pair 0 (the high-charm Pärlan pair)", got)
	}
}

// TestAIMatchIsCompetitive: the AI should win a healthy share of matches
// against the trivial "first legal move" driver, confirming its heuristics do
// something rather than playing randomly.
func TestAIBeatsTrivialOften(t *testing.T) {
	aiWins := 0
	matches := 40
	for seed := int64(0); seed < int64(matches); seed++ {
		rng := rand.New(rand.NewSource(seed + 100))
		s := NewGame(rng.Shuffle)
		guard := 0
		for s.Phase != PhaseMatchEnd {
			guard++
			if guard > 20000 {
				t.Fatalf("seed %d: match did not terminate", seed)
			}
			if s.Phase == PhaseRoundEnd {
				s.Continue()
				continue
			}
			if s.AITurn() {
				s.StepAI()
				continue
			}
			driveHumanTrivial(s)
		}
		if s.MatchWinner == AIIdx {
			aiWins++
		}
	}
	// A competent greedy AI should beat the "first legal move" strawman clearly.
	if aiWins < matches*3/5 {
		t.Errorf("AI won %d/%d vs the trivial driver, expected a clear majority", aiWins, matches)
	}
}

func driveHumanTrivial(s *State) {
	p := HumanIdx
	if s.ToAct() != p {
		return
	}
	if s.Pending != nil {
		if s.Pending.Action == Gift {
			_ = s.ResolveGift(p, 0)
		} else {
			_ = s.ResolveCompetition(p, 0)
		}
		return
	}
	for _, a := range AllActions {
		if !s.Available(p, a) {
			continue
		}
		switch a {
		case Secret:
			_ = s.DoSecret(p, 0)
		case TradeOff:
			_ = s.DoTradeOff(p, []int{0, 1})
		case Gift:
			_ = s.DoGift(p, []int{0, 1, 2})
		case Competition:
			_ = s.DoCompetition(p, []int{0, 1}, []int{2, 3})
		}
		return
	}
}
