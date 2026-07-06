package game

import "testing"

// --- LegalPlay: investments-before-numbers ordering, both directions ------

func TestLegalPlayEmptyRowAcceptsAnything(t *testing.T) {
	if !LegalPlay(Card{Suit: SuitOken, Rank: RankInvestment}, nil) {
		t.Fatal("an investment card should be able to start a fresh expedition")
	}
	if !LegalPlay(Card{Suit: SuitOken, Rank: 5}, nil) {
		t.Fatal("a number card should be able to start a fresh expedition")
	}
}

func TestLegalPlayInvestmentsCanStackOnInvestments(t *testing.T) {
	row := []Card{{Suit: SuitHavet, Rank: RankInvestment}}
	if !LegalPlay(Card{Suit: SuitHavet, Rank: RankInvestment}, row) {
		t.Fatal("a 2nd investment should be legal to add while only investments have been played")
	}
}

func TestLegalPlayNumberAfterInvestmentIsFine(t *testing.T) {
	row := []Card{{Suit: SuitHavet, Rank: RankInvestment}, {Suit: SuitHavet, Rank: RankInvestment}}
	if !LegalPlay(Card{Suit: SuitHavet, Rank: 3}, row) {
		t.Fatal("the first number card after investments should be legal at any rank")
	}
}

// Direction 1 (the spec's literal wording): once ANY number card has been
// played in a suit, no further investment cards may be added to it.
func TestLegalPlayNumberCardBlocksLaterInvestments(t *testing.T) {
	row := []Card{
		{Suit: SuitVulkanen, Rank: RankInvestment},
		{Suit: SuitVulkanen, Rank: 4},
	}
	if LegalPlay(Card{Suit: SuitVulkanen, Rank: RankInvestment}, row) {
		t.Fatal("an investment must be rejected once a number card has been played in that suit")
	}
	// Also true when the suit started directly with a number card (no
	// investments played at all yet).
	row2 := []Card{{Suit: SuitVulkanen, Rank: 3}}
	if LegalPlay(Card{Suit: SuitVulkanen, Rank: RankInvestment}, row2) {
		t.Fatal("an investment must be rejected once any number card has been played, even with zero prior investments")
	}
}

// Direction 2: investments must come before numbers — but this is really
// the same rule restated; check it holds regardless of how many investments
// preceded the (single, blocking) number card.
func TestLegalPlayInvestmentsMustPrecedeNumbers(t *testing.T) {
	for invCount := 0; invCount <= 3; invCount++ {
		var row []Card
		for i := 0; i < invCount; i++ {
			row = append(row, Card{Suit: SuitPolaren, Rank: RankInvestment})
		}
		row = append(row, Card{Suit: SuitPolaren, Rank: 6})
		if LegalPlay(Card{Suit: SuitPolaren, Rank: RankInvestment}, row) {
			t.Fatalf("invCount=%d: investment must be illegal once a number card is on top", invCount)
		}
	}
}

func TestLegalPlayNumbersMustBeNonDecreasing(t *testing.T) {
	row := []Card{{Suit: SuitDjungeln, Rank: 5}}
	if LegalPlay(Card{Suit: SuitDjungeln, Rank: 4}, row) {
		t.Fatal("a lower-ranked number card must be illegal")
	}
	if !LegalPlay(Card{Suit: SuitDjungeln, Rank: 5}, row) {
		t.Fatal("an equal-ranked number card must be legal (rule is >=, not >)")
	}
	if !LegalPlay(Card{Suit: SuitDjungeln, Rank: 6}, row) {
		t.Fatal("a higher-ranked number card must be legal")
	}
}

// --- Score: the exact formula, including operator precedence -------------

// The spec's own hand-worked example: sum=25, 1 investment, 9 cards played
// -> (25-20)*(1+1) = 10, +20 bonus (>=8 cards) = 30.
func TestScoreSpecWorkedExample(t *testing.T) {
	row := []Card{
		{Suit: SuitOken, Rank: RankInvestment},
		{Suit: SuitOken, Rank: 2}, {Suit: SuitOken, Rank: 2}, {Suit: SuitOken, Rank: 3},
		{Suit: SuitOken, Rank: 3}, {Suit: SuitOken, Rank: 3}, {Suit: SuitOken, Rank: 4},
		{Suit: SuitOken, Rank: 4}, {Suit: SuitOken, Rank: 4},
	}
	if got := SumNumbers(row); got != 25 {
		t.Fatalf("test setup bug: sum = %d, want 25", got)
	}
	if got := CountInvestments(row); got != 1 {
		t.Fatalf("test setup bug: investments = %d, want 1", got)
	}
	if len(row) != 9 {
		t.Fatalf("test setup bug: len(row) = %d, want 9", len(row))
	}
	if got := Score(row); got != 30 {
		t.Fatalf("Score = %d, want 30", got)
	}
}

// A second hand-worked example with no investments and fewer than 8 cards,
// to check the plain (multiplier=1, no bonus) path in isolation:
// sum=22, 0 investments, 3 cards -> (22-20)*1 = 2, no bonus.
func TestScoreNoInvestmentNoBonus(t *testing.T) {
	row := []Card{
		{Suit: SuitHavet, Rank: 6}, {Suit: SuitHavet, Rank: 7}, {Suit: SuitHavet, Rank: 9},
	}
	if got := SumNumbers(row); got != 22 {
		t.Fatalf("test setup bug: sum = %d, want 22", got)
	}
	if got := Score(row); got != 2 {
		t.Fatalf("Score = %d, want 2", got)
	}
}

// A third worked example below breakeven with a multiplier, to check the
// multiplier correctly amplifies a LOSS too: sum=15, 2 investments ->
// (15-20)*(1+2) = -15.
func TestScoreMultiplierAmplifiesALoss(t *testing.T) {
	row := []Card{
		{Suit: SuitVulkanen, Rank: RankInvestment}, {Suit: SuitVulkanen, Rank: RankInvestment},
		{Suit: SuitVulkanen, Rank: 7}, {Suit: SuitVulkanen, Rank: 8},
	}
	if got := SumNumbers(row); got != 15 {
		t.Fatalf("test setup bug: sum = %d, want 15", got)
	}
	if got := Score(row); got != -15 {
		t.Fatalf("Score = %d, want -15", got)
	}
}

func TestScoreUntouchedExpeditionIsZero(t *testing.T) {
	if got := Score(nil); got != 0 {
		t.Fatalf("Score(nil) = %d, want 0", got)
	}
	if got := Score([]Card{}); got != 0 {
		t.Fatalf("Score([]Card{}) = %d, want 0", got)
	}
}

// The +20 bonus must apply AFTER the multiplier, not before: verify by
// comparing an 8-card row against the same row with the 8th card removed.
func TestScoreEightCardBonusAppliesAfterMultiplier(t *testing.T) {
	base := []Card{
		{Suit: SuitPolaren, Rank: RankInvestment},
		{Suit: SuitPolaren, Rank: 2}, {Suit: SuitPolaren, Rank: 3}, {Suit: SuitPolaren, Rank: 4},
		{Suit: SuitPolaren, Rank: 5}, {Suit: SuitPolaren, Rank: 6}, {Suit: SuitPolaren, Rank: 7},
	}
	// 7 cards: no bonus yet. sum=27, inv=1 -> (27-20)*2 = 14.
	if got := Score(base); got != 14 {
		t.Fatalf("7-card Score = %d, want 14", got)
	}
	eighth := append(append([]Card(nil), base...), Card{Suit: SuitPolaren, Rank: 8})
	// 8 cards: sum=35, inv=1 -> (35-20)*2 = 30, +20 bonus = 50 (NOT (35-20+20)*2=70).
	if got := Score(eighth); got != 50 {
		t.Fatalf("8-card Score = %d, want 50 (bonus applied after the multiplier)", got)
	}
}

func TestTotalScoreSumsAllFiveSuits(t *testing.T) {
	var rows [NumSuits][]Card
	rows[SuitOken] = []Card{{Suit: SuitOken, Rank: 10}, {Suit: SuitOken, Rank: 10}} // sum 20 -> 0
	rows[SuitHavet] = []Card{{Suit: SuitHavet, Rank: 2}}                            // sum 2 -> -18
	// the rest stay untouched (nil) -> 0 each
	if got := TotalScore(rows); got != -18 {
		t.Fatalf("TotalScore = %d, want -18", got)
	}
}
