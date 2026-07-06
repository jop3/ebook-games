package game

// This file implements the two rules everything else hinges on: which plays
// are legal into an expedition row, and how a finished row is scored. Both
// are pure functions over a []Card row, independently unit-tested against
// hand-worked examples — the same discipline this repo applies to its other
// scoring formulas (see sushi/game/score.go).

// LegalPlay reports whether card may be added to the end of row, per the
// two ordering rules:
//   - All of a suit's investment cards must be played before any number
//     card in that suit — equivalently, once any number card has been
//     played in a suit, no further investment cards may be added to it.
//   - Number cards must be played in non-decreasing order versus the last
//     number card already played in that suit (equal ranks are legal; the
//     deck only has one card per rank, but the rule itself is "≥", not ">").
//
// row is assumed to already be a legally-built sequence (empty, or some
// investment cards followed by non-decreasing number cards) — LegalPlay only
// checks whether appending card keeps it that way.
func LegalPlay(card Card, row []Card) bool {
	if len(row) == 0 {
		return true // any card may start a fresh expedition
	}
	last := row[len(row)-1]
	if card.IsInvestment() {
		// An investment can only follow another investment (or start the
		// row) — the instant a number card lands, the investment window for
		// that suit is closed for good.
		return last.IsInvestment()
	}
	if last.IsInvestment() {
		return true // first number card in the suit — no prior number to compare against
	}
	return card.Rank >= last.Rank
}

// breakEven is the number sum an expedition needs just to score 0 before any
// investment multiplier or the 8-card bonus is applied.
const breakEven = 20

// eightCardBonus is the flat bonus awarded to an expedition with 8 or more
// cards played in it, applied AFTER the investment multiplier.
const eightCardBonus = 20

// eightCardThreshold is the card count (investments + numbers together) that
// triggers eightCardBonus.
const eightCardThreshold = 8

// Score computes one expedition's final score from its played row (in play
// order; discards never count). An untouched (empty) row scores 0.
// Otherwise: (sum of number cards - 20) * (1 + investment cards played),
// then +20 if 8 or more cards were played in this expedition — the bonus is
// applied AFTER the multiplier, not before.
func Score(row []Card) int {
	if len(row) == 0 {
		return 0
	}
	inv := CountInvestments(row)
	sum := SumNumbers(row)
	score := (sum - breakEven) * (1 + inv)
	if len(row) >= eightCardThreshold {
		score += eightCardBonus
	}
	return score
}

// TotalScore sums Score across every suit's row.
func TotalScore(rows [NumSuits][]Card) int {
	total := 0
	for _, r := range rows {
		total += Score(r)
	}
	return total
}
