package game

import "testing"

func newTestGame() *GameState {
	return NewGameSeeded(ModeHotseat, 0, 42)
}

// --- Take3 -------------------------------------------------------------

func TestTake3Legal(t *testing.T) {
	gs := newTestGame()
	before := gs.Bank
	ok := gs.Take3([3]Color{ColorSolid, ColorRing, ColorCross})
	if !ok {
		t.Fatal("taking 3 distinct colors with tokens available should be legal")
	}
	p := gs.Players[0]
	if p.Tokens[ColorSolid] != 1 || p.Tokens[ColorRing] != 1 || p.Tokens[ColorCross] != 1 {
		t.Fatalf("player should hold 1 of each taken color, got %+v", p.Tokens)
	}
	if gs.Bank[ColorSolid] != before[ColorSolid]-1 {
		t.Fatal("bank should have decremented")
	}
	if gs.Turn != 1 {
		t.Fatalf("turn should have passed to player 1, got %d", gs.Turn)
	}
}

func TestTake3RejectsDuplicateColors(t *testing.T) {
	gs := newTestGame()
	if gs.CanTake3([3]Color{ColorSolid, ColorSolid, ColorRing}) {
		t.Fatal("duplicate colors in a take-3 must be illegal")
	}
}

func TestTake3RejectsEmptyPile(t *testing.T) {
	gs := newTestGame()
	gs.Bank[ColorDot] = 0
	if gs.CanTake3([3]Color{ColorSolid, ColorRing, ColorDot}) {
		t.Fatal("a color with 0 tokens in the bank cannot be part of a take-3")
	}
}

// --- Take2: the >=4-BEFORE gotcha ---------------------------------------

func TestTake2RequiresAtLeast4Before(t *testing.T) {
	gs := newTestGame()
	gs.Bank[ColorRing] = 4
	if !gs.CanTake2(ColorRing) {
		t.Fatal("exactly 4 tokens before taking should be legal for take-2")
	}
	if !gs.Take2(ColorRing) {
		t.Fatal("take-2 should succeed with bank==4")
	}
	if gs.Bank[ColorRing] != 2 {
		t.Fatalf("bank should now hold 2, got %d", gs.Bank[ColorRing])
	}
	if gs.Players[0].Tokens[ColorRing] != 2 {
		t.Fatalf("player should hold 2, got %d", gs.Players[0].Tokens[ColorRing])
	}
}

// The classic off-by-one: 3 (or fewer) tokens must NOT be enough, even
// though 3-2=1 >= 0 would "work" arithmetically.
func TestTake2RejectsWhenBankHasFewerThan4Before(t *testing.T) {
	gs := newTestGame()
	for _, n := range []int{0, 1, 2, 3} {
		gs.Bank[ColorCross] = n
		if gs.CanTake2(ColorCross) {
			t.Fatalf("bank==%d before taking must NOT be legal for take-2 (needs >=4 before)", n)
		}
		if gs.Take2(ColorCross) {
			t.Fatalf("Take2 must fail outright with bank==%d", n)
		}
	}
}

func TestTake2RejectsExactly2Before(t *testing.T) {
	// The literal off-by-one described in the spec: a naive implementer
	// might check bank >= 2 (enough to physically remove 2 tokens) instead
	// of bank >= 4 (the actual rule). Guard it explicitly.
	gs := newTestGame()
	gs.Bank[ColorStripe] = 2
	if gs.CanTake2(ColorStripe) {
		t.Fatal("bank==2 before taking must be illegal for take-2, despite 2 tokens physically being enough to remove")
	}
}

// --- Reserve -------------------------------------------------------------

func TestReserveTableauGrantsGoldAndRefillsSlot(t *testing.T) {
	gs := newTestGame()
	card := gs.Tableau[0][0]
	if card.Tier == 0 {
		t.Fatal("setup: expected a real card in tableau[0][0]")
	}
	goldBefore := gs.BankGold
	if !gs.ReserveTableau(0, 0) {
		t.Fatal("reserving a face-up card with an empty reserve list should be legal")
	}
	p := gs.Players[0]
	if len(p.Reserved) != 1 || p.Reserved[0] != card {
		t.Fatalf("reserved list should contain the exact card taken, got %+v", p.Reserved)
	}
	if p.Gold != 1 || gs.BankGold != goldBefore-1 {
		t.Fatalf("reserving should grant 1 gold from the bank, got player gold=%d bankGold=%d", p.Gold, gs.BankGold)
	}
	if gs.Tableau[0][0] == card {
		t.Fatal("the tableau slot should have been refilled with a different card")
	}
	if gs.Tableau[0][0].Tier == 0 {
		t.Fatal("the tableau slot should have been refilled (tier-1 deck was far from empty)")
	}
}

func TestReserveNoGoldWhenBankGoldEmpty(t *testing.T) {
	gs := newTestGame()
	gs.BankGold = 0
	if !gs.ReserveTableau(0, 0) {
		t.Fatal("reserve should still succeed with no gold left in the bank")
	}
	if gs.Players[0].Gold != 0 {
		t.Fatal("no gold should be granted when the bank has none left")
	}
}

func TestReserveMaxThreeCap(t *testing.T) {
	gs := newTestGame()
	for i := 0; i < 3; i++ {
		if !gs.ReserveTableau(0, 0) {
			t.Fatalf("reserve #%d (of 3) should be legal", i+1)
		}
		gs.Turn = 0 // keep it player 0's turn for this test's purposes
		gs.Phase = PhasePlaying
	}
	if gs.CanReserveTableau(0, 0) {
		t.Fatal("a 4th reservation must be illegal (cap is 3)")
	}
	if gs.ReserveTableau(1, 0) {
		t.Fatal("ReserveTableau must reject once at the cap")
	}
	if gs.CanReserveBlind(2) {
		t.Fatal("blind reservation must also respect the 3-reserved cap")
	}
}

func TestReserveBlindDrawsFromDeckDirectly(t *testing.T) {
	gs := newTestGame()
	deckBefore := len(gs.Decks[2])
	tableauBefore := gs.Tableau[2]
	if !gs.ReserveBlind(2) {
		t.Fatal("blind reserve of tier 3 should be legal with cards in the deck")
	}
	if len(gs.Decks[2]) != deckBefore-1 {
		t.Fatal("blind reserve should remove exactly one card from the deck")
	}
	if gs.Tableau[2] != tableauBefore {
		t.Fatal("blind reserve must not touch the face-up tableau slots")
	}
	if len(gs.Players[0].Reserved) != 1 {
		t.Fatal("the drawn card should land in the reserved list")
	}
}

func TestReserveRejectsEmptySlotOrDeck(t *testing.T) {
	gs := newTestGame()
	gs.Tableau[0][0] = Card{} // empty slot
	if gs.CanReserveTableau(0, 0) {
		t.Fatal("reserving an empty tableau slot must be illegal")
	}
	gs.Decks[0] = nil
	if gs.CanReserveBlind(0) {
		t.Fatal("blind-reserving from an empty deck must be illegal")
	}
}

// --- Buy: affordability with bonuses + gold wildcard ----------------------

func TestBuyTableauPaysWithTokensAndBonusDiscount(t *testing.T) {
	gs := newTestGame()
	gs.Tableau[0][0] = Card{Tier: 1, Color: ColorDot, Points: 0, Cost: [NumColors]int{2, 1, 0, 0, 0}}
	p := &gs.Players[0]
	p.Bonuses[ColorSolid] = 1 // 1 free Solid from an owned card
	p.Tokens[ColorSolid] = 1  // covers the remaining (2-1)=1 Solid needed
	p.Tokens[ColorRing] = 1

	bankSolidBefore := gs.Bank[ColorSolid]
	if !gs.CanBuyTableau(0, 0) {
		t.Fatal("cost {2 Solid, 1 Ring} with a Solid bonus + exact tokens should be affordable")
	}
	if !gs.BuyTableau(0, 0) {
		t.Fatal("buy should succeed")
	}
	if p.Tokens[ColorSolid] != 0 || p.Tokens[ColorRing] != 0 {
		t.Fatalf("all spent tokens should be gone, got %+v", p.Tokens)
	}
	if gs.Bank[ColorSolid] != bankSolidBefore+1 {
		t.Fatal("the 1 Solid token actually spent should return to the bank")
	}
	if p.Bonuses[ColorDot] != 1 {
		t.Fatal("the bought card's bonus color should now be owned")
	}
	if len(p.Cards) != 1 {
		t.Fatal("the card should be added to owned cards")
	}
}

func TestBuyUsesGoldForShortfall(t *testing.T) {
	gs := newTestGame()
	gs.Tableau[0][0] = Card{Tier: 1, Color: ColorDot, Cost: [NumColors]int{3, 0, 0, 0, 0}}
	p := &gs.Players[0]
	p.Tokens[ColorSolid] = 1
	p.Gold = 2
	if !gs.CanBuyTableau(0, 0) {
		t.Fatal("1 Solid token + 2 gold should cover a cost of 3 Solid")
	}
	gs.BuyTableau(0, 0)
	if p.Gold != 0 {
		t.Fatalf("both gold tokens should have been spent, got %d left", p.Gold)
	}
	if p.Tokens[ColorSolid] != 0 {
		t.Fatal("the 1 Solid token should have been spent")
	}
}

func TestBuyRejectsWhenUnaffordable(t *testing.T) {
	gs := newTestGame()
	gs.Tableau[0][0] = Card{Tier: 1, Color: ColorDot, Cost: [NumColors]int{3, 0, 0, 0, 0}}
	p := &gs.Players[0]
	p.Tokens[ColorSolid] = 1
	p.Gold = 1 // short by 1
	if gs.CanBuyTableau(0, 0) {
		t.Fatal("1 token + 1 gold against a cost of 3 should NOT be affordable")
	}
	tokBefore, goldBefore, cardsBefore := p.Tokens, p.Gold, len(p.Cards)
	if gs.BuyTableau(0, 0) {
		t.Fatal("an unaffordable buy must be rejected")
	}
	if p.Tokens != tokBefore || p.Gold != goldBefore || len(p.Cards) != cardsBefore {
		t.Fatal("a rejected buy must not mutate player state")
	}
}

func TestBuyReserved(t *testing.T) {
	gs := newTestGame()
	card := Card{Tier: 1, Color: ColorDot, Points: 2, Cost: [NumColors]int{1, 0, 0, 0, 0}}
	p := &gs.Players[0]
	p.Reserved = append(p.Reserved, card)
	p.Tokens[ColorSolid] = 1
	if !gs.CanBuyReserved(0) {
		t.Fatal("should be able to afford the reserved card")
	}
	if !gs.BuyReserved(0) {
		t.Fatal("buying the reserved card should succeed")
	}
	if len(p.Reserved) != 0 {
		t.Fatal("reserved card should be removed from the reserved list")
	}
	if p.Prestige != 2 {
		t.Fatalf("prestige should include the card's points, got %d", p.Prestige)
	}
}

// --- Nobles: auto-claim vs multi-qualify active-player choice --------------

func TestNobleAutoClaimsWhenExactlyOneQualifies(t *testing.T) {
	gs := newTestGame()
	gs.Nobles = []Noble{
		{ID: 0, Requirement: [NumColors]int{3, 3, 3, 0, 0}},
	}
	gs.Tableau[0][0] = Card{Tier: 1, Color: ColorCross, Cost: [NumColors]int{0, 0, 0, 0, 0}}
	p := &gs.Players[0]
	p.Bonuses[ColorSolid] = 3
	p.Bonuses[ColorRing] = 3
	p.Bonuses[ColorCross] = 2 // one short; this buy brings it to 3

	if !gs.BuyTableau(0, 0) {
		t.Fatal("free card should be buyable")
	}
	if gs.Phase != PhasePlaying {
		t.Fatalf("with only 1 noble qualifying, it should auto-claim (no PhaseNobleChoice), got phase %v", gs.Phase)
	}
	if len(gs.Nobles) != 0 {
		t.Fatal("the noble should have been claimed and removed from the board")
	}
	if len(gs.Players[0].Nobles) != 1 {
		t.Fatal("player 0 should now own the noble")
	}
	if gs.Players[0].Prestige != NoblePoints {
		t.Fatalf("prestige should include the noble's 3 points, got %d", gs.Players[0].Prestige)
	}
}

// GOTCHA: when 2+ nobles qualify simultaneously, it's the ACTIVE player's
// choice, not automatic/first-match.
func TestNobleMultipleQualifyActivePlayerChooses(t *testing.T) {
	gs := newTestGame()
	gs.Nobles = []Noble{
		{ID: 0, Requirement: [NumColors]int{3, 3, 0, 0, 0}},
		{ID: 1, Requirement: [NumColors]int{3, 0, 3, 0, 0}},
	}
	gs.Tableau[0][0] = Card{Tier: 1, Color: ColorSolid, Cost: [NumColors]int{0, 0, 0, 0, 0}}
	p := &gs.Players[0]
	p.Bonuses[ColorRing] = 3
	p.Bonuses[ColorCross] = 3
	p.Bonuses[ColorSolid] = 2 // the buy below brings this to 3, qualifying BOTH nobles at once

	if !gs.BuyTableau(0, 0) {
		t.Fatal("free card should be buyable")
	}
	if gs.Phase != PhaseNobleChoice {
		t.Fatalf("2 nobles qualifying at once must require an active-player choice, got phase %v", gs.Phase)
	}
	if len(gs.PendingNobles) != 2 {
		t.Fatalf("both nobles should be pending choices, got %v", gs.PendingNobles)
	}
	if gs.Turn != 0 {
		t.Fatal("turn must not have passed yet — the choice belongs to the player who just moved")
	}

	// The active player (still player 0) picks the SECOND pending noble
	// (index 1 in PendingNobles), which is noble ID 1 (not the "first
	// match" ID 0) — proving this isn't an automatic first-match pick.
	wantNobleID := gs.Nobles[gs.PendingNobles[1]].ID
	if !gs.ChooseNoble(1) {
		t.Fatal("choosing the 2nd pending noble should succeed")
	}
	if gs.Phase == PhaseNobleChoice {
		t.Fatal("phase should have advanced past the noble choice")
	}
	if len(gs.Players[0].Nobles) != 1 || gs.Players[0].Nobles[0].ID != wantNobleID {
		t.Fatalf("player should own exactly the CHOSEN noble (ID %d), got %+v", wantNobleID, gs.Players[0].Nobles)
	}
	if len(gs.Nobles) != 1 {
		t.Fatal("exactly one noble should remain on the board (the unchosen one)")
	}
	if gs.Players[0].Prestige != NoblePoints {
		t.Fatalf("prestige should be exactly 3 (one noble), got %d", gs.Players[0].Prestige)
	}
}

func TestChooseNobleRejectedOutsidePendingPhase(t *testing.T) {
	gs := newTestGame()
	if gs.ChooseNoble(0) {
		t.Fatal("ChooseNoble must fail when no choice is pending")
	}
}

// --- Token cap discard -----------------------------------------------------

func TestTokenCapForcesDiscardPhase(t *testing.T) {
	gs := newTestGame()
	p := &gs.Players[0]
	p.Tokens[ColorSolid] = 9
	gs.Bank[ColorRing] = 4 // enough to take2
	if !gs.Take2(ColorRing) {
		t.Fatal("take-2 should be legal")
	}
	if gs.Phase != PhaseDiscard {
		t.Fatalf("holding 11 tokens (>10) should force PhaseDiscard, got %v", gs.Phase)
	}
	if gs.DiscardNeeded != 1 {
		t.Fatalf("should need to discard exactly 1, got %d", gs.DiscardNeeded)
	}
	if gs.Turn != 0 {
		t.Fatal("turn must not pass until discard is resolved")
	}
	if !gs.DiscardColor(ColorSolid) {
		t.Fatal("discarding a held color should succeed")
	}
	if gs.Phase != PhasePlaying {
		t.Fatal("phase should return to playing once discard need reaches 0")
	}
	if gs.Turn != 1 {
		t.Fatal("turn should now have passed")
	}
}

func TestDiscardRejectsColorNotHeld(t *testing.T) {
	gs := newTestGame()
	p := &gs.Players[0]
	p.Tokens[ColorSolid] = 11
	gs.Phase = PhaseDiscard
	gs.DiscardNeeded = 1
	if gs.DiscardColor(ColorDot) {
		t.Fatal("discarding a color with 0 held tokens must fail")
	}
}

func TestDiscardGold(t *testing.T) {
	gs := newTestGame()
	p := &gs.Players[0]
	p.Gold = 1
	gs.Phase = PhaseDiscard
	gs.DiscardNeeded = 1
	goldBefore := gs.BankGold
	if !gs.DiscardGold() {
		t.Fatal("discarding a held gold token should succeed")
	}
	if p.Gold != 0 || gs.BankGold != goldBefore+1 {
		t.Fatal("gold should move from player back to the bank")
	}
	if gs.Phase != PhasePlaying {
		t.Fatal("phase should resolve once discard need reaches 0")
	}
}

// --- End-game trigger + "finish the round" turn parity ---------------------

func TestEndTrigger(t *testing.T) {
	if EndTrigger(14) {
		t.Fatal("14 prestige must not trigger game-end")
	}
	if !EndTrigger(15) {
		t.Fatal("15 prestige must trigger game-end")
	}
	if !EndTrigger(20) {
		t.Fatal("more than 15 prestige must trigger game-end")
	}
}

// GOTCHA: game-end is "finish the round", not "stop immediately at 15
// prestige" — a trigger on player 0's turn must still let player 1 (the
// last player in the fixed 2-player turn order) take one more turn.
func TestGameEndFinishesTheRoundWhenTriggeredByFirstPlayer(t *testing.T) {
	gs := newTestGame()
	gs.Players[0].Prestige = 12
	gs.Tableau[0][0] = Card{Tier: 1, Color: ColorDot, Points: 3, Cost: [NumColors]int{0, 0, 0, 0, 0}}
	if !gs.BuyTableau(0, 0) {
		t.Fatal("free card buy should succeed")
	}
	if gs.Players[0].Prestige != 15 {
		t.Fatalf("player 0 should now have 15 prestige, got %d", gs.Players[0].Prestige)
	}
	if gs.Phase == PhaseDone {
		t.Fatal("the game must NOT end immediately when player 0 (not last in turn order) triggers 15+")
	}
	if !gs.EndTriggered {
		t.Fatal("EndTriggered should now be set")
	}
	if gs.Turn != 1 {
		t.Fatalf("turn should have passed to player 1 so they get their turn this round, got %d", gs.Turn)
	}

	// Player 1 now takes their turn (any legal action); the game must end
	// once IT completes, regardless of player 1's own prestige.
	gs.Bank[ColorRing] = 4
	if !gs.Take2(ColorRing) {
		t.Fatal("player 1's turn should be playable")
	}
	if gs.Phase != PhaseDone {
		t.Fatalf("the game must end once player 1 (last in turn order) finishes their turn this round, got %v", gs.Phase)
	}
	if gs.Winner() != 0 {
		t.Fatalf("player 0 (15 prestige) should win, got winner %d", gs.Winner())
	}
}

// If the LAST player in turn order (player 1) is the one who triggers 15+,
// the game ends immediately after their own turn — no extra turn is owed.
func TestGameEndImmediateWhenTriggeredByLastPlayer(t *testing.T) {
	gs := newTestGame()
	gs.Turn = 1
	gs.Players[1].Prestige = 12
	gs.Tableau[0][0] = Card{Tier: 1, Color: ColorDot, Points: 3, Cost: [NumColors]int{0, 0, 0, 0, 0}}
	if !gs.BuyTableau(0, 0) {
		t.Fatal("free card buy should succeed")
	}
	if gs.Phase != PhaseDone {
		t.Fatalf("player 1 (last in turn order) triggering 15+ should end the game immediately, got %v", gs.Phase)
	}
	if gs.Winner() != 1 {
		t.Fatalf("player 1 should win, got %d", gs.Winner())
	}
}

// --- FinalWinner tiebreak ---------------------------------------------------

func TestFinalWinnerByPrestige(t *testing.T) {
	if w := FinalWinner(PlayerSummary{Prestige: 16}, PlayerSummary{Prestige: 15}); w != 0 {
		t.Fatalf("higher prestige should win, got %d", w)
	}
	if w := FinalWinner(PlayerSummary{Prestige: 10}, PlayerSummary{Prestige: 15}); w != 1 {
		t.Fatalf("higher prestige should win, got %d", w)
	}
}

func TestFinalWinnerTiebreakFewestCards(t *testing.T) {
	a := PlayerSummary{Prestige: 15, CardCount: 8}
	b := PlayerSummary{Prestige: 15, CardCount: 6}
	if w := FinalWinner(a, b); w != 1 {
		t.Fatalf("equal prestige: FEWER cards should win (efficiency tiebreak), got %d", w)
	}
	if w := FinalWinner(b, a); w != 0 {
		t.Fatalf("symmetric check failed, got %d", w)
	}
}

func TestFinalWinnerGenuineTie(t *testing.T) {
	a := PlayerSummary{Prestige: 15, CardCount: 7}
	b := PlayerSummary{Prestige: 15, CardCount: 7}
	if w := FinalWinner(a, b); w != -1 {
		t.Fatalf("identical prestige and card count should be a tie (-1), got %d", w)
	}
}

// --- Noble qualification pure function --------------------------------------

func TestNobleQualifies(t *testing.T) {
	req := [NumColors]int{3, 0, 2, 0, 0}
	if NobleQualifies([NumColors]int{2, 0, 2, 0, 0}, req) {
		t.Fatal("short of the requirement in one color must not qualify")
	}
	if !NobleQualifies([NumColors]int{3, 5, 2, 0, 0}, req) {
		t.Fatal("meeting or exceeding every required color should qualify")
	}
}

func TestQualifyingNoblesMultiple(t *testing.T) {
	nobles := []Noble{
		{Requirement: [NumColors]int{3, 0, 0, 0, 0}},
		{Requirement: [NumColors]int{0, 3, 0, 0, 0}},
		{Requirement: [NumColors]int{0, 0, 0, 0, 4}},
	}
	bonuses := [NumColors]int{3, 3, 0, 0, 0}
	got := QualifyingNobles(bonuses, nobles)
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("expected nobles [0 1] to qualify, got %v", got)
	}
}
