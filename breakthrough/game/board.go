// Package game implements the rules of Breakthrough with no dependency on the
// inkview SDK, so it can be unit-tested cgo-free.
//
// The board is 8 columns by 6 rows. Each side starts with pawns filling its
// nearest two ranks: Black on the bottom two rows (rows Rows-2, Rows-1),
// White on the top two rows (rows 0, 1). A pawn moves exactly one step
// straight forward onto an EMPTY square (never a capture), or exactly one
// step diagonally forward onto a square held by an ENEMY pawn (always a
// capture) — the reverse of chess pawns: straight never captures, diagonal
// always captures. There is no double-step, no en passant, and no promotion.
//
// Black's forward direction is toward row 0 (decreasing y); White's forward
// direction is toward row Rows-1 (increasing y). Black wins by reaching row
// 0 (White's back rank); White wins by reaching row Rows-1 (Black's back
// rank). A side also wins immediately if the opponent has zero pawns left,
// or if the opponent has no legal move on their turn.
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

// Board dimensions: 8 columns, 6 rows — fits the portrait screen with room to
// spare and keeps games shorter than a full 8x8.
const (
	Cols = 8
	Rows = 6
)

// Board holds the grid, indexed [y][x] (row-major, y=0 is the top row).
type Board [Rows][Cols]Cell

func inBounds(x, y int) bool { return x >= 0 && x < Cols && y >= 0 && y < Rows }

// At returns the cell at (x,y). Out-of-bounds coordinates return Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b[y][x]
}

func (b *Board) set(x, y int, c Cell) { b[y][x] = c }

// NewBoard returns a board in the standard Breakthrough starting position:
// Black fills the bottom two ranks (y = Rows-2, Rows-1), White fills the top
// two ranks (y = 0, 1).
func NewBoard() Board {
	var b Board
	for x := 0; x < Cols; x++ {
		b.set(x, Rows-1, Black)
		b.set(x, Rows-2, Black)
		b.set(x, 0, White)
		b.set(x, 1, White)
	}
	return b
}

// Count returns the number of pawns of the given color on the board.
func (b *Board) Count(c Cell) int {
	n := 0
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if b[y][x] == c {
				n++
			}
		}
	}
	return n
}

// ForwardDY returns the row-index delta of side's forward direction: Black
// advances toward row 0 (dy=-1), White advances toward row Rows-1 (dy=+1).
// Direction is mirrored by side — never hardcode "up" without checking whose
// pawn is moving.
func ForwardDY(side Side) int {
	if side == Black {
		return -1
	}
	return 1
}

// GoalRow returns the row a pawn of side must reach to win: row 0 for Black
// (White's back rank), row Rows-1 for White (Black's back rank).
func GoalRow(side Side) int {
	if side == Black {
		return 0
	}
	return Rows - 1
}

// HomeRows returns the two starting ranks for side (nearest, then next).
func HomeRows(side Side) (near, far int) {
	if side == Black {
		return Rows - 1, Rows - 2
	}
	return 0, 1
}

// Move is a single pawn move from From to To (both board coordinates); it
// says nothing about who moved. Capture is true iff this move is a diagonal
// capture (never true for a straight move, always true for a diagonal one —
// diagonal moves onto an empty square do not exist in Breakthrough).
type Move struct {
	From, To image.Point
	Capture  bool
}

// MovesFrom returns every legal move for the pawn at p, assuming p holds a
// pawn of side (callers should check that first; an empty/wrong-side p
// yields no moves since the destinations are checked against side's rules
// but the source cell's actual contents are never re-verified here).
//
// Exactly 3 candidate destinations are ever considered: one straight ahead
// (legal only onto an empty square) and two forward-diagonal (legal only
// onto a square holding an enemy pawn, which is always captured). A
// straight destination occupied by anything, friend or foe, is illegal —
// pawns never capture by moving straight. A diagonal destination that is
// empty is equally illegal — pawns never make a quiet diagonal move.
func (b *Board) MovesFrom(p image.Point, side Side) []Move {
	var out []Move
	dy := ForwardDY(side)
	enemy := side.Opponent()

	// Straight: legal only onto Empty.
	if sx, sy := p.X, p.Y+dy; inBounds(sx, sy) && b.At(sx, sy) == Empty {
		out = append(out, Move{From: p, To: image.Pt(sx, sy)})
	}
	// Diagonals: legal only onto an enemy pawn (always a capture).
	for _, dx := range [2]int{-1, 1} {
		dx2, dy2 := p.X+dx, p.Y+dy
		if inBounds(dx2, dy2) && b.At(dx2, dy2) == enemy {
			out = append(out, Move{From: p, To: image.Pt(dx2, dy2), Capture: true})
		}
	}
	return out
}

// LegalMoves returns every legal move for side across the whole board.
func (b *Board) LegalMoves(side Side) []Move {
	var moves []Move
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if b.At(x, y) != side {
				continue
			}
			moves = append(moves, b.MovesFrom(image.Pt(x, y), side)...)
		}
	}
	return moves
}

// IsLegalMove reports whether side may legally play m.
func (b *Board) IsLegalMove(side Side, m Move) bool {
	if !inBounds(m.From.X, m.From.Y) || !inBounds(m.To.X, m.To.Y) {
		return false
	}
	if b.At(m.From.X, m.From.Y) != side {
		return false
	}
	dx, dy := m.To.X-m.From.X, m.To.Y-m.From.Y
	if dy != ForwardDY(side) {
		return false
	}
	switch dx {
	case 0:
		return b.At(m.To.X, m.To.Y) == Empty
	case -1, 1:
		return b.At(m.To.X, m.To.Y) == side.Opponent()
	default:
		return false
	}
}

// Apply plays move m (assumed legal — callers should check IsLegalMove
// first) and returns the resulting board. A diagonal move always removes
// the enemy pawn at To; a straight move never removes anything (there is
// nothing to remove — To was required to be Empty).
func (b Board) Apply(m Move) Board {
	mover := b.At(m.From.X, m.From.Y)
	nb := b
	nb.set(m.From.X, m.From.Y, Empty)
	nb.set(m.To.X, m.To.Y, mover)
	return nb
}
