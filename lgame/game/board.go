// Package game implements the rules of the L-Game (Edward de Bono's abstract
// board game) with no dependency on the inkview SDK, so it can be
// unit-tested cgo-free.
//
// The board is a fixed 4x4 grid. Each side owns one L-tetromino piece (4
// cells) that may be placed in any of its 8 rotation/reflection
// orientations; two neutral single-cell pieces are shared on the board.
//
// A full turn is: (1) mandatory — lift your own L piece and place it
// somewhere new (any orientation), not overlapping any other piece; (2)
// optional — move either neutral piece to any other empty cell. If, on your
// turn, you have no legal placement for your L piece at all (given the
// board as it stands, before you'd move), you lose immediately.
package game

import "image"

// Cell is one of the four states of a board square.
type Cell uint8

const (
	Empty Cell = iota
	Black
	White
	Neutral
)

// Side names a player color. It is an alias of Cell so the same values
// (Black/White) are used both for board contents and to say whose turn it
// is; Empty and Neutral are never meaningful Sides.
type Side = Cell

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

// Size is the edge length of the board.
const Size = 4

// Board holds the 4x4 grid, indexed [y][x] (row-major, y=0 is the top row).
type Board [Size][Size]Cell

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the cell at (x,y). Out-of-bounds coordinates return Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b[y][x]
}

func (b *Board) set(x, y int, c Cell) { b[y][x] = c }

// Count returns the number of cells holding the given value.
func (b *Board) Count(c Cell) int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b[y][x] == c {
				n++
			}
		}
	}
	return n
}

// NewBoard returns a fresh, legal, 180°-rotationally-symmetric starting
// position: each side's L-piece is a valid orientation (see shapes.go), the
// two neutral pieces sit at a symmetric pair of the remaining cells, no
// pieces overlap, and — checked by an exhaustive search over every
// orientation/anchor/neutral-pair combination in board_test.go — neither
// side can immediately trap the other in a single opening move.
//
// The de Bono rulebook doesn't fix one canonical starting diagram the way
// some other games' exact rosters do (nothing in the spec called out a
// specific layout to verify), so this is a deliberately constructed,
// verified-balanced layout rather than a guessed reproduction of a printed
// diagram. (An earlier, more wall-hugging candidate layout was tried and
// rejected during development: it let Black force an immediate win on the
// very first move, which an exhaustive safety search caught — see the
// "no immediate forced win from the start" test.)
//
//	B B B N
//	. . B .
//	. W . .
//	N W W W
func NewBoard() Board {
	var b Board
	// Black L: row y=0 columns 0-2, plus a foot at (2,1).
	for _, p := range []image.Point{{0, 0}, {1, 0}, {2, 0}, {2, 1}} {
		b.set(p.X, p.Y, Black)
	}
	// White L: the point-reflection of Black's L through the board center,
	// i.e. (x,y) -> (3-x, 3-y) — guaranteed to be a valid orientation (a
	// 180° rotation is one of the 8 orientations) and guaranteed not to
	// overlap Black's cells since none of Black's cells are self-symmetric.
	for _, p := range []image.Point{{3, 3}, {2, 3}, {1, 3}, {1, 2}} {
		b.set(p.X, p.Y, White)
	}
	// Two neutral pieces at a symmetric pair of the remaining empty cells.
	b.set(3, 0, Neutral)
	b.set(0, 3, Neutral)
	return b
}

// occupiedCells returns every cell currently holding c, in row-major order.
func (b *Board) occupiedCells(c Cell) []image.Point {
	var out []image.Point
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b[y][x] == c {
				out = append(out, image.Pt(x, y))
			}
		}
	}
	return out
}
