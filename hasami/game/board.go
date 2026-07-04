// Package game implements the rules of Hasami shogi ("scissors shogi") with no
// dependency on the inkview SDK, so it can be unit-tested cgo-free.
//
// The board is 9x9. Each side starts with 9 men filling their nearest rank
// (Black on the bottom row, White on the top row). A man moves like a rook:
// any distance in a straight line, horizontally or vertically, onto an empty
// square, without jumping over other men. Capturing is custodial: a straight
// run of one or more enemy men bracketed, as a direct result of the mover's
// move, by two of the mover's men is removed; a man sitting in a board corner
// is captured when the mover occupies both cells orthogonally adjacent to
// that corner.
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
// (Black/White) are used both for board contents and to say whose turn/move
// it is; Empty is never a meaningful Side.
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
const Size = 9

// Board holds the 9x9 grid, indexed [y][x] (row-major, y=0 is the top row).
type Board [Size][Size]Cell

// dirs4 are the four orthogonal rook directions.
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

// NewBoard returns a board in the standard Hasami shogi starting position:
// Black fills the bottom rank (y=Size-1), White fills the top rank (y=0).
func NewBoard() Board {
	var b Board
	for x := 0; x < Size; x++ {
		b.set(x, Size-1, Black)
		b.set(x, 0, White)
	}
	return b
}

// Count returns the number of men of the given color on the board.
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

// Move is a single rook move of one man from From to To (both board
// coordinates); it says nothing about who moved or what it captures.
type Move struct {
	From, To image.Point
}

// DestinationsFrom returns every empty square reachable by a rook move from p
// (a straight, unobstructed ray in each of the 4 orthogonal directions). It
// does not check who (if anyone) occupies p.
func (b *Board) DestinationsFrom(p image.Point) []image.Point {
	var out []image.Point
	if !inBounds(p.X, p.Y) {
		return out
	}
	for _, d := range dirs4 {
		cx, cy := p.X+d.X, p.Y+d.Y
		for inBounds(cx, cy) && b.At(cx, cy) == Empty {
			out = append(out, image.Pt(cx, cy))
			cx += d.X
			cy += d.Y
		}
	}
	return out
}

// LegalMoves returns every legal move for side: a rook move from each of
// side's men to each empty square reachable in a straight, unobstructed line.
func (b *Board) LegalMoves(side Side) []Move {
	var moves []Move
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != side {
				continue
			}
			from := image.Pt(x, y)
			for _, to := range b.DestinationsFrom(from) {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}
	return moves
}

// IsLegalMove reports whether side may legally play m: m.From must hold one
// of side's men, m.To must be empty, and the two must be connected by a clear
// straight (horizontal or vertical) line.
func (b *Board) IsLegalMove(side Side, m Move) bool {
	if !inBounds(m.From.X, m.From.Y) || !inBounds(m.To.X, m.To.Y) {
		return false
	}
	if b.At(m.From.X, m.From.Y) != side {
		return false
	}
	if m.From == m.To {
		return false
	}
	if m.From.X != m.To.X && m.From.Y != m.To.Y {
		return false // not a straight line
	}
	if b.At(m.To.X, m.To.Y) != Empty {
		return false
	}
	dx, dy := sign(m.To.X-m.From.X), sign(m.To.Y-m.From.Y)
	x, y := m.From.X+dx, m.From.Y+dy
	for x != m.To.X || y != m.To.Y {
		if b.At(x, y) != Empty {
			return false // blocked; a man cannot jump over another
		}
		x += dx
		y += dy
	}
	return true
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}
