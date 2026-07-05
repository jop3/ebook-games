// Package game implements the rules, scoring, drafting engine, and AI for
// Sushi — a reimplementation of the Sushi Go! card-drafting formula under an
// original neutral name. It has no dependency on the ink SDK, so it builds
// and tests natively (no cgo, no device emulator) with `go test ./game/`.
package game

// CardKind identifies one of the game's card categories. Nigiri and Maki
// carry a subtype (point value / icon count) in Card.N; the rest ignore N.
type CardKind uint8

const (
	KindTempura CardKind = iota
	KindSashimi
	KindDumpling
	KindNigiri
	KindWasabi
	KindMaki
	KindChopsticks
	KindPudding
)

func (k CardKind) String() string {
	switch k {
	case KindTempura:
		return "Tempura"
	case KindSashimi:
		return "Sashimi"
	case KindDumpling:
		return "Dumplings"
	case KindNigiri:
		return "Nigiri"
	case KindWasabi:
		return "Wasabi"
	case KindMaki:
		return "Maki"
	case KindChopsticks:
		return "Chopsticks"
	case KindPudding:
		return "Pudding"
	default:
		return "?"
	}
}

// Card is a single sushi card. For Kind==KindNigiri, N is the base point
// value: 1=egg, 2=salmon, 3=squid. For Kind==KindMaki, N is the number of
// maki icons printed on the card (1, 2, or 3). N is unused (0) otherwise.
type Card struct {
	Kind CardKind
	N    int
}

// Convenience constructors used by deck-building, tests, and the AI.
func NigiriEgg() Card    { return Card{Kind: KindNigiri, N: 1} }
func NigiriSalmon() Card { return Card{Kind: KindNigiri, N: 2} }
func NigiriSquid() Card  { return Card{Kind: KindNigiri, N: 3} }
func Maki1() Card        { return Card{Kind: KindMaki, N: 1} }
func Maki2() Card        { return Card{Kind: KindMaki, N: 2} }
func Maki3() Card        { return Card{Kind: KindMaki, N: 3} }
func Tempura() Card      { return Card{Kind: KindTempura} }
func Sashimi() Card      { return Card{Kind: KindSashimi} }
func Dumpling() Card     { return Card{Kind: KindDumpling} }
func Wasabi() Card       { return Card{Kind: KindWasabi} }
func Chopsticks() Card   { return Card{Kind: KindChopsticks} }
func Pudding() Card      { return Card{Kind: KindPudding} }

// Hand is the set of cards a player is currently holding, offered face-up for
// drafting one (or, with Chopsticks, two) at a time.
type Hand []Card

// countKind returns how many cards of the given kind are in cards.
func countKind(cards []Card, k CardKind) int {
	n := 0
	for _, c := range cards {
		if c.Kind == k {
			n++
		}
	}
	return n
}

// makiCount returns the total number of maki icons (summed over N) in cards.
func makiCount(cards []Card) int {
	n := 0
	for _, c := range cards {
		if c.Kind == KindMaki {
			n += c.N
		}
	}
	return n
}

// --- Deck --------------------------------------------------------------

// NewDeck builds one standard 108-card Sushi deck. Composition (this is the
// well-known standard Sushi Go! deck, used as-is since the spec does not
// prescribe exact counts):
//
//	14 Tempura, 14 Sashimi, 14 Dumpling,
//	5 Nigiri-egg, 10 Nigiri-salmon, 5 Nigiri-squid (20 total),
//	6 Wasabi,
//	6 Maki(1), 12 Maki(2), 8 Maki(3) (26 total),
//	4 Chopsticks,
//	10 Pudding.
//
// Total: 14+14+14+20+6+26+4+10 = 108.
func NewDeck() []Card {
	d := make([]Card, 0, 108)
	add := func(c Card, n int) {
		for i := 0; i < n; i++ {
			d = append(d, c)
		}
	}
	add(Tempura(), 14)
	add(Sashimi(), 14)
	add(Dumpling(), 14)
	add(NigiriEgg(), 5)
	add(NigiriSalmon(), 10)
	add(NigiriSquid(), 5)
	add(Wasabi(), 6)
	add(Maki1(), 6)
	add(Maki2(), 12)
	add(Maki3(), 8)
	add(Chopsticks(), 4)
	add(Pudding(), 10)
	return d
}

// Shuffler is anything that can permute a deck in place, satisfied by
// (*rand.Rand).Shuffle. Injected so tests can supply a deterministic or
// no-op shuffler.
type Shuffler func(n int, swap func(i, j int))

func shuffleDeck(d []Card, shuffle Shuffler) {
	if shuffle == nil {
		return
	}
	shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
}
