// Package game implements the rules, round engine, scoring, and AI for
// Geishorna — a reimplementation of the Hanamikoji formula under a neutral
// working title with an original geisha roster. It has no dependency on the
// ink SDK, so it builds and tests natively (no cgo, no device emulator) with
// `go test ./game/`.
//
// Baserat på Hanamikoji (Kota Nakayama / EmperorS4).
package game

// NumGeishas is the number of geishas (and item types) in the game.
const NumGeishas = 7

// Geisha is one of the 7 geishas whose favor the players contest. Charm is
// her point value; it is ALSO the number of item cards of her type in the
// deck (a value-2 geisha has 2 cards, value-5 has 5, and so on), so the deck
// totals 2+2+2+3+3+4+5 = 21 cards. Tag is the compact 1-letter label drawn on
// item cards and geisha tracks (ASCII only — the device renders unknown
// glyphs as broken boxes; see POCKETBOOK_GAMEDEV_GUIDE §9).
type Geisha struct {
	Name  string
	Tag   string
	Charm int
}

// Geishas is the fixed roster, index-stable (a Card's Geisha field is an
// index into this array). Listed high charm to low so display order can just
// follow the slice. The names are original flavor, not the source game's.
var Geishas = [NumGeishas]Geisha{
	{"Pärlan", "P", 5},
	{"Vinet", "V", 4},
	{"Blomman", "B", 3},
	{"Fjädern", "F", 3},
	{"Teet", "T", 2},
	{"Bläcket", "K", 2},
	{"Slöjan", "S", 2},
}

// TotalCards is the size of a full item deck (== sum of every geisha's charm).
const TotalCards = 21

// TotalCharm is the sum of every geisha's charm (21). A player needs favor of
// geishas totaling >= CharmToWin of these to win the match.
const TotalCharm = 21

// Card is a single item card, identified only by which geisha it belongs to.
type Card struct {
	Geisha int
}

// Charm returns the charm value of the geisha this card belongs to.
func (c Card) Charm() int { return Geishas[c.Geisha].Charm }

// Tag returns the 1-letter label of the geisha this card belongs to.
func (c Card) Tag() string { return Geishas[c.Geisha].Tag }

// NewDeck builds one standard 21-card item deck: Charm copies of each
// geisha's item card.
func NewDeck() []Card {
	d := make([]Card, 0, TotalCards)
	for g := 0; g < NumGeishas; g++ {
		for n := 0; n < Geishas[g].Charm; n++ {
			d = append(d, Card{Geisha: g})
		}
	}
	return d
}

// Shuffler is anything that can permute a deck in place, satisfied by
// (*rand.Rand).Shuffle. Injected so tests can supply a deterministic or no-op
// shuffler (mirrors sushi/expeditionen's Shuffler).
type Shuffler func(n int, swap func(i, j int))

func shuffleDeck(d []Card, shuffle Shuffler) {
	if shuffle == nil {
		return
	}
	shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
}
