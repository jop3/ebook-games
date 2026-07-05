// Package game implements Mosaik's rules, scoring, and AI as pure logic with
// no SDK dependency, so it is unit-tested and `go vet`/`go test` clean
// without cgo (see ../main.go and ../ui.go for rendering/input).
//
// Mosaik is a simplified 2-player reimplementation of Azul (Michael
// Kiesling, Next Move/Plan B Games): draft coloured tiles from shared
// factory displays onto your pattern lines, then tile them onto your wall
// for points at the end of each round — but every tile that overflows a
// line costs you on the floor line. The game ends at the end of the round in
// which any player completes a full wall row; bonuses are added once, and
// the highest total wins.
package game

// Color identifies one of the 5 tile patterns (rendered in the UI as 5
// distinct greyscale patterns: solid, ring, cross-hatch, diagonal stripes,
// dotted — see ui.go's tile chip renderer). The rules never care about the
// visual pattern, only that there are exactly 5 distinct, equally-supplied
// colors.
type Color uint8

const (
	ColorSolid Color = iota
	ColorRing
	ColorCross
	ColorStripe
	ColorDot

	NumColors = 5
)

// TilesPerColor is how many tiles of each color are in the full 100-tile
// supply (5 colors x 20 = 100), per the base Azul rules.
const TilesPerColor = 20

// WallSize is the wall's edge length (and the number of pattern lines / the
// number of factories in this 2-player simplification).
const WallSize = 5

// NumFactories is the number of factory displays for a 2-player game (per
// the base rules: 2 players -> 5 factories, each filled with 4 tiles).
const NumFactories = 5

// FactoryFill is how many tiles fill each factory display at the start of a
// round.
const FactoryFill = 4

// wallColOf returns the wall column that a tile of color c belongs in when
// placed in pattern-line/wall row `row`. This is a diagonal-shift Latin
// square:
//
//	column = (color - row) mod 5
//
// which guarantees each color appears exactly once per row and exactly once
// per column (row 0 is "in order" 0..4; each subsequent row is the previous
// row rotated left by one), matching the fixed, shared wall layout every
// Azul board uses.
func wallColOf(row int, c Color) int {
	return (((int(c) - row) % WallSize) + WallSize) % WallSize
}

// ColorAt is the inverse of wallColOf: it returns which color's fixed wall
// cell sits at (row, col). Used by the UI to paint the faint "target
// pattern" on wall cells that are not yet filled.
func ColorAt(row, col int) Color {
	return Color((col + row) % WallSize)
}

// NewBag returns a freshly shuffled bag of all 100 tiles (20 of each of the
// 5 colors), using rng for the shuffle so tests and screenshots can be made
// deterministic via a seeded source.
func NewBag(rng interface{ Intn(int) int }) []Color {
	bag := make([]Color, 0, NumColors*TilesPerColor)
	for c := Color(0); c < NumColors; c++ {
		for i := 0; i < TilesPerColor; i++ {
			bag = append(bag, c)
		}
	}
	shuffle(bag, rng)
	return bag
}

// shuffle performs a Fisher-Yates shuffle using rng.Intn for the random
// index, so callers can pass either *rand.Rand or a test double.
func shuffle(tiles []Color, rng interface{ Intn(int) int }) {
	for i := len(tiles) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		tiles[i], tiles[j] = tiles[j], tiles[i]
	}
}

// countColor returns how many tiles of color c are in tiles.
func countColor(tiles []Color, c Color) int {
	n := 0
	for _, t := range tiles {
		if t == c {
			n++
		}
	}
	return n
}

// colorsPresent returns the distinct colors present in tiles, in color-index
// order — used to enumerate move candidates from a factory or the center.
func colorsPresent(tiles []Color) []Color {
	var seen [NumColors]bool
	for _, t := range tiles {
		seen[t] = true
	}
	var out []Color
	for c := Color(0); c < NumColors; c++ {
		if seen[c] {
			out = append(out, c)
		}
	}
	return out
}

// extractColor splits tiles into the ones matching color c (taken) and
// everything else (rest), preserving relative order within each group.
func extractColor(tiles []Color, c Color) (taken, rest []Color) {
	for _, t := range tiles {
		if t == c {
			taken = append(taken, t)
		} else {
			rest = append(rest, t)
		}
	}
	return taken, rest
}
