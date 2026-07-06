package game

import "testing"

func TestNewGameDealsHandsAndLeavesTheRest(t *testing.T) {
	s := NewGame(nil)
	if len(s.Players[HumanIdx].Hand) != HandSize {
		t.Fatalf("human hand = %d, want %d", len(s.Players[HumanIdx].Hand), HandSize)
	}
	if len(s.Players[AIIdx].Hand) != HandSize {
		t.Fatalf("AI hand = %d, want %d", len(s.Players[AIIdx].Hand), HandSize)
	}
	if len(s.Deck) != 60-2*HandSize {
		t.Fatalf("deck = %d, want %d", len(s.Deck), 60-2*HandSize)
	}
	if s.Turn != HumanIdx {
		t.Fatalf("Turn = %d, want HumanIdx", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", s.Phase)
	}
	if s.AwaitingDraw() {
		t.Fatal("a fresh game should not already be awaiting a draw")
	}
}

func TestPlayCardMovesCardFromHandToRow(t *testing.T) {
	s := NewGame(nil)
	card := s.Players[HumanIdx].Hand[0]
	before := len(s.Players[HumanIdx].Hand)
	if err := s.PlayCard(HumanIdx, 0); err != nil {
		t.Fatalf("PlayCard: %v", err)
	}
	if len(s.Players[HumanIdx].Hand) != before-1 {
		t.Fatalf("hand length = %d, want %d", len(s.Players[HumanIdx].Hand), before-1)
	}
	row := s.Players[HumanIdx].Rows[card.Suit]
	if len(row) != 1 || row[0] != card {
		t.Fatalf("row = %v, want [%v]", row, card)
	}
	if !s.AwaitingDraw() {
		t.Fatal("after a play, the turn should be awaiting a draw")
	}
}

func TestPlayCardRejectsIllegalPlay(t *testing.T) {
	s := NewGame(nil)
	p := &s.Players[HumanIdx]
	p.Hand = []Card{{Suit: SuitOken, Rank: 5}, {Suit: SuitOken, Rank: 3}}
	if err := s.PlayCard(HumanIdx, 0); err != nil {
		t.Fatalf("first play (starts the row at 5): %v", err)
	}
	if err := s.DrawFromDeck(HumanIdx); err != nil {
		t.Fatalf("draw: %v", err)
	}
	// AI's turn now; force it back to human to isolate the illegal-play check.
	s.Turn = HumanIdx
	s.awaitingDraw = false
	p.Hand = append(p.Hand, Card{Suit: SuitOken, Rank: 3})
	idx := len(p.Hand) - 1
	if err := s.PlayCard(HumanIdx, idx); err != errIllegalPlay {
		t.Fatalf("PlayCard with a lower rank than the row's last card: err = %v, want errIllegalPlay", err)
	}
}

func TestDiscardCardIsAlwaysLegal(t *testing.T) {
	s := NewGame(nil)
	card := s.Players[HumanIdx].Hand[0]
	if err := s.DiscardCard(HumanIdx, 0); err != nil {
		t.Fatalf("DiscardCard: %v", err)
	}
	pile := s.Discards[card.Suit]
	if len(pile) != 1 || pile[0] != card {
		t.Fatalf("discard pile = %v, want [%v]", pile, card)
	}
	if !s.AwaitingDraw() {
		t.Fatal("after a discard, the turn should be awaiting a draw")
	}
}

func TestMustPlayOrDiscardBeforeActingAgain(t *testing.T) {
	s := NewGame(nil)
	if err := s.PlayCard(HumanIdx, 0); err != nil {
		t.Fatalf("PlayCard: %v", err)
	}
	if err := s.PlayCard(HumanIdx, 0); err != errMustDrawFirst {
		t.Fatalf("acting again before drawing: err = %v, want errMustDrawFirst", err)
	}
}

func TestMustDrawBeforeTheTurnPasses(t *testing.T) {
	s := NewGame(nil)
	if err := s.DrawFromDeck(HumanIdx); err != errMustPlayFirst {
		t.Fatalf("drawing before playing/discarding: err = %v, want errMustPlayFirst", err)
	}
}

func TestDrawFromDeckPassesTheTurn(t *testing.T) {
	s := NewGame(nil)
	if err := s.DiscardCard(HumanIdx, 0); err != nil {
		t.Fatalf("DiscardCard: %v", err)
	}
	handBefore := len(s.Players[HumanIdx].Hand)
	if err := s.DrawFromDeck(HumanIdx); err != nil {
		t.Fatalf("DrawFromDeck: %v", err)
	}
	if len(s.Players[HumanIdx].Hand) != handBefore+1 {
		t.Fatalf("hand length = %d, want %d", len(s.Players[HumanIdx].Hand), handBefore+1)
	}
	if s.Turn != AIIdx {
		t.Fatalf("Turn = %d, want AIIdx after the human's draw", s.Turn)
	}
	if s.AwaitingDraw() {
		t.Fatal("should not be awaiting a draw right after drawing")
	}
}

func TestDrawFromDiscardTakesTheVisibleTopCard(t *testing.T) {
	s := NewGame(nil)
	// Set up a known discard pile for SuitHavet.
	s.Discards[SuitHavet] = []Card{{Suit: SuitHavet, Rank: 4}, {Suit: SuitHavet, Rank: 6}}
	if err := s.DiscardCard(HumanIdx, 0); err != nil {
		t.Fatalf("DiscardCard: %v", err)
	}
	handBefore := len(s.Players[HumanIdx].Hand)
	if err := s.DrawFromDiscard(HumanIdx, SuitHavet); err != nil {
		t.Fatalf("DrawFromDiscard: %v", err)
	}
	if len(s.Players[HumanIdx].Hand) != handBefore+1 {
		t.Fatalf("hand length = %d, want %d", len(s.Players[HumanIdx].Hand), handBefore+1)
	}
	got := s.Players[HumanIdx].Hand[len(s.Players[HumanIdx].Hand)-1]
	if got != (Card{Suit: SuitHavet, Rank: 6}) {
		t.Fatalf("drew %v, want the pile's TOP card {Havet,6}", got)
	}
	if len(s.Discards[SuitHavet]) != 1 {
		t.Fatalf("discard pile left with %d cards, want 1", len(s.Discards[SuitHavet]))
	}
}

func TestDrawFromEmptyDiscardIsRejected(t *testing.T) {
	s := NewGame(nil)
	if err := s.DiscardCard(HumanIdx, 0); err != nil {
		t.Fatalf("DiscardCard: %v", err)
	}
	if err := s.DrawFromDiscard(HumanIdx, SuitPolaren); err != errEmptyDiscard {
		t.Fatalf("drawing from an empty pile: err = %v, want errEmptyDiscard", err)
	}
}

func TestWrongPlayerCannotAct(t *testing.T) {
	s := NewGame(nil)
	if err := s.PlayCard(AIIdx, 0); err != errNotYourTurn {
		t.Fatalf("AI acting on the human's turn: err = %v, want errNotYourTurn", err)
	}
}

func TestDeckExhaustionEndsTheRound(t *testing.T) {
	s := NewGame(nil)
	// Drain the deck down to exactly 1 card left, alternating players.
	for len(s.Deck) > 1 {
		pi := s.Turn
		if err := s.DiscardCard(pi, 0); err != nil {
			t.Fatalf("DiscardCard: %v", err)
		}
		if err := s.DrawFromDeck(pi); err != nil {
			t.Fatalf("DrawFromDeck: %v", err)
		}
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want still PhasePlaying with 1 card left", s.Phase)
	}
	pi := s.Turn
	if err := s.DiscardCard(pi, 0); err != nil {
		t.Fatalf("DiscardCard: %v", err)
	}
	if err := s.DrawFromDeck(pi); err != nil {
		t.Fatalf("final DrawFromDeck: %v", err)
	}
	if len(s.Deck) != 0 {
		t.Fatalf("deck = %d, want 0", len(s.Deck))
	}
	if s.Phase != PhaseDone {
		t.Fatalf("Phase = %v, want PhaseDone once the deck is drawn dry", s.Phase)
	}
	// No further actions should be accepted once the round is over.
	if err := s.DiscardCard(HumanIdx, 0); err != errGameOver {
		t.Fatalf("acting after PhaseDone: err = %v, want errGameOver", err)
	}
}

func TestWinnerComparesTotalScores(t *testing.T) {
	s := NewGame(nil)
	s.Players[HumanIdx].Rows[SuitOken] = []Card{{Suit: SuitOken, Rank: 10}, {Suit: SuitOken, Rank: 10}, {Suit: SuitOken, Rank: 10}}
	s.Players[AIIdx].Rows[SuitHavet] = []Card{{Suit: SuitHavet, Rank: 2}}
	w := s.Winner()
	if len(w) != 1 || w[0] != HumanIdx {
		t.Fatalf("Winner() = %v, want [HumanIdx]", w)
	}
}

func TestWinnerTieReturnsBoth(t *testing.T) {
	s := NewGame(nil)
	// Both players untouched (0-0) is already a tie.
	w := s.Winner()
	if len(w) != 2 {
		t.Fatalf("Winner() = %v, want both players tied at 0", w)
	}
}
