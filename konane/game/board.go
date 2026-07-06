// Package game implements the rules of Konane ("stone jumping"), the
// traditional Hawaiian jump-capture game, with no dependency on the inkview
// SDK, so it can be unit-tested cgo-free.
//
// The board is 8x8, completely filled with alternating black/white stones
// (a checkerboard fill, like a fully-set chessboard with no empty squares).
// A one-time opening phase removes exactly two stones (Black removes one of
// the two center stones, then White removes one of its own stones adjacent
// to the resulting gap); after that, the ONLY move in the entire game is a
// jump: move a stone orthogonally over an adjacent enemy stone into the
// empty cell immediately beyond, removing the jumped stone. A single turn
// may chain multiple jumps with the same stone. There is no non-capturing
// move and no pass: a side with zero legal jumps on its turn loses at once.
package game

import "image"

// Cell is one of the three states of a board square.
type Cell uint8

const (
	Empty Cell = iota
	Black
	White
)

// Side names a player color. It is an alias of Cell so the same values
// (Black/White) are used both for board contents and to say whose turn it
// is; Empty is never a meaningful Side.
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
const Size = 8

// Board holds the 8x8 grid, indexed [y][x] (row-major, y=0 is the top row).
type Board [Size][Size]Cell

// dirs4 are the four orthogonal jump directions.
var dirs4 = [4]image.Point{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the cell at (x,y). Out-of-bounds coordinates return Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b[y][x]
}

func (b *Board) set(x, y int, c Cell) { b[y][x] = c }

// NewBoard returns a board completely filled in an alternating checkerboard
// pattern: Black on cells where (x+y) is even, White where it is odd. Every
// one of the 64 cells starts occupied — there are no empty squares until the
// opening phase removes the first two stones.
func NewBoard() Board {
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if (x+y)%2 == 0 {
				b.set(x, y, Black)
			} else {
				b.set(x, y, White)
			}
		}
	}
	return b
}

// Count returns the number of stones of the given color on the board.
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
