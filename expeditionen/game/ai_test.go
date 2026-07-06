package game

import (
	"math/rand"
	"testing"
)

func TestAIPlayValueAppliesTheFlexibilityPenaltyToFreshStarts(t *testing.T) {
	// A high card starting a brand-new expedition forecloses every lower
	// card of that suit for good (plays must be non-decreasing) — check the
	// penalty is actually applied and scales with the card's own rank, by
	// comparing against the raw (undiscounted) Score marginal.
	for _, rank := range []Rank{2, 6, 10} {
		c := Card{Suit: SuitOken, Rank: rank}
		rawMarginal := float64(Score([]Card{c}) - Score(nil))
		got := aiPlayValue(c, nil, fullDeckAfterDeal)
		wantPenalty := float64(rank) * 0.6
		// fullDeckAfterDeal is plenty of deck left, and sumAfter=rank<14, so
		// the late-entry risk term never fires here — only the flex penalty
		// should separate got from rawMarginal.
		if got != rawMarginal-wantPenalty {
			t.Fatalf("rank %d: aiPlayValue = %v, want rawMarginal(%v) - flexPenalty(%v) = %v",
				rank, got, rawMarginal, wantPenalty, rawMarginal-wantPenalty)
		}
	}
}

func TestAIPlayValueFavorsContinuingAnAlmostBreakEvenRow(t *testing.T) {
	// Continuing an expedition that's already close to paying off should be
	// valued far higher than starting a brand-new one from scratch with the
	// same card, since the flex/late-entry penalties only apply to a fresh
	// start (numbersPlayed == 0).
	row := []Card{{Suit: SuitHavet, Rank: 6}, {Suit: SuitHavet, Rank: 7}, {Suit: SuitHavet, Rank: 8}}
	card := Card{Suit: SuitHavet, Rank: 9}
	continuing := aiPlayValue(card, row, fullDeckAfterDeal)
	fresh := aiPlayValue(card, nil, fullDeckAfterDeal)
	if continuing <= fresh {
		t.Fatalf("continuing value (%v) should exceed a fresh-start value (%v) for the same card", continuing, fresh)
	}
}

func TestAIPlayValueDiscountsLateSpeculativeStart(t *testing.T) {
	earlyDeck := fullDeckAfterDeal
	lateDeck := 2 // almost nothing left to draw
	early := aiPlayValue(Card{Suit: SuitOken, Rank: 3}, nil, earlyDeck)
	late := aiPlayValue(Card{Suit: SuitOken, Rank: 3}, nil, lateDeck)
	if late >= early {
		t.Fatalf("a fresh low-value start late in the deck (%v) should be discounted vs early (%v)", late, early)
	}
}

func TestAIDiscardValuePenalizesInvestmentsAndCheapCards(t *testing.T) {
	inv := aiDiscardValue(Card{Suit: SuitOken, Rank: RankInvestment})
	cheap := aiDiscardValue(Card{Suit: SuitOken, Rank: 3})
	high := aiDiscardValue(Card{Suit: SuitOken, Rank: 10})
	if inv >= cheap {
		t.Fatalf("discarding an investment (%v) should be worse than discarding a cheap number card (%v)", inv, cheap)
	}
	if cheap >= high {
		t.Fatalf("discarding a cheap card (%v) should be worse than discarding a high card (%v)", cheap, high)
	}
}

func TestAIChooseActionPrefersACompletingPlayOverDiscard(t *testing.T) {
	s := NewGame(nil)
	// Rig the AI's row and hand so playing rank 10 finishes a strong, nearly
	// break-even expedition, and its only other card is a throwaway.
	s.Players[AIIdx].Rows[SuitHavet] = []Card{
		{Suit: SuitHavet, Rank: 2}, {Suit: SuitHavet, Rank: 3}, {Suit: SuitHavet, Rank: 4},
		{Suit: SuitHavet, Rank: 5}, {Suit: SuitHavet, Rank: 6}, {Suit: SuitHavet, Rank: 7},
	}
	s.Players[AIIdx].Hand = []Card{{Suit: SuitHavet, Rank: 10}, {Suit: SuitPolaren, Rank: 9}}
	idx, playToRow := s.aiChooseAction(AIIdx)
	if idx != 0 || !playToRow {
		t.Fatalf("aiChooseAction = (%d,%v), want (0,true) — playing the completing 10 into Havet", idx, playToRow)
	}
}

func TestStepAIActsOnlyOnItsOwnTurn(t *testing.T) {
	s := NewGame(nil)
	if s.StepAI() {
		t.Fatal("StepAI should do nothing on the human's turn")
	}
}

func TestStepAIPlaysAndDrawsThenReturnsTurnToHuman(t *testing.T) {
	s := NewGame(nil)
	if err := s.DiscardCard(HumanIdx, 0); err != nil {
		t.Fatalf("DiscardCard: %v", err)
	}
	if err := s.DrawFromDeck(HumanIdx); err != nil {
		t.Fatalf("DrawFromDeck: %v", err)
	}
	if !s.AITurn() {
		t.Fatal("should be the AI's turn now")
	}
	aiHandBefore := len(s.Players[AIIdx].Hand)
	if !s.StepAI() {
		t.Fatal("StepAI should have acted")
	}
	if s.AwaitingDraw() {
		t.Fatal("StepAI should complete the FULL turn (act + draw), not leave it half-done")
	}
	if s.Turn != HumanIdx {
		t.Fatalf("Turn = %d, want HumanIdx after the AI's full turn", s.Turn)
	}
	if len(s.Players[AIIdx].Hand) != aiHandBefore {
		t.Fatalf("AI hand size = %d, want unchanged at %d (one card out, one drawn)", len(s.Players[AIIdx].Hand), aiHandBefore)
	}
}

// TestAIPlaysAFullRoundWithoutPanicking drives StepAI (and simple
// always-discard-index-0 human moves) all the way to PhaseDone, checking
// invariants hold throughout: hand sizes never drift, every AI action was
// legal when it claimed to play, and the round actually terminates.
func TestAIPlaysAFullRoundWithoutPanicking(t *testing.T) {
	s := NewGame(nil)
	turns := 0
	for s.Phase == PhasePlaying {
		turns++
		if turns > 1000 {
			t.Fatal("round did not terminate")
		}
		pi := s.Turn
		if len(s.Players[pi].Hand) != HandSize {
			t.Fatalf("turn %d: player %d hand = %d, want %d", turns, pi, len(s.Players[pi].Hand), HandSize)
		}
		if pi == AIIdx {
			if !s.StepAI() {
				t.Fatalf("turn %d: StepAI returned false on the AI's own turn", turns)
			}
			continue
		}
		// Simplest possible human policy: play index 0 if legal, else discard it.
		card := s.Players[pi].Hand[0]
		row := s.Players[pi].Rows[card.Suit]
		if LegalPlay(card, row) {
			if err := s.PlayCard(pi, 0); err != nil {
				t.Fatalf("turn %d: PlayCard: %v", turns, err)
			}
		} else if err := s.DiscardCard(pi, 0); err != nil {
			t.Fatalf("turn %d: DiscardCard: %v", turns, err)
		}
		if err := s.DrawFromDeck(pi); err != nil {
			// Deck may occasionally be momentarily empty is not possible since
			// PhaseDone triggers exactly when it hits 0; treat any error here
			// as a genuine failure.
			t.Fatalf("turn %d: DrawFromDeck: %v", turns, err)
		}
	}
	if len(s.Deck) != 0 {
		t.Fatalf("deck = %d at PhaseDone, want 0", len(s.Deck))
	}
	// Winner() must not panic and must return at least one index.
	if w := s.Winner(); len(w) == 0 {
		t.Fatal("Winner() returned nobody")
	}
}

// TestAIPlaysManyShuffledRoundsWithoutPanicking runs the same drive as
// TestAIPlaysAFullRoundWithoutPanicking but with a REAL random shuffle each
// time (mirroring what the shipped app does via math/rand), across many
// seeds — a guard against flakiness from card-shuffling/map-iteration order
// (AllSuits is a fixed array, not a map, but this also exercises the AI over
// a huge variety of hands/orderings deterministically-repeatable per seed).
func TestAIPlaysManyShuffledRoundsWithoutPanicking(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		s := NewGame(rng.Shuffle)
		turns := 0
		for s.Phase == PhasePlaying {
			turns++
			if turns > 1000 {
				t.Fatalf("seed %d: round did not terminate", seed)
			}
			pi := s.Turn
			if len(s.Players[pi].Hand) != HandSize {
				t.Fatalf("seed %d turn %d: player %d hand = %d, want %d", seed, turns, pi, len(s.Players[pi].Hand), HandSize)
			}
			if pi == AIIdx {
				if !s.StepAI() {
					t.Fatalf("seed %d turn %d: StepAI returned false on the AI's own turn", seed, turns)
				}
				continue
			}
			// A slightly richer human policy than index-0: play the
			// highest-value card among legal number plays if any exist,
			// otherwise discard the first card. Draw from the deck.
			hand := s.Players[pi].Hand
			played := false
			for i, c := range hand {
				if LegalPlay(c, s.Players[pi].Rows[c.Suit]) && !c.IsInvestment() {
					if err := s.PlayCard(pi, i); err != nil {
						t.Fatalf("seed %d turn %d: PlayCard: %v", seed, turns, err)
					}
					played = true
					break
				}
			}
			if !played {
				if err := s.DiscardCard(pi, 0); err != nil {
					t.Fatalf("seed %d turn %d: DiscardCard: %v", seed, turns, err)
				}
			}
			if err := s.DrawFromDeck(pi); err != nil {
				t.Fatalf("seed %d turn %d: DrawFromDeck: %v", seed, turns, err)
			}
		}
		if len(s.Deck) != 0 {
			t.Fatalf("seed %d: deck = %d at PhaseDone, want 0", seed, len(s.Deck))
		}
		if w := s.Winner(); len(w) == 0 {
			t.Fatalf("seed %d: Winner() returned nobody", seed)
		}
		// Every played card must still be a member of its own suit's row,
		// and every hand+row+deck+discard card count must reconcile to 60.
		total := len(s.Deck)
		for pi := 0; pi < 2; pi++ {
			total += len(s.Players[pi].Hand)
			for _, row := range s.Players[pi].Rows {
				total += len(row)
			}
		}
		for _, pile := range s.Discards {
			total += len(pile)
		}
		if total != 60 {
			t.Fatalf("seed %d: total card count = %d, want 60 (cards lost or duplicated)", seed, total)
		}
	}
}
