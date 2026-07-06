package game

import "testing"

func TestNewGameSetup(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 1)
	for c := Color(0); c < NumColors; c++ {
		if gs.Bank[c] != TokensPerColor2P {
			t.Fatalf("bank color %v = %d, want %d", c, gs.Bank[c], TokensPerColor2P)
		}
	}
	if gs.BankGold != GoldTokens {
		t.Fatalf("bank gold = %d, want %d", gs.BankGold, GoldTokens)
	}
	if len(gs.Nobles) != NumNobles2P {
		t.Fatalf("nobles drawn = %d, want %d", len(gs.Nobles), NumNobles2P)
	}
	// Nobles must be distinct (no noble template drawn twice).
	seen := map[int]bool{}
	for _, n := range gs.Nobles {
		if seen[n.ID] {
			t.Fatalf("duplicate noble ID %d drawn", n.ID)
		}
		seen[n.ID] = true
	}
	for t2 := 0; t2 < NumTiers; t2++ {
		for s := 0; s < TableauSlots; s++ {
			if gs.Tableau[t2][s].Tier == 0 {
				t.Fatalf("tableau[%d][%d] should be dealt a real card at game start", t2, s)
			}
			if gs.Tableau[t2][s].Tier != t2+1 {
				t.Fatalf("tableau[%d][%d] has tier %d, want %d", t2, s, gs.Tableau[t2][s].Tier, t2+1)
			}
		}
	}
	wantDeckLeft := [NumTiers]int{40 - 4, 30 - 4, 20 - 4}
	for i, want := range wantDeckLeft {
		if len(gs.Decks[i]) != want {
			t.Fatalf("deck %d has %d cards left, want %d", i, len(gs.Decks[i]), want)
		}
	}
	if gs.Turn != 0 {
		t.Fatal("player 0 should move first")
	}
	if gs.Phase != PhasePlaying {
		t.Fatal("game should start in PhasePlaying")
	}
	if gs.Winner() != -1 {
		t.Fatal("Winner() should be -1 (undetermined) before the game ends")
	}
}

func TestNewGameSeededIsDeterministic(t *testing.T) {
	a := NewGameSeeded(ModeAI, DepthEasy, 777)
	b := NewGameSeeded(ModeAI, DepthEasy, 777)
	if a.Tableau != b.Tableau {
		t.Fatal("same seed should deal an identical tableau")
	}
	for i := range a.Nobles {
		if a.Nobles[i].ID != b.Nobles[i].ID {
			t.Fatal("same seed should draw identical nobles in the same order")
		}
	}
}

func TestCloneIndependence(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 5)
	clone := gs.Clone()
	clone.Bank[ColorSolid] = 999
	clone.Players[0].Tokens[ColorRing] = 42
	clone.Decks[0] = clone.Decks[0][:1]
	clone.Nobles = clone.Nobles[:1]

	if gs.Bank[ColorSolid] == 999 {
		t.Fatal("mutating the clone's bank must not affect the original")
	}
	if gs.Players[0].Tokens[ColorRing] == 42 {
		t.Fatal("mutating the clone's player tokens must not affect the original")
	}
	if len(gs.Decks[0]) == 1 {
		t.Fatal("mutating the clone's deck must not affect the original")
	}
	if len(gs.Nobles) == 1 {
		t.Fatal("mutating the clone's nobles must not affect the original")
	}
}

func TestAITurn(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthEasy, 1)
	if gs.AITurn() {
		t.Fatal("player 0 moves first; should not report AI turn yet")
	}
	gs.Turn = 1
	if !gs.AITurn() {
		t.Fatal("player 1 in ModeAI should report AI turn")
	}
	gs.Phase = PhaseDone
	if gs.AITurn() {
		t.Fatal("a finished game should never report an AI turn")
	}
	gs2 := NewGameSeeded(ModeHotseat, 0, 1)
	gs2.Turn = 1
	if gs2.AITurn() {
		t.Fatal("hot-seat mode should never report an AI turn")
	}
}
