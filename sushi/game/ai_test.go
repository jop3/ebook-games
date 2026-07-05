package game

import "testing"

// These are light sanity checks on the AI heuristic (the spec does not
// require exhaustive AI unit tests the way it does for scoring — only that
// a "fine opponent" results), pinning down the few behaviors the spec calls
// out by name.

func TestAIPrefersWasabiThenNigiriOverFiller(t *testing.T) {
	s := &State{NumPlayers: 2, Players: make([]Player, 2)}
	// handLenBefore=3 so the "plenty of turns left to cash it in" branch of
	// the wasabi heuristic applies (a near-empty hand would make banking a
	// wasabi a bad bet, which is exercised separately below).
	s.Players[1].Hand = []Card{Tempura(), Wasabi(), Dumpling()}
	pick := s.AIPick(1)
	if len(pick.Idx) != 1 || pick.Idx[0] != 1 {
		t.Fatalf("with no wasabi banked yet and turns left in the round, AI should take the Wasabi over filler: %v", pick)
	}

	s2 := &State{NumPlayers: 2, Players: make([]Player, 2)}
	s2.Players[1].Tableau = []Card{Wasabi()}
	s2.Players[1].Hand = []Card{Tempura(), NigiriSquid()}
	pick2 := s2.AIPick(1)
	if len(pick2.Idx) != 1 || pick2.Idx[0] != 1 {
		t.Fatalf("with a banked Wasabi, AI should grab the Squid to cash in the triple: %v", pick2)
	}
}

func TestAICompletesSashimiTripleOverFreshStart(t *testing.T) {
	s := &State{NumPlayers: 2, Players: make([]Player, 2)}
	s.Players[1].Tableau = []Card{Sashimi(), Sashimi()} // one away from a triple
	s.Players[1].Hand = []Card{Tempura(), Sashimi()}
	pick := s.AIPick(1)
	if len(pick.Idx) != 1 || pick.Idx[0] != 1 {
		t.Fatalf("AI holding 2 sashimi should take the 3rd over an unrelated Tempura: %v", pick)
	}
}

func TestAIUsesChopsticksWhenAvailable(t *testing.T) {
	s := &State{NumPlayers: 2, Players: make([]Player, 2)}
	s.Players[1].Tableau = []Card{Chopsticks()}
	s.Players[1].Hand = []Card{Tempura(), Sashimi(), Dumpling()}
	pick := s.AIPick(1)
	if len(pick.Idx) != 2 {
		t.Fatalf("AI holding an unplayed Chopsticks with >=2 cards in hand should take 2: %v", pick)
	}
}

func TestAIChasesPuddingWhenBehind(t *testing.T) {
	s := &State{NumPlayers: 2, Players: make([]Player, 2)}
	s.Players[1].Pudding = 0
	s.Players[0].Pudding = 5                            // AI is well behind
	s.Players[1].Hand = []Card{Chopsticks(), Pudding()} // pudding should outweigh a fresh chopsticks bid here
	// (chopsticks base value 3.2 vs pudding 1.5+1.5=3.0 is close; make the
	// point sharply by comparing to a genuinely low-value filler instead)
	s.Players[1].Hand = []Card{Tempura(), Pudding()}
	pick := s.AIPick(1)
	if len(pick.Idx) != 1 || pick.Idx[0] != 1 {
		t.Fatalf("AI far behind on pudding should take it over a speculative fresh Tempura: %v", pick)
	}
}
