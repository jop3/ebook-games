// Package game implements the rules of Munkar (based on the board game
// Donuts by Funforge) with no dependency on the inkview SDK, so it can be
// unit-tested cgo-free.
//
// The board is 6x6, built from four fixed 3x3 tiles. Every cell carries a
// line orientation (horizontal, vertical, or one of the two diagonals).
// Players alternate placing a ring of their color on an empty cell; the
// orientation of the cell just filled dictates the row/column/diagonal the
// opponent must play on next ("direction-forcing"). Placing a ring runs a
// custodial capture check along all 4 geometric axes through it: if the
// mover's own contiguous run (including the new ring) is bounded immediately
// on both ends by an opponent ring, both those enemy bookends flip to the
// mover's color. Getting 5 of your rings in a row (any axis) wins outright;
// if the board fills with no five-in-a-row, the player with the larger
// orthogonally-connected group of rings wins (equal size is a draw).
package game

import "image"

// Cell is the color of a ring on the board (or Empty). Matches the spec's
// "0 empty / 1 / 2 (player)" via named constants, matching this repo's other
// two-player games (see othello/hasami's Cell/Black/White).
type Cell int8

const (
	Empty Cell = iota
	Black      // human in ModeAI; first player in ModeHotseat
	White      // AI in ModeAI; second player in ModeHotseat
)

// Opponent returns the other player color. Only meaningful for Black/White.
func (c Cell) Opponent() Cell {
	switch c {
	case Black:
		return White
	case White:
		return Black
	default:
		return Empty
	}
}

// Orient is the line glyph drawn in a board cell: horizontal, vertical, or
// one of the two diagonals. It both decorates the cell and (via
// direction-forcing) determines which line the opponent must play on next.
type Orient uint8

const (
	OrientH  Orient = iota // "—" horizontal
	OrientV                // "│" vertical
	OrientD1               // "╱" rising diagonal (x+y constant)
	OrientD2               // "╲" falling diagonal (x-y constant)
)

// Glyph returns the printable character for o, used by the rules text and UI.
func (o Orient) Glyph() string {
	switch o {
	case OrientH:
		return "—"
	case OrientV:
		return "│"
	case OrientD1:
		return "╱"
	case OrientD2:
		return "╲"
	default:
		return "?"
	}
}

// Size is the edge length of the board.
const Size = 6

// Board holds the fixed per-cell line orientations plus the mutable ring
// contents. Both are fixed-size arrays so Board is a small, copyable,
// comparable value (no pointers/slices), matching this repo's other games.
type Board struct {
	Line [Size][Size]Orient // fixed for the lifetime of a game
	Ring [Size][Size]Cell   // 0 (Empty) / Black / White
}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the ring color at (x,y). Out-of-bounds coordinates return Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b.Ring[y][x]
}

// Count returns the number of rings of the given color on the board.
func (b *Board) Count(c Cell) int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.Ring[y][x] == c {
				n++
			}
		}
	}
	return n
}

// --- fixed tiles -------------------------------------------------------

// baseTile is the fixed 3x3 line-orientation pattern used to build every
// quadrant of the 6x6 board (design-locked per the spec): a balanced mix of
// all four orientations with no monotonous line.
//
//	—  │  ╱
//	╲  —  │
//	│  ╱  —
var baseTile = [3][3]Orient{
	{OrientH, OrientV, OrientD1},
	{OrientD2, OrientH, OrientV},
	{OrientV, OrientD1, OrientH},
}

// rotateOrient returns o rotated 90 degrees: physically rotating the board
// swaps horizontal<->vertical and swaps the two diagonals.
func rotateOrient(o Orient) Orient {
	switch o {
	case OrientH:
		return OrientV
	case OrientV:
		return OrientH
	case OrientD1:
		return OrientD2
	case OrientD2:
		return OrientD1
	default:
		return o
	}
}

// rotateTile returns t rotated 90 degrees clockwise: both the grid position
// of each cell and its own line orientation rotate together.
func rotateTile(t [3][3]Orient) [3][3]Orient {
	var out [3][3]Orient
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			out[y][x] = rotateOrient(t[2-x][y])
		}
	}
	return out
}

// quadrantOrigins are the top-left corners of the four 3x3 tiles making up
// the 6x6 board.
var quadrantOrigins = [4]image.Point{{X: 0, Y: 0}, {X: 3, Y: 0}, {X: 0, Y: 3}, {X: 3, Y: 3}}

// RandSource is the minimal randomness Board construction needs — satisfied
// by *math/rand.Rand — kept as an interface so game/ never has to import
// math/rand's full surface into its public API.
type RandSource interface {
	Intn(n int) int
}

// buildLineGrid fills a 6x6 grid of line orientations from four copies of
// baseTile, each independently rotated by a random multiple of 90 degrees.
// This is the ONLY source of randomness in a Munkar game (see NewBoard):
// each of the four quadrants independently gets one of the tile's 4
// rotations, so both which rotation appears and where it lands are
// effectively shuffled.
func buildLineGrid(rng RandSource) [Size][Size]Orient {
	var grid [Size][Size]Orient
	for _, q := range quadrantOrigins {
		tile := baseTile
		for i, n := 0, rng.Intn(4); i < n; i++ {
			tile = rotateTile(tile)
		}
		for y := 0; y < 3; y++ {
			for x := 0; x < 3; x++ {
				grid[q.Y+y][q.X+x] = tile[y][x]
			}
		}
	}
	return grid
}

// NewBoard returns an empty board with a freshly-shuffled line layout.
func NewBoard(rng RandSource) Board {
	return Board{Line: buildLineGrid(rng)}
}

// --- direction-forcing ---------------------------------------------------

// axisCells returns every cell (in-bounds) on the line through p implied by
// orientation o: the full row/column for H/V, or the full diagonal for
// D1/D2. p itself is included.
func axisCells(o Orient, p image.Point) []image.Point {
	var out []image.Point
	switch o {
	case OrientH:
		for x := 0; x < Size; x++ {
			out = append(out, image.Pt(x, p.Y))
		}
	case OrientV:
		for y := 0; y < Size; y++ {
			out = append(out, image.Pt(p.X, y))
		}
	case OrientD1: // "╱": x+y is constant along the line
		s := p.X + p.Y
		for x := 0; x < Size; x++ {
			if y := s - x; y >= 0 && y < Size {
				out = append(out, image.Pt(x, y))
			}
		}
	case OrientD2: // "╲": x-y is constant along the line
		d := p.X - p.Y
		for x := 0; x < Size; x++ {
			if y := x - d; y >= 0 && y < Size {
				out = append(out, image.Pt(x, y))
			}
		}
	}
	return out
}

// ForcedCells returns the empty cells on the line through last, per the
// orientation of the cell at last (which must already hold a ring — it was
// the most recently placed one). An empty result means the line is
// completely full, so the opponent may play anywhere.
func ForcedCells(b Board, last image.Point) []image.Point {
	o := b.Line[last.Y][last.X]
	var out []image.Point
	for _, p := range axisCells(o, last) {
		if p == last {
			continue
		}
		if b.At(p.X, p.Y) == Empty {
			out = append(out, p)
		}
	}
	return out
}

// emptyCells returns every empty cell on the board, in row-major order.
func emptyCells(b Board) []image.Point {
	var out []image.Point
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.Ring[y][x] == Empty {
				out = append(out, image.Pt(x, y))
			}
		}
	}
	return out
}

// LegalMoves returns the cells a player may legally place on: the forced
// cells implied by the previous placement (last, only meaningful if
// hasLast), or every empty cell if there is no constraint yet (hasLast is
// false, i.e. this is the game's very first move) or the forced line is
// completely full (ForcedCells returns empty).
func LegalMoves(b Board, last image.Point, hasLast bool) []image.Point {
	if hasLast {
		if forced := ForcedCells(b, last); len(forced) > 0 {
			return forced
		}
	}
	return emptyCells(b)
}
