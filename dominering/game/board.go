// Package game implements the rules of Domineering (Conway) with no
// dependency on the inkview SDK, so it can be unit-tested cgo-free.
//
// Two players share a rectangular board of empty cells. Player V may only
// place a 1x2 domino VERTICALLY (covering two cells in the same column, one
// above the other); player H may only place a domino HORIZONTALLY (covering
// two cells in the same row, side by side). Turns alternate. This is normal
// play convention: a player who cannot legally place their domino on their
// turn LOSES (equivalently, the last player able to move wins) — there is no
// misère twist.
package game

import (
	"image"
	"math/bits"
)

// Side names a player: V places vertical dominoes only, H places horizontal
// dominoes only. The orientation is fixed for the whole game — a side may
// never place in the other orientation.
type Side uint8

const (
	V Side = iota
	H
)

// Opponent returns the other side.
func (s Side) Opponent() Side {
	if s == V {
		return H
	}
	return V
}

// String names the side for status text / debugging.
func (s Side) String() string {
	if s == V {
		return "V"
	}
	return "H"
}

// Default board sizes offered on the menu.
const (
	SizeStandard = 8 // classic 8x8 ("Vanlig")
	SizeSmall    = 6 // smaller "Lätt" option
)

// Board is a square grid of occupied/empty cells. The board never exceeds
// 8x8 (64 cells) in this game, so occupancy is packed into a single uint64
// bitmask (bit y*Size+x set means that cell is covered by a domino) rather
// than a 2D slice — Apply then becomes a cheap value copy instead of an
// allocating deep copy, which matters a lot since the AI search applies and
// discards huge numbers of boards.
type Board struct {
	Size int
	occ  uint64
}

// NewBoard returns an empty size x size board (size must be <= 8).
func NewBoard(size int) Board {
	return Board{Size: size}
}

// BoardFromRows builds a Board directly from a rows[y][x] matrix (true =
// occupied, false = empty); rows must be square. Used to construct
// hand-picked positions in tests.
func BoardFromRows(rows [][]bool) Board {
	b := NewBoard(len(rows))
	for y, row := range rows {
		for x, occupied := range row {
			if occupied {
				b.markOccupied(x, y)
			}
		}
	}
	return b
}

func (b *Board) inBounds(x, y int) bool {
	return x >= 0 && x < b.Size && y >= 0 && y < b.Size
}

func (b *Board) bit(x, y int) uint64 {
	return 1 << uint(y*b.Size+x)
}

func (b *Board) markOccupied(x, y int) {
	b.occ |= b.bit(x, y)
}

// Empty reports whether (x,y) is on the board and unoccupied.
func (b *Board) Empty(x, y int) bool {
	return b.inBounds(x, y) && b.occ&b.bit(x, y) == 0
}

// EmptyCount returns the number of unoccupied cells remaining.
func (b *Board) EmptyCount() int {
	return b.Size*b.Size - bits.OnesCount64(b.occ)
}

// Move is a single domino placement: the two cells it covers. A and B are
// always orthogonally adjacent along the mover's fixed orientation (A is the
// top or left cell, B the bottom or right cell of the pair).
type Move struct {
	A, B image.Point
}

// LegalMoves returns every legal placement for side on b: every pair of
// adjacent empty cells in side's fixed orientation (vertical for V,
// horizontal for H). A side can never generate a move in the other
// orientation — that check lives entirely here, in the move generator.
func (b *Board) LegalMoves(side Side) []Move {
	var moves []Move
	switch side {
	case V:
		for y := 0; y < b.Size-1; y++ {
			for x := 0; x < b.Size; x++ {
				if b.Empty(x, y) && b.Empty(x, y+1) {
					moves = append(moves, Move{A: image.Pt(x, y), B: image.Pt(x, y+1)})
				}
			}
		}
	case H:
		for y := 0; y < b.Size; y++ {
			for x := 0; x < b.Size-1; x++ {
				if b.Empty(x, y) && b.Empty(x+1, y) {
					moves = append(moves, Move{A: image.Pt(x, y), B: image.Pt(x+1, y)})
				}
			}
		}
	}
	return moves
}

// HasMove reports whether side has at least one legal placement, without
// allocating the full move list.
func (b *Board) HasMove(side Side) bool {
	switch side {
	case V:
		for y := 0; y < b.Size-1; y++ {
			for x := 0; x < b.Size; x++ {
				if b.Empty(x, y) && b.Empty(x, y+1) {
					return true
				}
			}
		}
	case H:
		for y := 0; y < b.Size; y++ {
			for x := 0; x < b.Size-1; x++ {
				if b.Empty(x, y) && b.Empty(x+1, y) {
					return true
				}
			}
		}
	}
	return false
}

// IsLegalMove reports whether side may legally play m: A and B must be on the
// board, both empty, and adjacent along side's fixed orientation specifically
// (a vertical pair for V, a horizontal pair for H — never the other way
// round, regardless of which two adjacent empty cells are passed in).
func (b *Board) IsLegalMove(side Side, m Move) bool {
	if !b.inBounds(m.A.X, m.A.Y) || !b.inBounds(m.B.X, m.B.Y) {
		return false
	}
	if !b.Empty(m.A.X, m.A.Y) || !b.Empty(m.B.X, m.B.Y) {
		return false
	}
	switch side {
	case V:
		return m.A.X == m.B.X && abs(m.A.Y-m.B.Y) == 1
	case H:
		return m.A.Y == m.B.Y && abs(m.A.X-m.B.X) == 1
	}
	return false
}

// Apply returns a new board with m's two cells marked occupied. It does not
// check legality — callers should check IsLegalMove (or only ever pass moves
// from LegalMoves) first. This is a plain value copy (no heap allocation).
func (b *Board) Apply(m Move) Board {
	nb := *b
	nb.markOccupied(m.A.X, m.A.Y)
	nb.markOccupied(m.B.X, m.B.Y)
	return nb
}

// PartnersFrom returns, for a candidate first-tapped cell p, every OTHER
// empty cell that would complete a legal domino for side including p — i.e.
// the "ghost" second-cell candidates the UI should highlight after the first
// tap. In side's fixed orientation there are at most two such cells (the
// neighbor on each side of p along that axis); normally only one is legal
// (the other is off the board or already occupied), which is what "auto-
// highlight the single valid second cell" means in practice, but both are
// returned when both happen to be free so the UI can highlight every legal
// completion.
func (b *Board) PartnersFrom(side Side, p image.Point) []image.Point {
	if !b.Empty(p.X, p.Y) {
		return nil
	}
	var candidates []image.Point
	switch side {
	case V:
		candidates = []image.Point{{X: p.X, Y: p.Y - 1}, {X: p.X, Y: p.Y + 1}}
	case H:
		candidates = []image.Point{{X: p.X - 1, Y: p.Y}, {X: p.X + 1, Y: p.Y}}
	}
	var out []image.Point
	for _, c := range candidates {
		if b.Empty(c.X, c.Y) {
			out = append(out, c)
		}
	}
	return out
}

// MakeMove builds a Move from two adjacent cells, in canonical (A=top/left,
// B=bottom/right) order, regardless of the order p1/p2 were tapped in.
func MakeMove(p1, p2 image.Point) Move {
	if p1.Y > p2.Y || (p1.Y == p2.Y && p1.X > p2.X) {
		p1, p2 = p2, p1
	}
	return Move{A: p1, B: p2}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
