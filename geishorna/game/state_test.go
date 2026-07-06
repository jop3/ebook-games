package game

import (
	"math/rand"
	"testing"
)

func TestNewDeckComposition(t *testing.T) {
	d := NewDeck()
	if len(d) != TotalCards {
		t.Fatalf("len(NewDeck()) = %d, want %d", len(d), TotalCards)
	}
	counts := map[int]int{}
	for _, c := range d {
		counts[c.Geisha]++
	}
	if len(counts) != NumGeishas {
		t.Fatalf("deck spans %d geishas, want %d", len(counts), NumGeishas)
	}
	sumCharm := 0
	for g := 0; g < NumGeishas; g++ {
		if counts[g] != Geishas[g].Charm {
			t.Errorf("geisha %d (%s): %d cards, want %d (its charm)", g, Geishas[g].Name, counts[g], Geishas[g].Charm)
		}
		sumCharm += Geishas[g].Charm
	}
	if sumCharm != TotalCharm {
		t.Errorf("total charm = %d, want %d", sumCharm, TotalCharm)
	}
}

func TestActionCardCounts(t *testing.T) {
	want := map[Action]int{Secret: 1, TradeOff: 2, Gift: 3, Competition: 4}
	sum := 0
	for a, n := range want {
		if a.Cards() != n {
			t.Errorf("%s.Cards() = %d, want %d", a.Name(), a.Cards(), n)
		}
		sum += n
	}
	// 6 dealt + 4 drawn = 10 cards used across the 4 actions.
	if sum != HandStart+NumActions {
		t.Errorf("actions consume %d cards, but a player handles %d", sum, HandStart+NumActions)
	}
}

func TestNewGameDeal(t *testing.T) {
	s := NewGame(nil)
	// Human leads round 1 and has just drawn: 6 dealt + 1 draw = 7.
	if got := len(s.Players[HumanIdx].Hand); got != HandStart+1 {
		t.Errorf("leader hand = %d, want %d", got, HandStart+1)
	}
	if got := len(s.Players[AIIdx].Hand); got != HandStart {
		t.Errorf("follower hand = %d, want %d", got, HandStart)
	}
	// 21 - 1 removed - 12 dealt - 1 drawn = 7 left in the deck.
	if got := len(s.Deck); got != 7 {
		t.Errorf("deck = %d, want 7", got)
	}
	if s.Turn != HumanIdx {
		t.Errorf("round 1 leader = %d, want human", s.Turn)
	}
	for i := 0; i < NumGeishas; i++ {
		if s.Favor[i] != -1 {
			t.Errorf("favor[%d] = %d at start, want -1 (neutral)", i, s.Favor[i])
		}
	}
}

func TestSecretIsScoredForActor(t *testing.T) {
	s := NewGame(nil)
	c := s.Players[HumanIdx].Hand[0]
	if err := s.DoSecret(HumanIdx, 0); err != nil {
		t.Fatal(err)
	}
	if !s.Players[HumanIdx].HasSecret || s.Players[HumanIdx].Secret != c {
		t.Fatalf("secret not stashed correctly")
	}
	// The secret does not appear in the public field until reveal.
	if s.Players[HumanIdx].Field[c.Geisha] != 0 {
		t.Errorf("secret leaked into the public field before reveal")
	}
	if !s.Players[HumanIdx].Used[Secret] {
		t.Errorf("secret marker not marked used")
	}
	if s.Turn != AIIdx {
		t.Errorf("turn did not pass after secret")
	}
}

func TestTradeOffRemovesCards(t *testing.T) {
	s := NewGame(nil)
	before := len(s.Players[HumanIdx].Hand)
	if err := s.DoTradeOff(HumanIdx, []int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if got := len(s.Players[HumanIdx].Hand); got != before-2 {
		t.Errorf("hand = %d after trade-off, want %d", got, before-2)
	}
	for i := 0; i < NumGeishas; i++ {
		if s.Players[HumanIdx].Field[i] != 0 || s.Players[AIIdx].Field[i] != 0 {
			t.Errorf("trade-off should score for no one, geisha %d changed", i)
		}
	}
}

func TestGiftSplit(t *testing.T) {
	s := NewGame(nil)
	s.Players[HumanIdx].Hand = []Card{{Geisha: 0}, {Geisha: 1}, {Geisha: 2}, {Geisha: 3}}
	if err := s.DoGift(HumanIdx, []int{0, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if s.Pending == nil || s.Pending.Action != Gift {
		t.Fatalf("gift should open a pending offer")
	}
	if s.ToAct() != AIIdx {
		t.Errorf("ToAct = %d after gift, want AI (the chooser)", s.ToAct())
	}
	// AI (chooser) keeps geisha 2's card; actor keeps geishas 0 and 1.
	if err := s.ResolveGift(AIIdx, 2); err != nil {
		t.Fatal(err)
	}
	if s.Players[AIIdx].Field[2] != 1 {
		t.Errorf("chooser should hold geisha 2")
	}
	if s.Players[HumanIdx].Field[0] != 1 || s.Players[HumanIdx].Field[1] != 1 {
		t.Errorf("actor should keep geishas 0 and 1")
	}
	if s.Turn != AIIdx {
		t.Errorf("after resolving the human's gift it should be the AI's turn")
	}
}

func TestCompetitionSplit(t *testing.T) {
	s := NewGame(nil)
	s.Players[HumanIdx].Hand = []Card{{Geisha: 0}, {Geisha: 1}, {Geisha: 2}, {Geisha: 3}, {Geisha: 4}}
	if err := s.DoCompetition(HumanIdx, []int{0, 1}, []int{2, 3}); err != nil {
		t.Fatal(err)
	}
	if s.Pending == nil || s.Pending.Action != Competition {
		t.Fatalf("competition should open a pending offer")
	}
	// Chooser takes pair index 1 ({2,3}); actor keeps pair 0 ({0,1}).
	if err := s.ResolveCompetition(AIIdx, 1); err != nil {
		t.Fatal(err)
	}
	if s.Players[AIIdx].Field[2] != 1 || s.Players[AIIdx].Field[3] != 1 {
		t.Errorf("chooser should hold geishas 2 and 3")
	}
	if s.Players[HumanIdx].Field[0] != 1 || s.Players[HumanIdx].Field[1] != 1 {
		t.Errorf("actor should keep geishas 0 and 1")
	}
}

func TestIllegalActions(t *testing.T) {
	s := NewGame(nil)
	if err := s.DoSecret(AIIdx, 0); err == nil {
		t.Errorf("AI acting on the human's turn should fail")
	}
	if err := s.DoSecret(HumanIdx, 99); err == nil {
		t.Errorf("out-of-range index should fail")
	}
	if err := s.DoTradeOff(HumanIdx, []int{0, 0}); err == nil {
		t.Errorf("duplicate indices should fail")
	}
	if err := s.DoSecret(HumanIdx, 0); err != nil {
		t.Fatal(err)
	}
	// It is now the AI's turn; a human action must be rejected.
	if err := s.DoTradeOff(HumanIdx, []int{0, 1}); err == nil {
		t.Errorf("acting out of turn should fail")
	}
}

// TestScoringAndFavor checks reveal, per-geisha majority, ties, and favor
// persistence directly by hand-setting fields.
func TestScoringAndFavor(t *testing.T) {
	s := NewGame(nil)
	// Pretend the round has played out; hand-set the final fields.
	s.Players[HumanIdx].Field = [NumGeishas]int{2, 0, 1, 0, 0, 0, 0}
	s.Players[AIIdx].Field = [NumGeishas]int{1, 3, 1, 0, 0, 0, 0}
	// Give the human a secret on geisha 3 to swing it.
	s.Players[HumanIdx].HasSecret = true
	s.Players[HumanIdx].Secret = Card{Geisha: 3}
	// Force all markers used so scoreRound is legitimate to call via afterAction.
	for p := 0; p < 2; p++ {
		for a := range s.Players[p].Used {
			s.Players[p].Used[a] = true
		}
	}
	s.scoreRound()

	// geisha 0: human 2 vs AI 1 -> human.
	if s.Favor[0] != HumanIdx {
		t.Errorf("geisha 0 favor = %d, want human", s.Favor[0])
	}
	// geisha 1: human 0 vs AI 3 -> AI.
	if s.Favor[1] != AIIdx {
		t.Errorf("geisha 1 favor = %d, want AI", s.Favor[1])
	}
	// geisha 2: tie 1-1 -> stays neutral.
	if s.Favor[2] != -1 {
		t.Errorf("geisha 2 favor = %d, want neutral (tie)", s.Favor[2])
	}
	// geisha 3: human's secret makes it 1-0 human.
	if s.Favor[3] != HumanIdx {
		t.Errorf("geisha 3 favor = %d, want human (secret)", s.Favor[3])
	}
}

func TestTieKeepsExistingMarker(t *testing.T) {
	s := NewGame(nil)
	s.Favor[0] = AIIdx // AI already holds geisha 0 from a prior round
	s.Players[HumanIdx].Field = [NumGeishas]int{1}
	s.Players[AIIdx].Field = [NumGeishas]int{1}
	for p := 0; p < 2; p++ {
		for a := range s.Players[p].Used {
			s.Players[p].Used[a] = true
		}
	}
	s.scoreRound()
	if s.Favor[0] != AIIdx {
		t.Errorf("a tie should leave geisha 0 with its prior holder (AI), got %d", s.Favor[0])
	}
}

func TestMatchWinConditions(t *testing.T) {
	// Charm >= 11 wins. Geishas: P=5, V=4, then a 2 -> 11.
	s := NewGame(nil)
	s.Favor = [NumGeishas]int{HumanIdx, HumanIdx, -1, -1, HumanIdx, -1, -1}
	if _, charm := s.Standing(HumanIdx); charm != 11 {
		t.Fatalf("setup charm = %d, want 11", charm)
	}
	if s.matchWinner() != HumanIdx {
		t.Errorf("11 charm should win the match for the human")
	}

	// 4 geishas wins even at low charm (three 2s + one 3 = 9 charm).
	s2 := NewGame(nil)
	s2.Favor = [NumGeishas]int{-1, -1, AIIdx, -1, AIIdx, AIIdx, AIIdx}
	if g, c := s2.Standing(AIIdx); g != 4 || c != 9 {
		t.Fatalf("setup = %d geishas / %d charm, want 4 / 9", g, c)
	}
	if s2.matchWinner() != AIIdx {
		t.Errorf("4 geishas should win the match for the AI")
	}

	// Nobody at threshold -> no winner yet.
	s3 := NewGame(nil)
	s3.Favor = [NumGeishas]int{HumanIdx, AIIdx, -1, -1, -1, -1, -1}
	if s3.matchWinner() != -1 {
		t.Errorf("no threshold met should mean no winner")
	}
}

// trivialDrive plays whoever is to move with the simplest legal policy: resolve
// an open offer by taking index 0, else spend the first affordable action on
// the lowest-index cards. Enough to run whole rounds/matches end to end.
func trivialDrive(s *State) {
	p := s.ToAct()
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

// TestFullRoundConsumesEverything drives a whole round and checks the card
// arithmetic closes out exactly.
func TestFullRoundConsumesEverything(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	s := NewGame(rng.Shuffle)
	for guard := 0; s.Phase == PhaseAction; guard++ {
		if guard > 100 {
			t.Fatal("round did not terminate")
		}
		trivialDrive(s)
	}
	if s.Phase != PhaseRoundEnd {
		t.Fatalf("phase = %v after a full round, want round-end", s.Phase)
	}
	for p := 0; p < 2; p++ {
		if s.Players[p].usedCount() != NumActions {
			t.Errorf("player %d used %d markers, want %d", p, s.Players[p].usedCount(), NumActions)
		}
		if len(s.Players[p].Hand) != 0 {
			t.Errorf("player %d ended with %d cards, want 0", p, len(s.Players[p].Hand))
		}
	}
	if len(s.Deck) != 0 {
		t.Errorf("deck = %d at round end, want 0", len(s.Deck))
	}
	placed := 0
	for i := 0; i < NumGeishas; i++ {
		placed += s.Players[HumanIdx].Field[i] + s.Players[AIIdx].Field[i]
	}
	// Fields include revealed secrets. Trade-offs (2 each) and the 1 removed
	// card are gone: 21 - 1 - 4 = 16 placed.
	if placed != TotalCards-1-2*2 {
		t.Errorf("placed cards = %d, want %d", placed, TotalCards-1-2*2)
	}
}

// TestMatchTerminates plays entire matches with the trivial driver and checks
// they always reach a winner within a sane number of rounds.
func TestMatchTerminates(t *testing.T) {
	for seed := int64(0); seed < 30; seed++ {
		rng := rand.New(rand.NewSource(seed))
		s := NewGame(rng.Shuffle)
		guard := 0
		for s.Phase != PhaseMatchEnd {
			guard++
			if guard > 5000 {
				t.Fatalf("seed %d: match did not terminate", seed)
			}
			if s.Phase == PhaseRoundEnd {
				s.Continue()
				continue
			}
			trivialDrive(s)
		}
		if s.MatchWinner != HumanIdx && s.MatchWinner != AIIdx {
			t.Fatalf("seed %d: match ended with no winner (%d)", seed, s.MatchWinner)
		}
		g, c := s.Standing(s.MatchWinner)
		if g < GeishasToWin && c < CharmToWin {
			t.Fatalf("seed %d: winner holds %d geishas / %d charm, below both thresholds", seed, g, c)
		}
	}
}
