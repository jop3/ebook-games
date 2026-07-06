package game

import "testing"

func TestNewDeckHas60Cards(t *testing.T) {
	d := NewDeck()
	if len(d) != 60 {
		t.Fatalf("len(NewDeck()) = %d, want 60", len(d))
	}
}

func TestNewDeckComposition(t *testing.T) {
	d := NewDeck()
	counts := map[Suit]map[Rank]int{}
	for _, c := range d {
		if counts[c.Suit] == nil {
			counts[c.Suit] = map[Rank]int{}
		}
		counts[c.Suit][c.Rank]++
	}
	if len(counts) != NumSuits {
		t.Fatalf("deck spans %d suits, want %d", len(counts), NumSuits)
	}
	for _, s := range AllSuits {
		if got := counts[s][RankInvestment]; got != 3 {
			t.Errorf("suit %v: %d investment cards, want 3", s, got)
		}
		for r := Rank(minNumberRank); r <= maxNumberRank; r++ {
			if got := counts[s][r]; got != 1 {
				t.Errorf("suit %v rank %d: %d cards, want 1", s, r, got)
			}
		}
		total := 0
		for _, n := range counts[s] {
			total += n
		}
		if total != 12 {
			t.Errorf("suit %v: %d total cards, want 12", s, total)
		}
	}
}

func TestShuffleDeckNilIsNoOp(t *testing.T) {
	d := NewDeck()
	before := append([]Card(nil), d...)
	shuffleDeck(d, nil)
	for i := range d {
		if d[i] != before[i] {
			t.Fatalf("shuffleDeck(nil) mutated the deck at index %d", i)
		}
	}
}

func TestCountInvestmentsAndSumNumbers(t *testing.T) {
	row := []Card{
		{Suit: SuitOken, Rank: RankInvestment},
		{Suit: SuitOken, Rank: RankInvestment},
		{Suit: SuitOken, Rank: 4},
		{Suit: SuitOken, Rank: 7},
	}
	if got := CountInvestments(row); got != 2 {
		t.Fatalf("CountInvestments = %d, want 2", got)
	}
	if got := SumNumbers(row); got != 11 {
		t.Fatalf("SumNumbers = %d, want 11", got)
	}
}
