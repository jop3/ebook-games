package game

import "math/rand"

// Card is a single program card the robot can slot into a register.
type Card uint8

const (
	Move1 Card = iota
	Move2
	Move3
	BackUp
	RotR
	RotL
	UTurn
	CardNone Card = 255 // empty register
)

// Label is the short Swedish name shown on a card face fallback.
func (c Card) Label() string {
	switch c {
	case Move1:
		return "Fram 1"
	case Move2:
		return "Fram 2"
	case Move3:
		return "Fram 3"
	case BackUp:
		return "Back"
	case RotR:
		return "Höger"
	case RotL:
		return "Vänster"
	case UTurn:
		return "U-sväng"
	}
	return ""
}

// IsMove reports whether the card translates the robot (vs. only rotating).
func (c Card) IsMove() bool { return c == Move1 || c == Move2 || c == Move3 || c == BackUp }

// deckComposition is the program-deck card distribution, scaled down from the
// tabletop game. A robot draws its hand from a shuffled copy of this deck; the
// mix is what makes "the move you want isn't in your hand" a real constraint.
var deckComposition = []struct {
	card Card
	n    int
}{
	{Move1, 18}, {Move2, 12}, {Move3, 6}, {BackUp, 6},
	{RotR, 18}, {RotL, 18}, {UTurn, 6},
}

// newDeck returns a fresh, ordered program deck (unshuffled).
func newDeck() []Card {
	var d []Card
	for _, e := range deckComposition {
		for i := 0; i < e.n; i++ {
			d = append(d, e.card)
		}
	}
	return d
}

// drawHand draws n cards from the deck, reshuffling a fresh deck when it runs
// short (the discard pile is abstracted away — each round is an independent draw
// from a full shuffled deck, which keeps hands varied and cheap to compute).
func drawHand(rng *rand.Rand, n int) []Card {
	deck := newDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	if n > len(deck) {
		n = len(deck)
	}
	hand := make([]Card, n)
	copy(hand, deck[:n])
	return hand
}

// cardPriority is the tie-break priority a card carries when two robots are the
// same distance from the antenna (higher wins). Rotations react fastest, then
// moves — mirroring the tabletop priority ordering closely enough for ties.
func cardPriority(c Card) int {
	switch c {
	case UTurn:
		return 70
	case RotL:
		return 60
	case RotR:
		return 50
	case BackUp:
		return 40
	case Move1:
		return 30
	case Move2:
		return 20
	case Move3:
		return 10
	}
	return 0
}
