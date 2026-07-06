// Package game implements the rules, scoring, turn engine, and AI for
// Expeditionen — a reimplementation of the Lost Cities card-and-hand-
// management formula under a neutral working title. It has no dependency on
// the ink SDK, so it builds and tests natively (no cgo, no device emulator)
// with `go test ./game/`.
//
// Baserat på Lost Cities (Kosmos / Reiner Knizia).
package game

// Suit identifies one of the 5 expeditions. Each carries its own row of
// played cards and its own discard pile.
type Suit int

const (
	SuitOken     Suit = iota // Öknen — the Desert expedition
	SuitDjungeln             // Djungeln — the Jungle expedition
	SuitHavet                // Havet — the Sea expedition
	SuitVulkanen             // Vulkanen — the Volcano expedition
	SuitPolaren              // Polaren — the Polar expedition
)

// NumSuits is the number of expeditions (and discard piles) in the game.
const NumSuits = 5

// AllSuits lists every suit in a fixed, stable display order.
var AllSuits = [NumSuits]Suit{SuitOken, SuitDjungeln, SuitHavet, SuitVulkanen, SuitPolaren}

func (s Suit) String() string {
	switch s {
	case SuitOken:
		return "Öknen"
	case SuitDjungeln:
		return "Djungeln"
	case SuitHavet:
		return "Havet"
	case SuitVulkanen:
		return "Vulkanen"
	case SuitPolaren:
		return "Polaren"
	default:
		return "?"
	}
}

// Abbrev is a short (1-letter) label used in compact summaries.
func (s Suit) Abbrev() string {
	switch s {
	case SuitOken:
		return "Ö"
	case SuitDjungeln:
		return "D"
	case SuitHavet:
		return "H"
	case SuitVulkanen:
		return "V"
	case SuitPolaren:
		return "P"
	default:
		return "?"
	}
}

// Rank is a card's face value. RankInvestment (0) marks one of the 3
// wager/investment cards each suit has; number cards use Rank(2)..Rank(10).
type Rank int

// RankInvestment is the sentinel Rank value for an investment/wager card.
const RankInvestment Rank = 0

// investmentsPerSuit and numberCardsPerSuit define the standard deck: 3
// investment cards plus one number card for each of 2..10, per suit — 12
// cards per suit, 60 total across 5 suits.
const (
	investmentsPerSuit = 3
	minNumberRank      = 2
	maxNumberRank      = 10
)

// Card is a single expedition card: which suit it belongs to, and its rank
// (RankInvestment, or a number 2..10).
type Card struct {
	Suit Suit
	Rank Rank
}

// IsInvestment reports whether c is one of a suit's 3 investment/wager cards.
func (c Card) IsInvestment() bool { return c.Rank == RankInvestment }

// Shuffler is anything that can permute a deck in place, satisfied by
// (*rand.Rand).Shuffle. Injected so tests can supply a deterministic or
// no-op shuffler (mirrors sushi/game's Shuffler).
type Shuffler func(n int, swap func(i, j int))

// NewDeck builds one standard 60-card Expeditionen deck: 5 suits, each with
// 3 investment cards and 9 number cards (2 through 10).
func NewDeck() []Card {
	d := make([]Card, 0, NumSuits*(investmentsPerSuit+(maxNumberRank-minNumberRank+1)))
	for _, s := range AllSuits {
		for i := 0; i < investmentsPerSuit; i++ {
			d = append(d, Card{Suit: s, Rank: RankInvestment})
		}
		for r := minNumberRank; r <= maxNumberRank; r++ {
			d = append(d, Card{Suit: s, Rank: Rank(r)})
		}
	}
	return d
}

func shuffleDeck(d []Card, shuffle Shuffler) {
	if shuffle == nil {
		return
	}
	shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
}

// CountInvestments returns how many investment cards are in row.
func CountInvestments(row []Card) int {
	n := 0
	for _, c := range row {
		if c.IsInvestment() {
			n++
		}
	}
	return n
}

// SumNumbers returns the sum of the number-card ranks in row (investment
// cards contribute nothing to the sum).
func SumNumbers(row []Card) int {
	sum := 0
	for _, c := range row {
		if !c.IsInvestment() {
			sum += int(c.Rank)
		}
	}
	return sum
}
