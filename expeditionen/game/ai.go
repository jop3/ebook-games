package game

// AI heuristic: no minimax, no lookahead tree — the opponent's hand is
// hidden information, so (mirroring sushi/game/ai.go's philosophy) the AI
// values each candidate action by a greedy expected-marginal-score estimate
// computed only from public information (its own hand/rows, the visible
// discard piles, and how many cards remain in the draw pile) plus a couple
// of well-known Lost-Cities strategy heuristics:
//
//   - An expedition needs a number-card sum of at least 20 just to break
//     even (more once investment cards are in it, since they multiply the
//     shortfall too) — so starting, or investing in, a suit late in the
//     deck with little hope of reaching 20 is bad.
//   - Playing a HIGH number card to start (or extend) a nearly-empty
//     expedition forecloses ever using a lower card of that suit later
//     (plays must be non-decreasing), so it is discounted versus playing a
//     low card early and saving flexibility for what's drawn afterward.
//   - Discarding hands a card to BOTH players (either can draw the top of a
//     pile) — cheap number cards and investment cards are the most
//     dangerous to give away for free, so they're valued lower to discard.

// fullDeckAfterDeal is how many cards remain in the draw pile right after
// the opening deal (60 total - 2*HandSize) — used to scale late-game
// discounts on a 0..1 fraction rather than a hardcoded card count.
const fullDeckAfterDeal = 60 - 2*HandSize

// deckFactor scales ~1.0 (plenty of the deck left) down to a floor of 0.2
// (deck nearly exhausted) — used to discount speculative plays (investments,
// fresh expeditions) that need time to pay off.
func deckFactor(cardsLeftInDeck int) float64 {
	f := float64(cardsLeftInDeck) / float64(fullDeckAfterDeal)
	if f < 0.2 {
		f = 0.2
	}
	if f > 1 {
		f = 1
	}
	return f
}

// aiPlayValue estimates the value of playing card c onto row (row is this
// player's CURRENT row for c.Suit; caller has already checked LegalPlay).
func aiPlayValue(c Card, row []Card, cardsLeftInDeck int) float64 {
	if c.IsInvestment() {
		invCount := CountInvestments(row)
		// Each investment already in the row raises both the multiplier and
		// the risk of a suit that never gets going (a x3 multiplier on a
		// deficit is much worse than a x1 one) — value further investments
		// less as they stack up, and discount the whole thing late in the
		// deck when there's little time left to cash the multiplier in.
		v := 4.0 - float64(invCount)*1.2
		return v * deckFactor(cardsLeftInDeck)
	}

	before := Score(row)
	after := Score(appendCard(row, c))
	marginal := float64(after - before)

	numbersPlayed := len(row) - CountInvestments(row)
	if numbersPlayed == 0 {
		// This would be the first number card in the suit: a high starting
		// rank forecloses ever playing any lower card of this suit later.
		marginal -= float64(c.Rank) * 0.6
	}

	// Late-entry risk: starting a suit from nothing, late in the deck, while
	// still far short of breakeven, rarely has time to pay off.
	sumAfter := SumNumbers(row) + int(c.Rank)
	if numbersPlayed == 0 && cardsLeftInDeck < fullDeckAfterDeal/2 && sumAfter < 14 {
		marginal -= 6
	}
	return marginal
}

// aiDiscardValue estimates the (usually negative) cost of discarding c
// face-up, where either player may later draw it straight back off the top.
// Cheap number cards and investment cards are the most dangerous to give
// away for free; high number cards are the safest (least useful as a cheap
// gift, since whoever takes it forecloses their own low cards in that suit).
func aiDiscardValue(c Card) float64 {
	v := 0.5
	switch {
	case c.IsInvestment():
		v -= 2.5
	case c.Rank <= 4:
		v -= 1.5
	case c.Rank >= 9:
		v += 0.5
	}
	return v
}

// appendCard returns a fresh slice with c appended after row, without
// mutating row's backing array (row is live game state).
func appendCard(row []Card, c Card) []Card {
	out := make([]Card, len(row), len(row)+1)
	copy(out, row)
	return append(out, c)
}

// aiChooseAction picks the AI's best play-or-discard for its current hand:
// among every (card, legal-play-or-discard) option, the one with the
// highest heuristic value. Discarding is always a legal fallback for every
// card, so this always returns a valid choice given a non-empty hand.
func (s *State) aiChooseAction(pi int) (handIdx int, playToRow bool) {
	hand := s.Players[pi].Hand
	var bestVal float64 = negInf
	for i, c := range hand {
		row := s.Players[pi].Rows[c.Suit]
		if LegalPlay(c, row) {
			if v := aiPlayValue(c, row, len(s.Deck)); v > bestVal {
				bestVal, handIdx, playToRow = v, i, true
			}
		}
		if v := aiDiscardValue(c); v > bestVal {
			bestVal, handIdx, playToRow = v, i, false
		}
	}
	return handIdx, playToRow
}

// deckDrawBaseline is the AI's rough expected value for a random, unseen
// card drawn from the deck — used as the bar a KNOWN discard-pile top card
// must clear to be worth taking instead.
const deckDrawBaseline = 3.0

// aiChooseDraw picks the AI's draw source: the deck, or the most useful
// visible discard-pile top card if one clears the deck's baseline value.
func (s *State) aiChooseDraw(pi int) (fromDeck bool, suit Suit) {
	fromDeck = true
	best := deckDrawBaseline
	for _, sIdx := range AllSuits {
		pile := s.Discards[sIdx]
		if len(pile) == 0 {
			continue
		}
		top := pile[len(pile)-1]
		row := s.Players[pi].Rows[top.Suit]
		var v float64
		if LegalPlay(top, row) {
			// A known, guaranteed-useful card beats a random unknown one by
			// a small certainty bonus on top of its play value.
			v = aiPlayValue(top, row, len(s.Deck)) + 1.0
		} else {
			v = -5.0 // dead weight: can't currently be played into that suit
		}
		if v > best {
			best, fromDeck, suit = v, false, sIdx
		}
	}
	return fromDeck, suit
}

const negInf = -1 << 30

// StepAI performs one full AI turn (choose + play/discard, then choose +
// draw) in a single call, mirroring hasami/sushi's "compute the AI's move
// only after the human's move has been drawn" deferred-AI pattern: the UI
// calls this from Draw() after painting the human's move, so the AI's own
// reply lands on the next frame. Returns false if it is not currently the
// AI's turn (or the round is already over).
func (s *State) StepAI() bool {
	if s.Phase != PhasePlaying || s.Turn != AIIdx {
		return false
	}
	if !s.awaitingDraw {
		idx, playToRow := s.aiChooseAction(AIIdx)
		if playToRow {
			_ = s.PlayCard(AIIdx, idx)
		} else {
			_ = s.DiscardCard(AIIdx, idx)
		}
	}
	if s.Phase != PhasePlaying {
		return true // the play/discard step can't end the round, but guard anyway
	}
	fromDeck, suit := s.aiChooseDraw(AIIdx)
	if fromDeck {
		_ = s.DrawFromDeck(AIIdx)
	} else {
		_ = s.DrawFromDiscard(AIIdx, suit)
	}
	return true
}
