// Package game implements the rules of Stadskärnan ("The Town Core"), a
// territory-enclosure game for two players inspired by the mechanics of
// Cathedral (Gamewright/Playroom Entertainment; original board game design by
// Robert P. Moore). This is an original implementation: the piece geometry,
// names and art below are our own — only the enclosure/capture MECHANIC is
// shared with the game that inspired it (game rules and mechanics are not
// copyrightable; the specific 13-piece shape roster here was designed from
// scratch for this port, not copied from any rulebook).
//
// No dependency on the inkview SDK, so this package is fully unit-testable
// cgo-free.
//
// The board is 10x10. A single neutral 5-square cross-shaped Cathedral piece
// is placed first (by Black, by convention); White then takes the first
// regular turn. Players alternate placing one of their own remaining
// building pieces (any rotation/reflection) onto empty, unsealed cells.
// After every placement, any region of empty/opponent cells that becomes
// fully walled off by one color's pieces (and/or the Cathedral) without
// touching the board edge is enclosed: opposing pieces caught inside are
// captured (removed and returned to that opponent's hand); the region itself
// is permanently sealed (no further placements there by either side). The
// game ends once neither side can legally place any remaining piece; the
// winner is whoever has fewest total unplaced squares left in hand.
package game

import "image"

// Size is the edge length of the board.
const Size = 10

// Cell is one of the four states of a board square.
type Cell uint8

const (
	Empty Cell = iota
	Black
	White
	Cathedral
)

// Opponent returns the other player color. Only meaningful for Black/White;
// Cathedral and Empty have no opponent and return Empty.
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

// Board holds the 10x10 grid (indexed [y][x], y=0 is the top row), which
// piece occupies each cell (for capture bookkeeping), and which cells have
// been permanently sealed by a past enclosure.
type Board struct {
	Owner   [Size][Size]Cell
	PieceID [Size][Size]int8 // -1 = no piece occupies this cell
	Sealed  [Size][Size]bool // true = no placement here, ever again
}

// NewBoard returns an empty board (no Cathedral placed yet).
func NewBoard() Board {
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.PieceID[y][x] = -1
		}
	}
	return b
}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the owner of the cell at (x,y). Out-of-bounds returns Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b.Owner[y][x]
}

// IsSealed reports whether (x,y) has been permanently sealed by a past
// enclosure. Out-of-bounds is reported as sealed (never placeable).
func (b *Board) IsSealed(x, y int) bool {
	if !inBounds(x, y) {
		return true
	}
	return b.Sealed[y][x]
}

// Count returns the number of cells currently owned by c (Black, White or
// Cathedral).
func (b *Board) Count(c Cell) int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.Owner[y][x] == c {
				n++
			}
		}
	}
	return n
}

// dirs4 are the four orthogonal directions used for adjacency/flood-fill.
var dirs4 = [4]image.Point{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}
