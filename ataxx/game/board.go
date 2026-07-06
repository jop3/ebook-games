// Package game implements the rules of Ataxx with no dependency on the
// inkview SDK, so it can be unit-tested cgo-free.
//
// The board is 7x7. Each side starts with 2 men in diagonally opposite
// corners: this implementation puts Black at the top-left (0,0) and
// bottom-right (6,6) corners, and White at the top-right (6,0) and
// bottom-left (0,6) corners — the classic Ataxx opening, using all four
// corners. The spec also mentions an optional variant that blocks 2 corner
// cells instead of using them for the second player's starting men; this
// implementation deliberately skips that variant (no cells are ever
// permanently blocked) in favor of the more common all-four-corners opening,
// and documents the choice here rather than in a runtime flag.
//
// A move is either a CLONE (Chebyshev distance 1 — any of the 8 neighboring
// cells: the mover's original man stays put and a new man appears at the
// destination) or a JUMP (Chebyshev distance exactly 2 — any of the 16 cells
// in the ring around the origin: the original man vacates its cell and lands
// on the destination). Either way, every one of the 8 neighbors of the
// DESTINATION that holds an enemy man flips to the mover's color — this is
// the "gotcha" that a 4-neighbor-only implementation would get wrong.
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
const Size = 7

// Board holds the 7x7 grid, indexed [y][x] (row-major, y=0 is the top row).
type Board [Size][Size]Cell

// neighbor8 are the 8 unit offsets around a cell (Chebyshev distance 1),
// used both for clone destinations and for the post-move flip scan.
var neighbor8 = [8]image.Point{
	{X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1},
	{X: -1, Y: 0}, {X: 1, Y: 0},
	{X: -1, Y: 1}, {X: 0, Y: 1}, {X: 1, Y: 1},
}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the cell at (x,y). Out-of-bounds coordinates return Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b[y][x]
}

func (b *Board) set(x, y int, c Cell) { b[y][x] = c }

// NewBoard returns a board in the classic Ataxx starting position: Black at
// the top-left and bottom-right corners, White at the top-right and
// bottom-left corners (see the package doc comment for the corner choice).
func NewBoard() Board {
	var b Board
	b.set(0, 0, Black)
	b.set(Size-1, Size-1, Black)
	b.set(Size-1, 0, White)
	b.set(0, Size-1, White)
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

// IsFull reports whether every cell on the board is occupied.
func (b *Board) IsFull() bool {
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b[y][x] == Empty {
				return false
			}
		}
	}
	return true
}

// Move is a single move of one man from From to To (both board coordinates);
// it says nothing about who moved or what it flips.
type Move struct {
	From, To image.Point
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// chebyshev returns the Chebyshev (king-move) distance for offset (dx,dy).
func chebyshev(dx, dy int) int {
	ax, ay := abs(dx), abs(dy)
	if ay > ax {
		return ay
	}
	return ax
}

// dist returns the Chebyshev distance between m.From and m.To.
func (m Move) dist() int {
	return chebyshev(m.To.X-m.From.X, m.To.Y-m.From.Y)
}

// IsClone reports whether m is a clone move (distance 1): the source stays
// occupied and a new man appears at the destination.
func (m Move) IsClone() bool { return m.dist() == 1 }

// IsJump reports whether m is a jump move (distance 2): the source is
// vacated and the man lands on the destination.
func (m Move) IsJump() bool { return m.dist() == 2 }

// CloneDestinations returns every empty cell at Chebyshev distance 1 from p
// (the "near ring" — up to 8 cells, fewer near an edge/corner).
func (b *Board) CloneDestinations(p image.Point) []image.Point {
	var out []image.Point
	for _, d := range neighbor8 {
		x, y := p.X+d.X, p.Y+d.Y
		if inBounds(x, y) && b.At(x, y) == Empty {
			out = append(out, image.Pt(x, y))
		}
	}
	return out
}

// JumpDestinations returns every empty cell at Chebyshev distance exactly 2
// from p (the "far ring" — up to 16 cells: the full ring of the 5x5
// neighborhood minus the inner 3x3, not merely the 8 straight-line
// double-distance cells).
func (b *Board) JumpDestinations(p image.Point) []image.Point {
	var out []image.Point
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if chebyshev(dx, dy) != 2 {
				continue
			}
			x, y := p.X+dx, p.Y+dy
			if inBounds(x, y) && b.At(x, y) == Empty {
				out = append(out, image.Pt(x, y))
			}
		}
	}
	return out
}

// LegalMoves returns every legal move (clone + jump) for side.
func (b *Board) LegalMoves(side Side) []Move {
	var moves []Move
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != side {
				continue
			}
			from := image.Pt(x, y)
			for _, to := range b.CloneDestinations(from) {
				moves = append(moves, Move{From: from, To: to})
			}
			for _, to := range b.JumpDestinations(from) {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}
	return moves
}

// IsLegalMove reports whether side may legally play m: m.From must hold one
// of side's men, m.To must be empty, and the two must be at Chebyshev
// distance 1 (clone) or 2 (jump) — any other distance (including 0) is not a
// legal move.
func (b *Board) IsLegalMove(side Side, m Move) bool {
	if !inBounds(m.From.X, m.From.Y) || !inBounds(m.To.X, m.To.Y) {
		return false
	}
	if b.At(m.From.X, m.From.Y) != side {
		return false
	}
	if b.At(m.To.X, m.To.Y) != Empty {
		return false
	}
	d := m.dist()
	return d == 1 || d == 2
}
